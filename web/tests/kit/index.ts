export { act, render, type Rendered } from './render';
export { user, type PointerStep } from './user';
export { useFakeTime, type FakeTime } from './time';
export { fakeRaf, type FakeRaf } from './raf';
export {
  stubElementFromPoint,
  recordAnimate,
  fake2d,
  resize,
  type AnimateCall,
  type Canvas2dCall,
} from './dom';
export { expectConsole, type ConsoleLevel } from './console';
export { memStorage, loadVectors, loadExample, type VectorName } from './data';
export { fakeEnv, type FakeEnvController, type FakeOverlayRecord } from './shell';
