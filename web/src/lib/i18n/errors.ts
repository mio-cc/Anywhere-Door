import { ERRORS } from './generated/messages';
import { t, type Vars } from './t';

export type ErrorLike =
  { code: string; message?: string; details?: unknown } | { kind: 'network' | 'timeout' | 'aborted' };

function detailsToVars(details: unknown): Vars | undefined {
  if (typeof details !== 'object' || details === null || Array.isArray(details)) {
    return undefined;
  }
  const out: Record<string, string | number> = {};
  for (const [k, v] of Object.entries(details as Record<string, unknown>)) {
    if (typeof v === 'string' || typeof v === 'number') {
      out[k] = v;
    } else if (Array.isArray(v)) {
      const strs = v.filter((x): x is string | number => typeof x === 'string' || typeof x === 'number');
      if (strs.length > 0) {
        out[k] = strs.join(', ');
      }
    }
  }
  return out;
}

export function errorText(e: ErrorLike): string {
  if ('kind' in e) {
    if (e.kind === 'network') return t('common.net.network');
    if (e.kind === 'timeout') return t('common.net.timeout');
    return t('common.net.aborted');
  }
  if (e.message !== undefined && e.message !== '') {
    return e.message;
  }
  if (Object.prototype.hasOwnProperty.call(ERRORS, e.code)) {
    return t.dyn(`err.${e.code}`, detailsToVars(e.details));
  }
  return ERRORS['internal'] ?? '';
}
