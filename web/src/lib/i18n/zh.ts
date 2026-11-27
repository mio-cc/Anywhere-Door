// 唯一界面文案表（U.15-R5、W.10 §4.2–§4.4）。
// 每个值必须是一行内的单引号字符串字面量，便于字体子集扫描（U.04-R2）。
export const zh = {
  // ── common. 通用（W.10）──
  'common.ok': '确定',
  'common.cancel': '取消',
  'common.retry': '重试',
  'common.save': '保存',
  'common.delete': '删除',
  'common.undo': '撤销',
  'common.view': '查看',
  'common.copyDetails': '复制错误详情',
  'common.openSettings': '打开设置',
  'common.unknown': '未知',
  'common.loadingWhat': '正在加载{what}…',
  'common.net.network': '与 Anywhere Door 的连接已断开。连接恢复后再试',
  'common.net.timeout': '服务没有及时响应。稍后重试',
  'common.net.aborted': '已取消',
  'common.scopeGone': '那一层已经不存在了',

  // ── fmt. 格式化（W.10）──
  'fmt.justNow': '刚刚',
  'fmt.minAgo': '{n} 分钟前',
  'fmt.hourAgo': '{n} 小时前',
  'fmt.dayAgo': '{n} 天前',
  'fmt.yesterday': '昨天',
  'fmt.zone': '（UTC{offset}）',
  'fmt.wan': '{n} 万',
  'fmt.yi': '{n} 亿',
  'fmt.lt1s': '<1 秒',
  'fmt.sec': '{n} 秒',
  'fmt.minSec': '{m} 分 {s} 秒',
  'fmt.hourMin': '{h} 小时 {m} 分',

  // ── shell. 外壳与启动（W.02）──

  // ── shortcut. 快捷键说明（W.02.04）──

  // ── ops. Op 层与预检（W.05 / W.10）──
  'ops.precheck.readonly': '离线或只读时不能修改图',
  'ops.precheck.frozen': '「{label}」已冻结。先解除冻结再修改',
  'ops.precheck.paramLocked': '这个参数由节点类型固定，不能修改',
  'ops.precheck.unknownType': '找不到节点类型「{type}」。它所在的包可能已停用',
  'ops.precheck.noPort': '这个端口已经不存在了',
  'ops.precheck.direction': '只能从输出端口连到输入端口',
  'ops.precheck.crossScope': '只能连接同一层里的节点',
  'ops.precheck.cycle': '环只能通过循环节点形成',
  'ops.precheck.duplicateEdge': '这两个端口已经连着了',
  'ops.precheck.groupInvalid': '先选中同一层里至少 2 个节点',
  'ops.precheck.labelLength': '显示名要在 1–80 个字之间',
  'ops.precheck.tooManyTags': '标签最多 16 个',
  'ops.precheck.replaces': '将替换原连线',

  // ── canvas. 画布引擎（W.06）──

  // ── top. 顶部条（W.07.01）──

  // ── ls. 左侧栏（W.07.02）──

  // ── insp. 右侧栏：属性（W.07.03）──

  // ── chat. 右侧栏：对话（W.07.04）──

  // ── mon. 右侧栏：监控（W.07.05）──

  // ── dock. 停靠栏与浮动面板（W.07.06）──

  // ── comp. 底部对话框（W.07.07）──

  // ── float. 浮动回复（W.07.08）──

  // ── appr. 审批卡片（W.07.09）──

  // ── notice. 提示、横幅与通知中心（W.07.10）──

  // ── menu. 右键菜单与命令面板（W.07.11）──

  // ── ui. 基础组件库（W.08 / W.10）──
  'ui.close': '关闭',
  'ui.clear': '清空',
  'ui.loading': '加载中',
  'ui.decrease': '减少',
  'ui.increase': '增加',
  'ui.locked': '已锁定',
  'ui.max': '最大为 {n}',
  'ui.min': '最小为 {n}',
  'ui.noMatch': '没有匹配的选项',
  'ui.filter': '筛选',
  'ui.submenu': '展开子菜单',
  'ui.menu': '菜单',
  'ui.ai': 'AI',

  // ── home. 会话首页与空状态（W.09.01）──

  // ── center. 中心窗口外壳（W.09.02）──

  // ── set. 设置（W.09.02）──

  // ── lib. 资源库（W.09.02）──

  // ── welcome. 首次引导（W.09.03）──

  // ── tour. 画布导览（W.09.03）──

  // ── pkg. 包（W.09.04）──

  // ── conn. 连接（W.09.04）──

  // ── hist. 运行历史与回放（W.09.05）──
} as const satisfies Record<string, string>;
