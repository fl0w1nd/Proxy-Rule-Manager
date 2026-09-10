<script lang="ts">
  import { onMount } from 'svelte';
  import CodePanel from './CodePanel.svelte';
  import { OverlayScrollbars } from 'overlayscrollbars';
  import { RETRO_SCROLLBAR_OPTIONS, setupArrowClicks } from '../utils/scrollbars';
  import { Compartment, EditorState, type Extension } from '@codemirror/state';
  import { EditorView, keymap, lineNumbers, highlightActiveLine, highlightActiveLineGutter, drawSelection, panels, placeholder as placeholderExt } from '@codemirror/view';
  import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands';
  import { bracketMatching, indentOnInput, syntaxHighlighting, HighlightStyle } from '@codemirror/language';
  import { yaml } from '@codemirror/lang-yaml';
  import { javascript } from '@codemirror/lang-javascript';
  import { lintGutter, setDiagnostics } from '@codemirror/lint';
  import { search, searchKeymap, highlightSelectionMatches } from '@codemirror/search';
  import { tags } from '@lezer/highlight';
  import type { ConfigValidationIssue } from '../api/client';

  export type EditorLanguage = 'yaml' | 'javascript' | 'text';

  let {
    value,
    filename = '',
    mode = 'editor',
    language = 'text',
    issues = [],
    readonly = false,
    label = '文件内容',
    fill = false,
    height = '',
    placeholder = '',
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
    /** Explicit container height (e.g. '200px'); defaults to the tall panel sizing. */
    height?: string;
    placeholder?: string;
    onchange?: (value: string) => void;
  } = $props();

  let host: HTMLDivElement;
  let panelsTop: HTMLDivElement;
  let view = $state<EditorView>();
  let lineCount = $state(1);
  const isReadonly = $derived(readonly || mode === 'preview');
  const editable = new Compartment();
  const lang = new Compartment();
  const attrs = new Compartment();
  const hint = new Compartment();
  const colors = HighlightStyle.define([
    { tag: tags.comment, color: 'var(--terminal-muted)', fontStyle: 'italic' },
    { tag: [tags.string, tags.special(tags.string)], color: 'var(--diff-add)' },
    { tag: [tags.number, tags.bool, tags.null], color: 'var(--sec)' },
    { tag: [tags.propertyName, tags.definition(tags.propertyName)], color: 'var(--display)' },
    { tag: [tags.variableName, tags.keyword], color: 'var(--terminal-text)' },
    { tag: [tags.punctuation, tags.meta], color: 'var(--terminal-muted)' },
  ]);

  function languageExtensions(mode: EditorLanguage): Extension[] {
    if (mode === 'yaml') return [yaml(), lintGutter(), syntaxHighlighting(colors)];
    if (mode === 'javascript') return [javascript(), syntaxHighlighting(colors)];
    return [];
  }

  onMount(() => {
    const editor = new EditorView({
      parent: host,
      state: EditorState.create({
        doc: value,
        extensions: [
          lineNumbers(), history(), drawSelection(), bracketMatching(), indentOnInput(),
          highlightActiveLine(), highlightActiveLineGutter(),
          search({ top: true }),
          // OverlayScrollbars 会把宿主改成横向 flex；面板容器若挂在编辑器根节点
          // 会被挤成左侧窄条，因此重定向到编辑器外的专用容器。
          panels({ topContainer: panelsTop }),
          highlightSelectionMatches(),
          keymap.of([...defaultKeymap, ...historyKeymap, ...searchKeymap, indentWithTab]),
          EditorState.lineSeparator.of(value.includes('\r\n') ? '\r\n' : '\n'),
          lang.of(languageExtensions(language)),
          editable.of([EditorState.readOnly.of(isReadonly), EditorView.editable.of(!isReadonly)]),
          attrs.of(EditorView.contentAttributes.of({ 'aria-label': label, 'aria-multiline': 'true' })),
          hint.of(placeholder ? placeholderExt(placeholder) : []),
          EditorView.updateListener.of(update => {
            if (update.docChanged) {
              lineCount = update.state.doc.lines;
              onchange?.(update.state.sliceDoc());
            }
          }),
          EditorView.theme({
            '&': { backgroundColor: 'var(--terminal-bg)', color: 'var(--terminal-text)', fontSize: '13px' },
            '.cm-scroller': { fontFamily: 'var(--font-code)', lineHeight: '20px', overflow: 'auto' },
            '.cm-content': { caretColor: 'var(--terminal-text)', padding: '10px 0' },
            '.cm-line': { padding: '0 16px' },
            '.cm-lineNumbers .cm-gutterElement': { minWidth: '52px', padding: '0 12px 0 8px' },
            '.cm-gutters': { backgroundColor: 'var(--terminal-bg)', color: 'var(--terminal-muted)', borderRight: '1px solid var(--border)' },
            '.cm-activeLine, .cm-activeLineGutter': { backgroundColor: 'var(--surface-2)' },
            '&.cm-focused .cm-selectionBackground, .cm-selectionBackground': { backgroundColor: 'var(--selected)' },
            '.cm-content ::selection, .cm-line ::selection, .cm-line span::selection': { backgroundColor: 'var(--selected) !important', color: 'var(--selected-text) !important' },
            '.cm-cursor, .cm-dropCursor': { borderLeftColor: 'var(--terminal-text)' },
            '.cm-content:focus, .cm-content:focus-visible': { outline: 'none' },
            '.cm-placeholder': { color: 'var(--terminal-muted)' },
            '.cm-tooltip': { backgroundColor: 'var(--surface)', color: 'var(--text)', border: '1px solid var(--border-vis)', fontFamily: 'var(--font-code)' },
            '.cm-diagnostic-error': { borderLeftColor: 'var(--error-border)' },
            '.cm-selectionMatch': { backgroundColor: 'var(--surface-3)' },
            '.cm-searchMatch': { backgroundColor: 'var(--status-warning)', outline: '1px solid var(--border-vis)' },
            '.cm-searchMatch-selected': { backgroundColor: 'var(--selected)', color: 'var(--selected-text)' },
            '.cm-panels': { backgroundColor: 'var(--surface-2)', color: 'var(--text)', borderBottom: '1px solid var(--border-vis)' },
            '.cm-panel.cm-search': {
              padding: '8px 38px 8px 12px',
              display: 'flex', flexWrap: 'wrap', alignItems: 'center',
              columnGap: '8px', rowGap: '6px',
              fontFamily: 'var(--font-ui)', fontSize: '12px',
            },
            '.cm-panel.cm-search input, .cm-panel.cm-search button, .cm-panel.cm-search label': { margin: '0' },
            '.cm-panel.cm-search br': { flex: '0 0 100%', height: '0' },
            '.cm-panel.cm-search label': {
              display: 'inline-flex', alignItems: 'center', gap: '6px',
              color: 'var(--sec)', fontSize: '12px', whiteSpace: 'nowrap', cursor: 'pointer',
            },
            '.cm-panel.cm-search input[type="checkbox"]': {
              appearance: 'none', width: '13px', height: '13px', margin: '0', flex: 'none',
              backgroundColor: 'var(--surface)', border: '1px solid var(--border-vis)', borderRadius: '2px',
              boxShadow: 'var(--edge-inset)', cursor: 'pointer',
            },
            '.cm-panel.cm-search input[type="checkbox"]:checked': {
              backgroundColor: 'var(--accent)', boxShadow: 'none',
              backgroundImage: 'linear-gradient(var(--text), var(--text))',
              backgroundSize: '5px 5px', backgroundPosition: 'center', backgroundRepeat: 'no-repeat',
            },
            '.cm-panel.cm-search .cm-textfield': {
              backgroundColor: 'var(--surface)', color: 'var(--text)',
              border: '1px solid var(--border-vis)', borderRadius: '3px',
              boxShadow: 'var(--edge-inset)', padding: '3px 8px',
              minHeight: '26px', boxSizing: 'border-box',
              fontFamily: 'var(--font-code)', fontSize: '12px',
              flex: '1 1 160px', minWidth: '120px',
            },
            '.cm-panel.cm-search .cm-button': {
              backgroundColor: 'var(--surface)', color: 'var(--text)',
              backgroundImage: 'none',
              border: '1px solid var(--border-vis)', borderRadius: '4px',
              boxShadow: 'var(--edge-raised)', padding: '3px 10px',
              minHeight: '26px', boxSizing: 'border-box',
              fontFamily: 'var(--font-ui)', fontSize: '12px', lineHeight: '18px', cursor: 'pointer',
            },
            '.cm-panel.cm-search .cm-button:hover': { backgroundColor: 'var(--surface-3)' },
            '.cm-panel.cm-search .cm-button:active': { boxShadow: 'var(--edge-pressed)' },
            '.cm-panel.cm-search button[name="close"]': {
              position: 'absolute', top: '7px', right: '10px',
              width: '22px', height: '22px', padding: '0',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              backgroundColor: 'transparent', border: '1px solid transparent', borderRadius: '3px',
              color: 'var(--sec)', fontSize: '14px', lineHeight: '1',
              fontFamily: 'var(--font-ui)', cursor: 'pointer',
            },
            '.cm-panel.cm-search button[name="close"]:hover': { color: 'var(--text)', backgroundColor: 'var(--surface-3)' },
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
    view.dispatch({ effects: hint.reconfigure(placeholder ? placeholderExt(placeholder) : []) });
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

<div class="code-editor" class:fill style={height ? `height: ${height}; min-height: 0;` : undefined}>
  <CodePanel {filename} {mode} stat={`${lineCount} LINES`} fill>
    <div class="editor-host">
      <div class="editor-panels" bind:this={panelsTop}></div>
      <div class="editor-scroll" bind:this={host}></div>
    </div>
  </CodePanel>
</div>

<style>
  .code-editor { display: flex; flex-direction: column; min-width: 0; overflow: hidden; }
  .code-editor:not(.fill) { height: min(65vh, 720px); min-height: 320px; }
  .code-editor.fill { flex: 1; min-height: 0; height: 100%; }
  .editor-host { flex: 1; min-height: 0; overflow: hidden; display: flex; flex-direction: column; }
  .editor-panels { flex: none; min-width: 0; }
  .editor-scroll { flex: 1 1 auto; min-height: 0; display: flex; flex-direction: column; }
  .editor-scroll :global(.cm-editor) { flex: 1 1 auto; min-height: 0; }
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
