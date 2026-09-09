import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '../api/client';
import GeositeView from './GeositeView.svelte';

const provider = {
  name: 'v2fly',
  version: 'test-v1',
  lists: 3,
  variants: 2,
  entries: 12,
  files: 4,
  result: 'updated',
  checked_at: '2026-09-08T12:00:00.000Z',
  clients: ['surge'],
};

beforeAll(() => {
  HTMLDialogElement.prototype.showModal = function showModal() { this.setAttribute('open', ''); };
  HTMLDialogElement.prototype.close = function close() { this.removeAttribute('open'); };
});
beforeEach(() => {
  vi.spyOn(api, 'getConfig').mockResolvedValue({
    version: 4,
    config: { clients: [{ id: 'surge', name: 'Surge' }], geosite: { providers: [{ name: 'v2fly', clients: ['surge'] }] } },
  });
  vi.spyOn(api, 'getGeositeProviders').mockResolvedValue({
    items: [provider],
    supported: ['v2fly', 'loyalsoldier'],
  });
});
afterEach(() => vi.restoreAllMocks());

describe('GeositeView', () => {
  it('adds a provider and saves output clients', async () => {
    const save = vi.spyOn(api, 'patchConfig').mockResolvedValue({ version: 5, warnings: [] });
    render(GeositeView);
    await waitFor(() => expect(screen.getByRole('button', { name: '添加提供商' })).toBeEnabled());
    await fireEvent.click(screen.getByRole('button', { name: '添加提供商' }));
    await fireEvent.click(screen.getByRole('combobox', { name: '提供商' }));
    await fireEvent.click(screen.getByRole('option', { name: 'Loyalsoldier' }));
    await fireEvent.click(screen.getByRole('checkbox', { name: 'Surge' }));
    await fireEvent.click(screen.getByRole('button', { name: '保存' }));
    await screen.findByText('提供商已保存');
    expect(save).toHaveBeenCalledWith(4, [{
      op: 'update_geosite',
      value: { providers: [{ name: 'v2fly', clients: ['surge'] }, { name: 'loyalsoldier', clients: ['surge'] }] },
    }]);
  });

  it('filters catalog by name and searches list contents', async () => {
    vi.spyOn(api, 'getGeositeCatalog').mockImplementation(async (_provider, query = '', match = 'name') => {
      if (match === 'content' && query === 'youtube.com') {
        return { provider: 'v2fly', lists: [{ name: 'google', entries: 3, variants: [{ attr: 'cn', entries: 1 }] }], total: 1 };
      }
      return {
        provider: 'v2fly',
        lists: [
          { name: 'ads', entries: 1, variants: [] },
          { name: 'google', entries: 3, variants: [{ attr: 'cn', entries: 1 }] },
        ],
        total: 2,
      };
    });
    vi.spyOn(api, 'getGeositeList').mockResolvedValue({
      provider: 'v2fly', list: 'google', total: 1, offset: 0, limit: 100,
      items: [{ type: 'full', value: 'youtube.com', attrs: [] }],
    });
    render(GeositeView);
    await fireEvent.click(await screen.findByRole('button', { name: '目录' }));
    await screen.findByText('google');
    await fireEvent.input(screen.getByPlaceholderText('搜索列表或变体…'), { target: { value: 'goo' } });
    expect(screen.getByText('google')).toBeInTheDocument();
    expect(screen.queryByText('ads')).not.toBeInTheDocument();
    await fireEvent.click(screen.getByRole('tab', { name: '内容' }));
    await fireEvent.input(screen.getByPlaceholderText('域名或规则内容…'), { target: { value: 'youtube.com' } });
    await waitFor(() => expect(api.getGeositeCatalog).toHaveBeenCalledWith('v2fly', 'youtube.com', 'content'));
    await waitFor(() => expect(screen.queryByText('ads')).not.toBeInTheDocument());
    expect(screen.getByText('google')).toBeInTheDocument();
    await fireEvent.click(screen.getByRole('button', { name: /^google/ }));
    await screen.findByText('youtube.com');
    expect(screen.getByText('1/1')).toBeInTheDocument();
  });

  it('deletes a provider after confirmation', async () => {
    const save = vi.spyOn(api, 'patchConfig').mockResolvedValue({ version: 5, warnings: [] });
    render(GeositeView);
    await fireEvent.click(await screen.findByRole('button', { name: '删除' }));
    await fireEvent.click(screen.getByRole('button', { name: '确认删除' }));
    await screen.findByText('提供商已删除');
    expect(save).toHaveBeenCalledWith(4, [{ op: 'update_geosite', value: null }]);
  });
});
