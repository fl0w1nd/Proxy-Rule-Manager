import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

/** 生成一个随机的列表项 key，用于 React key 管理 */
export function createListItemKey(): string {
  return Math.random().toString(36).slice(2, 10);
}

/** 批量生成列表项 keys */
export function createListItemKeys(count: number): string[] {
  return Array.from({ length: count }, () => createListItemKey());
}

/** 将 ISO 时间字符串格式化为本地时间 */
export function formatTimestamp(value: string): string {
  return new Date(value).toLocaleString("zh-CN");
}

/** 将字节数格式化为可读字符串 */
export function formatBytes(value?: number): string {
  if (!value && value !== 0) return "-";
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / (1024 * 1024)).toFixed(1)} MB`;
}

/** 将 ISO 时间字符串格式化为相对时间（中文） */
export function formatRelativeTime(value: string): string {
  const now = Date.now();
  const then = new Date(value).getTime();
  const diff = now - then;
  if (diff < 0) return "刚刚";
  const seconds = Math.floor(diff / 1000);
  if (seconds < 60) return "刚刚";
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes} 分钟前`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} 小时前`;
  const days = Math.floor(hours / 24);
  if (days <= 7) return `${days} 天前`;
  return formatTimestamp(value);
}

/** 把未来时间格式化为"N 分钟/小时/天 后"。已过的时间统一返回"即将"。 */
export function formatTimeUntil(value: string): string {
  const diff = new Date(value).getTime() - Date.now();
  if (diff <= 60_000) return "即将";
  const minutes = Math.floor(diff / 60_000);
  if (minutes < 60) return `${minutes} 分钟后`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} 小时后`;
  const days = Math.floor(hours / 24);
  if (days <= 30) return `${days} 天后`;
  return formatTimestamp(value);
}

/** 把毫秒数格式化成可读时长，UI 用，输入为空返回 "-"。 */
export function formatDurationMs(ms?: number | null): string {
  if (ms == null || !Number.isFinite(ms) || ms < 0) return "-";
  if (ms < 1000) return `${Math.round(ms)} ms`;
  const s = ms / 1000;
  if (s < 60) return `${s.toFixed(1)} s`;
  const totalSec = Math.round(s);
  const m = Math.floor(totalSec / 60);
  const rs = totalSec - m * 60;
  return `${m}m ${rs}s`;
}

