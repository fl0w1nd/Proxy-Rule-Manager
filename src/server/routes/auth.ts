import type { Hono } from "hono";

export function registerAuthRoutes(app: Hono) {
  app.get("/api/auth/required", (c) => {
    const required = !!process.env.ADMIN_TOKEN;
    return c.json({ required });
  });
}
