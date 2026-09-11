import { fireEvent, render, screen } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';
import { beforeAll, describe, expect, it, vi } from 'vitest';
import PixelCheckbox from './PixelCheckbox.svelte';
import PixelDialog from './PixelDialog.svelte';
import PixelQuantity from './PixelQuantity.svelte';
import PixelSelect from './PixelSelect.svelte';
import PixelSwitch from './PixelSwitch.svelte';
import PixelTabs from './PixelTabs.svelte';
import PixelTooltip from './PixelTooltip.svelte';

const options = [{ value: 'manual', label: '手动更新' }, { value: 'interval', label: '固定间隔' }, { value: 'cron', label: 'Cron 定时' }];

beforeAll(() => {
  HTMLDialogElement.prototype.showModal = function showModal() { this.setAttribute('open', ''); };
  HTMLDialogElement.prototype.close = function close() { this.removeAttribute('open'); };
});

describe('pixel controls', () => {
  it('selects with the keyboard and closes on outside clicks and Tab', async () => {
    const user = userEvent.setup();
    const onchange = vi.fn();
    render(PixelSelect, { id: 'mode', label: '调度模式', options, value: 'manual', onchange });
    await user.tab();
    expect(screen.getByRole('combobox')).toHaveFocus();
    await user.keyboard('{ArrowDown}{ArrowDown}{Enter}');
    expect(onchange).toHaveBeenCalledWith('interval');
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
    await user.click(screen.getByRole('combobox'));
    await fireEvent.pointerDown(document.body);
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
    await user.click(screen.getByRole('combobox'));
    await user.tab();
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
  });

  it('toggles a switch using Space', async () => {
    const user = userEvent.setup();
    render(PixelSwitch, { label: '自动更新' });
    await user.tab();
    await user.keyboard(' ');
    expect(screen.getByRole('switch')).toHaveAttribute('aria-checked', 'true');
  });

  it('moves tab selection and focus with arrow keys, skipping disabled tabs', async () => {
    const user = userEvent.setup();
    render(PixelTabs, { id: 'settings', label: '设置', items: [options[0], { ...options[1], disabled: true }, options[2]], value: 'manual' });
    await user.tab();
    await user.keyboard('{ArrowRight}');
    expect(screen.getByRole('tab', { name: 'Cron 定时' })).toHaveFocus();
    expect(screen.getByRole('tab', { name: 'Cron 定时' })).toHaveAttribute('aria-selected', 'true');
  });

  it('closes a dialog on Escape and restores the previous focus', async () => {
    const oncancel = vi.fn();
    const trigger = document.createElement('button');
    trigger.textContent = '打开';
    document.body.append(trigger);
    trigger.focus();
    render(PixelDialog, { open: true, title: '离开运行设置？', oncancel });
    const dialog = document.querySelector('dialog');
    expect(dialog).toHaveAttribute('open');
    await fireEvent(dialog!, new Event('cancel', { cancelable: true }));
    expect(oncancel).toHaveBeenCalled();
    expect(trigger).toHaveFocus();
    trigger.remove();
  });

  it('shows a tooltip on focus and closes it with Escape', async () => {
    const user = userEvent.setup();
    render(PixelTooltip, { label: '单主机并发', text: '同一主机同时下载的上限，不能超过全局并发。' });
    await user.tab();
    expect(screen.getByRole('tooltip')).toHaveTextContent('同一主机同时下载的上限，不能超过全局并发。');
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
  });

  it('glues a numeric amount to a unit select and serializes on edit', async () => {
    const user = userEvent.setup();
    const base = { id: 'timeout', label: '抓取超时', kind: 'duration' as const, units: ['ms', 's', 'm'] };
    let value = '15s';
    const onchange = vi.fn((next: string) => { value = next; });
    const view = render(PixelQuantity, { ...base, value, onchange });
    const amount = screen.getByRole('textbox');
    expect(amount).toHaveValue('15');
    expect(screen.getByRole('combobox', { name: '抓取超时单位' })).toHaveTextContent('秒');
    await user.click(screen.getByRole('combobox', { name: '抓取超时单位' }));
    await user.click(screen.getByRole('option', { name: '分' }));
    expect(onchange).toHaveBeenCalledWith('15m');
    await view.rerender({ ...base, value, onchange });
    expect(amount).toHaveValue('15');
    expect(screen.getByRole('combobox', { name: '抓取超时单位' })).toHaveTextContent('分');
    await fireEvent.input(amount, { target: { value: '2' } });
    expect(onchange).toHaveBeenLastCalledWith('2m');
  });

  it('confirms from the primary action and restores the previous focus', async () => {
    const user = userEvent.setup();
    const onconfirm = vi.fn();
    const trigger = document.createElement('button');
    trigger.textContent = '打开';
    document.body.append(trigger);
    trigger.focus();
    render(PixelDialog, { open: true, title: '离开运行设置？', confirmLabel: '放弃修改并离开', onconfirm });
    await user.click(screen.getByRole('button', { name: '放弃修改并离开' }));
    expect(onconfirm).toHaveBeenCalled();
    expect(trigger).toHaveFocus();
    trigger.remove();
  });

  it('toggles a pixel checkbox via click and keyboard', async () => {
    const user = userEvent.setup();
    const onchange = vi.fn();
    render(PixelCheckbox, { label: 'no-resolve', size: 'sm', onchange });
    const checkbox = screen.getByRole('checkbox', { name: 'no-resolve' });
    expect(checkbox).not.toBeChecked();
    await user.click(checkbox);
    expect(checkbox).toBeChecked();
    expect(onchange).toHaveBeenLastCalledWith(true);
    await user.keyboard(' ');
    expect(checkbox).not.toBeChecked();
    expect(onchange).toHaveBeenLastCalledWith(false);
  });
});
