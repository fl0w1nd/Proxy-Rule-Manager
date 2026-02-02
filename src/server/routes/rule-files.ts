import type { Hono } from "hono";
import { getRuleContent, getCdnSettings, buildResponseHeaders } from "../../lib/storage-adapter";
import { CLIENT_PATH_NAMES, ClientType } from "../../lib/schema";

export function registerRuleFileRoutes(app: Hono) {
  app.get("/Rules/:client/:file", async (c) => {
    const clientPath = c.req.param("client");
    const file = c.req.param("file");

    if (!file.endsWith(".list")) {
      return c.text("# Invalid file format", 400);
    }

    const ruleName = file.replace(".list", "");

    const client = Object.entries(CLIENT_PATH_NAMES).find(([, name]) => name === clientPath)
      ?.[0] as ClientType | undefined;

    if (!client) {
      return c.text(`# Unknown client: ${clientPath}`, 404);
    }

    const content = await getRuleContent(ruleName, client);

    if (!content) {
      return c.text(
        `# Rule \"${ruleName}\" not found for client \"${clientPath}\"\\n# Please run sync first`,
        404
      );
    }

    const cdnSettings = await getCdnSettings();
    const headers = buildResponseHeaders(cdnSettings);

    return c.text(content, 200, headers);
  });
}
