// ESLint 自定义规则 hx/no-react-in-canvas-core（W.01 §4、W.12 §6.4、P.09 K179）：
// 画布引擎核心 canvas/core/** 不允许出现 JSX 或调用 use* hook（import 限制由 no-restricted-imports 负责）。

/** @type {import('eslint').Rule.RuleModule} */
export default {
  meta: {
    type: 'problem',
    docs: {
      description: '禁止在 canvas/core/** 中使用 React hook（use*）或 JSX（W-R1、G.06-R3）',
    },
    schema: [],
    messages: {
      noJsx: 'canvas/core/** 是纯 TypeScript 画布引擎，不能使用 JSX（W-R1）',
      noHook: 'canvas/core/** 是纯 TypeScript 画布引擎，不能调用 React hook "{{name}}"（W-R1）',
    },
  },
  create(context) {
    function isHookName(name) {
      return /^use[A-Z0-9_]/.test(name);
    }

    return {
      JSXElement(node) {
        context.report({ node, messageId: 'noJsx' });
      },
      JSXFragment(node) {
        context.report({ node, messageId: 'noJsx' });
      },
      CallExpression(node) {
        const callee = node.callee;
        if (callee.type === 'Identifier' && isHookName(callee.name)) {
          context.report({ node, messageId: 'noHook', data: { name: callee.name } });
          return;
        }
        if (
          callee.type === 'MemberExpression' &&
          !callee.computed &&
          callee.property.type === 'Identifier' &&
          isHookName(callee.property.name)
        ) {
          context.report({ node, messageId: 'noHook', data: { name: callee.property.name } });
        }
      },
    };
  },
};
