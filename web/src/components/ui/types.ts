export type Size = 'md' | 'sm';
export type Tone = 'neutral' | 'accent' | 'ok' | 'danger' | 'wait' | 'model';
export type Point = { readonly x: number; readonly y: number };
export type Anchor = HTMLElement | DOMRect;
export type Placement =
  | 'top'
  | 'bottom'
  | 'left'
  | 'right'
  | 'top-start'
  | 'top-end'
  | 'bottom-start'
  | 'bottom-end'
  | 'right-start'
  | 'left-start';
export type Layer = 'popover' | 'modal' | 'toast' | 'tooltip' | 'pane';
export type CloseReason = 'esc' | 'outside' | 'tab' | 'action';
export type { IconName } from '@/styles/icons';

/** 判断是否处于输入法组字状态（U.13.04-R6、W.08.02 §6.1）。 */
export function isImeComposing(e: KeyboardEvent): boolean {
  return e.isComposing || (e as unknown as { readonly keyCode?: number }).keyCode === 229;
}
