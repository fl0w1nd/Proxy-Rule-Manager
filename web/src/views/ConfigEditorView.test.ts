import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, expect, it, vi } from 'vitest';
import { EditorView } from '@codemirror/view';
import { api, APIRequestError } from '../api/client';
import ConfigEditorView from './ConfigEditorView.svelte';

afterEach(() => vi.restoreAllMocks());

async function editor() {
  const element = await screen.findByRole('textbox', { name: 'YAML 配置文件' });
  return EditorView.findFromDOM(element)!;
}

it('preserves the editor and selection when saving the submitted source', async () => {
  vi.spyOn(api, 'getConfigRaw').mockResolvedValue({ yaml: '# comment\nclients: []\n', path: 'config.yaml', version: 4 });
  const save = vi.spyOn(api, 'saveConfigRaw').mockResolvedValue({ version: 5, warnings: [] });
  render(ConfigEditorView);
  const view = await editor();
  view.dispatch({ changes: { from: 9, insert: ' updated' }, selection: { anchor: 17 } });
  await fireEvent.click(screen.getByRole('button', { name: '保存配置' }));
  await screen.findByText('配置已保存');
  expect(save).toHaveBeenCalledWith('# comment updated\nclients: []\n', 4);
  expect(await editor()).toBe(view);
  expect(view.state.selection.main.anchor).toBe(17);
  expect(screen.getByRole('button', { name: '保存配置' })).toBeDisabled();
});

it('clears diagnostics on edits and discards validation of an older draft', async () => {
  vi.spyOn(api, 'getConfigRaw').mockResolvedValue({ yaml: '@clients: []\n', path: 'config.yaml', version: 1 });
  const validationError = new APIRequestError(422, { error: { code: 'config_invalid', message: '配置校验失败', details: { errors: [{ line: 1, path: 'config', message: 'invalid YAML' }] } } });
  const validate = vi.spyOn(api, 'validateConfig').mockRejectedValue(validationError);
  render(ConfigEditorView);
  const view = await editor();
  await fireEvent.click(screen.getByRole('button', { name: '校验' }));
  await screen.findByRole('button', { name: /第 1 行/ });
  view.dispatch({ changes: { from: 0, to: 1 } });
  await waitFor(() => expect(screen.queryByLabelText('配置错误')).not.toBeInTheDocument());
  let reject!: (error: unknown) => void;
  validate.mockImplementation(() => new Promise((_, fail) => { reject = fail; }));
  await fireEvent.click(screen.getByRole('button', { name: '校验' }));
  view.dispatch({ changes: { from: 0, insert: '# comment\n' } });
  reject(validationError);
  await waitFor(() => expect(screen.getByRole('button', { name: '校验' })).toBeEnabled());
  expect(screen.queryByLabelText('配置错误')).not.toBeInTheDocument();
});
