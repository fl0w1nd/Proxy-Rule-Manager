import { describe, it, expect } from 'vitest';
import { initRetroBodyScroll, retroScroll } from './scrollbars';

describe('scrollbars', () => {
  it('initializes retro scroll on body without errors', () => {
    const cleanup = initRetroBodyScroll();
    expect(document.documentElement.hasAttribute('data-overlayscrollbars-initialize')).toBe(true);
    cleanup?.();
  });

  it('attaches to custom container without errors', () => {
    const div = document.createElement('div');
    document.body.appendChild(div);
    const action = retroScroll(div);
    expect(action).toBeDefined();
    action.destroy();
    div.remove();
  });
});
