import type { Hono } from "hono";
import * as fs from "node:fs/promises";
import * as path from "node:path";
import AdmZip from "adm-zip";
import { getConfig, getConfigRaw, getConfigRev, invalidateCache, saveConfig } from "../../lib/storage-adapter";
import { detectCircularDependency } from "../../lib/sync-engine";
import { validateConfig } from "../../lib/schema";
import { verifyAdmin } from "../auth";
import { jsonError } from "../errors";
import { getDbFilePath, getSourcesDir } from "../../lib/data-paths";

const SOURCE_FILE_PATTERN = /^[A-Za-z0-9._-]+$/;

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
    if (!verifyAdmin(c.req.header("authorization"))) {
      return c.json({ error: "Unauthorized" }, 401);
    }

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
    if (!verifyAdmin(c.req.header("authorization"))) {
      return c.json({ error: "Unauthorized" }, 401);
    }

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

  app.get("/api/config/export", async (c) => {
    if (!verifyAdmin(c.req.header("authorization"))) {
      return c.json({ error: "Unauthorized" }, 401);
    }

    try {
      const zip = new AdmZip();
      const dbPath = getDbFilePath();
      const dbBuffer = await fs.readFile(dbPath);
      zip.addFile("db.json", dbBuffer);

      const sourcesDir = getSourcesDir();
      try {
        const entries = await fs.readdir(sourcesDir, { withFileTypes: true });
        for (const entry of entries) {
          if (!entry.isFile()) continue;
          const filePath = path.join(sourcesDir, entry.name);
          const content = await fs.readFile(filePath);
          zip.addFile(`sources/${entry.name}`, content);
        }
      } catch {
        // No sources directory yet.
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
      console.error("Failed to export config:", error);
      return jsonError(c, error, "Failed to export config");
    }
  });

  app.post("/api/config/import", async (c) => {
    if (!verifyAdmin(c.req.header("authorization"))) {
      return c.json({ error: "Unauthorized" }, 401);
    }

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
      const sourceFiles: { name: string; data: Buffer }[] = [];

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
          const fileName = normalized.slice("sources/".length);
          if (!fileName || fileName.includes("/") || !SOURCE_FILE_PATTERN.test(fileName)) continue;
          sourceFiles.push({ name: fileName, data: entry.getData() });
        }
      }

      if (!dbPayload) {
        return c.json({ error: "Import file missing db.json" }, 400);
      }

      const sourcesDir = getSourcesDir();
      await fs.rm(sourcesDir, { recursive: true, force: true });
      await fs.mkdir(sourcesDir, { recursive: true });

      await Promise.all(
        sourceFiles.map((source) =>
          fs.writeFile(path.join(sourcesDir, source.name), source.data)
        )
      );

      await atomicWriteFile(getDbFilePath(), dbPayload);
      invalidateCache();

      return c.json({ success: true });
    } catch (error) {
      console.error("Failed to import config:", error);
      return jsonError(c, error, "Failed to import config");
    }
  });
}
