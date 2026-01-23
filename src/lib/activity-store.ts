import * as fs from "node:fs/promises";
import * as path from "node:path";
import { ClientType } from "./schema";
import { getDataDir } from "./data-paths";

const RECORDS_DIR = path.join(getDataDir(), "records");
const CHANGES_DIR = path.join(RECORDS_DIR, "changes");
const FAILURES_DIR = path.join(RECORDS_DIR, "failures");
const RETENTION_DAYS = 7;

export interface ChangeRecordMeta {
  id: string;
  timestamp: string;
  ruleName: string;
  client: ClientType;
  changeType: "created" | "updated" | "deleted";
  sizeBytes?: number;
}

export interface ChangeRecordInput extends ChangeRecordMeta {
  diff: string;
}

export interface ChangeRecordSummary extends ChangeRecordMeta {
  date: string;
  fileName: string;
}

export interface FailureRecord {
  id: string;
  timestamp: string;
  ruleName: string;
  client?: ClientType;
  source?: string;
  message: string;
  stage: string;
  jobId?: string;
}

export interface ActivityList<T> {
  items: T[];
  total: number;
  page: number;
  pageSize: number;
}

function formatDateKey(date: Date = new Date()): string {
  return date.toISOString().split("T")[0];
}

function getRecentDateKeys(days: number = RETENTION_DAYS): string[] {
  const dates: string[] = [];
  for (let i = 0; i < days; i++) {
    const date = new Date();
    date.setDate(date.getDate() - i);
    dates.push(formatDateKey(date));
  }
  return dates;
}

async function listAvailableDateDirs(baseDir: string): Promise<string[]> {
  await pruneOldDirs(baseDir);
  try {
    const entries = await fs.readdir(baseDir, { withFileTypes: true });
    return entries
      .filter((entry) => entry.isDirectory() && /^\d{4}-\d{2}-\d{2}$/.test(entry.name))
      .map((entry) => entry.name);
  } catch {
    return [];
  }
}

async function ensureDir(dirPath: string): Promise<void> {
  await fs.mkdir(dirPath, { recursive: true });
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

async function pruneOldDirs(baseDir: string): Promise<void> {
  await ensureDir(baseDir);
  const cutoff = new Date();
  cutoff.setDate(cutoff.getDate() - (RETENTION_DAYS - 1));
  const cutoffKey = formatDateKey(cutoff);
  const entries = await fs.readdir(baseDir, { withFileTypes: true });
  await Promise.all(
    entries.map(async (entry) => {
      if (!entry.isDirectory()) return;
      if (entry.name < cutoffKey) {
        await fs.rm(path.join(baseDir, entry.name), { recursive: true, force: true });
      }
    })
  );
}

function buildChangeFileName(meta: ChangeRecordMeta, timestampMs: number): string {
  const rule = encodeURIComponent(meta.ruleName);
  const client = encodeURIComponent(meta.client);
  const size = typeof meta.sizeBytes === "number" ? meta.sizeBytes : 0;
  return `${timestampMs}@${rule}@${client}@${meta.changeType}@${size}@${meta.id}.diff`;
}

function parseChangeFileName(
  fileName: string,
  date: string
): ChangeRecordSummary | null {
  if (!fileName.endsWith(".diff")) return null;
  const baseName = fileName.slice(0, -".diff".length);
  const parts = baseName.split("@");
  if (parts.length !== 6) return null;

  const [timestampMs, ruleEncoded, clientEncoded, changeType, sizeStr, id] = parts;
  if (!["created", "updated", "deleted"].includes(changeType)) return null;
  const timestampNumber = Number(timestampMs);
  if (!Number.isFinite(timestampNumber)) return null;

  let ruleName: string;
  let client: string;
  try {
    ruleName = decodeURIComponent(ruleEncoded);
    client = decodeURIComponent(clientEncoded);
  } catch {
    return null;
  }

  const sizeBytes = Number(sizeStr);
  const timestamp = new Date(timestampNumber).toISOString();

  return {
    id,
    timestamp,
    ruleName,
    client: client as ClientType,
    changeType: changeType as ChangeRecordMeta["changeType"],
    sizeBytes: Number.isFinite(sizeBytes) ? sizeBytes : undefined,
    date,
    fileName,
  };
}

function isSafeFileName(fileName: string, extension: string): boolean {
  return (
    fileName.endsWith(extension) &&
    !fileName.includes("/") &&
    !fileName.includes("\\")
  );
}

export async function recordRuleFileChanges(
  changes: ChangeRecordInput[]
): Promise<void> {
  if (changes.length === 0) return;
  await ensureDir(CHANGES_DIR);
  await pruneOldDirs(CHANGES_DIR);

  for (const change of changes) {
    const dateKey = change.timestamp.split("T")[0];
    const dirPath = path.join(CHANGES_DIR, dateKey);
    await ensureDir(dirPath);

    const timestampMs = Date.parse(change.timestamp) || Date.now();
    const fileName = buildChangeFileName(change, timestampMs);
    const filePath = path.join(dirPath, fileName);
    await atomicWriteFile(filePath, change.diff);
  }
}

export async function recordFailureRecords(
  records: FailureRecord[]
): Promise<void> {
  if (records.length === 0) return;
  await ensureDir(FAILURES_DIR);
  await pruneOldDirs(FAILURES_DIR);

  for (const record of records) {
    const dateKey = record.timestamp.split("T")[0];
    const dirPath = path.join(FAILURES_DIR, dateKey);
    await ensureDir(dirPath);

    const timestampMs = Date.parse(record.timestamp) || Date.now();
    const fileName = `${timestampMs}@${record.id}.json`;
    const filePath = path.join(dirPath, fileName);
    await atomicWriteFile(filePath, JSON.stringify(record, null, 2));
  }
}

export async function listChangeRecords(
  date?: string,
  page: number = 1,
  pageSize: number = 20,
  client?: string
): Promise<ActivityList<ChangeRecordSummary>> {
  await pruneOldDirs(CHANGES_DIR);
  const dateKeys = date ? [date] : getRecentDateKeys();
  const records: ChangeRecordSummary[] = [];

  for (const dateKey of dateKeys) {
    const dirPath = path.join(CHANGES_DIR, dateKey);
    let files: string[] = [];
    try {
      files = await fs.readdir(dirPath);
    } catch {
      continue;
    }

    for (const fileName of files) {
      const record = parseChangeFileName(fileName, dateKey);
      if (record) {
        records.push(record);
      }
    }
  }

  const filteredRecords = client
    ? records.filter((record) => record.client === client)
    : records;
  filteredRecords.sort((a, b) => b.timestamp.localeCompare(a.timestamp));
  const total = filteredRecords.length;
  const start = Math.max(0, (page - 1) * pageSize);
  const end = start + pageSize;

  return {
    items: filteredRecords.slice(start, end),
    total,
    page,
    pageSize,
  };
}

export async function readChangeDiff(
  date: string,
  fileName: string
): Promise<string | null> {
  if (!isSafeFileName(fileName, ".diff")) return null;
  const filePath = path.join(CHANGES_DIR, date, fileName);
  const resolved = path.resolve(filePath);
  if (!resolved.startsWith(path.resolve(CHANGES_DIR))) return null;
  try {
    return await fs.readFile(filePath, "utf-8");
  } catch {
    return null;
  }
}

export async function countChangeRecords(date: string): Promise<number> {
  const dirPath = path.join(CHANGES_DIR, date);
  try {
    const files = await fs.readdir(dirPath);
    const unique = new Set<string>();
    for (const fileName of files) {
      const record = parseChangeFileName(fileName, date);
      if (record) {
        unique.add(`${record.ruleName}:${record.client}`);
      }
    }
    return unique.size;
  } catch {
    return 0;
  }
}

export async function listFailureRecords(
  date?: string,
  page: number = 1,
  pageSize: number = 20,
  client?: string
): Promise<ActivityList<FailureRecord>> {
  await pruneOldDirs(FAILURES_DIR);
  const dateKeys = date ? [date] : getRecentDateKeys();
  const records: FailureRecord[] = [];

  for (const dateKey of dateKeys) {
    const dirPath = path.join(FAILURES_DIR, dateKey);
    let files: string[] = [];
    try {
      files = await fs.readdir(dirPath);
    } catch {
      continue;
    }

    for (const fileName of files) {
      if (!fileName.endsWith(".json")) continue;
      const filePath = path.join(dirPath, fileName);
      try {
        const content = await fs.readFile(filePath, "utf-8");
        const record = JSON.parse(content) as FailureRecord;
        records.push(record);
      } catch {
        continue;
      }
    }
  }

  const filteredRecords = client
    ? records.filter((record) => record.client === client)
    : records;
  filteredRecords.sort((a, b) => b.timestamp.localeCompare(a.timestamp));
  const total = filteredRecords.length;
  const start = Math.max(0, (page - 1) * pageSize);
  const end = start + pageSize;

  return {
    items: filteredRecords.slice(start, end),
    total,
    page,
    pageSize,
  };
}

export async function listActivityDates(): Promise<string[]> {
  const [changeDates, failureDates] = await Promise.all([
    listAvailableDateDirs(CHANGES_DIR),
    listAvailableDateDirs(FAILURES_DIR),
  ]);
  const unique = new Set<string>([...changeDates, ...failureDates]);
  return Array.from(unique).sort((a, b) => b.localeCompare(a));
}

export async function countFailureRecords(date: string): Promise<number> {
  const dirPath = path.join(FAILURES_DIR, date);
  try {
    const files = await fs.readdir(dirPath);
    return files.filter((file) => file.endsWith(".json")).length;
  } catch {
    return 0;
  }
}

export async function clearActivityRecords(): Promise<void> {
  await fs.rm(RECORDS_DIR, { recursive: true, force: true });
  await ensureDir(CHANGES_DIR);
  await ensureDir(FAILURES_DIR);
}
