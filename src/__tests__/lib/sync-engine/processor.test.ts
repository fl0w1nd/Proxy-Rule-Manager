import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { processRule } from "@/lib/sync-engine/processor";
import type { RuleConfig, TransformersConfig, ClientConfig, ClientType } from "@/lib/schema";

// Mock 外部依赖
vi.mock("@/lib/sync-engine/fetcher", () => ({
    fetchSource: vi.fn(),
}));

vi.mock("@/lib/local-source-store", () => ({
    readLocalSourceContent: vi.fn(),
}));

import { fetchSource } from "@/lib/sync-engine/fetcher";
import { readLocalSourceContent } from "@/lib/local-source-store";

const mockedFetchSource = vi.mocked(fetchSource);
const mockedReadLocal = vi.mocked(readLocalSourceContent);

function createRule(overrides: Partial<RuleConfig> = {}): RuleConfig {
    return {
        name: "test-rule",
        sources: [],
        tags: [],
        output: { clients: ["clash_meta"] },
        ...overrides,
    };
}

describe("processRule", () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    describe("no sources", () => {
        it("should return error when rule has no sources", async () => {
            const rule = createRule({ sources: undefined });
            const result = await processRule(rule, {}, new Map());
            expect(result.errors).toContain("Rule has no sources");
            expect(result.contents.size).toBe(0);
        });
    });

    describe("url sources", () => {
        it("should fetch and process url source", async () => {
            mockedFetchSource.mockResolvedValueOnce({
                content: "DOMAIN,test.com",
                error: undefined,
            });
            const rule = createRule({
                sources: [{ type: "url", url: "https://example.com/rules.list" }],
            });
            const result = await processRule(rule, {}, new Map());
            expect(result.errors).toHaveLength(0);
            expect(result.contents.get("clash_meta")).toBe("DOMAIN,test.com");
        });

        it("should record error when fetch fails", async () => {
            mockedFetchSource.mockResolvedValueOnce({
                content: "",
                error: "Network error",
            });
            const rule = createRule({
                sources: [{ type: "url", url: "https://example.com/fail.list" }],
            });
            const result = await processRule(rule, {}, new Map());
            expect(result.errors.some((e) => e.includes("Network error"))).toBe(true);
        });
    });

    describe("local sources", () => {
        it("should use inline content", async () => {
            const rule = createRule({
                sources: [{ type: "local", content: "DOMAIN,local.com" }],
            });
            const result = await processRule(rule, {}, new Map());
            expect(result.contents.get("clash_meta")).toBe("DOMAIN,local.com");
        });

        it("should read content from contentRef", async () => {
            mockedReadLocal.mockResolvedValueOnce("DOMAIN,ref.com");
            const rule = createRule({
                sources: [{ type: "local", contentRef: "my-source.txt" }],
            });
            const result = await processRule(rule, {}, new Map());
            expect(result.contents.get("clash_meta")).toBe("DOMAIN,ref.com");
            expect(mockedReadLocal).toHaveBeenCalledWith("my-source.txt");
        });

        it("should skip when contentRef returns null", async () => {
            mockedReadLocal.mockResolvedValueOnce(null);
            const rule = createRule({
                sources: [{ type: "local", contentRef: "missing.txt" }],
            });
            const result = await processRule(rule, {}, new Map());
            expect(result.errors.some((e) => e.includes("No sources fetched"))).toBe(true);
        });
    });

    describe("ref sources", () => {
        it("should resolve ref from cache", async () => {
            const cache = new Map<string, Map<ClientType, string>>();
            cache.set("base-rule", new Map([["clash_meta", "DOMAIN,base.com"]]));
            const rule = createRule({
                sources: [{ type: "ref", ref: "base-rule" }],
            });
            const result = await processRule(rule, {}, cache);
            expect(result.contents.get("clash_meta")).toBe("DOMAIN,base.com");
        });

        it("should fallback to first available client content when target client not in ref", async () => {
            const cache = new Map<string, Map<ClientType, string>>();
            cache.set("base-rule", new Map([["shadowrocket", "DOMAIN,sr.com"]]));
            const rule = createRule({
                sources: [{ type: "ref", ref: "base-rule" }],
            });
            const result = await processRule(rule, {}, cache);
            expect(result.contents.get("clash_meta")).toBe("DOMAIN,sr.com");
        });

        it("should error when ref rule not in cache", async () => {
            const rule = createRule({
                sources: [{ type: "ref", ref: "nonexistent" }],
            });
            const result = await processRule(rule, {}, new Map());
            expect(result.errors.some((e) => e.includes("not found in cache"))).toBe(true);
        });

        it("should error when ref has empty contents", async () => {
            const cache = new Map<string, Map<ClientType, string>>();
            cache.set("empty-rule", new Map());
            const rule = createRule({
                sources: [{ type: "ref", ref: "empty-rule" }],
            });
            const result = await processRule(rule, {}, cache);
            expect(result.errors.some((e) => e.includes("has no content"))).toBe(true);
        });
    });

    describe("transforms", () => {
        it("should apply rule-level transforms", async () => {
            const rule = createRule({
                sources: [{ type: "local", content: "DOMAIN,old.com" }],
                transforms: [
                    { type: "replace", target: "all", pattern: "old", replacement: "new" },
                ],
            });
            const result = await processRule(rule, {}, new Map());
            expect(result.contents.get("clash_meta")).toBe("DOMAIN,new.com");
        });

        it("should not transform when transforms array is empty", async () => {
            const rule = createRule({
                sources: [{ type: "local", content: "DOMAIN,keep.com" }],
                transforms: [],
            });
            const result = await processRule(rule, {}, new Map());
            expect(result.contents.get("clash_meta")).toBe("DOMAIN,keep.com");
        });
    });

    describe("merge strategies", () => {
        it("should concat multiple sources by default", async () => {
            const rule = createRule({
                sources: [
                    { type: "local", content: "DOMAIN,a.com" },
                    { type: "local", content: "DOMAIN,b.com" },
                ],
            });
            const result = await processRule(rule, {}, new Map());
            expect(result.contents.get("clash_meta")).toBe("DOMAIN,a.com\nDOMAIN,b.com");
        });

        it("should apply union merge strategy", async () => {
            const rule = createRule({
                sources: [
                    { type: "local", content: "DOMAIN,a.com\nDOMAIN,b.com" },
                    { type: "local", content: "DOMAIN,b.com\nDOMAIN,c.com" },
                ],
                merge: { strategy: "union", dedupe: false },
            });
            const result = await processRule(rule, {}, new Map());
            const content = result.contents.get("clash_meta")!;
            expect(content).toContain("DOMAIN,a.com");
            expect(content).toContain("DOMAIN,b.com");
            expect(content).toContain("DOMAIN,c.com");
            // union 去重
            expect(content.split("\n").filter((l) => l === "DOMAIN,b.com")).toHaveLength(1);
        });
    });

    describe("multiple clients", () => {
        it("should produce output for each client", async () => {
            const rule = createRule({
                sources: [{ type: "local", content: "DOMAIN,test.com" }],
                output: { clients: ["clash_meta", "shadowrocket"] },
            });
            const result = await processRule(rule, {}, new Map());
            expect(result.contents.get("clash_meta")).toBe("DOMAIN,test.com");
            expect(result.contents.get("shadowrocket")).toBe("DOMAIN,test.com");
        });
    });

    describe("client overrides", () => {
        it("should apply client-specific transforms", async () => {
            const rule = createRule({
                sources: [{ type: "local", content: "DOMAIN,test.com" }],
                output: {
                    clients: ["clash_meta"],
                    client_overrides: {
                        clash_meta: {
                            enabled: true,
                            useGlobalTransforms: true,
                            transforms: [
                                { type: "replace", target: "all", pattern: "test", replacement: "clash" },
                            ],
                        },
                    },
                },
            });
            const result = await processRule(rule, {}, new Map());
            expect(result.contents.get("clash_meta")).toBe("DOMAIN,clash.com");
        });

        it("should skip transforms when client override is disabled", async () => {
            const rule = createRule({
                sources: [{ type: "local", content: "DOMAIN,test.com" }],
                output: {
                    clients: ["clash_meta"],
                    client_overrides: {
                        clash_meta: {
                            enabled: false,
                            useGlobalTransforms: true,
                            transforms: [
                                { type: "replace", target: "all", pattern: "test", replacement: "nope" },
                            ],
                        },
                    },
                },
            });
            const result = await processRule(rule, {}, new Map());
            expect(result.contents.get("clash_meta")).toBe("DOMAIN,test.com");
        });

        it("should apply client global transforms from clientsConfig", async () => {
            const clientsConfig: ClientConfig[] = [
                {
                    id: "clash_meta",
                    displayName: "Clash Meta",
                    pathName: "Clash Meta",
                    transforms: [
                        { type: "replace", target: "all", pattern: "DOMAIN", replacement: "DOMAIN-SUFFIX" },
                    ],
                },
            ];
            const rule = createRule({
                sources: [{ type: "local", content: "DOMAIN,test.com" }],
            });
            const result = await processRule(rule, {}, new Map(), clientsConfig);
            expect(result.contents.get("clash_meta")).toBe("DOMAIN-SUFFIX,test.com");
        });

        it("should skip global transforms when useGlobalTransforms is false", async () => {
            const clientsConfig: ClientConfig[] = [
                {
                    id: "clash_meta",
                    displayName: "Clash Meta",
                    pathName: "Clash Meta",
                    transforms: [
                        { type: "replace", target: "all", pattern: "DOMAIN", replacement: "CHANGED" },
                    ],
                },
            ];
            const rule = createRule({
                sources: [{ type: "local", content: "DOMAIN,test.com" }],
                output: {
                    clients: ["clash_meta"],
                    client_overrides: {
                        clash_meta: {
                            enabled: true,
                            useGlobalTransforms: false,
                            transforms: [],
                        },
                    },
                },
            });
            const result = await processRule(rule, {}, new Map(), clientsConfig);
            expect(result.contents.get("clash_meta")).toBe("DOMAIN,test.com");
        });

        it("should apply global then client override transforms in order", async () => {
            const clientsConfig: ClientConfig[] = [
                {
                    id: "clash_meta",
                    displayName: "Clash Meta",
                    pathName: "Clash Meta",
                    transforms: [
                        { type: "replace", target: "all", pattern: "DOMAIN", replacement: "DOMAIN-SUFFIX" },
                    ],
                },
            ];
            const rule = createRule({
                sources: [{ type: "local", content: "DOMAIN,test.com" }],
                output: {
                    clients: ["clash_meta"],
                    client_overrides: {
                        clash_meta: {
                            enabled: true,
                            useGlobalTransforms: true,
                            transforms: [
                                { type: "replace", target: "all", pattern: "test", replacement: "final" },
                            ],
                        },
                    },
                },
            });
            const result = await processRule(rule, {}, new Map(), clientsConfig);
            // 先全局 DOMAIN -> DOMAIN-SUFFIX，再覆盖 test -> final
            expect(result.contents.get("clash_meta")).toBe("DOMAIN-SUFFIX,final.com");
        });
    });

    describe("mixed sources", () => {
        it("should combine url and local sources", async () => {
            mockedFetchSource.mockResolvedValueOnce({
                content: "DOMAIN,remote.com",
                error: undefined,
            });
            const rule = createRule({
                sources: [
                    { type: "url", url: "https://example.com/rules.list" },
                    { type: "local", content: "DOMAIN,local.com" },
                ],
            });
            const result = await processRule(rule, {}, new Map());
            expect(result.contents.get("clash_meta")).toBe("DOMAIN,remote.com\nDOMAIN,local.com");
        });

        it("should combine ref and local sources", async () => {
            const cache = new Map<string, Map<ClientType, string>>();
            cache.set("base", new Map([["clash_meta", "DOMAIN,base.com"]]));
            const rule = createRule({
                sources: [
                    { type: "ref", ref: "base" },
                    { type: "local", content: "DOMAIN,extra.com" },
                ],
            });
            const result = await processRule(rule, {}, cache);
            expect(result.contents.get("clash_meta")).toBe("DOMAIN,base.com\nDOMAIN,extra.com");
        });
    });
});
