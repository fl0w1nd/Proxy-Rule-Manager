import {
  RuleConfig,
  ClientType,
  ArtifactMeta,
  TransformersConfig,
  GeositeProvider,
} from "./schema";
import {
  getConfig,
  getClients,
  getArtifactMeta,
  saveArtifactMetas,
  acquireRuleLock,
  releaseRuleLock,
  acquireGlobalSyncLock,
  releaseGlobalSyncLock,
  createJob,
  completeJob,
  incrementDailyStats,
  updateLastSyncInfo,
  uploadRuleContent,
  uploadGeositeRuleContent,
  getRuleContent,
  getGeositeRuleContent,
} from "./storage-adapter";
import {
  ChangeRecordInput,
  FailureRecord,
  recordRuleFileChanges,
  recordFailureRecords,
} from "./activity-store";
import {
  addRuleHeader,
  computeHash,
  normalizeEffectiveRuleContent,
  stripManagedRuleHeader,
} from "./transformer";
import { fetchSource } from "./sync-engine/fetcher";
import { processRule } from "./sync-engine/processor";
import { extractDependencies, topologicalSort } from "./sync-engine/dependency-graph";
import { createActivityDiff } from "./diff";
import { randomUUID } from "node:crypto";
import { refreshGeositeProvider, renderGeositeSource } from "./geosite";
import { isGeositeRule, getGeositeOutputName, getPrimaryGeositeSource } from "./rule-classification";

export { detectCircularDependency } from "./sync-engine/dependency-graph";

interface SyncExecutionResult {
  success: boolean;
  changedRules: string[];
  failedRules: { name: string; error: string }[];
  jobId: string;
}

async function uploadRuleContentToPath(
  rule: RuleConfig,
  client: ClientType,
  content: string
): Promise<{ url: string; path: string }> {
  if (isGeositeRule(rule)) {
    const source = getPrimaryGeositeSource(rule)!;
    return uploadGeositeRuleContent(client, source.provider!, getGeositeOutputName(source), content);
  }
  return uploadRuleContent(rule.name, client, content);
}

async function readRuleContentFromPath(
  rule: RuleConfig,
  client: ClientType
): Promise<string | null> {
  if (isGeositeRule(rule)) {
    const source = getPrimaryGeositeSource(rule)!;
    return getGeositeRuleContent(client, source.provider!, getGeositeOutputName(source));
  }
  return getRuleContent(rule.name, client);
}

function parseFailureDetails(message: string): { client?: ClientType; source?: string } {
  let source: string | undefined;
  const sourceMatch = message.match(/^Source (.+?):/);
  if (sourceMatch) {
    source = sourceMatch[1];
  }
  const refMatch = message.match(/^Ref "(.+?)"/);
  if (refMatch) {
    source = `ref:${refMatch[1]}`;
  }
  const refRuleMatch = message.match(/^Ref rule "(.+?)"/);
  if (refRuleMatch) {
    source = `ref:${refRuleMatch[1]}`;
  }
  const depMatch = message.match(/^Dependency rule "(.+?)"/);
  if (depMatch) {
    source = `dependency:${depMatch[1]}`;
  }

  const clientMatch = message.match(/client ([^ )]+)\b/);
  const client = clientMatch ? (clientMatch[1] as ClientType) : undefined;

  return { client, source };
}

function buildFailureRecords(
  ruleName: string,
  errors: string[],
  jobId: string,
  stage: string = "process_rule"
): FailureRecord[] {
  const timestamp = new Date().toISOString();
  return errors.map((message) => {
    const { client, source } = parseFailureDetails(message);
    return {
      id: randomUUID(),
      timestamp,
      ruleName,
      client,
      source,
      message,
      stage,
      jobId,
    };
  });
}

function shouldTrackRuleActivity(rule: RuleConfig): boolean {
  return !isGeositeRule(rule);
}

function collectGeositeProviders(rules: RuleConfig[]): GeositeProvider[] {
  const providers = new Set<GeositeProvider>();
  for (const rule of rules) {
    const source = getPrimaryGeositeSource(rule);
    if (source?.provider) {
      providers.add(source.provider);
    }
  }
  return Array.from(providers);
}

function buildDependentRuleIndex(rules: RuleConfig[]): Map<string, string[]> {
  const dependents = new Map<string, string[]>();

  for (const rule of rules) {
    for (const source of rule.sources || []) {
      if (source.type !== "ref" || !source.ref) continue;
      const current = dependents.get(source.ref) || [];
      current.push(rule.name);
      dependents.set(source.ref, current);
    }
  }

  return dependents;
}

function collectAffectedRules(rules: RuleConfig[], seedRuleNames: string[]): Set<string> {
  const affectedRules = new Set<string>(seedRuleNames);
  const dependents = buildDependentRuleIndex(rules);
  const queue = [...seedRuleNames];

  while (queue.length > 0) {
    const currentRuleName = queue.shift()!;
    for (const dependentRuleName of dependents.get(currentRuleName) || []) {
      if (affectedRules.has(dependentRuleName)) continue;
      affectedRules.add(dependentRuleName);
      queue.push(dependentRuleName);
    }
  }

  return affectedRules;
}

async function executeSelectiveSync(
  seedRuleNames: string[],
  options: { lockMode: "rule" | "global"; jobType: "partial_sync" }
): Promise<SyncExecutionResult> {
  const uniqueSeedRuleNames = Array.from(new Set(seedRuleNames));
  const primaryRuleName = uniqueSeedRuleNames[0] || "sync";
  const lockResult = options.lockMode === "global"
    ? await acquireGlobalSyncLock()
    : await acquireRuleLock(primaryRuleName);

  if (!lockResult.acquired) {
    return {
      success: false,
      changedRules: [],
      failedRules: [{ name: primaryRuleName, error: lockResult.reason || "Rule is being updated" }],
      jobId: "",
    };
  }

  try {
    const config = await getConfig();
    const clients = await getClients();
    const affectedRules = collectAffectedRules(config.rules, uniqueSeedRuleNames);
    const rulesToProcess = config.rules.filter((rule) => affectedRules.has(rule.name));
    const sortedRules = topologicalSort(rulesToProcess, true);
    const job = await createJob(options.jobType, Array.from(affectedRules));
    const changedRules: string[] = [];
    const failedRules: { name: string; error: string }[] = [];
    const ruleFileChanges: ChangeRecordInput[] = [];
    const failureRecords: FailureRecord[] = [];
    const pendingArtifactMetas: ArtifactMeta[] = [];
    let blobWriteCount = 0;

    const ruleContentsCache = new Map<string, Map<ClientType, string>>();
    const allDependencies = new Set<string>();
    const processed = new Set<string>();
    const ruleByName = new Map(config.rules.map((rule) => [rule.name, rule]));

    function collectTransitiveDeps(ruleNameToCheck: string) {
      if (processed.has(ruleNameToCheck)) return;
      processed.add(ruleNameToCheck);

      const rule = ruleByName.get(ruleNameToCheck);
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
      const depRules = config.rules.filter((rule) => allDependencies.has(rule.name));
      const sortedDepRules = topologicalSort(depRules, true);

      for (const depRule of sortedDepRules) {
        const result = await processRule(depRule, config.transformers || {}, ruleContentsCache, clients);
        if (result.contents.size > 0) {
          ruleContentsCache.set(depRule.name, result.contents);
        }
      }
    }

    for (const rule of sortedRules) {
      const trackActivity = shouldTrackRuleActivity(rule);
      const processResult = await processRule(
        rule,
        config.transformers || {},
        ruleContentsCache,
        clients
      );

      if (processResult.errors.length > 0) {
        if (trackActivity) {
          failureRecords.push(
            ...buildFailureRecords(rule.name, processResult.errors, job.jobId)
          );
        }
        failedRules.push({
          name: rule.name,
          error: processResult.errors.join("; "),
        });
        continue;
      }

      ruleContentsCache.set(rule.name, processResult.contents);

      for (const [client, content] of processResult.contents) {
        const existingMeta = await getArtifactMeta(rule.name, client);
        const normalizedContent = normalizeEffectiveRuleContent(content);
        const hash = await computeHash(normalizedContent);
        const syncedAt = new Date().toISOString();
        const outputContent = addRuleHeader(content, rule.name, rule.description, syncedAt);

        let previousContent: string | null | undefined = undefined;
        if (existingMeta) {
          previousContent = await readRuleContentFromPath(rule, client);
          if (previousContent) {
            const previousSourceContent = stripManagedRuleHeader(previousContent);
            const previousNormalizedHash = await computeHash(
              normalizeEffectiveRuleContent(previousSourceContent)
            );
            if (previousNormalizedHash === hash) {
              if (previousSourceContent === content) {
                if (existingMeta.lastHash !== hash) {
                  pendingArtifactMetas.push({ ...existingMeta, lastHash: hash });
                }
                continue;
              }

              const sizeBytes = new TextEncoder().encode(outputContent).length;
              const { url, path } = await uploadRuleContentToPath(rule, client, outputContent);
              blobWriteCount += 1;
              pendingArtifactMetas.push({
                ...existingMeta,
                lastHash: hash,
                lastUpdatedAt: syncedAt,
                blobPath: path,
                blobUrl: url,
                sizeBytes,
              });
              continue;
            }
          }
        }

        if (previousContent === undefined) {
          previousContent = await readRuleContentFromPath(rule, client);
        }

        const sizeBytes = new TextEncoder().encode(outputContent).length;
        const { url, path } = await uploadRuleContentToPath(rule, client, outputContent);
        blobWriteCount += 1;
        const meta: ArtifactMeta = {
          ruleName: rule.name,
          client,
          lastHash: hash,
          lastUpdatedAt: syncedAt,
          blobPath: path,
          blobUrl: url,
          sizeBytes,
        };
        pendingArtifactMetas.push(meta);

        if (trackActivity) {
          const previousSourceContent = stripManagedRuleHeader(previousContent);
          const diff = createActivityDiff(
            existingMeta ? "updated" : "created",
            normalizeEffectiveRuleContent(previousSourceContent),
            normalizedContent
          );
          const changeRecord: ChangeRecordInput = {
            id: randomUUID(),
            timestamp: meta.lastUpdatedAt,
            ruleName: rule.name,
            client,
            changeType: existingMeta ? "updated" : "created",
            diff,
            sizeBytes,
          };
          ruleFileChanges.push(changeRecord);
        }

        if (!changedRules.includes(rule.name)) {
          changedRules.push(rule.name);
        }
      }
    }

    await saveArtifactMetas(pendingArtifactMetas);

    const today = new Date().toISOString().split("T")[0];
    await incrementDailyStats(today, {
      blobWriteCount,
      rulesChanged: changedRules.length,
      totalRulesProcessed: sortedRules.length,
      failedSources: failureRecords.length,
    });

    await updateLastSyncInfo({
      lastPartialSyncAt: new Date().toISOString(),
    });

    await recordRuleFileChanges(ruleFileChanges);
    await recordFailureRecords(failureRecords);

    await completeJob(job.jobId, changedRules, failedRules);

    return {
      success: failedRules.length === 0,
      changedRules,
      failedRules,
      jobId: job.jobId,
    };
  } finally {
    if (options.lockMode === "global") {
      await releaseGlobalSyncLock();
    } else {
      await releaseRuleLock(primaryRuleName);
    }
  }
}

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
  const ruleFileChanges: ChangeRecordInput[] = [];
  const failureRecords: FailureRecord[] = [];
  const pendingArtifactMetas: ArtifactMeta[] = [];
  let blobWriteCount = 0;

  try {
    const config = await getConfig();
    const clients = await getClients();
    const geositeProviders = collectGeositeProviders(config.rules);
    if (geositeProviders.length > 0) {
      const refreshResults = await Promise.allSettled(
        geositeProviders.map(async (provider) => {
          await refreshGeositeProvider(provider);
          return provider;
        })
      );

      const failedProviderRefreshes = refreshResults
        .map((result, index) => ({ result, provider: geositeProviders[index] }))
        .filter(
          (item): item is { result: PromiseRejectedResult; provider: GeositeProvider } =>
            item.result.status === "rejected"
        );

      if (failedProviderRefreshes.length > 0) {
        for (const item of failedProviderRefreshes) {
          const errorMessage = item.result.reason instanceof Error
            ? item.result.reason.message
            : String(item.result.reason);
          failedRules.push({
            name: `geosite:${item.provider}`,
            error: errorMessage,
          });
        }

        if (failureRecords.length > 0) {
          await recordFailureRecords(failureRecords);
        }
        await completeJob(job.jobId, changedRules, failedRules);
        await updateLastSyncInfo({
          lastFullSyncAt: new Date().toISOString(),
          totalRulesCount: config.rules.length,
          changedRulesCount: 0,
          failedRulesCount: failedRules.length,
        });

        return {
          success: false,
          changedRules,
          failedRules,
          jobId: job.jobId,
        };
      }
    }
    const sortedRules = topologicalSort(config.rules);

    const ruleContentsCache = new Map<string, Map<ClientType, string>>();

    for (const rule of sortedRules) {
      const trackActivity = shouldTrackRuleActivity(rule);
      const processResult = await processRule(
        rule,
        config.transformers || {},
        ruleContentsCache,
        clients
      );

      if (processResult.errors.length > 0) {
        if (trackActivity) {
          failureRecords.push(
            ...buildFailureRecords(rule.name, processResult.errors, job.jobId)
          );
        }
        failedRules.push({
          name: rule.name,
          error: processResult.errors.join("; "),
        });
        continue;
      }

      ruleContentsCache.set(rule.name, processResult.contents);

      for (const [client, content] of processResult.contents) {
        const existingMeta = await getArtifactMeta(rule.name, client);
        const normalizedContent = normalizeEffectiveRuleContent(content);
        const hash = await computeHash(normalizedContent);
        const syncedAt = new Date().toISOString();
        const outputContent = addRuleHeader(content, rule.name, rule.description, syncedAt);

        let previousContent: string | null | undefined = undefined;
        if (existingMeta) {
          previousContent = await readRuleContentFromPath(rule, client);
          if (previousContent) {
            const previousSourceContent = stripManagedRuleHeader(previousContent);
            const previousNormalizedHash = await computeHash(
              normalizeEffectiveRuleContent(previousSourceContent)
            );
            if (previousNormalizedHash === hash) {
              if (previousSourceContent === content) {
                if (existingMeta.lastHash !== hash) {
                  pendingArtifactMetas.push({ ...existingMeta, lastHash: hash });
                }
                continue;
              }

              const sizeBytes = new TextEncoder().encode(outputContent).length;
              const { url, path } = await uploadRuleContentToPath(rule, client, outputContent);
              blobWriteCount += 1;
              pendingArtifactMetas.push({
                ...existingMeta,
                lastHash: hash,
                lastUpdatedAt: syncedAt,
                blobPath: path,
                blobUrl: url,
                sizeBytes,
              });
              continue;
            }
          }
        }

        if (previousContent === undefined) {
          previousContent = await readRuleContentFromPath(rule, client);
        }

        const sizeBytes = new TextEncoder().encode(outputContent).length;
        const { url, path } = await uploadRuleContentToPath(rule, client, outputContent);
        blobWriteCount += 1;
        const meta: ArtifactMeta = {
          ruleName: rule.name,
          client,
          lastHash: hash,
          lastUpdatedAt: syncedAt,
          blobPath: path,
          blobUrl: url,
          sizeBytes,
        };
        pendingArtifactMetas.push(meta);

        if (trackActivity) {
          const previousSourceContent = stripManagedRuleHeader(previousContent);
          const diff = createActivityDiff(
            existingMeta ? "updated" : "created",
            normalizeEffectiveRuleContent(previousSourceContent),
            normalizedContent
          );
          const changeRecord: ChangeRecordInput = {
            id: randomUUID(),
            timestamp: meta.lastUpdatedAt,
            ruleName: rule.name,
            client,
            changeType: existingMeta ? "updated" : "created",
            diff,
            sizeBytes,
          };
          ruleFileChanges.push(changeRecord);
        }

        if (!changedRules.includes(rule.name)) {
          changedRules.push(rule.name);
        }
      }
    }

    await saveArtifactMetas(pendingArtifactMetas);

    const today = new Date().toISOString().split("T")[0];
    await incrementDailyStats(today, {
      syncCount: 1,
      blobWriteCount,
      rulesChanged: changedRules.length,
      totalRulesProcessed: sortedRules.length,
      failedSources: failureRecords.length,
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

    await recordRuleFileChanges(ruleFileChanges);
    await recordFailureRecords(failureRecords);

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
export async function executePartialSync(ruleName: string): Promise<SyncExecutionResult> {
  return executeSelectiveSync([ruleName], {
    lockMode: "rule",
    jobType: "partial_sync",
  });
}

export async function executeBatchPartialSync(ruleNames: string[]): Promise<SyncExecutionResult> {
  return executeSelectiveSync(ruleNames, {
    lockMode: "global",
    jobType: "partial_sync",
  });
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

  if (rule.sources) {
    for (const source of rule.sources) {
      if (source.type === "ref" && source.ref) {
        allDependencies.add(source.ref);
      }
    }
  }

  // 获取客户端配置用于全局转换器
  const clients = await getClients();

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
        const result = await processRule(depRule, transformersConfig, ruleContentsCache, clients);
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
        // Get the cached content size for the referenced rule
        const refContents = ruleContentsCache.get(source.ref);
        let refSize = 0;
        if (refContents && refContents.size > 0) {
          // Use the first client's content size as reference
          const firstContent = refContents.values().next().value;
          refSize = firstContent?.length || 0;
        }
        sourceResults.push({
          url: `ref:${source.ref}`,
          success: true,
          size: refSize,
        });
      } else if (sourceType === "local") {
        sourceResults.push({
          url: "local",
          success: true,
          size: source.content?.length || 0,
        });
      } else if (sourceType === "geosite") {
        try {
          const content = await renderGeositeSource(source);
          sourceResults.push({
            url: `geosite:${source.provider}/${source.list}`,
            success: true,
            size: content.length,
          });
        } catch (error) {
          sourceResults.push({
            url: `geosite:${source.provider || "unknown"}/${source.list || "unknown"}`,
            success: false,
            error: String(error),
          });
        }
      }
    }
  }

  const result = await processRule(rule, transformersConfig, ruleContentsCache, clients);

  let truncated = false;
  let totalLines = 0;

  for (const [client, content] of result.contents) {
    const lines = content.split("\n");
    totalLines = Math.max(totalLines, lines.length);

    if (Number.isFinite(limitLines) && lines.length > limitLines) {
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
