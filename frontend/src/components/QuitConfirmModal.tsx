import React, { useState } from 'react';
import { Power, Minimize2, X, RotateCw } from 'lucide-react';
import { SiriusLogo } from './SiriusLogo';

interface QuitConfirmModalProps {
  isOpen: boolean;
  onClose: () => void;
  onStopEverything: () => void;
  onKeepBackground: () => void;
}

export const QuitConfirmModal: React.FC<QuitConfirmModalProps> = ({
  isOpen,
  onClose,
  onStopEverything,
  onKeepBackground,
}) => {
  const [isShuttingDown, setIsShuttingDown] = useState(false);
  const [shutdownStep, setShutdownStep] = useState(0);

  if (!isOpen) return null;

  const handleFullShutdown = () => {
    setIsShuttingDown(true);
    setShutdownStep(1);
    
    setTimeout(() => setShutdownStep(2), 400);
    setTimeout(() => setShutdownStep(3), 800);
    setTimeout(() => {
      onStopEverything();
    }, 1200);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-xs animate-fadeIn select-none">
      <div className="bg-[#181B20] border border-[#262B34] rounded-xl max-w-xl w-full shadow-2xl overflow-hidden flex flex-col text-slate-200">
        
        {/* Header */}
        <div className="p-4 sm:p-5 border-b border-[#262B34] flex items-center justify-between bg-[#0F1115]">
          <div className="flex items-center space-x-3">
            <div className="p-2 bg-[#181B20] text-slate-300 rounded-lg border border-[#262B34]">
              <SiriusLogo size={20} />
            </div>
            <div>
              <div className="flex items-center space-x-2">
                <h3 className="text-sm font-semibold text-white tracking-tight">Exit ProximaX Sirius Node</h3>
                <span className="text-[10px] font-mono bg-slate-800 text-slate-400 px-1.5 py-0.5 rounded border border-slate-700/50">
                  MAINNET
                </span>
              </div>
              <p className="text-[11px] text-slate-400 mt-0.5">
                Choose your preferred operational state before exiting.
              </p>
            </div>
          </div>
          {!isShuttingDown && (
            <button
              onClick={onClose}
              className="p-1.5 text-slate-400 hover:text-white rounded hover:bg-[#262B34] transition-colors"
            >
              <X className="w-4 h-4" />
            </button>
          )}
        </div>

        {/* Content Body */}
        <div className="p-5 space-y-3">
          {isShuttingDown ? (
            <div className="py-8 px-4 flex flex-col items-center justify-center space-y-4 text-center">
              <div className="flex items-center justify-center w-12 h-12 rounded-xl bg-[#0F1115] border border-[#262B34] text-blue-400">
                <RotateCw className="w-6 h-6 animate-spin text-blue-400" />
              </div>
              <div className="space-y-1">
                <h4 className="text-sm font-semibold text-white tracking-tight">Stopping Sirius Node Gracefully</h4>
                <p className="text-xs text-slate-400 font-mono">
                  {shutdownStep === 1 && '1/3 Flushing state cache to disk...'}
                  {shutdownStep === 2 && '2/3 Terminating Catapult consensus engine...'}
                  {shutdownStep === 3 && '3/3 Releasing port 3080 & supervisor...'}
                </p>
              </div>
              <div className="w-64 bg-[#0F1115] border border-[#262B34] rounded-full h-1.5 overflow-hidden">
                <div
                  className="h-full bg-blue-500 transition-all duration-300 rounded-full"
                  style={{ width: `${(shutdownStep / 3) * 100}%` }}
                />
              </div>
            </div>
          ) : (
            <>
              {/* Option 1: Full Shutdown */}
              <div
                onClick={handleFullShutdown}
                className="group relative p-4 rounded-lg bg-[#0F1115] border border-[#262B34] hover:border-rose-500/50 hover:bg-rose-950/20 transition-all cursor-pointer flex items-start space-x-3.5"
              >
                <div className="p-2 rounded-md bg-rose-500/10 text-rose-400 border border-rose-500/20 group-hover:bg-rose-500/20 group-hover:text-rose-300 transition-colors flex-shrink-0 mt-0.5">
                  <Power className="w-4 h-4" />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center justify-between">
                    <h4 className="text-xs font-semibold text-white group-hover:text-rose-300 transition-colors">
                      Stop Everything (Full Shutdown)
                    </h4>
                    <span className="text-[10px] font-mono text-rose-400 bg-rose-950/40 px-2 py-0.5 rounded border border-rose-800/40">
                      Complete Exit
                    </span>
                  </div>
                  <p className="text-[11px] text-slate-400 mt-1 leading-relaxed">
                    Safely flushes the RocksDB blockchain database to disk, terminates the Catapult validator engine, and frees port <code className="text-slate-300 font-mono">3080</code>.
                  </p>
                </div>
              </div>

              {/* Option 2: Keep Running in Background */}
              <div
                onClick={onKeepBackground}
                className="group relative p-4 rounded-lg bg-[#0F1115] border border-[#262B34] hover:border-blue-500/50 hover:bg-[#1E2228] transition-all cursor-pointer flex items-start space-x-3.5"
              >
                <div className="p-2 rounded-md bg-blue-500/10 text-blue-400 border border-blue-500/20 group-hover:bg-blue-500/20 transition-colors flex-shrink-0 mt-0.5">
                  <Minimize2 className="w-4 h-4" />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center justify-between">
                    <h4 className="text-xs font-semibold text-white group-hover:text-blue-300 transition-colors">
                      Keep Node Running in Background
                    </h4>
                    <span className="text-[10px] font-mono text-emerald-400 bg-emerald-950/40 px-2 py-0.5 rounded border border-emerald-800/40">
                      24/7 Harvesting
                    </span>
                  </div>
                  <p className="text-[11px] text-slate-400 mt-1 leading-relaxed">
                    Closes the window while the node continues harvesting and syncing in the system tray / background daemon. The web console remains accessible at <code className="text-blue-400 font-mono">http://localhost:3080</code>.
                  </p>
                </div>
              </div>
            </>
          )}
        </div>

        {/* Footer */}
        {!isShuttingDown && (
          <div className="p-3 bg-[#0F1115] border-t border-[#262B34] flex justify-end items-center space-x-2 text-xs">
            <button
              onClick={onClose}
              className="px-3.5 py-1.5 bg-[#181B20] hover:bg-[#262B34] text-slate-300 hover:text-white rounded font-medium border border-[#262B34] transition-colors"
            >
              Cancel &amp; Keep Open
            </button>
          </div>
        )}

      </div>
    </div>
  );
};
