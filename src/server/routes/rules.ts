import type { Hono } from "hono";
import {
  deleteArtifactMeta,
  deleteRuleContent,
  getConfig,
  saveConfig,
  renameRule,
} from "../../lib/storage-adapter";
import { executePartialSync } from "../../lib/sync-engine";
import { verifyAdmin } from "../auth";
import { jsonError } from "../errors";

export function registerRuleRoutes(app: Hono) {
  app.delete("/api/rules/:ruleName", async (c) => {
    if (!verifyAdmin(c.req.header("authorization"))) {
      return c.json({ error: "Unauthorized" }, 401);
    }

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
        if (r.compose_from?.includes(ruleName)) {
          dependentRules.push(r.name);
          continue;
        }
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

      for (const client of clients) {
        await deleteRuleContent(ruleName, client);
        await deleteArtifactMeta(ruleName, client);
      }

      config.rules.splice(ruleIndex, 1);
      await saveConfig(config);

      return c.json({ success: true, deletedRule: ruleName, deletedClients: clients });
    } catch (error) {
      console.error("Failed to delete rule:", error);
      return jsonError(c, error, "Failed to delete rule");
    }
  });

  app.post("/api/rules/:ruleName/refresh", async (c) => {
    if (!verifyAdmin(c.req.header("authorization"))) {
      return c.json({ error: "Unauthorized" }, 401);
    }

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
    if (!verifyAdmin(c.req.header("authorization"))) {
      return c.json({ error: "Unauthorized" }, 401);
    }

    try {
      const oldName = decodeURIComponent(c.req.param("ruleName"));
      const body = await c.req.json();
      const { newName } = body;

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
