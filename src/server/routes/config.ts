import type { Hono } from "hono";
import * as fs from "node:fs/promises";
import * as path from "node:path";
import AdmZip from "adm-zip";
import YAML from "yaml";
import {
  getConfig,
  getConfigRaw,
  getConfigRev,
  invalidateCache,
  resetDatabaseWithConfig,
  saveConfig,
} from "../../lib/storage-adapter";
import { detectCircularDependency } from "../../lib/sync-engine";
import { ClientConfig, DEFAULT_CLIENTS, validateConfig } from "../../lib/schema";
import { jsonError } from "../errors";
import { getDataDir, getDbFilePath, getIconSetDir, getSourcesDir, getClientFilesDir } from "../../lib/data-paths";

const SOURCE_FILE_PATTERN = /^[A-Za-z0-9._-]+$/;
const CLIENT_FILE_NAME_PATTERN = /^[^/\\\\]+\\.[^/\\\\]+$/;

async function addDirToZip(zip: AdmZip, sourceDir: string, prefix: string): Promise<void> {
  const entries = await fs.readdir(sourceDir, { withFileTypes: true });
  for (const entry of entries) {
    const entryPath = path.join(sourceDir, entry.name);
    const zipPath = `${prefix}/${entry.name}`;
    if (entry.isDirectory()) {
      await addDirToZip(zip, entryPath, zipPath);
      continue;
    }
    if (!entry.isFile()) continue;
    const content = await fs.readFile(entryPath);
    zip.addFile(zipPath, content);
  }
}

function buildClientsForConfig(config: ReturnType<typeof validateConfig>): ClientConfig[] {
  const clients = new Map<string, ClientConfig>();
  for (const client of DEFAULT_CLIENTS) {
    clients.set(client.id, { ...client });
  }
  for (const rule of config.rules) {
    for (const clientId of rule.output.clients) {
      if (!clients.has(clientId)) {
        clients.set(clientId, {
          id: clientId,
          displayName: clientId,
          pathName: clientId,
        });
      }
    }
  }
  return Array.from(clients.values());
}

async function atomicWriteFile(filePath: string, content: Buffer): Promise<void> {
  const dir = path.dirname(filePath);
  const base = path.basename(filePath);
  const tempPath = path.join(dir, `.${base}.${process.pid}.${Date.now()}.tmp`);
  await fs.mkdir(dir, { recursive: true });
  await fs.writeFile(tempPath, content);
  try {
    await fs.rename(tempPath, filePath);
  } catch (err: unknown) {
    const code = err && typeof err === "object" && "code" in err ? err.code : null;
    if (code === "EXDEV" || code === "EPERM" || code === "EEXIST") {
      await fs.unlink(filePath).catch(() => undefined);
      await fs.rename(tempPath, filePath);
      return;
    }
    await fs.unlink(tempPath).catch(() => undefined);
    throw err;
  }
}

export function registerConfigRoutes(app: Hono) {
  app.get("/api/config", async (c) => {
    try {
      const rawParam = c.req.query("raw");
      const useRaw = rawParam === "1" || rawParam === "true";
      const config = useRaw ? await getConfigRaw() : await getConfig();
      const rev = await getConfigRev();
      return c.json({ config, rev });
    } catch (error) {
      console.error("Failed to get config:", error);
      return jsonError(c, error, "Failed to get config");
    }
  });

  app.put("/api/config", async (c) => {
    try {
      const body = await c.req.json();
      const { config } = body;
      const validated = validateConfig(config);

      const cycle = detectCircularDependency(validated.rules);
      if (cycle) {
        const cycleStr = cycle.join(" → ");
        return c.json({ error: `检测到循环依赖: ${cycleStr}` }, 400);
      }

      const { rev } = await saveConfig(validated);
      const affectedRules = validated.rules.map((r) => r.name);
      return c.json({ success: true, rev, affectedRules });
    } catch (error) {
      console.error("Failed to save config:", error);
      return jsonError(c, error, "Failed to save config", 500, {
        validationMessage: "Invalid config format",
      });
    }
  });

  app.get("/api/database/backup", async (c) => {
    try {
      const zip = new AdmZip();
      const dbPath = getDbFilePath();
      const dbBuffer = await fs.readFile(dbPath);
      zip.addFile("db.json", dbBuffer);

      // client files - 直接打包 data/client 目录
      const clientFilesDir = getClientFilesDir();
      try {
        await addDirToZip(zip, clientFilesDir, "client-files");
      } catch {
        // No client files directory yet.
      }

      const sourcesDir = getSourcesDir();
      try {
        await addDirToZip(zip, sourcesDir, "sources");
      } catch {
        // No sources directory yet.
      }

      const wafDir = path.join(getDataDir(), "waf");
      try {
        await addDirToZip(zip, wafDir, "waf");
      } catch {
        // No waf directory yet.
      }

      const iconsetDir = getIconSetDir();
      try {
        await addDirToZip(zip, iconsetDir, "iconset");
      } catch {
        // No iconset directory yet.
      }

      const dateTag = new Date().toISOString().split("T")[0];
      const zipBuffer = zip.toBuffer();
      const arrayBuffer = zipBuffer.buffer.slice(
        zipBuffer.byteOffset,
        zipBuffer.byteOffset + zipBuffer.byteLength
      ) as ArrayBuffer;
      return c.body(arrayBuffer, 200, {
        "Content-Type": "application/zip",
        "Content-Disposition": `attachment; filename="proxy-rule-manager-${dateTag}.zip"`,
      });
    } catch (error) {
      console.error("Failed to backup database:", error);
      return jsonError(c, error, "Failed to backup database");
    }
  });

  app.post("/api/database/restore", async (c) => {
    try {
      const form = await c.req.formData();
      const file = form.get("file");
      if (!file || typeof (file as { arrayBuffer?: unknown }).arrayBuffer !== "function") {
        return c.json({ error: "Missing import file" }, 400);
      }

      const buffer = Buffer.from(await (file as Blob).arrayBuffer());
      const zip = new AdmZip(buffer);
      const entries = zip.getEntries();

      let dbPayload: Buffer | null = null;
      const sourceFiles: { path: string; data: Buffer }[] = [];
      const clientFileEntries: { fileName: string; data: Buffer }[] = [];
      const wafFiles: { path: string; data: Buffer }[] = [];
      const iconsetFiles: { path: string; data: Buffer }[] = [];

      for (const entry of entries) {
        if (entry.isDirectory) continue;
        const normalized = path.posix.normalize(entry.entryName);
        if (normalized.startsWith("..") || path.posix.isAbsolute(normalized)) {
          continue;
        }
        if (normalized === "db.json") {
          dbPayload = entry.getData();
          continue;
        }
        if (normalized.startsWith("sources/")) {
          const sourcePath = normalized.slice("sources/".length);
          if (!sourcePath || sourcePath.includes("..")) continue;
          const fileName = path.posix.basename(sourcePath);
          if (!SOURCE_FILE_PATTERN.test(fileName)) continue;
          sourceFiles.push({ path: sourcePath, data: entry.getData() });
        }
        if (normalized.startsWith("client-files/")) {
          const filePath = normalized.slice("client-files/".length);
          if (!filePath || filePath.includes("..")) continue;
          const fileName = path.posix.basename(filePath);
          if (!CLIENT_FILE_NAME_PATTERN.test(fileName)) continue;
          clientFileEntries.push({ fileName: filePath, data: entry.getData() });
        }
        if (normalized.startsWith("waf/")) {
          const wafPath = normalized.slice("waf/".length);
          if (!wafPath || wafPath.includes("..")) continue;
          wafFiles.push({ path: wafPath, data: entry.getData() });
        }
        if (normalized.startsWith("iconset/")) {
          const iconsetPath = normalized.slice("iconset/".length);
          if (!iconsetPath || iconsetPath.includes("..")) continue;
          iconsetFiles.push({ path: iconsetPath, data: entry.getData() });
        }
      }

      if (!dbPayload) {
        return c.json({ error: "Import file missing db.json" }, 400);
      }

      // Pre-validate db.json before any destructive operation
      let dbJson: Record<string, unknown>;
      try {
        dbJson = JSON.parse(dbPayload.toString("utf-8")) as Record<string, unknown>;
      } catch {
        return c.json({ error: "Invalid db.json format" }, 400);
      }
      dbJson.artifacts = {};
      dbJson.lastSyncInfo = {
        lastFullSyncAt: null,
        lastPartialSyncAt: null,
        lastSuccessfulSyncAt: null,
        totalRulesCount: 0,
        changedRulesCount: 0,
        failedRulesCount: 0,
      };

      const dataDir = getDataDir();
      // clean all data contents
      try {
        const entries = await fs.readdir(dataDir, { withFileTypes: true });
        for (const entry of entries) {
          await fs.rm(path.join(dataDir, entry.name), { recursive: true, force: true });
        }
      } catch {
        // ignore
      }

      const sourcesDir = getSourcesDir();
      await fs.mkdir(sourcesDir, { recursive: true });

      await Promise.all(
        sourceFiles.map((source) => {
          const targetPath = path.join(sourcesDir, source.path);
          return fs.mkdir(path.dirname(targetPath), { recursive: true })
            .then(() => fs.writeFile(targetPath, source.data));
        })
      );

      if (clientFileEntries.length > 0) {
        const clientDir = getClientFilesDir();
        await fs.mkdir(clientDir, { recursive: true });
        for (const entry of clientFileEntries) {
          const targetPath = path.join(clientDir, entry.fileName);
          await fs.mkdir(path.dirname(targetPath), { recursive: true });
          await fs.writeFile(targetPath, entry.data);
        }
      }

      if (wafFiles.length > 0) {
        const wafDir = path.join(dataDir, "waf");
        await fs.mkdir(wafDir, { recursive: true });
        for (const entry of wafFiles) {
          const targetPath = path.join(wafDir, entry.path);
          await fs.mkdir(path.dirname(targetPath), { recursive: true });
          await fs.writeFile(targetPath, entry.data);
        }
      }

      if (iconsetFiles.length > 0) {
        const iconsetDir = getIconSetDir();
        await fs.mkdir(iconsetDir, { recursive: true });
        for (const entry of iconsetFiles) {
          const targetPath = path.join(iconsetDir, entry.path);
          await fs.mkdir(path.dirname(targetPath), { recursive: true });
          await fs.writeFile(targetPath, entry.data);
        }
      }

      await atomicWriteFile(getDbFilePath(), Buffer.from(JSON.stringify(dbJson, null, 2)));
      invalidateCache();

      return c.json({ success: true });
    } catch (error) {
      console.error("Failed to restore database:", error);
      return jsonError(c, error, "Failed to restore database");
    }
  });

  app.get("/api/config/template/export", async (c) => {
    try {
      const config = await getConfig();
      const dateTag = new Date().toISOString().split("T")[0];
      const payload = JSON.stringify(config, null, 2);
      return c.body(payload, 200, {
        "Content-Type": "application/json",
        "Content-Disposition": `attachment; filename="proxy-rule-template-${dateTag}.json"`,
      });
    } catch (error) {
      console.error("Failed to export template:", error);
      return jsonError(c, error, "Failed to export template");
    }
  });

  app.post("/api/config/template/import", async (c) => {
    try {
      const form = await c.req.formData();
      const file = form.get("file");
      if (!file || typeof (file as { arrayBuffer?: unknown }).arrayBuffer !== "function") {
        return c.json({ error: "Missing import file" }, 400);
      }

      const buffer = Buffer.from(await (file as Blob).arrayBuffer());
      const rawText = buffer.toString("utf-8");
      const parsed =
        rawText.trim().startsWith("{") || rawText.trim().startsWith("[")
          ? JSON.parse(rawText)
          : YAML.parse(rawText);
      const validated = validateConfig(parsed);
      const clients = buildClientsForConfig(validated);
      const { rev } = await resetDatabaseWithConfig(validated, clients);
      invalidateCache();

      return c.json({ success: true, rev });
    } catch (error) {
      console.error("Failed to import template:", error);
      return jsonError(c, error, "Failed to import template", 500, {
        validationMessage: "Invalid config format",
      });
    }
  });
}
