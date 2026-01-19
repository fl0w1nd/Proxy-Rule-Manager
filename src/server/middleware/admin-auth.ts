/**
 * Admin 认证中间件
 *
 * 统一处理管理员认证和 Rate Limiting
 */

import type { Context, Next } from "hono";
import { verifyAdminWithRateLimit, getClientIp } from "../auth";

/**
 * 管理员认证中间件 - 带有 Rate Limiting
 */
export const adminAuth = async (c: Context, next: Next) => {
    const ip = getClientIp((name) => c.req.header(name));
    const authHeader = c.req.header("authorization");

    const result = await verifyAdminWithRateLimit(authHeader, ip);

    if (!result.success) {
        if (result.error === "blocked") {
            c.header("Retry-After", String(result.retryAfter || 60));
            return c.json(
                {
                    error: "Too many failed attempts",
                    retryAfter: result.retryAfter,
                    message: `请在 ${result.retryAfter} 秒后重试`,
                },
                429
            );
        }

        return c.json({ error: "Unauthorized" }, 401);
    }

    await next();
};
