import { useState } from 'react';
import { describe, expect, test, vi } from 'vitest';
import {
  expectConsole,
  fake2d,
  fakeRaf,
  loadExample,
  recordAnimate,
  render,
  resize,
  stubElementFromPoint,
  useFakeTime,
  user,
} from '@tests/kit';
import { verifyConsoleExpectations } from './console';

describe('W.12 kit 核心工具', () => {
  test('W12-T1 render 与角色查询', () => {
    const view = render(
      <div>
        <button type="button" aria-label="运行图">
          go
        </button>
        <div aria-hidden="true">
          <button type="button" aria-label="被祖先隐藏">
            hidden
          </button>
        </div>
        <span id="lbl-ref">引用标题</span>
        <button type="button" aria-labelledby="lbl-ref" aria-label="次级标签">
          x
        </button>
        <label htmlFor="graph-name">图名称</label>
        <input id="graph-name" type="text" />
        <input type="hidden" aria-label="隐藏域" />
        <input type="password" aria-label="密码框" />
      </div>,
    );
    const btn = view.getByRole('button', { name: '运行图' });
    expect(btn.tagName).toBe('BUTTON');
    expect(view.queryAllByRole('button', { name: '被祖先隐藏' })).toEqual([]);
    // aria-labelledby 优先于 aria-label
    expect(view.getByRole('button', { name: '引用标题' })).toBeTruthy();
    expect(view.queryAllByRole('button', { name: '次级标签' })).toEqual([]);
    // label 关联的 input 与 hidden/password 不认作 textbox
    expect(view.getByRole('textbox', { name: '图名称' }).id).toBe('graph-name');
    expect(view.queryAllByRole('textbox')).toHaveLength(1);

    view.rerender(
      <div>
        <button type="button" aria-label="重复">
          a
        </button>
        <button type="button" aria-label="重复">
          b
        </button>
      </div>,
    );
    expect(() => view.getByRole('button', { name: '重复' })).toThrow();
    expect(view.queryAllByRole('button', { name: '重复' })).toHaveLength(2);

    // loadExample 返回经 fromWire('Graph', ...) 映射的 camelCase Graph
    const example = loadExample('graph_minimal');
    expect(typeof example.schemaVersion).toBe('string');
    expect('schema_version' in (example as unknown as Record<string, unknown>)).toBe(false);

    view.unmount();
    expect(view.container.innerHTML).toBe('');
  });

  test('W12-T2 user 事件派发', async () => {
    const events: string[] = [];
    const changes: string[] = [];
    function ControlledForm() {
      const [val, setVal] = useState('');
      return (
        <div>
          <button
            type="button"
            onPointerDown={() => events.push('pointerdown')}
            onMouseDown={() => events.push('mousedown')}
            onPointerUp={() => events.push('pointerup')}
            onMouseUp={() => events.push('mouseup')}
            onClick={() => events.push('click')}
          >
            btn
          </button>
          <input
            aria-label="inp"
            value={val}
            onChange={(e) => {
              changes.push(e.target.value);
              setVal(e.target.value);
            }}
          />
          <button type="button">next-btn</button>
        </div>
      );
    }

    const view = render(<ControlledForm />);
    await user.click(view.getByRole('button', { name: 'btn' }));
    expect(events).toEqual(['pointerdown', 'mousedown', 'pointerup', 'mouseup', 'click']);

    const inp = view.getByRole('textbox', { name: 'inp' }) as HTMLInputElement;
    const keyLog: string[] = [];
    inp.addEventListener('keydown', (e) => keyLog.push(`down:${e.key}`));
    inp.addEventListener('keyup', (e) => keyLog.push(`up:${e.key}`));
    await user.type(inp, 'ab{Enter}');
    expect(inp.value).toBe('ab');
    expect(changes).toEqual(['a', 'ab']);
    expect(keyLog).toEqual(['down:a', 'up:a', 'down:b', 'up:b', 'down:Enter', 'up:Enter']);

    // Tab 派发 keydown/keyup 并尊重 preventDefault（焦点陷阱场景）
    const nextBtn = view.getByRole('button', { name: 'next-btn' });
    const tabEvents: string[] = [];
    const blockOnce = (e: KeyboardEvent) => {
      if (e.key === 'Tab') {
        tabEvents.push('blocked-keydown');
        e.preventDefault();
      }
    };
    inp.addEventListener('keydown', blockOnce);
    inp.focus();
    await user.tab();
    expect(tabEvents).toEqual(['blocked-keydown']);
    expect(document.activeElement).toBe(inp);
    inp.removeEventListener('keydown', blockOnce);

    await user.tab();
    expect(document.activeElement).toBe(nextBtn);

    // 使用 vi.spyOn 模拟 navigator.platform，避免污染实例自有属性
    const modHits: Array<{ ctrl: boolean; meta: boolean }> = [];
    inp.addEventListener('keydown', (e) => modHits.push({ ctrl: e.ctrlKey, meta: e.metaKey }));

    const spy = vi.spyOn(navigator, 'platform', 'get');
    spy.mockReturnValue('MacIntel');
    await user.key('{Mod+K}', inp);
    spy.mockReturnValue('Win32');
    await user.key('{Mod+K}', inp);
    spy.mockRestore();

    expect(modHits).toEqual([
      { ctrl: false, meta: true },
      { ctrl: true, meta: false },
    ]);
  });

  test('W12-T3 时间推进与恢复', async () => {
    const ft = useFakeTime({ now: '2026-01-15T10:00:00+08:00' });
    let ticks = 0;
    const id = setInterval(() => {
      ticks += 1;
    }, 60_000);
    await ft.advance(60_000);
    clearInterval(id);
    expect(ticks).toBe(1);
    expect(ft.now()).toBe(Date.parse('2026-01-15T10:01:00+08:00'));
  });

  test('W12-T4 fakeRaf', () => {
    const raf = fakeRaf();
    const called: number[] = [];
    raf.raf((t) => called.push(t));
    raf.raf((t) => called.push(t));
    raf.raf((t) => called.push(t));
    expect(raf.pending()).toBe(3);
    raf.step();
    expect(called).toHaveLength(3);
    expect(raf.pending()).toBe(0);
  });

  test('W12-T9 浏览器替身', async () => {
    const div = document.createElement('div');
    document.body.appendChild(div);

    // PointerEvent & capture
    div.setPointerCapture(7);
    expect(div.hasPointerCapture(7)).toBe(true);
    div.releasePointerCapture(7);
    expect(div.hasPointerCapture(7)).toBe(false);

    // ResizeObserver 回调第二参数是真实 observer，可合法调用 unobserve/disconnect
    let observedCount = 0;
    let observedWidth = 0;
    const ro = new ResizeObserver((entries, observer) => {
      observedCount += 1;
      observedWidth = entries[0]?.contentRect.width ?? 0;
      observer.unobserve(div);
    });
    ro.observe(div);
    resize(div, { width: 320, height: 180 });
    resize(div, { width: 640, height: 180 });
    expect(observedCount).toBe(1);
    expect(observedWidth).toBe(320);
    ro.disconnect();

    // IntersectionObserver & matchMedia
    const io = new IntersectionObserver(() => {});
    io.observe(div);
    io.disconnect();
    expect(window.matchMedia('(prefers-reduced-motion: reduce)').matches).toBe(false);

    // elementFromPoint
    stubElementFromPoint((x, y) => (x === 10 && y === 20 ? div : null));
    expect(document.elementFromPoint(10, 20)).toBe(div);
    expect(document.elementFromPoint(0, 0)).toBeNull();

    // animate
    const rec = recordAnimate();
    const anim = div.animate([{ opacity: 0 }, { opacity: 1 }], { duration: 120 });
    expect(rec.calls).toHaveLength(1);
    rec.finishAll();
    await anim.finished;

    // canvas 2d：默认不猜宽度（0），测试显式给定时返回给定值
    const default2d = fake2d();
    const canvas = document.createElement('canvas');
    const ctx0 = canvas.getContext('2d');
    expect(ctx0?.measureText('hello').width).toBe(0);
    expect(default2d.calls.some((c) => c.fn === 'measureText')).toBe(true);

    const custom2d = fake2d({ measureText: (text) => ({ width: text === 'abc' ? 42 : 0 }) });
    const ctx = canvas.getContext('2d');
    expect(ctx).not.toBeNull();
    ctx?.fillRect(0, 0, 10, 10);
    expect(ctx?.measureText('abc').width).toBe(42);
    expect(custom2d.calls.some((c) => c.fn === 'fillRect')).toBe(true);

    // clipboard
    await navigator.clipboard.writeText('hello');
    expect(await navigator.clipboard.readText()).toBe('hello');
  });

  test('W12-T10 控制台从严', () => {
    console.error('unexpected boom');
    const unexpectedErrs = verifyConsoleExpectations();
    expect(unexpectedErrs).toHaveLength(1);
    expect(unexpectedErrs[0]).toContain('unexpected boom');

    expectConsole('error', 'expected boom', 1);
    console.error('expected boom');
    expect(verifyConsoleExpectations()).toEqual([]);

    expectConsole('warn', 'never happened', 1);
    const missingErrs = verifyConsoleExpectations();
    expect(missingErrs).toHaveLength(1);
    expect(missingErrs[0]).toContain('never happened');
  });

  test('W12-T11 网络隔离', () => {
    expect(() => fetch('https://example.com')).toThrow(/禁止访问真实网络/);
    expect(() => new EventSource('https://example.com/sse')).toThrow(/禁止访问真实网络/);
    expect(() => new WebSocket('wss://example.com/ws')).toThrow(/禁止访问真实网络/);
  });
});
