import { isBlocked, recordFailure, clearRecord } from "./rate-limiter";

export function verifyAdmin(authHeader: string | undefined): boolean {
  const adminToken = process.env.ADMIN_TOKEN;
  if (!adminToken) return true;

  if (!authHeader?.startsWith("Bearer ")) return false;
  return authHeader.slice(7) === adminToken;
}

export function checkAuth(
  authHeader: string | undefined
): "admin" | "public" | "invalid" {
  const adminToken = process.env.ADMIN_TOKEN;
  if (!adminToken) return "admin";
  if (!authHeader) return "public";
  if (!authHeader.startsWith("Bearer ")) return "invalid";
  return authHeader.slice(7) === adminToken ? "admin" : "invalid";
}

/**
 * 带有 Rate Limit 检查的管理员验证
 */
export async function verifyAdminWithRateLimit(
  authHeader: string | undefined,
  ip: string
): Promise<{
  success: boolean;
  error?: "blocked" | "invalid_token" | "no_token";
  retryAfter?: number;
}> {
  // 1. 检查是否被阻塞
  const blockStatus = await isBlocked(ip);
  if (blockStatus.blocked) {
    return {
      success: false,
      error: "blocked",
      retryAfter: blockStatus.retryAfter,
    };
  }

  // 2. 验证 token
  const isValid = verifyAdmin(authHeader);
  if (!isValid) {
    // 只有在提供了 token 但验证失败时才记录失败
    if (authHeader) {
      await recordFailure(ip);
    }
    return {
      success: false,
      error: authHeader ? "invalid_token" : "no_token",
    };
  }

  // 3. 成功后清除记录
  await clearRecord(ip);
  return { success: true };
}

/**
 * 从请求头获取客户端 IP
 * @param headerFn - Hono 的 c.req.header 函数
 */
export function getClientIp(headerFn: (name: string) => string | undefined): string {
  return (
    headerFn("x-forwarded-for")?.split(",")[0]?.trim() ||
    headerFn("x-real-ip") ||
    headerFn("cf-connecting-ip") ||
    "unknown"
  );
}
