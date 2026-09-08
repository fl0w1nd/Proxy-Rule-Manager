<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { api, APIRequestError, type ConfigDocument, type ConfigSnapshot } from '../api/client';
  import PixelTabs from '../components/pixel/PixelTabs.svelte';
  import PixelSelect from '../components/pixel/PixelSelect.svelte';
  import PixelQuantity from '../components/pixel/PixelQuantity.svelte';
  import PixelTooltip from '../components/pixel/PixelTooltip.svelte';
  import PixelButton from '../components/pixel/PixelButton.svelte';
  import PixelIcon from '../components/pixel/PixelIcon.svelte';
  import { buildSettingsPatch, readSettings, scheduleIntervalUnits, settingsChanged, settingGroups, settingTabs, validateSettings, type RuntimeSettings } from './settings';

  interface Props { onstatechange?: (dirty: boolean, saving: boolean) => void; }
  let { onstatechange }: Props = $props();
  let snapshot = $state<ConfigSnapshot | null>(null);
  let baseline = $state<RuntimeSettings>(readSettings({}));
  let form = $state<RuntimeSettings>(readSettings({}));
  let loading = $state(true);
  let saving = $state(false);
  let message = $state('');
  let success = $state(false);
  let errors = $state<Record<string, string>>({});
  let formElement = $state<HTMLFormElement>();
  const dirty = $derived(!!snapshot && settingsChanged(baseline, form));
  const scheduleOptions = [
    { value: 'manual', label: '手动更新' },
    { value: 'interval', label: '固定间隔' },
    { value: 'cron', label: 'Cron 定时' },
  ];
  $effect(() => { onstatechange?.(dirty, saving); });

  async function load() {
    loading = true;
    message = '';
    errors = {};
    try {
      const result = await api.getConfig();
      baseline = readSettings(result.config);
      form = structuredClone($state.snapshot(baseline));
      snapshot = result;
    } catch (error) {
      success = false;
      snapshot = null;
      message = `读取设置失败：${(error as Error).message}`;
    } finally { loading = false; }
  }
  onMount(() => { void load(); });

  async function save(event: SubmitEvent) {
    event.preventDefault();
    if (!snapshot || saving || !dirty) return;
    message = '';
    errors = validateSettings(form, baseline);
    if (Object.keys(errors).length) {
      await tick();
      formElement?.querySelector<HTMLInputElement>('[aria-invalid="true"]')?.focus();
      return;
    }
    saving = true;
    success = false;
    try {
      const ops = buildSettingsPatch(snapshot.config, baseline, form);
      const result = await api.patchConfig(snapshot.version, ops);
      const config: ConfigDocument = structuredClone($state.snapshot(snapshot.config));
      const update = (config.update ??= {}) as Record<string, unknown>;
      for (const op of ops) {
        if ('value' in op) {
          if (op.op === 'update_history') Object.assign(update, op.value);
          else update[op.op.replace('update_', '')] = op.value;
        }
      }
      snapshot = { version: result.version, config };
      baseline = readSettings(config);
      form = structuredClone($state.snapshot(baseline));
      success = true;
      message = ['设置已保存', ...(result.warnings ?? [])].join('；');
    } catch (error) {
      message = `保存失败：${(error as Error).message}`;
      if (error instanceof APIRequestError) {
        if (error.status === 409) message += '。请刷新页面后重试。';
        for (const issue of error.details.errors ?? []) {
          const path = issue.path.replace(/^update\./, '');
          errors[path.startsWith('history_') ? `history.${path}` : path] = issue.message;
        }
      }
    } finally { saving = false; }
  }
</script>

<svelte:window onbeforeunload={(event) => { if (dirty || saving) { event.preventDefault(); event.returnValue = ''; } }} />

<div class="settings-view">
  <PixelTabs id="settings" label="系统设置" items={settingTabs} value="runtime" />
  <div role="tabpanel" id="settings-panel-runtime" aria-labelledby="settings-tab-runtime">
    {#if loading}
      <div class="settings-status" role="status">正在读取设置…</div>
    {:else if !snapshot}
      <div class="settings-status error" role="alert">
        <PixelIcon name="warn" size={16} />
        <span>{message}</span>
        <PixelButton size="sm" onclick={load}>重试</PixelButton>
      </div>
    {:else}
      <form bind:this={formElement} onsubmit={save} novalidate>
        <fieldset disabled={saving}>
          <section aria-labelledby="schedule-title">
            <h2 id="schedule-title">更新计划</h2>
            <div class="fields">
              <div class="field">
                <div class="label-row">
                  <label for="schedule-mode">调度模式</label>
                  <PixelTooltip label="调度模式" text="自动更新只在管理服务运行期间生效。" />
                </div>
                <PixelSelect id="schedule-mode" label="调度模式" options={scheduleOptions}
                  value={String(form.schedule.mode)} disabled={saving} onchange={(value) => { form.schedule.mode = value; }} />
              </div>
              <div class="field">
                <div class="label-row">
                  <label for="schedule-timezone">时区</label>
                  <PixelTooltip label="时区" text="Cron 按此时区解释。Local 表示本机时区。" />
                </div>
                <input id="schedule-timezone" class="pixel-input" bind:value={form.schedule.timezone} aria-invalid={!!errors['schedule.timezone']} aria-describedby="schedule-timezone-help" />
                <span id="schedule-timezone-help" class:error={!!errors['schedule.timezone']}>{errors['schedule.timezone'] || '例如 UTC、Asia/Shanghai'}</span>
              </div>
              {#if form.schedule.mode === 'interval'}
                <div class="field">
                  <label for="schedule-interval">更新间隔</label>
                  <PixelQuantity id="schedule-interval" label="更新间隔" kind="duration" units={scheduleIntervalUnits}
                    value={String(form.schedule.interval)} invalid={!!errors['schedule.interval']}
                    describedby={errors['schedule.interval'] ? 'schedule-interval-help' : undefined}
                    onchange={(next) => { form.schedule.interval = next; }} />
                  {#if errors['schedule.interval']}<span id="schedule-interval-help" class="error">{errors['schedule.interval']}</span>{/if}
                </div>
              {:else if form.schedule.mode === 'cron'}
                <div class="field">
                  <label for="schedule-cron">Cron 表达式</label>
                  <input id="schedule-cron" class="pixel-input" bind:value={form.schedule.cron} aria-invalid={!!errors['schedule.cron']} aria-describedby="schedule-cron-help" />
                  <span id="schedule-cron-help" class:error={!!errors['schedule.cron']}>{errors['schedule.cron'] || '分 时 日 月 周，例如 0 3 * * *'}</span>
                </div>
              {/if}
            </div>
          </section>
          {#each settingGroups as group}
            <section aria-labelledby="{group.key}-title">
              <h2 id="{group.key}-title">{group.title}</h2>
              <div class="fields">
                {#each group.fields as field}
                  {@const path = `${group.key}.${field.key}`}
                  <div class="field" class:wide={field.key === 'user_agent'}>
                    <div class="label-row">
                      <label for={path}>{field.label}</label>
                      {#if field.tooltip}<PixelTooltip label={field.label} text={field.tooltip} />{/if}
                    </div>
                    {#if field.kind === 'duration' || field.kind === 'size'}
                      <PixelQuantity id={path} label={field.label} kind={field.kind} units={field.units ?? []}
                        value={String(form[group.key][field.key])} invalid={!!errors[path]}
                        describedby={errors[path] ? `${path}-help` : undefined}
                        onchange={(next) => { form[group.key][field.key] = next; }} />
                    {:else}
                      <input id={path} class="pixel-input" bind:value={form[group.key][field.key]}
                        inputmode={field.kind === 'number' ? 'numeric' : undefined}
                        aria-invalid={!!errors[path]} aria-describedby={field.hint || errors[path] ? `${path}-help` : undefined} />
                    {/if}
                    {#if errors[path] || field.hint}<span id="{path}-help" class:error={!!errors[path]}>{errors[path] || field.hint}</span>{/if}
                  </div>
                {/each}
              </div>
            </section>
          {/each}
        </fieldset>
        <div class="save-bar">
          <PixelButton type="submit" variant="primary" disabled={!dirty || saving}>{saving ? '保存中…' : '保存设置'}</PixelButton>
        </div>
        {#if message && (!success || !dirty)}<p class="message" class:success role={success ? 'status' : 'alert'}>{message}</p>{/if}
      </form>
    {/if}
  </div>
</div>

<style>
  .settings-view { display: flex; flex-direction: column; gap: 20px; max-width: 1000px; }
  .settings-status {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 14px;
    border: 1px solid var(--border-vis);
    border-radius: 4px;
    background: var(--surface-2);
    color: var(--text);
    font: 400 12px/20px var(--font-ui);
  }
  .settings-status.error { background: var(--status-error); }
  form { border: 1px solid var(--border-vis); border-radius: 4px; background: var(--surface); box-shadow: inset 0 1px 0 var(--bevel-light), var(--shadow-panel); }
  form > :last-child { border-radius: 0 0 4px 4px; }
  fieldset { min-width: 0; margin: 0; padding: 0; border: 0; }
  section { padding: 20px 24px 24px; }
  section + section { border-top: 1px solid var(--border); }
  h2 { margin: 0 0 12px; font: 400 12px/20px var(--font-ui); color: var(--display); }
  .fields { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px 24px; }
  .field { display: flex; flex-direction: column; gap: 6px; min-width: 0; }
  .label-row { display: flex; align-items: center; gap: 6px; min-width: 0; }
  .wide { grid-column: 1 / -1; }
  label { font: 400 12px/20px var(--font-ui); color: var(--text); }
  .field span { font: 400 12px/20px var(--font-ui); color: var(--sec); }
  .field .error { color: var(--error-border); }
  input { width: 100%; min-width: 0; }
  .save-bar { display: flex; align-items: center; justify-content: flex-end; flex-wrap: wrap; gap: 12px; padding: 16px 24px; border-top: 1px solid var(--border-vis); background: var(--surface-2); }
  .message { margin: 0; padding: 12px 24px; background: var(--status-error); color: var(--text); font: 400 12px/20px var(--font-ui); overflow-wrap: anywhere; }
  .message.success { background: var(--status-success); }
  @media (max-width: 600px) {
    .fields { grid-template-columns: minmax(0, 1fr); }
    section { padding: 16px; }
    .save-bar { padding: 16px; }
  }
</style>
