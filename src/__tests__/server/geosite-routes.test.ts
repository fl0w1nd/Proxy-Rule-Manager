import { beforeEach, describe, expect, it, vi } from "vitest";
import { Hono } from "hono";

vi.mock("@/lib/geosite", () => ({
  getGeositeCatalog: vi.fn(),
  getGeositeCatalogSummary: vi.fn(),
  importAllGeositeRules: vi.fn(),
  importSelectedGeositeRules: vi.fn(),
  lookupGeositeListsByDomain: vi.fn(),
  listGeositeProviders: vi.fn(),
  previewGeositeSelection: vi.fn(),
  refreshGeositeProvider: vi.fn(),
}));

vi.mock("@/lib/storage-adapter", () => ({
  getClients: vi.fn(),
  getConfig: vi.fn(),
}));

vi.mock("@/lib/sync-engine", () => ({
  executeBatchPartialSync: vi.fn(),
}));

import { registerGeositeRoutes } from "@/server/routes/geosite";
import { importSelectedGeositeRules } from "@/lib/geosite";
import { getClients } from "@/lib/storage-adapter";
import { executeBatchPartialSync } from "@/lib/sync-engine";

const mockedImportSelectedGeositeRules = vi.mocked(importSelectedGeositeRules);
const mockedGetClients = vi.mocked(getClients);
const mockedExecuteBatchPartialSync = vi.mocked(executeBatchPartialSync);

describe("registerGeositeRoutes", () => {
  let app: Hono;

  beforeEach(() => {
    vi.clearAllMocks();
    app = new Hono();
    registerGeositeRoutes(app);

    mockedGetClients.mockResolvedValue([
      { id: "clash_meta", displayName: "Clash Meta" },
    ] as never);
  });

  it("syncs imported geosite rules immediately after import", async () => {
    mockedImportSelectedGeositeRules.mockResolvedValue({
      created: 1,
      updated: 1,
      skipped: 0,
      total: 2,
      ruleNames: ["geosite_v2fly_google", "geosite_v2fly_openai"],
    } as never);
    mockedExecuteBatchPartialSync.mockResolvedValue({
      success: true,
      changedRules: ["geosite_v2fly_google", "geosite_v2fly_openai"],
      failedRules: [],
      jobId: "job-1",
    } as never);

    const response = await app.request("/api/geosite/import-selected", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        provider: "v2fly",
        clientId: "clash_meta",
        lists: ["google", "openai"],
      }),
    });

    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toMatchObject({
      success: true,
      ruleNames: ["geosite_v2fly_google", "geosite_v2fly_openai"],
      sync: {
        syncedRules: ["geosite_v2fly_google", "geosite_v2fly_openai"],
        failedRules: [],
      },
    });
    expect(mockedExecuteBatchPartialSync).toHaveBeenCalledTimes(1);
    expect(mockedExecuteBatchPartialSync).toHaveBeenCalledWith(["geosite_v2fly_google", "geosite_v2fly_openai"]);
  });

  it("returns sync failures while keeping successful import result", async () => {
    mockedImportSelectedGeositeRules.mockResolvedValue({
      created: 1,
      updated: 0,
      skipped: 0,
      total: 1,
      ruleNames: ["geosite_v2fly_google"],
    } as never);
    mockedExecuteBatchPartialSync.mockResolvedValue({
      success: false,
      changedRules: [],
      failedRules: [{ name: "geosite_v2fly_google", error: "provider cache missing" }],
      jobId: "job-3",
    } as never);

    const response = await app.request("/api/geosite/import-selected", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        provider: "v2fly",
        clientId: "clash_meta",
        lists: ["google"],
      }),
    });

    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toMatchObject({
      success: true,
      ruleNames: ["geosite_v2fly_google"],
      sync: {
        syncedRules: [],
        failedRules: [{ name: "geosite_v2fly_google", error: "provider cache missing" }],
      },
    });
  });
});
