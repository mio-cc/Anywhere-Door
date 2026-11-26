import { describe, expectTypeOf, test } from 'vitest';
import type * as gen from './generated/types';
import type { EventOf, FromWire, Graph, GraphRead, Node, NodeDef, OpOf } from './index';

describe('W.04.03 类型映射静态校验', () => {
  test('FromWire 与生成类型结构一致', () => {
    expectTypeOf<FromWire<gen.GraphRead>['registryHash']>().toEqualTypeOf<string>();
    expectTypeOf<GraphRead['registryHash']>().toEqualTypeOf<string>();
    expectTypeOf<Extract<keyof Graph, 'registryHash'>>().toEqualTypeOf<never>();
    expectTypeOf<FromWire<gen.Node>['params']>().toEqualTypeOf<
      { readonly [k: string]: unknown } | undefined
    >();
    expectTypeOf<NonNullable<Node['params']>>().toEqualTypeOf<{ readonly [k: string]: unknown }>();
    expectTypeOf<
      NonNullable<NonNullable<FromWire<gen.NodeDef>['params']>[string]['ui']>['showIf']
    >().toEqualTypeOf<string | undefined>();
    expectTypeOf<NonNullable<NonNullable<NodeDef['params']>[string]['ui']>['showIf']>().toEqualTypeOf<
      string | undefined
    >();
    expectTypeOf<EventOf<'node.state'>['kind']>().toEqualTypeOf<'node.state'>();
    expectTypeOf<EventOf<'node.state'>['data']['backoffMs']>().toEqualTypeOf<number | undefined>();
    expectTypeOf<EventOf<'node.state'>['data']['to']>().toEqualTypeOf<gen.NodeState>();
    expectTypeOf<OpOf<'set_param'>['kind']>().toEqualTypeOf<'set_param'>();
    expectTypeOf<OpOf<'set_param'>['payload']['node']>().toEqualTypeOf<gen.Ulid>();
  });
});
