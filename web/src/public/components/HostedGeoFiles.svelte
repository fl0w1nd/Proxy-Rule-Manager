<script lang="ts">
  import type { PublicGeoFile, PublicView } from '../types';
  import { copyURL, formatBytes } from '../utils';

  interface Props {
    files: PublicGeoFile[];
    kind?: PublicView;
  }

  let { files, kind }: Props = $props();

  const labels: Record<string, string> = {
    geosite: 'Geosite',
    geoip: 'GeoIP',
    mmdb: 'MMDB',
    asn: 'ASN',
  };

  const title = $derived.by(() => {
    if (kind && labels[kind]) {
      return `${labels[kind]} 原始数据库`;
    }
    return '原始数据库';
  });

  const providerGroups = $derived.by(() => {
    const map = new Map<string, { provider: string; version?: string; files: PublicGeoFile[] }>();
    for (const f of files) {
      if (!map.has(f.provider)) {
        map.set(f.provider, { provider: f.provider, version: f.version, files: [] });
      }
      map.get(f.provider)!.files.push(f);
    }
    return Array.from(map.values());
  });

  async function copy(path: string, button: HTMLButtonElement) {
    const ok = await copyURL(path);
    if (ok) {
      const original = button.textContent;
      button.textContent = '已复制';
      setTimeout(() => { button.textContent = original; }, 1500);
    }
  }
</script>

{#if files.length > 0}
  <section class="public-block" aria-labelledby="geo-files-heading">
    <div class="block-head">
      <h2 class="block-title" id="geo-files-heading">{title}</h2>
    </div>
    {#each providerGroups as group (group.provider)}
      <article class="geo-provider hosted-geo-card">
        <header>
          <strong>{group.provider}</strong>
          {#if group.version}
            <span class="provider-version">版本：<code>{group.version}</code></span>
          {/if}
        </header>
        <div class="file-list">
          {#each group.files as file (`${file.kind}/${file.provider}/${file.name}`)}
            <div class="hosted-file-row">
              <div class="file-info">
                <span class="file-name">{file.name}</span>
                <span class="file-size">{formatBytes(file.size)}</span>
              </div>
              <div class="row-actions">
                <button class="pill-btn" type="button" onclick={(event) => copy(file.path, event.currentTarget)}>复制链接</button>
                <a class="pill-btn" href={file.path} target="_blank" rel="noopener" download={file.name}>下载</a>
              </div>
            </div>
          {/each}
        </div>
      </article>
    {/each}
  </section>
{:else if kind === 'mmdb' || kind === 'asn'}
  <section class="public-block" aria-labelledby="geo-files-heading">
    <div class="block-head">
      <h2 class="block-title" id="geo-files-heading">{title}</h2>
    </div>
    <div class="empty">还没有发布 {labels[kind] || ''} 原始数据库。</div>
  </section>
{/if}

<style>
  .hosted-geo-card {
    margin-bottom: 16px;
    border: 1px solid var(--border-vis);
    border-radius: 4px;
    background: var(--surface);
    box-shadow: var(--shadow-panel);
    overflow: hidden;
  }
  .hosted-geo-card > header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 12px 18px;
    background: var(--surface-2);
    border-bottom: 1px solid var(--border-vis);
  }
  .hosted-geo-card > header strong {
    font: 400 20px/24px var(--font-display);
    color: var(--display);
    letter-spacing: 0;
  }
  .provider-version {
    color: var(--dim);
    font: 12px/20px var(--font-ui);
  }
  .provider-version code {
    font: 12px/20px var(--font-code);
    color: var(--sec);
  }
  .file-list {
    display: flex;
    flex-direction: column;
  }
  .hosted-file-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 12px 18px;
    border-bottom: 1px solid var(--border);
    transition: background-color 80ms linear;
  }
  .hosted-file-row:last-child {
    border-bottom: 0;
  }
  .hosted-file-row:hover {
    background: var(--surface-2);
  }
  .file-info {
    display: flex;
    align-items: baseline;
    gap: 12px;
    min-width: 0;
    flex-wrap: wrap;
  }
  .file-name {
    font: 400 20px/24px var(--font-display);
    color: var(--display);
    overflow-wrap: anywhere;
  }
  .file-size {
    font: 12px/20px var(--font-ui);
    color: var(--dim);
    font-variant-numeric: tabular-nums;
  }
  .row-actions {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-shrink: 0;
  }
  @media (max-width: 520px) {
    .hosted-file-row {
      flex-direction: column;
      align-items: flex-start;
      gap: 10px;
    }
    .row-actions {
      width: 100%;
      justify-content: flex-end;
    }
  }
</style>
