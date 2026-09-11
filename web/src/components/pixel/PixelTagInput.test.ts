import { fireEvent, render, screen } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import PixelTagInput from './PixelTagInput.svelte';

describe('PixelTagInput', () => {
  it('renders initial tags and allows deleting a tag by clicking remove button', async () => {
    const onchange = vi.fn();
    const user = userEvent.setup();
    render(PixelTagInput, { value: ['core', 'direct'], onchange });

    expect(screen.getByText('core')).toBeInTheDocument();
    expect(screen.getByText('direct')).toBeInTheDocument();

    const removeButtons = screen.getAllByRole('button', { name: /删除标签/ });
    expect(removeButtons).toHaveLength(2);

    await user.click(removeButtons[0]);
    expect(onchange).toHaveBeenCalledWith(['direct']);
  });

  it('adds a tag on Enter and clears input field', async () => {
    const onchange = vi.fn();
    const user = userEvent.setup();
    render(PixelTagInput, { value: ['core'], onchange });

    const input = screen.getByRole('textbox', { name: '标签输入' });
    await user.type(input, 'proxy{enter}');

    expect(onchange).toHaveBeenCalledWith(['core', 'proxy']);
    expect(input).toHaveValue('');
  });

  it('adds a tag on comma input and prevents duplicate tags', async () => {
    const onchange = vi.fn();
    const user = userEvent.setup();
    render(PixelTagInput, { value: ['core'], onchange });

    const input = screen.getByRole('textbox', { name: '标签输入' });
    await user.type(input, 'media,');

    expect(onchange).toHaveBeenCalledWith(['core', 'media']);

    // Attempt to type existing tag
    await user.type(input, 'core{enter}');
    // Should not add duplicate
    expect(onchange).not.toHaveBeenCalledWith(['core', 'media', 'core']);
  });

  it('deletes the last tag on Backspace when input is empty', async () => {
    const onchange = vi.fn();
    const user = userEvent.setup();
    render(PixelTagInput, { value: ['tag1', 'tag2'], onchange });

    const input = screen.getByRole('textbox', { name: '标签输入' });
    await user.click(input);
    await user.keyboard('{Backspace}');

    expect(onchange).toHaveBeenCalledWith(['tag1']);
  });

  it('commits pending input on blur', async () => {
    const onchange = vi.fn();
    const user = userEvent.setup();
    render(PixelTagInput, { value: [], onchange });

    const input = screen.getByRole('textbox', { name: '标签输入' });
    await user.type(input, 'unconfirmed');
    await fireEvent.blur(input);

    expect(onchange).toHaveBeenCalledWith(['unconfirmed']);
    expect(input).toHaveValue('');
  });

  it('splits pasted comma-separated text into multiple tags', async () => {
    const onchange = vi.fn();
    render(PixelTagInput, { value: ['init'], onchange });

    const input = screen.getByRole('textbox', { name: '标签输入' });
    await fireEvent.paste(input, {
      clipboardData: {
        getData: (format: string) => (format === 'text' ? 'alpha, beta，gamma' : ''),
      },
    });

    expect(onchange).toHaveBeenCalledWith(['init', 'alpha', 'beta', 'gamma']);
  });

  it('respects disabled state', async () => {
    render(PixelTagInput, { value: ['tag1'], disabled: true });
    const input = screen.getByRole('textbox', { name: '标签输入' });
    expect(input).toBeDisabled();
    expect(screen.queryByRole('button', { name: /删除标签/ })).not.toBeInTheDocument();
  });
});
