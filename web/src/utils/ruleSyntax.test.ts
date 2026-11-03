import { afterEach, describe, expect, it, vi } from 'vitest';
import { EditorState } from '@codemirror/state';
import { CompletionContext, type CompletionResult } from '@codemirror/autocomplete';
import { api } from '../api/client';
import {
  createLinter, diagnosticMessage, diagnosticReason, editorDiagnostics, formatRuleLine, formatYAMLItem, ruleCompletion,
} from './ruleSyntax';

const syntax = {
  types: [
    { name: 'DOMAIN', kind: 'domain' },
    { name: 'DOMAIN-SUFFIX', kind: 'domain_suffix' },
    { name: 'HOST', kind: 'domain', alias: true },
  ],
  flags: ['no-resolve', 'src'],
};

function complete(doc: string, explicit = false) {
  const state = EditorState.create({ doc });
  return ruleCompletion(() => syntax)(new CompletionContext(state, doc.length, explicit)) as CompletionResult;
}

afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks(); });

describe('rule syntax helpers', () => {
  it('formats rule lines and YAML items', () => {
    expect(formatRuleLine('ip-cidr', ' 10.0.0.0/8 ', true)).toBe('IP-CIDR,10.0.0.0/8,no-resolve');
    expect(formatYAMLItem("DOMAIN,it's.example")).toBe("  - 'DOMAIN,it''s.example'");
  });

  it('translates parser reasons and keeps unknown ones', () => {
    expect(diagnosticReason('unknown rule type "FOO"')).toBe('未知规则类型 FOO');
    expect(diagnosticReason('something else')).toBe('something else');
    expect(diagnosticMessage({ line: 4, text: 'x', reason: 'empty value for domain' })).toBe('第 4 行：domain 的值为空');
    expect(editorDiagnostics({
      confidence: 0, entries: 0, kinds: [], diagnostic_count: 2, lines: [], warnings: [],
      diagnostics: [{ text: '(document)', reason: 'x' }, { line: 2, text: 'y', reason: 'z' }],
    })).toEqual([{ line: 2, message: 'z' }]);
  });

  it('completes rule types at the start of a line and flags after the value', () => {
    const types = complete('DOM');
    expect(types.from).toBe(0);
    expect(types.options.map((item) => item.label)).toEqual(['DOMAIN', 'DOMAIN-SUFFIX', 'HOST']);
    expect(types.options[0].apply).toBe('DOMAIN,');
    expect(complete('  - DOM').from).toBe(4);
    expect(complete('IP-CIDR,1.1.1.1/32,no').options.map((item) => item.label)).toEqual(['no-resolve', 'src']);
    expect(complete('DOMAIN,exa')).toBeNull();
    expect(complete('# DOM')).toBeNull();
    expect(complete('')).toBeNull();
    expect(complete('', true).options).toHaveLength(3);
  });

  it('debounces lint requests and drops stale results', async () => {
    vi.useFakeTimers();
    const report = { confidence: 1, entries: 1, kinds: [], diagnostics: [], diagnostic_count: 0, lines: [], warnings: [] };
    const lint = vi.spyOn(api, 'lintRules').mockResolvedValue(report);
    const results: unknown[] = [];
    const linter = createLinter((value) => results.push(value), 100);
    linter.run('a');
    linter.run('b', 'x.list');
    await vi.advanceTimersByTimeAsync(150);
    expect(lint).toHaveBeenCalledOnce();
    expect(lint).toHaveBeenCalledWith('b', 'x.list', expect.any(AbortSignal));
    expect(results).toEqual([report]);
    linter.run('c');
    linter.cancel();
    await vi.advanceTimersByTimeAsync(150);
    expect(lint).toHaveBeenCalledOnce();
  });
});
