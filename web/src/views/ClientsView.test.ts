import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { beforeAll, beforeEach, afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '../api/client';
import ClientsView from './ClientsView.svelte';

beforeAll(() => {
  HTMLDialogElement.prototype.showModal = function () { this.setAttribute('open', ''); };
  HTMLDialogElement.prototype.close = function () { this.removeAttribute('open'); };
});
beforeEach(() => {
  vi.spyOn(api, 'listTemplates').mockResolvedValue({ items: [{ id: 'surge', name: 'Surge', builtin: true, codec: 'linelist', extension: '.list' }] });
  vi.spyOn(api, 'getConfig').mockResolvedValue({ version: 4, config: { clients: [{ id: 'surge', name: 'Surge', template: 'surge', icon: '/static/icons/surge.svg' }], rules: [] } });
});
afterEach(() => vi.restoreAllMocks());

describe('ClientsView', () => {
  it('saves a non-IP variant and preserves client properties', async () => {
    const save = vi.spyOn(api, 'patchConfig').mockResolvedValue({ version: 5, warnings: [] });
    render(ClientsView);
    await fireEvent.click(await screen.findByRole('button', { name: '编辑' }));
    await fireEvent.click(screen.getByRole('button', { name: '添加变体' }));
    await fireEvent.input(screen.getByLabelText('变体 ID'), { target: { value: 'surge-non-ip' } });
    await fireEvent.click(screen.getByRole('button', { name: '添加过滤' }));
    await fireEvent.click(screen.getByLabelText('domain'));
    await fireEvent.click(screen.getByRole('button', { name: '保存客户端' }));
    await screen.findByText('客户端已保存');
    expect(save).toHaveBeenCalledWith(4, [{ op: 'update_client', id: 'surge', value: expect.objectContaining({
      icon: '/static/icons/surge.svg', template: 'surge', variants: [expect.objectContaining({ id: 'surge-non-ip', ops: [expect.objectContaining({ type: 'include_kinds', kinds: ['domain'] })] })],
    }) }]);
    await fireEvent.click(screen.getByRole('button', { name: '关闭' }));
    await fireEvent.click(screen.getByRole('button', { name: '编辑' }));
    expect(screen.getByLabelText('变体 ID')).toHaveValue('surge-non-ip');
  });

  it('requires confirmation before discarding client edits', async () => {
    render(ClientsView);
    await fireEvent.click(await screen.findByRole('button', { name: '编辑' }));
    await fireEvent.input(screen.getByLabelText('名称'), { target: { value: 'Changed' } });
    await fireEvent.click(screen.getByRole('button', { name: '关闭抽屉' }));
    expect(screen.getByRole('dialog', { name: '放弃未保存的修改？' })).toBeInTheDocument();
    await fireEvent.click(screen.getByRole('button', { name: '继续编辑' }));
    expect(screen.getByLabelText('名称')).toHaveValue('Changed');
    await fireEvent.click(screen.getByRole('button', { name: '关闭抽屉' }));
    await fireEvent.click(screen.getByRole('button', { name: '放弃修改' }));
    await waitFor(() => expect(screen.queryByRole('dialog', { name: '编辑客户端' })).not.toBeInTheDocument());
  });

  it('blocks deletion of a client referenced by geosite', async () => {
    vi.mocked(api.getConfig).mockResolvedValue({ version: 4, config: { clients: [{ id: 'surge', name: 'Surge', template: 'surge' }], geosite: { providers: [{ name: 'v2fly', clients: ['surge'] }] } } });
    const save = vi.spyOn(api, 'patchConfig');
    render(ClientsView);
    await fireEvent.click(await screen.findByRole('button', { name: '删除' }));
    expect(screen.getByRole('status')).toHaveTextContent('Geosite v2fly');
    expect(save).not.toHaveBeenCalled();
  });

  it('picks a root icon by short name', async () => {
    vi.spyOn(api, 'listIcons').mockResolvedValue({ items: [{ id: 'mihomo', file: 'mihomo.svg' }, { id: 'surge', file: 'surge.svg' }] });
    const save = vi.spyOn(api, 'patchConfig').mockResolvedValue({ version: 5, warnings: [] });
    render(ClientsView);
    await fireEvent.click(await screen.findByRole('button', { name: '编辑' }));
    await fireEvent.click(screen.getByRole('button', { name: '选择' }));
    await fireEvent.click(await screen.findByRole('radio', { name: 'mihomo' }));
    await fireEvent.click(screen.getByRole('button', { name: '使用' }));
    await fireEvent.click(screen.getByRole('button', { name: '保存客户端' }));
    await screen.findByText('客户端已保存');
    expect(save).toHaveBeenCalledWith(4, [{ op: 'update_client', id: 'surge', value: expect.objectContaining({ icon: 'mihomo' }) }]);
  });

  it('creates a client using a template from the dropdown', async () => {
    const save = vi.spyOn(api, 'patchConfig').mockResolvedValue({ version: 5, warnings: [] });
    render(ClientsView);
    await waitFor(() => expect(screen.getByRole('button', { name: '新建客户端' })).toBeEnabled());
    await fireEvent.click(screen.getByRole('button', { name: '新建客户端' }));
    await fireEvent.input(screen.getByLabelText('客户端 ID'), { target: { value: 'custom' } });
    await fireEvent.click(screen.getByRole('combobox', { name: '客户端模板' }));
    await fireEvent.click(screen.getByRole('option', { name: 'Surge · surge' }));
    await fireEvent.click(screen.getByRole('button', { name: '保存客户端' }));
    await waitFor(() => expect(save).toHaveBeenCalledWith(4, [{ op: 'add_client', value: expect.objectContaining({ id: 'custom', template: 'surge' }) }]));
  });
});
