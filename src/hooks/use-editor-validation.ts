import { useMemo } from "react";
import YAML from "yaml";

export interface ValidationError {
  line: number;
  column: number;
  message: string;
}

export interface ValidationResult {
  errors: ValidationError[];
  hasErrors: boolean;
}

/**
 * 解析 JSON.parse 错误信息中的位置
 * 常见格式：
 * - "Expected ',' or '}' ... at position 42"
 * - "Unexpected token } in JSON at position 42"
 */
function parseJsonError(content: string, error: unknown): ValidationError {
  const msg = error instanceof Error ? error.message : String(error);

  // 尝试提取 position
  const posMatch = msg.match(/position\s+(\d+)/i);
  if (posMatch) {
    const pos = parseInt(posMatch[1], 10);
    // 将字符偏移转换为行列
    let line = 1;
    let column = 1;
    for (let i = 0; i < pos && i < content.length; i++) {
      if (content[i] === "\n") {
        line++;
        column = 1;
      } else {
        column++;
      }
    }
    return { line, column, message: msg };
  }

  // 无法解析位置，放在第 1 行
  return { line: 1, column: 1, message: msg };
}

/**
 * 解析 YAML 解析错误
 * yaml v2.x 使用 YAMLParseError，含有 linePos 信息
 */
function parseYamlErrors(content: string): ValidationError[] {
  if (!content.trim()) return [];

  try {
    const doc = YAML.parseDocument(content);
    const errors: ValidationError[] = [];

    for (const err of doc.errors) {
      const pos = err.linePos;
      if (pos && pos.length > 0) {
        errors.push({
          line: pos[0].line,
          column: pos[0].col,
          message: err.message,
        });
      } else {
        errors.push({
          line: 1,
          column: 1,
          message: err.message,
        });
      }
    }

    return errors;
  } catch (e) {
    // 兜底
    const msg = e instanceof Error ? e.message : String(e);
    return [{ line: 1, column: 1, message: msg }];
  }
}

/**
 * 验证 JSON 内容
 */
function parseJsonErrors(content: string): ValidationError[] {
  if (!content.trim()) return [];

  try {
    JSON.parse(content);
    return [];
  } catch (e) {
    return [parseJsonError(content, e)];
  }
}

/**
 * 编辑器语法校验 hook
 *
 * @param content - 编辑器内容
 * @param language - Monaco 编辑器语言标识 (yaml / json / 其他)
 * @returns 校验结果 { errors, hasErrors }
 */
export function useEditorValidation(
  content: string,
  language: string
): ValidationResult {
  return useMemo(() => {
    let errors: ValidationError[] = [];

    if (language === "yaml") {
      errors = parseYamlErrors(content);
    } else if (language === "json") {
      errors = parseJsonErrors(content);
    }

    return { errors, hasErrors: errors.length > 0 };
  }, [content, language]);
}
