import type { Hono } from "hono";
import { z } from "zod";
import {
  getGeositeCatalog,
  getGeositeCatalogSummary,
  importAllGeositeRules,
  importSelectedGeositeRules,
  lookupGeositeListsByDomain,
  listGeositeProviders,
  previewGeositeSelection,
  refreshGeositeProvider,
} from "../../lib/geosite";
import { GeositeProviderSchema } from "../../lib/schema";
import { getClients, getConfig } from "../../lib/storage-adapter";
import { getPrimaryGeositeSource, isGeositeRule } from "../../lib/rule-classification";
import { executeBatchPartialSync } from "../../lib/sync-engine";
import { jsonError } from "../errors";

const ImportAllSchema = z.object({
  provider: GeositeProviderSchema,
  clientId: z.string().min(1),
});

const ImportSelectedSchema = z.object({
  provider: GeositeProviderSchema,
  clientId: z.string().min(1),
  lists: z.array(
    z.union([
      z.string().min(1),
      z.object({
        list: z.string().min(1),
        attrs: z.array(z.string().min(1)).optional(),
      }),
    ])
  ).min(1),
});

export async function syncImportedGeositeRules(ruleNames: string[]): Promise<{
  syncedRules: string[];
  failedRules: { name: string; error: string }[];
}> {
  const uniqueRuleNames = Array.from(new Set(ruleNames));
  if (uniqueRuleNames.length === 0) {
    return { syncedRules: [], failedRules: [] };
  }

  const result = await executeBatchPartialSync(uniqueRuleNames);
  const failedRuleNames = new Set(result.failedRules.map((item) => item.name));

  return {
    syncedRules: uniqueRuleNames.filter((ruleName) => !failedRuleNames.has(ruleName)),
    failedRules: result.failedRules.map((item) => ({
      name: item.name,
      error: item.error,
    })),
  };
}

export function registerGeositeRoutes(app: Hono) {
  app.get("/api/geosite/providers", async (c) => {
    try {
      const providers = await listGeositeProviders();
      return c.json({ providers });
    } catch (error) {
      return jsonError(c, error, "Failed to list geosite providers");
    }
  });

  app.post("/api/geosite/providers/:provider/refresh", async (c) => {
    try {
      const provider = GeositeProviderSchema.parse(c.req.param("provider"));
      const cache = await refreshGeositeProvider(provider);
      return c.json({
        success: true,
        provider,
        resolvedVersion: cache.resolvedVersion,
        fetchedAt: cache.fetchedAt,
        catalogCount: cache.catalog.length,
      });
    } catch (error) {
      return jsonError(c, error, "Failed to refresh geosite provider");
    }
  });

  app.get("/api/geosite/catalog", async (c) => {
    try {
      const provider = GeositeProviderSchema.parse(c.req.query("provider"));
      const [cache, catalogSummary, config] = await Promise.all([
        getGeositeCatalog(provider),
        getGeositeCatalogSummary(provider),
        getConfig(),
      ]);

      const importedRules = new Map<string, { ruleName: string; clients: string[] }>();
      for (const rule of config.rules) {
        if (!isGeositeRule(rule)) continue;
        const source = getPrimaryGeositeSource(rule);
        if (!source || source.provider !== provider || !source.list) continue;
        importedRules.set(source.list, {
          ruleName: rule.name,
          clients: rule.output.clients,
        });
      }

      return c.json({
        provider,
        resolvedVersion: cache.resolvedVersion,
        fetchedAt: cache.fetchedAt,
        catalog: catalogSummary.map((item) => ({
          name: item.name,
          imported: importedRules.has(item.name),
          ruleName: importedRules.get(item.name)?.ruleName || null,
          clients: importedRules.get(item.name)?.clients || [],
          attrs: item.attrs,
          entryCount: item.entryCount,
        })),
      });
    } catch (error) {
      return jsonError(c, error, "Failed to get geosite catalog");
    }
  });

  app.get("/api/geosite/domain-lookup", async (c) => {
    try {
      const provider = GeositeProviderSchema.parse(c.req.query("provider"));
      const domain = (c.req.query("domain") || "").trim();
      if (domain.length < 2) {
        return c.json({ matches: [] });
      }
      const matches = await lookupGeositeListsByDomain(provider, domain);
      return c.json({ matches });
    } catch (error) {
      return jsonError(c, error, "Failed to lookup geosite domain");
    }
  });

  app.post("/api/geosite/import-all", async (c) => {
    try {
      const body = await c.req.json();
      const parsed = ImportAllSchema.parse(body);
      const clients = await getClients();
      if (!clients.some((client) => client.id === parsed.clientId)) {
        return c.json({ error: `Client "${parsed.clientId}" not found` }, 400);
      }
      const result = await importAllGeositeRules(parsed.provider, parsed.clientId);
      const syncResult = await syncImportedGeositeRules(result.ruleNames);
      return c.json({
        success: true,
        ...result,
        sync: syncResult,
      });
    } catch (error) {
      return jsonError(c, error, "Failed to import geosite rules");
    }
  });

  app.post("/api/geosite/import-selected", async (c) => {
    try {
      const body = await c.req.json();
      const parsed = ImportSelectedSchema.parse(body);
      const clients = await getClients();
      if (!clients.some((client) => client.id === parsed.clientId)) {
        return c.json({ error: `Client "${parsed.clientId}" not found` }, 400);
      }
      const result = await importSelectedGeositeRules(parsed.provider, parsed.clientId, parsed.lists);
      const syncResult = await syncImportedGeositeRules(result.ruleNames);
      return c.json({
        success: true,
        ...result,
        sync: syncResult,
      });
    } catch (error) {
      return jsonError(c, error, "Failed to import selected geosite rules");
    }
  });

  app.get("/api/geosite/preview", async (c) => {
    try {
      const provider = GeositeProviderSchema.parse(c.req.query("provider"));
      const list = c.req.query("list");
      const clientId = c.req.query("client");
      const attrs = (c.req.query("attrs") || "")
        .split(",")
        .map((item) => item.trim())
        .filter(Boolean);

      if (!list || !clientId) {
        return c.json({ error: "provider, list and client are required" }, 400);
      }

      const clients = await getClients();
      if (!clients.some((client) => client.id === clientId)) {
        return c.json({ error: `Client "${clientId}" not found` }, 400);
      }

      const preview = await previewGeositeSelection(provider, list, clientId, attrs);
      return c.json({
        content: preview.content,
        totalEntries: preview.totalEntries,
      });
    } catch (error) {
      return jsonError(c, error, "Failed to preview geosite selection");
    }
  });
}
