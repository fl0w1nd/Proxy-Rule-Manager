import { describe, expect, it } from 'vitest';
import {
  moveRuleOrder,
  readRule,
  refChoices,
  ruleReferences,
  serializeRule,
  serializeSource,
  validateRule,
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
    expect(validateRule({ id: 'child', name: 'Child', sources: [{ kind: 'ref', ref: 'child' }], outputs: [] }, rules, clients, files, 'child')).toBe('来源 1 不能引用自身');
    expect(validateRule({ id: 'base', name: 'Base', sources: [{ kind: 'ref', ref: 'child' }], outputs: [] }, rules, clients, files, 'base')).toContain('循环');
    expect(validateRule({ id: 'custom', name: 'Custom', sources: [{ kind: 'file', file: 'missing.list' }], outputs: ['mihomo'] }, rules, clients, files, '')).toContain('不存在的本地文件');
    expect(validateRule({
      id: 'custom', name: 'Custom', sources: [{ kind: 'file', file: 'direct.list' }],
      ops: [{ type: 'include_kinds', kinds: ['domain'] }], outputs: ['mihomo', 'sing-box'],
    }, rules, clients, files, '')).toBe('');
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
  expect(validateRule(draft, rules, clients, files, '')).toContain('不存在的本地文件');
});
