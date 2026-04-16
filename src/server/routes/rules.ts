import type { Hono } from "hono";
import {
  deleteArtifactMeta,
  deleteRuleContent,
  deleteGeositeRuleContent,
  getArtifactMeta,
  getConfig,
  getRuleContent,
  getGeositeRuleContent,
  saveConfig,
  renameRule,
} from "../../lib/storage-adapter";
import { executePartialSync } from "../../lib/sync-engine";
import { recordRuleFileChanges, type ChangeRecordInput } from "../../lib/activity-store";
import { createLineDiff } from "../../lib/diff";
import { randomUUID } from "node:crypto";
import { jsonError } from "../errors";
import { isGeositeRule, getGeositeOutputName, getPrimaryGeositeSource } from "../../lib/rule-classification";

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

      for (const client of clients) {
        const isGeosite = isGeositeRule(rule);
        const geositeSource = isGeosite ? getPrimaryGeositeSource(rule) : undefined;
        const geositeOutputName = geositeSource ? getGeositeOutputName(geositeSource) : undefined;

        const previousContent = isGeosite && geositeSource
          ? await getGeositeRuleContent(client, geositeSource.provider!, geositeOutputName!)
          : await getRuleContent(ruleName, client);
        const meta = await getArtifactMeta(ruleName, client);

        if (isGeosite && geositeSource) {
          await deleteGeositeRuleContent(client, geositeSource.provider!, geositeOutputName!);
        } else {
          await deleteRuleContent(ruleName, client);
        }
        await deleteArtifactMeta(ruleName, client);

        if (previousContent || meta) {
          const diff = createLineDiff(previousContent, "");
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
      const ruleNames: string[] = body.ruleNames;

      if (!Array.isArray(ruleNames) || ruleNames.length === 0) {
        return c.json({ error: "ruleNames must be a non-empty array" }, 400);
      }

      const config = await getConfig();
      const namesToDelete = new Set(ruleNames);
      const results: { deleted: string[]; notFound: string[]; blocked: { name: string; dependents: string[] }[] } = {
        deleted: [],
        notFound: [],
        blocked: [],
      };

      // Check all rules first
      for (const ruleName of ruleNames) {
        const ruleIndex = config.rules.findIndex((r) => r.name === ruleName);
        if (ruleIndex === -1) {
          results.notFound.push(ruleName);
          continue;
        }

        // Check for dependents outside the batch
        const dependentRules: string[] = [];
        for (const r of config.rules) {
          if (namesToDelete.has(r.name)) continue;
          if (r.sources?.some((s) => s.type === "ref" && s.ref === ruleName)) {
            dependentRules.push(r.name);
          }
        }

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

      // Perform deletions
      const changeRecords: ChangeRecordInput[] = [];

      for (const ruleName of ruleNames) {
        const ruleIndex = config.rules.findIndex((r) => r.name === ruleName);
        if (ruleIndex === -1) continue;

        const rule = config.rules[ruleIndex];
        const clients = rule.output.clients;

        for (const client of clients) {
          const isGeosite = isGeositeRule(rule);
          const geositeSource = isGeosite ? getPrimaryGeositeSource(rule) : undefined;
          const geositeOutputName = geositeSource ? getGeositeOutputName(geositeSource) : undefined;

          const previousContent = isGeosite && geositeSource
            ? await getGeositeRuleContent(client, geositeSource.provider!, geositeOutputName!)
            : await getRuleContent(ruleName, client);

          if (isGeosite && geositeSource) {
            await deleteGeositeRuleContent(client, geositeSource.provider!, geositeOutputName!);
          } else {
            await deleteRuleContent(ruleName, client);
          }
          await deleteArtifactMeta(ruleName, client);

          if (previousContent) {
            const diff = createLineDiff(previousContent, "");
            const sizeBytes = new TextEncoder().encode(previousContent).length;
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
        results.deleted.push(ruleName);
      }

      await saveConfig(config);
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
