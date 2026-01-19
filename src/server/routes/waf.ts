/**
 * WAF 管理 API 路由
 *
 * 提供 IP 封禁管理功能
 */

import type { Hono } from "hono";
import { verifyAdmin, getClientIp } from "../auth";
import {
    getAllBans,
    upsertBan,
    removeBan,
    cleanupExpiredBans,
    getBanStats,
} from "../../lib/ban-store";
import {
    getAllFailureRecords,
    calculateBlockDuration,
} from "../rate-limiter";
import { jsonError } from "../errors";

export function registerWafRoutes(app: Hono) {
    /**
     * 获取所有封禁记录
     */
    app.get("/api/waf/bans", async (c) => {
        if (!verifyAdmin(c.req.header("authorization"))) {
            return c.json({ error: "Unauthorized" }, 401);
        }

        try {
            const bans = await getAllBans();
            return c.json({ bans });
        } catch (error) {
            return jsonError(c, error, "Failed to get bans");
        }
    });

    /**
     * 手动添加封禁
     */
    app.post("/api/waf/bans", async (c) => {
        if (!verifyAdmin(c.req.header("authorization"))) {
            return c.json({ error: "Unauthorized" }, 401);
        }

        try {
            const body = await c.req.json();
            const { ip, reason, permanent, durationSeconds } = body;

            if (!ip || typeof ip !== "string") {
                return c.json({ error: "IP address is required" }, 400);
            }

            const now = new Date();
            let expiresAt: string | null = null;

            if (!permanent && durationSeconds) {
                expiresAt = new Date(
                    now.getTime() + durationSeconds * 1000
                ).toISOString();
            } else if (!permanent) {
                // 默认 1 小时
                expiresAt = new Date(now.getTime() + 3600 * 1000).toISOString();
            }

            await upsertBan({
                ip,
                reason: reason || "manual_ban",
                bannedAt: now.toISOString(),
                expiresAt,
                failCount: 0,
            });

            return c.json({ success: true, message: `IP ${ip} has been banned` });
        } catch (error) {
            return jsonError(c, error, "Failed to add ban");
        }
    });

    /**
     * 移除封禁
     */
    app.delete("/api/waf/bans/:ip", async (c) => {
        if (!verifyAdmin(c.req.header("authorization"))) {
            return c.json({ error: "Unauthorized" }, 401);
        }

        try {
            const ip = decodeURIComponent(c.req.param("ip"));
            const removed = await removeBan(ip);

            if (removed) {
                return c.json({ success: true, message: `IP ${ip} has been unbanned` });
            } else {
                return c.json({ error: "IP not found in ban list" }, 404);
            }
        } catch (error) {
            return jsonError(c, error, "Failed to remove ban");
        }
    });

    /**
     * 获取 WAF 统计信息
     */
    app.get("/api/waf/stats", async (c) => {
        if (!verifyAdmin(c.req.header("authorization"))) {
            return c.json({ error: "Unauthorized" }, 401);
        }

        try {
            const banStats = await getBanStats();
            const failureRecords = getAllFailureRecords();

            // 统计当前被临时阻塞的 IP
            const now = Date.now();
            let currentlyBlocked = 0;
            for (const [, record] of failureRecords) {
                const blockDuration = calculateBlockDuration(record.count);
                const blockedUntil = record.lastFailedAt + blockDuration * 1000;
                if (now < blockedUntil) {
                    currentlyBlocked++;
                }
            }

            return c.json({
                bans: banStats,
                temporary: {
                    totalTracked: failureRecords.size,
                    currentlyBlocked,
                },
            });
        } catch (error) {
            return jsonError(c, error, "Failed to get WAF stats");
        }
    });

    /**
     * 获取活跃失败记录（调试用）
     */
    app.get("/api/waf/failures", async (c) => {
        if (!verifyAdmin(c.req.header("authorization"))) {
            return c.json({ error: "Unauthorized" }, 401);
        }

        try {
            const records = getAllFailureRecords();
            const now = Date.now();

            const failures = Array.from(records.entries()).map(([ip, record]) => {
                const blockDuration = calculateBlockDuration(record.count);
                const blockedUntil = record.lastFailedAt + blockDuration * 1000;
                const isBlocked = now < blockedUntil;

                return {
                    ip,
                    failCount: record.count,
                    lastFailedAt: new Date(record.lastFailedAt).toISOString(),
                    blockDuration,
                    isBlocked,
                    blockedUntil: isBlocked
                        ? new Date(blockedUntil).toISOString()
                        : null,
                };
            });

            return c.json({ failures });
        } catch (error) {
            return jsonError(c, error, "Failed to get failure records");
        }
    });

    /**
     * 清理过期封禁
     */
    app.post("/api/waf/cleanup", async (c) => {
        if (!verifyAdmin(c.req.header("authorization"))) {
            return c.json({ error: "Unauthorized" }, 401);
        }

        try {
            const removed = await cleanupExpiredBans();
            return c.json({
                success: true,
                message: `Cleaned up ${removed} expired bans`,
            });
        } catch (error) {
            return jsonError(c, error, "Failed to cleanup bans");
        }
    });

    /**
     * 获取当前请求者的 IP
     */
    app.get("/api/waf/my-ip", async (c) => {
        const ip = getClientIp((name) => c.req.header(name));
        return c.json({ ip });
    });
}
