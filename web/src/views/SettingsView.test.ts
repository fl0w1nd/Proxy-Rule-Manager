import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api, APIRequestError } from '../api/client';
import SettingsView from './SettingsView.svelte';

const snapshot = { version: 7, config: { update: { fetch: { timeout: '20s', user_agent: 'PRM-UA' } } } };
afterEach(() => vi.restoreAllMocks());

describe('SettingsView', () => {
  it('initializes clean and saves a typed patch with the loaded version', async () => {
    vi.spyOn(api, 'getConfig').mockResolvedValue(snapshot);
    const patch = vi.spyOn(api, 'patchConfig').mockResolvedValue({ version: 8, warnings: [] });
    render(SettingsView);
    const timeout = await screen.findByLabelText('抓取超时');
    expect(timeout).toHaveValue('20');
    expect(screen.getByLabelText('保留时长')).toHaveValue('7');
    expect(screen.getByRole('button', { name: '单主机并发说明' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '保存设置' })).toBeDisabled();
    await fireEvent.input(timeout, { target: { value: '' } });
    await fireEvent.click(screen.getByRole('button', { name: '保存设置' }));
    expect(patch).not.toHaveBeenCalled();
    expect(timeout).toHaveAttribute('aria-invalid', 'true');
    await fireEvent.input(timeout, { target: { value: '25' } });
    await fireEvent.click(screen.getByRole('button', { name: '保存设置' }));
    await screen.findByText('设置已保存');
    expect(patch).toHaveBeenCalledWith(7, [{ op: 'update_fetch', value: { timeout: '25s', user_agent: 'PRM-UA' } }]);
    expect(screen.getByRole('button', { name: '保存设置' })).toBeDisabled();
    expect(screen.getByRole('tab', { name: '备份与恢复' })).toBeEnabled();
  });

  it('retains edits after a version conflict', async () => {
    vi.spyOn(api, 'getConfig').mockResolvedValue(snapshot);
    vi.spyOn(api, 'patchConfig').mockRejectedValue(new APIRequestError(409, {
      error: { code: 'version_conflict', message: '配置版本已变化', details: { current_version: 8 } },
    }));
    render(SettingsView);
    const timeout = await screen.findByLabelText('抓取超时');
    await fireEvent.input(timeout, { target: { value: '25' } });
    await fireEvent.click(screen.getByRole('button', { name: '保存设置' }));
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('请刷新页面后重试'));
    expect(timeout).toHaveValue('25');
    expect(screen.getByRole('button', { name: '保存设置' })).toBeEnabled();
  });

  it('opens an imported file in the config editor', async () => {
    vi.spyOn(api, 'getConfig').mockResolvedValue(snapshot);
    vi.spyOn(api, 'getConfigRaw').mockResolvedValue({ yaml: 'clients: []\n', path: 'config.yaml', version: 7 });
    vi.spyOn(api, 'listConfigBackups').mockResolvedValue({ version: 7, items: [] });
    render(SettingsView);
    await screen.findByLabelText('抓取超时');
    await fireEvent.click(screen.getByRole('tab', { name: '备份与恢复' }));
    await screen.findByRole('button', { name: '导入配置文件' });
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    Object.defineProperty(input, 'files', { configurable: true, value: [new File(['# imported\n'], 'config.yaml', { type: 'text/yaml' })] });
    await fireEvent.change(input);
    await screen.findByRole('textbox', { name: 'YAML 配置文件' });
    expect(screen.getByRole('button', { name: '保存配置' })).toBeEnabled();
  });
});
