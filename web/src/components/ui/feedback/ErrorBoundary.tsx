import { Component, type ErrorInfo, type ReactNode } from 'react';

export interface ErrorBoundaryProps {
  readonly fallback: (retry: () => void) => ReactNode;
  readonly children: ReactNode;
  readonly name: string;
  readonly resetKey?: unknown;
}

interface ErrorBoundaryState {
  readonly error: unknown;
}

/** 全前端唯一的 class 组件：React 19 区域错误边界（W.08-R13、W.08.05 §6.8）。 */
export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  constructor(props: ErrorBoundaryProps) {
    super(props);
    this.state = { error: null };
  }

  static getDerivedStateFromError(error: unknown): ErrorBoundaryState {
    return { error };
  }

  override componentDidCatch(error: Error, info: ErrorInfo): void {
    console.error('[ErrorBoundary]', this.props.name, error, info.componentStack);
  }

  override componentDidUpdate(prevProps: ErrorBoundaryProps): void {
    if (this.state.error !== null && !Object.is(prevProps.resetKey, this.props.resetKey)) {
      this.setState({ error: null });
    }
  }

  private readonly retry = (): void => {
    this.setState({ error: null });
  };

  override render(): ReactNode {
    if (this.state.error !== null) {
      return this.props.fallback(this.retry);
    }
    return this.props.children;
  }
}
