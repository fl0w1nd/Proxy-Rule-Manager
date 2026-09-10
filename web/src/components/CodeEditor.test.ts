import { render, screen, waitFor } from '@testing-library/svelte';
import { expect, it } from 'vitest';
import { EditorView } from '@codemirror/view';
import { openSearchPanel } from '@codemirror/search';
import CodeEditor from './CodeEditor.svelte';

async function mount(value = 'clients: []\n', props: Record<string, unknown> = {}) {
  const { container } = render(CodeEditor, { value, language: 'yaml', label: '测试编辑器', ...props });
  const content = await screen.findByRole('textbox', { name: '测试编辑器' });
  return { container, view: EditorView.findFromDOM(content)! };
}

it('opens the search panel inside the dedicated top container', async () => {
  const { container, view } = await mount();
  openSearchPanel(view);
  await waitFor(() => expect(container.querySelector('.editor-panels .cm-panel.cm-search')).toBeTruthy());
  // 面板容器若落在编辑器根节点（横向 flex 宿主）会被挤成侧栏，这里锁定结构
  expect(container.querySelector('.cm-editor > .cm-panels-top')).toBeNull();
  expect(container.querySelector('.cm-scroller .cm-panels-top')).toBeNull();
  expect(container.querySelector('.editor-scroll')).toContainElement(container.querySelector('.cm-editor'));
});

it('keeps the panel empty and the editor filling the host when search is closed', async () => {
  const { container } = await mount();
  expect(container.querySelector('.editor-panels')).toBeEmptyDOMElement();
  expect(container.querySelector('.cm-panel.cm-search')).toBeNull();
});
