import React, { useState } from 'react';
import { ArrowUpCircle, X, ShieldCheck, Sparkles } from 'lucide-react';
import { EngineUpdateStatus } from '../types';

interface EngineUpdateBannerProps {
  engineStatus: EngineUpdateStatus | null;
  onOpenModal: () => void;
}

export const EngineUpdateBanner: React.FC<EngineUpdateBannerProps> = ({
  engineStatus,
  onOpenModal,
}) => {
  const [dismissed, setDismissed] = useState(false);

  if (!engineStatus?.hasUpdate || dismissed) {
    return null;
  }

  const currentVer = engineStatus.currentVersion || 'v1.9.7';
  const targetVer = engineStatus.targetVersion || 'v1.9.8';

  return (
    <div className="bg-gradient-to-r from-indigo-950/80 via-purple-950/70 to-zinc-900 border-b border-indigo-500/40 px-4 py-3 shadow-lg relative animate-fadeIn">
      <div className="max-w-7xl mx-auto flex flex-col sm:flex-row items-center justify-between gap-3">
        <div className="flex items-center space-x-3 text-sm">
          <div className="w-8 h-8 rounded-lg bg-indigo-500/20 border border-indigo-400/40 flex items-center justify-center flex-shrink-0 animate-pulse">
            <Sparkles className="w-4 h-4 text-indigo-300" />
          </div>
          <div>
            <div className="flex items-center space-x-2">
              <span className="font-semibold text-zinc-100">
                Sirius Engine Update Available:
              </span>
              <span className="font-mono font-bold text-indigo-300 px-2 py-0.5 rounded bg-indigo-900/60 border border-indigo-700/60 text-xs">
                {targetVer}
              </span>
              <span className="text-xs text-zinc-400 hidden md:inline">
                (Current: {currentVer})
              </span>
              <span className="text-[11px] px-2 py-0.5 rounded-full bg-emerald-500/20 text-emerald-300 border border-emerald-500/30 font-medium hidden sm:inline">
                Protocol Compatible
              </span>
            </div>
            <p className="text-xs text-zinc-300/90 mt-0.5 line-clamp-1">
              {engineStatus.releaseNotes || 'Consensus performance enhancements, RocksDB memory cache optimizations, and fast-sync resilience improvements.'}
            </p>
          </div>
        </div>

        <div className="flex items-center space-x-2.5 flex-shrink-0">
          <button
            onClick={onOpenModal}
            className="px-3.5 py-1.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold shadow-md shadow-indigo-600/30 transition-all flex items-center space-x-1.5"
          >
            <ArrowUpCircle className="w-4 h-4" />
            <span>Review & Update Engine</span>
          </button>
          <button
            onClick={() => setDismissed(true)}
            className="p-1.5 text-zinc-400 hover:text-zinc-200 rounded-lg hover:bg-zinc-800 transition-colors"
            title="Dismiss notification"
          >
            <X className="w-4 h-4" />
          </button>
        </div>
      </div>
    </div>
  );
};
