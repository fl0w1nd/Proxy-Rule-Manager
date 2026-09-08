<script lang="ts">
  import { onMount } from 'svelte';
  import CodePanel from './CodePanel.svelte';
  import { OverlayScrollbars } from 'overlayscrollbars';
  import { RETRO_SCROLLBAR_OPTIONS, setupArrowClicks } from '../utils/scrollbars';
  import { Compartment, EditorState, type Extension } from '@codemirror/state';
  import { EditorView, keymap, lineNumbers, highlightActiveLine, highlightActiveLineGutter, drawSelection } from '@codemirror/view';
  import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands';
  import { bracketMatching, indentOnInput, syntaxHighlighting, HighlightStyle } from '@codemirror/language';
  import { yaml } from '@codemirror/lang-yaml';
  import { lintGutter, setDiagnostics } from '@codemirror/lint';
  import { tags } from '@lezer/highlight';
  import type { ConfigValidationIssue } from '../api/client';

  export type EditorLanguage = 'yaml' | 'text';

  let {
    value,
    filename = '',
    mode = 'editor',
    language = 'text',
    issues = [],
    readonly = false,
    label = '文件内容',
    fill = false,
    onchange,
  }: {
    value: string;
    filename?: string;
    mode?: 'preview' | 'editor';
    language?: EditorLanguage;
    issues?: ConfigValidationIssue[];
    readonly?: boolean;
    label?: string;
    fill?: boolean;
    onchange?: (value: string) => void;
  } = $props();

  let host: HTMLDivElement;
  let view = $state<EditorView>();
  let lineCount = $state(1);
  const isReadonly = $derived(readonly || mode === 'preview');
  const editable = new Compartment();
  const lang = new Compartment();
  const attrs = new Compartment();
  const colors = HighlightStyle.define([
    { tag: tags.comment, color: 'var(--terminal-muted)', fontStyle: 'italic' },
    { tag: [tags.string, tags.special(tags.string)], color: 'var(--diff-add)' },
    { tag: [tags.number, tags.bool, tags.null], color: 'var(--sec)' },
    { tag: [tags.propertyName, tags.definition(tags.propertyName)], color: 'var(--display)' },
    { tag: [tags.punctuation, tags.meta], color: 'var(--terminal-muted)' },
  ]);

  function languageExtensions(mode: EditorLanguage): Extension[] {
    if (mode !== 'yaml') return [];
    return [yaml(), lintGutter(), syntaxHighlighting(colors)];
  }

  onMount(() => {
    const editor = new EditorView({
      parent: host,
      state: EditorState.create({
        doc: value,
        extensions: [
          lineNumbers(), history(), drawSelection(), bracketMatching(), indentOnInput(),
          highlightActiveLine(), highlightActiveLineGutter(),
          keymap.of([...defaultKeymap, ...historyKeymap, indentWithTab]),
          EditorState.lineSeparator.of(value.includes('\r\n') ? '\r\n' : '\n'),
          lang.of(languageExtensions(language)),
          editable.of([EditorState.readOnly.of(isReadonly), EditorView.editable.of(!isReadonly)]),
          attrs.of(EditorView.contentAttributes.of({ 'aria-label': label, 'aria-multiline': 'true' })),
          EditorView.updateListener.of(update => {
            if (update.docChanged) {
              lineCount = update.state.doc.lines;
              onchange?.(update.state.sliceDoc());
            }
          }),
          EditorView.theme({
            '&': { backgroundColor: 'var(--terminal-bg)', color: 'var(--terminal-text)', fontSize: '13px', height: '100%' },
            '.cm-scroller': { fontFamily: 'var(--font-code)', lineHeight: '20px', overflow: 'auto' },
            '.cm-content': { caretColor: 'var(--terminal-text)', padding: '10px 0' },
            '.cm-line': { padding: '0 16px' },
            '.cm-lineNumbers .cm-gutterElement': { minWidth: '52px', padding: '0 12px 0 8px' },
            '.cm-gutters': { backgroundColor: 'var(--terminal-bg)', color: 'var(--terminal-muted)', borderRight: '1px solid var(--border)' },
            '.cm-activeLine, .cm-activeLineGutter': { backgroundColor: 'var(--surface-2)' },
            '&.cm-focused .cm-selectionBackground, .cm-selectionBackground': { backgroundColor: 'var(--selected)' },
            '.cm-content ::selection, .cm-line ::selection, .cm-line span::selection': { backgroundColor: 'var(--selected) !important', color: 'var(--selected-text) !important' },
            '.cm-cursor, .cm-dropCursor': { borderLeftColor: 'var(--terminal-text)' },
            '&.cm-focused, .cm-content:focus, .cm-content:focus-visible': { outline: 'none' },
            '.cm-tooltip': { backgroundColor: 'var(--surface)', color: 'var(--text)', border: '1px solid var(--border-vis)', fontFamily: 'var(--font-code)' },
            '.cm-diagnostic-error': { borderLeftColor: 'var(--error-border)' },
          }),
        ],
      }),
    });
    const scrollbars = OverlayScrollbars({
      target: editor.dom,
      elements: { viewport: editor.scrollDOM },
    }, RETRO_SCROLLBAR_OPTIONS);
    const cleanupArrows = setupArrowClicks(scrollbars);
    lineCount = editor.state.doc.lines;
    view = editor;
    return () => {
      cleanupArrows();
      scrollbars.destroy();
      editor.destroy();
    };
  });
  $effect(() => {
    if (!view) return;
    if (view.state.sliceDoc() === value) return;
    view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: value } });
  });
  $effect(() => {
    if (!view) return;
    view.dispatch({ effects: editable.reconfigure([EditorState.readOnly.of(isReadonly), EditorView.editable.of(!isReadonly)]) });
  });
  $effect(() => {
    if (!view) return;
    view.dispatch({ effects: lang.reconfigure(languageExtensions(language)) });
  });
  $effect(() => {
    if (!view) return;
    view.dispatch({
      effects: attrs.reconfigure(EditorView.contentAttributes.of({ 'aria-label': label, 'aria-multiline': 'true' })),
    });
  });
  $effect(() => {
    if (!view) return;
    const editor = view;
    if (language !== 'yaml') {
      editor.dispatch(setDiagnostics(editor.state, []));
      return;
    }
    editor.dispatch(setDiagnostics(editor.state, issues.filter(issue => issue.line && issue.line <= editor.state.doc.lines).map(issue => {
      const line = editor.state.doc.line(issue.line!);
      return { from: line.from, to: line.to, severity: 'error' as const, message: `${issue.path}: ${issue.message}` };
    })));
  });
  export function focusLine(number: number) {
    if (!view || number < 1 || number > view.state.doc.lines) return;
    view.dispatch({ selection: { anchor: view.state.doc.line(number).from }, scrollIntoView: true });
    view.focus();
  }
</script>

<div class="code-editor" class:fill>
  <CodePanel {filename} {mode} stat={`${lineCount} LINES`} fill>
    <div class="editor-host" bind:this={host}></div>
  </CodePanel>
</div>

<style>
  .code-editor { display: flex; flex-direction: column; min-width: 0; overflow: hidden; }
  .code-editor:not(.fill) { height: min(65vh, 720px); min-height: 320px; }
  .code-editor.fill { flex: 1; min-height: 0; height: 100%; }
  .editor-host { flex: 1; min-height: 0; overflow: hidden; }
  .code-editor :global(.cm-editor),
  .code-editor :global(.cm-editor.cm-focused),
  .code-editor :global(.cm-content:focus),
  .code-editor :global(.cm-content:focus-visible) { outline: none; }
  .code-editor :global(.cm-selectionBackground),
  .code-editor :global(.cm-focused .cm-selectionBackground) { background-color: var(--selected) !important; }
  .code-editor :global(.cm-content ::selection),
  .code-editor :global(.cm-line ::selection),
  .code-editor :global(.cm-line span::selection) { background-color: var(--selected) !important; color: var(--selected-text) !important; }
</style>
