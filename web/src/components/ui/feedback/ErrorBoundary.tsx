import { Component, type ErrorInfo, type ReactNode } from 'react';

export interface ErrorBoundaryProps {
  readonly fallback: (retry: () => void) => ReactNode;
  readonly children: ReactNode;
  readonly name: string;
  readonly resetKey?: unknown;
}

interface ErrorBoundaryState {
  readonly hasError: boolean;
  readonly error: unknown;
}

/** 全前端唯一的 class 组件：React 19 区域错误边界（W.08-R13、W.08.05 §6.8、P.09 K307）。 */
export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  constructor(props: ErrorBoundaryProps) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: unknown): ErrorBoundaryState {
    return { hasError: true, error };
  }

  override componentDidCatch(error: Error, info: ErrorInfo): void {
    console.error('[ErrorBoundary]', this.props.name, error, info.componentStack);
  }

  override componentDidUpdate(prevProps: ErrorBoundaryProps): void {
    if (this.state.hasError && !Object.is(prevProps.resetKey, this.props.resetKey)) {
      this.setState({ hasError: false, error: null });
    }
  }

  private readonly retry = (): void => {
    this.setState({ hasError: false, error: null });
  };

  override render(): ReactNode {
    if (this.state.hasError) {
      return this.props.fallback(this.retry);
    }
    return this.props.children;
  }
}
