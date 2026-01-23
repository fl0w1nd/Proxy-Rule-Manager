/**
 * Storage Adapter - Local File System Implementation
 * Replaces Vercel Blob + Upstash Redis with local JSON and file storage.
 */

import * as fs from "node:fs/promises";
import * as path from "node:path";
import * as crypto from "node:crypto";
import {
    RulesConfig,
    ArtifactMeta,
    JobRecord,
    DailyStats,
    DEFAULT_CONFIG,
    validateConfig,
    ClientType,
    CLIENT_PATH_NAMES,
    ClientConfig,
    DEFAULT_CLIENTS,
    updateClientMappings,
    SyncSchedule,
    DEFAULT_SYNC_SCHEDULE,
    ClientFileMeta,
} from "./schema";
import { normalizeSyncSchedule } from "./sync-schedule";
import {
    getDataDir as getDataDirPath,
    getDbFilePath,
    getRulesDir as getRulesDirPath,
    getSourcesDir,
} from "./data-paths";
import {
    externalizeConfigLocalSources,
    hydrateConfigLocalSources,
    pruneLocalSources,
} from "./local-source-store";

// --- Configuration ---
const DATA_DIR = getDataDirPath();
const RULES_DIR = getRulesDirPath();
const RECORDS_DIR = path.join(DATA_DIR, "records");
const SOURCES_DIR = getSourcesDir();
const DB_FILE = getDbFilePath();
const RESERVED_CLIENT_DIRS = new Set(["rules", "sources", "db.json"]);

// --- Database Schema ---
interface Database {
    config: RulesConfig;
    configRev: number;
    clients: ClientConfig[]; // 动态客户端列表
    clientFiles: Record<string, ClientFileMeta>; // key: fileId
    artifacts: Record<string, ArtifactMeta>; // key: "ruleName:client"
    jobs: Record<string, JobRecord>;
    dailyStats: Record<string, DailyStats>;
    locks: Record<string, number>; // key -> expireTimestamp
    lastSyncInfo: LastSyncInfo;
    syncSchedule: SyncSchedule; // 定时同步配置
}

export interface LastSyncInfo {
    lastFullSyncAt: string | null;
    lastPartialSyncAt: string | null;
    lastSuccessfulSyncAt: string | null;
    totalRulesCount: number;
    changedRulesCount: number;
    failedRulesCount: number;
}

const DEFAULT_DB: Database = {
    config: DEFAULT_CONFIG,
    configRev: 0,
    clients: DEFAULT_CLIENTS,
    clientFiles: {},
    artifacts: {},
    jobs: {},
    dailyStats: {},
    locks: {},
    lastSyncInfo: {
        lastFullSyncAt: null,
        lastPartialSyncAt: null,
        lastSuccessfulSyncAt: null,
        totalRulesCount: 0,
        changedRulesCount: 0,
        failedRulesCount: 0,
    },
    syncSchedule: DEFAULT_SYNC_SCHEDULE,
};

// --- Database Operations (Simple JSON file) ---
let dbCache: Database | null = null;

async function ensureDataDir(): Promise<void> {
    await fs.mkdir(DATA_DIR, { recursive: true });
    await fs.mkdir(RULES_DIR, { recursive: true });
}

async function atomicWriteFile(filePath: string, content: string): Promise<void> {
    const dir = path.dirname(filePath);
    const base = path.basename(filePath);
    const tempPath = path.join(dir, `.${base}.${process.pid}.${Date.now()}.tmp`);

    await fs.writeFile(tempPath, content, "utf-8");

    try {
        await fs.rename(tempPath, filePath);
    } catch (err: unknown) {
        const code = err && typeof err === "object" && "code" in err ? err.code : null;
        if (code === "EXDEV" || code === "EPERM" || code === "EEXIST") {
            await fs.unlink(filePath).catch(() => undefined);
            await fs.rename(tempPath, filePath);
            return;
        }
        await fs.unlink(tempPath).catch(() => undefined);
        throw err;
    }
}

async function loadDb(): Promise<Database> {
    if (dbCache) return dbCache;

    try {
        await ensureDataDir();
        const content = await fs.readFile(DB_FILE, "utf-8");
        dbCache = JSON.parse(content) as Database;
        // 初始化客户端映射
        updateClientMappings(dbCache.clients);
        if (hasInlineLocalSources(dbCache.config)) {
            const { config: externalized, refs } = await externalizeConfigLocalSources(dbCache.config);
            await pruneLocalSources(refs);
            dbCache.config = externalized;
            await saveDb(dbCache);
        }
        return dbCache;
    } catch {
        // File doesn't exist or is invalid, use default
        dbCache = { ...DEFAULT_DB };
        updateClientMappings(dbCache.clients);
        return dbCache;
    }
}

function hasInlineLocalSources(config: RulesConfig): boolean {
    for (const rule of config.rules) {
        if (!rule.sources) continue;
        for (const source of rule.sources) {
            if (source.type === "local" && typeof source.content === "string") {
                return true;
            }
        }
    }
    return false;
}

async function saveDb(db: Database): Promise<void> {
    await ensureDataDir();
    dbCache = db;
    await atomicWriteFile(DB_FILE, JSON.stringify(db, null, 2));
}


// --- Config Management ---
export async function getConfig(): Promise<RulesConfig> {
    const db = await loadDb();
    try {
        const validated = validateConfig(db.config);
        return await hydrateConfigLocalSources(validated);
    } catch {
        return DEFAULT_CONFIG;
    }
}

export async function getConfigRaw(): Promise<RulesConfig> {
    const db = await loadDb();
    try {
        return validateConfig(db.config);
    } catch {
        return DEFAULT_CONFIG;
    }
}

export async function saveConfig(config: RulesConfig): Promise<{ rev: number }> {
    const db = await loadDb();
    const validated = validateConfig(config);
    const { config: externalized, refs } = await externalizeConfigLocalSources(validated);
    await pruneLocalSources(refs);
    db.config = externalized;
    db.configRev += 1;
    await saveDb(db);
    return { rev: db.configRev };
}

export async function resetDatabaseWithConfig(
    config: RulesConfig,
    clients: ClientConfig[] = DEFAULT_CLIENTS
): Promise<{ rev: number }> {
    const validated = validateConfig(config);
    await ensureDataDir();
    await Promise.all([
        fs.rm(RULES_DIR, { recursive: true, force: true }),
        fs.rm(RECORDS_DIR, { recursive: true, force: true }),
        fs.rm(SOURCES_DIR, { recursive: true, force: true }),
    ]);

    const { config: externalized, refs } = await externalizeConfigLocalSources(validated);
    await pruneLocalSources(refs);

    const newDb: Database = {
        ...DEFAULT_DB,
        config: externalized,
        configRev: 1,
        clients,
    };

    dbCache = newDb;
    updateClientMappings(newDb.clients);
    await saveDb(newDb);
    return { rev: newDb.configRev };
}

export async function getConfigRev(): Promise<number> {
    const db = await loadDb();
    return db.configRev;
}

// --- Client Management ---
export async function getClients(): Promise<ClientConfig[]> {
    const db = await loadDb();
    return db.clients;
}

export async function saveClients(clients: ClientConfig[]): Promise<void> {
    const db = await loadDb();
    db.clients = clients;
    updateClientMappings(clients);
    await saveDb(db);
}

export async function addClient(client: ClientConfig): Promise<void> {
    const db = await loadDb();
    if (RESERVED_CLIENT_DIRS.has(client.id)) {
        throw new Error(`Client id "${client.id}" is reserved`);
    }
    if (db.clients.find(c => c.id === client.id)) {
        throw new Error(`Client with id "${client.id}" already exists`);
    }
    db.clients.push(client);
    updateClientMappings(db.clients);
    await saveDb(db);
}

export async function updateClient(
    clientId: string,
    updates: Partial<ClientConfig>
): Promise<{ renamedPath?: { from: string; to: string } }> {
    const db = await loadDb();
    const index = db.clients.findIndex(c => c.id === clientId);
    if (index === -1) {
        throw new Error(`Client "${clientId}" not found`);
    }

    const oldClient = db.clients[index];
    const newClient = { ...oldClient, ...updates };

    let renamedPath: { from: string; to: string } | undefined;

    // 如果 pathName 变更，需要重命名目录
    if (updates.pathName && updates.pathName !== oldClient.pathName) {
        const oldDir = path.join(RULES_DIR, oldClient.pathName);
        const newDir = path.join(RULES_DIR, updates.pathName);

        try {
            await fs.access(oldDir);
            await fs.rename(oldDir, newDir);
            renamedPath = { from: oldClient.pathName, to: updates.pathName };

            // 更新 artifacts 中的路径
            for (const key of Object.keys(db.artifacts)) {
                const artifact = db.artifacts[key];
                if (artifact.client === clientId) {
                    artifact.blobPath = artifact.blobPath.replace(
                        `/Rules/${oldClient.pathName}/`,
                        `/Rules/${updates.pathName}/`
                    );
                }
            }
        } catch (err: unknown) {
            // 只忽略目录不存在的情况，其他错误需要抛出
            const isNotFound = err && typeof err === "object" && "code" in err && err.code === "ENOENT";
            if (!isNotFound) {
                throw new Error(`重命名目录失败: ${err}`);
            }
            // 目录不存在，不需要重命名，也无需更新 artifacts 路径
        }
    }

    // 如果 id 变更，需要更新所有引用
    if (updates.id && updates.id !== clientId) {
        // 检查新 ID 是否已存在
        if (db.clients.find(c => c.id === updates.id)) {
            throw new Error(`Client with id "${updates.id}" already exists`);
        }
        if (RESERVED_CLIENT_DIRS.has(updates.id)) {
            throw new Error(`Client id "${updates.id}" is reserved`);
        }

        // 更新 artifacts 的 key 和 client 字段
        const newArtifacts: Record<string, ArtifactMeta> = {};
        for (const [key, artifact] of Object.entries(db.artifacts)) {
            if (artifact.client === clientId) {
                const newKey = key.replace(`:${clientId}`, `:${updates.id}`);
                newArtifacts[newKey] = { ...artifact, client: updates.id };
            } else {
                newArtifacts[key] = artifact;
            }
        }
        db.artifacts = newArtifacts;

        // 更新客户端配置文件的 clientId 和目录
        try {
            const oldDir = getClientFilesDir(clientId);
            const newDir = getClientFilesDir(updates.id);
            await fs.access(oldDir);
            await fs.rename(oldDir, newDir);
        } catch (err: unknown) {
            const isNotFound = err && typeof err === "object" && "code" in err && err.code === "ENOENT";
            if (!isNotFound) {
                throw new Error(`重命名客户端文件目录失败: ${err}`);
            }
        }

        for (const file of Object.values(db.clientFiles)) {
            if (file.clientId === clientId) {
                file.clientId = updates.id;
            }
        }

        // 更新配置中的 clients 引用和 client_overrides
        for (const rule of db.config.rules) {
            const clientIndex = rule.output.clients.indexOf(clientId);
            if (clientIndex !== -1) {
                rule.output.clients[clientIndex] = updates.id;
            }
            // 迁移 client_overrides 的键名
            if (rule.output.client_overrides?.[clientId]) {
                rule.output.client_overrides[updates.id] = rule.output.client_overrides[clientId];
                delete rule.output.client_overrides[clientId];
            }
        }
    }

    db.clients[index] = newClient;
    updateClientMappings(db.clients);
    await saveDb(db);

    return { renamedPath };
}

export async function deleteClient(clientId: string): Promise<void> {
    const db = await loadDb();
    const index = db.clients.findIndex(c => c.id === clientId);
    if (index === -1) {
        throw new Error(`Client "${clientId}" not found`);
    }

    const client = db.clients[index];

    // 删除对应目录
    const dir = path.join(RULES_DIR, client.pathName);
    try {
        await fs.rm(dir, { recursive: true });
    } catch {
        // 目录不存在，忽略
    }

    // 删除相关 artifacts
    const keysToDelete = Object.keys(db.artifacts).filter(k => k.endsWith(`:${clientId}`));
    for (const key of keysToDelete) {
        delete db.artifacts[key];
    }

    // 删除客户端配置文件元数据与文件
    const clientFileIds = Object.keys(db.clientFiles).filter(
        (id) => db.clientFiles[id]?.clientId === clientId
    );
    for (const id of clientFileIds) {
        const file = db.clientFiles[id];
        if (file) {
            const filePath = getClientFilePath(file.clientId, file.name, file.ext);
            await fs.unlink(filePath).catch(() => undefined);
            delete db.clientFiles[id];
        }
    }
    try {
        const clientFilesDir = getClientFilesDir(clientId);
        if (clientFilesDir !== RULES_DIR && clientFilesDir !== SOURCES_DIR && clientFilesDir !== DATA_DIR) {
            await fs.rm(clientFilesDir, { recursive: true, force: true });
        }
    } catch {
        // ignore
    }

    // 从配置中移除该客户端引用和 client_overrides
    for (const rule of db.config.rules) {
        rule.output.clients = rule.output.clients.filter(c => c !== clientId);
        // 清理 client_overrides 中的配置
        if (rule.output.client_overrides?.[clientId]) {
            delete rule.output.client_overrides[clientId];
        }
    }

    db.clients.splice(index, 1);
    updateClientMappings(db.clients);
    await saveDb(db);
}

// --- Client File Management ---

function getClientFilesDir(clientId: string): string {
    if (RESERVED_CLIENT_DIRS.has(clientId)) {
        throw new Error(`Client id "${clientId}" is reserved and cannot store files`);
    }
    return path.join(DATA_DIR, clientId);
}

function getClientFilePath(clientId: string, name: string, ext: string): string {
    if (name.includes("/") || name.includes("\\") || name.includes("..")) {
        throw new Error("Invalid file name");
    }
    if (ext.includes("/") || ext.includes("\\") || ext.includes("..")) {
        throw new Error("Invalid file extension");
    }
    const safeName = `${name}.${ext}`;
    return path.join(getClientFilesDir(clientId), safeName);
}

async function ensureClientFilesDir(clientId: string): Promise<string> {
    const dir = getClientFilesDir(clientId);
    await fs.mkdir(dir, { recursive: true });
    return dir;
}

function findClientFileByName(db: Database, clientId: string, name: string, ext: string): ClientFileMeta | undefined {
    return Object.values(db.clientFiles).find(
        (file) => file.clientId === clientId && file.name === name && file.ext === ext
    );
}

export async function listClientFiles(clientId: string): Promise<ClientFileMeta[]> {
    const db = await loadDb();
    return Object.values(db.clientFiles).filter((file) => file.clientId === clientId);
}

export async function listPublicClientFiles(): Promise<ClientFileMeta[]> {
    const db = await loadDb();
    return Object.values(db.clientFiles).filter((file) => file.isPublic);
}

export async function getClientFileMeta(fileId: string): Promise<ClientFileMeta | null> {
    const db = await loadDb();
    return db.clientFiles[fileId] || null;
}

export async function getClientFileContent(fileId: string): Promise<string | null> {
    const db = await loadDb();
    const meta = db.clientFiles[fileId];
    if (!meta) return null;
    try {
        const filePath = getClientFilePath(meta.clientId, meta.name, meta.ext);
        return await fs.readFile(filePath, "utf-8");
    } catch {
        return null;
    }
}

export async function createClientFile(
    clientId: string,
    input: { name: string; ext: string; isPublic: boolean; content: string }
): Promise<ClientFileMeta> {
    const db = await loadDb();
    if (findClientFileByName(db, clientId, input.name, input.ext)) {
        throw new Error(`File "${input.name}.${input.ext}" already exists`);
    }
    await ensureClientFilesDir(clientId);
    const filePath = getClientFilePath(clientId, input.name, input.ext);
    await fs.writeFile(filePath, input.content ?? "", "utf-8");

    const now = new Date().toISOString();
    const meta: ClientFileMeta = {
        id: crypto.randomUUID(),
        clientId,
        name: input.name,
        ext: input.ext,
        isPublic: !!input.isPublic,
        createdAt: now,
        updatedAt: now,
    };
    db.clientFiles[meta.id] = meta;
    await saveDb(db);
    return meta;
}

export async function updateClientFile(
    fileId: string,
    updates: Partial<{ name: string; ext: string; isPublic: boolean; content: string }>
): Promise<ClientFileMeta> {
    const db = await loadDb();
    const meta = db.clientFiles[fileId];
    if (!meta) {
        throw new Error(`Client file "${fileId}" not found`);
    }

    const nextName = updates.name ?? meta.name;
    const nextExt = updates.ext ?? meta.ext;
    const nameChanged = nextName !== meta.name || nextExt !== meta.ext;

    if (nameChanged) {
        const existing = findClientFileByName(db, meta.clientId, nextName, nextExt);
        if (existing && existing.id !== fileId) {
            throw new Error(`File "${nextName}.${nextExt}" already exists`);
        }
    }

    const oldPath = getClientFilePath(meta.clientId, meta.name, meta.ext);
    const newPath = getClientFilePath(meta.clientId, nextName, nextExt);

    if (nameChanged) {
        await ensureClientFilesDir(meta.clientId);
        try {
            await fs.rename(oldPath, newPath);
        } catch {
            // 如果旧文件不存在，尝试直接写入新路径
            await fs.writeFile(newPath, updates.content ?? "", "utf-8");
        }
    }

    if (typeof updates.content === "string") {
        await ensureClientFilesDir(meta.clientId);
        await fs.writeFile(newPath, updates.content, "utf-8");
    }

    const updated: ClientFileMeta = {
        ...meta,
        name: nextName,
        ext: nextExt,
        isPublic: typeof updates.isPublic === "boolean" ? updates.isPublic : meta.isPublic,
        updatedAt: new Date().toISOString(),
    };

    db.clientFiles[fileId] = updated;
    await saveDb(db);
    return updated;
}

export async function deleteClientFile(fileId: string): Promise<void> {
    const db = await loadDb();
    const meta = db.clientFiles[fileId];
    if (!meta) {
        throw new Error(`Client file "${fileId}" not found`);
    }
    const filePath = getClientFilePath(meta.clientId, meta.name, meta.ext);
    try {
        await fs.unlink(filePath);
    } catch {
        // ignore
    }
    delete db.clientFiles[fileId];
    await saveDb(db);
}

export async function getPublicClientFile(
    clientId: string,
    name: string,
    ext: string
): Promise<{ meta: ClientFileMeta; content: string } | null> {
    const db = await loadDb();
    const meta = findClientFileByName(db, clientId, name, ext);
    if (!meta || !meta.isPublic) return null;
    try {
        const filePath = getClientFilePath(clientId, name, ext);
        const content = await fs.readFile(filePath, "utf-8");
        return { meta, content };
    } catch {
        return null;
    }
}

// --- Artifact Metadata ---
function artifactKey(ruleName: string, client: ClientType): string {
    return `${ruleName}:${client}`;
}

export async function getArtifactMeta(
    ruleName: string,
    client: ClientType
): Promise<ArtifactMeta | null> {
    const db = await loadDb();
    return db.artifacts[artifactKey(ruleName, client)] || null;
}

export async function saveArtifactMeta(meta: ArtifactMeta): Promise<void> {
    const db = await loadDb();
    db.artifacts[artifactKey(meta.ruleName, meta.client)] = meta;
    await saveDb(db);
}

export async function deleteArtifactMeta(
    ruleName: string,
    client: ClientType
): Promise<void> {
    const db = await loadDb();
    delete db.artifacts[artifactKey(ruleName, client)];
    await saveDb(db);
}

export async function getAllArtifactMetas(): Promise<ArtifactMeta[]> {
    const db = await loadDb();
    return Object.values(db.artifacts);
}

// --- Rule Rename ---
export async function renameRule(
    oldName: string,
    newName: string
): Promise<{ renamedFiles: string[] }> {
    const db = await loadDb();

    // 查找规则
    const ruleIndex = db.config.rules.findIndex(r => r.name === oldName);
    if (ruleIndex === -1) {
        throw new Error(`Rule "${oldName}" not found`);
    }

    // 检查新名称是否已存在
    if (db.config.rules.find(r => r.name === newName)) {
        throw new Error(`Rule "${newName}" already exists`);
    }

    const rule = db.config.rules[ruleIndex];
    const renamedFiles: string[] = [];

    // 重命名每个客户端的规则文件
    for (const clientId of rule.output.clients) {
        const clientPathName = CLIENT_PATH_NAMES[clientId];
        if (!clientPathName) continue;

        const oldFilePath = path.join(RULES_DIR, clientPathName, `${oldName}.list`);
        const newFilePath = path.join(RULES_DIR, clientPathName, `${newName}.list`);

        try {
            await fs.access(oldFilePath);
            await fs.rename(oldFilePath, newFilePath);
            renamedFiles.push(`${clientPathName}/${newName}.list`);
        } catch {
            // 文件不存在，跳过
        }

        // 更新 artifact metadata
        const oldKey = artifactKey(oldName, clientId);
        const newKey = artifactKey(newName, clientId);
        if (db.artifacts[oldKey]) {
            const artifact = db.artifacts[oldKey];
            artifact.ruleName = newName;
            artifact.blobPath = artifact.blobPath.replace(`/${oldName}.list`, `/${newName}.list`);
            db.artifacts[newKey] = artifact;
            delete db.artifacts[oldKey];
        }
    }

    // 更新规则配置
    db.config.rules[ruleIndex].name = newName;

    // 更新其他规则中对该规则的引用 (ref)
    for (const r of db.config.rules) {
        if (r.sources) {
            for (const source of r.sources) {
                if (source.ref === oldName) {
                    source.ref = newName;
                }
            }
        }
    }

    await saveDb(db);
    return { renamedFiles };
}

// --- Lock Management (In-memory with persistence) ---
const LOCK_TTL_MS = 5 * 60 * 1000; // 5 minutes

async function cleanupExpiredLocks(): Promise<void> {
    const db = await loadDb();
    const now = Date.now();
    let changed = false;
    for (const key of Object.keys(db.locks)) {
        if (db.locks[key] < now) {
            delete db.locks[key];
            changed = true;
        }
    }
    if (changed) await saveDb(db);
}

export async function acquireLock(lockKey: string): Promise<boolean> {
    await cleanupExpiredLocks();
    const db = await loadDb();
    const now = Date.now();

    if (db.locks[lockKey] && db.locks[lockKey] > now) {
        return false; // Lock exists and not expired
    }

    db.locks[lockKey] = now + LOCK_TTL_MS;
    await saveDb(db);
    return true;
}

export async function releaseLock(lockKey: string): Promise<void> {
    const db = await loadDb();
    delete db.locks[lockKey];
    await saveDb(db);
}

export async function isLocked(lockKey: string): Promise<boolean> {
    await cleanupExpiredLocks();
    const db = await loadDb();
    return !!db.locks[lockKey] && db.locks[lockKey] > Date.now();
}

// Global sync lock helpers
export async function isGlobalSyncLocked(): Promise<boolean> {
    return isLocked("sync:global");
}

export async function hasActiveRuleLocks(): Promise<boolean> {
    await cleanupExpiredLocks();
    const db = await loadDb();
    const now = Date.now();
    for (const key of Object.keys(db.locks)) {
        if (key.startsWith("rule:") && db.locks[key] > now) {
            return true;
        }
    }
    return false;
}

export async function acquireRuleLock(
    ruleName: string
): Promise<{ acquired: boolean; reason?: string }> {
    if (await isGlobalSyncLocked()) {
        return { acquired: false, reason: "Global sync is in progress" };
    }

    const acquired = await acquireLock(`rule:${ruleName}`);
    if (!acquired) {
        return { acquired: false, reason: "Rule is already being processed" };
    }

    // Double check global lock
    if (await isGlobalSyncLocked()) {
        await releaseLock(`rule:${ruleName}`);
        return { acquired: false, reason: "Global sync started, please retry" };
    }

    return { acquired: true };
}

export async function releaseRuleLock(ruleName: string): Promise<void> {
    await releaseLock(`rule:${ruleName}`);
}

export async function acquireGlobalSyncLock(): Promise<{
    acquired: boolean;
    reason?: string;
}> {
    if (await hasActiveRuleLocks()) {
        return { acquired: false, reason: "Partial sync is in progress" };
    }

    const acquired = await acquireLock("sync:global");
    if (!acquired) {
        return { acquired: false, reason: "Another sync is already running" };
    }

    // Double check rule locks
    if (await hasActiveRuleLocks()) {
        await releaseLock("sync:global");
        return { acquired: false, reason: "Partial sync started, please retry" };
    }

    return { acquired: true };
}

export async function releaseGlobalSyncLock(): Promise<void> {
    await releaseLock("sync:global");
}

// --- Job Management ---
export async function createJob(
    type: "full_sync" | "partial_sync",
    affectedRules?: string[]
): Promise<JobRecord> {
    const db = await loadDb();
    const jobId = `job_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
    const job: JobRecord = {
        jobId,
        type,
        status: "running",
        startedAt: new Date().toISOString(),
        affectedRules,
        logs: [],
    };
    db.jobs[jobId] = job;
    await saveDb(db);
    return job;
}

export async function updateJob(job: JobRecord): Promise<void> {
    const db = await loadDb();
    db.jobs[job.jobId] = job;
    await saveDb(db);
}

export async function getJob(jobId: string): Promise<JobRecord | null> {
    const db = await loadDb();
    return db.jobs[jobId] || null;
}

export async function completeJob(
    jobId: string,
    changedRules: string[],
    failedRules: { name: string; error: string }[]
): Promise<void> {
    const db = await loadDb();
    const job = db.jobs[jobId];
    if (job) {
        job.status = failedRules.length > 0 ? "failed" : "completed";
        job.completedAt = new Date().toISOString();
        job.changedRules = changedRules;
        job.failedRules = failedRules;
        await saveDb(db);
    }
}

// --- Daily Stats ---
export async function getDailyStats(date: string): Promise<DailyStats> {
    const db = await loadDb();
    const stats = db.dailyStats[date];
    if (!stats) {
        return {
            date,
            syncCount: 0,
            blobWriteCount: 0,
            rulesChanged: 0,
            totalRulesProcessed: 0,
            failedSources: 0,
        };
    }
    return {
        date: stats.date,
        syncCount: stats.syncCount || 0,
        blobWriteCount: stats.blobWriteCount || 0,
        rulesChanged: stats.rulesChanged || 0,
        totalRulesProcessed: stats.totalRulesProcessed || 0,
        failedSources: stats.failedSources || 0,
    };
}

export async function incrementDailyStats(
    date: string,
    updates: Partial<Omit<DailyStats, "date">>
): Promise<void> {
    const db = await loadDb();
    const stats = db.dailyStats[date] || {
        date,
        syncCount: 0,
        blobWriteCount: 0,
        rulesChanged: 0,
        totalRulesProcessed: 0,
        failedSources: 0,
    };

    if (updates.syncCount) stats.syncCount += updates.syncCount;
    if (updates.blobWriteCount) stats.blobWriteCount += updates.blobWriteCount;
    if (updates.rulesChanged) stats.rulesChanged += updates.rulesChanged;
    if (updates.totalRulesProcessed)
        stats.totalRulesProcessed += updates.totalRulesProcessed;
    if (updates.failedSources) stats.failedSources += updates.failedSources;

    db.dailyStats[date] = stats;
    await saveDb(db);
}
// --- Last Sync Info ---
export async function getLastSyncInfo(): Promise<LastSyncInfo> {
    const db = await loadDb();
    return db.lastSyncInfo;
}

export async function updateLastSyncInfo(
    updates: Partial<LastSyncInfo>
): Promise<void> {
    const db = await loadDb();
    db.lastSyncInfo = { ...db.lastSyncInfo, ...updates };
    await saveDb(db);
}

// --- Sync Schedule ---
export async function getSyncSchedule(): Promise<SyncSchedule> {
    const db = await loadDb();
    if (!db.syncSchedule) {
        throw new Error("Sync schedule missing in database");
    }
    return normalizeSyncSchedule(db.syncSchedule);
}

export async function updateSyncSchedule(
    updates: Partial<SyncSchedule>
): Promise<void> {
    const db = await loadDb();
    if (!db.syncSchedule) {
        throw new Error("Sync schedule missing in database");
    }
    const current = normalizeSyncSchedule(db.syncSchedule);
    db.syncSchedule = normalizeSyncSchedule({ ...current, ...updates });
    await saveDb(db);
}

// --- Rule File Storage (replaces Vercel Blob) ---
export function getRuleFilePath(ruleName: string, client: ClientType): string {
    const clientPath = CLIENT_PATH_NAMES[client];
    return path.join(RULES_DIR, clientPath, `${ruleName}.list`);
}

export function getRulePublicPath(ruleName: string, client: ClientType): string {
    const clientPath = CLIENT_PATH_NAMES[client];
    return `/Rules/${clientPath}/${ruleName}.list`;
}

export async function uploadRuleContent(
    ruleName: string,
    client: ClientType,
    content: string
): Promise<{ url: string; path: string }> {
    const filePath = getRuleFilePath(ruleName, client);
    const dir = path.dirname(filePath);
    await fs.mkdir(dir, { recursive: true });
    await atomicWriteFile(filePath, content);

    const publicPath = getRulePublicPath(ruleName, client);
    return {
        url: publicPath, // Local path serves as URL
        path: publicPath,
    };
}

export async function deleteRuleContent(
    ruleName: string,
    client: ClientType
): Promise<void> {
    const filePath = getRuleFilePath(ruleName, client);
    try {
        await fs.unlink(filePath);
    } catch {
        // File doesn't exist, ignore
    }
}

export async function getRuleContent(
    ruleName: string,
    client: ClientType
): Promise<string | null> {
    const filePath = getRuleFilePath(ruleName, client);
    try {
        return await fs.readFile(filePath, "utf-8");
    } catch {
        return null;
    }
}

export async function listAllRules(): Promise<
    { ruleName: string; client: ClientType; url: string }[]
> {
    const result: { ruleName: string; client: ClientType; url: string }[] = [];

    for (const [client, clientPath] of Object.entries(CLIENT_PATH_NAMES)) {
        const clientDir = path.join(RULES_DIR, clientPath);
        try {
            const files = await fs.readdir(clientDir);
            for (const file of files) {
                if (file.endsWith(".list")) {
                    const ruleName = file.replace(".list", "");
                    result.push({
                        ruleName,
                        client: client as ClientType,
                        url: getRulePublicPath(ruleName, client as ClientType),
                    });
                }
            }
        } catch {
            // Directory doesn't exist, skip
        }
    }

    return result;
}

// --- Utility ---
export function getDataDir(): string {
    return DATA_DIR;
}

export function getRulesDir(): string {
    return RULES_DIR;
}

// Invalidate cache (useful for tests)
export function invalidateCache(): void {
    dbCache = null;
}
