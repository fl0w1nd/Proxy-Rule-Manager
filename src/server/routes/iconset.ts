import type { Hono } from "hono";
import { promises as fs } from "node:fs";
import * as path from "node:path";
import { getIconSetDir } from "../../lib/data-paths";
import { getCdnSettings, buildResponseHeaders } from "../../lib/storage-adapter";
import { verifyAdmin } from "../auth";
import { jsonError } from "../errors";

interface IconInfo {
  id: string;
  name: string;
  url: string;
  size: number;
  createdAt: string;
}

const IMAGE_EXTENSIONS = [".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".ico"];

function isImageFile(filename: string): boolean {
  const ext = path.extname(filename).toLowerCase();
  return IMAGE_EXTENSIONS.includes(ext);
}

function getMimeType(filename: string): string {
  const ext = path.extname(filename).toLowerCase();
  const mimeTypes: Record<string, string> = {
    ".png": "image/png",
    ".jpg": "image/jpeg",
    ".jpeg": "image/jpeg",
    ".gif": "image/gif",
    ".svg": "image/svg+xml",
    ".webp": "image/webp",
    ".ico": "image/x-icon",
  };
  return mimeTypes[ext] || "application/octet-stream";
}

async function ensureIconSetDir(): Promise<string> {
  const dir = getIconSetDir();
  await fs.mkdir(dir, { recursive: true });
  return dir;
}

async function getUniqueFilename(dir: string, originalName: string): Promise<string> {
  const ext = path.extname(originalName);
  const baseName = path.basename(originalName, ext);

  let candidate = originalName;
  let exists = await fileExists(path.join(dir, candidate));

  if (!exists) {
    return candidate;
  }

  let copyNum = 0;
  while (exists) {
    copyNum++;
    if (copyNum === 1) {
      candidate = `${baseName}_copy${ext}`;
    } else {
      candidate = `${baseName}_copy_${copyNum}${ext}`;
    }
    exists = await fileExists(path.join(dir, candidate));
  }

  return candidate;
}

async function fileExists(filePath: string): Promise<boolean> {
  try {
    await fs.access(filePath);
    return true;
  } catch {
    return false;
  }
}

export function registerIconSetRoutes(app: Hono) {
  app.get("/api/iconset", async (c) => {
    try {
      const dir = await ensureIconSetDir();
      const files = await fs.readdir(dir);

      const icons: IconInfo[] = [];
      for (const file of files) {
        if (!isImageFile(file)) continue;

        const filePath = path.join(dir, file);
        const stat = await fs.stat(filePath);

        icons.push({
          id: file,
          name: path.basename(file, path.extname(file)),
          url: `/IconSet/${encodeURIComponent(file)}`,
          size: stat.size,
          createdAt: stat.birthtime.toISOString(),
        });
      }

      icons.sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime());

      return c.json({ icons });
    } catch (error) {
      console.error("Failed to list icons:", error);
      return jsonError(c, error, "Failed to list icons");
    }
  });

  app.post("/api/iconset/upload", async (c) => {
    if (!verifyAdmin(c.req.header("authorization"))) {
      return c.json({ error: "Unauthorized" }, 401);
    }

    try {
      const dir = await ensureIconSetDir();
      const formData = await c.req.formData();
      const files = formData.getAll("files") as File[];

      if (files.length === 0) {
        return c.json({ error: "No files provided" }, 400);
      }

      const uploaded: IconInfo[] = [];
      const renamed: { original: string; renamed: string }[] = [];
      const errors: { name: string; error: string }[] = [];

      for (const file of files) {
        if (!(file instanceof File)) {
          errors.push({ name: "unknown", error: "Invalid file" });
          continue;
        }

        if (!isImageFile(file.name)) {
          errors.push({ name: file.name, error: "Not a valid image file" });
          continue;
        }

        try {
          const uniqueName = await getUniqueFilename(dir, file.name);
          const filePath = path.join(dir, uniqueName);
          const buffer = Buffer.from(await file.arrayBuffer());
          await fs.writeFile(filePath, buffer);

          if (uniqueName !== file.name) {
            renamed.push({ original: file.name, renamed: uniqueName });
          }

          const stat = await fs.stat(filePath);
          uploaded.push({
            id: uniqueName,
            name: path.basename(uniqueName, path.extname(uniqueName)),
            url: `/IconSet/${encodeURIComponent(uniqueName)}`,
            size: stat.size,
            createdAt: stat.birthtime.toISOString(),
          });
        } catch (err) {
          errors.push({
            name: file.name,
            error: err instanceof Error ? err.message : "Upload failed",
          });
        }
      }

      return c.json({ success: true, uploaded, renamed, errors });
    } catch (error) {
      console.error("Failed to upload icons:", error);
      return jsonError(c, error, "Failed to upload icons");
    }
  });

  app.put("/api/iconset/:id", async (c) => {
    if (!verifyAdmin(c.req.header("authorization"))) {
      return c.json({ error: "Unauthorized" }, 401);
    }

    try {
      const id = decodeURIComponent(c.req.param("id"));
      const body = await c.req.json();
      const { newName } = body;

      if (!newName || typeof newName !== "string") {
        return c.json({ error: "newName is required" }, 400);
      }

      const dir = await ensureIconSetDir();
      const oldPath = path.join(dir, id);

      if (!(await fileExists(oldPath))) {
        return c.json({ error: "Icon not found" }, 404);
      }

      const ext = path.extname(id);
      const newFilename = newName.endsWith(ext) ? newName : `${newName}${ext}`;
      const newPath = path.join(dir, newFilename);

      if (oldPath !== newPath && (await fileExists(newPath))) {
        return c.json({ error: "An icon with this name already exists" }, 409);
      }

      await fs.rename(oldPath, newPath);

      const stat = await fs.stat(newPath);
      const icon: IconInfo = {
        id: newFilename,
        name: path.basename(newFilename, ext),
        url: `/IconSet/${encodeURIComponent(newFilename)}`,
        size: stat.size,
        createdAt: stat.birthtime.toISOString(),
      };

      return c.json({ success: true, icon });
    } catch (error) {
      console.error("Failed to rename icon:", error);
      return jsonError(c, error, "Failed to rename icon");
    }
  });

  app.delete("/api/iconset/:id", async (c) => {
    if (!verifyAdmin(c.req.header("authorization"))) {
      return c.json({ error: "Unauthorized" }, 401);
    }

    try {
      const id = decodeURIComponent(c.req.param("id"));
      const dir = await ensureIconSetDir();
      const filePath = path.join(dir, id);

      if (!(await fileExists(filePath))) {
        return c.json({ error: "Icon not found" }, 404);
      }

      await fs.unlink(filePath);

      return c.json({ success: true, deleted: id });
    } catch (error) {
      console.error("Failed to delete icon:", error);
      return jsonError(c, error, "Failed to delete icon");
    }
  });

  app.get("/IconSet/:filename", async (c) => {
    try {
      const filename = decodeURIComponent(c.req.param("filename"));
      const dir = getIconSetDir();
      const filePath = path.join(dir, filename);

      const normalizedPath = path.normalize(filePath);
      if (!normalizedPath.startsWith(dir)) {
        return c.text("Invalid path", 400);
      }

      if (!(await fileExists(filePath))) {
        return c.text("Not found", 404);
      }

      const content = await fs.readFile(filePath);
      const mimeType = getMimeType(filename);

      const cdnSettings = await getCdnSettings();
      const headers = buildResponseHeaders(cdnSettings);
      // 覆盖 Content-Type 为图片的 MIME 类型
      headers["Content-Type"] = mimeType;

      return new Response(content, {
        status: 200,
        headers,
      });
    } catch (error) {
      console.error("Failed to serve icon:", error);
      return c.text("Internal server error", 500);
    }
  });
}
