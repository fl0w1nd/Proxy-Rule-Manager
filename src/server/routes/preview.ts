import type { Hono } from "hono";
import { previewRule } from "../../lib/sync-engine";
import { validateRule, ClientType } from "../../lib/schema";
import { getConfig } from "../../lib/storage-adapter";
import { verifyAdmin } from "../auth";
import { jsonError } from "../errors";

export function registerPreviewRoutes(app: Hono) {
  app.post("/api/preview", async (c) => {
    if (!verifyAdmin(c.req.header("authorization"))) {
      return c.json({ error: "Unauthorized" }, 401);
    }

    try {
      const body = await c.req.json();
      const { ruleName, rule: ruleConfig, limitLines = 2000 } = body;

      let ruleToPreview;
      let transformersConfig = {};

      if (ruleName) {
        const config = await getConfig();
        const existingRule = config.rules.find((r) => r.name === ruleName);
        if (!existingRule) {
          return c.json({ error: `Rule \"${ruleName}\" not found` }, 404);
        }
        ruleToPreview = existingRule;
        transformersConfig = config.transformers || {};
      } else if (ruleConfig) {
        ruleToPreview = validateRule(ruleConfig);
        const config = await getConfig();
        transformersConfig = config.transformers || {};
      } else {
        return c.json({ error: "Either ruleName or rule config is required" }, 400);
      }

      const result = await previewRule(ruleToPreview, transformersConfig, limitLines);

      const contentsObj: Record<ClientType, string> = {} as Record<ClientType, string>;
      for (const [client, content] of result.contents) {
        contentsObj[client] = content;
      }

      return c.json({ contents: contentsObj, diagnostics: result.diagnostics });
    } catch (error) {
      console.error("Failed to preview rule:", error);
      return jsonError(c, error, "Failed to preview rule", 500, {
        validationMessage: "Invalid rule format",
      });
    }
  });
}
