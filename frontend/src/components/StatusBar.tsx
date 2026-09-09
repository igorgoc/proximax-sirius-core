import React, { useState, useRef, useEffect } from 'react';
import { X } from 'lucide-react';
import { NodeMetrics, NodeConfig } from '../types';
import { formatNumber } from '../utils/format';

interface StatusBarProps {
  metrics: NodeMetrics | null;
  config: NodeConfig | null;
  onOpenWizard: () => void;
  onOpenAbout: () => void;
}

export const StatusBar: React.FC<StatusBarProps> = ({
  metrics,
  config,
  onOpenAbout
}) => {
  const [popoverOpen, setPopoverOpen] = useState(false);
  const popoverRef = useRef<HTMLDivElement>(null);

  const isRunning = metrics?.status === 'running';
  const blockHeight = metrics?.blockHeight || 0;
  const networkHeight = metrics?.networkHeight || 0;
  const peersCount = metrics?.peersCount || 0;

  const isSynced = isRunning && networkHeight > 0 && (networkHeight - blockHeight <= 2);

  // Close popover when clicking outside or pressing Escape
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (popoverRef.current && !popoverRef.current.contains(event.target as Node)) {
        setPopoverOpen(false);
      }
    };
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setPopoverOpen(false);
    };

    if (popoverOpen) {
      document.addEventListener('mousedown', handleClickOutside);
      window.addEventListener('keydown', handleKeyDown);
    }
    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [popoverOpen]);

  const truncate = (str?: string, front = 14, back = 10) => {
    if (!str) return '—';
    if (str.length <= front + back) return str;
    return `${str.slice(0, front)}...${str.slice(-back)}`;
  };

  return (
    <footer className="bg-[#0F1115] border-t border-[#262B34] text-slate-400 text-xs py-2 px-4 select-none flex-shrink-0">
      <div className="max-w-5xl mx-auto flex items-center justify-between">
        
        {/* Left: Minimal Connection Status Dot, Height & Hardware Stats */}
        <div className="flex items-center space-x-3 sm:space-x-4 min-w-0">
          <div className="flex items-center space-x-2 flex-shrink-0">
            <span
              className={`w-2 h-2 rounded-full ${
                isRunning
                  ? isSynced
                    ? 'bg-emerald-400 shadow-[0_0_6px_#34d399]'
                    : 'bg-blue-400 animate-pulse'
                  : 'bg-slate-600'
              }`}
            />
            <span className="text-slate-300 font-medium font-sans">
              {isRunning
                ? isSynced
                  ? 'Synchronized'
                  : `Syncing (${formatNumber(blockHeight)} / ${formatNumber(networkHeight)})`
                : 'Offline'}
            </span>
          </div>

          <div className="hidden sm:flex items-center space-x-1.5 text-slate-500 font-mono text-[11px] flex-shrink-0">
            <span>•</span>
            <span>{peersCount} peers</span>
          </div>

          {/* System Resource Stats Segment (Clickable with Telemetry Popover) */}
          {isRunning && (
            <div className="relative flex-shrink-0" ref={popoverRef}>
              <button
                type="button"
                onClick={() => setPopoverOpen(!popoverOpen)}
                className="hidden md:flex items-center space-x-2 text-slate-500 hover:text-slate-200 font-mono text-[11px] cursor-pointer transition-colors px-1.5 py-0.5 rounded hover:bg-[#181B20] focus:outline-none"
                title="Click for detailed system resource telemetry"
              >
                <span>•</span>
                <span>CPU {metrics?.cpuPercent || '0%'}</span>
                <span>•</span>
                <span>RAM {metrics?.memoryUsage || '0 MB'}</span>
                <span>•</span>
                <span>Disk Free {metrics?.diskFree || '0 B'}</span>
                {metrics?.uptime && (
                  <>
                    <span>•</span>
                    <span>Up {metrics.uptime}</span>
                  </>
                )}
              </button>

              {/* Detail Telemetry Popover */}
              {popoverOpen && (
                <div className="absolute bottom-full left-0 mb-2.5 w-80 bg-[#181B20] border border-[#262B34] rounded-lg shadow-2xl p-4 text-xs font-sans select-none z-50 animate-in fade-in zoom-in-95 duration-100">
                  {/* Popover Header */}
                  <div className="flex items-center justify-between pb-2 mb-3 border-b border-[#262B34]/80">
                    <div className="flex items-center space-x-2">
                      <span className="w-1.5 h-1.5 rounded-full bg-emerald-400" />
                      <span className="font-semibold text-slate-200 text-xs">
                        System Resource Telemetry
                      </span>
                    </div>
                    <button
                      type="button"
                      onClick={(e) => {
                        e.stopPropagation();
                        setPopoverOpen(false);
                      }}
                      className="text-slate-500 hover:text-slate-300 p-0.5 rounded transition-colors"
                    >
                      <X className="w-3.5 h-3.5" />
                    </button>
                  </div>

                  {/* Telemetry Key-Value Grid */}
                  <div className="space-y-2 font-mono text-[11px]">
                    <div className="flex items-center justify-between">
                      <span className="text-slate-500 font-sans">Process Engine</span>
                      <span className="text-slate-200 font-sans">{metrics?.containerId || 'Native Sirius Core'}</span>
                    </div>

                    <div className="flex items-center justify-between">
                      <span className="text-slate-500 font-sans">Uptime</span>
                      <span className="text-slate-200">{metrics?.uptime || '0s'}</span>
                    </div>

                    <div className="flex items-center justify-between">
                      <span className="text-slate-500 font-sans">Active Threads</span>
                      <span className="text-slate-200">
                        {metrics?.threadsCount ? `${metrics.threadsCount} threads` : '—'}
                      </span>
                    </div>

                    <div className="flex items-center justify-between">
                      <span className="text-slate-500 font-sans">CPU Utilization</span>
                      <span className="text-slate-200">{metrics?.cpuPercent || '0.0%'}</span>
                    </div>

                    <div className="flex items-center justify-between">
                      <span className="text-slate-500 font-sans">RAM Usage</span>
                      <span className="text-slate-200">{metrics?.memoryUsage || '0 MB'}</span>
                    </div>

                    <div className="flex items-center justify-between">
                      <span className="text-slate-500 font-sans">Disk Free Space</span>
                      <span className="text-emerald-400">{metrics?.diskFree || '0 B'}</span>
                    </div>

                    {metrics?.diskUsage && (
                      <div className="flex items-center justify-between">
                        <span className="text-slate-500 font-sans">Disk Allocation</span>
                        <span className="text-slate-300 text-[10px]">{metrics.diskUsage}</span>
                      </div>
                    )}

                    <div className="flex items-center justify-between">
                      <span className="text-slate-500 font-sans">Disk I/O Rate</span>
                      <span className="text-slate-200">{metrics?.diskRate || 'Normal (RocksDB WAL)'}</span>
                    </div>

                    <div className="flex items-center justify-between">
                      <span className="text-slate-500 font-sans">Network I/O Rate</span>
                      <span className="text-slate-200">{metrics?.networkRate || 'Active P2P Mesh'}</span>
                    </div>
                  </div>

                  {/* Footer Meta */}
                  <div className="mt-2.5 pt-2 border-t border-[#262B34]/60 text-[10px] font-sans text-slate-500 flex items-center justify-between">
                    <span>Path: {config?.dataPath ? truncate(config.dataPath, 14, 10) : './chainconfig/data'}</span>
                    <span>P2P Port :{config?.port || 7900}</span>
                  </div>
                </div>
              )}
            </div>
          )}
        </div>

        {/* Right: Version & Quick Links */}
        <div className="flex items-center space-x-3 text-[11px] text-slate-500 flex-shrink-0">
          <button
            onClick={onOpenAbout}
            className="hover:text-slate-300 transition-colors"
          >
            {metrics?.image || 'Sirius Core v1.9.7'}
          </button>
        </div>

      </div>
    </footer>
  );
};
