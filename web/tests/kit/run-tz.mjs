// 双时区运行单元测试（W.12 §6.2、W.10）：
// 1) TZ=Asia/Shanghai 跑全部测试；
// 2) TZ=America/New_York 跑 src/lib/i18n 与 *.tz.test.* 测试。
import { spawnSync } from 'node:child_process';
import { existsSync, readdirSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const webDir = resolve(here, '..', '..');
const vitestBin = resolve(webDir, 'node_modules', 'vitest', 'vitest.mjs');

function findTzTargets() {
  const targets = [];
  const i18nDir = resolve(webDir, 'src', 'lib', 'i18n');
  if (existsSync(i18nDir)) {
    const hasI18nTest = readdirSync(i18nDir).some((f) => /\.test\.tsx?$/.test(f));
    if (hasI18nTest) targets.push('src/lib/i18n');
  }
  function walk(dir, relBase) {
    if (!existsSync(dir)) return;
    for (const ent of readdirSync(dir, { withFileTypes: true })) {
      if (ent.name === 'node_modules' || ent.name === 'dist' || ent.name === 'lint') continue;
      const full = join(dir, ent.name);
      const rel = relBase ? `${relBase}/${ent.name}` : ent.name;
      if (ent.isDirectory()) {
        walk(full, rel);
      } else if (/\.tz\.test\.tsx?$/.test(ent.name)) {
        targets.push(rel);
      }
    }
  }
  walk(resolve(webDir, 'src'), 'src');
  walk(resolve(webDir, 'tests', 'kit'), 'tests/kit');
  return targets;
}

function runVitest(tz, args) {
  const res = spawnSync(process.execPath, [vitestBin, 'run', ...args], {
    cwd: webDir,
    stdio: 'inherit',
    env: {
      ...process.env,
      TZ: tz,
      LC_ALL: 'zh_CN.UTF-8',
    },
  });
  if (res.error) throw res.error;
  if (res.status !== 0) {
    process.exit(res.status ?? 1);
  }
}

console.log('== Vitest (TZ=Asia/Shanghai) ==');
runVitest('Asia/Shanghai', []);

const tzTargets = findTzTargets();
if (tzTargets.length > 0) {
  console.log(`== Vitest (TZ=America/New_York): ${tzTargets.join(', ')} ==`);
  runVitest('America/New_York', tzTargets);
}
