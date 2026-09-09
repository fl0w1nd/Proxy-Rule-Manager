import { describe, it, expect, beforeEach } from 'vitest';
import { lockScroll } from './scrollLock';

describe('scrollLock', () => {
  beforeEach(() => {
    document.documentElement.className = '';
    document.documentElement.style.overflow = '';
    document.documentElement.style.overscrollBehavior = '';
    document.body.className = '';
    document.body.style.overflow = '';
    document.body.style.overscrollBehavior = '';
  });

  it('locks html and body overflow on lockScroll and restores on unlock', () => {
    const unlock = lockScroll();
    expect(document.documentElement.style.overflow).toBe('hidden');
    expect(document.documentElement.style.overscrollBehavior).toBe('none');
    expect(document.body.style.overflow).toBe('hidden');
    expect(document.body.style.overscrollBehavior).toBe('none');
    expect(document.documentElement.classList.contains('scroll-locked')).toBe(true);
    expect(document.body.classList.contains('scroll-locked')).toBe(true);

    unlock();
    expect(document.documentElement.style.overflow).toBe('');
    expect(document.documentElement.style.overscrollBehavior).toBe('');
    expect(document.body.style.overflow).toBe('');
    expect(document.body.style.overscrollBehavior).toBe('');
    expect(document.documentElement.classList.contains('scroll-locked')).toBe(false);
    expect(document.body.classList.contains('scroll-locked')).toBe(false);
  });

  it('supports nested locks with reference counting', () => {
    const unlock1 = lockScroll();
    const unlock2 = lockScroll();

    expect(document.body.classList.contains('scroll-locked')).toBe(true);

    unlock1();
    // Still locked because unlock2 is active
    expect(document.body.classList.contains('scroll-locked')).toBe(true);

    unlock2();
    // Now fully unlocked
    expect(document.body.classList.contains('scroll-locked')).toBe(false);
  });

  it('intercepts wheel events occurring outside active modal panel', () => {
    const modal = document.createElement('div');
    document.body.appendChild(modal);
    modal.getBoundingClientRect = () => ({
      left: 100,
      right: 500,
      top: 50,
      bottom: 600,
      width: 400,
      height: 550,
      x: 100,
      y: 50,
      toJSON: () => {},
    });

    const unlock = lockScroll(modal);

    // Event outside modal panel (e.g. clientX: 20, clientY: 20)
    const outsideEvent = new MouseEvent('wheel', {
      bubbles: true,
      cancelable: true,
      clientX: 20,
      clientY: 20,
    });
    window.dispatchEvent(outsideEvent);
    expect(outsideEvent.defaultPrevented).toBe(true);

    // Event inside modal panel (e.g. clientX: 200, clientY: 200)
    const insideEvent = new MouseEvent('wheel', {
      bubbles: true,
      cancelable: true,
      clientX: 200,
      clientY: 200,
    });
    window.dispatchEvent(insideEvent);
    expect(insideEvent.defaultPrevented).toBe(false);

    unlock();
    modal.remove();
  });
});
