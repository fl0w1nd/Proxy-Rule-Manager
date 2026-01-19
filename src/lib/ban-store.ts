/**
 * IP 封禁存储适配器
 *
 * 封禁数据持久化到 data/waf/bans.json
 */

import { promises as fs } from "node:fs";
import * as path from "node:path";

const DATA_DIR = path.join(process.cwd(), "data", "waf");
const BANS_FILE = path.join(DATA_DIR, "bans.json");

export interface BanRecord {
    ip: string;
    reason: string;
    bannedAt: string;
    expiresAt: string | null; // null = 永久封禁
    failCount: number;
}

interface BansData {
    bans: BanRecord[];
}

/**
 * 确保目录存在
 */
async function ensureDir(): Promise<void> {
    try {
        await fs.mkdir(DATA_DIR, { recursive: true });
    } catch {
        // 目录可能已存在
    }
}

/**
 * 读取封禁数据
 */
async function readBansData(): Promise<BansData> {
    try {
        const content = await fs.readFile(BANS_FILE, "utf-8");
        return JSON.parse(content);
    } catch {
        return { bans: [] };
    }
}

/**
 * 写入封禁数据
 */
async function writeBansData(data: BansData): Promise<void> {
    await ensureDir();
    await fs.writeFile(BANS_FILE, JSON.stringify(data, null, 2), "utf-8");
}

/**
 * 获取所有封禁记录
 */
export async function getAllBans(): Promise<BanRecord[]> {
    const data = await readBansData();
    return data.bans;
}

/**
 * 检查 IP 是否被封禁
 */
export async function checkBan(ip: string): Promise<BanRecord | null> {
    const data = await readBansData();
    return data.bans.find((b) => b.ip === ip) || null;
}

/**
 * 添加或更新封禁记录
 */
export async function upsertBan(record: BanRecord): Promise<void> {
    const data = await readBansData();
    const index = data.bans.findIndex((b) => b.ip === record.ip);

    if (index >= 0) {
        data.bans[index] = record;
    } else {
        data.bans.push(record);
    }

    await writeBansData(data);
}

/**
 * 移除封禁
 */
export async function removeBan(ip: string): Promise<boolean> {
    const data = await readBansData();
    const index = data.bans.findIndex((b) => b.ip === ip);

    if (index >= 0) {
        data.bans.splice(index, 1);
        await writeBansData(data);
        return true;
    }

    return false;
}

/**
 * 清理过期封禁
 */
export async function cleanupExpiredBans(): Promise<number> {
    const data = await readBansData();
    const now = new Date();
    const originalLength = data.bans.length;

    data.bans = data.bans.filter((ban) => {
        if (ban.expiresAt === null) return true; // 永久封禁保留
        return new Date(ban.expiresAt) > now;
    });

    if (data.bans.length < originalLength) {
        await writeBansData(data);
    }

    return originalLength - data.bans.length;
}

/**
 * 获取封禁统计
 */
export async function getBanStats(): Promise<{
    total: number;
    permanent: number;
    temporary: number;
}> {
    const bans = await getAllBans();
    const permanent = bans.filter((b) => b.expiresAt === null).length;

    return {
        total: bans.length,
        permanent,
        temporary: bans.length - permanent,
    };
}
