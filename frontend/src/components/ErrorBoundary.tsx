import React, { Component, ErrorInfo, ReactNode } from 'react';
import { AlertOctagon, RefreshCw, RotateCcw } from 'lucide-react';

interface Props {
  children: ReactNode;
  fallbackTitle?: string;
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

  private handleReset = () => {
    try {
      sessionStorage.clear();
    } catch {}
    window.location.reload();
  };

  public render() {
    if (this.state.hasError) {
      return (
        <div className="min-h-screen bg-zinc-950 text-zinc-100 flex items-center justify-center p-6 select-none">
          <div className="max-w-lg w-full bg-zinc-900/90 border border-red-500/40 rounded-2xl p-6 shadow-2xl backdrop-blur-sm space-y-4">
            <div className="flex items-center space-x-3">
              <div className="p-2.5 rounded-xl bg-red-500/10 border border-red-500/30 text-red-400">
                <AlertOctagon className="w-6 h-6" />
              </div>
              <div>
                <h2 className="text-base font-bold text-zinc-100">
                  {this.props.fallbackTitle || 'UI Runtime Exception Intercepted'}
                </h2>
                <p className="text-xs text-zinc-400">
                  The Sirius Cockpit encountered an unexpected interface error.
                </p>
              </div>
            </div>

            <div className="bg-zinc-950/80 border border-zinc-800 rounded-xl p-3.5 text-xs font-mono text-red-300 overflow-x-auto max-h-40">
              {this.state.error?.message || 'Unknown runtime error'}
            </div>

            <div className="flex items-center space-x-3 pt-2">
              <button
                onClick={this.handleReload}
                className="flex-1 py-2 px-4 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-white font-semibold text-xs shadow-lg shadow-indigo-600/20 transition-all flex items-center justify-center space-x-2"
              >
                <RefreshCw className="w-4 h-4" />
                <span>Reload Cockpit</span>
              </button>
              <button
                onClick={this.handleReset}
                className="py-2 px-4 rounded-xl bg-zinc-800 hover:bg-zinc-700 text-zinc-300 text-xs font-medium transition-colors flex items-center space-x-2"
                title="Clear temporary session state and reload"
              >
                <RotateCcw className="w-4 h-4" />
                <span>Reset State</span>
              </button>
            </div>
          </div>
        </div>
      );
    }

    return this.props.children;
  }
}
