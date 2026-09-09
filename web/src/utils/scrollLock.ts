/**
 * Modal scroll lock utility.
 * Prevents background page scrolling (including infinite wheel scroll chaining)
 * when a modal, drawer, or dialog is open.
 */

let lockCount = 0;
let prevHtmlOverflow = '';
let prevHtmlOverscroll = '';
let prevBodyOverflow = '';
let prevBodyOverscroll = '';

const activeModals = new Set<HTMLElement>();

function isInsideModal(targetModal: HTMLElement, event: MouseEvent | TouchEvent | WheelEvent): boolean {
  if (!targetModal || !targetModal.isConnected) return false;

  const clientX = 'clientX' in event ? event.clientX : (event.touches?.[0]?.clientX ?? 0);
  const clientY = 'clientY' in event ? event.clientY : (event.touches?.[0]?.clientY ?? 0);

  const rect = targetModal.getBoundingClientRect();
  const insidePanel = (
    clientX >= rect.left &&
    clientX <= rect.right &&
    clientY >= rect.top &&
    clientY <= rect.bottom
  );

  return insidePanel;
}

function handleWheel(event: WheelEvent) {
  if (activeModals.size === 0) return;

  // Check if pointer is inside any currently active modal panel
  for (const modal of activeModals) {
    if (isInsideModal(modal, event)) {
      // Inside active modal panel, allow normal scrolling within modal content
      return;
    }
  }

  // Pointer is outside all active modal panels (on backdrop or background page)
  event.preventDefault();
  event.stopPropagation();
}

function handleTouchMove(event: TouchEvent) {
  if (activeModals.size === 0) return;

  for (const modal of activeModals) {
    if (isInsideModal(modal, event)) {
      return;
    }
  }

  event.preventDefault();
  event.stopPropagation();
}

export function lockScroll(modalElement?: HTMLElement): () => void {
  if (typeof document === 'undefined') {
    return () => {};
  }

  if (modalElement) {
    activeModals.add(modalElement);
  }

  if (lockCount === 0) {
    prevHtmlOverflow = document.documentElement.style.overflow;
    prevHtmlOverscroll = document.documentElement.style.overscrollBehavior;
    prevBodyOverflow = document.body.style.overflow;
    prevBodyOverscroll = document.body.style.overscrollBehavior;

    document.documentElement.style.overflow = 'hidden';
    document.documentElement.style.overscrollBehavior = 'none';
    document.body.style.overflow = 'hidden';
    document.body.style.overscrollBehavior = 'none';

    document.documentElement.classList.add('scroll-locked');
    document.body.classList.add('scroll-locked');

    window.addEventListener('wheel', handleWheel, { passive: false, capture: true });
    window.addEventListener('touchmove', handleTouchMove, { passive: false, capture: true });
  }

  lockCount++;

  let cleaned = false;
  return () => {
    if (cleaned) return;
    cleaned = true;

    if (modalElement) {
      activeModals.delete(modalElement);
    }

    lockCount = Math.max(0, lockCount - 1);

    if (lockCount === 0) {
      document.documentElement.style.overflow = prevHtmlOverflow;
      document.documentElement.style.overscrollBehavior = prevHtmlOverscroll;
      document.body.style.overflow = prevBodyOverflow;
      document.body.style.overscrollBehavior = prevBodyOverscroll;

      document.documentElement.classList.remove('scroll-locked');
      document.body.classList.remove('scroll-locked');

      window.removeEventListener('wheel', handleWheel, { capture: true });
      window.removeEventListener('touchmove', handleTouchMove, { capture: true });
    }
  };
}
