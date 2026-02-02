import type { Hono } from "hono";
import { getCdnSettings, updateCdnSettings } from "../../lib/storage-adapter";
import { CdnSettingsSchema } from "../../lib/schema";
import { verifyAdmin } from "../auth";
import { jsonError } from "../errors";

export function registerCdnSettingsRoutes(app: Hono) {
  app.get("/api/cdn-settings", async (c) => {
    if (!verifyAdmin(c.req.header("authorization"))) {
      return c.json({ error: "Unauthorized" }, 401);
    }

    try {
      const settings = await getCdnSettings();
      return c.json({ settings });
    } catch (error) {
      console.error("Failed to get CDN settings:", error);
      return jsonError(c, error, "Failed to get CDN settings");
    }
  });

  app.put("/api/cdn-settings", async (c) => {
    if (!verifyAdmin(c.req.header("authorization"))) {
      return c.json({ error: "Unauthorized" }, 401);
    }

    try {
      const body = await c.req.json();
      const validated = CdnSettingsSchema.partial().parse(body);
      const updated = await updateCdnSettings(validated);
      return c.json({ success: true, settings: updated });
    } catch (error) {
      console.error("Failed to update CDN settings:", error);
      return jsonError(c, error, "Failed to update CDN settings");
    }
  });
}
