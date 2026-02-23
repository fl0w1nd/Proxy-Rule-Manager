import { describe, it, expect } from "vitest";
import {
    createTransformByType,
    getTransformTypeUpdates,
} from "@/lib/transform-utils";

describe("createTransformByType", () => {
    it("should create replace transform with pattern and replacement", () => {
        const transform = createTransformByType("replace");
        expect(transform).toEqual({
            type: "replace",
            target: "all",
            pattern: "",
            replacement: "",
        });
    });

    it("should create remove_lines transform with pattern", () => {
        const transform = createTransformByType("remove_lines");
        expect(transform).toEqual({
            type: "remove_lines",
            target: "all",
            pattern: "",
        });
    });

    it("should create use transform with only type and target", () => {
        const transform = createTransformByType("use");
        expect(transform).toEqual({
            type: "use",
            target: "all",
        });
    });
});

describe("getTransformTypeUpdates", () => {
    it("should return replace updates clearing use and flags", () => {
        const updates = getTransformTypeUpdates("replace");
        expect(updates).toEqual({
            type: "replace",
            use: undefined,
            pattern: "",
            replacement: "",
            flags: undefined,
        });
    });

    it("should return remove_lines updates clearing use, replacement, and flags", () => {
        const updates = getTransformTypeUpdates("remove_lines");
        expect(updates).toEqual({
            type: "remove_lines",
            use: undefined,
            pattern: "",
            replacement: undefined,
            flags: undefined,
        });
    });

    it("should return use updates with empty use and clearing pattern/replacement/flags", () => {
        const updates = getTransformTypeUpdates("use");
        expect(updates).toEqual({
            type: "use",
            use: "",
            pattern: undefined,
            replacement: undefined,
            flags: undefined,
        });
    });
});
