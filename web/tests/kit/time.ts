import { act } from 'react';
import { vi } from 'vitest';
import { setClock } from '@/lib/i18n';

export interface FakeTime {
  advance(ms: number): Promise<void>;
  now(): number;
}

export function useFakeTime(o?: { now?: string }): FakeTime {
  vi.useFakeTimers();
  const start = o?.now !== undefined ? Date.parse(o.now) : Date.parse('2026-01-15T10:00:00+08:00');
  vi.setSystemTime(start);
  setClock({ now: () => Date.now() });
  return {
    async advance(ms: number): Promise<void> {
      await act(async () => {
        await vi.advanceTimersByTimeAsync(ms);
      });
    },
    now(): number {
      return Date.now();
    },
  };
}

export function resetFakeTime(): void {
  setClock(null);
  // 无论测试经 useFakeTime 还是直接调用 vi.useFakeTimers，统一恢复真实定时器（W.12 §6.2、K302）
  if (vi.isFakeTimers()) {
    vi.useRealTimers();
  }
}
