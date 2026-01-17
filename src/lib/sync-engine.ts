import { RuleConfig, ClientType, ArtifactMeta, TransformersConfig } from "./schema";
import {
  getConfig,
  getArtifactMeta,
  saveArtifactMeta,
  acquireRuleLock,
  releaseRuleLock,
  acquireGlobalSyncLock,
  releaseGlobalSyncLock,
  createJob,
  completeJob,
  incrementDailyStats,
  updateLastSyncInfo,
  uploadRuleContent,
} from "./storage-adapter";
import { computeHash } from "./transformer";
import { fetchSource } from "./sync-engine/fetcher";
import { processRule } from "./sync-engine/processor";
import { extractDependencies, topologicalSort } from "./sync-engine/dependency-graph";

export { detectCircularDependency } from "./sync-engine/dependency-graph";

// 执行全量同步
export async function executeFullSync(): Promise<{
  success: boolean;
  changedRules: string[];
  failedRules: { name: string; error: string }[];
  jobId: string;
}> {
  const lockResult = await acquireGlobalSyncLock();
  if (!lockResult.acquired) {
    return {
      success: false,
      changedRules: [],
      failedRules: [{ name: "sync", error: lockResult.reason || "Failed to acquire sync lock" }],
      jobId: "",
    };
  }

  const job = await createJob("full_sync");
  const changedRules: string[] = [];
  const failedRules: { name: string; error: string }[] = [];
  let blobWriteCount = 0;

  try {
    const config = await getConfig();
    const sortedRules = topologicalSort(config.rules);

    const ruleContentsCache = new Map<string, Map<ClientType, string>>();

    for (const rule of sortedRules) {
      const processResult = await processRule(
        rule,
        config.transformers || {},
        ruleContentsCache
      );

      if (processResult.errors.length > 0) {
        failedRules.push({
          name: rule.name,
          error: processResult.errors.join("; "),
        });
        continue;
      }

      ruleContentsCache.set(rule.name, processResult.contents);

      for (const [client, content] of processResult.contents) {
        const hash = await computeHash(content);
        const existingMeta = await getArtifactMeta(rule.name, client);

        if (!existingMeta || existingMeta.lastHash !== hash) {
          const { url, path } = await uploadRuleContent(rule.name, client, content);
          blobWriteCount++;

          const meta: ArtifactMeta = {
            ruleName: rule.name,
            client,
            lastHash: hash,
            lastUpdatedAt: new Date().toISOString(),
            blobPath: path,
            blobUrl: url,
            sizeBytes: new TextEncoder().encode(content).length,
          };
          await saveArtifactMeta(meta);

          if (!changedRules.includes(rule.name)) {
            changedRules.push(rule.name);
          }
        }
      }
    }

    const today = new Date().toISOString().split("T")[0];
    await incrementDailyStats(today, {
      syncCount: 1,
      blobWriteCount,
      rulesChanged: changedRules.length,
      totalRulesProcessed: sortedRules.length,
      failedSources: failedRules.length,
    });

    const syncInfo: Parameters<typeof updateLastSyncInfo>[0] = {
      lastFullSyncAt: new Date().toISOString(),
      totalRulesCount: sortedRules.length,
      changedRulesCount: changedRules.length,
      failedRulesCount: failedRules.length,
    };
    if (failedRules.length === 0) {
      syncInfo.lastSuccessfulSyncAt = new Date().toISOString();
    }
    await updateLastSyncInfo(syncInfo);

    await completeJob(job.jobId, changedRules, failedRules);

    return {
      success: failedRules.length === 0,
      changedRules,
      failedRules,
      jobId: job.jobId,
    };
  } finally {
    await releaseGlobalSyncLock();
  }
}

// 执行局部同步（刷新指定规则及其下游依赖）
export async function executePartialSync(ruleName: string): Promise<{
  success: boolean;
  changedRules: string[];
  failedRules: { name: string; error: string }[];
  jobId: string;
}> {
  const lockResult = await acquireRuleLock(ruleName);
  if (!lockResult.acquired) {
    return {
      success: false,
      changedRules: [],
      failedRules: [{ name: ruleName, error: lockResult.reason || "Rule is being updated" }],
      jobId: "",
    };
  }

  try {
    const config = await getConfig();

    const affectedRules = new Set<string>([ruleName]);

    function findDependents(name: string) {
      for (const rule of config.rules) {
        const dependsViaCompose = rule.compose_from?.includes(name);
        const dependsViaRef = rule.sources?.some((s) => s.type === "ref" && s.ref === name);

        if (dependsViaCompose || dependsViaRef) {
          if (!affectedRules.has(rule.name)) {
            affectedRules.add(rule.name);
            findDependents(rule.name);
          }
        }
      }
    }
    findDependents(ruleName);

    const rulesToProcess = config.rules.filter((r) => affectedRules.has(r.name));
    const sortedRules = topologicalSort(rulesToProcess, true);

    const job = await createJob("partial_sync", Array.from(affectedRules));
    const changedRules: string[] = [];
    const failedRules: { name: string; error: string }[] = [];
    let blobWriteCount = 0;

    const ruleContentsCache = new Map<string, Map<ClientType, string>>();

    const allDependencies = new Set<string>();
    const processed = new Set<string>();

    function collectTransitiveDeps(ruleNameToCheck: string) {
      if (processed.has(ruleNameToCheck)) return;
      processed.add(ruleNameToCheck);

      const rule = config.rules.find((r) => r.name === ruleNameToCheck);
      if (!rule) return;

      const deps = extractDependencies(rule);
      for (const dep of deps) {
        if (!affectedRules.has(dep)) {
          allDependencies.add(dep);
          collectTransitiveDeps(dep);
        }
      }
    }

    for (const rule of sortedRules) {
      collectTransitiveDeps(rule.name);
    }

    if (allDependencies.size > 0) {
      const depRules = config.rules.filter((r) => allDependencies.has(r.name));
      const sortedDepRules = topologicalSort(depRules, true);

      for (const depRule of sortedDepRules) {
        const result = await processRule(depRule, config.transformers || {}, ruleContentsCache);
        if (result.contents.size > 0) {
          ruleContentsCache.set(depRule.name, result.contents);
        }
      }
    }

    for (const rule of sortedRules) {
      const processResult = await processRule(
        rule,
        config.transformers || {},
        ruleContentsCache
      );

      if (processResult.errors.length > 0) {
        failedRules.push({
          name: rule.name,
          error: processResult.errors.join("; "),
        });
        continue;
      }

      ruleContentsCache.set(rule.name, processResult.contents);

      for (const [client, content] of processResult.contents) {
        const hash = await computeHash(content);
        const existingMeta = await getArtifactMeta(rule.name, client);

        if (!existingMeta || existingMeta.lastHash !== hash) {
          const { url, path } = await uploadRuleContent(rule.name, client, content);
          blobWriteCount++;

          const meta: ArtifactMeta = {
            ruleName: rule.name,
            client,
            lastHash: hash,
            lastUpdatedAt: new Date().toISOString(),
            blobPath: path,
            blobUrl: url,
            sizeBytes: new TextEncoder().encode(content).length,
          };
          await saveArtifactMeta(meta);

          if (!changedRules.includes(rule.name)) {
            changedRules.push(rule.name);
          }
        }
      }
    }

    const today = new Date().toISOString().split("T")[0];
    await incrementDailyStats(today, {
      blobWriteCount,
      rulesChanged: changedRules.length,
      totalRulesProcessed: sortedRules.length,
    });

    await updateLastSyncInfo({
      lastPartialSyncAt: new Date().toISOString(),
    });

    await completeJob(job.jobId, changedRules, failedRules);

    return {
      success: failedRules.length === 0,
      changedRules,
      failedRules,
      jobId: job.jobId,
    };
  } finally {
    await releaseRuleLock(ruleName);
  }
}

// 预览规则（不保存）
export async function previewRule(
  rule: RuleConfig,
  transformersConfig: TransformersConfig = {},
  limitLines: number = 2000
): Promise<{
  contents: Map<ClientType, string>;
  diagnostics: {
    sourceResults: { url: string; success: boolean; error?: string; size?: number }[];
    truncated: boolean;
    totalLines: number;
  };
}> {
  const ruleContentsCache = new Map<string, Map<ClientType, string>>();

  const allDependencies = new Set<string>();

  if (rule.compose_from) {
    for (const dep of rule.compose_from) {
      allDependencies.add(dep);
    }
  }

  if (rule.sources) {
    for (const source of rule.sources) {
      if (source.type === "ref" && source.ref) {
        allDependencies.add(source.ref);
      }
    }
  }

  if (allDependencies.size > 0) {
    const config = await getConfig();

    const depsToProcess = new Set<string>(allDependencies);
    const processed = new Set<string>();

    function collectTransitiveDeps(depName: string) {
      const depRule = config.rules.find((r) => r.name === depName);
      if (!depRule) return;

      const subDeps = extractDependencies(depRule);
      for (const subDep of subDeps) {
        if (!depsToProcess.has(subDep) && !processed.has(subDep)) {
          depsToProcess.add(subDep);
          collectTransitiveDeps(subDep);
        }
      }
    }

    for (const dep of allDependencies) {
      collectTransitiveDeps(dep);
    }

    const depsRules = config.rules.filter((r) => depsToProcess.has(r.name));
    const sortedDeps = topologicalSort(depsRules, true);

    for (const depRule of sortedDeps) {
      if (!processed.has(depRule.name)) {
        const result = await processRule(depRule, transformersConfig, ruleContentsCache);
        if (result.contents.size > 0) {
          ruleContentsCache.set(depRule.name, result.contents);
        }
        processed.add(depRule.name);
      }
    }
  }

  const sourceResults: { url: string; success: boolean; error?: string; size?: number }[] = [];

  if (rule.sources) {
    for (const source of rule.sources) {
      const sourceType = source.type || "url";
      if (sourceType === "url" && source.url) {
        const { content, error } = await fetchSource(source.url);
        sourceResults.push({
          url: source.url,
          success: !error,
          error,
          size: content.length,
        });
      } else if (sourceType === "ref" && source.ref) {
        sourceResults.push({
          url: `ref:${source.ref}`,
          success: true,
          size: 0,
        });
      } else if (sourceType === "local") {
        sourceResults.push({
          url: "local",
          success: true,
          size: source.content?.length || 0,
        });
      }
    }
  }

  const result = await processRule(rule, transformersConfig, ruleContentsCache);

  let truncated = false;
  let totalLines = 0;

  for (const [client, content] of result.contents) {
    const lines = content.split("\n");
    totalLines = Math.max(totalLines, lines.length);

    if (lines.length > limitLines) {
      truncated = true;
      result.contents.set(client, lines.slice(0, limitLines).join("\n") + "\n# ... (truncated)");
    }
  }

  return {
    contents: result.contents,
    diagnostics: {
      sourceResults,
      truncated,
      totalLines,
    },
  };
}
