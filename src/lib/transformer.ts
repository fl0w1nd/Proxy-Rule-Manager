import {
  TransformersConfig,
  Transform,
} from "./schema";

// 执行预定义脚本转换器
function executeScript(content: string, script: string): string {
  try {
    // 创建一个沙箱执行环境
    const fn = new Function("content", script + "\nreturn transform(content);");
    const result = fn(content);
    return typeof result === "string" ? result : content;
  } catch (error) {
    console.error("Script execution error:", error);
    return content;
  }
}


// 执行新版转换（支持指定目标来源）
function executeNewTransform(
  contents: string[],
  transform: Transform,
  transformersConfig: TransformersConfig
): string[] {
  const targetIndices = transform.target === "all"
    ? contents.map((_, i) => i)
    : (transform.target as number[]);
  
  return contents.map((content, index) => {
    if (!targetIndices.includes(index)) {
      return content;
    }
    
    switch (transform.type) {
      case "use": {
        const transformer = transformersConfig[transform.use || ""];
        if (transformer && transformer.script) {
          return executeScript(content, transformer.script);
        }
        return content;
      }
      
      case "replace": {
        if (!transform.pattern) return content;
        const regex = new RegExp(transform.pattern, transform.flags || "g");
        return content.replace(regex, transform.replacement || "");
      }
      
      case "remove_lines": {
        if (!transform.pattern) return content;
        const regex = new RegExp(transform.pattern);
        return content
          .split("\n")
          .filter((line) => !regex.test(line))
          .join("\n");
      }
      
      default:
        return content;
    }
  });
}

// 执行新版转换（对多个来源内容）
export function applyNewTransforms(
  contents: string[],
  transforms: Transform[],
  transformersConfig: TransformersConfig = {}
): string[] {
  let result = [...contents];
  
  for (const transform of transforms) {
    result = executeNewTransform(result, transform, transformersConfig);
  }
  
  return result;
}

// 合并多个内容
export function mergeContents(
  contents: string[],
  strategy: "concat" | "union" | "intersect",
  dedupe: boolean = false
): string {
  if (contents.length === 0) return "";
  
  switch (strategy) {
    case "concat": {
      let result = contents.join("\n");
      if (dedupe) {
        const lines = result.split("\n");
        const seen = new Set<string>();
        const deduped: string[] = [];
        for (const line of lines) {
          const trimmed = line.trim();
          if (trimmed && !seen.has(trimmed)) {
            seen.add(trimmed);
            deduped.push(line);
          } else if (!trimmed) {
            deduped.push(line);
          }
        }
        result = deduped.join("\n");
      }
      return result;
    }
    
    case "union": {
      const allLines = new Set<string>();
      for (const content of contents) {
        for (const line of content.split("\n")) {
          const trimmed = line.trim();
          if (trimmed) {
            allLines.add(trimmed);
          }
        }
      }
      return Array.from(allLines).join("\n");
    }
    
    case "intersect": {
      if (contents.length === 1) {
        return contents[0];
      }
      
      const lineSets = contents.map((content) => {
        const set = new Set<string>();
        for (const line of content.split("\n")) {
          const trimmed = line.trim();
          if (trimmed) {
            set.add(trimmed);
          }
        }
        return set;
      });
      
      const intersection = lineSets.reduce((acc, set) => {
        return new Set([...acc].filter((x) => set.has(x)));
      });
      
      return Array.from(intersection).join("\n");
    }
    
    default:
      return contents.join("\n");
  }
}

function normalizeLineEndings(content: string): string {
  return content.replace(/\r\n/g, "\n").replace(/\r/g, "\n");
}

function getEffectiveRuleLines(content: string | null | undefined): string[] {
  if (!content) return [];

  return normalizeLineEndings(content)
    .split("\n")
    .map((line) => line.trim())
    .filter((line) => line.length > 0 && !line.startsWith("#"));
}

function formatHeaderTimestamp(timestamp: string): string {
  const date = new Date(timestamp);
  if (Number.isNaN(date.getTime())) {
    return timestamp;
  }

  const pad = (value: number) => value.toString().padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}

function getRuleTypeCounts(lines: string[]): Array<[string, number]> {
  const counts = new Map<string, number>();

  for (const line of lines) {
    const type = line.split(",", 1)[0].trim() || "UNKNOWN";
    counts.set(type, (counts.get(type) || 0) + 1);
  }

  return Array.from(counts.entries()).sort((a, b) => a[0].localeCompare(b[0]));
}

// 为规则内容添加头部注释
export function addRuleHeader(
  content: string,
  ruleName: string,
  description?: string,
  updatedAt?: string
): string {
  void ruleName;
  void description;

  const normalizedContent = normalizeLineEndings(content);
  const effectiveLines = getEffectiveRuleLines(normalizedContent);
  const typeCounts = getRuleTypeCounts(effectiveLines);
  const timestamp = formatHeaderTimestamp(updatedAt || new Date().toISOString());

  const headerLines = [
    `# 规则数量：${effectiveLines.length} 条`,
    `# 更新时间：${timestamp}`,
    "# 规则类型：",
    ...typeCounts.map(([type, count]) => `# ${type}: ${count} 条`),
  ];

  return `${headerLines.join("\n")}\n\n${normalizedContent}`;
}

export function stripManagedRuleHeader(content: string | null | undefined): string {
  if (!content) return "";

  const normalizedContent = normalizeLineEndings(content);
  const lines = normalizedContent.split("\n");

  if (
    !lines[0]?.startsWith("# 规则数量：") ||
    !lines[1]?.startsWith("# 更新时间：") ||
    lines[2] !== "# 规则类型："
  ) {
    return normalizedContent;
  }

  let index = 3;
  while (index < lines.length && lines[index].startsWith("# ")) {
    index += 1;
  }
  while (index < lines.length && lines[index].trim() === "") {
    index += 1;
  }

  return lines.slice(index).join("\n");
}

export function normalizeEffectiveRuleContent(content: string | null | undefined): string {
  return getEffectiveRuleLines(content).join("\n");
}

// 计算内容的 SHA-256 哈希
export async function computeHash(content: string): Promise<string> {
  const encoder = new TextEncoder();
  const data = encoder.encode(content);
  const hashBuffer = await crypto.subtle.digest("SHA-256", data);
  const hashArray = Array.from(new Uint8Array(hashBuffer));
  return hashArray.map((b) => b.toString(16).padStart(2, "0")).join("");
}
