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

