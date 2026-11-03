import type { Completion, CompletionContext, CompletionResult, CompletionSource } from '@codemirror/autocomplete';
import { api, type LintReport, type RuleDiagnostic, type RuleSyntax } from '../api/client';

/** 行级诊断，line 从 1 开始；超出文档范围的行会被忽略。 */
export interface EditorDiagnostic { line: number; message: string; severity?: 'error' | 'warning' }

/** 后端不可用时的兜底：只含最常用的写法，保证插入栏和补全仍能工作。 */
export const fallbackRuleSyntax: RuleSyntax = {
  types: [
    'DOMAIN', 'DOMAIN-SUFFIX', 'DOMAIN-KEYWORD', 'DOMAIN-WILDCARD', 'DOMAIN-REGEX', 'GEOSITE',
    'IP-CIDR', 'IP-SUFFIX', 'IP-ASN', 'GEOIP', 'DST-PORT', 'SRC-PORT', 'PROCESS-NAME', 'NETWORK',
  ].map((name) => ({ name, kind: name.toLowerCase().replaceAll('-', '_') })),
  flags: ['no-resolve'],
};

let syntaxPromise: Promise<RuleSyntax> | null = null;

/** 规则语法表只取一次；失败时回退到内置列表，下次调用会重试。 */
export function loadRuleSyntax(): Promise<RuleSyntax> {
  syntaxPromise ??= api.getRuleSyntax().catch(() => {
    syntaxPromise = null;
    return fallbackRuleSyntax;
  });
  return syntaxPromise;
}

/** 这些类型的值可以带 no-resolve。 */
export const noResolveKinds = new Set(['ip_cidr', 'ip_suffix', 'ip_asn', 'geoip']);

export function formatRuleLine(type: string, value: string, noResolve = false): string {
  const parts = [type.trim().toUpperCase(), value.trim()];
  if (noResolve) parts.push('no-resolve');
  return parts.join(',');
}

/** YAML payload 文件里的一项：`  - 'DOMAIN,a.com'`。 */
export function formatYAMLItem(line: string): string {
  return `  - '${line.replaceAll("'", "''")}'`;
}

const reasonLabels: [RegExp, string][] = [
  [/^unknown rule type "(.+)"$/, '未知规则类型 $1'],
  [/^could not detect rule format$/, '无法识别规则格式（文件为空或没有可识别的规则行）'],
  [/^no payload\/rules list found in YAML$/, 'YAML 中没有 payload: 或 rules: 列表'],
  [/^empty value for (.+)$/, '$1 的值为空'],
  [/^not a TYPE,VALUE rule line$/, '不是 类型,值 格式的规则行'],
  [/^terminal rule \((MATCH|FINAL)\) is not valid inside a rule set$/, '规则集中不能包含终止规则 $1'],
  [/^nested (RULE-SET|DOMAIN-SET) references cannot be resolved$/, '不支持嵌套引用 $1'],
];

/** 把解析器的英文原因翻成中文；未覆盖的原样保留。 */
export function diagnosticReason(reason: string): string {
  for (const [pattern, label] of reasonLabels) {
    if (pattern.test(reason)) return reason.replace(pattern, label);
  }
  return reason;
}

export function diagnosticMessage(item: RuleDiagnostic): string {
  return item.line ? `第 ${item.line} 行：${diagnosticReason(item.reason)}` : diagnosticReason(item.reason);
}

export function editorDiagnostics(report: LintReport | null): EditorDiagnostic[] {
  return (report?.diagnostics ?? [])
    .filter((item) => item.line)
    .map((item) => ({ line: item.line!, message: diagnosticReason(item.reason) }));
}

/** 文件级（没有行号）的诊断，需要在编辑器外单独展示。 */
export function documentDiagnostics(report: LintReport | null): string[] {
  return (report?.diagnostics ?? []).filter((item) => !item.line).map((item) => diagnosticReason(item.reason));
}

export function formatLabel(format?: string): string {
  if (format === 'classical') return '规则行列表';
  if (format === 'mihomo-yaml') return 'YAML payload';
  return '未识别';
}

/**
 * 防抖校验：内容变化后等待 delay 毫秒再请求，新的输入会取消旧请求，
 * 过期结果不会回写。
 */
export function createLinter(onResult: (report: LintReport | null, error?: string) => void, delay = 400) {
  let timer: ReturnType<typeof setTimeout> | undefined;
  let controller: AbortController | undefined;
  let seq = 0;
  function cancel() {
    clearTimeout(timer);
    controller?.abort();
    seq++;
  }
  function run(content: string, name = '', immediate = false) {
    cancel();
    const id = seq;
    const exec = async () => {
      controller = new AbortController();
      try {
        const report = await api.lintRules(content, name, controller.signal);
        if (id === seq) onResult(report);
      } catch (error) {
        if (id === seq && (error as Error).name !== 'AbortError') onResult(null, (error as Error).message);
      }
    };
    if (immediate) void exec();
    else timer = setTimeout(exec, delay);
  }
  return { run, cancel };
}

/**
 * 规则行补全：行首（或 YAML 列表项开头）补全规则类型，`类型,值,` 之后补全参数。
 */
export function ruleCompletion(getSyntax: () => RuleSyntax): CompletionSource {
  return (context: CompletionContext): CompletionResult | null => {
    const line = context.state.doc.lineAt(context.pos);
    const before = line.text.slice(0, context.pos - line.from);
    const prefix = /^(\s*-\s*['"]?|\s*)/.exec(before)![0];
    const body = before.slice(prefix.length);
    if (body.startsWith('#') || body.startsWith('//') || body.startsWith(';')) return null;
    const syntax = getSyntax();
    const commas = body.split(',').length - 1;
    if (commas === 0) {
      const word = /^[A-Za-z][\w-]*$/.exec(body);
      if (!word && !context.explicit) return null;
      if (body && !word) return null;
      const options: Completion[] = syntax.types.map((type) => ({
        label: type.name,
        detail: type.alias ? `别名 · ${type.kind}` : type.kind,
        type: 'keyword',
        apply: `${type.name},`,
        boost: type.alias ? -10 : 0,
      }));
      return { from: line.from + prefix.length, options, validFor: /^[\w-]*$/ };
    }
    if (commas >= 2) {
      const last = body.slice(body.lastIndexOf(',') + 1);
      if (!/^[\w-]*$/.test(last)) return null;
      if (!last && !context.explicit) return null;
      return {
        from: context.pos - last.length,
        options: syntax.flags.map((flag) => ({ label: flag, type: 'property' })),
        validFor: /^[\w-]*$/,
      };
    }
    return null;
  };
}
