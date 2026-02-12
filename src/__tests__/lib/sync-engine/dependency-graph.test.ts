import { describe, it, expect } from "vitest";
import {
    extractDependencies,
    detectCircularDependency,
    topologicalSort,
} from "@/lib/sync-engine/dependency-graph";
import type { RuleConfig } from "@/lib/schema";

// 创建测试规则的辅助函数
function createRule(
    name: string,
    options: {
        refs?: string[];
        clients?: string[];
    } = {}
): RuleConfig {
    const sources = options.refs?.map((ref) => ({ type: "ref" as const, ref }));
    return {
        name,
        sources,
        tags: [],
        output: {
            clients: options.clients || ["clash_meta"],
        },
    };
}

describe("extractDependencies", () => {
    it("should extract dependencies from sources with ref type", () => {
        const rule = createRule("A", { refs: ["D", "E"] });
        const deps = extractDependencies(rule);
        expect(deps).toEqual(new Set(["D", "E"]));
    });

    it("should return empty set for rule with no dependencies", () => {
        const rule = createRule("A");
        const deps = extractDependencies(rule);
        expect(deps).toEqual(new Set());
    });

    it("should handle duplicate dependencies", () => {
        const rule = createRule("A", { refs: ["B", "B"] });
        const deps = extractDependencies(rule);
        expect(deps).toEqual(new Set(["B"]));
        expect(deps.size).toBe(1);
    });
});

describe("detectCircularDependency", () => {
    it("should return null for rules with no dependencies", () => {
        const rules = [createRule("A"), createRule("B"), createRule("C")];
        expect(detectCircularDependency(rules)).toBeNull();
    });

    it("should return null for valid dependency chain", () => {
        const rules = [
            createRule("A", { refs: ["B"] }),
            createRule("B", { refs: ["C"] }),
            createRule("C"),
        ];
        expect(detectCircularDependency(rules)).toBeNull();
    });

    it("should detect simple cycle A → B → A", () => {
        const rules = [
            createRule("A", { refs: ["B"] }),
            createRule("B", { refs: ["A"] }),
        ];
        const cycle = detectCircularDependency(rules);
        expect(cycle).not.toBeNull();
        expect(cycle).toContain("A");
        expect(cycle).toContain("B");
    });

    it("should detect longer cycle A → B → C → A", () => {
        const rules = [
            createRule("A", { refs: ["B"] }),
            createRule("B", { refs: ["C"] }),
            createRule("C", { refs: ["A"] }),
        ];
        const cycle = detectCircularDependency(rules);
        expect(cycle).not.toBeNull();
        expect(cycle!.length).toBeGreaterThanOrEqual(3);
    });

    it("should detect self-reference A → A", () => {
        const rules = [createRule("A", { refs: ["A"] })];
        const cycle = detectCircularDependency(rules);
        expect(cycle).not.toBeNull();
        expect(cycle).toContain("A");
    });

    it("should return null for diamond dependency (no cycle)", () => {
        // A → B, A → C, B → D, C → D
        const rules = [
            createRule("A", { refs: ["B", "C"] }),
            createRule("B", { refs: ["D"] }),
            createRule("C", { refs: ["D"] }),
            createRule("D"),
        ];
        expect(detectCircularDependency(rules)).toBeNull();
    });

    it("should ignore dependencies to rules not in the set", () => {
        const rules = [createRule("A", { refs: ["NonExistent"] })];
        expect(detectCircularDependency(rules)).toBeNull();
    });
});

describe("topologicalSort", () => {
    it("should sort rules with no dependencies", () => {
        const rules = [createRule("C"), createRule("A"), createRule("B")];
        const sorted = topologicalSort(rules);
        expect(sorted.map((r) => r.name)).toHaveLength(3);
        // 所有规则都应该在结果中
        expect(sorted.map((r) => r.name)).toContain("A");
        expect(sorted.map((r) => r.name)).toContain("B");
        expect(sorted.map((r) => r.name)).toContain("C");
    });

    it("should place dependencies before dependents", () => {
        const rules = [
            createRule("A", { refs: ["B"] }),
            createRule("B", { refs: ["C"] }),
            createRule("C"),
        ];
        const sorted = topologicalSort(rules);
        const names = sorted.map((r) => r.name);
        expect(names.indexOf("C")).toBeLessThan(names.indexOf("B"));
        expect(names.indexOf("B")).toBeLessThan(names.indexOf("A"));
    });

    it("should handle complex dependency graph", () => {
        // A → B, C; B → D; C → D; D → E
        const rules = [
            createRule("A", { refs: ["B", "C"] }),
            createRule("B", { refs: ["D"] }),
            createRule("C", { refs: ["D"] }),
            createRule("D", { refs: ["E"] }),
            createRule("E"),
        ];
        const sorted = topologicalSort(rules);
        const names = sorted.map((r) => r.name);

        // E 必须在 D 之前
        expect(names.indexOf("E")).toBeLessThan(names.indexOf("D"));
        // D 必须在 B 和 C 之前
        expect(names.indexOf("D")).toBeLessThan(names.indexOf("B"));
        expect(names.indexOf("D")).toBeLessThan(names.indexOf("C"));
        // B 和 C 必须在 A 之前
        expect(names.indexOf("B")).toBeLessThan(names.indexOf("A"));
        expect(names.indexOf("C")).toBeLessThan(names.indexOf("A"));
    });

    it("should throw error for circular dependency", () => {
        const rules = [
            createRule("A", { refs: ["B"] }),
            createRule("B", { refs: ["A"] }),
        ];
        expect(() => topologicalSort(rules)).toThrow("循环依赖");
    });

    it("should throw for missing deps when skipMissingDepsCheck is false", () => {
        const rules = [createRule("A", { refs: ["NonExistent"] })];
        expect(() => topologicalSort(rules)).toThrow("依赖缺失");
    });

    it("should not throw for missing deps when skipMissingDepsCheck is true", () => {
        const rules = [createRule("A", { refs: ["NonExistent"] })];
        expect(() => topologicalSort(rules, true)).not.toThrow();
    });

    it("should handle subset of rules with skipMissingDepsCheck", () => {
        // 只处理 A，但 A 依赖 B（不在列表中）
        const rules = [createRule("A", { refs: ["B"] })];
        const sorted = topologicalSort(rules, true);
        expect(sorted).toHaveLength(1);
        expect(sorted[0].name).toBe("A");
    });
});
