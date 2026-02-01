import { createTwoFilesPatch } from "diff";

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
