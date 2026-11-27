import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { ESLint } from 'eslint';
import { describe, expect, test, vi } from 'vitest';
import { useFakeTime } from '@tests/kit';
import type { Ulid } from '@/lib/protocol';
import { createLocationSaver, createNavHistoryTracker, parseUrl, toUrl } from './nav';

const here = dirname(fileURLToPath(import.meta.url));
const webDir = resolve(here, '..', '..', '..');

const SID = '01JD8X3Q5V7W9Y2Z4A6B8C0D1E' as Ulid;
const GID = '01JD8X3Q5V7W9Y2Z4A6B8C0D2F' as Ulid;
const SCOPE_X = '01JD8X3Q5V7W9Y2Z4A6B8C0D3G' as Ulid;
const SCOPE_Y = '01JD8X3Q5V7W9Y2Z4A6B8C0D4H' as Ulid;

describe('W.02.01 nav', () => {
  test('W0201-T1 toUrl / parseUrl 往返：root、子层与非法 ULID 返回 null', () => {
    const locRoot = { sessionId: SID, graphId: GID, scope: 'root' as const };
    const urlRoot = toUrl(locRoot);
    expect(urlRoot).toBe(`/c/${SID}/g/${GID}`);
    expect(parseUrl(new URL(urlRoot, 'http://localhost'))).toEqual(locRoot);

    const locSub = { sessionId: SID, graphId: GID, scope: SCOPE_X };
    const urlSub = toUrl(locSub);
    expect(urlSub).toBe(`/c/${SID}/g/${GID}?scope=${SCOPE_X}`);
    expect(parseUrl(new URL(urlSub, 'http://localhost'))).toEqual(locSub);

    // 只有 sessionId
    expect(parseUrl(new URL(`/c/${SID}`, 'http://localhost'))).toEqual({ sessionId: SID });

    // 非法 ULID、空路径段、尾斜杠、空 scope、重复 scope 全部返回 null（K309 D05）
    expect(parseUrl(new URL('/c/not-a-ulid', 'http://localhost'))).toBeNull();
    expect(parseUrl(new URL(`/c/${SID}/g/short`, 'http://localhost'))).toBeNull();
    expect(parseUrl(new URL(`/c/${SID}/g/${GID}?scope=bad_scope`, 'http://localhost'))).toBeNull();
    expect(parseUrl(new URL(`//c/${SID}`, 'http://localhost'))).toBeNull();
    expect(parseUrl(new URL(`/c/${SID}/`, 'http://localhost'))).toBeNull();
    expect(parseUrl(new URL(`/c/${SID}/g/${GID}?scope=`, 'http://localhost'))).toBeNull();
    expect(
      parseUrl(new URL(`/c/${SID}/g/${GID}?scope=${SCOPE_X}&scope=${SCOPE_Y}`, 'http://localhost')),
    ).toBeNull();

    // toUrl 运行时校验非法 ULID / scope 抛 RangeError
    expect(() => toUrl({ sessionId: 'bad' as Ulid, graphId: GID, scope: 'root' })).toThrow(RangeError);
    expect(() => toUrl({ sessionId: SID, graphId: GID, scope: 'bad' as Ulid })).toThrow(RangeError);
  });

  test('W0201-T6 last_location 节流：1 秒内 5 次层变化只发 1 次 PATCH，内容是最后一次', async () => {
    const clock = useFakeTime();
    const patch = vi.fn();
    const saver = createLocationSaver(patch, 1000);

    try {
      saver.save(SID, { graphId: GID, scope: 'root' });
      await clock.advance(150);
      saver.save(SID, { graphId: GID, scope: SCOPE_X });
      await clock.advance(150);
      saver.save(SID, { graphId: GID, scope: 'root' });
      await clock.advance(150);
      saver.save(SID, { graphId: GID, scope: SCOPE_X });
      await clock.advance(150);
      saver.save(SID, { graphId: GID, scope: SCOPE_Y });

      await clock.advance(999);
      expect(patch).not.toHaveBeenCalled();

      await clock.advance(1);
      expect(patch).toHaveBeenCalledTimes(1);
      expect(patch).toHaveBeenCalledWith(SID, { graphId: GID, scope: SCOPE_Y });
    } finally {
      saver.dispose();
    }
  });

  test('NavHistoryTracker 与 ESLint useNavigate 限制', async () => {
    const tracker = createNavHistoryTracker();
    tracker.sync('k1', `/c/${SID}/g/${GID}`, 'POP');
    tracker.sync('k2', `/c/${SID}/g/${GID}?scope=${SCOPE_X}`, 'PUSH');
    expect(tracker.previousPath()).toBe(`/c/${SID}/g/${GID}`);

    const { createConfig } = (await import('../../eslint.config.js')) as {
      createConfig: (opts?: { enableTypeChecked?: boolean }) => unknown[];
    };
    const eslint = new ESLint({
      cwd: webDir,
      overrideConfigFile: true,
      overrideConfig: createConfig({ enableTypeChecked: false }) as never,
    });
    const [badRes] = await eslint.lintText(
      "import { useNavigate } from 'react-router';\nexport const f = useNavigate;\n",
      {
        filePath: resolve(webDir, 'src', 'panels', 'bad.tsx'),
      },
    );
    expect(
      badRes?.messages.some((m) => m.ruleId === 'no-restricted-imports' && m.message.includes('W.02.01')),
    ).toBe(true);

    // K309 D07：react-router-dom 与 namespace 访问均被拦
    const [badDom] = await eslint.lintText(
      "import { useNavigate } from 'react-router-dom';\nexport const f = useNavigate;\n",
      {
        filePath: resolve(webDir, 'src', 'panels', 'bad2.tsx'),
      },
    );
    expect(
      badDom?.messages.some((m) => m.ruleId === 'no-restricted-imports' && m.message.includes('W.02.01')),
    ).toBe(true);

    const [badNs] = await eslint.lintText(
      "import * as rr from 'react-router';\nexport const f = rr.useNavigate;\n",
      {
        filePath: resolve(webDir, 'src', 'panels', 'bad3.tsx'),
      },
    );
    expect(
      badNs?.messages.some((m) => m.ruleId === 'no-restricted-syntax' && m.message.includes('W.02.01')),
    ).toBe(true);
  }, 15000);
});
