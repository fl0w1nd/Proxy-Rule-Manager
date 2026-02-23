import type { Hono } from "hono";
import { executeFullSync } from "../../lib/sync-engine";
import { getSyncSchedule, updateSyncSchedule } from "../../lib/storage-adapter";
import { getNextSyncAt, validateCronExpression } from "../../lib/sync-schedule";
import { jsonError } from "../errors";

export function registerSyncRoutes(app: Hono) {
  app.post("/api/sync/full", async (c) => {
    try {
      const result = await executeFullSync();
      return c.json(result);
    } catch (error) {
      console.error("Failed to execute full sync:", error);
      return jsonError(c, error, "Failed to execute full sync");
    }
  });

  app.get("/api/sync/schedule", async (c) => {
    try {
      const schedule = await getSyncSchedule();
      return c.json({ schedule });
    } catch (error) {
      console.error("Failed to get sync schedule:", error);
      return jsonError(c, error, "Failed to get sync schedule");
    }
  });

  app.put("/api/sync/schedule", async (c) => {
    try {
      const body = await c.req.json();
      const { mode, intervalHours, cronExpression } = body;

      const resolvedMode = mode === "cron" ? "cron" : "interval";
      if (resolvedMode === "interval") {
        if (typeof intervalHours !== "number" || intervalHours < 1) {
          return c.json({ error: "intervalHours must be a number >= 1" }, 400);
        }
      } else {
        if (typeof cronExpression !== "string" || !cronExpression.trim()) {
          return c.json({ error: "cronExpression must be a non-empty string" }, 400);
        }
        try {
          validateCronExpression(cronExpression.trim());
        } catch (error) {
          return c.json({ error: "Invalid cron expression", detail: String(error) }, 400);
        }
      }

      await updateSyncSchedule({
        mode: resolvedMode,
        intervalHours: resolvedMode === "interval" ? intervalHours : undefined,
        cronExpression: resolvedMode === "cron" ? cronExpression.trim() : undefined,
      });

      const currentSchedule = await getSyncSchedule();
      const nextSyncAt = getNextSyncAt(currentSchedule, new Date());
      await updateSyncSchedule({ nextSyncAt });
      const schedule = await getSyncSchedule();
      return c.json({ success: true, schedule });
    } catch (error) {
      console.error("Failed to update sync schedule:", error);
      return jsonError(c, error, "Failed to update sync schedule");
    }
  });
}
