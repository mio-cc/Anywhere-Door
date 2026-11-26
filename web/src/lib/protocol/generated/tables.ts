/* 由 tools/gen 生成，请勿手改 */

// 来源：schema/builtin/*.yaml（C.14 §5.3），照原样输出，键不转换；只有 lib/protocol 读这些表（W.04.04-R1）。
import type { NodeState, RunStatus } from "./types";

/** 容器规则（C.02.04 §5）（container_rules.yaml） */
export const CONTAINER_RULES = [
  {
    "type": "循环",
    "children": "required",
    "ref": "forbidden"
  },
  {
    "type": "扇出",
    "children": "required",
    "ref": "forbidden"
  },
  {
    "type": "子图引用",
    "children": "one_of",
    "ref": "one_of"
  }
] as const;

/** 参数控件映射（C.03.02 §4）（widget_map.yaml） */
export const WIDGET_MAP = {
  "rules": [
    {
      "types": [
        "integer",
        "number"
      ],
      "when": {
        "span_steps": 100
      },
      "widget": "slider"
    },
    {
      "types": [
        "integer",
        "number"
      ],
      "widget": "stepper"
    },
    {
      "types": [
        "boolean"
      ],
      "widget": "switch"
    },
    {
      "types": [
        "enum"
      ],
      "when": {
        "max_options": 4,
        "max_label_chars": 6
      },
      "widget": "segmented"
    },
    {
      "types": [
        "enum"
      ],
      "widget": "select"
    },
    {
      "types": [
        "string"
      ],
      "widget": "input"
    },
    {
      "types": [
        "text"
      ],
      "widget": "textarea"
    },
    {
      "types": [
        "expr"
      ],
      "widget": "code"
    },
    {
      "types": [
        "model_ref"
      ],
      "widget": "model"
    },
    {
      "types": [
        "rolecard_ref"
      ],
      "widget": "rolecard"
    },
    {
      "types": [
        "type_ref",
        "recipe_ref",
        "tool_ref"
      ],
      "widget": "picker"
    },
    {
      "types": [
        "object"
      ],
      "when": {
        "has_schema": true
      },
      "subform": true
    },
    {
      "types": [
        "object"
      ],
      "widget": "code"
    }
  ]
} as const;

/** 数据类型 → 显示类别（C.04.01 §3）（category_map.yaml） */
export const CATEGORY_MAP = {
  "categories": [
    "any",
    "ctx",
    "ref",
    "list",
    "tool",
    "gen",
    "ctrl",
    "state"
  ],
  "rules": [
    {
      "type": "任意",
      "category": "any"
    },
    {
      "list": true,
      "category": "list"
    },
    {
      "type": "上下文包",
      "category": "ctx"
    },
    {
      "type": "资源引用",
      "category": "ref"
    },
    {
      "type": "工具端点",
      "category": "tool"
    },
    {
      "type": "状态",
      "category": "state"
    },
    {
      "default": true,
      "category": "gen"
    }
  ]
} as const;

/** 聚合算子 → 输出类型（C.04.03 §2）（aggregate_ops.yaml） */
export const AGGREGATE_OPS = {
  "count": "整数@1",
  "group": "聚合结果@1",
  "group_ratio": "聚合结果@1",
  "vote": "$T",
  "dedupe": "列表[$T]",
  "sort": "列表[$T]",
  "filter": "列表[$T]",
  "top_k": "列表[$T]",
  "concat": "列表[$T]",
  "stats": "统计@1",
  "divergence": "分歧报告@1"
} as const;

/** 授权默认判定表（C.09 §5.1）（authz_matrix.yaml） */
export const AUTHZ_MATRIX = {
  "rows": [
    {
      "actions": [
        "graph.read",
        "run.read",
        "trace.read"
      ],
      "levels": [
        "allow",
        "allow",
        "allow",
        "allow",
        "allow"
      ]
    },
    {
      "actions": [
        "op.view"
      ],
      "levels": [
        "deny",
        "allow",
        "allow",
        "allow",
        "allow"
      ]
    },
    {
      "actions": [
        "op.add_node",
        "op.restore_nodes",
        "op.connect",
        "op.disconnect",
        "op.set_param",
        "op.promote_param",
        "op.demote_param",
        "op.set_label",
        "op.set_tags",
        "op.group",
        "op.set_subgraph_ref",
        "op.set_graph_title",
        "op.set_graph_meta",
        "op.create_branch",
        "op.merge_branch",
        "op.revert_op"
      ],
      "levels": [
        "deny",
        "propose",
        {
          "if": "overrides_ops",
          "then": "allow",
          "else": "propose"
        },
        "allow",
        "allow"
      ]
    },
    {
      "actions": [
        "op.remove_nodes",
        "op.ungroup",
        "op.revert_to",
        "op.delete_branch"
      ],
      "levels": [
        "deny",
        "propose",
        "ask",
        "allow",
        "allow"
      ]
    },
    {
      "actions": [
        "res.edit_rolecard",
        "res.edit_recipe",
        "res.define_type"
      ],
      "levels": [
        "deny",
        "propose",
        "ask",
        "allow",
        "allow"
      ]
    },
    {
      "actions": [
        "res.switch_model"
      ],
      "levels": [
        "deny",
        "propose",
        {
          "if": "in_overrides_models",
          "then": "allow",
          "else": "ask"
        },
        "allow",
        "allow"
      ]
    },
    {
      "actions": [
        "authz.set_mandate"
      ],
      "levels": [
        "deny",
        "deny",
        "deny",
        "ask",
        "allow"
      ]
    },
    {
      "actions": [
        "authz.request_elevation"
      ],
      "levels": [
        "deny",
        "allow",
        "allow",
        "allow",
        "allow"
      ]
    },
    {
      "actions": [
        "run.start",
        "run.pause"
      ],
      "levels": [
        "deny",
        "ask",
        "allow",
        "allow",
        "allow"
      ]
    },
    {
      "actions": [
        "run.cancel"
      ],
      "levels": [
        "deny",
        "allow",
        "allow",
        "allow",
        "allow"
      ]
    },
    {
      "actions": [
        "orch.spawn"
      ],
      "levels": [
        "deny",
        "deny",
        {
          "if": "overrides_allow_spawn",
          "then": "allow",
          "else": "deny"
        },
        "allow",
        "allow"
      ]
    },
    {
      "actions": [
        "orch.self_modify"
      ],
      "levels": [
        "deny",
        "deny",
        "deny",
        "ask",
        "allow"
      ]
    },
    {
      "actions": [
        "tool.L0"
      ],
      "levels": [
        "deny",
        "allow",
        "allow",
        "allow",
        "allow"
      ]
    },
    {
      "actions": [
        "tool.L1",
        "tool.L2"
      ],
      "levels": [
        "deny",
        "ask",
        {
          "if": "within_tools_max_risk",
          "then": "allow",
          "else": "ask"
        },
        "allow",
        "allow"
      ]
    },
    {
      "actions": [
        "tool.L3"
      ],
      "levels": [
        "deny",
        "ask",
        "ask",
        "ask",
        "allow"
      ]
    },
    {
      "actions": [
        "estop.trigger"
      ],
      "levels": [
        "allow",
        "allow",
        "allow",
        "allow",
        "allow"
      ]
    },
    {
      "actions": [
        "estop.release"
      ],
      "levels": [
        "deny",
        "deny",
        "deny",
        "deny",
        "deny"
      ]
    }
  ],
  "fixed": {
    "op.set_lock": "user",
    "op.migrate": "runtime"
  },
  "op_actions": {
    "move_nodes": "op.view",
    "set_node_ui": "op.view",
    "add_note": "op.view",
    "update_note": "op.view",
    "remove_notes": "op.view",
    "set_mandate": "authz.set_mandate",
    "apply_patch": "per_sub_op",
    "import_apply": "per_sub_op"
  }
} as const;

/** 运行和节点的状态机（C.07 §6）（state_machine.yaml） */
export const STATE_MACHINE = {
  "run": {
    "states": [
      "pending",
      "running",
      "paused",
      "done",
      "failed",
      "cancelled"
    ],
    "terminal": [
      "done",
      "failed",
      "cancelled"
    ],
    "transitions": {
      "pending": [
        "running",
        "cancelled"
      ],
      "running": [
        "paused",
        "done",
        "failed",
        "cancelled"
      ],
      "paused": [
        "running",
        "cancelled"
      ]
    }
  },
  "node": {
    "states": [
      "idle",
      "queued",
      "running",
      "waiting",
      "done",
      "failed",
      "cancelled",
      "skipped",
      "cached"
    ],
    "terminal": [
      "done",
      "failed",
      "cancelled",
      "skipped",
      "cached"
    ],
    "transitions": {
      "idle": [
        "queued",
        "cancelled",
        "skipped",
        "cached"
      ],
      "queued": [
        "running",
        "cancelled"
      ],
      "running": [
        "queued",
        "waiting",
        "done",
        "failed",
        "cancelled"
      ],
      "waiting": [
        "running",
        "failed",
        "cancelled"
      ],
      "failed": [
        "queued"
      ]
    }
  }
} as const;

const runTransitions: Readonly<Record<string, readonly string[]>> = STATE_MACHINE.run.transitions;
const nodeTransitions: Readonly<Record<string, readonly string[]>> = STATE_MACHINE.node.transitions;
const runTerminal: readonly string[] = STATE_MACHINE.run.terminal;
const nodeTerminal: readonly string[] = STATE_MACHINE.node.terminal;

/** 运行状态 from 能不能进入 to（C.07）。 */
export function runCanTransition(from: RunStatus, to: RunStatus): boolean {
  return (runTransitions[from] ?? []).includes(to);
}

/** 运行状态 s 是不是终态。 */
export function runIsTerminal(s: RunStatus): boolean {
  return runTerminal.includes(s);
}

/** 节点状态 from 能不能进入 to（C.07）。 */
export function nodeCanTransition(from: NodeState, to: NodeState): boolean {
  return (nodeTransitions[from] ?? []).includes(to);
}

/** 节点状态 s 是不是终态。 */
export function nodeIsTerminal(s: NodeState): boolean {
  return nodeTerminal.includes(s);
}

/** 快捷键总表（U.13.04、W.02.04 §4.1）（shortcuts.yaml） */
export const SHORTCUTS = [
  {
    "id": "palette.open",
    "keys": [
      "Mod+K"
    ],
    "layer": "global",
    "in_input": true,
    "hold": false,
    "label": "shortcut.palette.open",
    "group": "全局"
  },
  {
    "id": "center.open",
    "keys": [
      "Mod+,"
    ],
    "layer": "global",
    "in_input": true,
    "hold": false,
    "label": "shortcut.center.open",
    "group": "全局"
  },
  {
    "id": "run.start",
    "keys": [
      "Mod+Enter"
    ],
    "layer": "global",
    "in_input": false,
    "hold": false,
    "label": "shortcut.run.start",
    "group": "全局"
  },
  {
    "id": "estop.trigger",
    "keys": [
      "Mod+."
    ],
    "layer": "global",
    "in_input": true,
    "hold": false,
    "label": "shortcut.estop.trigger",
    "group": "全局"
  },
  {
    "id": "ls.toggle",
    "keys": [
      "["
    ],
    "layer": "global",
    "in_input": false,
    "hold": false,
    "label": "shortcut.ls.toggle",
    "group": "全局"
  },
  {
    "id": "rs.toggle",
    "keys": [
      "]"
    ],
    "layer": "global",
    "in_input": false,
    "hold": false,
    "label": "shortcut.rs.toggle",
    "group": "全局"
  },
  {
    "id": "rs.insp",
    "keys": [
      "I"
    ],
    "layer": "global",
    "in_input": false,
    "hold": false,
    "label": "shortcut.rs.insp",
    "group": "全局"
  },
  {
    "id": "focus.toggle",
    "keys": [
      "F"
    ],
    "layer": "global",
    "in_input": false,
    "hold": false,
    "label": "shortcut.focus.toggle",
    "group": "全局"
  },
  {
    "id": "lib.search",
    "keys": [
      "/"
    ],
    "layer": "global",
    "in_input": false,
    "hold": false,
    "label": "shortcut.lib.search",
    "group": "全局"
  },
  {
    "id": "help.keys",
    "keys": [
      "Shift+/"
    ],
    "layer": "global",
    "in_input": false,
    "hold": false,
    "label": "shortcut.help.keys",
    "group": "全局"
  },
  {
    "id": "session.prev",
    "keys": [
      "Alt+["
    ],
    "layer": "global",
    "in_input": false,
    "hold": false,
    "label": "shortcut.session.prev",
    "group": "全局"
  },
  {
    "id": "session.next",
    "keys": [
      "Alt+]"
    ],
    "layer": "global",
    "in_input": false,
    "hold": false,
    "label": "shortcut.session.next",
    "group": "全局"
  },
  {
    "id": "session.new",
    "keys": [
      "Alt+N"
    ],
    "layer": "global",
    "in_input": false,
    "hold": false,
    "label": "shortcut.session.new",
    "group": "全局"
  },
  {
    "id": "tool.select",
    "keys": [
      "V"
    ],
    "layer": "canvas",
    "in_input": false,
    "hold": false,
    "label": "shortcut.tool.select",
    "group": "画布"
  },
  {
    "id": "tool.pan",
    "keys": [
      "H"
    ],
    "layer": "canvas",
    "in_input": false,
    "hold": false,
    "label": "shortcut.tool.pan",
    "group": "画布"
  },
  {
    "id": "tool.pan.hold",
    "keys": [
      "Space"
    ],
    "layer": "canvas",
    "in_input": false,
    "hold": true,
    "label": "shortcut.tool.pan.hold",
    "group": "画布"
  },
  {
    "id": "node.quickAdd",
    "keys": [
      "A"
    ],
    "layer": "canvas",
    "in_input": false,
    "hold": false,
    "label": "shortcut.node.quickAdd",
    "group": "画布"
  },
  {
    "id": "sel.delete",
    "keys": [
      "Delete",
      "Backspace"
    ],
    "layer": "canvas",
    "in_input": false,
    "hold": false,
    "label": "shortcut.sel.delete",
    "group": "画布"
  },
  {
    "id": "sel.all",
    "keys": [
      "Mod+A"
    ],
    "layer": "canvas",
    "in_input": false,
    "hold": false,
    "label": "shortcut.sel.all",
    "group": "画布"
  },
  {
    "id": "sel.duplicate",
    "keys": [
      "Mod+D"
    ],
    "layer": "canvas",
    "in_input": false,
    "hold": false,
    "label": "shortcut.sel.duplicate",
    "group": "画布"
  },
  {
    "id": "clip.copy",
    "keys": [
      "Mod+C"
    ],
    "layer": "canvas",
    "in_input": false,
    "hold": false,
    "label": "shortcut.clip.copy",
    "group": "画布"
  },
  {
    "id": "clip.paste",
    "keys": [
      "Mod+V"
    ],
    "layer": "canvas",
    "in_input": false,
    "hold": false,
    "label": "shortcut.clip.paste",
    "group": "画布"
  },
  {
    "id": "group",
    "keys": [
      "Mod+G"
    ],
    "layer": "canvas",
    "in_input": false,
    "hold": false,
    "label": "shortcut.group",
    "group": "画布"
  },
  {
    "id": "ungroup",
    "keys": [
      "Mod+Shift+G"
    ],
    "layer": "canvas",
    "in_input": false,
    "hold": false,
    "label": "shortcut.ungroup",
    "group": "画布"
  },
  {
    "id": "undo",
    "keys": [
      "Mod+Z"
    ],
    "layer": "canvas",
    "in_input": false,
    "hold": false,
    "label": "shortcut.undo",
    "group": "画布"
  },
  {
    "id": "redo",
    "keys": [
      "Mod+Shift+Z",
      "Mod+Y"
    ],
    "layer": "canvas",
    "in_input": false,
    "hold": false,
    "label": "shortcut.redo",
    "group": "画布"
  },
  {
    "id": "view.fit",
    "keys": [
      "1"
    ],
    "layer": "canvas",
    "in_input": false,
    "hold": false,
    "label": "shortcut.view.fit",
    "group": "画布"
  },
  {
    "id": "view.focusSel",
    "keys": [
      "2"
    ],
    "layer": "canvas",
    "in_input": false,
    "hold": false,
    "label": "shortcut.view.focusSel",
    "group": "画布"
  },
  {
    "id": "layout.tidy",
    "keys": [
      "L"
    ],
    "layer": "canvas",
    "in_input": false,
    "hold": false,
    "label": "shortcut.layout.tidy",
    "group": "画布"
  },
  {
    "id": "node.rename",
    "keys": [
      "F2"
    ],
    "layer": "canvas",
    "in_input": false,
    "hold": false,
    "label": "shortcut.node.rename",
    "group": "画布"
  },
  {
    "id": "sel.move",
    "keys": [
      "ArrowUp",
      "ArrowDown",
      "ArrowLeft",
      "ArrowRight"
    ],
    "layer": "canvas",
    "in_input": false,
    "hold": false,
    "label": "shortcut.sel.move",
    "group": "画布"
  },
  {
    "id": "sel.moveFar",
    "keys": [
      "Shift+ArrowUp",
      "Shift+ArrowDown",
      "Shift+ArrowLeft",
      "Shift+ArrowRight"
    ],
    "layer": "canvas",
    "in_input": false,
    "hold": false,
    "label": "shortcut.sel.moveFar",
    "group": "画布"
  },
  {
    "id": "focus.nextNode",
    "keys": [
      "Tab"
    ],
    "layer": "canvas",
    "in_input": false,
    "hold": false,
    "label": "shortcut.focus.nextNode",
    "group": "画布"
  },
  {
    "id": "focus.prevNode",
    "keys": [
      "Shift+Tab"
    ],
    "layer": "canvas",
    "in_input": false,
    "hold": false,
    "label": "shortcut.focus.prevNode",
    "group": "画布"
  },
  {
    "id": "link.keyboard",
    "keys": [
      "C"
    ],
    "layer": "canvas",
    "in_input": false,
    "hold": false,
    "label": "shortcut.link.keyboard",
    "group": "画布"
  },
  {
    "id": "menu.context",
    "keys": [
      "Shift+F10",
      "ContextMenu"
    ],
    "layer": "canvas",
    "in_input": false,
    "hold": false,
    "label": "shortcut.menu.context",
    "group": "画布"
  },
  {
    "id": "nest.peek",
    "keys": [
      "P"
    ],
    "layer": "nest",
    "in_input": false,
    "hold": false,
    "label": "shortcut.nest.peek",
    "group": "嵌套导航"
  },
  {
    "id": "nest.enter",
    "keys": [
      "Enter"
    ],
    "layer": "nest",
    "in_input": false,
    "hold": false,
    "label": "shortcut.nest.enter",
    "group": "嵌套导航"
  },
  {
    "id": "nest.up",
    "keys": [
      "Alt+ArrowUp"
    ],
    "layer": "nest",
    "in_input": false,
    "hold": false,
    "label": "shortcut.nest.up",
    "group": "嵌套导航"
  },
  {
    "id": "canvas.escape",
    "keys": [
      "Escape"
    ],
    "layer": "nest",
    "in_input": false,
    "hold": false,
    "label": "shortcut.canvas.escape",
    "group": "嵌套导航"
  },
  {
    "id": "composer.send",
    "keys": [
      "Enter",
      "Mod+Enter"
    ],
    "layer": "composer",
    "in_input": true,
    "hold": false,
    "label": "shortcut.composer.send",
    "group": "对话框"
  },
  {
    "id": "composer.newline",
    "keys": [
      "Shift+Enter"
    ],
    "layer": "composer",
    "in_input": true,
    "hold": false,
    "label": "shortcut.composer.newline",
    "group": "对话框"
  },
  {
    "id": "composer.editLast",
    "keys": [
      "ArrowUp"
    ],
    "layer": "composer",
    "in_input": true,
    "hold": false,
    "label": "shortcut.composer.editLast",
    "group": "对话框"
  },
  {
    "id": "composer.blur",
    "keys": [
      "Escape"
    ],
    "layer": "composer",
    "in_input": true,
    "hold": false,
    "label": "shortcut.composer.blur",
    "group": "对话框"
  }
] as const;

