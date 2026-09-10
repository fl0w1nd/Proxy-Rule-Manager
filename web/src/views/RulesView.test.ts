import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { EditorView } from '@codemirror/view';
import { api } from '../api/client';
import RulesView from './RulesView.svelte';

/** Replaces the content of the preprocess CodeEditor found by its aria-label. */
function typePreprocessScript(host: HTMLElement, script: string) {
  const view = EditorView.findFromDOM(host);
  if (!view) throw new Error('CodeEditor not found for host element');
  view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: script } });
}

const clients = [
  { id: 'mihomo', name: 'Mihomo', template: 'mihomo-classical' },
  { id: 'sing-box', name: 'sing-box', template: 'singbox' },
];
const files = [{ name: 'direct.list', size: 24, lines: 2, modified_at: '2026-09-08T12:00:00.000Z' }];
const existing = {
  id: 'base',
  name: 'Base',
  tags: ['core'],
  preprocess: 'function process(content) { return content; }',
  sources: [{ url: 'https://example/base.list' }],
  outputs: ['mihomo'],
};

beforeAll(() => {
  HTMLDialogElement.prototype.showModal = function showModal() { this.setAttribute('open', ''); };
  HTMLDialogElement.prototype.close = function close() { this.removeAttribute('open'); };
});
beforeEach(() => {
  vi.spyOn(api, 'getConfig').mockResolvedValue({ version: 4, config: { clients, rules: [existing] } });
  vi.spyOn(api, 'getRules').mockResolvedValue({ items: [{ id: 'base', name: 'Base', entries: 12, version_at: '2026-09-08T12:00:00.000Z' }] });
  vi.spyOn(api, 'listLocalFiles').mockResolvedValue({ items: files });
  vi.spyOn(api, 'listTemplates').mockResolvedValue({ items: [
    { id: 'mihomo-classical', name: 'Classical', codec: 'text', extension: '.list', builtin: true },
  ] });
});
afterEach(() => vi.restoreAllMocks());

describe('RulesView', () => {
  it('updates selected rules and disables the batch action during an update', async () => {
    vi.mocked(api.getConfig).mockResolvedValue({
      version: 4,
      config: { clients, rules: [existing, { id: 'child', name: 'Child', sources: [{ ref: 'base' }], outputs: ['mihomo'] }] },
    });
    const onStartUpdate = vi.fn();
    const { rerender } = render(RulesView, { onStartUpdate });
    await screen.findByText('Child');
    await fireEvent.click(screen.getByRole('switch', { name: '选择' }));
    const toolbar = screen.getByText('已选 0').parentElement!;
    const update = within(toolbar).getByRole('button', { name: '更新' });
    expect(update).toBeDisabled();
    await fireEvent.click(screen.getByLabelText('选择 Base'));
    await fireEvent.click(screen.getByLabelText('选择 Child'));
    expect(update).toBeEnabled();
    await fireEvent.click(update);
    expect(onStartUpdate).toHaveBeenCalledExactlyOnceWith('rules', ['base', 'child']);
    await rerender({ onStartUpdate, isUpdating: true });
    expect(update).toBeDisabled();
  });

  it('creates a filtered local-file rule and keeps existing fields on save', async () => {
    const save = vi.spyOn(api, 'patchConfig').mockResolvedValue({ version: 5, warnings: [] });
    render(RulesView, { onStartUpdate: vi.fn() });
    await waitFor(() => expect(screen.getByRole('button', { name: '新建规则' })).toBeEnabled());
    await fireEvent.click(screen.getByRole('button', { name: '新建规则' }));
    await fireEvent.input(screen.getByLabelText('规则 ID'), { target: { value: 'custom' } });
    await fireEvent.input(screen.getByLabelText('名称'), { target: { value: 'Custom' } });
    await fireEvent.click(screen.getByRole('tab', { name: '来源' }));
    await fireEvent.click(screen.getByRole('combobox', { name: '来源 1 类型' }));
    await fireEvent.click(screen.getByRole('option', { name: '本地文件' }));
    await fireEvent.click(screen.getByRole('combobox', { name: '来源 1 文件' }));
    await fireEvent.click(screen.getByRole('option', { name: 'direct.list' }));
    await fireEvent.click(screen.getByRole('tab', { name: '处理' }));
    await fireEvent.click(within(screen.getByRole('region', { name: '全局过滤链' })).getByRole('button', { name: '添加过滤' }));
    await fireEvent.click(screen.getByLabelText('domain'));
    await fireEvent.click(screen.getByRole('tab', { name: '输出' }));
    await fireEvent.click(screen.getByLabelText('Mihomo'));
    await fireEvent.click(screen.getByLabelText('sing-box'));
    await fireEvent.click(screen.getByRole('button', { name: '保存规则' }));
    await screen.findByText('规则已保存');
    expect(save).toHaveBeenCalledWith(4, [{ op: 'add_rule', value: expect.objectContaining({
      id: 'custom',
      name: 'Custom',
      sources: [{ file: 'direct.list' }],
      ops: [expect.objectContaining({ type: 'include_kinds', kinds: ['domain'] })],
      outputs: ['mihomo', 'sing-box'],
    }) }]);
  });

  it('preserves preprocess and tags when updating a rule', async () => {
    const save = vi.spyOn(api, 'patchConfig').mockResolvedValue({ version: 5, warnings: [] });
    render(RulesView, { onStartUpdate: vi.fn() });
    await fireEvent.click(await screen.findByRole('button', { name: '编辑' }));
    await fireEvent.input(screen.getByLabelText('名称'), { target: { value: 'Base Updated' } });
    await fireEvent.click(screen.getByRole('button', { name: '保存规则' }));
    await screen.findByText('规则已保存');
    expect(save).toHaveBeenCalledWith(4, [{ op: 'update_rule', id: 'base', value: expect.objectContaining({
      name: 'Base Updated',
      tags: ['core'],
      preprocess: 'function process(content) { return content; }',
      sources: [{ url: 'https://example/base.list' }],
      outputs: ['mihomo'],
    }) }]);
  });

  it('requires confirmation before discarding rule edits', async () => {
    render(RulesView, { onStartUpdate: vi.fn() });
    await fireEvent.click(await screen.findByRole('button', { name: '编辑' }));
    await fireEvent.input(screen.getByLabelText('名称'), { target: { value: 'Changed' } });
    await fireEvent.click(screen.getByRole('button', { name: '关闭抽屉' }));
    expect(screen.getByRole('dialog', { name: '放弃未保存的修改？' })).toBeInTheDocument();
    await fireEvent.click(screen.getByRole('button', { name: '继续编辑' }));
    expect(screen.getByLabelText('名称')).toHaveValue('Changed');
  });

  it('batches output clients and blocks deleting a referenced rule', async () => {
    vi.mocked(api.getConfig).mockResolvedValue({
      version: 4,
      config: {
        clients,
        rules: [existing, { id: 'child', name: 'Child', sources: [{ ref: 'base' }], outputs: ['mihomo'] }],
      },
    });
    const save = vi.spyOn(api, 'patchConfig').mockResolvedValue({ version: 5, warnings: [] });
    render(RulesView, { onStartUpdate: vi.fn() });
    await screen.findByText('Child');
    await fireEvent.click(screen.getByRole('switch', { name: '选择' }));
    await fireEvent.click(screen.getByLabelText('选择 Child'));
    await fireEvent.click(screen.getByRole('combobox', { name: '批量输出客户端' }));
    await fireEvent.click(screen.getByRole('option', { name: 'sing-box' }));
    await fireEvent.click(screen.getByRole('button', { name: '添加输出' }));
    await waitFor(() => expect(save).toHaveBeenCalledWith(4, [{ op: 'batch_add_output', rule_ids: ['child'], output_ids: ['sing-box'] }]));
    await fireEvent.click(screen.getAllByRole('button', { name: '删除' })[0]);
    expect(screen.getByRole('status')).toHaveTextContent('Child');
    expect(save).toHaveBeenCalledTimes(1);
  });

  it('shows output client icons and hides the refs connector when a rule has no references', async () => {
    render(RulesView, { onStartUpdate: vi.fn() });
    await screen.findByText('Base');
    const outputs = screen.getByRole('button', { name: /输出客户端 1 个：Mihomo/ });
    expect(outputs.querySelectorAll('img')).toHaveLength(1);
    expect(outputs.querySelector('img')?.getAttribute('src')).toBe('/static/icons/mihomo.svg');
    expect(screen.queryByRole('button', { name: /引用关系/ })).toBeNull();
  });

  it('renders a single connector glyph per reference direction and lists formats in the outputs bubble', async () => {
    vi.mocked(api.getConfig).mockResolvedValue({
      version: 4,
      config: {
        clients,
        rules: [existing, { id: 'child', name: 'Child', sources: [{ ref: 'base' }], outputs: ['mihomo'] }],
      },
    });
    render(RulesView, { onStartUpdate: vi.fn() });
    await screen.findByText('Child');
    expect(screen.getByRole('button', { name: '引用关系，引用 1，被引用 0' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '引用关系，引用 0，被引用 1' })).toBeInTheDocument();
    const outputs = screen.getAllByRole('button', { name: /输出客户端 1 个：Mihomo/ })[0];
    await fireEvent.focus(outputs);
    expect(await screen.findByText('Classical · .list')).toBeInTheDocument();
  });

  it('marks the invalid field inline and clears it on input when saving fails validation', async () => {
    render(RulesView, { onStartUpdate: vi.fn() });
    await waitFor(() => expect(screen.getByRole('button', { name: '新建规则' })).toBeEnabled());
    await fireEvent.click(screen.getByRole('button', { name: '新建规则' }));
    await fireEvent.input(screen.getByLabelText('规则 ID'), { target: { value: 'custom' } });
    await fireEvent.click(screen.getByRole('button', { name: '保存规则' }));
    const nameInput = screen.getByPlaceholderText('显示名称');
    expect(nameInput).toHaveAttribute('aria-invalid', 'true');
    expect(screen.getAllByText('请输入规则名称').length).toBeGreaterThan(0);
    await fireEvent.input(nameInput, { target: { value: 'Custom' } });
    expect(nameInput).not.toHaveAttribute('aria-invalid');
  });

  it('shows client format labels next to the output checkboxes', async () => {
    render(RulesView, { onStartUpdate: vi.fn() });
    await fireEvent.click(await screen.findByRole('button', { name: '编辑' }));
    await fireEvent.click(screen.getByRole('tab', { name: '输出' }));
    await screen.findByText('Classical · .list');
    expect(screen.getByText('singbox')).toBeInTheDocument();
  });

  it('previews a draft without saving', async () => {
    const preview = vi.spyOn(api, 'previewRule').mockResolvedValue({
      rule_id: 'base',
      rule_name: 'Base',
      elapsed_ms: 12,
      sources: [{ label: 'https://example/base.list', type: 'url', entries: 2, diagnostics: 0, duration_ms: 8 }],
      pre_ops: 2,
      post_ops: 1,
      merged: 1,
      ops_diff: { added: 0, removed: 1, groups: [{ kind: 'ip_cidr', removed: ['ip_cidr,1.1.1.1/32'] }] },
      outputs: [
        { id: 'mihomo', client_id: 'mihomo', client_name: 'Mihomo', name: 'Standard', output: 'DOMAIN,example.com\n' },
        { id: 'sing-box', client_id: 'sing-box', client_name: 'sing-box', name: 'Standard', output: '{"version":3,"rules":[{"domain":["example.com"]}]}' },
      ],
    });
    render(RulesView, { onStartUpdate: vi.fn() });
    await fireEvent.click(await screen.findByRole('button', { name: '编辑' }));
    await fireEvent.click(screen.getByRole('button', { name: '预览当前修改' }));
    await screen.findByText(/合并 1 条/);
    expect(preview).toHaveBeenCalledWith(expect.objectContaining({ id: 'base', preprocess: existing.preprocess }));
    expect(screen.getByText('DOMAIN,example.com')).toBeInTheDocument();
    expect(screen.getByText('https://example/base.list')).toBeInTheDocument();
    await fireEvent.click(within(screen.getByRole('tablist', { name: '输出客户端' })).getByRole('tab', { name: 'sing-box' }));
    expect(within(screen.getByRole('tablist', { name: '客户端格式' })).getAllByRole('tab')).toHaveLength(1);
    expect(screen.getByText('{"version":3,"rules":[{"domain":["example.com"]}]}')).toBeInTheDocument();
    await fireEvent.click(screen.getByRole('button', { name: '返回编辑' }));
    await fireEvent.input(screen.getByLabelText('名称'), { target: { value: 'Changed draft' } });
    await fireEvent.click(screen.getByRole('button', { name: '预览当前修改' }));
    await waitFor(() => expect(preview).toHaveBeenLastCalledWith(expect.objectContaining({ name: 'Changed draft' })));
  });

  it('previews a saved rule from the row action', async () => {
    const previewMock = vi.spyOn(api, 'previewRule').mockResolvedValue({
      rule_id: 'base',
      rule_name: 'Base',
      elapsed_ms: 5,
      sources: [{ label: 'https://example/base.list', type: 'url', entries: 2, diagnostics: 0, duration_ms: 3 }],
      pre_ops: 2,
      post_ops: 2,
      merged: 2,
      ops_diff: { added: 0, removed: 0, groups: [] },
      outputs: [{ id: 'mihomo', client_id: 'mihomo', client_name: 'Mihomo', name: 'Standard', output: 'DOMAIN,example.com\n' }],
    });
    render(RulesView, { onStartUpdate: vi.fn() });
    await fireEvent.click(await screen.findByRole('button', { name: '预览' }));
    await screen.findByText(/合并 2 条/);
    expect(previewMock).toHaveBeenCalledWith(expect.objectContaining({ id: 'base' }));
    expect(screen.getByText('DOMAIN,example.com')).toBeInTheDocument();
  });

  it('explains a binary preview tab instead of disabling it', async () => {
    vi.spyOn(api, 'previewRule').mockResolvedValue({
      rule_id: 'base',
      rule_name: 'Base',
      elapsed_ms: 7,
      sources: [{ label: 'https://example/base.list', type: 'url', entries: 2, diagnostics: 0, duration_ms: 4 }],
      pre_ops: 2,
      post_ops: 2,
      merged: 2,
      ops_diff: { added: 0, removed: 0, groups: [] },
      outputs: [
        { id: 'mihomo', client_id: 'mihomo', client_name: 'Mihomo', name: 'Standard', output: 'DOMAIN,example.com\n' },
        { id: 'singbox-binary', client_id: 'sing-box', client_name: 'sing-box', name: 'Binary', binary: true },
      ],
    });
    render(RulesView, { onStartUpdate: vi.fn() });
    await fireEvent.click(await screen.findByRole('button', { name: '预览' }));
    await screen.findByText(/合并 2 条/);

    const clients_ = within(screen.getByRole('tablist', { name: '输出客户端' }));
    const formats = () => within(screen.getByRole('tablist', { name: '客户端格式' }));

    // 全为二进制的客户端也能选中，不会被禁用
    const binaryClient = clients_.getByRole('tab', { name: 'sing-box' });
    expect(binaryClient).toBeEnabled();
    await fireEvent.click(binaryClient);

    expect(formats().getByRole('tab', { name: 'Binary' })).toBeEnabled();
    expect(screen.getByText('二进制规则集无法以文本预览，可下载后导入客户端。')).toBeInTheDocument();
    expect(screen.queryByText('DOMAIN,example.com')).not.toBeInTheDocument();

    // 切回文本客户端后代码面板回来
    await fireEvent.click(clients_.getByRole('tab', { name: 'Mihomo' }));
    expect(screen.getByText('DOMAIN,example.com')).toBeInTheDocument();
  });
});

it('groups sources by dragging and saves shared processing inside the group', async () => {
  vi.mocked(api.getConfig).mockResolvedValue({ version: 4, config: { clients, rules: [{ ...existing, sources: [
    { url: 'https://example/a' }, { url: 'https://example/b' },
  ] }] } });
  const save = vi.spyOn(api, 'patchConfig').mockResolvedValue({ version: 5, warnings: [] });
  render(RulesView, { onStartUpdate: vi.fn() });
  await fireEvent.click(await screen.findByRole('button', { name: '编辑' }));
  await fireEvent.click(screen.getByRole('tab', { name: '来源' }));
  await fireEvent.dragStart(screen.getByRole('button', { name: '拖动来源 2' }), { dataTransfer: { setData: vi.fn() } });
  await fireEvent.drop(screen.getByRole('region', { name: '来源 1' }));
  const group = screen.getByRole('region', { name: '来源 1' });
  expect(within(group).getByText('来源组 1 · 2 个来源')).toBeInTheDocument();
  const scriptSection = within(group).getByText('预处理 · 继承统一').closest('details')!;
  expect(scriptSection.open).toBe(false);
  await fireEvent.click(within(group).getByText('预处理 · 继承统一'));
  const script = 'function process(content) { return content.trim(); }';
  typePreprocessScript(within(group).getByLabelText('JavaScript'), script);
  await fireEvent.click(within(group).getByText('组内过滤链 · 0 项'));
  await fireEvent.click(within(group).getByRole('button', { name: '添加过滤' }));
  await fireEvent.click(within(group).getByLabelText('domain'));
  await fireEvent.click(screen.getByRole('button', { name: '保存规则' }));
  await screen.findByText('规则已保存');
  expect(save).toHaveBeenCalledWith(4, [{ op: 'update_rule', id: 'base', value: expect.objectContaining({
    preprocess: existing.preprocess,
    sources: [{ group: [{ url: 'https://example/a' }, { url: 'https://example/b' }], preprocess: script, ops: [{ type: 'include_kinds', kinds: ['domain'] }] }],
  }) }]);
});

it('keeps preprocessing opt-out and filters when moving a member out of its group', async () => {
  const ops = [{ type: 'include_kinds', kinds: ['domain'] }];
  vi.mocked(api.getConfig).mockResolvedValue({ version: 4, config: { clients, rules: [{ ...existing, sources: [
    { group: [{ url: 'https://example/a' }, { url: 'https://example/b' }], preprocess: '', ops },
  ] }] } });
  const save = vi.spyOn(api, 'patchConfig').mockResolvedValue({ version: 5, warnings: [] });
  render(RulesView, { onStartUpdate: vi.fn() });
  await fireEvent.click(await screen.findByRole('button', { name: '编辑' }));
  await fireEvent.click(screen.getByRole('tab', { name: '来源' }));
  await fireEvent.click(screen.getAllByRole('button', { name: '移出组' })[1]);
  await fireEvent.click(screen.getByRole('button', { name: '保存规则' }));
  await screen.findByText('规则已保存');
  expect(save).toHaveBeenCalledWith(4, [{ op: 'update_rule', id: 'base', value: expect.objectContaining({
    sources: [{ group: [{ url: 'https://example/a' }], preprocess: '', ops }, { group: [{ url: 'https://example/b' }], preprocess: '', ops }],
  }) }]);
});

it('edits shared preprocessing while preserving flat sources', async () => {
  const sources = [{ url: 'https://example/a' }, { url: 'https://example/b' }];
  vi.mocked(api.getConfig).mockResolvedValue({ version: 4, config: { clients, rules: [{ ...existing, sources }] } });
  const save = vi.spyOn(api, 'patchConfig').mockResolvedValue({ version: 5, warnings: [] });
  render(RulesView, { onStartUpdate: vi.fn() });
  await fireEvent.click(await screen.findByRole('button', { name: '编辑' }));
  await fireEvent.click(screen.getByRole('tab', { name: '来源' }));
  expect(within(screen.getByRole('region', { name: '统一预处理' })).getByText(/继承 2 · 覆盖 0 · 禁用 0/)).toBeInTheDocument();
  const script = 'function process(content) { return content.trim(); }';
  typePreprocessScript(within(screen.getByRole('region', { name: '统一预处理' })).getByLabelText('JavaScript'), script);
  await fireEvent.click(screen.getByRole('button', { name: '保存规则' }));
  await screen.findByText('规则已保存');
  expect(save).toHaveBeenCalledWith(4, [{ op: 'update_rule', id: 'base', value: expect.objectContaining({ sources, preprocess: script }) }]);
});

it('keeps inheritance when grouping a source that relies on the unified preprocess', async () => {
  const save = vi.spyOn(api, 'patchConfig').mockResolvedValue({ version: 5, warnings: [] });
  render(RulesView, { onStartUpdate: vi.fn() });
  await fireEvent.click(await screen.findByRole('button', { name: '编辑' }));
  await fireEvent.click(screen.getByRole('tab', { name: '来源' }));
  await fireEvent.click(screen.getByRole('button', { name: '转为来源组' }));
  expect(screen.getByText('预处理 · 继承统一')).toBeInTheDocument();
  await fireEvent.click(screen.getByRole('button', { name: '保存规则' }));
  await screen.findByText('规则已保存');
  const payload = save.mock.calls[0][1][0] as { value: Record<string, unknown> };
  const sources = payload.value.sources as Record<string, unknown>[];
  expect(sources[0].group).toEqual([{ url: 'https://example/base.list' }]);
  expect(sources[0]).not.toHaveProperty('preprocess');
  expect(payload.value.preprocess).toBe(existing.preprocess);
});

it('configures per-source preprocess and filters without converting to a group', async () => {
  const save = vi.spyOn(api, 'patchConfig').mockResolvedValue({ version: 5, warnings: [] });
  render(RulesView, { onStartUpdate: vi.fn() });
  await fireEvent.click(await screen.findByRole('button', { name: '编辑' }));
  await fireEvent.click(screen.getByRole('tab', { name: '来源' }));
  const region = screen.getByRole('region', { name: '来源 1' });
  await fireEvent.click(within(region).getByText('预处理 · 继承统一'));
  const script = 'function process(content) { return content.toUpperCase(); }';
  typePreprocessScript(within(region).getByLabelText('JavaScript'), script);
  await fireEvent.click(within(region).getByText('过滤链 · 0 项'));
  await fireEvent.click(within(region).getByRole('button', { name: '添加过滤' }));
  await fireEvent.click(within(region).getByLabelText('domain'));
  await fireEvent.click(screen.getByRole('button', { name: '保存规则' }));
  await screen.findByText('规则已保存');
  expect(save).toHaveBeenCalledWith(4, [{ op: 'update_rule', id: 'base', value: expect.objectContaining({
    preprocess: existing.preprocess,
    sources: [{ url: 'https://example/base.list', preprocess: script, ops: [{ type: 'include_kinds', kinds: ['domain'] }] }],
  }) }]);
});

it('converts a newly added source to a one-source group with independent processing', async () => {
  const save = vi.spyOn(api, 'patchConfig').mockResolvedValue({ version: 5, warnings: [] });
  render(RulesView, { onStartUpdate: vi.fn() });
  await fireEvent.click(await screen.findByRole('button', { name: '编辑' }));
  await fireEvent.click(screen.getByRole('tab', { name: '来源' }));
  await fireEvent.click(screen.getByRole('button', { name: '添加来源' }));
  const plain = screen.getByRole('region', { name: '来源 2' });
  expect(within(plain).queryByText('添加组内来源')).toBeNull();
  await fireEvent.input(within(plain).getByLabelText('URL'), { target: { value: 'https://example/new' } });
  await fireEvent.click(within(plain).getByRole('button', { name: '转为来源组' }));
  const group = screen.getByRole('region', { name: '来源 2' });
  expect(within(group).getByText('来源组 2 · 1 个来源')).toBeInTheDocument();
  await fireEvent.click(within(group).getByText('预处理 · 继承统一'));
  const script = 'function process(content) { return content.trim(); }';
  typePreprocessScript(within(group).getByLabelText('JavaScript'), script);
  await fireEvent.click(within(group).getByText('组内过滤链 · 0 项'));
  await fireEvent.click(within(group).getByRole('button', { name: '添加过滤' }));
  await fireEvent.click(within(group).getByLabelText('domain'));
  await fireEvent.click(screen.getByRole('button', { name: '保存规则' }));
  await screen.findByText('规则已保存');
  expect(save).toHaveBeenCalledWith(4, [{ op: 'update_rule', id: 'base', value: expect.objectContaining({
    preprocess: existing.preprocess,
    sources: [{ url: 'https://example/base.list' }, {
      group: [{ url: 'https://example/new' }], preprocess: script,
      ops: [{ type: 'include_kinds', kinds: ['domain'] }],
    }],
  }) }]);
});
