import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { EditorView } from '@codemirror/view';
import { api, APIRequestError } from '../api/client';
import LocalFilesView from './LocalFilesView.svelte';

const item = { name: 'used.list', size: 24, lines: 2, modified_at: '2026-09-08T12:00:00.000Z' };

beforeAll(() => {
  HTMLDialogElement.prototype.showModal = function showModal() { this.setAttribute('open', ''); };
  HTMLDialogElement.prototype.close = function close() { this.removeAttribute('open'); };
});
afterEach(() => {
  vi.restoreAllMocks();
});

describe('LocalFilesView', () => {
  it('creates a list file from the centered editor', async () => {
    vi.spyOn(api, 'listLocalFiles').mockResolvedValue({ items: [] });
    const create = vi.spyOn(api, 'createLocalFile').mockResolvedValue({
      name: 'my-direct.list', size: 28, lines: 2, modified_at: '2026-09-08T12:18:00.000Z',
      content: 'example.com\nexample.org\n',
    });
    render(LocalFilesView);
    await screen.findByText('还没有本地规则文件');
    await fireEvent.click(screen.getByRole('button', { name: '新建文件' }));
    await fireEvent.input(screen.getByLabelText('文件名'), { target: { value: '../secret.list' } });
    await fireEvent.click(screen.getByRole('button', { name: '保存' }));
    expect(create).not.toHaveBeenCalled();
    expect(screen.getByText('文件名无效')).toBeInTheDocument();
    await fireEvent.input(screen.getByRole('textbox', { name: /^文件名/ }), { target: { value: 'my-direct.list' } });
    const box = await screen.findByRole('textbox', { name: '文件内容' });
    EditorView.findFromDOM(box)!.dispatch({ changes: { from: 0, insert: 'example.com\nexample.org\n' } });
    await fireEvent.click(screen.getByRole('button', { name: '保存' }));
    await screen.findByText('已保存 my-direct.list');
    expect(create).toHaveBeenCalledWith('my-direct.list', 'example.com\nexample.org\n');
    expect(screen.getByRole('cell', { name: 'my-direct.list' })).toBeInTheDocument();
    expect(screen.getByRole('cell', { name: '2' })).toBeInTheDocument();
  });

  it('keeps a referenced file when delete is rejected', async () => {
    vi.spyOn(api, 'listLocalFiles').mockResolvedValue({ items: [item] });
    vi.spyOn(api, 'deleteLocalFile').mockRejectedValue(new APIRequestError(409, {
      error: { code: 'file_in_use', message: '文件正被规则引用，无法删除', details: { rules: [{ id: 'direct', name: 'Direct' }] } },
    }));
    render(LocalFilesView);
    await screen.findByText('used.list');
    await fireEvent.click(screen.getByRole('button', { name: '删除' }));
    await fireEvent.click(screen.getByRole('button', { name: '确认删除' }));
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('文件正被规则 Direct 引用，无法删除'));
    expect(screen.getByText('used.list')).toBeInTheDocument();
  });
});
