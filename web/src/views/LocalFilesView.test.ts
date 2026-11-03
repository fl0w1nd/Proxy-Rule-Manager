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

  it('shows lint status, inserts rules and confirms saving unparseable content', async () => {
    const broken = { ...item, entries: 1, diagnostics: 1, format: 'classical', rules: [{ id: 'direct', name: 'Direct' }] };
    vi.spyOn(api, 'listLocalFiles').mockResolvedValue({ items: [broken] });
    vi.spyOn(api, 'getRuleSyntax').mockResolvedValue({ types: [{ name: 'DOMAIN-SUFFIX', kind: 'domain_suffix' }, { name: 'IP-CIDR', kind: 'ip_cidr' }], flags: ['no-resolve'] });
    vi.spyOn(api, 'getLocalFile').mockResolvedValue({ ...broken, content: 'DOMAIN,a.com\nNOPE,b\n' });
    const report = {
      format: 'classical', confidence: 1, entries: 1, kinds: [{ kind: 'domain', count: 1 }],
      diagnostics: [{ line: 2, text: 'NOPE,b', reason: 'unknown rule type "NOPE"' }], diagnostic_count: 1,
      lines: [{ line: 1, text: 'DOMAIN,a.com', kind: 'domain', value: 'a.com' }], warnings: [],
    };
    const lint = vi.spyOn(api, 'lintRules').mockResolvedValue(report);
    const save = vi.spyOn(api, 'saveLocalFile').mockResolvedValue({ ...broken, content: '' });
    render(LocalFilesView);
    expect(await screen.findByText('1 行无法解析')).toBeInTheDocument();
    expect(screen.getByText('Direct')).toBeInTheDocument();

    await fireEvent.click(screen.getByRole('button', { name: '编辑' }));
    const status = await screen.findByRole('status', { name: '语法检查' });
    await waitFor(() => expect(status).toHaveTextContent('⚠ 1 行无法解析'), { timeout: 2000 });
    expect(screen.getByRole('button', { name: /2 NOPE,b — 未知规则类型 NOPE/ })).toBeInTheDocument();

    await fireEvent.input(screen.getByLabelText('插入规则值'), { target: { value: 'example.com' } });
    await fireEvent.click(screen.getByRole('button', { name: '插入' }));
    const box = screen.getByRole('textbox', { name: '文件内容' });
    expect(EditorView.findFromDOM(box)!.state.doc.toString()).toContain('DOMAIN-SUFFIX,example.com');

    await fireEvent.click(screen.getByRole('button', { name: '保存' }));
    expect(await screen.findByText(/引用此文件的规则 Direct 会在更新时报错/)).toBeInTheDocument();
    expect(save).not.toHaveBeenCalled();
    await fireEvent.click(screen.getByRole('button', { name: '仍然保存' }));
    await waitFor(() => expect(save).toHaveBeenCalledOnce());
    expect(lint).toHaveBeenCalled();

    await fireEvent.click(screen.getByRole('tab', { name: /解析结果/ }));
    expect(screen.getByRole('cell', { name: 'a.com' })).toBeInTheDocument();
  });
});
