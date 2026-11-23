/* 由 tools/gen 生成，请勿手改 */

// 前端字段映射表（C.14-R8、W.04.03 §5.1）：按类型列出每个字段的值怎么转换，字段名一律转 camelCase。
//   "raw"      值整体原样（标量、开放对象、没有类型约束、值都不用转换的数组或映射）
//   { t }      结构类型或判别联合，查 WIRE_TYPES
//   { list }   数组，逐项转换
//   { map }    映射：键是数据、原样，值转换
// union：判别联合。有 field 的，field 的值按 disc 的取值查 branches；没有 field 的，整个对象按 branches 查。
// 表里没有的字段（服务端更新的版本）和不认识的判别值：字段名照常转换，值原样（C.01-R18）。

export type WireNode =
  | "raw"
  | { readonly t: WireTypeName }
  | { readonly list: WireNode }
  | { readonly map: WireNode };

export interface WireUnion {
  readonly disc: string;
  readonly field?: string;
  readonly branches: { readonly [value: string]: WireTypeName };
}

export interface WireType {
  readonly fields: { readonly [key: string]: WireNode };
  readonly union?: WireUnion;
}

export type WireTypeName =
  | "Checked"
  | "Cmd"
  | "CmdMove"
  | "CmdSay"
  | "DataBye"
  | "DataHello"
  | "Envelope"
  | "Shape"
  | "ShapeCircle"
  | "ShapeSquare"
  | "StreamEvent"
  | "Thing"
  | "ThingBox"
  | "ThingRowsItem"
  ;

export const WIRE_TYPES: { readonly [K in WireTypeName]: WireType } = {
  Checked: {
    fields: {
      "kind": "raw",
      "p": "raw",
    },
  },
  Cmd: {
    fields: {
      "args": "raw",
      "kind": "raw",
      "note": "raw",
    },
    union: {
      disc: "kind",
      field: "args",
      branches: {
        "move": "CmdMove",
        "say": "CmdSay",
      },
    },
  },
  CmdMove: {
    fields: {
      "dx": "raw",
      "scores": "raw",
      "size": "raw",
    },
  },
  CmdSay: {
    fields: {
      "at": "raw",
      "mood": { t: "ThingBox" },
    },
  },
  DataBye: {
    fields: {},
  },
  DataHello: {
    fields: {
      "who": "raw",
    },
  },
  Envelope: {
    fields: {
      "data": "raw",
      "kind": "raw",
      "seq": "raw",
      "sid": "raw",
    },
  },
  Shape: {
    fields: {},
    union: {
      disc: "kind",
      branches: {
        "circle": "ShapeCircle",
        "square": "ShapeSquare",
      },
    },
  },
  ShapeCircle: {
    fields: {
      "kind": "raw",
      "r": "raw",
    },
  },
  ShapeSquare: {
    fields: {
      "at": "raw",
      "kind": "raw",
      "open": "raw",
      "side": "raw",
    },
  },
  StreamEvent: {
    fields: {
      "data": "raw",
      "kind": "raw",
      "seq": "raw",
      "sid": "raw",
    },
    union: {
      disc: "kind",
      field: "data",
      branches: {
        "bye": "DataBye",
        "hello": "DataHello",
      },
    },
  },
  Thing: {
    fields: {
      "at": "raw",
      "box": { t: "ThingBox" },
      "child": { t: "Thing" },
      "cmd": { t: "Cmd" },
      "color": "raw",
      "count": "raw",
      "extra": "raw",
      "fixed": "raw",
      "id": "raw",
      "maybe": "raw",
      "mode": "raw",
      "name": "raw",
      "num_or_str": "raw",
      "on": "raw",
      "open": "raw",
      "ratio": "raw",
      "raw": "raw",
      "rows": { list: { t: "ThingRowsItem" } },
      "scores": "raw",
      "shape": { t: "Shape" },
      "tags": "raw",
    },
  },
  ThingBox: {
    fields: {
      "h": "raw",
      "w": "raw",
    },
  },
  ThingRowsItem: {
    fields: {
      "label": "raw",
    },
  },
};
