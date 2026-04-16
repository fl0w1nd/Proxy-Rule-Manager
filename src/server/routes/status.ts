import type { Hono } from "hono";
import {
  getAllArtifactMetas,
  getConfigRaw,
  getDailyStats,
  getLastSyncInfo,
  getClients,
} from "../../lib/storage-adapter";
import { RulesConfig } from "../../lib/schema";
import packageJson from "../../../package.json";
import { countChangeRecords, countChangeRecordsByCategory, countFailureRecords } from "../../lib/activity-store";
import { checkAuth, getClientIp, verifyAdminWithRateLimit } from "../auth";
import { jsonError } from "../errors";
import { getGeositeOutputName, getPrimaryGeositeSource, isGeositeRule } from "../../lib/rule-classification";

function countRuleFiles(config: RulesConfig, filter?: (rule: RulesConfig["rules"][number]) => boolean): number {
  let count = 0;
  for (const rule of config.rules) {
    if (filter && !filter(rule)) continue;
    count += rule.output.clients.length;
  }
  return count;
}

function countRules(config: RulesConfig, filter?: (rule: RulesConfig["rules"][number]) => boolean): number {
  if (!filter) return config.rules.length;
  return config.rules.filter(filter).length;
}

export function registerStatusRoutes(app: Hono) {
  app.get("/api/status", async (c) => {
    const ip = getClientIp((name) => c.req.header(name));
    const authHeader = c.req.header("authorization");

    // 如果提供了 token，使用带 Rate Limit 的验证
    if (authHeader) {
      const result = await verifyAdminWithRateLimit(authHeader, ip);
      if (!result.success) {
        if (result.error === "blocked") {
          c.header("Retry-After", String(result.retryAfter || 60));
          return c.json(
            {
              error: "Too many failed attempts",
              retryAfter: result.retryAfter,
            },
            429
          );
        }
        return c.json({ error: "Unauthorized" }, 401);
      }
    }

    const authResult = checkAuth(authHeader);
    const isAdmin = authResult === "admin";

    try {
      const config = await getConfigRaw();
      const artifactMetas = await getAllArtifactMetas();
      const ruleFilesCount = countRuleFiles(config, (r) => !isGeositeRule(r));
      const geositeRuleFilesCount = countRuleFiles(config, isGeositeRule);
      const rulesCount = countRules(config, (r) => !isGeositeRule(r));
      const geositeRulesCount = countRules(config, isGeositeRule);

      const rulesStatus = config.rules
        .filter((rule) => !isGeositeRule(rule))
        .map((rule) => {
          const metas = artifactMetas.filter((m) => m.ruleName === rule.name);
          return {
            name: rule.name,
            displayName: rule.displayName,
            description: rule.description,
            icon: rule.icon,
            tags: rule.tags || [],
            clients: rule.output.clients,
            lastUpdated:
              metas.length > 0
                ? metas.reduce((latest, m) => {
                  return new Date(m.lastUpdatedAt) > new Date(latest)
                    ? m.lastUpdatedAt
                    : latest;
                }, metas[0].lastUpdatedAt)
                : null,
            hasError: false,
          };
        });

      const geositeRules = config.rules
        .filter((rule) => isGeositeRule(rule))
        .map((rule) => {
          const metas = artifactMetas.filter((m) => m.ruleName === rule.name);
          const source = getPrimaryGeositeSource(rule)!;
          return {
            name: source.list || rule.name,
            displayName: rule.displayName,
            description: rule.description,
            icon: rule.icon,
            tags: rule.tags || [],
            clients: rule.output.clients,
            provider: source.provider || "v2fly",
            list: source.list || rule.name,
            attrs: source.attrs || [],
            outputName: getGeositeOutputName(source),
            lastUpdated:
              metas.length > 0
                ? metas.reduce((latest, m) => {
                  return new Date(m.lastUpdatedAt) > new Date(latest)
                    ? m.lastUpdatedAt
                    : latest;
                }, metas[0].lastUpdatedAt)
                : null,
          };
        });

      if (!isAdmin) {
        const lastSyncInfo = await getLastSyncInfo();
        const clientsConfig = await getClients();
        const publicClients = clientsConfig.map((c) => ({
          id: c.id,
          displayName: c.displayName,
        }));
        return c.json({
          rulesCount,
          geositeRulesCount,
          lastSyncAt: lastSyncInfo.lastSuccessfulSyncAt || lastSyncInfo.lastFullSyncAt,
          rules: rulesStatus,
          geositeRules,
          clients: publicClients,
          version: packageJson.version,
        });
      }

      const lastSyncInfo = await getLastSyncInfo();
      const today = new Date().toISOString().split("T")[0];
      const todayStats = await getDailyStats(today);
      const todayChangeCount = await countChangeRecords(today);
      const { createdRecords, updatedRecords } = await countChangeRecordsByCategory(today);
      const todayFailureCount = await countFailureRecords(today);

      const clientsConfig = await getClients();
      const clientsList = clientsConfig.map((c) => ({
        id: c.id,
        displayName: c.displayName,
      }));

      return c.json({
        rulesCount,
        geositeRulesCount,
        ruleFilesCount,
        geositeRuleFilesCount,
        lastSync: lastSyncInfo,
        needsInit: config.rules.length === 0,
        todayStats: {
          ...todayStats,
          ruleFilesChanged: todayChangeCount,
          createdRecords,
          updatedRecords,
          failureRecords: todayFailureCount,
        },
        rules: rulesStatus,
        geositeRules,
        clients: clientsList,
        version: packageJson.version,
      });
    } catch (error) {
      console.error("Failed to get status:", error);
      return jsonError(c, error, "Failed to get status");
    }
  });
}
