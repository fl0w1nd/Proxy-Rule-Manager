import * as fs from "node:fs/promises";
import * as path from "node:path";
import { randomUUID } from "node:crypto";
import { RulesConfig } from "./schema";
import { getSourcesDir } from "./data-paths";

const LOCAL_SOURCE_EXTENSION = ".txt";
const SOURCE_REF_PATTERN = /^[A-Za-z0-9._-]+$/;

function normalizeSourceRef(ref?: string | null): string | null {
  if (!ref) return null;
  const trimmed = ref.trim();
  if (!trimmed) return null;
  const withExt = trimmed.endsWith(LOCAL_SOURCE_EXTENSION)
    ? trimmed
    : `${trimmed}${LOCAL_SOURCE_EXTENSION}`;
  if (!SOURCE_REF_PATTERN.test(withExt)) return null;
  return withExt;
}

function generateSourceRef(): string {
  return `${randomUUID()}${LOCAL_SOURCE_EXTENSION}`;
}

async function ensureSourcesDir(): Promise<void> {
  await fs.mkdir(getSourcesDir(), { recursive: true });
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

function buildSourcePath(ref: string): string | null {
  const normalized = normalizeSourceRef(ref);
  if (!normalized) return null;
  const baseDir = getSourcesDir();
  const filePath = path.join(baseDir, normalized);
  const resolved = path.resolve(filePath);
  if (!resolved.startsWith(path.resolve(baseDir))) return null;
  return filePath;
}

export async function saveLocalSourceContent(ref: string | undefined, content: string): Promise<string> {
  const normalized = normalizeSourceRef(ref) ?? generateSourceRef();
  await ensureSourcesDir();
  const filePath = buildSourcePath(normalized);
  if (!filePath) {
    throw new Error("Invalid local source reference");
  }
  await atomicWriteFile(filePath, content);
  return normalized;
}

export async function readLocalSourceContent(ref: string): Promise<string | null> {
  const filePath = buildSourcePath(ref);
  if (!filePath) return null;
  try {
    return await fs.readFile(filePath, "utf-8");
  } catch {
    return null;
  }
}

export async function pruneLocalSources(keepRefs: Set<string>): Promise<void> {
  await ensureSourcesDir();
  const baseDir = getSourcesDir();
  const normalizedRefs = new Set<string>();
  for (const ref of keepRefs) {
    const normalized = normalizeSourceRef(ref);
    if (normalized) normalizedRefs.add(normalized);
  }
  const entries = await fs.readdir(baseDir, { withFileTypes: true });
  await Promise.all(
    entries.map(async (entry) => {
      if (!entry.isFile()) return;
      if (!normalizedRefs.has(entry.name)) {
        await fs.rm(path.join(baseDir, entry.name), { force: true });
      }
    })
  );
}

function cloneConfig(config: RulesConfig): RulesConfig {
  if (typeof structuredClone === "function") {
    return structuredClone(config);
  }
  return JSON.parse(JSON.stringify(config)) as RulesConfig;
}

export async function hydrateConfigLocalSources(config: RulesConfig): Promise<RulesConfig> {
  const cloned = cloneConfig(config);
  for (const rule of cloned.rules) {
    if (!rule.sources) continue;
    for (const source of rule.sources) {
      if (source.type !== "local") continue;
      if (typeof source.content === "string") continue;
      if (!source.contentRef) continue;
      const content = await readLocalSourceContent(source.contentRef);
      if (content !== null) {
        source.content = content;
      }
    }
  }
  return cloned;
}

export async function externalizeConfigLocalSources(
  config: RulesConfig
): Promise<{ config: RulesConfig; refs: Set<string> }> {
  const cloned = cloneConfig(config);
  const refs = new Set<string>();

  for (const rule of cloned.rules) {
    if (!rule.sources) continue;
    for (const source of rule.sources) {
      if (source.type !== "local") continue;
      if (typeof source.content === "string") {
        const ref = await saveLocalSourceContent(source.contentRef, source.content);
        source.contentRef = ref;
        delete source.content;
        refs.add(ref);
      } else if (source.contentRef) {
        const normalized = normalizeSourceRef(source.contentRef);
        if (normalized) {
          source.contentRef = normalized;
          refs.add(normalized);
        }
      }
    }
  }

  return { config: cloned, refs };
}
