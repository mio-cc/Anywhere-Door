// 与 项目树_v0.5/tools/术语检查.json 的 rules 保持一致（W.10 §6.5、U.15-R1、G.02.05-R26）。
export interface ForbiddenTermRule {
  readonly pattern: RegExp;
  readonly show: string;
  readonly use: string;
}

export const FORBIDDEN_TERMS: readonly ForbiddenTermRule[] = [
  { pattern: /子系统|嵌套系统|子流程/, show: '子系统 / 嵌套系统 / 子流程', use: '子图' },
  { pattern: /运行抽屉/, show: '运行抽屉', use: '监控页' },
  { pattern: /检查器|Inspector/, show: '检查器', use: '属性页' },
  { pattern: /挂起/, show: '挂起', use: '等待中 / 等待审批' },
  { pattern: /封装为子图/, show: '封装为子图', use: '打包为子图' },
  { pattern: /(?<!引导)气泡/, show: '气泡', use: '浮动回复' },
  { pattern: /工具栏/, show: '工具栏', use: '停靠栏' },
  { pattern: /侧边栏/, show: '侧边栏', use: '左侧栏 / 右侧栏' },
  { pattern: /输入栏|聊天框/, show: '输入栏 / 聊天框', use: '对话框' },
  { pattern: /鹰眼/, show: '鹰眼', use: '小地图' },
  { pattern: /推理强度/, show: '推理强度', use: '思考强度' },
  { pattern: /挡位/, show: '挡位', use: '强度（思考强度）；授权用"档位"' },
  { pattern: /参数转端口|转为端口/, show: '参数转端口', use: '参数接线' },
  { pattern: /设置中心/, show: '设置中心', use: '中心窗口' },
  { pattern: /同名接口|变量接口|节点上的接口|悬停接口/, show: '接口（指节点端口）', use: '端口' },
  { pattern: /运行页/, show: '运行页', use: '监控页' },
  {
    pattern: /harnessd|harnessctl|(?<!DeepSeek )Harness(?!Client)/,
    show: 'Harness / harnessd / harnessctl',
    use: 'Anywhere Door / anydoor / anydoorctl',
  },
];
