// ESLint 自定义规则 hx/no-bare-store（G.06.02-R3、W.12 §6.4、P.09 K179）：
// 禁止不带选择器直接调用 useXxxStore()，防止订阅整个 store 造成大面积重渲染。

/** @type {import('eslint').Rule.RuleModule} */
export default {
  meta: {
    type: 'problem',
    docs: {
      description: '禁止不带选择器调用 use*Store()（G.06.02-R3）',
    },
    schema: [],
    messages: {
      bareStore: '调用 {{name}}() 必须传入选择器函数，禁止订阅整个 store（G.06.02-R3）',
    },
  },
  create(context) {
    function isStoreHook(name) {
      return /^use[A-Z][A-Za-z0-9]*Store$/.test(name);
    }

    return {
      CallExpression(node) {
        if (node.callee.type !== 'Identifier') return;
        const name = node.callee.name;
        if (!isStoreHook(name)) return;
        if (node.arguments.length === 0) {
          context.report({ node, messageId: 'bareStore', data: { name } });
        }
      },
    };
  },
};
