export interface AnimateCall {
  readonly el: Element;
  readonly keyframes: Keyframe[];
  readonly options: KeyframeAnimationOptions;
}

export interface Canvas2dCall {
  readonly fn: string;
  readonly args: unknown[];
}

type ElementFromPointFn = (x: number, y: number) => Element | null;
type MeasureTextFn = (text: string) => TextMetrics;

let installed = false;
let elementFromPointImpl: ElementFromPointFn = () => null;
let animateRecorder: AnimateCall[] | null = null;
const pendingAnimations: Array<() => void> = [];
let current2dCalls: Canvas2dCall[] = [];
let currentMeasureText: MeasureTextFn = () => ({ width: 0 }) as TextMetrics;
let clipboardText = '';

const resizeObservers = new Set<{
  observer: ResizeObserver;
  cb: ResizeObserverCallback;
  targets: Set<Element>;
}>();

const capturedPointers = new WeakMap<Element, Set<number>>();

function makeFakeAnimation(onFinish?: () => void): Animation {
  let resolveFinished: ((a: Animation) => void) | undefined;
  const finished = new Promise<Animation>((resolve) => {
    resolveFinished = resolve;
  });
  let playState: AnimationPlayState = 'finished';
  const anim = {
    get playState() {
      return playState;
    },
    finished,
    onfinish: null as ((this: Animation, ev: AnimationPlaybackEvent) => unknown) | null,
    cancel() {
      playState = 'idle';
    },
    finish() {
      playState = 'finished';
      if (resolveFinished) resolveFinished(anim);
      if (typeof anim.onfinish === 'function') {
        anim.onfinish.call(anim, new Event('finish') as AnimationPlaybackEvent);
      }
      onFinish?.();
    },
    play() {
      playState = 'running';
    },
    pause() {
      playState = 'paused';
    },
  } as unknown as Animation;
  if (animateRecorder) {
    playState = 'running';
    pendingAnimations.push(() => {
      anim.finish();
    });
  } else {
    resolveFinished?.(anim);
  }
  return anim;
}

function makeFake2dContext(calls: Canvas2dCall[], measureTextFn: MeasureTextFn): CanvasRenderingContext2D {
  const state: Record<string, unknown> = {
    fillStyle: '#000000',
    strokeStyle: '#000000',
    lineWidth: 1,
    globalAlpha: 1,
    font: '12px sans-serif',
    textAlign: 'start',
    textBaseline: 'alphabetic',
  };
  return new Proxy(state, {
    get(target, prop) {
      if (typeof prop === 'string' && prop in target) {
        return target[prop];
      }
      if (prop === 'measureText') {
        return (text: string) => {
          calls.push({ fn: 'measureText', args: [text] });
          return measureTextFn(text);
        };
      }
      if (typeof prop === 'string') {
        return (...args: unknown[]) => {
          calls.push({ fn: prop, args });
        };
      }
      return undefined;
    },
    set(target, prop, value: unknown) {
      if (typeof prop === 'string') {
        target[prop] = value;
      }
      return true;
    },
  }) as unknown as CanvasRenderingContext2D;
}

function createStubPointerEventClass(): typeof PointerEvent {
  return class StubPointerEvent extends MouseEvent {
    readonly pointerId: number;
    readonly pointerType: string;
    readonly isPrimary: boolean;
    readonly width: number;
    readonly height: number;
    readonly pressure: number;

    constructor(type: string, init: PointerEventInit = {}) {
      super(type, init);
      this.pointerId = init.pointerId ?? 1;
      this.pointerType = init.pointerType ?? 'mouse';
      this.isPrimary = init.isPrimary ?? true;
      this.width = init.width ?? 1;
      this.height = init.height ?? 1;
      this.pressure = init.pressure ?? 0;
    }
  } as unknown as typeof PointerEvent;
}

class StubResizeObserver implements ResizeObserver {
  private readonly entry: { observer: ResizeObserver; cb: ResizeObserverCallback; targets: Set<Element> };

  constructor(cb: ResizeObserverCallback) {
    this.entry = { observer: this, cb, targets: new Set() };
    resizeObservers.add(this.entry);
  }

  observe(target: Element): void {
    this.entry.targets.add(target);
  }

  unobserve(target: Element): void {
    this.entry.targets.delete(target);
  }

  disconnect(): void {
    this.entry.targets.clear();
    resizeObservers.delete(this.entry);
  }
}

class StubIntersectionObserver implements IntersectionObserver {
  readonly root: Element | Document | null = null;
  readonly rootMargin = '0px';
  readonly scrollMargin = '0px';
  readonly thresholds: readonly number[] = [0];

  observe(): void {
    // 空实现
  }
  unobserve(): void {
    // 空实现
  }
  disconnect(): void {
    // 空实现
  }
  takeRecords(): IntersectionObserverEntry[] {
    return [];
  }
}

export function installDomStubs(): void {
  if (installed || typeof window === 'undefined' || typeof document === 'undefined') return;
  installed = true;

  if (typeof window.PointerEvent === 'undefined') {
    const PointerEventCtor = createStubPointerEventClass();
    Object.defineProperty(window, 'PointerEvent', {
      configurable: true,
      writable: true,
      value: PointerEventCtor,
    });
    Object.defineProperty(globalThis, 'PointerEvent', {
      configurable: true,
      writable: true,
      value: PointerEventCtor,
    });
  }

  Object.defineProperty(Element.prototype, 'setPointerCapture', {
    configurable: true,
    writable: true,
    value(this: Element, pointerId: number) {
      let set = capturedPointers.get(this);
      if (!set) {
        set = new Set();
        capturedPointers.set(this, set);
      }
      set.add(pointerId);
    },
  });

  Object.defineProperty(Element.prototype, 'releasePointerCapture', {
    configurable: true,
    writable: true,
    value(this: Element, pointerId: number) {
      capturedPointers.get(this)?.delete(pointerId);
    },
  });

  Object.defineProperty(Element.prototype, 'hasPointerCapture', {
    configurable: true,
    writable: true,
    value(this: Element, pointerId: number) {
      return capturedPointers.get(this)?.has(pointerId) ?? false;
    },
  });

  Object.defineProperty(globalThis, 'ResizeObserver', {
    configurable: true,
    writable: true,
    value: StubResizeObserver,
  });

  Object.defineProperty(globalThis, 'IntersectionObserver', {
    configurable: true,
    writable: true,
    value: StubIntersectionObserver,
  });

  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    writable: true,
    value: (query: string): MediaQueryList =>
      ({
        matches: false,
        media: query,
        onchange: null,
        addEventListener() {
          // 空实现
        },
        removeEventListener() {
          // 空实现
        },
        dispatchEvent() {
          return false;
        },
      }) as unknown as MediaQueryList,
  });

  Object.defineProperty(document, 'elementFromPoint', {
    configurable: true,
    writable: true,
    value: (x: number, y: number): Element | null => elementFromPointImpl(x, y),
  });

  Object.defineProperty(Element.prototype, 'animate', {
    configurable: true,
    writable: true,
    value(
      this: Element,
      keyframes: Keyframe[] | PropertyIndexedKeyframes | null,
      options?: number | KeyframeAnimationOptions,
    ): Animation {
      const kfList = Array.isArray(keyframes) ? keyframes : [];
      const opts: KeyframeAnimationOptions =
        typeof options === 'number' ? { duration: options } : (options ?? {});
      if (animateRecorder) {
        animateRecorder.push({ el: this, keyframes: kfList, options: opts });
      }
      return makeFakeAnimation();
    },
  });

  Object.defineProperty(HTMLCanvasElement.prototype, 'getContext', {
    configurable: true,
    writable: true,
    value(contextId: string): RenderingContext | null {
      if (contextId === '2d') {
        return makeFake2dContext(current2dCalls, currentMeasureText);
      }
      return null;
    },
  });

  Object.defineProperty(navigator, 'clipboard', {
    configurable: true,
    value: {
      writeText(text: string): Promise<void> {
        clipboardText = text;
        return Promise.resolve();
      },
      readText(): Promise<string> {
        return Promise.resolve(clipboardText);
      },
    },
  });
}

export function stubElementFromPoint(fn: ElementFromPointFn): void {
  elementFromPointImpl = fn;
}

export function recordAnimate(): { readonly calls: readonly AnimateCall[]; finishAll(): void } {
  const calls: AnimateCall[] = [];
  animateRecorder = calls;
  return {
    calls,
    finishAll() {
      const list = pendingAnimations.splice(0, pendingAnimations.length);
      for (const fn of list) fn();
    },
  };
}

export function fake2d(o?: { measureText?: (text: string) => Partial<TextMetrics> }): {
  context: CanvasRenderingContext2D;
  readonly calls: readonly Canvas2dCall[];
} {
  const calls: Canvas2dCall[] = [];
  current2dCalls = calls;
  currentMeasureText = (text: string) =>
    ({ width: 0, ...(o?.measureText ? o.measureText(text) : {}) }) as TextMetrics;
  return { context: makeFake2dContext(calls, currentMeasureText), calls };
}

export function resize(el: Element, rect: Partial<DOMRectReadOnly>): void {
  const contentRect = {
    x: rect.x ?? 0,
    y: rect.y ?? 0,
    width: rect.width ?? 0,
    height: rect.height ?? 0,
    top: rect.top ?? rect.y ?? 0,
    left: rect.left ?? rect.x ?? 0,
    right: rect.right ?? (rect.x ?? 0) + (rect.width ?? 0),
    bottom: rect.bottom ?? (rect.y ?? 0) + (rect.height ?? 0),
    toJSON() {
      return this;
    },
  } as DOMRectReadOnly;

  for (const obs of Array.from(resizeObservers)) {
    if (obs.targets.has(el)) {
      const entry = {
        target: el,
        contentRect,
        borderBoxSize: [],
        contentBoxSize: [],
        devicePixelContentBoxSize: [],
      } as ResizeObserverEntry;
      obs.cb([entry], obs.observer);
    }
  }
}

export function resetDomStubs(): void {
  elementFromPointImpl = () => null;
  animateRecorder = null;
  pendingAnimations.length = 0;
  current2dCalls = [];
  currentMeasureText = () => ({ width: 0 }) as TextMetrics;
  clipboardText = '';
  resizeObservers.clear();
}
