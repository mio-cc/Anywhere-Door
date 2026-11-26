export type ConsoleLevel = 'warn' | 'error';

interface ConsoleExpectation {
  level: ConsoleLevel;
  match: string | RegExp;
  times: number | undefined;
  hits: number;
}

const origWarn = console.warn.bind(console);
const origError = console.error.bind(console);

let installed = false;
let expectations: ConsoleExpectation[] = [];
let unexpected: string[] = [];

function formatArgs(args: readonly unknown[]): string {
  return args
    .map((a) => {
      if (typeof a === 'string') return a;
      if (a instanceof Error) return a.message;
      try {
        return JSON.stringify(a);
      } catch {
        return String(a);
      }
    })
    .join(' ');
}

function matches(msg: string, match: string | RegExp): boolean {
  return typeof match === 'string' ? msg.includes(match) : match.test(msg);
}

function intercept(level: ConsoleLevel, args: unknown[]): void {
  const msg = formatArgs(args);
  for (const exp of expectations) {
    if (exp.level === level && matches(msg, exp.match)) {
      exp.hits += 1;
      return;
    }
  }
  unexpected.push(`[console.${level}] ${msg}`);
}

export function installConsoleGuard(): void {
  if (installed) return;
  installed = true;
  console.warn = (...args: unknown[]) => {
    intercept('warn', args);
  };
  console.error = (...args: unknown[]) => {
    intercept('error', args);
  };
}

export function expectConsole(level: ConsoleLevel, match: string | RegExp, times?: number): void {
  expectations.push({ level, match, times, hits: 0 });
}

export function resetConsoleExpectations(): void {
  expectations = [];
  unexpected = [];
}

export function verifyConsoleExpectations(): string[] {
  const errors: string[] = [...unexpected];
  for (const exp of expectations) {
    const desc = typeof exp.match === 'string' ? `"${exp.match}"` : exp.match.toString();
    if (exp.times !== undefined) {
      if (exp.hits !== exp.times) {
        errors.push(
          `预期 console.${exp.level}(${desc}) 调用 ${String(exp.times)} 次，实际 ${String(exp.hits)} 次`,
        );
      }
    } else if (exp.hits === 0) {
      errors.push(`预期 console.${exp.level}(${desc}) 至少调用 1 次，实际 0 次`);
    }
  }
  resetConsoleExpectations();
  return errors;
}

export function restoreConsoleGuard(): void {
  if (!installed) return;
  installed = false;
  console.warn = origWarn;
  console.error = origError;
  resetConsoleExpectations();
}
