import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { beforeAll, beforeEach, afterEach, expect, it, vi } from 'vitest';
import { EditorView } from '@codemirror/view';
import { api } from '../api/client';
import TemplatesView from './TemplatesView.svelte';

const item = { id: 'surge', name: 'Surge', builtin: true, codec: 'linelist', extension: '.list' };
const yaml = 'id: surge\nname: Surge\ncodec: linelist\nextension: .list\nkind_map:\n  domain: DOMAIN\n';
beforeAll(() => {
  HTMLDialogElement.prototype.showModal = function () { this.setAttribute('open', ''); };
  HTMLDialogElement.prototype.close = function () { this.removeAttribute('open'); };
});
beforeEach(() => {
  vi.spyOn(api, 'listTemplates').mockResolvedValue({ items: [item] });
  vi.spyOn(api, 'getTemplate').mockResolvedValue({ ...item, yaml, version: 'first' });
});
afterEach(() => vi.restoreAllMocks());

it('shows builtins read-only and copies them into an editable template', async () => {
  const save = vi.spyOn(api, 'saveTemplate').mockResolvedValue({ ...item, builtin: false, id: 'custom-client', yaml: yaml.replace('id: surge', 'id: custom-client'), version: 'saved' });
  render(TemplatesView);
  await fireEvent.click(await screen.findByRole('button', { name: '查看模板' }));
  const box = await screen.findByRole('textbox', { name: '模板 YAML' });
  expect(box).toHaveAttribute('contenteditable', 'false');
  expect(screen.queryByRole('button', { name: '保存模板' })).not.toBeInTheDocument();
  await fireEvent.click(screen.getByRole('button', { name: '关闭' }));
  await fireEvent.click(screen.getByRole('button', { name: '复制' }));
  const editableBox = await screen.findByRole('textbox', { name: '模板 YAML' });
  expect(editableBox).toHaveAttribute('contenteditable', 'true');
  await fireEvent.click(screen.getByRole('button', { name: '保存模板' }));
  await waitFor(() => expect(save).toHaveBeenCalledWith(yaml.replace('id: surge', 'id: custom-client'), undefined, undefined));
});

it('ignores a preview response after the template changes', async () => {
  let resolve!: (v: Awaited<ReturnType<typeof api.validateTemplate>>) => void;
  vi.spyOn(api, 'validateTemplate').mockReturnValue(new Promise(r => { resolve = r; }));
  render(TemplatesView);
  await waitFor(() => expect(screen.getByRole('button', { name: '新建模板' })).toBeEnabled());
  await fireEvent.click(screen.getByRole('button', { name: '新建模板' }));
  await fireEvent.click(screen.getByRole('button', { name: '校验' }));
  const box = await screen.findByRole('textbox', { name: '模板 YAML' });
  EditorView.findFromDOM(box)!.dispatch({ changes: { from: 0, insert: '# edit\n' } });
  resolve({ valid: true, errors: [], output: 'OUTDATED', extension: '.list', sample: [] });
  await waitFor(() => expect(screen.getByRole('button', { name: '校验' })).toBeEnabled());
  expect(screen.queryByText('模板语法校验通过')).not.toBeInTheDocument();
  expect(screen.queryByText('OUTDATED')).not.toBeInTheDocument();
});

it('supports testing template with PixelSelect and PixelCheckbox', async () => {
  vi.spyOn(api, 'validateTemplate').mockResolvedValue({
    valid: true,
    errors: [],
    output: 'DOMAIN,example.com\nIP-CIDR,1.1.1.1/32,no-resolve\n',
    extension: '.list',
    sample: [],
  });
  render(TemplatesView);
  await fireEvent.click(await screen.findByRole('button', { name: '查看模板' }));
  await fireEvent.click(screen.getByRole('tab', { name: '测试' }));
  expect(await screen.findByText(/测试 IR 规则/)).toBeInTheDocument();

  const comboboxes = screen.getAllByRole('combobox');
  expect(comboboxes.length).toBeGreaterThan(0);

  const checkboxes = screen.getAllByRole('checkbox');
  expect(checkboxes.length).toBeGreaterThan(0);
});
