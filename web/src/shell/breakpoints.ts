import { BREAKPOINTS, bpOf, type Breakpoint } from '@/state/types';

export { BREAKPOINTS, bpOf, type Breakpoint };

export function isOverlayRightSidebar(bp: Breakpoint): boolean {
  return bp === 'md' || bp === 'sm' || bp === 'xs';
}
