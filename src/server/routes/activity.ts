import type { Hono } from "hono";
import {
  listChangeRecords,
  listFailureRecords,
  readChangeDiff,
  listActivityDates,
} from "../../lib/activity-store";
import { verifyAdmin } from "../auth";

function parsePageParam(value: string | null, fallback: number): number {
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed <= 0) return fallback;
  return Math.floor(parsed);
}

function parsePageSizeParam(value: string | null, fallback: number): number {
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed <= 0) return fallback;
  return Math.min(Math.floor(parsed), 100);
}

function isDateKey(value: string): boolean {
  return /^\d{4}-\d{2}-\d{2}$/.test(value);
}

export function registerActivityRoutes(app: Hono) {
  app.get("/api/activity/changes", async (c) => {
    if (!verifyAdmin(c.req.header("authorization"))) {
      return c.json({ error: "Unauthorized" }, 401);
    }

    const date = c.req.query("date");
    if (date && !isDateKey(date)) {
      return c.json({ error: "Invalid date format" }, 400);
    }

    const page = parsePageParam(c.req.query("page") ?? null, 1);
    const pageSize = parsePageSizeParam(c.req.query("pageSize") ?? null, 20);

    const client = c.req.query("client");
    const result = await listChangeRecords(
      date || undefined,
      page,
      pageSize,
      client || undefined
    );
    return c.json(result);
  });

  app.get("/api/activity/changes/:date/:fileName", async (c) => {
    if (!verifyAdmin(c.req.header("authorization"))) {
      return c.json({ error: "Unauthorized" }, 401);
    }

    const { date, fileName } = c.req.param();
    if (!isDateKey(date)) {
      return c.json({ error: "Invalid date format" }, 400);
    }

    const diff = await readChangeDiff(date, fileName);
    if (!diff) {
      return c.json({ error: "Diff not found" }, 404);
    }

    return c.json({ diff });
  });

  app.get("/api/activity/failures", async (c) => {
    if (!verifyAdmin(c.req.header("authorization"))) {
      return c.json({ error: "Unauthorized" }, 401);
    }

    const date = c.req.query("date");
    if (date && !isDateKey(date)) {
      return c.json({ error: "Invalid date format" }, 400);
    }

    const page = parsePageParam(c.req.query("page") ?? null, 1);
    const pageSize = parsePageSizeParam(c.req.query("pageSize") ?? null, 20);

    const client = c.req.query("client");
    const result = await listFailureRecords(
      date || undefined,
      page,
      pageSize,
      client || undefined
    );
    return c.json(result);
  });

  app.get("/api/activity/dates", async (c) => {
    if (!verifyAdmin(c.req.header("authorization"))) {
      return c.json({ error: "Unauthorized" }, 401);
    }

    const dates = await listActivityDates();
    return c.json({ dates });
  });
}
