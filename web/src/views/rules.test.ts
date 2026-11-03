import { describe, expect, it } from 'vitest';
import {
  addInput,
  dissolveGroup,
  duplicateSource,
  inputSummary,
  moveInput,
  moveLosesProcessing,
  moveRuleOrder,
  moveSource,
  preprocessMode,
  readRule,
  refChoices,
  removeInput,
  ruleReferences,
  serializeRule,
  serializeSource,
  splitInput,
  validateRule,
  validateRuleAll,
} from './rules';

const clients = [{ id: 'mihomo', name: 'Mihomo' }, { id: 'sing-box', name: 'sing-box' }];
const files = [{ name: 'direct.list' }];
const rules = [
  { id: 'base', name: 'Base', sources: [{ url: 'https://example/base.list' }] },
  { id: 'child', name: 'Child', sources: [{ ref: 'base' }] },
];

describe('rule editor helpers', () => {
  it('serializes a full rule without dropping preprocess or tags', () => {
    const value = serializeRule(readRule({
      id: 'base',
      name: 'Base',
      tags: ['core'],
      preprocess: 'function process(content) { return content; }',
      sources: [{ url: 'https://example/base.list', label: 'remote' }],
      ops: [{ type: 'include_kinds', kinds: ['domain'] }],
      merge: { strategy: 'union' },
      outputs: ['mihomo'],
    }));
    expect(value).toEqual({
      id: 'base',
      name: 'Base',
      tags: ['core'],
      preprocess: 'function process(content) { return content; }',
      sources: [{ url: 'https://example/base.list', label: 'remote' }],
      ops: [{ type: 'include_kinds', kinds: ['domain'] }],
      outputs: ['mihomo'],
    });
  });

  it('writes compact geosite refs and omits empty optionals', () => {
    expect(serializeSource({ kind: 'geosite', provider: 'v2fly', list: 'google', attrs: 'cn, ads' })).toEqual({
      geosite: 'v2fly/google@cn,ads',
    });
    expect(serializeRule(readRule({ id: 'x', name: 'X', sources: [{ content: 'DOMAIN,x.example' }], outputs: [] }))).toEqual({
      id: 'x',
      name: 'X',
      sources: [{ content: 'DOMAIN,x.example' }],
      outputs: [],
    });
  });

  it('filters self and downstream rules from ref choices', () => {
    expect(refChoices(rules, 'base').map((item) => item.value)).toEqual([]);
    expect(refChoices(rules, 'child').map((item) => item.value)).toEqual(['base']);
    expect(ruleReferences(rules, 'base')).toEqual(['Child']);
  });

  it('reorders rule IDs by dropping onto a target', () => {
    expect(moveRuleOrder(['a', 'b', 'c'], 'c', 'a')).toEqual(['c', 'a', 'b']);
    expect(moveRuleOrder(['a', 'b', 'c'], 'a', 'c')).toEqual(['b', 'c', 'a']);
  });

  it('rejects missing sources, unknown files and cyclic refs', () => {
    expect(validateRule({ id: 'child', name: 'Child', sources: [{ kind: 'ref', ref: 'child' }], outputs: [] }, rules, clients, files, 'child')).toEqual({ path: 'sources', message: '来源 1 不能引用自身', source: 0 });
    expect(validateRule({ id: 'base', name: 'Base', sources: [{ kind: 'ref', ref: 'child' }], outputs: [] }, rules, clients, files, 'base')?.message).toContain('循环');
    expect(validateRule({ id: 'custom', name: 'Custom', sources: [{ kind: 'file', file: 'missing.list' }], outputs: ['mihomo'] }, rules, clients, files, '')?.message).toContain('不存在的本地文件');
    expect(validateRule({
      id: 'custom', name: 'Custom', sources: [{ kind: 'file', file: 'direct.list' }],
      ops: [{ type: 'include_kinds', kinds: ['domain'] }], outputs: ['mihomo', 'sing-box'],
    }, rules, clients, files, '')).toBeNull();
    expect(validateRule({ id: '', name: 'X', sources: [{ kind: 'url', url: 'https://a' }], outputs: [] }, rules, clients, files, '')?.path).toBe('id');
    expect(validateRule({ id: 'x', name: '', sources: [{ kind: 'url', url: 'https://a' }], outputs: [] }, rules, clients, files, '')?.path).toBe('name');
  });
});

it('round-trips grouped sources, overrides and explicit preprocessing opt-out', () => {
  const raw = {
    id: 'grouped', name: 'Grouped', outputs: ['mihomo'],
    preprocess: 'function process(content) { return content; }',
    sources: [
      { group: [{ url: 'https://example/a' }, { ref: 'base' }], preprocess: '', ops: [{ type: 'include_kinds', kinds: ['domain'] }] },
      { content: 'DOMAIN,b.example', preprocess: 'function process(content) { return content.trim(); }' },
    ],
  };
  expect(serializeRule(readRule(raw))).toEqual(raw);
  expect(ruleReferences([{ ...raw }], 'base')).toEqual(['Grouped']);
  expect(refChoices([raw, { id: 'base' }], 'base')).toEqual([]);
});

it('validates files and refs inside source groups', () => {
  const draft = readRule({ id: 'grouped', name: 'Grouped', outputs: [], sources: [{ group: [{ file: 'missing.list' }] }] });
  expect(validateRule(draft, rules, clients, files, '')?.message).toContain('不存在的本地文件');
});

describe('source rows', () => {
  const ops = [{ type: 'include_kinds', kinds: ['domain'] }];
  const a = { kind: 'url' as const, url: 'https://example/a', ops: [] };
  const b = { kind: 'url' as const, url: 'https://example/b', ops: [] };

  it('keeps untouched sources byte-for-byte when round-tripping', () => {
    for (const sources of [
      [{ url: 'https://example/a' }],
      [{ url: 'https://example/a', label: 'remote', preprocess: '', ops }],
      [{ group: [{ url: 'https://example/a' }] }],
      [{ group: [{ url: 'https://example/a', label: 'a' }, { file: 'direct.list' }], label: 'g', preprocess: 'function process(c) { return c; }', ops }],
      [{ ref: 'base' }, { geosite: 'v2fly/google@cn' }, { geoip: 'loyalsoldier/cn' }, { content: 'DOMAIN,a.com' }],
    ]) {
      const raw = { id: 'x', name: 'X', outputs: [], sources };
      expect(serializeRule(readRule(raw))).toEqual(raw);
    }
  });

  it('upgrades a plain source to a group only when a second input is added', () => {
    const plain = { ...a, label: 'remote', preprocess: '', ops };
    const [group] = addInput([plain], 0, b);
    expect(serializeSource(group)).toEqual({ group: [{ url: 'https://example/a', label: 'remote' }, { url: 'https://example/b' }], preprocess: '', ops });
    // 删到只剩一个输入时保持 group，不改写结构
    expect(serializeSource(removeInput([group], 0, 1)[0])).toEqual({ group: [{ url: 'https://example/a', label: 'remote' }], preprocess: '', ops });
    // 显式解散才退回普通来源，处理配置随之保留
    expect(serializeSource(dissolveGroup(removeInput([group], 0, 1), 0)[0])).toEqual({ url: 'https://example/a', label: 'remote', preprocess: '', ops });
  });

  it('splits an input out with the row processing so results stay the same', () => {
    const group = { kind: 'group' as const, group: [a, b], preprocess: '', ops };
    const next = splitInput([group], 0, 1);
    expect(next.map(serializeSource)).toEqual([
      { group: [{ url: 'https://example/a' }], preprocess: '', ops },
      { url: 'https://example/b', preprocess: '', ops },
    ]);
    expect(splitInput([group, b], 0, 0, 2).map(serializeSource)).toEqual([
      { group: [{ url: 'https://example/b' }], preprocess: '', ops },
      { url: 'https://example/b' },
      { url: 'https://example/a', preprocess: '', ops },
    ]);
  });

  it('moves inputs between rows and reports lost processing', () => {
    const custom = { ...b, preprocess: 'function process(c) { return c; }' };
    expect(moveLosesProcessing([a, custom], 1, 0)).toBe(true);
    expect(moveLosesProcessing([a, b], 1, 0)).toBe(false);
    expect(moveInput([a, custom], 1, 0, 0).map(serializeSource)).toEqual([{ group: [{ url: 'https://example/a' }, { url: 'https://example/b' }] }]);
    const group = { kind: 'group' as const, group: [a, b] };
    expect(moveLosesProcessing([group, custom], 0, 1)).toBe(false);
    expect(moveInput([group, custom], 0, 0, 1).map(serializeSource)).toEqual([
      { group: [{ url: 'https://example/b' }] },
      { group: [{ url: 'https://example/b' }, { url: 'https://example/a' }], preprocess: 'function process(c) { return c; }' },
    ]);
    expect(moveSource([a, b, group], 2, 0)).toEqual([group, a, b]);
    expect(duplicateSource([a], 0)).toEqual([a, a]);
  });

  it('numbers issues the way the editor shows them', () => {
    const draft = readRule({ id: 'x', name: 'X', outputs: [], sources: [
      { url: 'https://example/a' },
      { group: [{ url: '' }, { file: 'missing.list' }], label: '组' },
    ] });
    expect(validateRuleAll(draft, rules, clients, files, '')).toEqual([
      { path: 'sources', message: '请输入组 的输入 1的 URL', source: 1, input: 0 },
      { path: 'sources', message: '组 的输入 2 引用了不存在的本地文件', source: 1, input: 1 },
    ]);
  });

  it('summarizes inputs and preprocess modes', () => {
    expect(inputSummary({ kind: 'url', url: 'https://rules.example/a/b.list' })).toBe('rules.example/a/b.list');
    expect(inputSummary({ kind: 'content', content: 'DOMAIN,a.com\nDOMAIN,b.com\n' })).toBe('DOMAIN,a.com 等 2 行');
    expect(inputSummary({ kind: 'geosite', provider: 'v2fly', list: 'google', attrs: 'cn' })).toBe('v2fly/google@cn');
    expect(preprocessMode({ kind: 'url' })).toBe('inherit');
    expect(preprocessMode({ kind: 'url', preprocess: '' })).toBe('off');
    expect(preprocessMode({ kind: 'url', preprocess: 'x' })).toBe('custom');
  });
});
