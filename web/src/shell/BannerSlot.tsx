import type { JSX, ReactNode } from 'react';
import { useT } from '@/lib/i18n';
import type { ShellLayout } from '@/state/uiStore';

export interface BannerSlotProps {
  readonly bp: ShellLayout['bp'];
  readonly children?: ReactNode;
}

/** 顶部条下方横幅槽位：急停、离线、子图库提示与窄屏提示（W.02.02 §6.6）。 */
export function BannerSlot(p: BannerSlotProps): JSX.Element | null {
  const { bp, children } = p;
  const t = useT();
  const showNarrow = bp === 'xs';

  if (!showNarrow && !children) return null;

  return (
    <div className="hx-app__banner" role="region" aria-live="polite">
      {children}
      {showNarrow ? (
        <div className="hx-app__narrow" role="status">
          {t('shell.narrowScreen')}
        </div>
      ) : null}
    </div>
  );
}
