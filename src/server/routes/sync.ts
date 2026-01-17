import type { Hono } from "hono";
import { executeFullSync } from "../../lib/sync-engine";
import { getSyncSchedule, updateSyncSchedule } from "../../lib/storage-adapter";
import { verifyAdmin } from "../auth";
import { jsonError } from "../errors";

export function registerSyncRoutes(app: Hono) {
  app.post("/api/sync/full", async (c) => {
    if (!verifyAdmin(c.req.header("authorization"))) {
      return c.json({ error: "Unauthorized" }, 401);
    }

    try {
      const result = await executeFullSync();
      return c.json(result);
    } catch (error) {
      console.error("Failed to execute full sync:", error);
      return jsonError(c, error, "Failed to execute full sync");
    }
  });

  app.get("/api/sync/schedule", async (c) => {
    if (!verifyAdmin(c.req.header("authorization"))) {
      return c.json({ error: "Unauthorized" }, 401);
    }

    try {
      const schedule = await getSyncSchedule();
      return c.json({ schedule });
    } catch (error) {
      console.error("Failed to get sync schedule:", error);
      return jsonError(c, error, "Failed to get sync schedule");
    }
  });

  app.put("/api/sync/schedule", async (c) => {
    if (!verifyAdmin(c.req.header("authorization"))) {
      return c.json({ error: "Unauthorized" }, 401);
    }

    try {
      const body = await c.req.json();
      const { intervalHours } = body;

      if (typeof intervalHours !== "number" || intervalHours < 1) {
        return c.json({ error: "intervalHours must be a number >= 1" }, 400);
      }

      await updateSyncSchedule({ intervalHours });
      const currentSchedule = await getSyncSchedule();
      const lastSync = currentSchedule.lastScheduledSyncAt
        ? new Date(currentSchedule.lastScheduledSyncAt)
        : new Date();
      const nextSyncAt = new Date(
        lastSync.getTime() + intervalHours * 60 * 60 * 1000
      ).toISOString();
      await updateSyncSchedule({ nextSyncAt });
      const schedule = await getSyncSchedule();
      return c.json({ success: true, schedule });
    } catch (error) {
      console.error("Failed to update sync schedule:", error);
      return jsonError(c, error, "Failed to update sync schedule");
    }
  });
}
