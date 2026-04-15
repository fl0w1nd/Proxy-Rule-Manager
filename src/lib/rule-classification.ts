import type { RuleConfig, SourceConfig, GeositeProvider } from "./schema";

function normalizeGeositeAttrs(attrs: string[] | undefined): string[] {
  return Array.from(
    new Set(
      (attrs || [])
        .map((item) => item.trim().toLowerCase())
        .filter(Boolean)
    )
  ).sort();
}

export function isGeositeSource(source: Pick<SourceConfig, "type"> | null | undefined): boolean {
  return (source?.type || "url") === "geosite";
}

export function getPrimaryGeositeSource(rule: Pick<RuleConfig, "sources"> | null | undefined): SourceConfig | undefined {
  if (!rule?.sources || rule.sources.length === 0) {
    return undefined;
  }
  const [source] = rule.sources;
  return isGeositeSource(source) ? source : undefined;
}

export function isGeositeRule(rule: Pick<RuleConfig, "sources"> | null | undefined): boolean {
  return !!getPrimaryGeositeSource(rule);
}

export function getGeositeInternalRuleName(provider: GeositeProvider, list: string, attrs: string[] = []): string {
  const normalizedAttrs = normalizeGeositeAttrs(attrs);
  if (normalizedAttrs.length === 0) {
    return `geosite_${provider}_${list}`;
  }
  return `geosite_${provider}_${list}@${normalizedAttrs.join("+")}`;
}

export function getGeositeOutputName(source: Pick<SourceConfig, "list" | "attrs">): string {
  const list = (source.list || "").trim();
  const attrs = normalizeGeositeAttrs(source.attrs);

  if (attrs.length === 0) {
    return list;
  }

  return `${list}@${attrs.join("+")}`;
}
