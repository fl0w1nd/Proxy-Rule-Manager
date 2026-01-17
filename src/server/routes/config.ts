import type { Hono } from "hono";
import { getConfig, getConfigRev, saveConfig } from "../../lib/storage-adapter";
import { detectCircularDependency } from "../../lib/sync-engine";
import { validateConfig } from "../../lib/schema";
import { verifyAdmin } from "../auth";
import { jsonError } from "../errors";

export function registerConfigRoutes(app: Hono) {
  app.get("/api/config", async (c) => {
    if (!verifyAdmin(c.req.header("authorization"))) {
      return c.json({ error: "Unauthorized" }, 401);
    }

    try {
      const config = await getConfig();
      const rev = await getConfigRev();
      return c.json({ config, rev });
    } catch (error) {
      console.error("Failed to get config:", error);
      return jsonError(c, error, "Failed to get config");
    }
  });

  app.put("/api/config", async (c) => {
    if (!verifyAdmin(c.req.header("authorization"))) {
      return c.json({ error: "Unauthorized" }, 401);
    }

    try {
      const body = await c.req.json();
      const { config } = body;
      const validated = validateConfig(config);

      const cycle = detectCircularDependency(validated.rules);
      if (cycle) {
        const cycleStr = cycle.join(" → ");
        return c.json({ error: `检测到循环依赖: ${cycleStr}` }, 400);
      }

      const { rev } = await saveConfig(validated);
      const affectedRules = validated.rules.map((r) => r.name);
      return c.json({ success: true, rev, affectedRules });
    } catch (error) {
      console.error("Failed to save config:", error);
      return jsonError(c, error, "Failed to save config", 500, {
        validationMessage: "Invalid config format",
      });
    }
  });
}
