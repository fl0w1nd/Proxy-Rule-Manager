import { beforeEach, describe, expect, it, vi } from "vitest";
import { Hono } from "hono";

vi.mock("@/lib/storage-adapter", () => ({
  getConfig: vi.fn(),
  saveConfig: vi.fn(),
  getRuleContent: vi.fn(),
  getGeositeRuleContent: vi.fn(),
  deleteRuleContent: vi.fn(),
  deleteGeositeRuleContent: vi.fn(),
  getArtifactMeta: vi.fn(),
  getAllArtifactMetas: vi.fn(),
  deleteArtifactMeta: vi.fn(),
  deleteArtifactMetas: vi.fn(),
  renameRule: vi.fn(),
}));

vi.mock("@/lib/sync-engine", () => ({
  executePartialSync: vi.fn(),
}));

vi.mock("@/lib/activity-store", () => ({
  recordRuleFileChanges: vi.fn(),
}));

vi.mock("@/lib/diff", () => ({
  createLineDiff: vi.fn().mockReturnValue("mock-diff"),
  createActivityDiff: vi.fn().mockReturnValue("mock-diff"),
}));

import { registerRuleRoutes } from "@/server/routes/rules";
import {
  getConfig,
  saveConfig,
  getRuleContent,
  getAllArtifactMetas,
  deleteRuleContent,
  deleteArtifactMeta,
  deleteArtifactMetas,
} from "@/lib/storage-adapter";
import { recordRuleFileChanges } from "@/lib/activity-store";
import type { RulesConfig } from "@/lib/schema";

const mockedGetConfig = vi.mocked(getConfig);
const mockedSaveConfig = vi.mocked(saveConfig);
const mockedGetRuleContent = vi.mocked(getRuleContent);
const mockedGetAllArtifactMetas = vi.mocked(getAllArtifactMetas);
const mockedDeleteRuleContent = vi.mocked(deleteRuleContent);
const mockedDeleteArtifactMeta = vi.mocked(deleteArtifactMeta);
const mockedDeleteArtifactMetas = vi.mocked(deleteArtifactMetas);
const mockedRecordRuleFileChanges = vi.mocked(recordRuleFileChanges);

function makeConfig(rules: RulesConfig["rules"]): RulesConfig {
  return { version: 1, rules, transformers: {} };
}

function makeRule(name: string, clients: string[] = ["clash_meta"]) {
  return {
    name,
    description: "",
    tags: [],
    sources: [{ type: "url" as const, url: `https://example.com/${name}.txt` }],
    output: { clients },
    transforms: [],
  };
}

function makeRefRule(name: string, ref: string, clients: string[] = ["clash_meta"]) {
  return {
    name,
    description: "",
    tags: [],
    sources: [{ type: "ref" as const, ref }],
    output: { clients },
    transforms: [],
  };
}

function post(app: Hono, path: string, body: unknown) {
  return app.request(path, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(body),
  });
}

describe("POST /api/rules/batch-delete", () => {
  let app: Hono;

  beforeEach(() => {
    vi.clearAllMocks();
    app = new Hono();
    registerRuleRoutes(app);

    mockedSaveConfig.mockResolvedValue({ rev: 1 } as never);
    mockedDeleteRuleContent.mockResolvedValue(undefined as never);
    mockedDeleteArtifactMeta.mockResolvedValue(undefined as never);
    mockedDeleteArtifactMetas.mockResolvedValue(undefined as never);
    mockedGetAllArtifactMetas.mockResolvedValue([] as never);
    mockedRecordRuleFileChanges.mockResolvedValue(undefined as never);
  });

  it("returns 400 when ruleNames is empty", async () => {
    const res = await post(app, "/api/rules/batch-delete", { ruleNames: [] });
    expect(res.status).toBe(400);
  });

  it("returns 400 when ruleNames is missing", async () => {
    const res = await post(app, "/api/rules/batch-delete", {});
    expect(res.status).toBe(400);
  });

  it("deletes multiple rules in a single request", async () => {
    const config = makeConfig([makeRule("a"), makeRule("b"), makeRule("c")]);
    mockedGetConfig.mockResolvedValue(config as never);
    mockedGetRuleContent.mockResolvedValue("DOMAIN,example.com" as never);

    const res = await post(app, "/api/rules/batch-delete", {
      ruleNames: ["a", "b"],
    });

    expect(res.status).toBe(200);
    const json = await res.json();
    expect(json.success).toBe(true);
    expect(json.deleted).toEqual(["a", "b"]);
    expect(json.notFound).toEqual([]);

    // Only one saveConfig call for the entire batch
    expect(mockedSaveConfig).toHaveBeenCalledTimes(1);
    expect(mockedDeleteArtifactMetas).toHaveBeenCalledTimes(1);
    // Only one recordRuleFileChanges call for the entire batch
    expect(mockedRecordRuleFileChanges).toHaveBeenCalledTimes(1);
    // Remaining rules in saved config
    const savedConfig = mockedSaveConfig.mock.calls[0][0] as RulesConfig;
    expect(savedConfig.rules.map((r) => r.name)).toEqual(["c"]);
  });

  it("reports notFound rules without blocking others", async () => {
    const config = makeConfig([makeRule("a")]);
    mockedGetConfig.mockResolvedValue(config as never);
    mockedGetRuleContent.mockResolvedValue(null as never);

    const res = await post(app, "/api/rules/batch-delete", {
      ruleNames: ["a", "missing"],
    });

    expect(res.status).toBe(200);
    const json = await res.json();
    expect(json.deleted).toEqual(["a"]);
    expect(json.notFound).toEqual(["missing"]);
  });

  it("blocks deletion when an external rule depends on a target", async () => {
    const config = makeConfig([
      makeRule("base"),
      makeRefRule("dependent", "base"),
    ]);
    mockedGetConfig.mockResolvedValue(config as never);

    const res = await post(app, "/api/rules/batch-delete", {
      ruleNames: ["base"],
    });

    expect(res.status).toBe(400);
    const json = await res.json();
    expect(json.blocked).toEqual([
      { name: "base", dependents: ["dependent"] },
    ]);
    expect(mockedSaveConfig).not.toHaveBeenCalled();
  });

  it("allows co-deletion of rules that only depend on each other", async () => {
    const config = makeConfig([
      makeRule("base"),
      makeRefRule("child", "base"),
    ]);
    mockedGetConfig.mockResolvedValue(config as never);
    mockedGetRuleContent.mockResolvedValue(null as never);

    const res = await post(app, "/api/rules/batch-delete", {
      ruleNames: ["base", "child"],
    });

    expect(res.status).toBe(200);
    const json = await res.json();
    expect(json.deleted).toEqual(["base", "child"]);
    expect(json.blocked).toEqual([]);
  });

  it("records change records for rules with existing content", async () => {
    const config = makeConfig([makeRule("a", ["clash_meta", "surge"])]);
    mockedGetConfig.mockResolvedValue(config as never);
    mockedGetRuleContent.mockResolvedValue("DOMAIN,example.com" as never);

    const res = await post(app, "/api/rules/batch-delete", {
      ruleNames: ["a"],
    });

    expect(res.status).toBe(200);
    const records = mockedRecordRuleFileChanges.mock.calls[0][0];
    expect(records).toHaveLength(2);
    expect(records[0].changeType).toBe("deleted");
    expect(records[1].changeType).toBe("deleted");
    expect(records.map((r: { client: string }) => r.client).sort()).toEqual(["clash_meta", "surge"]);
  });
});
