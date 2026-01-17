type DiffOp = { type: "equal" | "insert" | "delete"; line: string };

function splitLines(text: string): string[] {
  if (!text) return [];
  return text.replace(/\r\n/g, "\n").replace(/\r/g, "\n").split("\n");
}

function diffLines(oldLines: string[], newLines: string[]): DiffOp[] {
  const n = oldLines.length;
  const m = newLines.length;
  const max = n + m;
  const offset = max;
  const v = new Array(2 * max + 1).fill(0);
  const trace: number[][] = [];

  for (let d = 0; d <= max; d++) {
    trace.push(v.slice());
    for (let k = -d; k <= d; k += 2) {
      const kIndex = k + offset;
      const down = v[kIndex + 1] ?? 0;
      const right = v[kIndex - 1] ?? 0;
      let x: number;

      if (k === -d || (k !== d && right < down)) {
        x = down;
      } else {
        x = right + 1;
      }

      let y = x - k;
      while (x < n && y < m && oldLines[x] === newLines[y]) {
        x += 1;
        y += 1;
      }
      v[kIndex] = x;
      if (x >= n && y >= m) {
        return backtrack(trace, oldLines, newLines, offset);
      }
    }
  }

  return [];
}

function backtrack(
  trace: number[][],
  oldLines: string[],
  newLines: string[],
  offset: number
): DiffOp[] {
  let x = oldLines.length;
  let y = newLines.length;
  const edits: DiffOp[] = [];

  for (let d = trace.length - 1; d >= 0; d--) {
    const v = trace[d];
    const k = x - y;
    const kIndex = k + offset;
    const down = v[kIndex + 1] ?? 0;
    const right = v[kIndex - 1] ?? 0;
    let prevK: number;

    if (k === -d || (k !== d && right < down)) {
      prevK = k + 1;
    } else {
      prevK = k - 1;
    }

    const prevX = v[prevK + offset] ?? 0;
    const prevY = prevX - prevK;

    while (x > prevX && y > prevY) {
      edits.push({ type: "equal", line: oldLines[x - 1] });
      x -= 1;
      y -= 1;
    }

    if (d === 0) {
      break;
    }

    if (x === prevX) {
      edits.push({ type: "insert", line: newLines[y - 1] });
      y -= 1;
    } else {
      edits.push({ type: "delete", line: oldLines[x - 1] });
      x -= 1;
    }
  }

  return edits.reverse();
}

export function createLineDiff(
  before: string | null,
  after: string,
  contextLines: number = 3
): string {
  const beforeText = before ?? "";
  const oldLines = splitLines(beforeText);
  const newLines = splitLines(after);
  const ops = diffLines(oldLines, newLines);
  const output: string[] = ["--- before", "+++ after"];

  let oldLine = 1;
  let newLine = 1;
  let hunkLines: string[] = [];
  let hunkOldStart = 0;
  let hunkNewStart = 0;
  let pendingContext = 0;
  const preContext: string[] = [];

  const flushHunk = () => {
    if (hunkLines.length === 0) return;
    let oldCount = 0;
    let newCount = 0;
    for (const line of hunkLines) {
      if (line.startsWith("-")) {
        oldCount += 1;
      } else if (line.startsWith("+")) {
        newCount += 1;
      } else {
        oldCount += 1;
        newCount += 1;
      }
    }
    output.push(`@@ -${hunkOldStart},${oldCount} +${hunkNewStart},${newCount} @@`);
    output.push(...hunkLines);
    hunkLines = [];
  };

  const pushContextLine = (line: string) => {
    hunkLines.push(` ${line}`);
    pendingContext = Math.max(0, pendingContext - 1);
  };

  for (const op of ops) {
    if (op.type === "equal") {
      if (hunkLines.length > 0) {
        if (pendingContext > 0) {
          pushContextLine(op.line);
          if (pendingContext === 0) {
            flushHunk();
          }
        } else {
          flushHunk();
          preContext.length = 0;
          preContext.push(op.line);
        }
      } else {
        preContext.push(op.line);
        if (preContext.length > contextLines) {
          preContext.shift();
        }
      }
      oldLine += 1;
      newLine += 1;
      continue;
    }

    if (hunkLines.length === 0) {
      hunkOldStart = oldLine - preContext.length;
      hunkNewStart = newLine - preContext.length;
      for (const line of preContext) {
        hunkLines.push(` ${line}`);
      }
      preContext.length = 0;
    }

    if (op.type === "delete") {
      hunkLines.push(`-${op.line}`);
      oldLine += 1;
    } else {
      hunkLines.push(`+${op.line}`);
      newLine += 1;
    }
    pendingContext = contextLines;
  }

  flushHunk();

  return output.join("\n");
}
