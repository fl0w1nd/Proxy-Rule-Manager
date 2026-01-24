import type { Hono } from "hono";
import {
  getAllArtifactMetas,
  getConfigRaw,
  getDailyStats,
  getLastSyncInfo,
  getClients,
} from "../../lib/storage-adapter";
import { RulesConfig } from "../../lib/schema";
import { countChangeRecords, countFailureRecords } from "../../lib/activity-store";
import { checkAuth, getClientIp, verifyAdminWithRateLimit } from "../auth";
import { jsonError } from "../errors";

function countRuleFiles(config: RulesConfig): number {
  let count = 0;
  for (const rule of config.rules) {
    count += rule.output.clients.length;
  }
  return count;
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
      const ruleFilesCount = countRuleFiles(config);

      const rulesStatus = config.rules.map((rule) => {
        const metas = artifactMetas.filter((m) => m.ruleName === rule.name);
        return {
          name: rule.name,
          displayName: rule.displayName,
          description: rule.description,
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

      if (!isAdmin) {
        const lastSyncInfo = await getLastSyncInfo();
        const clientsConfig = await getClients();
        const publicClients = clientsConfig.map((c) => ({
          id: c.id,
          displayName: c.displayName,
          pathName: c.pathName,
        }));
        return c.json({
          rulesCount: config.rules.length,
          lastSyncAt: lastSyncInfo.lastSuccessfulSyncAt || lastSyncInfo.lastFullSyncAt,
          rules: rulesStatus,
          clients: publicClients,
        });
      }

      const lastSyncInfo = await getLastSyncInfo();
      const today = new Date().toISOString().split("T")[0];
      const todayStats = await getDailyStats(today);
      const todayChangeCount = await countChangeRecords(today);
      const todayFailureCount = await countFailureRecords(today);

      const clientsConfig = await getClients();
      const clientsList = clientsConfig.map((c) => ({
        id: c.id,
        displayName: c.displayName,
        pathName: c.pathName,
      }));

      return c.json({
        rulesCount: config.rules.length,
        ruleFilesCount,
        lastSync: lastSyncInfo,
        needsInit: config.rules.length === 0,
        todayStats: {
          ...todayStats,
          ruleFilesChanged: todayChangeCount,
          failureRecords: todayFailureCount,
        },
        rules: rulesStatus,
        clients: clientsList,
      });
    } catch (error) {
      console.error("Failed to get status:", error);
      return jsonError(c, error, "Failed to get status");
    }
  });
}
