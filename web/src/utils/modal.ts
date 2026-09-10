/**
 * Shared modal behavior for PixelDialog / PixelDrawer:
 * - showModal + scroll lock + focus restore
 * - autofocus the first interactive control on open
 * - stacked modals: only the bottom-most layer keeps its backdrop,
 *   so nested dialogs/drawers don't double-darken the page
 * - exit animation before the dialog is actually closed
 */

import { lockScroll } from './scrollLock';

export const MODAL_EXIT_MS = 120;

function reducedMotion(): boolean {
  return (
    typeof window !== 'undefined' &&
    typeof window.matchMedia === 'function' &&
    window.matchMedia('(prefers-reduced-motion: reduce)').matches
  );
}

const stack: HTMLDialogElement[] = [];

function refreshDepths() {
  stack.forEach((item, index) => {
    item.dataset.modalDepth = String(index + 1);
  });
}

const pendingExits = new WeakMap<HTMLDialogElement, { timer: number; cancel: () => void }>();

export interface ModalSession {
  finishClose: () => void;
}

export function openModal(dialog: HTMLDialogElement): ModalSession {
  const pending = pendingExits.get(dialog);
  if (pending) {
    window.clearTimeout(pending.timer);
    pending.cancel();
    pendingExits.delete(dialog);
  }

  const previousFocus = document.activeElement as HTMLElement | null;
  if (!dialog.open) {
    dialog.showModal();
  }
  if (!stack.includes(dialog)) {
    stack.push(dialog);
  }
  refreshDepths();
  const unlock = lockScroll(dialog);

  const focusTarget =
    dialog.querySelector<HTMLElement>('[data-autofocus]') ??
    dialog.querySelector<HTMLElement>('input:not(:disabled), textarea:not(:disabled), select:not(:disabled)') ??
    dialog.querySelector<HTMLElement>('button:not(:disabled)');
  focusTarget?.focus({ preventScroll: true });

  let closed = false;
  return {
    finishClose() {
      if (closed) return;
      closed = true;
      const index = stack.indexOf(dialog);
      if (index >= 0) stack.splice(index, 1);
      refreshDepths();
      unlock();
      if (dialog.isConnected && dialog.hasAttribute('open')) dialog.close();
      delete dialog.dataset.modalDepth;
      previousFocus?.focus?.({ preventScroll: true });
    },
  };
}

/** Resolved exit duration in ms; 0 when animations are disabled or unstyled (e.g. jsdom). */
function exitDuration(dialog: HTMLDialogElement): number {
  if (reducedMotion() || typeof getComputedStyle !== 'function') return 0;
  const raw = getComputedStyle(dialog).animationDuration;
  if (!raw) return 0;
  const first = raw.split(',')[0].trim();
  if (first.endsWith('ms')) {
    const ms = Number.parseFloat(first);
    return Number.isFinite(ms) && ms > 0 ? Math.min(ms, 1000) : 0;
  }
  const seconds = Number.parseFloat(first);
  return Number.isFinite(seconds) && seconds > 0 ? Math.min(seconds * 1000, 1000) : 0;
}

/** Plays the exit animation (CSS class) before running the real close. */
export function closeModalWithExit(dialog: HTMLDialogElement, exitClass: string, finish: () => void) {
  const existing = pendingExits.get(dialog);
  if (existing) {
    window.clearTimeout(existing.timer);
    existing.cancel();
    pendingExits.delete(dialog);
  }

  dialog.classList.add(exitClass);
  const ms = exitDuration(dialog);
  if (ms <= 0) {
    dialog.classList.remove(exitClass);
    finish();
    return;
  }
  const timer = window.setTimeout(() => {
    pendingExits.delete(dialog);
    dialog.classList.remove(exitClass);
    finish();
  }, ms);

  pendingExits.set(dialog, {
    timer,
    cancel() {
      dialog.classList.remove(exitClass);
    },
  });
}
