import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ChangesView from './ChangesView.svelte';

afterEach(() => {
  vi.restoreAllMocks();
});

describe('ChangesView', () => {
  it('renders a unified IR diff with exact totals and omitted counts', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({
      items: [{
        update_id: 'run-1',
        finished_at: '2026-08-16T12:00:00Z',
        origin: 'web',
        scope: 'all',
        rule_id: 'openai',
        rule_name: 'OpenAI',
        added: 101,
        removed: 2,
        added_samples: ['domain,new.example'],
        removed_samples: ['domain,old.example', 'ip_cidr,192.0.2.0/24'],
        added_omitted: 100,
        removed_omitted: 0,
      }],
    }), { status: 200, headers: { 'Content-Type': 'application/json' } })));

    render(ChangesView);

    expect(await screen.findByText('OpenAI')).toBeInTheDocument();
    expect(screen.getByText('+101')).toBeInTheDocument();
    expect(screen.getByText('-2')).toBeInTheDocument();

    await fireEvent.click(screen.getByText('OpenAI'));
    const diff = screen.getByRole('region', { name: 'OpenAI IR Diff' });
    expect(within(diff).getByText('- domain,old.example')).toHaveClass('del-line');
    expect(within(diff).getByText('+ domain,new.example')).toHaveClass('add-line');
    expect(within(diff).getByText('… 另有 100 条新增已省略')).toBeInTheDocument();
    expect(diff).not.toHaveTextContent('client');
  });

  it('shows the logical-change empty state', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ items: [] }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    })));

    render(ChangesView);

    await waitFor(() => expect(screen.getByText('暂无变更记录')).toBeInTheDocument());
  });
});
