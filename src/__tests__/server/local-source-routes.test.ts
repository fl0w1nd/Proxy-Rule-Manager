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

vi.mock("@/lib/local-source-store", () => ({
  saveLocalSourceContent: vi.fn(),
}));

vi.mock("@/lib/activity-store", () => ({
  recordRuleFileChanges: vi.fn(),
}));

vi.mock("@/lib/diff", () => ({
  createLineDiff: vi.fn().mockReturnValue("mock-diff"),
  createActivityDiff: vi.fn().mockReturnValue("mock-diff"),
}));

import { registerRuleRoutes } from "@/server/routes/rules";
import { getConfig } from "@/lib/storage-adapter";
import { executePartialSync } from "@/lib/sync-engine";
import { saveLocalSourceContent } from "@/lib/local-source-store";
import type { RulesConfig } from "@/lib/schema";

const mockedGetConfig = vi.mocked(getConfig);
const mockedExecutePartialSync = vi.mocked(executePartialSync);
const mockedSaveLocalSourceContent = vi.mocked(saveLocalSourceContent);

function makeConfig(rules: RulesConfig["rules"]): RulesConfig {
  return { version: 1, rules, transformers: {} };
}

function makeLocalRule(name: string, clients: string[] = ["clash_meta"]) {
  return {
    name,
    description: "",
    tags: [],
    sources: [
      { type: "local" as const, contentRef: "abc-123.txt", name: "custom list" },
    ],
    output: { clients },
    transforms: [],
  };
}

function makeMixedRule(name: string, clients: string[] = ["clash_meta"]) {
  return {
    name,
    description: "",
    tags: [],
    sources: [
      { type: "url" as const, url: "https://example.com/list.txt" },
      { type: "local" as const, contentRef: "def-456.txt", name: "local part" },
      { type: "local" as const, contentRef: "ghi-789.txt" },
    ],
    output: { clients },
    transforms: [],
  };
}

function makeUrlRule(name: string, clients: string[] = ["clash_meta"]) {
  return {
    name,
    description: "",
    tags: [],
    sources: [{ type: "url" as const, url: "https://example.com/list.txt" }],
    output: { clients },
    transforms: [],
  };
}

let app: Hono;

beforeEach(() => {
  vi.clearAllMocks();
  app = new Hono();
  registerRuleRoutes(app);
});

function put(path: string, body: unknown) {
  return app.request(path, {
    method: "PUT",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(body),
  });
}

describe("GET /api/rules/local-sources", () => {
  it("returns rules with local sources", async () => {
    mockedGetConfig.mockResolvedValue(
      makeConfig([makeLocalRule("rule-a"), makeUrlRule("rule-b"), makeMixedRule("rule-c")]) as never,
    );

    const res = await app.request("/api/rules/local-sources");
    expect(res.status).toBe(200);
    const json = await res.json();

    expect(json.rules).toHaveLength(2);
    expect(json.rules[0].ruleName).toBe("rule-a");
    expect(json.rules[0].sources).toEqual([
      { sourceIndex: 0, name: "custom list", contentRef: "abc-123.txt" },
    ]);

    expect(json.rules[1].ruleName).toBe("rule-c");
    expect(json.rules[1].sources).toEqual([
      { sourceIndex: 1, name: "local part", contentRef: "def-456.txt" },
      { sourceIndex: 2, name: null, contentRef: "ghi-789.txt" },
    ]);
  });

  it("returns empty array when no rules have local sources", async () => {
    mockedGetConfig.mockResolvedValue(makeConfig([makeUrlRule("rule-a")]) as never);

    const res = await app.request("/api/rules/local-sources");
    expect(res.status).toBe(200);
    const json = await res.json();
    expect(json.rules).toEqual([]);
  });

  it("returns empty array when no rules exist", async () => {
    mockedGetConfig.mockResolvedValue(makeConfig([]) as never);

    const res = await app.request("/api/rules/local-sources");
    expect(res.status).toBe(200);
    const json = await res.json();
    expect(json.rules).toEqual([]);
  });
});

describe("PUT /api/rules/:ruleName/local-source", () => {
  it("updates local source content and triggers sync", async () => {
    mockedGetConfig.mockResolvedValue(makeConfig([makeLocalRule("rule-a")]) as never);
    mockedSaveLocalSourceContent.mockResolvedValue("abc-123.txt" as never);
    mockedExecutePartialSync.mockResolvedValue({
      success: true,
      changedRules: ["rule-a"],
      failedRules: [],
    } as never);

    const res = await put("/api/rules/rule-a/local-source", {
      sourceIndex: 0,
      content: "DOMAIN,example.com",
    });

    expect(res.status).toBe(200);
    const json = await res.json();
    expect(json.success).toBe(true);
    expect(json.ruleName).toBe("rule-a");
    expect(json.sourceIndex).toBe(0);
    expect(json.contentRef).toBe("abc-123.txt");
    expect(json.sync.success).toBe(true);

    expect(mockedSaveLocalSourceContent).toHaveBeenCalledWith("abc-123.txt", "DOMAIN,example.com");
    expect(mockedExecutePartialSync).toHaveBeenCalledWith("rule-a");
  });

  it("returns 400 when sourceIndex is missing", async () => {
    const res = await put("/api/rules/rule-a/local-source", {
      content: "DOMAIN,example.com",
    });
    expect(res.status).toBe(400);
  });

  it("returns 400 when sourceIndex is negative", async () => {
    const res = await put("/api/rules/rule-a/local-source", {
      sourceIndex: -1,
      content: "DOMAIN,example.com",
    });
    expect(res.status).toBe(400);
  });

  it("returns 400 when content is missing", async () => {
    const res = await put("/api/rules/rule-a/local-source", {
      sourceIndex: 0,
    });
    expect(res.status).toBe(400);
  });

  it("returns 404 when rule does not exist", async () => {
    mockedGetConfig.mockResolvedValue(makeConfig([]) as never);

    const res = await put("/api/rules/nonexistent/local-source", {
      sourceIndex: 0,
      content: "DOMAIN,example.com",
    });
    expect(res.status).toBe(404);
    const json = await res.json();
    expect(json.error).toBe("Rule not found");
  });

  it("returns 404 when sourceIndex is out of range", async () => {
    mockedGetConfig.mockResolvedValue(makeConfig([makeLocalRule("rule-a")]) as never);

    const res = await put("/api/rules/rule-a/local-source", {
      sourceIndex: 5,
      content: "DOMAIN,example.com",
    });
    expect(res.status).toBe(404);
  });

  it("returns 404 when source at index is not local type", async () => {
    mockedGetConfig.mockResolvedValue(makeConfig([makeMixedRule("rule-a")]) as never);

    const res = await put("/api/rules/rule-a/local-source", {
      sourceIndex: 0,
      content: "DOMAIN,example.com",
    });
    expect(res.status).toBe(404);
    const json = await res.json();
    expect(json.error).toContain("not a local source");
  });

  it("works with mixed sources by targeting a local source index", async () => {
    mockedGetConfig.mockResolvedValue(makeConfig([makeMixedRule("rule-a")]) as never);
    mockedSaveLocalSourceContent.mockResolvedValue("def-456.txt" as never);
    mockedExecutePartialSync.mockResolvedValue({
      success: true,
      changedRules: ["rule-a"],
      failedRules: [],
    } as never);

    const res = await put("/api/rules/rule-a/local-source", {
      sourceIndex: 1,
      content: "DOMAIN,new.com",
    });

    expect(res.status).toBe(200);
    const json = await res.json();
    expect(json.success).toBe(true);
    expect(json.contentRef).toBe("def-456.txt");
    expect(mockedSaveLocalSourceContent).toHaveBeenCalledWith("def-456.txt", "DOMAIN,new.com");
  });

  it("handles URL-encoded rule names", async () => {
    const ruleName = "my rule / special";
    mockedGetConfig.mockResolvedValue(
      makeConfig([{ ...makeLocalRule(ruleName) }]) as never,
    );
    mockedSaveLocalSourceContent.mockResolvedValue("abc-123.txt" as never);
    mockedExecutePartialSync.mockResolvedValue({
      success: true,
      changedRules: [ruleName],
      failedRules: [],
    } as never);

    const res = await put(`/api/rules/${encodeURIComponent(ruleName)}/local-source`, {
      sourceIndex: 0,
      content: "DOMAIN,example.com",
    });

    expect(res.status).toBe(200);
    const json = await res.json();
    expect(json.ruleName).toBe(ruleName);
  });
});
