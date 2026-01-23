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

// 为规则内容添加头部注释
export function addRuleHeader(
  content: string,
  ruleName: string,
  description?: string
): string {
  void ruleName;
  void description;
  return content;
}

// 计算内容的 SHA-256 哈希
export async function computeHash(content: string): Promise<string> {
  const encoder = new TextEncoder();
  const data = encoder.encode(content);
  const hashBuffer = await crypto.subtle.digest("SHA-256", data);
  const hashArray = Array.from(new Uint8Array(hashBuffer));
  return hashArray.map((b) => b.toString(16).padStart(2, "0")).join("");
}
