/* global document, window */
(() => {
  const d = document.documentElement;
  let p = 'paper';
  let m = 'light';
  let r = 'auto';
  try {
    const ls = window.localStorage;
    const sp = ls.getItem('hx:theme-palette');
    if (sp === 'paper' || sp === 'graphite' || sp === 'whale' || sp === 'amber') p = sp;
    const sm = ls.getItem('hx:theme-mode');
    if (sm === 'light' || sm === 'dark' || sm === 'system') m = sm;
    const sr = ls.getItem('hx:motion');
    if (sr === 'auto' || sr === 'reduce') r = sr;
  } catch {
    /* ignore */
  }
  const mm = (q) => {
    try {
      return Boolean(window.matchMedia && window.matchMedia(q).matches);
    } catch {
      return false;
    }
  };
  const t = m === 'system' ? (mm('(prefers-color-scheme: dark)') ? 'dark' : 'light') : m;
  d.setAttribute('data-palette', p);
  d.setAttribute('data-theme', t);
  if (r === 'reduce' || mm('(prefers-reduced-motion: reduce)')) d.setAttribute('data-motion', 'reduce');
  else d.removeAttribute('data-motion');
})();
