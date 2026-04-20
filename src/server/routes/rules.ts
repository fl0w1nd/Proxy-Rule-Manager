import type { Hono } from "hono";
import {
  deleteArtifactMeta,
  deleteArtifactMetas,
  deleteRuleContent,
  deleteGeositeRuleContent,
  getAllArtifactMetas,
  getArtifactMeta,
  getConfig,
  getRuleContent,
  getGeositeRuleContent,
  saveConfig,
  renameRule,
} from "../../lib/storage-adapter";
import { executePartialSync } from "../../lib/sync-engine";
import { saveLocalSourceContent } from "../../lib/local-source-store";
import { recordRuleFileChanges, type ChangeRecordInput } from "../../lib/activity-store";
import { createActivityDiff } from "../../lib/diff";
import { randomUUID } from "node:crypto";
import { jsonError } from "../errors";
import { isGeositeRule, getGeositeOutputName, getPrimaryGeositeSource } from "../../lib/rule-classification";
import type { ClientType, RuleConfig } from "../../lib/schema";

function artifactMetaKey(ruleName: string, client: string): string {
  return `${ruleName}:${client}`;
}

function buildDependentRuleIndex(rules: Array<{ name: string; sources?: Array<{ type?: string; ref?: string }> }>): Map<string, string[]> {
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

export function registerRuleRoutes(app: Hono) {
  app.delete("/api/rules/:ruleName", async (c) => {
    try {
      const ruleName = decodeURIComponent(c.req.param("ruleName"));
      const config = await getConfig();

      const ruleIndex = config.rules.findIndex((r) => r.name === ruleName);
      if (ruleIndex === -1) {
        return c.json({ error: "Rule not found" }, 404);
      }

      const dependentRules: string[] = [];
      for (const r of config.rules) {
        if (r.name === ruleName) continue;
        if (r.sources?.some((s) => s.type === "ref" && s.ref === ruleName)) {
          dependentRules.push(r.name);
        }
      }

      if (dependentRules.length > 0) {
        return c.json(
          {
            error: `无法删除规则 \"${ruleName}\"，它被以下规则引用: ${dependentRules.join(", ")}`,
            dependentRules,
          },
          400
        );
      }

      const rule = config.rules[ruleIndex];
      const clients = rule.output.clients;
      const changeRecords: ChangeRecordInput[] = [];
      const trackActivity = !isGeositeRule(rule);

      for (const client of clients) {
        const isGeosite = isGeositeRule(rule);
        const geositeSource = isGeosite ? getPrimaryGeositeSource(rule) : undefined;
        const geositeOutputName = geositeSource ? getGeositeOutputName(geositeSource) : undefined;

        const previousContent = trackActivity
          ? (isGeosite && geositeSource
            ? await getGeositeRuleContent(client, geositeSource.provider!, geositeOutputName!)
            : await getRuleContent(ruleName, client))
          : null;
        const meta = trackActivity ? await getArtifactMeta(ruleName, client) : null;

        if (isGeosite && geositeSource) {
          await deleteGeositeRuleContent(client, geositeSource.provider!, geositeOutputName!);
        } else {
          await deleteRuleContent(ruleName, client);
        }
        await deleteArtifactMeta(ruleName, client);

        if (trackActivity && (previousContent || meta)) {
          const diff = createActivityDiff("deleted", previousContent, "");
          const sizeBytes = previousContent
            ? new TextEncoder().encode(previousContent).length
            : undefined;
          changeRecords.push({
            id: randomUUID(),
            timestamp: new Date().toISOString(),
            ruleName,
            client,
            changeType: "deleted",
            diff,
            sizeBytes,
          });
        }
      }

      config.rules.splice(ruleIndex, 1);
      await saveConfig(config);
      await recordRuleFileChanges(changeRecords);

      return c.json({ success: true, deletedRule: ruleName, deletedClients: clients });
    } catch (error) {
      console.error("Failed to delete rule:", error);
      return jsonError(c, error, "Failed to delete rule");
    }
  });

  app.post("/api/rules/batch-delete", async (c) => {
    try {
      const body = await c.req.json();
      const requestedRuleNames: string[] = Array.isArray(body.ruleNames)
        ? Array.from(new Set(body.ruleNames.filter((item: unknown): item is string => typeof item === "string" && item.trim().length > 0)))
        : [];

      if (requestedRuleNames.length === 0) {
        return c.json({ error: "ruleNames must be a non-empty array" }, 400);
      }

      const config = await getConfig();
      const namesToDelete = new Set(requestedRuleNames);
      const ruleByName = new Map<string, RuleConfig>(config.rules.map((rule) => [rule.name, rule]));
      const dependentsByRule = buildDependentRuleIndex(config.rules);
      const results: { deleted: string[]; notFound: string[]; blocked: { name: string; dependents: string[] }[] } = {
        deleted: [],
        notFound: [],
        blocked: [],
      };

      for (const ruleName of requestedRuleNames) {
        const rule = ruleByName.get(ruleName);
        if (!rule) {
          results.notFound.push(ruleName);
          continue;
        }

        const dependentRules = (dependentsByRule.get(rule.name) || []).filter((dependentName) => !namesToDelete.has(dependentName));

        if (dependentRules.length > 0) {
          results.blocked.push({ name: ruleName, dependents: dependentRules });
        }
      }

      if (results.blocked.length > 0) {
        return c.json(
          {
            error: `${results.blocked.length} rules cannot be deleted due to external dependencies`,
            ...results,
          },
          400
        );
      }

      const targetRules = requestedRuleNames
        .map((ruleName) => ruleByName.get(ruleName))
        .filter((rule): rule is RuleConfig => Boolean(rule));
      const artifactMetas = await getAllArtifactMetas();
      const artifactMetaByKey = new Map(
        artifactMetas.map((meta) => [artifactMetaKey(meta.ruleName, meta.client), meta])
      );
      const changeRecords: ChangeRecordInput[] = [];
      const artifactEntriesToDelete: Array<{ ruleName: string; client: ClientType }> = [];

      await Promise.all(
        targetRules.flatMap((rule) => {
          const isGeosite = isGeositeRule(rule);
          const trackActivity = !isGeosite;
          const geositeSource = isGeosite ? getPrimaryGeositeSource(rule) : undefined;
          const geositeOutputName = geositeSource ? getGeositeOutputName(geositeSource) : undefined;

          return rule.output.clients.map(async (client) => {
            const meta = trackActivity ? artifactMetaByKey.get(artifactMetaKey(rule.name, client)) : null;
            const previousContent = trackActivity
              ? (isGeosite && geositeSource
                ? await getGeositeRuleContent(client, geositeSource.provider!, geositeOutputName!)
                : await getRuleContent(rule.name, client))
              : null;

            if (isGeosite && geositeSource) {
              await deleteGeositeRuleContent(client, geositeSource.provider!, geositeOutputName!);
            } else {
              await deleteRuleContent(rule.name, client);
            }

            artifactEntriesToDelete.push({ ruleName: rule.name, client });

            if (trackActivity && (previousContent || meta)) {
              const diff = createActivityDiff("deleted", previousContent, "");
              const sizeBytes = previousContent
                ? new TextEncoder().encode(previousContent).length
                : undefined;
              changeRecords.push({
                id: randomUUID(),
                timestamp: new Date().toISOString(),
                ruleName: rule.name,
                client,
                changeType: "deleted",
                diff,
                sizeBytes,
              });
            }
          });
        })
      );

      for (const rule of targetRules) {
        const clients = rule.output.clients;
        void clients;
        results.deleted.push(rule.name);
      }

      config.rules = config.rules.filter((rule) => !namesToDelete.has(rule.name));
      await saveConfig(config);
      await deleteArtifactMetas(artifactEntriesToDelete);
      await recordRuleFileChanges(changeRecords);

      return c.json({ success: true, ...results });
    } catch (error) {
      console.error("Failed to batch delete rules:", error);
      return jsonError(c, error, "Failed to batch delete rules");
    }
  });

  app.post("/api/rules/:ruleName/refresh", async (c) => {
    try {
      const ruleName = decodeURIComponent(c.req.param("ruleName"));
      const result = await executePartialSync(ruleName);
      return c.json(result);
    } catch (error) {
      console.error("Failed to refresh rule:", error);
      return jsonError(c, error, "Failed to refresh rule");
    }
  });

  app.get("/api/rules/local-sources", async (c) => {
    try {
      const config = await getConfig();
      const rules = config.rules
        .map((rule) => {
          const localSources = (rule.sources || [])
            .map((source, index) => ({ source, index }))
            .filter(({ source }) => source.type === "local")
            .map(({ source, index }) => ({
              sourceIndex: index,
              name: source.name || null,
              contentRef: source.contentRef || null,
            }));

          if (localSources.length === 0) return null;
          return { ruleName: rule.name, sources: localSources };
        })
        .filter(Boolean);

      return c.json({ rules });
    } catch (error) {
      console.error("Failed to list local sources:", error);
      return jsonError(c, error, "Failed to list local sources");
    }
  });

  app.put("/api/rules/:ruleName/local-source", async (c) => {
    try {
      const ruleName = decodeURIComponent(c.req.param("ruleName"));
      const body = await c.req.json();
      const { sourceIndex, content } = body;

      if (typeof sourceIndex !== "number" || sourceIndex < 0 || !Number.isInteger(sourceIndex)) {
        return c.json({ error: "sourceIndex must be a non-negative integer" }, 400);
      }
      if (typeof content !== "string") {
        return c.json({ error: "content must be a string" }, 400);
      }

      const config = await getConfig();
      const rule = config.rules.find((r) => r.name === ruleName);
      if (!rule) {
        return c.json({ error: "Rule not found" }, 404);
      }

      const sources = rule.sources || [];
      if (sourceIndex >= sources.length) {
        return c.json({ error: `sourceIndex ${sourceIndex} out of range (rule has ${sources.length} sources)` }, 404);
      }

      const source = sources[sourceIndex];
      if (source.type !== "local") {
        return c.json({ error: `Source at index ${sourceIndex} is not a local source (type: ${source.type})` }, 404);
      }

      const contentRef = await saveLocalSourceContent(source.contentRef, content);

      const syncResult = await executePartialSync(ruleName);

      return c.json({
        success: true,
        ruleName,
        sourceIndex,
        contentRef,
        sync: syncResult,
      });
    } catch (error) {
      console.error("Failed to update local source:", error);
      return jsonError(c, error, "Failed to update local source");
    }
  });

  app.put("/api/rules/:ruleName", async (c) => {
    try {
      const oldName = decodeURIComponent(c.req.param("ruleName"));
      const body = await c.req.json();
      const { newName } = body;

      const config = await getConfig();
      const existingRule = config.rules.find((rule) => rule.name === oldName);
      if (!existingRule) {
        return c.json({ error: "Rule not found" }, 404);
      }
      if (isGeositeRule(existingRule)) {
        return c.json({ error: `Rule "${oldName}" is system-managed and cannot be renamed` }, 400);
      }

      if (!newName || typeof newName !== "string") {
        return c.json({ error: "newName is required" }, 400);
      }

      const result = await renameRule(oldName, newName);
      return c.json({
        success: true,
        oldName,
        newName,
        renamedFiles: result.renamedFiles,
      });
    } catch (error) {
      console.error("Failed to rename rule:", error);
      const message = error instanceof Error ? error.message : "Failed to rename rule";
      return jsonError(c, error, message);
    }
  });
}
