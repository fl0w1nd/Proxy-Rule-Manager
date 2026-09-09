import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { api, APIRequestError } from '../api/client';
import BackupView from './BackupView.svelte';

const item = { id: 'config-20260908-121800.yaml', created_at: '2026-09-08T12:18:00.000Z', size: 2048 };
const backups = { version: 4, items: [item] };
const detail = {
  ...item,
  yaml: 'name: Base\n',
};

beforeAll(() => {
  HTMLDialogElement.prototype.showModal = function showModal() { this.setAttribute('open', ''); };
  HTMLDialogElement.prototype.close = function close() { this.removeAttribute('open'); };
});
afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe('BackupView', () => {
  it('shows the snapshot source and restores it', async () => {
    vi.spyOn(api, 'listConfigBackups').mockResolvedValue(backups);
    vi.spyOn(api, 'getConfigBackup').mockResolvedValue(detail);
    vi.spyOn(api, 'getConfigRaw').mockResolvedValue({ yaml: 'clients: []\n', path: '/data/config.yaml', version: 4 });
    const restore = vi.spyOn(api, 'restoreConfigBackup').mockResolvedValue({ version: 5, warnings: [] });
    URL.createObjectURL = vi.fn(() => 'blob:backup');
    URL.revokeObjectURL = vi.fn();
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
    render(BackupView);
    await screen.findByRole('button', { name: '查看配置' });
    await fireEvent.click(screen.getByRole('button', { name: '下载当前配置' }));
    await waitFor(() => expect(URL.createObjectURL).toHaveBeenCalled());
    expect(click).toHaveBeenCalled();
    await fireEvent.click(screen.getByRole('button', { name: '查看配置' }));
    await screen.findByRole('region', { name: '快照配置' });
    expect(screen.getByRole('region', { name: '快照配置' })).toHaveTextContent('name: Base');
    await fireEvent.click(screen.getByRole('button', { name: '回滚' }));
    await fireEvent.click(screen.getByRole('button', { name: '确认回滚' }));
    await screen.findByText('已回滚到所选快照');
    expect(restore).toHaveBeenCalledWith('config-20260908-121800.yaml', 4);
  });

  it('loads an imported file into the editor callback', async () => {
    vi.spyOn(api, 'listConfigBackups').mockResolvedValue({ version: 1, items: [] });
    const onimport = vi.fn();
    render(BackupView, { onimport });
    await screen.findByText('暂无快照');
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    Object.defineProperty(input, 'files', { configurable: true, value: [new File(['# imported\n'], 'config.yaml', { type: 'text/yaml' })] });
    await fireEvent.change(input);
    await waitFor(() => expect(onimport).toHaveBeenCalledWith('# imported\n'));
  });

  it('keeps the snapshot list after a version conflict', async () => {
    vi.spyOn(api, 'listConfigBackups').mockResolvedValue(backups);
    vi.spyOn(api, 'getConfigBackup').mockResolvedValue(detail);
    vi.spyOn(api, 'restoreConfigBackup').mockRejectedValue(new APIRequestError(409, {
      error: { code: 'config_version_conflict', message: '配置版本已经变化', details: { current_version: 5 } },
    }));
    render(BackupView);
    await fireEvent.click(await screen.findByRole('button', { name: '查看配置' }));
    await fireEvent.click(await screen.findByRole('button', { name: '回滚' }));
    await fireEvent.click(screen.getByRole('button', { name: '确认回滚' }));
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('请刷新快照列表后重试'));
    expect(screen.getByRole('button', { name: '回滚' })).toBeEnabled();
  });
});
