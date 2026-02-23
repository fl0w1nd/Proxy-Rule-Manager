import type { Hono } from "hono";
import { addClient, deleteClient, getClients, updateClient } from "../../lib/storage-adapter";
import { ClientConfigSchema } from "../../lib/schema";
import { jsonError } from "../errors";

export function registerClientRoutes(app: Hono) {
  app.get("/api/clients", async (c) => {
    try {
      const clients = await getClients();
      return c.json({ clients });
    } catch (error) {
      console.error("Failed to get clients:", error);
      return jsonError(c, error, "Failed to get clients");
    }
  });

  app.post("/api/clients", async (c) => {
    try {
      const body = await c.req.json();
      const client = ClientConfigSchema.parse(body);
      await addClient(client);
      return c.json({ success: true, client });
    } catch (error) {
      console.error("Failed to add client:", error);
      const message = error instanceof Error ? error.message : "Failed to add client";
      return jsonError(c, error, message, 500, {
        validationMessage: "Invalid client format",
      });
    }
  });

  app.put("/api/clients/:id", async (c) => {
    try {
      const clientId = decodeURIComponent(c.req.param("id"));
      const body = await c.req.json();
      const result = await updateClient(clientId, body);
      return c.json({ success: true, ...result });
    } catch (error) {
      console.error("Failed to update client:", error);
      const message = error instanceof Error ? error.message : "Failed to update client";
      return jsonError(c, error, message);
    }
  });

  app.delete("/api/clients/:id", async (c) => {
    try {
      const clientId = decodeURIComponent(c.req.param("id"));
      await deleteClient(clientId);
      return c.json({ success: true, deletedClient: clientId });
    } catch (error) {
      console.error("Failed to delete client:", error);
      const message = error instanceof Error ? error.message : "Failed to delete client";
      return jsonError(c, error, message);
    }
  });
}
