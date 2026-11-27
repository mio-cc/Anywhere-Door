import '@/styles/tokens.css';
import '@/styles/themes.css';
import '@/styles/base.css';

import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { App } from './App';
import { keys } from './shell/keys';

// W.02.04 §7：全前端唯一的全局键盘监听器
keys.attach(window);

const rootEl = document.getElementById('root');
if (rootEl) {
  createRoot(rootEl).render(
    <StrictMode>
      <App />
    </StrictMode>,
  );
}
