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
import { jsonError } from "./errors";
import { registerClientRoutes } from "./routes/clients";
import { registerConfigRoutes } from "./routes/config";
import { registerInitRoutes } from "./routes/init";
import { registerPreviewRoutes } from "./routes/preview";
import { registerRuleRoutes } from "./routes/rules";
import { registerRuleFileRoutes } from "./routes/rule-files";
import { registerStatusRoutes } from "./routes/status";
import { registerSyncRoutes } from "./routes/sync";
import { registerActivityRoutes } from "./routes/activity";
import { registerWafRoutes } from "./routes/waf";

const app = new Hono();

// --- Middleware ---
app.use("*", cors());

// --- API Routes ---
registerConfigRoutes(app);
registerStatusRoutes(app);
registerActivityRoutes(app);
registerSyncRoutes(app);
registerRuleRoutes(app);
registerClientRoutes(app);
registerPreviewRoutes(app);
registerInitRoutes(app);
registerWafRoutes(app);

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

    if (schedule.lastScheduledSyncAt) {
      const lastSync = new Date(schedule.lastScheduledSyncAt);
      const nextSync = new Date(lastSync.getTime() + schedule.intervalHours * 60 * 60 * 1000);

      if (now >= nextSync) {
        needsSync = true;
      }
    } else {
      needsSync = true;
    }

    if (needsSync) {
      console.log(`[Scheduled Sync] Starting scheduled sync at ${now.toISOString()}...`);

      const result = await executeFullSync();

      const nextSyncAt = new Date(now.getTime() + schedule.intervalHours * 60 * 60 * 1000).toISOString();
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
