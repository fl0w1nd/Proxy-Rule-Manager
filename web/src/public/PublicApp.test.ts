import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import PublicApp from './PublicApp.svelte';
import { clearPreviewCache } from './components/FilePreviewModal.svelte';
import type { PublicPageData } from './types';

function fixture(): PublicPageData {
  return {
    updated_at: '2026-08-16T00:00:00Z',
    admin_url: '/admin',
    clients: [
      {
        id: 'clash', name: 'Clash', icon: 'clash', rules: true, geosite: true,
        options: [
          { id: 'clash-yaml', name: 'YAML', ext: '.yaml', rules: true, geosite: true },
          { id: 'clash-mrs', name: 'MRS', ext: '.mrs', binary: true, rules: true, geosite: true },
        ],
      },
      {
        id: 'surge', name: 'Surge', icon: 'surge', rules: true, geosite: false,
        options: [{ id: 'surge', name: 'Surge', ext: '.list', rules: true, geosite: false }],
      },
    ],
    rules: [
      {
        id: 'openai', name: 'OpenAI', description: 'AI services', tags: ['AI'], entries: 12,
        files: [
          { target_id: 'clash-yaml', path: 'rules/clash-yaml/OpenAI.yaml', size: 1024 },
          { target_id: 'clash-mrs', path: 'rules/clash-mrs/OpenAI.mrs', size: 512 },
          { target_id: 'surge', path: 'rules/surge/OpenAI.list', size: 256 },
        ],
      },
      {
        id: 'streaming', name: 'Streaming', tags: ['Media'], entries: 8,
        files: [{ target_id: 'clash-yaml', path: 'rules/clash-yaml/Streaming.yaml', size: 128 }],
      },
    ],
    tags: ['AI', 'Media'],
    geosite: [{
      provider: 'v2 fly',
      lists: Array.from({ length: 101 }, (_, index) => ({
        name: index === 0 ? 'category/ai' : `list-${index}`,
        entries: index + 1,
        variants: index === 0 ? [{ attr: 'cn', entries: 2 }] : [],
      })),
    }],
    icon_sets: [{
      name: 'brand set',
      count: 61,
      icons: Array.from({ length: 61 }, (_, index) => index === 0 ? 'open ai#.svg' : `icon-${index}.svg`),
    }],
  };
}

beforeEach(() => {
  localStorage.clear();
  clearPreviewCache();
  document.documentElement.setAttribute('data-theme', 'dark');
  Object.defineProperty(navigator, 'clipboard', {
    configurable: true,
    value: { writeText: vi.fn().mockResolvedValue(undefined) },
  });
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe('PublicApp', () => {
  it('filters rules by text and tags and reports the visible count', async () => {
    const user = userEvent.setup();
    render(PublicApp, { data: fixture() });

    expect(screen.getByText('2 / 2')).toBeInTheDocument();
    await user.type(screen.getByPlaceholderText('搜索规则…'), 'open');
    expect(screen.getByText('1 / 2')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'OpenAI' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Streaming' })).not.toBeInTheDocument();

    await user.clear(screen.getByPlaceholderText('搜索规则…'));
    await user.click(screen.getByRole('button', { name: 'Media' }));
    expect(screen.getByText('1 / 2')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Streaming' })).toBeInTheDocument();
  });

  it('restores client state, switches views, and falls back to a geosite-capable client', async () => {
    const user = userEvent.setup();
    localStorage.setItem('prm-client', '1');
    localStorage.setItem('prm-target-clash', 'clash-mrs');
    render(PublicApp, { data: fixture() });

    await waitFor(() => expect(localStorage.getItem('prm-client')).toBe('1'));
    await user.click(screen.getByRole('button', { name: 'Geosite' }));
    expect(localStorage.getItem('prm-client')).toBe('0');
    expect(screen.getByText('格式：').parentElement).toHaveTextContent('Clash · MRS');

    await user.click(screen.getByRole('button', { name: '规则' }));
    await user.click(screen.getByRole('button', { name: /Clash/ }));
    await user.click(screen.getByRole('radio', { name: /YAML/ }));
    expect(localStorage.getItem('prm-target-clash')).toBe('clash-yaml');

    await user.click(screen.getByRole('button', { name: '图标' }));
    expect(screen.getByRole('heading', { name: /图标集/ })).toBeInTheDocument();
  });

  it('loads, truncates, caches, copies, and closes a rule preview', async () => {
    const user = userEvent.setup();
    const writeText = vi.spyOn(navigator.clipboard, 'writeText');
    const body = Array.from({ length: 501 }, (_, index) => `line-${index + 1}`).join('\n');
    const fetchMock = vi.fn().mockResolvedValue(new Response(body, { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
    render(PublicApp, { data: fixture() });

    // Open via action button
    await user.click(screen.getByRole('button', { name: '预览 OpenAI' }));
    expect(await screen.findByText(/仅显示前 500 行/)).toBeInTheDocument();
    expect(screen.getByText('500 / 501 LINES')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '复制链接' }));
    expect(writeText).toHaveBeenCalledWith(expect.stringContaining('/rules/clash-yaml/OpenAI.yaml'));

    await fireEvent.keyDown(window, { key: 'Escape' });
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();

    // Open via row click
    await user.click(screen.getByRole('button', { name: '查看规则 OpenAI' }));
    await screen.findByText('500 / 501 LINES');
    expect(fetchMock).toHaveBeenCalledTimes(1);

    await user.click(screen.getByRole('button', { name: '关闭' }));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('shows preview failures and closes from the backdrop', async () => {
    const user = userEvent.setup();
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('', { status: 503 })));
    render(PublicApp, { data: fixture() });

    await user.click(screen.getByRole('button', { name: 'Streaming' }));
    expect(await screen.findByText('LOAD ERROR')).toBeInTheDocument();
    const dialog = screen.getByRole('dialog');
    await fireEvent.click(dialog.parentElement!);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('paginates geosite lists and refreshes encoded preview paths with the target', async () => {
    const user = userEvent.setup();
    localStorage.setItem('prm-target-clash', 'clash-mrs');
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('payload', { status: 200 })));
    render(PublicApp, { data: fixture() });
    await user.click(screen.getByRole('button', { name: 'Geosite' }));

    expect(screen.queryByText('list-100')).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '显示全部 101 个' }));
    expect(screen.getByText('list-100')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'category/ai' })).toHaveAttribute('aria-expanded');
    expect(screen.getByRole('button', { name: 'list-1' })).not.toHaveAttribute('aria-expanded');
    expect(screen.getAllByRole('link', { name: '打开' })[0]).toHaveAttribute(
      'href',
      'rules/clash-mrs/geosite/v2%20fly/category%2Fai.mrs',
    );
    expect(screen.getAllByRole('link', { name: '打开' })[0]).toHaveAttribute('download');

    // 二进制格式：弹窗说明无法预览，不去读取文件
    await user.click(screen.getAllByRole('button', { name: '预览' })[0]);
    expect(await screen.findByText(/二进制规则集.*无法以文本预览/)).toBeInTheDocument();
    expect(screen.queryByText('READING')).not.toBeInTheDocument();
    expect(vi.mocked(fetch)).not.toHaveBeenCalled();

    // 弹窗开着时切回文本格式，重新加载文件
    await user.click(screen.getByRole('button', { name: /Clash/ }));
    await user.click(screen.getByRole('radio', { name: /YAML/ }));
    await screen.findByText('1 LINES');
    expect(screen.getByRole('link', { name: '打开文件' })).toHaveAttribute(
      'href',
      'rules/clash-yaml/geosite/v2%20fly/category%2Fai.yaml',
    );
  });

  it('opens icon sets, renders 60 at first, loads more, and encodes copy/download paths', async () => {
    const user = userEvent.setup();
    const writeText = vi.spyOn(navigator.clipboard, 'writeText');
    render(PublicApp, { data: fixture() });
    await user.click(screen.getByRole('button', { name: '图标' }));
    await user.click(screen.getByRole('button', { name: /brand set/ }));

    expect(screen.getAllByRole('article')).toHaveLength(60);
    const first = screen.getAllByRole('article')[0];
    expect(within(first).getByRole('link', { name: '下载' })).toHaveAttribute(
      'href',
      'static/icons/brand%20set/open%20ai%23.svg',
    );
    await user.click(within(first).getByRole('button', { name: '复制' }));
    expect(writeText).toHaveBeenCalledWith(expect.stringContaining('/static/icons/brand%20set/open%20ai%23.svg'));

    await user.click(screen.getByRole('button', { name: /加载更多/ }));
    expect(screen.getAllByRole('article')).toHaveLength(61);
  });
});


it('uses GeoIP catalogs and updates preview paths for the selected format', async () => {
  const data = fixture();
  data.clients[0].geoip = true;
  data.clients[0].options.forEach((option) => { option.geoip = true; });
  data.geoip = [{ provider: 'loyalsoldier', lists: [{ name: 'cn', entries: 2, variants: [], targets: ['clash-yaml', 'clash-mrs'] }] }];
  const fetchMock = vi.fn().mockImplementation(async () => new Response('IP-CIDR,1.2.3.0/24', { status: 200 }));
  vi.stubGlobal('fetch', fetchMock);
  render(PublicApp, { data });
  await fireEvent.click(screen.getByRole('button', { name: 'GeoIP' }));
  expect(screen.getByRole('heading', { name: 'GeoIP 列表' })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'cn' })).not.toHaveAttribute('aria-expanded');
  expect(screen.getByRole('link', { name: '打开' })).toHaveAttribute('href', 'rules/clash-yaml/geoip/loyalsoldier/cn.yaml');
  expect(screen.getByRole('link', { name: '打开' })).not.toHaveAttribute('download');
  await fireEvent.click(screen.getByRole('button', { name: '预览' }));
  await waitFor(() => expect(fetchMock).toHaveBeenCalledWith('rules/clash-yaml/geoip/loyalsoldier/cn.yaml', expect.anything()));
  await fireEvent.click(screen.getByRole('button', { name: /Clash/ }));
  await fireEvent.click(screen.getByRole('radio', { name: /MRS/ }));
  expect(screen.getByRole('link', { name: '打开' })).toHaveAttribute('download');
  await fireEvent.click(screen.getByRole('button', { name: '预览' }));
  expect(await screen.findByText(/二进制规则集.*无法以文本预览/)).toBeInTheDocument();
  expect(fetchMock).toHaveBeenCalledTimes(1); // 只有切换前那次 YAML 读取
});

it('explains that a binary rule artifact has no text preview', async () => {
  const user = userEvent.setup();
  const fetchMock = vi.fn().mockResolvedValue(new Response('payload', { status: 200 }));
  vi.stubGlobal('fetch', fetchMock);
  localStorage.setItem('prm-target-clash', 'clash-mrs');
  render(PublicApp, { data: fixture() });

  await user.click(await screen.findByRole('button', { name: '查看规则 OpenAI' }));
  expect(await screen.findByText(/二进制规则集.*无法以文本预览/)).toBeInTheDocument();
  expect(fetchMock).not.toHaveBeenCalled();
  expect(screen.getByRole('link', { name: '下载文件' })).toHaveAttribute('href', 'rules/clash-mrs/OpenAI.mrs');
  expect(screen.getByRole('link', { name: '下载文件' })).toHaveAttribute('download');
});

it('hides unpublished GeoIP lists and dims formats without files', async () => {
  const data = fixture();
  data.clients[0].geoip = true;
  data.clients[0].options[0].geoip = true;
  data.clients[0].options[1].geoip = false;
  data.clients[1].geoip = false;
  data.geoip = [{
    provider: 'loyalsoldier',
    lists: [
      { name: 'cn', entries: 2, variants: [], targets: ['clash-yaml'] },
      { name: 'private', entries: 1, variants: [], targets: ['clash-mrs'] },
    ],
  }];
  render(PublicApp, { data });
  await fireEvent.click(screen.getByRole('button', { name: 'GeoIP' }));
  expect(screen.getByText('cn')).toBeInTheDocument();
  expect(screen.queryByText('private')).not.toBeInTheDocument();
  await fireEvent.click(screen.getByRole('button', { name: /Clash/ }));
  expect(screen.getByRole('radio', { name: /MRS/ })).toBeDisabled();
  expect(screen.getByRole('button', { name: /Surge/ })).toBeDisabled();
});
