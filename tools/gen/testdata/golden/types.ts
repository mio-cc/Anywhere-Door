/* 由 tools/gen 生成，请勿手改 */

// 来源：schema/*.schema.json（C.14）。字段名是 schema 的 snake_case；前端只经 lib/protocol 使用（W.04.04-R1）。

/** 什么都行 */
export type Anything = unknown;

/** 只用于校验的 if/then，不是联合 */
export interface Checked {
  kind: "p" | "q";
  p?: string;
}

export interface CmdCommon {
  note?: string;
}

export type Cmd = CmdCommon & (
  | { kind: "move"; args: CmdMove }
  | { kind: "say"; args: CmdSay }
);

export type CmdKind = "move" | "say";

/** CmdKind 的取值（C.14-R6） */
export const CmdKind = {
  Move: "move",
  Say: "say",
} as const;

export interface CmdMove {
  dx: number;
  scores?: Scores;
  size?: Size;
}

export interface CmdSay {
  at?: Timestamp;
  mood?: ThingBox;
}

/** 颜色 */
export type Color = "red" | "dark_blue" | "tool.L0";

/** Color 的取值（C.14-R6） */
export const Color = {
  Red: "red",
  DarkBlue: "dark_blue",
  ToolL0: "tool.L0",
} as const;

export interface DataBye {
}

export interface DataHello {
  who: string;
}

export interface Envelope {
  data: { [key: string]: unknown };
  kind: string;
  seq: number;
  sid?: Ulid;
}

export type Extra = { [key: string]: unknown };

export type Scores = { [key: string]: number };

export type Shape = ShapeCircle | ShapeSquare;

export interface ShapeCircle {
  kind: "circle";
  r: number;
}

export type ShapeKind = "circle" | "square";

/** ShapeKind 的取值（C.14-R6） */
export const ShapeKind = {
  Circle: "circle",
  Square: "square",
} as const;

export interface ShapeSquare {
  at?: Timestamp;
  kind: "square";
  open?: string;
  side: number;
}

export type Size = number;

export interface StreamEventCommon {
  seq: number;
  sid: Ulid;
}

export type StreamEvent = StreamEventCommon & (
  | { kind: "bye"; data: DataBye }
  | { kind: "hello"; data: DataHello }
);

export type StreamEventKind = "hello" | "bye";

export type Tags = string[];

export interface Thing {
  at?: Timestamp;
  box: ThingBox;
  child?: Thing;
  cmd?: Cmd;
  color?: Color;
  count?: number;
  extra?: Extra;
  fixed?: "x";
  id: Ulid;
  maybe: Ulid | null;
  mode?: "a" | "b";
  /** 名字 */
  name: string;
  num_or_str?: string | number;
  on?: boolean;
  open?: { [key: string]: unknown };
  ratio?: number;
  /** 什么都行 */
  raw?: unknown;
  rows?: ThingRowsItem[];
  scores?: { [key: string]: number };
  shape?: Shape;
  tags?: Tags;
}

/** 把没填的可选字段设为 schema 的默认值（C.14 §3）。 */
export function withDefaultsThing(v: Thing): Thing {
  return { color: "red", count: 3, mode: "a", on: false, ratio: 0.5, ...v };
}

export interface ThingBox {
  h?: number;
  w: number;
}

export interface ThingRowsItem {
  label?: string;
}

export type Timestamp = string;

export type Ulid = string & { readonly __brand: "Ulid" };
