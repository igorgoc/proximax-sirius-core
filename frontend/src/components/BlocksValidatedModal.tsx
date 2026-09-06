import React, { useState, useMemo } from 'react';
import {
  X,
  Award,
  Coins,
  ExternalLink,
  Search,
  Check,
  Copy,
  RotateCcw
} from 'lucide-react';
import { HarvestStats } from '../types';
import { formatToLocalTime, formatRelativeTime, parseTimestamp } from '../utils/date';
import { getExplorerBlockUrl } from '../utils/explorer';
import { formatNumber } from '../utils/format';

interface BlocksValidatedModalProps {
  isOpen: boolean;
  onClose: () => void;
  harvestStats: HarvestStats | null;
  onReset?: () => void;
  isRunning?: boolean;
}

export const BlocksValidatedModal: React.FC<BlocksValidatedModalProps> = ({
  isOpen,
  onClose,
  harvestStats,
  onReset,
  isRunning = true
}) => {
  const [copiedHash, setCopiedHash] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [resetMenuOpen, setResetMenuOpen] = useState(false);
  const [resettingTarget, setResettingTarget] = useState<string | null>(null);
  const [confirmTarget, setConfirmTarget] = useState<string | null>(null);

  const handleReset = async (target: 'blocks' | 'fees' | 'all') => {
    if (confirmTarget !== target) {
      setConfirmTarget(target);
      setTimeout(() => setConfirmTarget(null), 4000);
      return;
    }

    setResettingTarget(target);
    try {
      const res = await fetch(`/api/harvesting/reset?target=${target}`, { method: 'POST' });
      if (res.ok) {
        setConfirmTarget(null);
        setResetMenuOpen(false);
        onReset?.();
      }
    } catch (e) {
      console.error(e);
    } finally {
      setResettingTarget(null);
    }
  };

  const validatedBlocks = harvestStats?.validatedBlocks || [];
  const totalAllTime = harvestStats?.totalBlocksValidated || validatedBlocks.length;
  const totalFeesXPX = harvestStats?.totalEarnedFeesXPX || 0;

  // Senior Data Analyst Metrics
  const { dailyBlockVelocity, latestBlock } = useMemo(() => {
    if (!validatedBlocks || validatedBlocks.length === 0) {
      return { avgCadenceHours: null, dailyBlockVelocity: null, avgFeePerBlock: '0.000', latestBlock: null };
    }

    const latest = validatedBlocks[0];
    let intervalsSumMs = 0;
    let intervalsCount = 0;

    // Calculate moving average interval over recent validated blocks
    for (let i = 0; i < Math.min(validatedBlocks.length - 1, 20); i++) {
      const tCurrent = parseTimestamp(validatedBlocks[i].timestamp)?.getTime() || 0;
      const tPrev = parseTimestamp(validatedBlocks[i + 1].timestamp)?.getTime() || 0;
      if (tCurrent > 0 && tPrev > 0 && tCurrent > tPrev) {
        const diff = tCurrent - tPrev;
        if (diff < 48 * 3600 * 1000) {
          intervalsSumMs += diff;
          intervalsCount++;
        }
      }
    }

    let avgHours = null;
    let velocity: number | null = null;
    if (intervalsCount > 0) {
      const avgMs = intervalsSumMs / intervalsCount;
      avgHours = (avgMs / (1000 * 3600)).toFixed(1);
      velocity = Math.round(24 / (avgMs / (1000 * 3600)));
    }

    const avgFee = (totalFeesXPX / (totalAllTime || 1)).toFixed(3);

    return {
      avgCadenceHours: avgHours,
      dailyBlockVelocity: velocity,
      avgFeePerBlock: avgFee,
      latestBlock: latest
    };
  }, [validatedBlocks, totalFeesXPX, totalAllTime]);

  // Filtered blocks based on search
  const filteredBlocks = useMemo(() => {
    if (!searchQuery.trim()) return validatedBlocks;
    const q = searchQuery.toLowerCase().trim();
    return validatedBlocks.filter(b => 
      b.height.toString().includes(q) || 
      (b.hash && b.hash.toLowerCase().includes(q))
    );
  }, [validatedBlocks, searchQuery]);

  const handleCopyHash = (hash: string) => {
    navigator.clipboard.writeText(hash);
    setCopiedHash(hash);
    setTimeout(() => setCopiedHash(null), 2000);
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-3 sm:p-5 bg-black/80 backdrop-blur-xs overflow-y-auto animate-fadeIn select-none">
      <div className="bg-[#181B20] border border-[#262B34] rounded-xl max-w-3xl w-full shadow-2xl overflow-hidden flex flex-col max-h-[90vh] my-auto text-slate-200">
        {/* Modal Header */}
        <div className="p-4 sm:p-5 border-b border-[#262B34] flex items-center justify-between bg-[#0F1115] flex-shrink-0">
          <div className="flex items-center space-x-3">
            <div className="p-1.5 bg-[#181B20] text-slate-300 rounded-md border border-[#262B34]">
              <Award className="w-4 h-4" />
            </div>
            <div>
              <div className="flex items-center space-x-2">
                <h3 className="text-sm font-semibold text-white tracking-tight">
                  Validated Blocks &amp; Harvesting Stats
                </h3>
                <span className={`text-[10px] font-mono px-2 py-0.5 rounded border ${
                  isRunning
                    ? 'bg-slate-800 text-slate-400 border-slate-700/50'
                    : 'bg-slate-800 text-slate-500 border-slate-700/30'
                }`}>
                  {isRunning ? 'POS+ Validator' : 'Node Offline'}
                </span>
              </div>
              <p className="text-[11px] text-slate-400 mt-0.5">
                On-chain validation metrics and historical signature log
              </p>
            </div>
          </div>
          <div className="flex items-center space-x-2 relative">
            {/* Reset Submenu Dropdown */}
            <div className="relative">
              <button
                type="button"
                onClick={() => setResetMenuOpen(!resetMenuOpen)}
                className="px-2.5 py-1.5 rounded-md text-xs font-mono font-medium bg-[#181B20] hover:bg-[#262B34] text-slate-300 hover:text-white border border-[#262B34] transition-colors flex items-center space-x-1.5"
                title="Open reset submenu"
              >
                <RotateCcw className="w-3.5 h-3.5 text-slate-400" />
                <span>Reset Options ▾</span>
              </button>

              {resetMenuOpen && (
                <>
                  <div
                    className="fixed inset-0 z-10"
                    onClick={() => {
                      setResetMenuOpen(false);
                      setConfirmTarget(null);
                    }}
                  />
                  <div className="absolute right-0 mt-1.5 w-64 bg-[#181B20] border border-[#262B34] rounded-lg shadow-xl z-20 p-1.5 space-y-1 text-xs">
                    <div className="px-2 py-1 text-[10px] uppercase font-medium text-slate-400 border-b border-[#262B34]">
                      Metric Reset Actions
                    </div>

                    {/* Action 1: Reset Blocks Validated */}
                    <button
                      type="button"
                      onClick={() => handleReset('blocks')}
                      disabled={!!resettingTarget}
                      className={`w-full text-left px-2.5 py-1.5 rounded font-mono transition-colors flex items-center justify-between ${
                        confirmTarget === 'blocks'
                          ? 'bg-rose-950/60 text-rose-300 border border-rose-800/60'
                          : 'text-slate-300 hover:bg-[#262B34] hover:text-white'
                      }`}
                    >
                      <div className="flex items-center space-x-2">
                        <Award className="w-3.5 h-3.5 text-slate-400" />
                        <span>{confirmTarget === 'blocks' ? 'Confirm Reset Blocks?' : 'Reset Blocks to 0'}</span>
                      </div>
                      {resettingTarget === 'blocks' && <RotateCcw className="w-3 h-3 animate-spin" />}
                    </button>

                    {/* Action 2: Reset Earned Fees */}
                    <button
                      type="button"
                      onClick={() => handleReset('fees')}
                      disabled={!!resettingTarget}
                      className={`w-full text-left px-2.5 py-1.5 rounded font-mono transition-colors flex items-center justify-between ${
                        confirmTarget === 'fees'
                          ? 'bg-rose-950/60 text-rose-300 border border-rose-800/60'
                          : 'text-slate-300 hover:bg-[#262B34] hover:text-white'
                      }`}
                    >
                      <div className="flex items-center space-x-2">
                        <Coins className="w-3.5 h-3.5 text-slate-400" />
                        <span>{confirmTarget === 'fees' ? 'Confirm Reset Fees?' : 'Reset Fees to 0'}</span>
                      </div>
                      {resettingTarget === 'fees' && <RotateCcw className="w-3 h-3 animate-spin" />}
                    </button>

                    {/* Action 3: Reset Both */}
                    <button
                      type="button"
                      onClick={() => handleReset('all')}
                      disabled={!!resettingTarget}
                      className={`w-full text-left px-2.5 py-1.5 rounded font-mono transition-colors flex items-center justify-between ${
                        confirmTarget === 'all'
                          ? 'bg-rose-950/80 text-rose-200 border border-rose-700/60'
                          : 'text-slate-400 hover:bg-[#262B34] hover:text-rose-400'
                      }`}
                    >
                      <div className="flex items-center space-x-2">
                        <RotateCcw className="w-3.5 h-3.5 text-rose-400" />
                        <span>{confirmTarget === 'all' ? 'Confirm Reset All?' : 'Reset All Metrics'}</span>
                      </div>
                      {resettingTarget === 'all' && <RotateCcw className="w-3 h-3 animate-spin" />}
                    </button>
                  </div>
                </>
              )}
            </div>

            <button
              onClick={onClose}
              className="p-1.5 text-slate-400 hover:text-white rounded-md hover:bg-[#262B34] transition-colors"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>

        {/* Modal Content */}
        <div className="p-5 overflow-y-auto space-y-4 flex-1 scrollbar-thin">
          
          {/* 1. Flattened Summary Hero (Single Card with Hairline Dividers, Bitcoin Core Style) */}
          <section className="bg-[#181B20] border border-[#262B34] rounded-lg p-5">
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-5 items-center">
              
              {/* Stat 1: Fees Earned (Primary Cobalt/Emerald Accent Number) */}
              <div>
                <span className="text-[11px] font-medium tracking-wider uppercase text-slate-400 block">
                  Fees Earned
                </span>
                <div className="mt-1.5 flex items-baseline space-x-1.5">
                  <span className="text-2xl sm:text-3xl font-semibold tracking-tight text-emerald-400 font-mono">
                    +{formatNumber(totalFeesXPX, 3)}
                  </span>
                  <span className="text-xs font-medium text-slate-400 font-sans">XPX</span>
                </div>
                <span className="text-[11px] text-slate-500 block mt-0.5">
                  Total block harvesting rewards
                </span>
              </div>

              {/* Stat 2: Avg Blocks / Day (Muted Velocity) */}
              <div className="sm:border-l sm:border-[#262B34] sm:pl-5">
                <span className="text-[11px] font-medium tracking-wider uppercase text-slate-400 block">
                  Avg Blocks / Day
                </span>
                <div className="mt-1.5 flex items-baseline space-x-1.5">
                  <span className="text-2xl sm:text-3xl font-semibold tracking-tight text-slate-200 font-mono">
                    {dailyBlockVelocity !== null ? dailyBlockVelocity.toLocaleString() : '—'}
                  </span>
                  <span className="text-xs font-medium text-slate-400 font-sans">blocks/day</span>
                </div>
                <span className="text-[11px] text-slate-500 block mt-0.5">
                  {isRunning ? 'Estimated validation velocity' : 'Historical velocity (paused)'}
                </span>
              </div>

              {/* Stat 3: Last Validated Block */}
              <div className="sm:border-l sm:border-[#262B34] sm:pl-5">
                <span className="text-[11px] font-medium tracking-wider uppercase text-slate-400 block">
                  {isRunning ? 'Last Validated Block' : 'Last Known Validated Block'}
                </span>
                {latestBlock ? (
                  <div className="mt-1.5 space-y-0.5">
                    <div className="flex items-center space-x-2 font-mono">
                      <a
                        href={getExplorerBlockUrl(latestBlock.height)}
                        target="_blank"
                        rel="noreferrer"
                        className="text-sm font-semibold text-blue-400 hover:underline flex items-center gap-1"
                      >
                        <span>#{latestBlock.height.toLocaleString()}</span>
                        <ExternalLink className="w-3 h-3 text-slate-500" />
                      </a>
                      <span className="text-xs font-medium text-emerald-400">
                        (+{latestBlock.feeXPX.toFixed(4)} XPX)
                      </span>
                    </div>
                    <span className="text-[11px] text-slate-400 block">
                      {formatRelativeTime(latestBlock.timestamp)} • {formatToLocalTime(latestBlock.timestamp, false)}
                    </span>
                    {!isRunning && (
                      <span className="text-[10px] text-slate-500 block font-mono">
                        Node offline • Historical record
                      </span>
                    )}
                  </div>
                ) : (
                  <div className="mt-1.5">
                    <span className="text-sm font-mono text-slate-500">None yet</span>
                    <span className="text-[11px] text-slate-500 block">
                      {isRunning ? 'Waiting for block generation' : 'Node is currently offline'}
                    </span>
                  </div>
                )}
              </div>

            </div>
          </section>

          {/* 2. Table Header & Search */}
          <div className="space-y-3 pt-1">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2.5">
              <div className="flex items-center space-x-2">
                <h4 className="text-xs font-semibold uppercase tracking-wider text-slate-400">
                  Last 10 Blocks Harvested
                </h4>
                <span className="text-[10px] bg-slate-800 text-slate-400 font-mono px-2 py-0.5 rounded border border-slate-700/50">
                  Showing {Math.min(filteredBlocks.length, 10)} of {validatedBlocks.length}
                </span>
              </div>

              {validatedBlocks.length > 0 && (
                <div className="relative w-full sm:w-64">
                  <Search className="w-3.5 h-3.5 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
                  <input
                    type="text"
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    placeholder="Search height or hash..."
                    className="w-full bg-[#0F1115] border border-[#262B34] rounded-md pl-8 pr-3 py-1.5 text-xs text-slate-100 placeholder-slate-500 focus:outline-none focus:border-blue-500 font-mono"
                  />
                </div>
              )}
            </div>

            {/* Blocks Table (Last 10) */}
            {filteredBlocks.length > 0 ? (
              <div className="border border-[#262B34] rounded-lg overflow-hidden shadow-xs bg-[#181B20]">
                <div className="overflow-x-auto">
                  <table className="w-full text-left text-xs">
                    <thead className="bg-[#0F1115] border-b border-[#262B34] text-slate-400 font-medium text-[11px]">
                      <tr>
                        <th className="py-2.5 px-3.5 font-medium">Block Height</th>
                        <th className="py-2.5 px-3.5 font-medium">Validated At (Local)</th>
                        <th className="py-2.5 px-3.5 text-center font-medium">Txs</th>
                        <th className="py-2.5 px-3.5 text-right font-medium">Fee Earned</th>
                        <th className="py-2.5 px-3.5 font-medium">Block Hash</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-[#262B34]/60 font-mono">
                      {filteredBlocks.slice(0, 10).map((b) => (
                        <tr key={b.height} className="hover:bg-[#1E2228] transition-colors">
                          <td className="py-2.5 px-3.5 font-medium text-blue-400">
                            <a
                              href={getExplorerBlockUrl(b.height)}
                              target="_blank"
                              rel="noreferrer"
                              className="hover:underline flex items-center gap-1"
                            >
                              <span>#{b.height.toLocaleString()}</span>
                              <ExternalLink className="w-3 h-3 text-slate-500" />
                            </a>
                          </td>
                          <td className="py-2.5 px-3.5 text-slate-300">
                            <div className="flex flex-col">
                              <span>{formatToLocalTime(b.timestamp)}</span>
                              <span className="text-[10px] text-slate-500 font-sans">
                                {formatRelativeTime(b.timestamp)}
                              </span>
                            </div>
                          </td>
                          <td className="py-2.5 px-3.5 text-center text-slate-400">
                            {b.numTransactions}
                          </td>
                          <td className="py-2.5 px-3.5 text-right font-semibold text-emerald-400">
                            +{b.feeXPX.toFixed(4)} XPX
                          </td>
                          <td className="py-2.5 px-3.5 text-[11px] text-slate-400 max-w-[140px] truncate">
                            <div className="flex items-center space-x-1.5">
                              <span className="truncate" title={b.hash}>{b.hash}</span>
                              <button
                                onClick={() => handleCopyHash(b.hash)}
                                className="p-0.5 hover:bg-[#262B34] rounded text-slate-500 hover:text-slate-200 transition-colors flex-shrink-0"
                                title="Copy Hash"
                              >
                                {copiedHash === b.hash ? (
                                  <Check className="w-3 h-3 text-emerald-400" />
                                ) : (
                                  <Copy className="w-3 h-3" />
                                )}
                              </button>
                            </div>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            ) : (
              <div className="text-center py-8 bg-[#181B20] border border-[#262B34] rounded-lg text-xs text-slate-400">
                {searchQuery ? 'No blocks matching your search query.' : 'No blocks validated yet.'}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};
