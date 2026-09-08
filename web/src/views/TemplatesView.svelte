<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { api, APIRequestError, type TemplateItem, type TemplateDetail, type TemplatePreview, type ConfigValidationIssue, type IREntry } from '../api/client';
  import PixelButton from '../components/pixel/PixelButton.svelte';
  import PixelBadge from '../components/pixel/PixelBadge.svelte';
  import PixelCard from '../components/pixel/PixelCard.svelte';
  import PixelDrawer from '../components/pixel/PixelDrawer.svelte';
  import PixelDialog from '../components/pixel/PixelDialog.svelte';
  import PixelTabs from '../components/pixel/PixelTabs.svelte';
  import PixelSelect from '../components/pixel/PixelSelect.svelte';
  import PixelCheckbox from '../components/pixel/PixelCheckbox.svelte';
  import CodeEditor from '../components/CodeEditor.svelte';
  import clientIcon from '../assets/icons/nav/clients.svg';

  let { onstatechange }: { onstatechange?: (dirty: boolean, busy: boolean) => void } = $props();
  let items = $state<TemplateItem[]>([]);
  let loading = $state(true);
  let busy = $state(false);
  let validating = $state(false);
  let testing = $state(false);
  let open = $state(false);
  let current = $state<TemplateDetail>();
  let content = $state('');
  let baseline = $state('');
  let issues = $state<ConfigValidationIssue[]>([]);
  let message = $state('');
  let error = $state(false);
  let discard = $state(false);
  let subView = $state<'source' | 'test'>('source');
  let testError = $state('');
  let requestId = 0;
  const readonly = $derived(!!current?.builtin);
  const dirty = $derived(open && !readonly && content !== baseline);
  $effect(() => { onstatechange?.(dirty, busy || validating); });
  onMount(() => { void load(); });
  onDestroy(() => { requestId++; onstatechange?.(false, false); });

  const initialTemplate = `id: custom-client
name: Custom Client
codec: linelist
extension: .list
kind_map:
  domain: DOMAIN
  domain_suffix: DOMAIN-SUFFIX
  domain_keyword: DOMAIN-KEYWORD
  ip_cidr: IP-CIDR
flag_kinds:
  - ip_cidr
hints:
  ip_cidr:
    ipv6_type_name: IP-CIDR6
`;

  const ruleKinds = [
    { value: 'domain', label: '域名 (domain)' },
    { value: 'domain_suffix', label: '后缀 (domain_suffix)' },
    { value: 'domain_keyword', label: '关键字 (domain_keyword)' },
    { value: 'domain_regex', label: '正则 (domain_regex)' },
    { value: 'domain_wildcard', label: '通配符 (domain_wildcard)' },
    { value: 'geosite', label: 'Geosite' },
    { value: 'ip_cidr', label: 'IP CIDR (ip_cidr)' },
    { value: 'ip_suffix', label: 'IP 后缀 (ip_suffix)' },
    { value: 'ip_asn', label: 'IP ASN (ip_asn)' },
    { value: 'geoip', label: 'GeoIP' },
    { value: 'dst_port', label: '目标端口 (dst_port)' },
    { value: 'src_port', label: '源端口 (src_port)' },
    { value: 'network', label: '网络协议 (network)' },
    { value: 'protocol', label: '应用协议 (protocol)' },
    { value: 'process_name', label: '进程名 (process_name)' },
  ];

  const presets: Record<string, IREntry[]> = {
    default: [
      { kind: 'domain', value: 'example.com' },
      { kind: 'domain_suffix', value: 'example.org' },
      { kind: 'domain_keyword', value: 'example' },
      { kind: 'ip_cidr', value: '192.0.2.0/24', flags: ['no-resolve'] },
      { kind: 'ip_cidr', value: '2001:db8::/32' },
    ],
    domain: [
      { kind: 'domain', value: 'google.com' },
      { kind: 'domain_suffix', value: 'github.com' },
      { kind: 'domain_keyword', value: 'twitter' },
      { kind: 'domain_regex', value: '.*\\.internal' },
      { kind: 'geosite', value: 'cn' },
    ],
    ip: [
      { kind: 'ip_cidr', value: '1.1.1.1/32', flags: ['no-resolve'] },
      { kind: 'ip_cidr', value: '192.168.0.0/16' },
      { kind: 'ip_cidr', value: '2001:db8::/32' },
      { kind: 'geoip', value: 'cn' },
      { kind: 'ip_asn', value: '13335' },
    ],
  };

  let testRules = $state<IREntry[]>(JSON.parse(JSON.stringify(presets.default)));
  let testPreview = $state<TemplatePreview>();
  let testTimer: ReturnType<typeof setTimeout> | null = null;

  async function load() {
    loading = true;
    try { items = (await api.listTemplates()).items; }
    catch (e) { fail(e); }
    finally { loading = false; }
  }
  function fail(e: unknown) {
    error = true; message = (e as Error).message;
    if (e instanceof APIRequestError) issues = e.details.errors ?? [];
  }
  function create() {
    current = undefined; content = initialTemplate; baseline = content;
    issues = []; message = ''; subView = 'source'; testPreview = undefined; open = true;
  }
  async function edit(item: TemplateItem) {
    busy = true; message = '';
    try {
      current = await api.getTemplate(item.id);
      content = current.yaml; baseline = content;
      issues = []; message = ''; subView = 'source'; testPreview = undefined; open = true;
    } catch (e) { fail(e); }
    finally { busy = false; }
  }
  async function copy(item: TemplateItem) {
    busy = true; message = '';
    try {
      const detail = await api.getTemplate(item.id);
      content = detail.yaml.replace(/^id:.*$/m, 'id: custom-client');
      current = undefined; baseline = ''; issues = []; message = ''; subView = 'source'; testPreview = undefined; open = true;
    } catch (e) { fail(e); }
    finally { busy = false; }
  }
  function changed(value: string) {
    if (value === content) return;
    content = value; issues = []; message = '';
    requestId++; validating = false;
    if (subView === 'test') scheduleTest();
  }
  function requestClose() {
    if (busy) return false;
    if (dirty) { discard = true; return false; }
    requestId++; validating = false;
    return true;
  }

  // 仅校验语法，显示校验结果或报错，不跳转 TAB
  async function validate() {
    const id = ++requestId;
    validating = true; issues = []; message = '';
    try {
      await api.validateTemplate(content);
      if (id !== requestId) return;
      error = false;
      message = '模板语法校验通过';
    } catch (e) {
      if (id === requestId) fail(e);
    } finally {
      if (id === requestId) validating = false;
    }
  }

  function scheduleTest() {
    if (testTimer) clearTimeout(testTimer);
    testTimer = setTimeout(() => { void runTest(); }, 250);
  }

  async function runTest() {
    testing = true; testError = '';
    try {
      const activeRules = testRules.filter(r => r.value.trim().length > 0);
      const result = await api.validateTemplate(content, activeRules.length > 0 ? activeRules : undefined);
      testPreview = result;
      testError = '';
    } catch (e: any) {
      testError = e.message || '测试渲染失败';
      if (e instanceof APIRequestError && e.details?.errors?.length) {
        testError += '：' + e.details.errors.map((err: any) => err.message).join('；');
      }
    } finally {
      testing = false;
    }
  }

  function addRule() {
    testRules = [...testRules, { kind: 'domain', value: '' }];
  }
  function removeRule(index: number) {
    testRules = testRules.filter((_, i) => i !== index);
    scheduleTest();
  }
  function moveRule(index: number, offset: number) {
    const next = [...testRules];
    [next[index], next[index + offset]] = [next[index + offset], next[index]];
    testRules = next;
    scheduleTest();
  }
  function loadPreset(key: string) {
    if (presets[key]) {
      testRules = JSON.parse(JSON.stringify(presets[key]));
      scheduleTest();
    }
  }
  function toggleFlag(entry: IREntry, flag: string, checked: boolean) {
    const flags = entry.flags ?? [];
    if (checked) {
      if (!flags.includes(flag)) entry.flags = [...flags, flag];
    } else {
      entry.flags = flags.filter(f => f !== flag);
      if (!entry.flags.length) delete entry.flags;
    }
    scheduleTest();
  }
  function kindPlaceholder(kind: string): string {
    if (kind === 'ip_cidr') return 'IP 网段 (如 192.168.1.0/24)';
    if (kind === 'ip_suffix') return 'IP 后缀 (如 .1 或 /24)';
    if (kind === 'geoip' || kind === 'geosite') return '分类代号 (如 cn, google)';
    if (kind === 'dst_port' || kind === 'src_port') return '端口 (如 443 或 80,443)';
    if (kind === 'domain_keyword') return '关键字 (如 google)';
    if (kind === 'domain_suffix') return '后缀 (如 example.org)';
    if (kind === 'domain_regex') return '正则 (如 .*\\.internal)';
    return '规则匹配值';
  }

  async function save() {
    if (busy || readonly) return;
    requestId++; validating = false; busy = true; message = ''; issues = [];
    try {
      current = await api.saveTemplate(content, current?.id, current?.version);
      baseline = content;
      items = [...items.filter(i => i.id !== current!.id), current].sort((a, b) => a.id.localeCompare(b.id));
      error = false; message = '模板已保存';
    } catch (e) { fail(e); }
    finally { busy = false; }
  }
</script>

<div class="templates-page">
  <div class="toolbar">
    <span>{items.length} 个模板</span>
    <div class="actions">
      <PixelButton disabled={loading || busy} onclick={load}>刷新</PixelButton>
      <PixelButton variant="primary" disabled={loading || busy} onclick={create}>新建模板</PixelButton>
    </div>
  </div>
  {#if message && !open}<div class="notice" class:error role="status">{message}</div>{/if}
  {#if loading}
    <div class="notice">读取模板…</div>
  {:else}
    <div class="cards">
      {#each items as item (item.id)}
        <PixelCard class="template-card">
          <div class="heading">
            <h2>{item.name || item.id}</h2>
            <PixelBadge>{item.builtin ? '内置 · 只读' : '自定义'}</PixelBadge>
          </div>
          <p class="template-id"><code>{item.id}</code></p>
          <p class="meta">{item.codec} · {item.extension}</p>
          <div class="card-footer">
            <div class="actions">
              <PixelButton size="sm" disabled={busy} onclick={() => copy(item)}>复制</PixelButton>
              <PixelButton size="sm" variant={item.builtin ? 'secondary' : 'primary'} disabled={busy} onclick={() => edit(item)}>
                {item.builtin ? '查看模板' : '编辑模板'}
              </PixelButton>
            </div>
          </div>
        </PixelCard>
      {/each}
    </div>
  {/if}
</div>

<PixelDrawer bind:open title={readonly ? '查看模板' : current ? '编辑模板' : '新建模板'} icon={clientIcon} width="1160px" onrequestclose={requestClose}>
  <div class="editor">
    <!-- 默认展开 TAB，不再依赖点击才展开 -->
    <div class="drawer-subnav">
      <PixelTabs
        id="template-subtabs"
        label="视图模式"
        items={[
          { value: 'source', label: '模板 YAML' },
          { value: 'test', label: '测试' },
        ]}
        value={subView}
        onchange={v => {
          subView = v as 'source' | 'test';
          if (subView === 'test') void runTest();
        }}
      />
      {#if message && !error}
        <div class="preview-status-pill">
          <span class="status-dot"></span>
          <span>{message}</span>
        </div>
      {/if}
    </div>

    {#if subView === 'source'}
      <!-- 模板 YAML 源码编辑视图 -->
      <section class="source-view">
        <div class="pane-head">
          <h3>模板 YAML</h3>
          <span class="pane-badge">{current ? `${current.id}.yaml` : 'template.yaml'}</span>
        </div>
        <div class="editor-wrapper">
          <CodeEditor
            value={content}
            filename={current ? `${current.id}.yaml` : 'template.yaml'}
            language="yaml"
            readonly={readonly || busy}
            {issues}
            label="模板 YAML"
            onchange={changed}
            fill
          />
        </div>
        {#if message}
          <div class="status-notice" class:error role="status">{message}</div>
        {/if}
        {#if issues.length}
          <ul class="issues" role="alert">
            {#each issues as issue}
              <li>{issue.line ? `第 ${issue.line} 行：` : ''}{issue.message}</li>
            {/each}
          </ul>
        {/if}
      </section>
    {:else if subView === 'test'}
      <!-- 测试 TAB：左侧自定义/内置加载 IR 规则，右侧最终输出结果并排 -->
      <div class="test-layout">
        <!-- 左侧：IR 规则构建与内置加载 -->
        <section class="test-pane ir-builder-pane">
          <div class="pane-head">
            <div class="head-left">
              <h3>测试 IR 规则（{testRules.length} 条）</h3>
            </div>
            <div class="head-actions">
              <span class="preset-label">预设:</span>
              <button type="button" class="preset-btn" onclick={() => loadPreset('default')}>默认</button>
              <button type="button" class="preset-btn" onclick={() => loadPreset('domain')}>域名</button>
              <button type="button" class="preset-btn" onclick={() => loadPreset('ip')}>IP</button>
              <PixelButton size="sm" onclick={addRule}>+ 添加</PixelButton>
            </div>
          </div>

          <div class="ir-rules-list">
            {#each testRules as entry, i}
              <div class="ir-rule-card">
                <div class="ir-rule-header">
                  <span class="rule-idx">{i + 1}.</span>
                  <PixelSelect
                    id="rule-kind-{i}"
                    label="规则 {i + 1} 类型"
                    options={ruleKinds}
                    value={entry.kind}
                    size="sm"
                    class="kind-select"
                    onchange={newKind => {
                      entry.kind = newKind;
                      scheduleTest();
                    }}
                  />
                  <div class="rule-actions">
                    <button type="button" class="rule-btn" disabled={i === 0} onclick={() => moveRule(i, -1)} aria-label="上移">↑</button>
                    <button type="button" class="rule-btn" disabled={i === testRules.length - 1} onclick={() => moveRule(i, 1)} aria-label="下移">↓</button>
                    <button type="button" class="rule-btn danger" onclick={() => removeRule(i)} aria-label="删除">✕</button>
                  </div>
                </div>

                <div class="ir-rule-fields">
                  <input
                    class="rule-value-input"
                    placeholder={kindPlaceholder(entry.kind)}
                    bind:value={entry.value}
                    aria-label="规则 {i + 1} 匹配值"
                    oninput={scheduleTest}
                  />
                  {#if entry.kind === 'ip_cidr' || entry.kind === 'ip_suffix'}
                    <PixelCheckbox
                      size="sm"
                      class="flag-chip"
                      label="no-resolve"
                      checked={entry.flags?.includes('no-resolve') ?? false}
                      onchange={checked => toggleFlag(entry, 'no-resolve', checked)}
                      title="切换 no-resolve 标志"
                    />
                  {/if}
                </div>
              </div>
            {/each}
            {#if testRules.length === 0}
              <div class="empty-rules">
                <p>当前暂无测试 IR 规则</p>
                <PixelButton size="sm" onclick={() => loadPreset('default')}>载入默认预设</PixelButton>
              </div>
            {/if}
          </div>
        </section>

        <!-- 右侧：最终结果输出 -->
        <section class="test-pane result-pane">
          <div class="pane-head">
            <div class="head-left">
              <h3>最终结果（输出）</h3>
              <span class="pane-badge">preview{testPreview?.extension || (current ? current.extension : '.list')}</span>
            </div>
            <div class="head-actions">
              <PixelButton size="sm" variant="primary" disabled={testing} onclick={runTest}>
                {testing ? '测试中…' : '刷新输出'}
              </PixelButton>
            </div>
          </div>

          <div class="editor-wrapper">
            {#if testError}
              <div class="notice error" role="alert">{testError}</div>
            {/if}
            <CodeEditor
              value={testPreview?.output ?? ''}
              filename={`preview${testPreview?.extension || (current ? current.extension : '.list')}`}
              mode="preview"
              fill
              label="目标客户端配置输出"
            />
          </div>
          {#if testPreview && !testPreview.output.trim()}
            <p class="meta empty-notice">此模板未输出规则，请检查 IR 规则类型与模板的匹配关系。</p>
          {/if}
        </section>
      </div>
    {/if}
  </div>

  {#snippet footer()}
    <PixelButton disabled={busy} onclick={() => { if (requestClose()) open = false; }}>关闭</PixelButton>
    <!-- 仅校验模板语法正确性，不跳转 TAB -->
    <PixelButton disabled={busy || validating} onclick={validate}>{validating ? '校验中…' : '校验'}</PixelButton>
    {#if !readonly}
      <PixelButton variant="primary" disabled={busy || (!!current && !dirty)} onclick={save}>{busy ? '保存中…' : '保存模板'}</PixelButton>
    {/if}
  {/snippet}
</PixelDrawer>

<PixelDialog bind:open={discard} title="放弃未保存的模板？" confirmLabel="放弃修改" cancelLabel="继续编辑" danger onconfirm={() => { requestId++; validating = false; open = false; }}>当前模板尚未保存，关闭后将丢弃这些修改。</PixelDialog>

<style>
  .templates-page { display: grid; gap: 16px; }
  .toolbar, .heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
  .toolbar { color: var(--sec); }
  .actions { display: flex; gap: 8px; }
  .cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 16px; }
  :global(.template-card) { display: flex; flex-direction: column; }
  :global(.template-card > .pixel-card-body) { display: flex; flex-direction: column; flex: 1; min-height: 0; }
  .template-id { margin: 8px 0 4px; }
  .meta { color: var(--sec); margin: 0 0 14px; font: 12px/20px var(--font-code); }
  .card-footer { margin-top: auto; padding-top: 14px; border-top: 1px solid var(--border); display: flex; justify-content: flex-end; }
  .card-footer .actions { margin: 0; }
  h2 { font: 400 24px/28px var(--font-display); margin: 0; color: var(--display); }
  h3 { font: 400 12px/20px var(--font-ui); margin: 0; color: var(--display); }
  code { font: 12px/20px var(--font-code); color: var(--sec); overflow-wrap: anywhere; }
  .notice { padding: 10px 14px; border: 1px solid var(--border-vis); border-radius: 4px; background: var(--surface-2); overflow-wrap: anywhere; }
  .notice.error, .issues { background: var(--status-error); }
  .issues { padding: 12px 12px 12px 30px; border-radius: 3px; overflow-wrap: anywhere; margin: 0; }

  /* 抽屉内部结构 */
  .editor { display: flex; flex-direction: column; height: 100%; min-height: 560px; gap: 12px; padding-bottom: 24px; }
  .drawer-subnav { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; padding-bottom: 6px; border-bottom: 1px solid var(--border); }
  .preview-status-pill { display: inline-flex; align-items: center; gap: 6px; font: 12px/18px var(--font-ui); color: var(--status-ok, #4ade80); }
  .status-dot { width: 6px; height: 6px; border-radius: 50%; background: currentColor; }

  .pane-head { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-bottom: 8px; flex-wrap: wrap; }
  .head-left { display: flex; align-items: center; gap: 8px; }
  .head-actions { display: flex; align-items: center; gap: 6px; }
  .pane-badge { font: 11px/16px var(--font-code); color: var(--sec); background: var(--surface-2); border: 1px solid var(--border-vis); padding: 1px 6px; border-radius: 3px; }

  .source-view { display: flex; flex-direction: column; flex: 1; min-height: 480px; }
  .editor-wrapper { flex: 1; min-height: 460px; display: flex; flex-direction: column; }
  .status-notice { margin-top: 10px; padding: 8px 12px; border: 1px solid var(--border-vis); border-radius: 4px; background: var(--surface-2); font: 12px/18px var(--font-ui); }
  .status-notice.error { background: var(--status-error); }

  /* 测试 TAB 布局：1:1 并排 */
  .test-layout { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; flex: 1; min-height: 500px; }
  .test-pane { display: flex; flex-direction: column; min-width: 0; height: 100%; }

  /* 左侧 IR 规则列表 */
  .ir-rules-list { flex: 1; min-height: 460px; max-height: calc(100vh - 230px); overflow-y: auto; display: flex; flex-direction: column; gap: 10px; padding-right: 4px; }
  .ir-rule-card { background: var(--surface-2); border: 1px solid var(--border-vis); border-radius: 4px; padding: 10px 12px; display: flex; flex-direction: column; gap: 8px; position: relative; }
  .ir-rule-card:focus-within, :global(.ir-rule-card:has([aria-expanded='true'])) { z-index: 10; }
  .ir-rule-header { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
  .rule-idx { font: 12px/20px var(--font-code); color: var(--sec); min-width: 18px; }
  :global(.kind-select) { flex: 1; min-width: 0; }
  .rule-actions { display: flex; align-items: center; gap: 4px; }
  .rule-btn { min-width: 24px; height: 24px; padding: 0 4px; background: var(--surface); color: var(--text); border: 1px solid var(--border-vis); border-radius: 3px; font: 11px/16px var(--font-ui); cursor: pointer; display: inline-flex; align-items: center; justify-content: center; }
  .rule-btn:hover:not(:disabled) { background: var(--surface-3); }
  .rule-btn:disabled { opacity: 0.35; cursor: not-allowed; }
  .rule-btn.danger:hover:not(:disabled) { background: var(--status-error); }

  .ir-rule-fields { display: flex; align-items: center; gap: 8px; }
  .rule-value-input { flex: 1; min-width: 0; height: 28px; padding: 2px 8px; background: var(--surface); color: var(--text); border: 1px solid var(--border-vis); border-radius: 3px; font: 12px/18px var(--font-code); box-shadow: var(--edge-inset); }
  .rule-value-input:focus-visible { outline: 1px solid var(--selected); border-color: var(--selected); }

  :global(.flag-chip) { display: inline-flex; align-items: center; height: 28px; padding: 0 8px; border: 1px solid var(--border-vis); border-radius: 3px; background: var(--surface); font-family: var(--font-code); color: var(--sec); cursor: pointer; user-select: none; white-space: nowrap; box-shadow: var(--edge-raised); }
  :global(.flag-chip:hover) { background: var(--surface-2); color: var(--text); }
  :global(.flag-chip.checked) { background: var(--surface-3); color: var(--text); border-color: var(--border-vis); }

  /* 预设快捷按钮 */
  .preset-label { font: 11px/16px var(--font-ui); color: var(--sec); margin-right: 2px; }
  .preset-btn { height: 24px; padding: 0 6px; background: var(--surface-2); color: var(--sec); border: 1px solid var(--border-vis); border-radius: 3px; font: 11px/16px var(--font-ui); cursor: pointer; }
  .preset-btn:hover { background: var(--surface-3); color: var(--text); }

  .empty-rules { padding: 28px; text-align: center; background: var(--surface-2); border: 1px dashed var(--border); border-radius: 4px; display: flex; flex-direction: column; align-items: center; gap: 10px; color: var(--sec); font: 12px/20px var(--font-ui); }
  .empty-notice { margin-top: 8px; color: var(--sec); }

  @media (max-width: 800px) {
    .test-layout { grid-template-columns: 1fr; }
  }
</style>
