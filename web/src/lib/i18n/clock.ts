import { useSyncExternalStore } from 'react';

export interface ClockSource {
  now(): number;
  frozen?: boolean;
}

let clockSrc: ClockSource | null = null;
let currentNow = Date.now();
let timer: ReturnType<typeof setTimeout> | null = null;
let visBound = false;
const listeners = new Set<() => void>();

export function clockNow(): number {
  return clockSrc ? clockSrc.now() : Date.now();
}

function isHidden(): boolean {
  return typeof document !== 'undefined' && document.hidden;
}

function clearMinuteTimer(): void {
  if (timer !== null) {
    clearTimeout(timer);
    timer = null;
  }
}

function notifyAll(): void {
  for (const fn of listeners) {
    fn();
  }
}

function scheduleNext(): void {
  clearMinuteTimer();
  if (listeners.size === 0 || isHidden() || clockSrc?.frozen) return;
  const now = clockNow();
  const msIntoMinute = ((now % 60_000) + 60_000) % 60_000;
  const delay = 60_000 - msIntoMinute + 50;
  timer = setTimeout(() => {
    timer = null;
    if (listeners.size === 0 || isHidden() || clockSrc?.frozen) return;
    currentNow = clockNow();
    notifyAll();
    scheduleNext();
  }, delay);
}

function onVisibilityChange(): void {
  if (isHidden()) {
    clearMinuteTimer();
    return;
  }
  if (listeners.size > 0 && !clockSrc?.frozen) {
    currentNow = clockNow();
    notifyAll();
    scheduleNext();
  }
}

function bindVisibility(): void {
  if (visBound || typeof document === 'undefined') return;
  visBound = true;
  document.addEventListener('visibilitychange', onVisibilityChange);
}

function unbindVisibility(): void {
  if (!visBound || typeof document === 'undefined') return;
  visBound = false;
  document.removeEventListener('visibilitychange', onVisibilityChange);
}

function subscribe(cb: () => void): () => void {
  const wasEmpty = listeners.size === 0;
  listeners.add(cb);
  if (wasEmpty) {
    currentNow = clockNow();
    bindVisibility();
    scheduleNext();
  }
  return () => {
    listeners.delete(cb);
    if (listeners.size === 0) {
      clearMinuteTimer();
      unbindVisibility();
    }
  };
}

function getSnapshot(): number {
  return currentNow;
}

export function useNow(): number {
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}

export function setClock(src: ClockSource | null): void {
  clockSrc = src;
  currentNow = clockNow();
  clearMinuteTimer();
  if (listeners.size > 0) {
    notifyAll();
    scheduleNext();
  }
}
