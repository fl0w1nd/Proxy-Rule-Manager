import { createTwoFilesPatch } from "diff";

const LARGE_ACTIVITY_DIFF_LINE_THRESHOLD = 500;
const LARGE_ACTIVITY_DIFF_CHAR_THRESHOLD = 50_000;

type ChangeType = "created" | "updated" | "deleted";

function countLines(content: string): number {
  if (!content) return 0;
  return content.split("\n").length;
}

function shouldSummarizeActivityDiff(content: string): boolean {
  return countLines(content) > LARGE_ACTIVITY_DIFF_LINE_THRESHOLD || content.length > LARGE_ACTIVITY_DIFF_CHAR_THRESHOLD;
}

function createLargeChangeSummary(changeType: Exclude<ChangeType, "updated">, content: string): string {
  return [
    `# ${changeType} summary`,
    "# full diff omitted for large payload",
    `# lines: ${countLines(content)}`,
    `# bytes: ${new TextEncoder().encode(content).length}`,
  ].join("\n");
}

export function createLineDiff(
  before: string | null,
  after: string,
  contextLines: number = 3
): string {
  const beforeText = before ?? "";
  return createTwoFilesPatch("before", "after", beforeText, after, undefined, undefined, {
    context: contextLines,
  });
}

export function createActivityDiff(
  changeType: ChangeType,
  before: string | null,
  after: string,
  contextLines: number = 3
): string {
  if (changeType === "updated") {
    return createLineDiff(before, after, contextLines);
  }

  const targetContent = changeType === "created" ? after : (before ?? "");
  if (shouldSummarizeActivityDiff(targetContent)) {
    return createLargeChangeSummary(changeType, targetContent);
  }

  return createLineDiff(before, after, contextLines);
}
