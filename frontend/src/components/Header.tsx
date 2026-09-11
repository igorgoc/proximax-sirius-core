import React from 'react';
import { Play, Square, RotateCw, HelpCircle } from 'lucide-react';
import { NodeMetrics, NodeConfig } from '../types';
import { SiriusLogo } from './SiriusLogo';

interface HeaderProps {
  metrics: NodeMetrics | null;
  config: NodeConfig | null;
  darkMode: boolean;
  setDarkMode: (val: boolean) => void;
  onStart: () => void;
  onStop: () => void;
  onRestart: () => void;
  onOpenWizard: () => void;
  onOpenAbout: () => void;
  setActiveTab: (tab: string) => void;
  loading: boolean;
}

export const Header: React.FC<HeaderProps> = ({
  metrics,
  config,
  onStart,
  onStop,
  onRestart,
  onOpenAbout,
  loading,
}) => {
  const isRunning = metrics?.status === 'running';
  const isStarting = metrics?.status === 'starting';
  const hasHarvestKey = Boolean(config?.hasHarvestKey);
  const isElectron = typeof window !== 'undefined' && (
    navigator.userAgent.includes('Electron') ||
    Boolean((window as any).process?.versions?.electron)
  );

  return (
    <header className="bg-[#0F1115] border-b border-[#262B34] text-slate-200 select-none flex-shrink-0 app-drag">
      <div className="max-w-5xl mx-auto px-4 py-3 flex items-center justify-between">
        
        {/* Left: Clean Brand Identity */}
        <div className={`flex items-center space-x-3 ${isElectron ? 'pl-20' : ''}`}>
          <div className="flex items-center space-x-2.5">
            <SiriusLogo size={20} showBadge={false} />
            <div className="flex items-center space-x-2">
              <span className="font-semibold text-sm text-white tracking-tight">Sirius Core</span>
              <span className="text-[10px] font-mono text-slate-400 bg-[#181B20] px-1.5 py-0.5 rounded border border-[#262B34]">
                Mainnet
              </span>
            </div>
          </div>
        </div>

        {/* Right: Power Controls & System Actions */}
        <div className="flex items-center space-x-3 app-no-drag">
          {!isRunning ? (
            <button
              disabled={loading || isStarting || !hasHarvestKey}
              onClick={onStart}
              title={!hasHarvestKey ? 'A valid harvest key is mandatory before starting the node' : undefined}
              className="flex items-center space-x-1.5 px-3 py-1.5 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded-md text-xs font-medium transition-colors"
            >
              <Play className="w-3.5 h-3.5 fill-current" />
              <span>{isStarting ? 'Starting...' : 'Start Node'}</span>
            </button>
          ) : (
            <div className="flex items-center space-x-2">
              <button
                disabled={loading}
                onClick={onRestart}
                className="p-1.5 text-slate-400 hover:text-white hover:bg-[#181B20] rounded-md transition-colors"
                title="Restart Node"
              >
                <RotateCw className="w-3.5 h-3.5" />
              </button>
              <button
                disabled={loading}
                onClick={onStop}
                className="flex items-center space-x-1.5 px-3 py-1.5 bg-rose-950/60 hover:bg-rose-900/80 border border-rose-800/40 text-rose-300 rounded-md text-xs font-medium transition-colors"
              >
                <Square className="w-3.5 h-3.5 fill-current" />
                <span>Stop</span>
              </button>
            </div>
          )}

          <button
            onClick={onOpenAbout}
            className="p-1.5 text-slate-400 hover:text-white hover:bg-[#181B20] rounded-md transition-colors"
            title="About Sirius Core"
          >
            <HelpCircle className="w-4 h-4" />
          </button>
        </div>

      </div>
    </header>
  );
};
