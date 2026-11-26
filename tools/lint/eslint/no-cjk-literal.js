// ESLint 自定义规则 hx/no-cjk-literal（G.06-R4、W.10 §6.5、W.12 §6.4、P.09 K173）：
// 禁止在前端业务源码里写中文字面量（Literal / TemplateElement / JSXText）；
// 按源码原文匹配，转义写法（如 '\uFFE5'）与注释不报错。

const CJK_RE = /[\u3400-\u9fff\u3000-\u303f\uff01-\uff5e]/;

/** @type {import('eslint').Rule.RuleModule} */
export default {
  meta: {
    type: 'problem',
    docs: {
      description: '禁止在组件与业务源码中直接写中文字面量，界面文案必须放入 lib/i18n/zh.ts（G.06-R4、W.10）',
    },
    schema: [],
    messages: {
      noCjk: '界面文案放进 lib/i18n/zh.ts（W.10）',
    },
  },
  create(context) {
    return {
      Literal(node) {
        if (typeof node.value !== 'string') return;
        if (typeof node.raw === 'string' && CJK_RE.test(node.raw)) {
          context.report({ node, messageId: 'noCjk' });
        }
      },
      TemplateElement(node) {
        if (typeof node.value?.raw === 'string' && CJK_RE.test(node.value.raw)) {
          context.report({ node, messageId: 'noCjk' });
        }
      },
      JSXText(node) {
        if (typeof node.raw === 'string' && CJK_RE.test(node.raw)) {
          context.report({ node, messageId: 'noCjk' });
        }
      },
    };
  },
};
