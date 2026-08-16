<script module lang="ts">
  const previewCache = new Map<string, { text: string; stat: string }>();
  const cacheLimit = 6;

  export function clearPreviewCache() {
    previewCache.clear();
  }
</script>

<script lang="ts">
  import PixelButton from '../../components/pixel/PixelButton.svelte';
  import PixelIcon from '../../components/pixel/PixelIcon.svelte';
  import type { PreviewItem, PublicClient, PublicClientOption } from '../types';
  import { copyURL, formatBytes, formatCount } from '../utils';

  interface Props {
    item: PreviewItem | null;
    client: PublicClient;
    target: PublicClientOption;
    onclose: () => void;
  }

  let { item, client, target, onclose }: Props = $props();
  let previewState = $state<'loading' | 'ready' | 'empty' | 'error'>('loading');
  let previewText = $state('加载中…');
  let previewStat = $state('READING');
  let copied = $state(false);
  const previewLines = 500;

  function remember(path: string, value: { text: string; stat: string }) {
    previewCache.delete(path);
    previewCache.set(path, value);
    if (previewCache.size > cacheLimit) {
      previewCache.delete(previewCache.keys().next().value!);
    }
  }

  $effect(() => {
    if (item) {
      const originalOverflow = document.body.style.overflow;
      document.body.style.overflow = 'hidden';
      return () => {
        document.body.style.overflow = originalOverflow;
      };
    }
  });

  $effect(() => {
    const current = item;
    copied = false;
    if (!current) return;
    if (!current.path) {
      previewState = 'empty';
      previewText = '没有可预览的内容。';
      previewStat = 'EMPTY';
      return;
    }
    const cached = previewCache.get(current.path);
    if (cached) {
      remember(current.path, cached);
      previewState = 'ready';
      previewText = cached.text;
      previewStat = cached.stat;
      return;
    }

    const controller = new AbortController();
    previewState = 'loading';
    previewText = '加载中…';
    previewStat = 'READING';
    fetch(current.path, { signal: controller.signal })
      .then((response) => {
        if (!response.ok) throw new Error(`http ${response.status}`);
        return response.text();
      })
      .then((text) => {
        const lines = text.split('\n');
        const total = lines.length;
        const truncated = total > previewLines;
        const rendered = truncated
          ? `${lines.slice(0, previewLines).join('\n')}\n… 仅显示前 ${previewLines} 行，完整内容请打开文件`
          : text;
        const stat = `${truncated ? `${previewLines} / ` : ''}${total} LINES`;
        remember(current.path!, { text: rendered, stat });
        previewState = 'ready';
        previewText = rendered;
        previewStat = stat;
      })
      .catch((error) => {
        if (error instanceof DOMException && error.name === 'AbortError') return;
        previewState = 'error';
        previewText = '无法加载预览。请通过 prm serve 访问，或点「打开文件」查看。';
        previewStat = 'LOAD ERROR';
      });
    return () => controller.abort();
  });

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape' && item) onclose();
  }

  async function handleCopy() {
    if (!item?.path) return;
    const ok = await copyURL(item.path);
    if (ok) {
      copied = true;
      setTimeout(() => { copied = false; }, 1500);
    }
  }
</script>

<svelte:window onkeydown={handleKeydown} />

{#if item}
  <div class="modal-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget) onclose(); }}>
    <div class="file-modal" role="dialog" aria-modal="true" aria-labelledby="preview-title">
      <header class="modal-head">
        <h2 id="preview-title">{item.title}</h2>
        <button type="button" class="modal-close" aria-label="关闭" onclick={onclose}><PixelIcon name="cross" size={12} /></button>
      </header>
      <div class="modal-tags">{#each item.tags as tag}<em>{tag}</em>{/each}</div>
      {#if item.description}<p class="modal-description">{item.description}</p>{/if}
      <div class="modal-meta">
        <img src={`static/icons/${client.icon}.svg`} width="24" height="24" alt="" aria-hidden="true" />
        <strong>{client.options.length > 1 ? `${client.name} · ${target.name}` : client.name}</strong>
        <span>{formatBytes(item.size)}</span>
        <span>{formatCount(item.entries)} 条</span>
        <div class="modal-actions">
          <PixelButton size="sm" disabled={!item.path} onclick={handleCopy}>{copied ? '已复制' : '复制链接'}</PixelButton>
          {#if item.path}<a class="pill-btn" href={item.path} target="_blank" rel="noopener">打开文件</a>{/if}
        </div>
      </div>
      <div class="preview-shell" data-state={previewState}>
        <header><span><i></i> FILE PREVIEW</span><span>{item.path?.split('/').pop() || 'NO FILE'}</span><b>{previewStat}</b></header>
        <pre>{#each previewText.split('\n') as line, index}<span data-line={index + 1}>{line || ' '}</span>{/each}</pre>
      </div>
    </div>
  </div>
{/if}
