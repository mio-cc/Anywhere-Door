/* 由 tools/gen tokens 生成，请勿手改 */
export interface ThemeMeta {
  readonly key: 'paper' | 'graphite' | 'whale' | 'amber';
  readonly name: string;
  readonly ref: string;
  readonly desc: string;
  readonly preview: {
    readonly light: readonly [string, string, string, string];
    readonly dark: readonly [string, string, string, string];
  };
}

export const THEMES = [
  {
    key: 'paper',
    name: '宣纸',
    ref: 'Claude',
    desc: '暖调纸面，所有灰色带一点黄褐；主色陶土橙；模型色灰蓝',
    preview: {
      light: ['#f4f2ec', '#fdfcf9', '#b9573a', '#46659c'],
      dark: ['#1a1a18', '#21211f', '#d97757', '#8fb0e3'],
    },
  },
  {
    key: 'graphite',
    name: '石墨',
    ref: 'Codex',
    desc: '冷灰哑光，像原生开发工具；主按钮是墨色，蓝色只表示运行中；模型色紫',
    preview: {
      light: ['#f8f8f9', '#ffffff', '#18191d', '#7048d6'],
      dark: ['#151517', '#1b1b1e', '#ececee', '#a78bfa'],
    },
  },
  {
    key: 'whale',
    name: '墨鲸',
    ref: 'DeepSeek Harness',
    desc: '冷白与墨色底，主色浅海蓝，模型色紫',
    preview: {
      light: ['#f8f9fd', '#ffffff', '#4866f8', '#8a3fd1'],
      dark: ['#121318', '#181a20', '#93a6ff', '#c58cfb'],
    },
  },
  {
    key: 'amber',
    name: '琥珀',
    ref: '设计规范 v0.3',
    desc: '中性灰底，主色琥珀，模型色靛蓝',
    preview: {
      light: ['#f8f8f6', '#ffffff', '#f0a01e', '#4f5fd6'],
      dark: ['#101217', '#15171d', '#f5a524', '#93a5ff'],
    },
  },
] as const satisfies readonly ThemeMeta[];

export const DEFAULT_PALETTE = 'paper' as const;
