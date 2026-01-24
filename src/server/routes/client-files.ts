import type { Hono } from "hono";
import { z } from "zod";
import {
  createClientFile,
  deleteClientFile,
  getClientFileContent,
  getClientFileMeta,
  listClientFiles,
  listPublicClientFiles,
  updateClientFile,
  getPublicClientFile,
} from "../../lib/storage-adapter";
import { verifyAdmin } from "../auth";
import { jsonError } from "../errors";

const ClientFileCreateSchema = z.object({
  configId: z.string().min(1),
  displayName: z.string().min(1),
  description: z.string().optional(),
  ext: z.string().min(1),
  isPublic: z.boolean().optional().default(false),
  content: z.string().optional().default(""),
});

const ClientFileUpdateSchema = z.object({
  configId: z.string().min(1).optional(),
  displayName: z.string().min(1).optional(),
  description: z.string().optional(),
  ext: z.string().min(1).optional(),
  isPublic: z.boolean().optional(),
  content: z.string().optional(),
});

function normalizeExt(ext: string): string {
  return ext.startsWith(".") ? ext.slice(1) : ext;
}

function validateSegment(value: string, label: string) {
  if (value.includes("/") || value.includes("\\") || value.includes("..")) {
    throw new Error(`${label} contains invalid characters`);
  }
}

export function registerClientFileRoutes(app: Hono) {
  // Admin APIs
  app.get("/api/clients/:id/files", async (c) => {
    if (!verifyAdmin(c.req.header("authorization"))) {
      return c.json({ error: "Unauthorized" }, 401);
    }

    try {
      const clientId = decodeURIComponent(c.req.param("id"));
      validateSegment(clientId, "Client ID");
      const files = await listClientFiles(clientId);
      return c.json({ files });
    } catch (error) {
      console.error("Failed to list client files:", error);
      return jsonError(c, error, "Failed to list client files");
    }
  });

  app.post("/api/clients/:id/files", async (c) => {
    if (!verifyAdmin(c.req.header("authorization"))) {
      return c.json({ error: "Unauthorized" }, 401);
    }

    try {
      const clientId = decodeURIComponent(c.req.param("id"));
      validateSegment(clientId, "Client ID");
      const body = await c.req.json();
      const parsed = ClientFileCreateSchema.parse(body);
      const ext = normalizeExt(parsed.ext);
      validateSegment(parsed.configId, "Config ID");
      validateSegment(ext, "File extension");

      const meta = await createClientFile(clientId, {
        configId: parsed.configId,
        displayName: parsed.displayName,
        description: parsed.description,
        ext,
        isPublic: !!parsed.isPublic,
        content: parsed.content || "",
      });
      return c.json({ success: true, file: meta });
    } catch (error) {
      console.error("Failed to create client file:", error);
      const message = error instanceof Error ? error.message : "Failed to create client file";
      return jsonError(c, error, message);
    }
  });

  app.get("/api/clients/:id/files/:fileId", async (c) => {
    if (!verifyAdmin(c.req.header("authorization"))) {
      return c.json({ error: "Unauthorized" }, 401);
    }

    try {
      const clientId = decodeURIComponent(c.req.param("id"));
      validateSegment(clientId, "Client ID");
      const fileId = decodeURIComponent(c.req.param("fileId"));
      const meta = await getClientFileMeta(fileId);
      if (!meta || meta.clientId !== clientId) {
        return c.json({ error: "Not Found" }, 404);
      }
      const content = await getClientFileContent(fileId);
      return c.json({ file: meta, content: content ?? "" });
    } catch (error) {
      console.error("Failed to get client file:", error);
      return jsonError(c, error, "Failed to get client file");
    }
  });

  app.put("/api/clients/:id/files/:fileId", async (c) => {
    if (!verifyAdmin(c.req.header("authorization"))) {
      return c.json({ error: "Unauthorized" }, 401);
    }

    try {
      const clientId = decodeURIComponent(c.req.param("id"));
      validateSegment(clientId, "Client ID");
      const fileId = decodeURIComponent(c.req.param("fileId"));
      const body = await c.req.json();
      const parsed = ClientFileUpdateSchema.parse(body);
      if (parsed.configId) validateSegment(parsed.configId, "Config ID");
      if (parsed.ext) validateSegment(parsed.ext, "File extension");
      const updates = {
        ...parsed,
        ext: parsed.ext ? normalizeExt(parsed.ext) : undefined,
      };

      const meta = await getClientFileMeta(fileId);
      if (!meta || meta.clientId !== clientId) {
        return c.json({ error: "Not Found" }, 404);
      }

      const updated = await updateClientFile(fileId, updates);
      return c.json({ success: true, file: updated });
    } catch (error) {
      console.error("Failed to update client file:", error);
      const message = error instanceof Error ? error.message : "Failed to update client file";
      return jsonError(c, error, message);
    }
  });

  app.delete("/api/clients/:id/files/:fileId", async (c) => {
    if (!verifyAdmin(c.req.header("authorization"))) {
      return c.json({ error: "Unauthorized" }, 401);
    }

    try {
      const clientId = decodeURIComponent(c.req.param("id"));
      validateSegment(clientId, "Client ID");
      const fileId = decodeURIComponent(c.req.param("fileId"));
      const meta = await getClientFileMeta(fileId);
      if (!meta || meta.clientId !== clientId) {
        return c.json({ error: "Not Found" }, 404);
      }
      await deleteClientFile(fileId);
      return c.json({ success: true, deletedFile: fileId });
    } catch (error) {
      console.error("Failed to delete client file:", error);
      const message = error instanceof Error ? error.message : "Failed to delete client file";
      return jsonError(c, error, message);
    }
  });

  // Public list (for public page)
  app.get("/api/client-files/public", async (c) => {
    try {
      const files = await listPublicClientFiles();
      return c.json({ files });
    } catch (error) {
      console.error("Failed to list public client files:", error);
      return jsonError(c, error, "Failed to list public client files");
    }
  });

  // Public file access
  app.get("/:clientId/:file", async (c) => {
    try {
      const clientId = decodeURIComponent(c.req.param("clientId"));
      const file = decodeURIComponent(c.req.param("file"));

      if (clientId === "api" || clientId === "Rules" || clientId === "templates" || clientId.startsWith("_")) {
        return c.text("Not Found", 404);
      }

      const lastDot = file.lastIndexOf(".");
      if (lastDot <= 0) {
        return c.text("# Invalid file format", 400);
      }

      const configId = file.slice(0, lastDot);
      const ext = file.slice(lastDot + 1);
      validateSegment(clientId, "Client ID");
      validateSegment(configId, "Config ID");
      validateSegment(ext, "File extension");

      const result = await getPublicClientFile(clientId, configId, ext);
      if (!result) {
        return c.text("# File not found", 404);
      }

      return c.text(result.content, 200, {
        "Content-Type": "text/plain; charset=utf-8",
        "Cache-Control": "no-cache",
      });
    } catch (error) {
      console.error("Failed to serve public client file:", error);
      return jsonError(c, error, "Failed to serve public client file");
    }
  });
}
