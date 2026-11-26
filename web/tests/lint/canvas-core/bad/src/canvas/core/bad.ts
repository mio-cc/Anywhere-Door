import { useState } from 'react';
export function useBadCanvasHook(): number {
  const [n] = useState(0);
  return n;
}
