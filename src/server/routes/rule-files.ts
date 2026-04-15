import type { Hono } from "hono";
import { getRuleContent, getCdnSettings, buildResponseHeaders } from "../../lib/storage-adapter";
import { ClientType } from "../../lib/schema";

export function registerRuleFileRoutes(app: Hono) {
  app.get("/Rules/:client/:file", async (c) => {
    const clientId = c.req.param("client");
    const file = c.req.param("file");

    if (!file.endsWith(".list")) {
      return c.text("# Invalid file format", 400);
    }

    const ruleName = file.replace(".list", "");
    const content = await getRuleContent(ruleName, clientId as ClientType);

    if (!content) {
      return c.text(
        `# Rule \"${ruleName}\" not found for client \"${clientId}\"\\n# Please run sync first`,
        404
      );
    }

    const cdnSettings = await getCdnSettings();
    const headers = buildResponseHeaders(cdnSettings);

    return c.text(content, 200, headers);
  });
}
