import { describe, it, expect } from "vitest";
import {
    mergeContents,
    applyNewTransforms,
    addRuleHeader,
    computeHash,
} from "./transformer";
import type { Transform, TransformersConfig } from "./schema";

describe("mergeContents", () => {
    describe("concat strategy", () => {
        it("should concatenate multiple contents", () => {
            const contents = ["line1\nline2", "line3\nline4"];
            const result = mergeContents(contents, "concat");
            expect(result).toBe("line1\nline2\nline3\nline4");
        });

        it("should dedupe when dedupe is true", () => {
            const contents = ["line1\nline2", "line2\nline3"];
            const result = mergeContents(contents, "concat", true);
            expect(result).toBe("line1\nline2\nline3");
        });

        it("should preserve empty lines when deduping", () => {
            const contents = ["line1\n\nline2", "\nline3"];
            const result = mergeContents(contents, "concat", true);
            expect(result.split("\n").filter((l) => l === "").length).toBeGreaterThan(0);
        });

        it("should return empty string for empty array", () => {
            expect(mergeContents([], "concat")).toBe("");
        });
    });

    describe("union strategy", () => {
        it("should return unique lines from all contents", () => {
            const contents = ["a\nb\nc", "b\nc\nd"];
            const result = mergeContents(contents, "union");
            const lines = result.split("\n");
            expect(lines).toHaveLength(4);
            expect(lines).toContain("a");
            expect(lines).toContain("b");
            expect(lines).toContain("c");
            expect(lines).toContain("d");
        });

        it("should ignore empty lines", () => {
            const contents = ["a\n\nb", "b\n\nc"];
            const result = mergeContents(contents, "union");
            expect(result.split("\n")).not.toContain("");
        });
    });

    describe("intersect strategy", () => {
        it("should return common lines only", () => {
            const contents = ["a\nb\nc", "b\nc\nd"];
            const result = mergeContents(contents, "intersect");
            const lines = result.split("\n");
            expect(lines).toContain("b");
            expect(lines).toContain("c");
            expect(lines).not.toContain("a");
            expect(lines).not.toContain("d");
        });

        it("should return original content for single input", () => {
            const contents = ["a\nb\nc"];
            const result = mergeContents(contents, "intersect");
            expect(result).toBe("a\nb\nc");
        });

        it("should return empty for no common lines", () => {
            const contents = ["a\nb", "c\nd"];
            const result = mergeContents(contents, "intersect");
            expect(result).toBe("");
        });
    });
});

describe("applyNewTransforms", () => {
    describe("replace transform", () => {
        it("should replace matching pattern", () => {
            const contents = ["DOMAIN,old.com", "DOMAIN,other.com"];
            const transforms: Transform[] = [
                { type: "replace", target: "all", pattern: "old\\.com", replacement: "new.com" },
            ];
            const result = applyNewTransforms(contents, transforms);
            expect(result[0]).toBe("DOMAIN,new.com");
            expect(result[1]).toBe("DOMAIN,other.com");
        });

        it("should only apply to specified targets", () => {
            const contents = ["line1", "line2", "line3"];
            const transforms: Transform[] = [
                { type: "replace", target: [0, 2], pattern: "line", replacement: "row" },
            ];
            const result = applyNewTransforms(contents, transforms);
            expect(result[0]).toBe("row1");
            expect(result[1]).toBe("line2"); // unchanged
            expect(result[2]).toBe("row3");
        });

        it("should handle empty pattern gracefully", () => {
            const contents = ["test"];
            const transforms: Transform[] = [{ type: "replace", target: "all" }];
            const result = applyNewTransforms(contents, transforms);
            expect(result[0]).toBe("test");
        });
    });

    describe("remove_lines transform", () => {
        it("should remove lines matching pattern", () => {
            const contents = ["# comment\nDOMAIN,test.com\n# another"];
            const transforms: Transform[] = [
                { type: "remove_lines", target: "all", pattern: "^#" },
            ];
            const result = applyNewTransforms(contents, transforms);
            expect(result[0]).toBe("DOMAIN,test.com");
        });

        it("should keep non-matching lines", () => {
            const contents = ["DOMAIN,a.com\nDOMAIN,b.com\nIP-CIDR,1.2.3.4/32"];
            const transforms: Transform[] = [
                { type: "remove_lines", target: "all", pattern: "^IP-CIDR" },
            ];
            const result = applyNewTransforms(contents, transforms);
            expect(result[0]).toBe("DOMAIN,a.com\nDOMAIN,b.com");
        });
    });

    describe("use transform", () => {
        it("should apply transformer script", () => {
            const contents = ["test content"];
            const transformers: TransformersConfig = {
                uppercase: {
                    name: "uppercase",
                    description: "Convert to uppercase",
                    script: "function transform(content) { return content.toUpperCase(); }",
                },
            };
            const transforms: Transform[] = [
                { type: "use", target: "all", use: "uppercase" },
            ];
            const result = applyNewTransforms(contents, transforms, transformers);
            expect(result[0]).toBe("TEST CONTENT");
        });

        it("should return original content if transformer not found", () => {
            const contents = ["test"];
            const transforms: Transform[] = [
                { type: "use", target: "all", use: "nonexistent" },
            ];
            const result = applyNewTransforms(contents, transforms);
            expect(result[0]).toBe("test");
        });
    });

    describe("chained transforms", () => {
        it("should apply multiple transforms in order", () => {
            const contents = ["# comment\nDOMAIN,old.com"];
            const transforms: Transform[] = [
                { type: "remove_lines", target: "all", pattern: "^#" },
                { type: "replace", target: "all", pattern: "old", replacement: "new" },
            ];
            const result = applyNewTransforms(contents, transforms);
            expect(result[0]).toBe("DOMAIN,new.com");
        });
    });
});

describe("addRuleHeader", () => {
    it("should return original content without modification", () => {
        const content = "DOMAIN,test.com";
        const result = addRuleHeader(content, "TestRule");
        expect(result).toBe(content);
    });

    it("should preserve existing comments and formatting", () => {
        const content = "# existing header\nDOMAIN,test.com";
        const result = addRuleHeader(content, "TestRule", "Test description");
        expect(result).toBe(content);
    });
});

describe("computeHash", () => {
    it("should return same hash for same content", async () => {
        const content = "test content";
        const hash1 = await computeHash(content);
        const hash2 = await computeHash(content);
        expect(hash1).toBe(hash2);
    });

    it("should return different hash for different content", async () => {
        const hash1 = await computeHash("content A");
        const hash2 = await computeHash("content B");
        expect(hash1).not.toBe(hash2);
    });

    it("should return 64-character hex string", async () => {
        const hash = await computeHash("test");
        expect(hash).toMatch(/^[0-9a-f]{64}$/);
    });

    it("should handle empty string", async () => {
        const hash = await computeHash("");
        expect(hash).toMatch(/^[0-9a-f]{64}$/);
    });

    it("should handle unicode content", async () => {
        const hash = await computeHash("中文内容 🎉");
        expect(hash).toMatch(/^[0-9a-f]{64}$/);
    });
});
