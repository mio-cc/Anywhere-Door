export interface FakeRaf {
  raf: (cb: FrameRequestCallback) => number;
  caf: (h: number) => void;
  step(n?: number): void;
  pending(): number;
}

export function fakeRaf(): FakeRaf {
  let nextId = 1;
  let now = 0;
  const queue = new Map<number, FrameRequestCallback>();

  return {
    raf(cb: FrameRequestCallback): number {
      const id = nextId;
      nextId += 1;
      queue.set(id, cb);
      return id;
    },
    caf(h: number): void {
      queue.delete(h);
    },
    step(n = 1): void {
      for (let i = 0; i < n; i += 1) {
        now += 16;
        const batch = Array.from(queue.entries());
        queue.clear();
        for (const [, cb] of batch) {
          cb(now);
        }
      }
    },
    pending(): number {
      return queue.size;
    },
  };
}
