/** @type {import('stylelint').Config} */
export default {
  ignoreFiles: ['dist/**', 'node_modules/**', 'src/styles/tokens.css', 'src/styles/themes.css'],
  rules: {
    'color-no-hex': true,
    'declaration-no-important': true,
    'selector-max-id': 0,
    'selector-class-pattern': [
      '^(hx-[a-z0-9]+(__[a-z0-9-]+)?(--[a-z0-9-]+)?|is-[a-z0-9-]+|has-[a-z0-9-]+)$',
      {
        message: '类名必须符合 G.04.06（hx-块__元素--变体 或 is-/has- 状态类）',
      },
    ],
    'custom-property-pattern': [
      '^(_|hx-)[a-z0-9-]+$',
      {
        message: '自定义属性必须以 --hx-（设计令牌）或 --_（组件局部变量）开头（G.04.06）',
      },
    ],
    'media-feature-name-disallowed-list': [
      ['width', 'max-width', 'min-width'],
      {
        message: '响应式断点请使用 .hx-app[data-bp=…]，不要写宽度媒体查询（W.02.02 §4.3）',
      },
    ],
    'declaration-property-value-disallowed-list': [
      {
        '/.*/': ['/(?:rgb|rgba|hsl|hsla|hwb|lab|lch|oklab|oklch|color|color-mix)\\(/i'],
        'font-size': ['/^\\d+(?:\\.\\d+)?(?:px|rem|em)$/i'],
        'border-radius': ['/^(?!0$|50%$|9999px$)\\d+(?:\\.\\d+)?(?:px|rem|em)$/i'],
        'z-index': ['/^-?\\d+$/'],
        'transition-duration': ['/^(?!0s$)\\d+(?:\\.\\d+)?m?s$/i'],
      },
      {
        message: '颜色、字号、圆角、层级与动效时长只能使用 var(--hx-*) 设计令牌（G.06.03-R1）',
      },
    ],
    'declaration-property-value-allowed-list': {
      'transition-property': [
        'none',
        'transform',
        'opacity',
        'color',
        'background-color',
        'border-color',
        'outline-color',
        'box-shadow',
        'fill',
        'stroke',
        '/^(transform|opacity|color|background-color|border-color|outline-color|box-shadow|fill|stroke)(,\\s*(transform|opacity|color|background-color|border-color|outline-color|box-shadow|fill|stroke))*$/',
      ],
    },
  },
};
