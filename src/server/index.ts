/**
 * Hono Backend Server
 * Lightweight replacement for Next.js API routes
 */

import { Hono } from "hono";
import { cors } from "hono/cors";
import { serveStatic } from "@hono/node-server/serve-static";
import { serve } from "@hono/node-server";
import { promises as fs } from "node:fs";
import * as path from "node:path";

import { executeFullSync } from "../lib/sync-engine";
import { getConfig, getSyncSchedule, updateSyncSchedule } from "../lib/storage-adapter";
import { getNextSyncAt } from "../lib/sync-schedule";
import { jsonError } from "./errors";
import { adminAuth } from "./middleware/admin-auth";
import { registerClientRoutes } from "./routes/clients";
import { registerClientFileRoutes } from "./routes/client-files";
import { registerAuthRoutes } from "./routes/auth";
import { registerConfigRoutes } from "./routes/config";
import { registerInitRoutes } from "./routes/init";
import { registerPreviewRoutes } from "./routes/preview";
import { registerRuleRoutes } from "./routes/rules";
import { registerRuleFileRoutes } from "./routes/rule-files";
import { registerStatusRoutes } from "./routes/status";
import { registerSyncRoutes } from "./routes/sync";
import { registerActivityRoutes } from "./routes/activity";
import { registerWafRoutes } from "./routes/waf";
import { registerCdnSettingsRoutes } from "./routes/cdn-settings";
import { registerIconSetRoutes } from "./routes/iconset";
import { registerGeositeRoutes } from "./routes/geosite";

const app = new Hono();

// --- Middleware ---
app.use("*", cors());

// Admin auth with rate limiting – skip public API paths
const PUBLIC_API_PATHS = new Set([
  "/api/auth/required",
  "/api/status",
  "/api/client-files/public",
  "/api/waf/my-ip",
  "/api/iconset",
]);

app.use("/api/*", async (c, next) => {
  if (PUBLIC_API_PATHS.has(c.req.path)) {
    return next();
  }
  return adminAuth(c, next);
});

// --- API Routes ---
registerConfigRoutes(app);
registerStatusRoutes(app);
registerActivityRoutes(app);
registerSyncRoutes(app);
registerRuleRoutes(app);
registerClientRoutes(app);
registerAuthRoutes(app);
registerPreviewRoutes(app);
registerInitRoutes(app);
registerWafRoutes(app);
registerCdnSettingsRoutes(app);
registerGeositeRoutes(app);

// --- IconSet Routes (before client files to avoid /:clientId/:file conflict) ---
registerIconSetRoutes(app);

registerClientFileRoutes(app);

// --- Rule File Serving (Real-time, no cache) ---
registerRuleFileRoutes(app);

// --- Static File Serving ---
app.use("/*", serveStatic({ root: "./out" }));

// Fallback for SPA routing
app.get("*", async (c) => {
  try {
    const indexPath = path.join(process.cwd(), "out", "index.html");
    const html = await fs.readFile(indexPath, "utf-8");
    return c.html(html);
  } catch {
    return c.text("Not Found", 404);
  }
});

app.onError((err, c) => {
  console.error("Unhandled error:", err);
  return jsonError(c, err, "Internal server error");
});

// --- Server Start ---
const port = parseInt(process.env.PORT || "3000", 10);

// --- Scheduled Sync Timer ---
let scheduledSyncTimer: NodeJS.Timeout | null = null;

async function checkAndExecuteScheduledSync() {
  try {
    const config = await getConfig();
    if (config.rules.length === 0) {
      return;
    }

    const schedule = await getSyncSchedule();
    const now = new Date();

    let needsSync = false;
    let nextSyncAt = schedule.nextSyncAt ? new Date(schedule.nextSyncAt) : null;
    const hasValidNextSyncAt = nextSyncAt && !Number.isNaN(nextSyncAt.getTime());

    if (!hasValidNextSyncAt && !schedule.lastScheduledSyncAt) {
      if (schedule.mode === "cron") {
        const initialNextSyncAt = getNextSyncAt(schedule, now);
        await updateSyncSchedule({ nextSyncAt: initialNextSyncAt });
        return;
      }

      needsSync = true;
    } else {
      if (!hasValidNextSyncAt) {
        const baseDate = schedule.lastScheduledSyncAt
          ? new Date(schedule.lastScheduledSyncAt)
          : now;
        nextSyncAt = new Date(getNextSyncAt(schedule, baseDate));
        await updateSyncSchedule({ nextSyncAt: nextSyncAt.toISOString() });
      }

      if (nextSyncAt && now >= nextSyncAt) {
        needsSync = true;
      }
    }

    if (needsSync) {
      console.log(`[Scheduled Sync] Starting scheduled sync at ${now.toISOString()}...`);

      const result = await executeFullSync();

      const nextSyncAt = getNextSyncAt(schedule, now);
      await updateSyncSchedule({
        lastScheduledSyncAt: now.toISOString(),
        nextSyncAt,
      });

      if (result.success) {
        console.log(`[Scheduled Sync] Completed! Changed rules: ${result.changedRules.length}`);
      } else {
        console.log(`[Scheduled Sync] Completed with errors. Failed: ${result.failedRules.length}`);
      }
    }
  } catch (error) {
    console.error("[Scheduled Sync] Error:", error);
  }
}

function startScheduledSyncTimer() {
  if (scheduledSyncTimer) {
    clearInterval(scheduledSyncTimer);
  }

  scheduledSyncTimer = setInterval(checkAndExecuteScheduledSync, 60 * 1000);
  checkAndExecuteScheduledSync();

  console.log("[Scheduled Sync] Timer started, checking every minute.");
}

console.log(`Starting server on port ${port}...`);
serve({ fetch: app.fetch, port }, (info) => {
  console.log(`Server running at http://localhost:${info.port}`);

  startScheduledSyncTimer();
});

export default app;
