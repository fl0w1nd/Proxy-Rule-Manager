/**
 * Rate Limiter - 基于 IP 的指数退避阻塞算法
 *
 * 使用指数增长公式计算阻塞时间：
 * blockDuration = count^exponent + baseDelay (秒)
 */

import { checkBan, upsertBan, removeBan } from "../lib/ban-store";

// 内存中的临时失败计数（重启后清除）
const failureRecords = new Map<
  string,
  { count: number; lastFailedAt: number }
>();

// 配置
const CONFIG = {
  exponent: 2, // 指数
  baseDelay: 5, // 基础延迟（秒）
  maxBlockDuration: 3600, // 最大阻塞时间（1小时）
  cleanupInterval: 60 * 1000, // 清理间隔（1分钟）
  recordMaxAge: 24 * 60 * 60 * 1000, // 活跃记录保留时间（24小时）
  permanentBanThreshold: 10, // 达到此次数永久封禁
};

/**
 * 计算阻塞时间（秒）
 */
export function calculateBlockDuration(
  count: number,
  exponent = CONFIG.exponent,
  base = CONFIG.baseDelay
): number {
  const duration = Math.pow(count, exponent) + base;
  return Math.min(duration, CONFIG.maxBlockDuration);
}

/**
 * 检查 IP 是否被阻塞
 */
export async function isBlocked(
  ip: string
): Promise<{ blocked: boolean; retryAfter?: number; reason?: string }> {
  // 1. 检查永久封禁列表
  const banRecord = await checkBan(ip);
  if (banRecord) {
    if (banRecord.expiresAt === null) {
      // 永久封禁
      return { blocked: true, reason: banRecord.reason };
    }
    const expiresAt = new Date(banRecord.expiresAt).getTime();
    const now = Date.now();
    if (now < expiresAt) {
      return {
        blocked: true,
        retryAfter: Math.ceil((expiresAt - now) / 1000),
        reason: banRecord.reason,
      };
    }
    // 封禁已过期，移除记录
    await removeBan(ip);
  }

  // 2. 检查临时阻塞（基于失败次数）
  const record = failureRecords.get(ip);
  if (!record) {
    return { blocked: false };
  }

  const blockDuration = calculateBlockDuration(record.count);
  const blockedUntil = record.lastFailedAt + blockDuration * 1000;
  const now = Date.now();

  if (now < blockedUntil) {
    return {
      blocked: true,
      retryAfter: Math.ceil((blockedUntil - now) / 1000),
      reason: "too_many_attempts",
    };
  }

  return { blocked: false };
}

/**
 * 记录失败尝试
 */
export async function recordFailure(ip: string): Promise<void> {
  const now = Date.now();
  const existing = failureRecords.get(ip);

  const newCount = (existing?.count || 0) + 1;
  failureRecords.set(ip, {
    count: newCount,
    lastFailedAt: now,
  });

  // 如果失败次数达到阈值，永久封禁
  if (newCount >= CONFIG.permanentBanThreshold) {
    await upsertBan({
      ip,
      reason: "auto_permanent_ban_brute_force",
      bannedAt: new Date(now).toISOString(),
      expiresAt: null, // 永久封禁
      failCount: newCount,
    });
    // 从内存中移除，因为已经永久封禁了
    failureRecords.delete(ip);
  }
}

/**
 * 清除记录（登录成功后调用）
 */
export async function clearRecord(ip: string): Promise<void> {
  failureRecords.delete(ip);
}

/**
 * 获取 IP 的失败统计
 */
export function getFailureStats(
  ip: string
): { count: number; lastFailedAt: number } | null {
  return failureRecords.get(ip) || null;
}

/**
 * 获取所有失败记录（用于调试/管理）
 */
export function getAllFailureRecords(): Map<
  string,
  { count: number; lastFailedAt: number }
> {
  return new Map(failureRecords);
}

/**
 * 清理过期的内存记录（24小时后自动清除）
 */
function cleanupExpiredRecords(): void {
  const now = Date.now();

  for (const [ip, record] of failureRecords.entries()) {
    if (now - record.lastFailedAt > CONFIG.recordMaxAge) {
      failureRecords.delete(ip);
    }
  }
}

// 启动定期清理
setInterval(cleanupExpiredRecords, CONFIG.cleanupInterval);

// 导出配置供测试使用
export { CONFIG as RATE_LIMITER_CONFIG };
