import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';
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

  it('adds and removes tag chips and saves them in the rule', async () => {
    const user = userEvent.setup();
    const save = vi.spyOn(api, 'patchConfig').mockResolvedValue({ version: 5, warnings: [] });
    render(RulesView, { onStartUpdate: vi.fn() });
    await user.click(await screen.findByRole('button', { name: '编辑' }));

    // Verify existing tag chip
    expect(screen.getByText('core')).toBeInTheDocument();

    // Type a new tag chip
    const tagInput = screen.getByRole('textbox', { name: '标签输入' });
    await user.type(tagInput, 'proxy{enter}');
    expect(screen.getByText('proxy')).toBeInTheDocument();

    // Remove the 'core' tag chip
    const removeCore = screen.getByRole('button', { name: '删除标签 core' });
    await user.click(removeCore);
    expect(screen.queryByText('core')).not.toBeInTheDocument();

    // Save rule
    await user.click(screen.getByRole('button', { name: '保存规则' }));
    await screen.findByText('规则已保存');

    expect(save).toHaveBeenCalledWith(4, [{ op: 'update_rule', id: 'base', value: expect.objectContaining({
      tags: ['proxy'],
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

async function openSources() {
  await fireEvent.click(await screen.findByRole('button', { name: '编辑' }));
  await fireEvent.click(screen.getByRole('tab', { name: '来源' }));
}

async function chooseMenu(menu: string, item: string) {
  await fireEvent.click(screen.getByRole('button', { name: menu }));
  await fireEvent.click(screen.getByRole('menuitem', { name: item }));
}

it('merges one source into another and saves shared processing on the row', async () => {
  vi.mocked(api.getConfig).mockResolvedValue({ version: 4, config: { clients, rules: [{ ...existing, sources: [
    { url: 'https://example/a' }, { url: 'https://example/b' },
  ] }] } });
  const save = vi.spyOn(api, 'patchConfig').mockResolvedValue({ version: 5, warnings: [] });
  render(RulesView, { onStartUpdate: vi.fn() });
  await openSources();
  expect(screen.getByText('∪ 并集')).toBeInTheDocument();
  await chooseMenu('来源 2 操作', '并入 来源 1');
  const row = screen.getByRole('region', { name: '来源 1' });
  expect(within(row).getByText('2 个输入')).toBeInTheDocument();
  await fireEvent.click(within(row).getByRole('radio', { name: '自定义' }));
  const script = 'function process(content) { return content.trim(); }';
  typePreprocessScript(within(row).getByLabelText('来源 1 预处理脚本'), script);
  await fireEvent.click(within(row).getByText('过滤 · 0 项'));
  await fireEvent.click(within(row).getByRole('button', { name: '添加过滤' }));
  await fireEvent.click(within(row).getByLabelText('domain'));
  await fireEvent.click(screen.getByRole('button', { name: '保存规则' }));
  await screen.findByText('规则已保存');
  expect(save).toHaveBeenCalledWith(4, [{ op: 'update_rule', id: 'base', value: expect.objectContaining({
    preprocess: existing.preprocess,
    sources: [{ group: [{ url: 'https://example/a' }, { url: 'https://example/b' }], preprocess: script, ops: [{ type: 'include_kinds', kinds: ['domain'] }] }],
  }) }]);
});

it('splits an input out of a multi-input source and keeps its processing', async () => {
  const ops = [{ type: 'include_kinds', kinds: ['domain'] }];
  vi.mocked(api.getConfig).mockResolvedValue({ version: 4, config: { clients, rules: [{ ...existing, sources: [
    { group: [{ url: 'https://example/a' }, { url: 'https://example/b' }], preprocess: '', ops },
  ] }] } });
  const save = vi.spyOn(api, 'patchConfig').mockResolvedValue({ version: 5, warnings: [] });
  render(RulesView, { onStartUpdate: vi.fn() });
  await openSources();
  await chooseMenu('来源 1 输入 2 操作', '拆为独立来源');
  expect(screen.getByRole('region', { name: '来源 2' })).toBeInTheDocument();
  await fireEvent.click(screen.getByRole('button', { name: '保存规则' }));
  await screen.findByText('规则已保存');
  expect(save).toHaveBeenCalledWith(4, [{ op: 'update_rule', id: 'base', value: expect.objectContaining({
    sources: [{ group: [{ url: 'https://example/a' }], preprocess: '', ops }, { url: 'https://example/b', preprocess: '', ops }],
  }) }]);
});

it('edits the default preprocess on the processing tab while preserving flat sources', async () => {
  const sources = [{ url: 'https://example/a' }, { url: 'https://example/b' }];
  vi.mocked(api.getConfig).mockResolvedValue({ version: 4, config: { clients, rules: [{ ...existing, sources }] } });
  const save = vi.spyOn(api, 'patchConfig').mockResolvedValue({ version: 5, warnings: [] });
  render(RulesView, { onStartUpdate: vi.fn() });
  await openSources();
  expect(screen.getByText(/默认预处理已配置 · 2 个来源使用/)).toBeInTheDocument();
  await fireEvent.click(screen.getByRole('button', { name: '编辑默认脚本' }));
  const region = screen.getByRole('region', { name: '默认预处理' });
  expect(within(region).getByText(/使用默认 2 · 自定义 0 · 关闭 0/)).toBeInTheDocument();
  const script = 'function process(content) { return content.trim(); }';
  typePreprocessScript(within(region).getByLabelText('默认预处理脚本'), script);
  await fireEvent.click(screen.getByRole('button', { name: '保存规则' }));
  await screen.findByText('规则已保存');
  expect(save).toHaveBeenCalledWith(4, [{ op: 'update_rule', id: 'base', value: expect.objectContaining({ sources, preprocess: script }) }]);
});

it('keeps inheritance when adding a second input to a source', async () => {
  const save = vi.spyOn(api, 'patchConfig').mockResolvedValue({ version: 5, warnings: [] });
  render(RulesView, { onStartUpdate: vi.fn() });
  await openSources();
  await fireEvent.click(within(screen.getByRole('region', { name: '来源 1' })).getByRole('button', { name: /添加输入/ }));
  const row = screen.getByRole('region', { name: '来源 1' });
  await fireEvent.input(within(row).getByLabelText('来源 1 输入 2 URL'), { target: { value: 'https://example/new' } });
  expect(within(row).getByRole('radio', { name: '使用默认' })).toHaveAttribute('aria-checked', 'true');
  await fireEvent.click(screen.getByRole('button', { name: '保存规则' }));
  await screen.findByText('规则已保存');
  const payload = save.mock.calls[0][1][0] as { value: Record<string, unknown> };
  const sources = payload.value.sources as Record<string, unknown>[];
  expect(sources[0].group).toEqual([{ url: 'https://example/base.list' }, { url: 'https://example/new' }]);
  expect(sources[0]).not.toHaveProperty('preprocess');
  expect(payload.value.preprocess).toBe(existing.preprocess);
});

it('configures per-source preprocess and filters without converting to a group', async () => {
  const save = vi.spyOn(api, 'patchConfig').mockResolvedValue({ version: 5, warnings: [] });
  render(RulesView, { onStartUpdate: vi.fn() });
  await openSources();
  const region = screen.getByRole('region', { name: '来源 1' });
  await fireEvent.click(within(region).getByRole('radio', { name: '自定义' }));
  const script = 'function process(content) { return content.toUpperCase(); }';
  typePreprocessScript(within(region).getByLabelText('来源 1 预处理脚本'), script);
  await fireEvent.click(within(region).getByText('过滤 · 0 项'));
  await fireEvent.click(within(region).getByRole('button', { name: '添加过滤' }));
  await fireEvent.click(within(region).getByLabelText('domain'));
  await fireEvent.click(screen.getByRole('button', { name: '保存规则' }));
  await screen.findByText('规则已保存');
  expect(save).toHaveBeenCalledWith(4, [{ op: 'update_rule', id: 'base', value: expect.objectContaining({
    preprocess: existing.preprocess,
    sources: [{ url: 'https://example/base.list', preprocess: script, ops: [{ type: 'include_kinds', kinds: ['domain'] }] }],
  }) }]);
});

it('adds a typed source from the menu and keeps a one-input group unless dissolved', async () => {
  vi.mocked(api.getConfig).mockResolvedValue({ version: 4, config: { clients, rules: [{ ...existing, sources: [
    { group: [{ url: 'https://example/a', label: 'a' }], preprocess: '' },
  ] }] } });
  const save = vi.spyOn(api, 'patchConfig').mockResolvedValue({ version: 5, warnings: [] });
  render(RulesView, { onStartUpdate: vi.fn() });
  await openSources();
  await chooseMenu('添加来源', '内联文本');
  const added = screen.getByRole('region', { name: '来源 2' });
  expect(within(added).getByRole('button', { name: /^来源 2：/ })).toHaveAttribute('aria-expanded', 'true');
  typePreprocessScript(within(added).getByLabelText('来源 2 内联文本'), 'DOMAIN,new.example');
  await fireEvent.click(screen.getByRole('button', { name: '保存规则' }));
  await screen.findByText('规则已保存');
  expect(save).toHaveBeenLastCalledWith(4, [{ op: 'update_rule', id: 'base', value: expect.objectContaining({
    sources: [{ group: [{ url: 'https://example/a', label: 'a' }], preprocess: '' }, { content: 'DOMAIN,new.example' }],
  }) }]);

  await chooseMenu('来源 1 操作', '解散为普通来源');
  await fireEvent.click(screen.getByRole('button', { name: '保存规则' }));
  await waitFor(() => expect(save).toHaveBeenLastCalledWith(5, [{ op: 'update_rule', id: 'base', value: expect.objectContaining({
    sources: [{ url: 'https://example/a', label: 'a', preprocess: '' }, { content: 'DOMAIN,new.example' }],
  }) }]));
});

it('confirms before a move drops the processing of a removed source', async () => {
  vi.mocked(api.getConfig).mockResolvedValue({ version: 4, config: { clients, rules: [{ ...existing, sources: [
    { url: 'https://example/a' }, { url: 'https://example/b', preprocess: '' },
  ] }] } });
  render(RulesView, { onStartUpdate: vi.fn() });
  await openSources();
  await chooseMenu('来源 2 操作', '并入 来源 1');
  expect(screen.getByRole('dialog', { name: '移入后使用目标来源的处理设置' })).toBeInTheDocument();
  await fireEvent.click(screen.getByRole('button', { name: '取消' }));
  expect(screen.getByRole('region', { name: '来源 2' })).toBeInTheDocument();
  await chooseMenu('来源 2 操作', '并入 来源 1');
  await fireEvent.click(screen.getByRole('button', { name: '移动' }));
  expect(screen.queryByRole('region', { name: '来源 2' })).toBeNull();
  expect(within(screen.getByRole('region', { name: '来源 1' })).getByText('2 个输入')).toBeInTheDocument();
});

it('reorders rows by dragging and shows the difference base', async () => {
  vi.mocked(api.getConfig).mockResolvedValue({ version: 4, config: { clients, rules: [{ ...existing, merge: { strategy: 'difference' }, sources: [
    { url: 'https://example/a' }, { url: 'https://example/b' },
  ] }] } });
  const save = vi.spyOn(api, 'patchConfig').mockResolvedValue({ version: 5, warnings: [] });
  render(RulesView, { onStartUpdate: vi.fn() });
  await openSources();
  expect(screen.getByText('基准')).toBeInTheDocument();
  expect(screen.getByText('− 减去')).toBeInTheDocument();
  await fireEvent.dragStart(screen.getByRole('button', { name: '拖动来源 2' }), { dataTransfer: { setData: vi.fn() } });
  await fireEvent.drop(screen.getByRole('region', { name: '来源 1' }));
  await fireEvent.click(screen.getByRole('button', { name: '保存规则' }));
  await screen.findByText('规则已保存');
  expect(save).toHaveBeenCalledWith(4, [{ op: 'update_rule', id: 'base', value: expect.objectContaining({
    merge: { strategy: 'difference' },
    sources: [{ url: 'https://example/b' }, { url: 'https://example/a' }],
  }) }]);
});

it('runs a single source through preview without outputs', async () => {
  const preview = vi.spyOn(api, 'previewRule').mockResolvedValue({
    rule_id: 'base', rule_name: 'Base', elapsed_ms: 3,
    sources: [{ label: '来源 1', type: 'url', entries: 1, diagnostics: 1, duration_ms: 2,
      diagnostic_items: [{ line: 3, text: 'NOPE,x', reason: 'unknown rule type "NOPE"' }] }],
    pre_ops: 1, post_ops: 1, merged: 1, ops_diff: { added: 0, removed: 0, groups: [] }, outputs: [],
  });
  render(RulesView, { onStartUpdate: vi.fn() });
  await openSources();
  await fireEvent.click(screen.getByRole('button', { name: '试运行' }));
  const result = await screen.findByRole('status', { name: '来源 1 试运行结果' });
  await waitFor(() => expect(result).toHaveTextContent('⚠ 1 行无法解析'));
  expect(result).toHaveTextContent('第 3 行：未知规则类型 NOPE');
  expect(preview).toHaveBeenCalledWith({
    id: 'base', name: 'Base', preprocess: existing.preprocess,
    sources: [{ url: 'https://example/base.list' }], outputs: [],
  });
});

it('expands the failing source and shows the error next to its input', async () => {
  vi.mocked(api.getConfig).mockResolvedValue({ version: 4, config: { clients, rules: [{ ...existing, sources: [
    { url: 'https://example/a' }, { group: [{ url: 'https://example/b' }, { file: 'missing.list' }] },
  ] }] } });
  render(RulesView, { onStartUpdate: vi.fn() });
  await fireEvent.click(await screen.findByRole('button', { name: '编辑' }));
  await fireEvent.click(screen.getByRole('button', { name: '保存规则' }));
  const alert = screen.getByRole('alert', { name: '表单错误' });
  expect(alert).toHaveTextContent('来源 2 的输入 2 引用了不存在的本地文件');
  const row = screen.getByRole('region', { name: '来源 2' });
  expect(within(row).getByRole('button', { name: /^来源 2：/ })).toHaveAttribute('aria-expanded', 'true');
  expect(within(row).getAllByText('来源 2 的输入 2 引用了不存在的本地文件').length).toBeGreaterThan(0);
});
