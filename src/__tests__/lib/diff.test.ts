import { describe, expect, it } from "vitest";
import { createActivityDiff } from "@/lib/diff";

describe("createActivityDiff", () => {
  it("summarizes large created payloads", () => {
    const content = Array.from({ length: 600 }, (_, index) => `DOMAIN,line-${index}.example`).join("\n");

    const diff = createActivityDiff("created", null, content);

    expect(diff).toContain("# created summary");
    expect(diff).toContain("# full diff omitted for large payload");
    expect(diff).toContain("# lines: 600");
  });

  it("keeps full diff output for updates", () => {
    const before = "DOMAIN,before.example";
    const after = "DOMAIN,after.example";

    const diff = createActivityDiff("updated", before, after);

    expect(diff).toContain("--- before");
    expect(diff).toContain("+++ after");
  });

  it("summarizes large deleted payloads", () => {
    const before = Array.from({ length: 700 }, (_, index) => `DOMAIN,deleted-${index}.example`).join("\n");

    const diff = createActivityDiff("deleted", before, "");

    expect(diff).toContain("# deleted summary");
    expect(diff).toContain("# lines: 700");
  });
});
