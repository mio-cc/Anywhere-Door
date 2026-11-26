// 扫描前端测试文件里的用例编号（W.12 §6.8），按叶子分组列出并检查重复。
// 支持 test('...')、it('...') 以及 test.each(...)( '...')、it.each(...)( '...')、test.for / it.for。
import { existsSync, readdirSync, readFileSync } from 'node:fs';
import { dirname, join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const TEST_CALL_RE = /^\s*(?:test|it)(?:\.(?:each|for)\s*\([^)]*\))?\s*\(\s*(['"`])([^'"`]+)\1/gm;
const ID_PREFIX_RE = /^([A-Z]\d{2,6}-T\d+)(?:\/([^\s]+))?(?:\s|$)/;

/**
 * 扫描指定目录列表中的所有前端测试文件，提取测试用例编号并检查重复。
 * @param {string[]} roots
 * @param {string} baseDir
 */
export function collectTestIds(roots, baseDir) {
  /** @type {Array<{ id: string; sub?: string; title: string; file: string; line: number }>} */
  const entries = [];
  /** @type {Map<string, Array<{ file: string; line: number; title: string }>>} */
  const seen = new Map();

  function walk(dir) {
    if (!existsSync(dir)) return;
    for (const ent of readdirSync(dir, { withFileTypes: true })) {
      if (ent.name === 'node_modules' || ent.name === 'dist' || ent.name === '.test') continue;
      const full = join(dir, ent.name);
      if (ent.isDirectory()) {
        walk(full);
      } else if (/\.(?:test|spec)\.tsx?$/.test(ent.name)) {
        const text = readFileSync(full, 'utf8');
        const rel = relative(baseDir, full).replace(/\\/g, '/');
        for (const m of text.matchAll(TEST_CALL_RE)) {
          const title = m[2];
          const idMatch = ID_PREFIX_RE.exec(title);
          if (!idMatch) continue;
          const id = idMatch[1];
          const sub = idMatch[2];
          const key = sub ? `${id}/${sub}` : id;
          const line = text.slice(0, m.index).split('\n').length;
          entries.push({ id, sub, title, file: rel, line });
          const list = seen.get(key) ?? [];
          list.push({ file: rel, line, title });
          seen.set(key, list);
        }
      }
    }
  }

  for (const r of roots) walk(r);

  const duplicates = [];
  for (const [key, locs] of seen.entries()) {
    if (locs.length > 1) {
      duplicates.push({ key, locs });
    }
  }

  /** @type {Map<string, typeof entries>} */
  const byLeaf = new Map();
  for (const e of entries) {
    const leaf = e.id.split('-')[0];
    const list = byLeaf.get(leaf) ?? [];
    list.push(e);
    byLeaf.set(leaf, list);
  }

  return { entries, byLeaf, duplicates };
}

function isMain() {
  if (!process.argv[1]) return false;
  return resolve(process.argv[1]) === fileURLToPath(import.meta.url);
}

if (isMain()) {
  const here = dirname(fileURLToPath(import.meta.url));
  const webDir = resolve(here, '..', '..');
  const repoRoot = resolve(webDir, '..');
  const { byLeaf, duplicates } = collectTestIds(
    [resolve(webDir, 'src'), resolve(webDir, 'tests'), resolve(repoRoot, 'tools', 'lint', 'eslint')],
    repoRoot,
  );
  for (const leaf of Array.from(byLeaf.keys()).sort()) {
    console.log(`[${leaf}]`);
    for (const item of byLeaf.get(leaf) ?? []) {
      const label = item.sub ? `${item.id}/${item.sub}` : item.id;
      console.log(`  ${label}  (${item.file}:${item.line})`);
    }
  }
  if (duplicates.length > 0) {
    console.error('发现重复的测试编号：');
    for (const d of duplicates) {
      const where = d.locs.map((l) => `${l.file}:${l.line}`).join('、');
      console.error(`  ${d.key}: ${where}`);
    }
    process.exit(1);
  }
}
