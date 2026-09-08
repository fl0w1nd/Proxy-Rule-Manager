<script lang="ts">
  import { onMount } from 'svelte';
  import { OverlayScrollbars } from 'overlayscrollbars';
  import { RETRO_SCROLLBAR_OPTIONS, setupArrowClicks } from '../utils/scrollbars';
  import { Compartment, EditorState } from '@codemirror/state';
  import { EditorView, keymap, lineNumbers, highlightActiveLine, highlightActiveLineGutter, drawSelection } from '@codemirror/view';
  import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands';
  import { bracketMatching, indentOnInput, syntaxHighlighting, HighlightStyle } from '@codemirror/language';
  import { yaml } from '@codemirror/lang-yaml';
  import { lintGutter, setDiagnostics } from '@codemirror/lint';
  import { tags } from '@lezer/highlight';
  import type { ConfigValidationIssue } from '../api/client';

  let { value, issues = [], readonly = false, onchange }: {
    value: string; issues?: ConfigValidationIssue[]; readonly?: boolean; onchange: (value: string) => void;
  } = $props();
  let host: HTMLDivElement;
  let view = $state<EditorView>();
  const editable = new Compartment();
  const colors = HighlightStyle.define([
    { tag: tags.comment, color: 'var(--terminal-muted)', fontStyle: 'italic' },
    { tag: [tags.string, tags.special(tags.string)], color: '#b8dba2' },
    { tag: [tags.number, tags.bool, tags.null], color: '#efdca4' },
    { tag: [tags.propertyName, tags.definition(tags.propertyName)], color: '#aed2ef' },
    { tag: [tags.punctuation, tags.meta], color: 'var(--terminal-muted)' },
  ]);
  onMount(() => {
    const editor = new EditorView({
      parent: host,
      state: EditorState.create({
        doc: value,
        extensions: [
          yaml(), lineNumbers(), history(), drawSelection(), bracketMatching(), indentOnInput(),
          highlightActiveLine(), highlightActiveLineGutter(), lintGutter(), syntaxHighlighting(colors),
          keymap.of([...defaultKeymap, ...historyKeymap, indentWithTab]),
          EditorState.lineSeparator.of(value.includes('\r\n') ? '\r\n' : '\n'),
          editable.of(EditorState.readOnly.of(readonly)),
          EditorView.contentAttributes.of({ 'aria-label': 'YAML 配置文件', 'aria-multiline': 'true' }),
          EditorView.updateListener.of(update => { if (update.docChanged) onchange(update.state.sliceDoc()); }),
          EditorView.theme({
            '&': { backgroundColor: 'var(--terminal-bg)', color: 'var(--terminal-text)', fontSize: '13px', height: 'min(65vh, 720px)', minHeight: '320px' },
            '.cm-scroller': { fontFamily: 'var(--font-code)', lineHeight: '20px', overflow: 'auto' },
            '.cm-content': { caretColor: 'var(--terminal-text)', padding: '8px 0' },
            '.cm-gutters': { backgroundColor: 'var(--terminal-bg)', color: 'var(--terminal-muted)', borderRight: '1px solid var(--border)' },
            '.cm-activeLine, .cm-activeLineGutter': { backgroundColor: '#ffffff0b' },
            '&.cm-focused .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection': { backgroundColor: '#58765388' },
            '.cm-cursor, .cm-dropCursor': { borderLeftColor: 'var(--terminal-text)' },
            '&.cm-focused, .cm-content:focus': { outline: 'none' },
            '.cm-tooltip': { backgroundColor: 'var(--surface)', color: 'var(--text)', border: '1px solid var(--border-vis)', fontFamily: 'var(--font-code)' },
            '.cm-diagnostic-error': { borderLeftColor: 'var(--error-border)' },
          }, { dark: true }),
        ],
      }),
    });
    const scrollbars = OverlayScrollbars({
      target: editor.dom,
      elements: { viewport: editor.scrollDOM },
    }, RETRO_SCROLLBAR_OPTIONS);
    const cleanupArrows = setupArrowClicks(scrollbars);
    view = editor;
    return () => {
      cleanupArrows();
      scrollbars.destroy();
      editor.destroy();
    };
  });
  $effect(() => {
    if (!view) return;
    view.dispatch({ effects: editable.reconfigure(EditorState.readOnly.of(readonly)) });
  });
  $effect(() => {
    if (!view) return;
    const editor = view;
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

<div class="yaml-editor" bind:this={host}></div>

<style>
  .yaml-editor { min-width: 0; overflow: hidden; border-block: 1px solid var(--border-vis); }
</style>
