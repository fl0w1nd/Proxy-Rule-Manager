import {
  Transformer,
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

// 执行旧版转换步骤（用于兼容旧配置）
function executeLegacyStep(content: string, step: Transformer): string {
  if ("use" in step) {
    // 这是引用，需要在外层处理
    return content;
  }
  
  const typedStep = step as { type: string; [key: string]: unknown };
  
  switch (typedStep.type) {
    case "replace": {
      const regex = new RegExp(typedStep.pattern as string, (typedStep.flags as string) || "g");
      return content.replace(regex, typedStep.replacement as string);
    }
    
    case "remove_lines": {
      const regex = new RegExp(typedStep.pattern as string);
      return content
        .split("\n")
        .filter((line) => !regex.test(line))
        .join("\n");
    }
    
    case "regex_extract": {
      const regex = new RegExp(typedStep.pattern as string, "gm");
      const results: string[] = [];
      let match;
      while ((match = regex.exec(content)) !== null) {
        let output = typedStep.template as string;
        for (let i = 0; i < match.length; i++) {
          output = output.replace(new RegExp(`\\$${i}`, "g"), match[i] || "");
        }
        results.push(output);
      }
      return results.join("\n");
    }
    
    case "dedupe": {
      const lines = content.split("\n");
      const seen = new Set<string>();
      const result: string[] = [];
      for (const line of lines) {
        const trimmed = line.trim();
        if (trimmed && !seen.has(trimmed)) {
          seen.add(trimmed);
          result.push(line);
        } else if (!trimmed) {
          result.push(line);
        }
      }
      return result.join("\n");
    }
    
    case "sort": {
      const lines = content.split("\n");
      const comments: string[] = [];
      const rules: string[] = [];
      
      for (const line of lines) {
        if (line.trim().startsWith("#") || line.trim() === "") {
          comments.push(line);
        } else {
          rules.push(line);
        }
      }
      
      rules.sort((a, b) => {
        if (typedStep.order === "desc") {
          return b.localeCompare(a);
        }
        return a.localeCompare(b);
      });
      
      return [...comments, ...rules].join("\n");
    }
    
    case "trim": {
      return content
        .split("\n")
        .map((line) => line.trim())
        .join("\n");
    }
    
    case "normalize_eol": {
      return content.replace(/\r\n/g, "\n").replace(/\r/g, "\n");
    }
    
    default:
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

// 执行一系列旧版转换
export function applyTransforms(
  content: string,
  transformers: Transformer[],
  transformersConfig: TransformersConfig = {}
): string {
  let result = content;
  
  for (const transformer of transformers) {
    if ("use" in transformer) {
      // 引用预定义转换器
      const scriptTransformer = transformersConfig[transformer.use];
      if (scriptTransformer && scriptTransformer.script) {
        result = executeScript(result, scriptTransformer.script);
      }
    } else {
      result = executeLegacyStep(result, transformer);
    }
  }
  
  return result;
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
  const now = new Date().toISOString().replace("T", " ").split(".")[0];
  const lines = content.split("\n").filter((l) => l.trim() && !l.trim().startsWith("#"));
  
  const stats: Record<string, number> = {};
  for (const line of lines) {
    const type = line.split(",")[0];
    if (type) {
      stats[type] = (stats[type] || 0) + 1;
    }
  }
  
  const header = [
    `# NAME: ${ruleName}`,
    `# UPDATED: ${now}`,
  ];
  
  if (description) {
    header.push(`# DESCRIPTION: ${description}`);
  }
  
  for (const [type, count] of Object.entries(stats)) {
    header.push(`# ${type}: ${count}`);
  }
  
  header.push(`# TOTAL: ${lines.length}`);
  
  return [...header, content].join("\n");
}

// 计算内容的 SHA-256 哈希
export async function computeHash(content: string): Promise<string> {
  const encoder = new TextEncoder();
  const data = encoder.encode(content);
  const hashBuffer = await crypto.subtle.digest("SHA-256", data);
  const hashArray = Array.from(new Uint8Array(hashBuffer));
  return hashArray.map((b) => b.toString(16).padStart(2, "0")).join("");
}
