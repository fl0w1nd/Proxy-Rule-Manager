import * as fs from "node:fs/promises";
import * as path from "node:path";
import { createHash } from "node:crypto";
import AdmZip from "adm-zip";
import * as protobuf from "protobufjs/index.js";
import type {
  ClientType,
  GeositeProvider,
  GeositeRenderProfile,
  RuleConfig,
  RulesConfig,
  SourceConfig,
} from "./schema";
import { applyNewTransforms } from "./transformer";
import { getGeositeDir } from "./data-paths";
import { getClients, getConfig, saveConfig } from "./storage-adapter";
import { getGeositeInternalRuleName, getPrimaryGeositeSource, isGeositeRule } from "./rule-classification";

export type GeositeEntryType = "domain" | "full" | "keyword" | "regexp";

export interface GeositeEntry {
  type: GeositeEntryType;
  value: string;
  attrs: string[];
}

export interface GeositeProviderCache {
  provider: GeositeProvider;
  resolvedVersion: string;
  fetchedAt: string;
  catalog: string[];
  entries: Record<string, GeositeEntry[]>;
}

export interface GeositeProviderStatus {
  provider: GeositeProvider;
  ready: boolean;
  fetchedAt: string | null;
  resolvedVersion: string | null;
  catalogCount: number;
}

export interface GeositeCatalogSummary {
  name: string;
  attrs: string[];
  entryCount: number;
}

interface RawInclude {
  list: string;
  requiredAttrs: string[];
  excludedAttrs: string[];
}

interface RawList {
  entries: GeositeEntry[];
  includes: RawInclude[];
}

const GEOSITE_DIR = getGeositeDir();
const GITHUB_HEADERS = {
  "User-Agent": "Proxy-Rule-Manager/1.0",
  Accept: "application/vnd.github+json",
};

const GEOSITE_PROTO = `
syntax = "proto3";

message Domain {
  enum Type {
    Plain = 0;
    Regex = 1;
    RootDomain = 2;
    Full = 3;
  }

  message Attribute {
    string key = 1;

    oneof typed_value {
      bool bool_value = 2;
      int64 int_value = 3;
    }
  }

  Type type = 1;
  string value = 2;
  repeated Attribute attribute = 3;
}

message GeoSite {
  string country_code = 1;
  repeated Domain domain = 2;
  bytes resource_hash = 3;
}

message GeoSiteList {
  repeated GeoSite entry = 1;
}
`;

let geositeProtoRoot: protobuf.Root | null = null;
const FETCH_TIMEOUT_MS = 20_000;
const MAX_PROVIDER_DOWNLOAD_BYTES = 50 * 1024 * 1024;
const providerRefreshLocks = new Map<GeositeProvider, Promise<GeositeProviderCache>>();
const providerMemoryCache = new Map<GeositeProvider, GeositeProviderCache>();
const lookupIndexCache = new Map<string, GeositeLookupIndex>();

interface GeositeLookupIndex {
  exactEntries: Array<{ listName: string; value: string }>;
  suffixEntries: Array<{ listName: string; value: string }>;
  keywordEntries: Array<{ listName: string; value: string }>;
  regexEntries: Array<{ listName: string; pattern: RegExp }>;
}

function getGeositeProtoRoot(): protobuf.Root {
  if (!geositeProtoRoot) {
    geositeProtoRoot = protobuf.parse(GEOSITE_PROTO).root;
  }
  return geositeProtoRoot;
}

function normalizeName(value: string): string {
  return value.trim().toLowerCase();
}

function normalizeAttrs(attrs: string[] | undefined): string[] {
  return Array.from(
    new Set(
      (attrs || [])
        .map((attr) => attr.trim().toLowerCase())
        .filter(Boolean)
    )
  ).sort();
}

function makeEntryKey(entry: GeositeEntry): string {
  return `${entry.type}:${entry.value}:${normalizeAttrs(entry.attrs).join("@")}`;
}

function dedupeEntries(entries: GeositeEntry[]): GeositeEntry[] {
  const seen = new Set<string>();
  const deduped: GeositeEntry[] = [];
  for (const entry of entries) {
    const normalized: GeositeEntry = {
      type: entry.type,
      value: entry.value,
      attrs: normalizeAttrs(entry.attrs),
    };
    const key = makeEntryKey(normalized);
    if (!seen.has(key)) {
      seen.add(key);
      deduped.push(normalized);
    }
  }
  return deduped;
}

function matchesRequiredAttrs(entry: GeositeEntry, requiredAttrs: string[], excludedAttrs: string[]): boolean {
  const entryAttrs = new Set(normalizeAttrs(entry.attrs));
  for (const attr of requiredAttrs) {
    if (!entryAttrs.has(attr)) {
      return false;
    }
  }
  for (const attr of excludedAttrs) {
    if (entryAttrs.has(attr)) {
      return false;
    }
  }
  return true;
}

function parseRuleType(ruleWithType: string): GeositeEntry {
  const [prefix, rest] = ruleWithType.includes(":")
    ? (ruleWithType.split(/:(.+)/, 2) as [string, string])
    : ["domain", ruleWithType];

  const value = (rest || "").trim();
  switch (prefix.trim().toLowerCase()) {
    case "full":
      return { type: "full", value, attrs: [] };
    case "domain":
      return { type: "domain", value, attrs: [] };
    case "keyword":
      return { type: "keyword", value, attrs: [] };
    case "regexp":
      return { type: "regexp", value, attrs: [] };
    default:
      return { type: "domain", value: ruleWithType.trim().toLowerCase(), attrs: [] };
  }
}

function stripComment(line: string): string {
  const idx = line.indexOf("#");
  if (idx === -1) {
    return line.trim();
  }
  return line.slice(0, idx).trim();
}

function ensureRawList(map: Map<string, RawList>, listName: string): RawList {
  const normalized = normalizeName(listName);
  let current = map.get(normalized);
  if (!current) {
    current = { entries: [], includes: [] };
    map.set(normalized, current);
  }
  return current;
}

function parseInclude(firstToken: string, restTokens: string[]): RawInclude {
  const includeValue = firstToken.slice("include:".length).trim();
  const parts = includeValue.split("@").map((item) => item.trim()).filter(Boolean);
  const list = normalizeName(parts[0] || "");
  const attrTokens = [
    ...parts.slice(1).map((attr) => `@${attr}`),
    ...restTokens,
  ].map((token) => token.trim()).filter(Boolean);

  const requiredAttrs: string[] = [];
  const excludedAttrs: string[] = [];

  for (const token of attrTokens) {
    if (!token.startsWith("@")) continue;
    const attr = token.slice(1).trim().toLowerCase();
    if (!attr) continue;
    if (attr.startsWith("-")) {
      excludedAttrs.push(attr.slice(1));
    } else {
      requiredAttrs.push(attr);
    }
  }

  return {
    list,
    requiredAttrs: normalizeAttrs(requiredAttrs),
    excludedAttrs: normalizeAttrs(excludedAttrs),
  };
}

function parseV2flyRawLists(files: Record<string, string>): Map<string, RawList> {
  const rawLists = new Map<string, RawList>();

  for (const [fileName, content] of Object.entries(files)) {
    const listName = normalizeName(fileName);
    const rawList = ensureRawList(rawLists, listName);
    const lines = content.split(/\r?\n/);

    for (const line of lines) {
      const stripped = stripComment(line);
      if (!stripped) continue;

      const tokens = stripped.split(/\s+/).filter(Boolean);
      if (tokens.length === 0) continue;

      const [firstToken, ...restTokens] = tokens;
      if (firstToken.startsWith("include:")) {
        rawList.includes.push(parseInclude(firstToken, restTokens));
        continue;
      }

      const entry = parseRuleType(firstToken);
      const attrs: string[] = [];
      const affiliations: string[] = [];

      for (const token of restTokens) {
        if (token.startsWith("@")) {
          attrs.push(token.slice(1));
        } else if (token.startsWith("&")) {
          affiliations.push(token.slice(1));
        }
      }

      entry.attrs = normalizeAttrs(attrs);
      rawList.entries.push(entry);

      for (const affiliation of affiliations) {
        ensureRawList(rawLists, affiliation).entries.push({ ...entry, attrs: [...entry.attrs] });
      }
    }
  }

  return rawLists;
}

function expandRawLists(rawLists: Map<string, RawList>): Record<string, GeositeEntry[]> {
  const memo = new Map<string, GeositeEntry[]>();
  const visiting = new Set<string>();

  const expand = (listName: string): GeositeEntry[] => {
    const normalized = normalizeName(listName);
    if (memo.has(normalized)) {
      return memo.get(normalized)!;
    }
    if (visiting.has(normalized)) {
      throw new Error(`Circular geosite include detected for list "${normalized}"`);
    }

    visiting.add(normalized);
    const rawList = rawLists.get(normalized) || { entries: [], includes: [] };
    const combined: GeositeEntry[] = rawList.entries.map((entry) => ({
      ...entry,
      attrs: [...entry.attrs],
    }));

    for (const include of rawList.includes) {
      const includedEntries = expand(include.list);
      for (const entry of includedEntries) {
        if (matchesRequiredAttrs(entry, include.requiredAttrs, include.excludedAttrs)) {
          combined.push({
            ...entry,
            attrs: [...entry.attrs],
          });
        }
      }
    }

    visiting.delete(normalized);
    const deduped = dedupeEntries(combined);
    memo.set(normalized, deduped);
    return deduped;
  };

  const expanded: Record<string, GeositeEntry[]> = {};
  for (const listName of rawLists.keys()) {
    const entries = expand(listName);
    if (entries.length > 0) {
      expanded[listName] = entries;
    }
  }

  return expanded;
}

async function fetchJson<T>(url: string): Promise<T> {
  const response = await fetch(url, {
    headers: GITHUB_HEADERS,
    signal: AbortSignal.timeout(FETCH_TIMEOUT_MS),
  });
  if (!response.ok) {
    throw new Error(`Failed to fetch ${url}: HTTP ${response.status}`);
  }
  return response.json() as Promise<T>;
}

async function fetchBuffer(url: string, headers?: Record<string, string>): Promise<Buffer> {
  const response = await fetch(url, {
    headers: headers || GITHUB_HEADERS,
    signal: AbortSignal.timeout(FETCH_TIMEOUT_MS),
  });
  if (!response.ok) {
    throw new Error(`Failed to fetch ${url}: HTTP ${response.status}`);
  }
  const contentLengthHeader = response.headers.get("content-length");
  const contentLength = contentLengthHeader ? Number(contentLengthHeader) : NaN;
  if (Number.isFinite(contentLength) && contentLength > MAX_PROVIDER_DOWNLOAD_BYTES) {
    throw new Error(`Provider asset too large: ${contentLength} bytes`);
  }
  const arrayBuffer = await response.arrayBuffer();
  if (arrayBuffer.byteLength > MAX_PROVIDER_DOWNLOAD_BYTES) {
    throw new Error(`Provider asset too large: ${arrayBuffer.byteLength} bytes`);
  }
  return Buffer.from(arrayBuffer);
}

async function writeProviderCache(cache: GeositeProviderCache): Promise<void> {
  await fs.mkdir(GEOSITE_DIR, { recursive: true });
  const filePath = path.join(GEOSITE_DIR, `${cache.provider}.json`);
  const tempPath = `${filePath}.${process.pid}.${Date.now()}.tmp`;
  await fs.writeFile(tempPath, JSON.stringify(cache), "utf-8");
  await fs.rename(tempPath, filePath);
  providerMemoryCache.set(cache.provider, cache);
}

export async function readGeositeProviderCache(provider: GeositeProvider): Promise<GeositeProviderCache | null> {
  const cached = providerMemoryCache.get(provider);
  if (cached) {
    return cached;
  }
  const filePath = path.join(GEOSITE_DIR, `${provider}.json`);
  try {
    const content = await fs.readFile(filePath, "utf-8");
    const parsed = JSON.parse(content) as GeositeProviderCache;
    providerMemoryCache.set(provider, parsed);
    return parsed;
  } catch {
    return null;
  }
}

async function getRepoDefaultBranch(owner: string, repo: string): Promise<string> {
  const metadata = await fetchJson<{ default_branch: string }>(`https://api.github.com/repos/${owner}/${repo}`);
  return metadata.default_branch;
}

async function refreshV2flyProvider(): Promise<GeositeProviderCache> {
  const defaultBranch = await getRepoDefaultBranch("v2fly", "domain-list-community");
  const commit = await fetchJson<{ sha: string }>(`https://api.github.com/repos/v2fly/domain-list-community/commits/${defaultBranch}`);
  const sha = commit.sha;
  const zipBuffer = await fetchBuffer(`https://codeload.github.com/v2fly/domain-list-community/zip/${sha}`);
  const zip = new AdmZip(zipBuffer);

  const rawFiles: Record<string, string> = {};
  for (const entry of zip.getEntries()) {
    if (entry.isDirectory) continue;
    const normalized = entry.entryName.replace(/\\/g, "/");
    const match = normalized.match(/^[^/]+\/data\/(.+)$/);
    if (!match) continue;
    const fileName = match[1];
    if (!fileName || fileName.includes("/")) continue;
    rawFiles[normalizeName(fileName)] = entry.getData().toString("utf-8");
  }

  const cache = buildV2flyCacheFromRawFiles(rawFiles, sha);
  await writeProviderCache(cache);
  return cache;
}

function mapDomainType(type: string | number | undefined): GeositeEntryType | null {
  switch (type) {
    case "Full":
    case 3:
      return "full";
    case "RootDomain":
    case 2:
      return "domain";
    case "Regex":
    case 1:
      return "regexp";
    case "Plain":
    case 0:
      return "keyword";
    default:
      return null;
  }
}

async function refreshLoyalsoldierProvider(): Promise<GeositeProviderCache> {
  const release = await fetchJson<{
    tag_name: string;
    assets: Array<{ name: string; browser_download_url: string }>;
  }>("https://api.github.com/repos/Loyalsoldier/v2ray-rules-dat/releases/latest");

  const asset = release.assets.find((item) => item.name === "geosite.dat");
  if (!asset) {
    throw new Error("geosite.dat asset not found in Loyalsoldier release");
  }

  const buffer = await fetchBuffer(asset.browser_download_url, {
    "User-Agent": "Proxy-Rule-Manager/1.0",
  });
  const cache = decodeLoyalsoldierGeositeDat(buffer, release.tag_name);
  await writeProviderCache(cache);
  return cache;
}

export function buildV2flyCacheFromRawFiles(
  files: Record<string, string>,
  resolvedVersion: string
): GeositeProviderCache {
  const expanded = expandRawLists(parseV2flyRawLists(files));
  return {
    provider: "v2fly",
    resolvedVersion,
    fetchedAt: new Date().toISOString(),
    catalog: Object.keys(expanded).sort(),
    entries: expanded,
  };
}

export function decodeLoyalsoldierGeositeDat(
  buffer: Buffer,
  resolvedVersion: string
): GeositeProviderCache {
  const root = getGeositeProtoRoot();
  const GeoSiteList = root.lookupType("GeoSiteList");
  const decoded = GeoSiteList.decode(buffer);
  const object = GeoSiteList.toObject(decoded, {
    enums: String,
    longs: String,
  }) as {
    entry?: Array<{
      countryCode?: string;
      domain?: Array<{
        type?: string | number;
        value?: string;
        attribute?: Array<{ key?: string }>;
      }>;
    }>;
  };

  const entries: Record<string, GeositeEntry[]> = {};
  for (const site of object.entry || []) {
    const listName = normalizeName(site.countryCode || "");
    if (!listName) continue;
    const siteEntries = dedupeEntries(
      (site.domain || [])
        .map((domain) => {
          const type = mapDomainType(domain.type);
          const value = domain.value?.trim();
          if (!type || !value) return null;
          return {
            type,
            value,
            attrs: normalizeAttrs((domain.attribute || []).map((attr) => attr.key || "")),
          } satisfies GeositeEntry;
        })
        .filter((entry): entry is GeositeEntry => !!entry)
    );
    if (siteEntries.length > 0) {
      entries[listName] = siteEntries;
    }
  }

  return {
    provider: "loyalsoldier",
    resolvedVersion,
    fetchedAt: new Date().toISOString(),
    catalog: Object.keys(entries).sort(),
    entries,
  };
}

export async function refreshGeositeProvider(provider: GeositeProvider): Promise<GeositeProviderCache> {
  const activeRefresh = providerRefreshLocks.get(provider);
  if (activeRefresh) {
    return activeRefresh;
  }

  const refreshPromise = (async () => {
    if (provider === "v2fly") {
      return refreshV2flyProvider();
    }
    return refreshLoyalsoldierProvider();
  })();

  providerRefreshLocks.set(provider, refreshPromise);

  try {
    return await refreshPromise;
  } finally {
    providerRefreshLocks.delete(provider);
  }
}

export async function ensureGeositeProviderCache(provider: GeositeProvider): Promise<GeositeProviderCache> {
  const existing = await readGeositeProviderCache(provider);
  if (existing) {
    return existing;
  }
  return refreshGeositeProvider(provider);
}

export async function listGeositeProviders(): Promise<GeositeProviderStatus[]> {
  const providers: GeositeProvider[] = ["v2fly", "loyalsoldier"];
  const caches = await Promise.all(providers.map((provider) => readGeositeProviderCache(provider)));
  return providers.map((provider, index) => {
    const cache = caches[index];
    return {
      provider,
      ready: !!cache,
      fetchedAt: cache?.fetchedAt || null,
      resolvedVersion: cache?.resolvedVersion || null,
      catalogCount: cache?.catalog.length || 0,
    };
  });
}

export async function getGeositeCatalog(provider: GeositeProvider): Promise<GeositeProviderCache> {
  return ensureGeositeProviderCache(provider);
}

export async function getGeositeCatalogSummary(provider: GeositeProvider): Promise<GeositeCatalogSummary[]> {
  const cache = await ensureGeositeProviderCache(provider);
  return cache.catalog.map((name) => {
    const entries = cache.entries[name] || [];
    const attrs = Array.from(
      new Set(entries.flatMap((entry) => entry.attrs || []))
    ).sort();
    return {
      name,
      attrs,
      entryCount: entries.length,
    };
  });
}

export async function resolveGeositeEntries(
  provider: GeositeProvider,
  list: string,
  attrs: string[] = []
): Promise<GeositeEntry[]> {
  const cache = await ensureGeositeProviderCache(provider);
  const entries = cache.entries[normalizeName(list)];
  if (!entries) {
    throw new Error(`Geosite list "${list}" not found for provider "${provider}"`);
  }
  const normalizedAttrs = normalizeAttrs(attrs);
  if (normalizedAttrs.length === 0) {
    return entries.map((entry) => ({ ...entry, attrs: [...entry.attrs] }));
  }
  return entries
    .filter((entry) => normalizedAttrs.every((attr) => entry.attrs.includes(attr)))
    .map((entry) => ({ ...entry, attrs: [...entry.attrs] }));
}

export function renderGeositeEntries(
  entries: GeositeEntry[],
  renderProfile: GeositeRenderProfile = "mihomo-classical"
): string {
  if (renderProfile !== "mihomo-classical") {
    throw new Error(`Unsupported geosite render profile: ${renderProfile}`);
  }

  return entries
    .map((entry) => {
      switch (entry.type) {
        case "domain":
          return `DOMAIN-SUFFIX,${entry.value}`;
        case "full":
          return `DOMAIN,${entry.value}`;
        case "keyword":
          return `DOMAIN-KEYWORD,${entry.value}`;
        case "regexp":
          return `DOMAIN-REGEX,${entry.value}`;
      }
    })
    .join("\n");
}

export async function renderGeositeSource(source: SourceConfig): Promise<string> {
  if (source.type !== "geosite" || !source.provider || !source.list) {
    throw new Error("Invalid geosite source");
  }
  const entries = await resolveGeositeEntries(source.provider, source.list, source.attrs || []);
  return renderGeositeEntries(entries, source.renderProfile || "mihomo-classical");
}

function buildDefaultGeositeRule(
  provider: GeositeProvider,
  list: string,
  clientId: ClientType,
  attrs: string[] = []
): RuleConfig {
  const normalizedAttrs = normalizeAttrSelection(attrs);
  return {
    name: getGeositeInternalRuleName(provider, list, normalizedAttrs),
    displayName: normalizedAttrs.length > 0 ? `${list}@${normalizedAttrs.join("+")}` : list,
    description: normalizedAttrs.length > 0
      ? `Geosite ${list}@${normalizedAttrs.join("+")} from ${provider}`
      : `Geosite ${list} from ${provider}`,
    tags: ["geosite", provider],
    sources: [
      {
        type: "geosite",
        provider,
        list,
        attrs: normalizedAttrs,
        renderProfile: "mihomo-classical",
      },
    ],
    output: {
      clients: [clientId],
    },
    transforms: [],
  };
}

function syncManagedGeositePresentation(rule: RuleConfig, provider: GeositeProvider, list: string, attrs: string[] = []): void {
  const normalizedAttrs = normalizeAttrSelection(attrs);
  rule.displayName = normalizedAttrs.length > 0 ? `${list}@${normalizedAttrs.join("+")}` : list;
  rule.description = normalizedAttrs.length > 0
    ? `Geosite ${list}@${normalizedAttrs.join("+")} from ${provider}`
    : `Geosite ${list} from ${provider}`;
}

function getLegacyGeositeInternalRuleName(provider: GeositeProvider, list: string, attrs: string[] = []): string {
  const normalizedAttrs = normalizeAttrSelection(attrs);
  if (normalizedAttrs.length === 0) {
    return `geosite_${provider}_${list}`;
  }
  return `geosite_${provider}_${list}__${normalizedAttrs.join("_")}`;
}

export interface ImportAllGeositeResult {
  created: number;
  updated: number;
  skipped: number;
  total: number;
  ruleNames: string[];
}

export interface GeositeImportSelection {
  list: string;
  attrs?: string[];
}

function normalizeAttrSelection(attrs: string[] | undefined): string[] {
  return Array.from(
    new Set(
      (attrs || [])
        .map((item) => item.trim().toLowerCase())
        .filter(Boolean)
    )
  ).sort();
}

function normalizeImportSelections(
  selections: Array<string | GeositeImportSelection>
): Array<{ list: string; attrs: string[] }> {
  return Array.from(
    new Map(
      selections.map((selection) => {
        const list = normalizeName(typeof selection === "string" ? selection : selection.list);
        const attrs = normalizeAttrSelection(typeof selection === "string" ? [] : selection.attrs);
        return [`${list}::${attrs.join("+")}`, { list, attrs }];
      })
    ).values()
  )
    .filter((item) => item.list)
    .sort((a, b) => `${a.list}::${a.attrs.join("+")}`.localeCompare(`${b.list}::${b.attrs.join("+")}`));
}

function createImportedGeositeRuleKey(
  provider: GeositeProvider,
  list: string,
  attrs: string[] = []
): string {
  const normalizedList = normalizeName(list);
  const normalizedAttrs = normalizeAttrSelection(attrs);
  return `${provider}::${normalizedList}::${normalizedAttrs.join("+")}`;
}

function buildImportedGeositeRuleIndex(
  config: RulesConfig,
  provider: GeositeProvider
): Map<string, RuleConfig> {
  const index = new Map<string, RuleConfig>();

  for (const rule of config.rules) {
    const source = getPrimaryGeositeSource(rule);
    if (!source || source.provider !== provider || !source.list) {
      continue;
    }
    index.set(createImportedGeositeRuleKey(provider, source.list, source.attrs || []), rule);
  }

  return index;
}

async function assertClientExists(clientId: ClientType): Promise<void> {
  const clients = await getClients();
  if (!clients.some((client) => client.id === clientId)) {
    throw new Error(`Client "${clientId}" not found`);
  }
}

export function upsertImportedGeositeRules(
  config: RulesConfig,
  provider: GeositeProvider,
  clientId: ClientType,
  selections: Array<string | GeositeImportSelection>
): ImportAllGeositeResult {
  const normalizedSelections = normalizeImportSelections(selections);
  const existingRuleIndex = buildImportedGeositeRuleIndex(config, provider);
  let created = 0;
  let updated = 0;
  let skipped = 0;
  const ruleNames: string[] = [];

  for (const selection of normalizedSelections) {
    const ruleName = getGeositeInternalRuleName(provider, selection.list, selection.attrs);
    const selectionKey = createImportedGeositeRuleKey(provider, selection.list, selection.attrs);
    const existing = existingRuleIndex.get(selectionKey);
    ruleNames.push(ruleName);

    if (!existing) {
      const newRule = buildDefaultGeositeRule(provider, selection.list, clientId, selection.attrs);
      config.rules.push(newRule);
      existingRuleIndex.set(selectionKey, newRule);
      created += 1;
      continue;
    }

    if (!isGeositeRule(existing)) {
      skipped += 1;
      continue;
    }

    const primarySource = getPrimaryGeositeSource(existing);
    if (
      !primarySource
      || createImportedGeositeRuleKey(primarySource.provider || provider, primarySource.list || "", primarySource.attrs || []) !== selectionKey
    ) {
      skipped += 1;
      continue;
    }

    syncManagedGeositePresentation(existing, provider, selection.list, selection.attrs);
    const nextRuleName = getGeositeInternalRuleName(provider, selection.list, selection.attrs);
    const legacyRuleName = getLegacyGeositeInternalRuleName(provider, selection.list, selection.attrs);
    if (existing.name === legacyRuleName || existing.name === nextRuleName) {
      existing.name = nextRuleName;
    }

    if (!existing.output.clients.includes(clientId)) {
      existing.output.clients = [...existing.output.clients, clientId];
      updated += 1;
    } else {
      skipped += 1;
    }
  }

  return {
    created,
    updated,
    skipped,
    total: normalizedSelections.length,
    ruleNames,
  };
}

export async function importAllGeositeRules(
  provider: GeositeProvider,
  clientId: ClientType
): Promise<ImportAllGeositeResult> {
  await assertClientExists(clientId);
  const cache = await ensureGeositeProviderCache(provider);
  const config = await getConfig();
  const result = upsertImportedGeositeRules(config as RulesConfig, provider, clientId, cache.catalog);
  await saveConfig(config as RulesConfig);
  return result;
}

export async function importSelectedGeositeRules(
  provider: GeositeProvider,
  clientId: ClientType,
  selections: Array<string | GeositeImportSelection>
): Promise<ImportAllGeositeResult> {
  await assertClientExists(clientId);
  const cache = await ensureGeositeProviderCache(provider);
  const selectedCatalog = Array.from(
    new Set(
      normalizeImportSelections(selections).map((item) => item.list)
    )
  );
  const availableLists = new Set(cache.catalog.map((item) => normalizeName(item)));

  for (const list of selectedCatalog) {
    if (!availableLists.has(list)) {
      throw new Error(`Geosite list "${list}" not found for provider "${provider}"`);
    }
  }

  const config = await getConfig();
  const result = upsertImportedGeositeRules(config as RulesConfig, provider, clientId, selections);
  await saveConfig(config as RulesConfig);
  return result;
}

export async function previewGeositeSelection(
  provider: GeositeProvider,
  list: string,
  clientId: ClientType,
  attrs: string[] = [],
  renderProfile: GeositeRenderProfile = "mihomo-classical"
): Promise<{ content: string; totalEntries: number }> {
  await assertClientExists(clientId);
  const entries = await resolveGeositeEntries(provider, list, attrs);
  let content = renderGeositeEntries(entries, renderProfile);
  const [clients, config] = await Promise.all([getClients(), getConfig()]);
  const clientConfig = clients.find((client) => client.id === clientId);
  const globalTransforms = clientConfig?.transforms || [];
  if (globalTransforms.length > 0) {
    content = applyNewTransforms([content], globalTransforms, config.transformers || {})[0] || content;
  }
  return {
    content,
    totalEntries: entries.length,
  };
}

export function lookupGeositeListsInEntries(
  entries: Record<string, GeositeEntry[]>,
  domain: string
): string[] {
  const normalizedDomain = domain.trim().toLowerCase().replace(/\.+$/, "");
  if (!normalizedDomain) {
    return [];
  }

  const indexKey = createHash("sha1").update(JSON.stringify(entries)).digest("hex");
  let index = lookupIndexCache.get(indexKey);
  if (!index) {
    index = {
      exactEntries: [],
      suffixEntries: [],
      keywordEntries: [],
      regexEntries: [],
    };

    for (const [listName, listEntries] of Object.entries(entries)) {
      for (const entry of listEntries) {
        const value = entry.value.trim().toLowerCase();
        if (!value) continue;

        if (entry.type === "full") {
          index.exactEntries.push({ listName, value });
          continue;
        }

        if (entry.type === "domain") {
          index.suffixEntries.push({ listName, value });
          continue;
        }

        if (entry.type === "keyword") {
          index.keywordEntries.push({ listName, value });
          continue;
        }

        try {
          index.regexEntries.push({ listName, pattern: new RegExp(entry.value) });
        } catch {
          // Skip invalid regex rules while keeping other entries searchable.
        }
      }
    }

    lookupIndexCache.set(indexKey, index);
  }

  const matches = new Set<string>();

  for (const entry of index.exactEntries) {
    if (normalizedDomain === entry.value) {
      matches.add(entry.listName);
    }
  }

  for (const entry of index.suffixEntries) {
    if (normalizedDomain === entry.value || normalizedDomain.endsWith(`.${entry.value}`)) {
      matches.add(entry.listName);
    }
  }

  for (const entry of index.keywordEntries) {
    if (normalizedDomain.includes(entry.value)) {
      matches.add(entry.listName);
    }
  }

  for (const entry of index.regexEntries) {
    if (entry.pattern.test(normalizedDomain)) {
      matches.add(entry.listName);
    }
  }

  return Array.from(matches).sort((a, b) => a.localeCompare(b));
}

export async function lookupGeositeListsByDomain(
  provider: GeositeProvider,
  domain: string
): Promise<string[]> {
  const cache = await ensureGeositeProviderCache(provider);
  return lookupGeositeListsInEntries(cache.entries, domain);
}
