import { describe, expect, test, vi } from 'vitest';
import { memStorage } from '@tests/kit';
import type { Ulid } from '@/lib/protocol';
import { attachable, canvasReadonly, createUiStore, rsDot, selectionOf } from './uiStore';

function uid(n: number): Ulid {
  return `01JD8X3Q5V7W9Y2Z4A6B8C${String(n).padStart(4, '0')}` as Ulid;
}

describe('W.03.03 uiStore', () => {
  test('W0303-T1 缺省值：空存储按宽度决定左栏开合，已有记录优先', () => {
    const s1 = createUiStore({ width: 1440, storage: memStorage() });
    expect(s1.getState().layout.lsOpen).toBe(true);
    expect(s1.getState().layout.rsOpen).toBe(false);
    expect(s1.getState().layout.rsTab).toBe('insp');
    expect(s1.getState().layout.lsFold).toEqual({ sess: false, tree: false, lib: true });

    const s2 = createUiStore({ width: 800, storage: memStorage() });
    expect(s2.getState().layout.lsOpen).toBe(false);

    const st3 = memStorage();
    st3.setItem('hx:ls-open', '1');
    const s3 = createUiStore({ width: 800, storage: st3 });
    expect(s3.getState().layout.lsOpen).toBe(true);
  });

  test('W0303-T2 非法值：回退缺省值且不改写原存储值', () => {
    const storage = memStorage();
    storage.setItem('hx:rs-tab', 'xyz');
    storage.setItem('hx:ls-fold', '{');
    storage.setItem('hx:mon', 'bad-json');

    const ui = createUiStore({ width: 1280, storage });
    expect(ui.getState().layout.rsTab).toBe('insp');
    expect(ui.getState().layout.lsFold).toEqual({ sess: false, tree: false, lib: true });
    expect(ui.getState().monitor).toEqual({ range: 'session', status: 'all' });

    // 原非法值未被覆盖
    expect(storage.getItem('hx:rs-tab')).toBe('xyz');
    expect(storage.getItem('hx:ls-fold')).toBe('{');
    expect(storage.getItem('hx:mon')).toBe('bad-json');
  });

  test('W0303-T3 专注模式：F 两次恢复原状、专注中打开左栏退出专注、刷新回到专注前', () => {
    const storage = memStorage();
    const ui = createUiStore({ width: 1440, storage });
    ui.actions.setRs(true, 'chat');
    expect(ui.getState().layout.lsOpen).toBe(true);
    expect(ui.getState().layout.rsOpen).toBe(true);

    // 第一次 toggleFocus：两侧收起，且不写 localStorage
    ui.actions.toggleFocus();
    expect(ui.getState().layout.lsOpen).toBe(false);
    expect(ui.getState().layout.rsOpen).toBe(false);
    expect(ui.getState().layout.focus).toEqual({ ls: true, rs: true });

    // 专注中刷新（新建 store）回到专注前的状态
    const reloaded = createUiStore({ width: 1440, storage });
    expect(reloaded.getState().layout.lsOpen).toBe(true);
    expect(reloaded.getState().layout.rsOpen).toBe(true);
    expect(reloaded.getState().layout.rsTab).toBe('chat');

    // 第二次 toggleFocus：恢复原状
    ui.actions.toggleFocus();
    expect(ui.getState().layout.lsOpen).toBe(true);
    expect(ui.getState().layout.rsOpen).toBe(true);
    expect(ui.getState().layout.focus).toBeNull();

    // 再次进入专注后按 [（setLs(true)）：退出专注且只打开左栏，同时把右栏关闭态写回存储（K309 D01）
    ui.actions.toggleFocus();
    ui.actions.setLs(true);
    expect(ui.getState().layout.focus).toBeNull();
    expect(ui.getState().layout.lsOpen).toBe(true);
    expect(ui.getState().layout.rsOpen).toBe(false);
    const afterExit = createUiStore({ width: 1440, storage });
    expect(afterExit.getState().layout.lsOpen).toBe(true);
    expect(afterExit.getState().layout.rsOpen).toBe(false);
  });

  test('W0303-T4 选中按会话分开：切回恢复并支持 pruneSelection', () => {
    const ui = createUiStore({ width: 1440, storage: memStorage() });
    const sessA = uid(1);
    const sessB = uid(2);
    const n1 = uid(10);
    const n2 = uid(11);
    const n3 = uid(12);
    const nb = uid(20);

    ui.actions.select(sessA, { scope: 'root', nodeIds: [n1, n2, n3], edgeIds: [], noteIds: [] });
    ui.actions.select(sessB, { scope: 'root', nodeIds: [nb], edgeIds: [], noteIds: [] });

    expect(selectionOf(ui.getState(), sessA)?.nodeIds).toEqual([n1, n2, n3]);
    expect(selectionOf(ui.getState(), sessB)?.nodeIds).toEqual([nb]);

    // 模拟在 B 期间 A 中的 n2 被删除
    ui.actions.pruneSelection(sessA, (_kind, id) => id !== n2);
    expect(selectionOf(ui.getState(), sessA)?.nodeIds).toEqual([n1, n3]);
    expect(selectionOf(ui.getState(), sessB)?.nodeIds).toEqual([nb]);
  });

  test('W0303-T5 跨层替换：切换到子图层 X 后原 root 层选中被替换', () => {
    const ui = createUiStore({ width: 1440, storage: memStorage() });
    const sess = uid(1);
    const scopeX = uid(99);
    ui.actions.select(sess, { scope: 'root', nodeIds: [uid(10), uid(11)], edgeIds: [], noteIds: [] });
    ui.actions.select(sess, { scope: scopeX, nodeIds: [uid(20)], edgeIds: [], noteIds: [] });

    expect(selectionOf(ui.getState(), sess)).toEqual({
      scope: scopeX,
      nodeIds: [uid(20)],
      edgeIds: [],
      noteIds: [],
    });
  });

  test('W0303-T6 相同值不 set：重复 select 与同值 setBp 不触发订阅', () => {
    const ui = createUiStore({ width: 1440, storage: memStorage() });
    const sess = uid(1);
    const listener = vi.fn();
    ui.subscribe(listener);

    const sel = { scope: 'root' as const, nodeIds: [uid(10), uid(10)], edgeIds: [], noteIds: [] };
    ui.actions.select(sess, sel);
    ui.actions.select(sess, { scope: 'root', nodeIds: [uid(10)], edgeIds: [], noteIds: [] });
    expect(listener).toHaveBeenCalledTimes(1);

    listener.mockClear();
    ui.actions.setBp('xl');
    expect(listener).toHaveBeenCalledTimes(0);

    ui.actions.setNet('online');
    expect(listener).toHaveBeenCalledTimes(0);
  });

  test('W0303-T7 小圆点：右栏收起且有选中为 true，打开或无选中为 false，选中不自动开右栏', () => {
    const ui = createUiStore({ width: 1440, storage: memStorage() });
    const sess = uid(1);

    // 无选中 + 右栏收起 → false
    expect(rsDot(ui.getState(), sess)).toBe(false);

    // 有选中 + 右栏收起 → true，且右栏仍收起（U.09.02-R1）
    ui.actions.select(sess, { scope: 'root', nodeIds: [uid(10)], edgeIds: [], noteIds: [] });
    expect(ui.getState().layout.rsOpen).toBe(false);
    expect(rsDot(ui.getState(), sess)).toBe(true);

    // 打开右栏 → false
    ui.actions.setRs(true);
    expect(rsDot(ui.getState(), sess)).toBe(false);
  });

  test('W0303-T8 附带上限：60 个截断为 50 + truncatedFrom=60；30 个不截断', () => {
    const ids60 = Array.from({ length: 60 }, (_, i) => uid(100 + i));
    const res60 = attachable({ scope: 'root', nodeIds: ids60, edgeIds: [], noteIds: [] });
    expect(res60.nodeIds).toHaveLength(50);
    expect(res60.truncatedFrom).toBe(60);

    const ids30 = ids60.slice(0, 30);
    const res30 = attachable({ scope: 'root', nodeIds: ids30, edgeIds: [], noteIds: [] });
    expect(res30.nodeIds).toHaveLength(30);
    expect('truncatedFrom' in res30).toBe(false);
  });

  test('W0303-T9 提示：同 groupKey 原地替换，第 21 条挤掉第 1 条', () => {
    const ui = createUiStore({ width: 1440, storage: memStorage() });
    const id1 = ui.actions.notify({ level: 'info', text: 't1' });
    ui.actions.notify({ level: 'model', text: 'AI 改了 1 处', groupKey: 'ai-edit' });
    ui.actions.notify({ level: 'ok', text: 't3' });
    const replacedId = ui.actions.notify({ level: 'model', text: 'AI 改了 2 处', groupKey: 'ai-edit' });

    const list1 = ui.getState().toasts;
    expect(list1).toHaveLength(3);
    expect(list1[1]?.id).toBe(replacedId);
    expect(list1[1]?.text).toBe('AI 改了 2 处');

    for (let i = 4; i <= 21; i += 1) {
      ui.actions.notify({ level: 'info', text: `t${String(i)}` });
    }
    const list2 = ui.getState().toasts;
    expect(list2).toHaveLength(20);
    expect(list2.some((x) => x.id === id1)).toBe(false);
    expect(list2[list2.length - 1]?.text).toBe('t21');
  });

  test('W0303-T10 选中不持久化：新建 store 后 selections 为空且 localStorage 无选中键', () => {
    const storage = memStorage();
    const ui1 = createUiStore({ width: 1440, storage });
    ui1.actions.select(uid(1), { scope: 'root', nodeIds: [uid(10)], edgeIds: [], noteIds: [] });

    const ui2 = createUiStore({ width: 1440, storage });
    expect(ui2.getState().selections.size).toBe(0);
    for (let i = 0; i < storage.length; i += 1) {
      const k = storage.key(i) ?? '';
      expect(k.toLowerCase()).not.toContain('sel');
    }
  });

  test('W0303-T11 只读：net 不为 online 时 canvasReadonly 为 true', () => {
    const ui = createUiStore({ width: 1440, storage: memStorage() });
    expect(canvasReadonly(ui.getState())).toBe(false);
    ui.actions.setNet('reconnecting');
    expect(canvasReadonly(ui.getState())).toBe(true);
    ui.actions.setNet('online');
    expect(canvasReadonly(ui.getState())).toBe(false);
  });

  test('W0303-T12 存储不可用：getItem/setItem 抛错仍回退缺省值且 action 照常工作', () => {
    const broken: Storage = {
      length: 0,
      clear() {
        throw new Error('blocked');
      },
      getItem() {
        throw new Error('blocked');
      },
      key() {
        return null;
      },
      removeItem() {
        throw new Error('blocked');
      },
      setItem() {
        throw new Error('quota');
      },
    };

    const ui = createUiStore({ width: 1440, storage: broken });
    expect(ui.getState().layout.lsOpen).toBe(true);
    ui.actions.setLs(false);
    expect(ui.getState().layout.lsOpen).toBe(false);
    ui.actions.setListView(true);
    expect(ui.getState().listView).toBe(true);
    ui.actions.revealInLibrary('Agent');
    expect(ui.getState().layout.lsOpen).toBe(true);
    expect(ui.getState().layout.lsFold.lib).toBe(false);
    expect(ui.getState().libQuery?.text).toBe('Agent');
  });
});
