import type { Hono } from "hono";
import { promises as fs } from "node:fs";
import * as path from "node:path";
import { getConfig, getSyncSchedule, saveConfig, updateSyncSchedule } from "../../lib/storage-adapter";
import { getNextSyncAt } from "../../lib/sync-schedule";
import { validateConfig } from "../../lib/schema";
import { verifyAdmin } from "../auth";
import { AppError, jsonError } from "../errors";

const DEFAULT_TEMPLATE_PATHS = [
  process.env.INITIAL_CONFIG_PATH,
  path.join(process.cwd(), "out", "templates", "initial-config.json"),
  path.join(process.cwd(), "public", "templates", "initial-config.json"),
].filter((candidate): candidate is string => Boolean(candidate));

async function loadInitialConfigTemplate(): Promise<string> {
  let lastError: unknown;

  for (const candidate of DEFAULT_TEMPLATE_PATHS) {
    try {
      return await fs.readFile(candidate, "utf-8");
    } catch (error) {
      lastError = error;
    }
  }

  throw new AppError(
    "Initial config template not found",
    500,
    "TEMPLATE_NOT_FOUND",
    { candidates: DEFAULT_TEMPLATE_PATHS, lastError: String(lastError) }
  );
}

export function registerInitRoutes(app: Hono) {
  app.get("/api/init", async (c) => {
    if (!verifyAdmin(c.req.header("authorization"))) {
      return c.json({ error: "Unauthorized" }, 401);
    }

    try {
      const config = await getConfig();
      return c.json({
        initialized: config.rules.length > 0,
        rulesCount: config.rules.length,
      });
    } catch (error) {
      console.error("Failed to check init status:", error);
      return jsonError(c, error, "Failed to check init status");
    }
  });

  app.post("/api/init", async (c) => {
    if (!verifyAdmin(c.req.header("authorization"))) {
      return c.json({ error: "Unauthorized" }, 401);
    }

    try {
      const currentConfig = await getConfig();
      if (currentConfig.rules.length > 0) {
        return c.json({
          success: false,
          message: "Already initialized with existing rules",
          rulesCount: currentConfig.rules.length,
        });
      }

      const templateContent = await loadInitialConfigTemplate();
      const initialConfig = validateConfig(JSON.parse(templateContent));

      const { rev } = await saveConfig(initialConfig);

      const currentSchedule = await getSyncSchedule();
      const now = new Date();

      await updateSyncSchedule({
        lastScheduledSyncAt: now.toISOString(),
        nextSyncAt: getNextSyncAt(currentSchedule, now),
      });
      return c.json({
        success: true,
        message: `Initialized with ${initialConfig.rules.length} rules`,
        rulesCount: initialConfig.rules.length,
        rev,
      });
    } catch (error) {
      console.error("Failed to initialize:", error);
      const message = error instanceof Error
        ? `Initialization failed: ${error.message}`
        : "Initialization failed";
      return jsonError(c, error, message, 500, {
        validationMessage: "Invalid config format",
      });
    }
  });
}
