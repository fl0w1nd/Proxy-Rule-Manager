import { OverlayScrollbars, type PartialOptions } from 'overlayscrollbars';

export const RETRO_SCROLLBAR_OPTIONS: PartialOptions = {
  scrollbars: {
    theme: 'os-theme-retro',
    visibility: 'auto',
    autoHide: 'never',
    clickScroll: true,
  },
};

export function setupArrowClicks(instance: OverlayScrollbars): () => void {
  const elements = instance.elements();
  const sbV = elements.scrollbarVertical?.scrollbar;
  const sbH = elements.scrollbarHorizontal?.scrollbar;
  const target = elements.scrollOffsetElement;

  const scroll = (dx: number, dy: number) => {
    if (target && typeof target.scrollBy === 'function') {
      target.scrollBy({ left: dx, top: dy, behavior: 'smooth' });
    } else if (typeof window !== 'undefined' && typeof window.scrollBy === 'function') {
      window.scrollBy({ left: dx, top: dy, behavior: 'smooth' });
    }
  };

  const cleanups: Array<() => void> = [];

  if (sbV) {
    const onPointerDown = (e: PointerEvent) => {
      const rect = sbV.getBoundingClientRect();
      const y = e.clientY - rect.top;
      if (y <= 16) {
        e.preventDefault();
        e.stopPropagation();
        scroll(0, -40);
      } else if (y >= rect.height - 16) {
        e.preventDefault();
        e.stopPropagation();
        scroll(0, 40);
      }
    };
    sbV.addEventListener('pointerdown', onPointerDown);
    cleanups.push(() => sbV.removeEventListener('pointerdown', onPointerDown));
  }

  if (sbH) {
    const onPointerDown = (e: PointerEvent) => {
      const rect = sbH.getBoundingClientRect();
      const x = e.clientX - rect.left;
      if (x <= 16) {
        e.preventDefault();
        e.stopPropagation();
        scroll(-40, 0);
      } else if (x >= rect.width - 16) {
        e.preventDefault();
        e.stopPropagation();
        scroll(40, 0);
      }
    };
    sbH.addEventListener('pointerdown', onPointerDown);
    cleanups.push(() => sbH.removeEventListener('pointerdown', onPointerDown));
  }

  return () => {
    cleanups.forEach((fn) => fn());
  };
}

export function retroScroll(node: HTMLElement, options?: PartialOptions) {
  const instance = OverlayScrollbars(node, {
    ...RETRO_SCROLLBAR_OPTIONS,
    ...options,
  });
  const cleanupArrows = setupArrowClicks(instance);

  return {
    destroy() {
      cleanupArrows();
      instance.destroy();
    },
  };
}

export function initRetroBodyScroll(): (() => void) | undefined {
  if (typeof document === 'undefined' || !document.body) return;
  document.documentElement.setAttribute('data-overlayscrollbars-initialize', '');
  document.body.setAttribute('data-overlayscrollbars-initialize', '');

  const instance = OverlayScrollbars(document.body, RETRO_SCROLLBAR_OPTIONS);
  const cleanupArrows = setupArrowClicks(instance);

  return () => {
    cleanupArrows();
    instance.destroy();
  };
}
