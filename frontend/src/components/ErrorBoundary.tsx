import React, { Component, ErrorInfo, ReactNode } from 'react';
import { AlertOctagon, RefreshCw, RotateCcw, X } from 'lucide-react';

interface Props {
  children: ReactNode;
  fallbackTitle?: string;
  onDismiss?: () => void;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

export class ErrorBoundary extends Component<Props, State> {
  public state: State = {
    hasError: false,
    error: null,
  };

  public static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  public componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('ErrorBoundary caught an unhandled error:', error, errorInfo);
  }

  private handleReload = () => {
    window.location.reload();
  };

  private handleDismiss = () => {
    try {
      if (this.props.fallbackTitle?.includes('WSL')) {
        sessionStorage.setItem('wsl_setup_dismissed', 'true');
      }
    } catch {}
    this.setState({ hasError: false, error: null });
    if (this.props.onDismiss) {
      this.props.onDismiss();
    }
  };

  private handleReset = () => {
    try {
      sessionStorage.clear();
    } catch {}
    this.setState({ hasError: false, error: null });
    window.location.reload();
  };

  public render() {
    if (this.state.hasError) {
      return (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm animate-fadeIn select-none">
          <div className="max-w-lg w-full bg-zinc-900 border border-red-500/40 rounded-2xl p-6 shadow-2xl space-y-4">
            <div className="flex items-center justify-between">
              <div className="flex items-center space-x-3">
                <div className="p-2.5 rounded-xl bg-red-500/10 border border-red-500/30 text-red-400">
                  <AlertOctagon className="w-5 h-5" />
                </div>
                <div>
                  <h2 className="text-sm font-bold text-zinc-100">
                    {this.props.fallbackTitle || 'UI Runtime Exception Intercepted'}
                  </h2>
                  <p className="text-xs text-zinc-400 mt-0.5">
                    An error occurred while rendering this interface module.
                  </p>
                </div>
              </div>
              <button
                onClick={this.handleDismiss}
                className="p-1.5 text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 rounded-lg transition-colors"
                title="Dismiss error"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <div className="bg-zinc-950/90 border border-zinc-800 rounded-xl p-3 text-xs font-mono text-red-300 overflow-x-auto max-h-36">
              {this.state.error?.message || 'Unknown runtime error'}
            </div>

            <div className="flex items-center space-x-3 pt-1">
              <button
                onClick={this.handleReload}
                className="flex-1 py-2 px-4 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-white font-semibold text-xs shadow-lg shadow-indigo-600/20 transition-all flex items-center justify-center space-x-2"
              >
                <RefreshCw className="w-3.5 h-3.5" />
                <span>Reload Cockpit</span>
              </button>
              <button
                onClick={this.handleDismiss}
                className="py-2 px-3 rounded-xl bg-zinc-800 hover:bg-zinc-700 text-zinc-300 text-xs font-medium transition-colors"
              >
                Dismiss
              </button>
              <button
                onClick={this.handleReset}
                className="py-2 px-3 rounded-xl bg-zinc-800 hover:bg-zinc-700 text-zinc-300 text-xs font-medium transition-colors flex items-center space-x-1.5"
                title="Clear temporary session state and reload"
              >
                <RotateCcw className="w-3.5 h-3.5" />
                <span>Reset</span>
              </button>
            </div>
          </div>
        </div>
      );
    }

    return this.props.children;
  }
}
