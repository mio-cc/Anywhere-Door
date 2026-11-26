// 本地 ESLint 插件 hx（G.06.04 §1、W.12 §6.4、P.09 K173）
import noBareStore from './no-bare-store.js';
import noCjkLiteral from './no-cjk-literal.js';
import noReactInCanvasCore from './no-react-in-canvas-core.js';

export default {
  meta: {
    name: 'eslint-plugin-hx',
    version: '0.1.0',
  },
  rules: {
    'no-react-in-canvas-core': noReactInCanvasCore,
    'no-bare-store': noBareStore,
    'no-cjk-literal': noCjkLiteral,
  },
};
