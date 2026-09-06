import React, { useState } from 'react';
import { 
  Clock, 
  Copy, 
  Check, 
  Layers, 
  Shield, 
  AlertCircle,
  AlertTriangle,
  Play,
  ExternalLink,
  Users
} from 'lucide-react';
import { NodeMetrics, NodeConfig, HarvestStats, StorageStatus, PortCheckResult, NetworkValidatorStats } from '../types';
import { getExplorerBlockUrl, getExplorerAddressUrl } from '../utils/explorer';
import { formatNumber } from '../utils/format';
import { formatToLocalTime, formatRelativeTime } from '../utils/date';
import { PeersModal } from './PeersModal';
import { BlocksValidatedModal } from './BlocksValidatedModal';

interface OverviewTabProps {
  metrics: NodeMetrics | null;
  config: NodeConfig | null;
  harvestStats: HarvestStats | null;
  networkValidatorStats?: NetworkValidatorStats | null;
  storageStatus?: StorageStatus | null;
  portCheck?: PortCheckResult | null;
  updateInfo?: any;
  autoRecovery?: boolean;
  onStart: () => void;
  onStop: () => void;
  onRestart: () => void;
  onOpenWizard: () => void;
  setActiveTab: (tab: string) => void;
  loading: boolean;
  onRefresh: () => void;
  onToggleAutoRecovery?: (val: boolean) => void;
}

export const OverviewTab: React.FC<OverviewTabProps> = ({
  metrics,
  config,
  harvestStats,
  networkValidatorStats,
  onStart,
  setActiveTab,
  loading,
}) => {
  const [copiedKey, setCopiedKey] = useState<string | null>(null);
  const [peersModalOpen, setPeersModalOpen] = useState(false);
  const [blocksModalOpen, setBlocksModalOpen] = useState(false);

  const handleCopy = (text: string, id: string) => {
    navigator.clipboard.writeText(text);
    setCopiedKey(id);
    setTimeout(() => setCopiedKey(null), 2000);
  };

  const isRunning = metrics?.status === 'running';
  const isStarting = metrics?.status === 'starting';
  const blockHeight = metrics?.blockHeight || 0;
  const networkHeight = metrics?.networkHeight || 0;
  
  // Strict synchronization calculation: never round up to 100.00% if any blocks behind
  const blocksBehind = Math.max(0, networkHeight - blockHeight);
  const isSynced = isRunning && networkHeight > 0 && blocksBehind <= 1;
  
  let syncPercent = 0;
  if (networkHeight > 0 && blockHeight > 0) {
    if (isSynced || blockHeight >= networkHeight) {
      syncPercent = 100;
    } else {
      // Floor to 2 decimals and clamp to 99.99% maximum when behind
      const raw = (blockHeight / networkHeight) * 100;
      syncPercent = Math.min(99.99, Math.floor(raw * 100) / 100);
    }
  }

  // Truncate long hashes / addresses (e.g. 1D33...27F2)
  const truncate = (str?: string, front = 6, back = 6) => {
    if (!str) return '—';
    if (str.length <= front + back) return str;
    return `${str.slice(0, front)}...${str.slice(-back)}`;
  };

  const harvestAddress = config?.harvestAddress || config?.harvestPublicKey || '';
  const totalHarvested = harvestStats?.totalEarnedFeesXPX ?? 0;
  const totalBlocks = harvestStats?.totalBlocksValidated ?? (harvestStats?.validatedBlocks?.length || 0);
  const recentBlocks = harvestStats?.validatedBlocks || [];

  return (
    <div className="space-y-6 max-w-5xl mx-auto px-4 py-6 select-none">
      
      {/* 0. Storage Location Unreachable / Startup Error Banner */}
      {metrics?.errorMessage && (
        <div className="bg-rose-950/40 border border-rose-800/60 rounded-lg p-5 flex flex-col sm:flex-row sm:items-center justify-between gap-3 animate-fadeIn">
          <div className="flex items-center space-x-3.5">
            <AlertTriangle className="w-5 h-5 text-rose-400 flex-shrink-0" />
            <div>
              <h3 className="text-sm font-semibold text-rose-200">Startup Pre-Flight Check Failed</h3>
              <p className="text-xs text-rose-300 mt-0.5 font-mono">
                {metrics.errorMessage}
              </p>
            </div>
          </div>
          <button
            onClick={() => setActiveTab('config')}
            className="flex items-center space-x-1.5 px-3 py-1.5 bg-rose-900/60 hover:bg-rose-800 border border-rose-700/60 text-white rounded text-xs font-medium transition-colors flex-shrink-0"
          >
            <span>Change Path in Settings</span>
            <ExternalLink className="w-3 h-3" />
          </button>
        </div>
      )}

      {/* 1. Offline Banner State (When Node is Stopped) */}
      {!isRunning && (
        <div className="bg-[#181B20] border border-[#262B34] rounded-lg p-5 flex items-center justify-between">
          <div className="flex items-center space-x-3.5">
            <AlertCircle className="w-5 h-5 text-slate-400" />
            <div>
              <h3 className="text-sm font-medium text-slate-200">Sirius Node is Offline</h3>
              <p className="text-xs text-slate-400 mt-0.5">
                Start the node to participate in consensus and begin harvesting blocks.
              </p>
            </div>
          </div>
          <button
            onClick={onStart}
            disabled={loading || isStarting}
            className="flex items-center space-x-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded-md text-xs font-semibold transition-colors"
          >
            <Play className="w-3.5 h-3.5 fill-current" />
            <span>{isStarting ? 'Starting...' : 'Start Node'}</span>
          </button>
        </div>
      )}

      {/* 2. Hero Section: Primary Financial & Rewards Focus (Bitcoin Core Balance Pattern) */}
      <section className="bg-[#181B20] border border-[#262B34] rounded-lg p-6">
        <div className="flex flex-col md:flex-row md:items-baseline justify-between gap-4">
          <div>
            <span className="text-xs font-medium tracking-wider uppercase text-slate-400">
              Total Block Harvesting Rewards
            </span>
            <div className="mt-2 flex items-baseline space-x-2">
              <span className="text-3xl sm:text-4xl font-semibold tracking-tight text-emerald-400 font-mono">
                +{formatNumber(totalHarvested, 2)}
              </span>
              <span className="text-lg font-medium text-slate-400">XPX</span>
            </div>
          </div>

          <div className="flex items-center space-x-6 pt-2 md:pt-0 border-t md:border-t-0 border-[#262B34]">
            <div>
              <span className="text-xs text-slate-400 block">Blocks Generated</span>
              <button 
                onClick={() => setBlocksModalOpen(true)}
                className="text-sm font-mono font-medium text-slate-200 hover:text-blue-400 hover:underline inline-flex items-center space-x-1"
                title="View All Harvested Blocks"
              >
                <span>{totalBlocks} blocks</span>
              </button>
            </div>
            <div className="h-8 w-px bg-[#262B34]" />
            <div>
              <span className="text-xs text-slate-400 block">Active Pool</span>
              <span className="text-sm font-mono font-medium text-slate-300">
                {networkValidatorStats?.estimatedStakedPoolXPX ? `${formatNumber(networkValidatorStats.estimatedStakedPoolXPX / 1000000, 1)}M XPX` : 'Sirius Mainnet'}
              </span>
            </div>
          </div>
        </div>
      </section>

      {/* 3. Core Status Columns: Blockchain Sync vs. Validator State */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        
        {/* Left Column: Blockchain Synchronization */}
        <section className="bg-[#181B20] border border-[#262B34] rounded-lg p-5 flex flex-col justify-between">
          <div>
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-xs font-semibold tracking-wider uppercase text-slate-400 flex items-center space-x-2">
                <Layers className="w-3.5 h-3.5 text-blue-400" />
                <span>Blockchain Synchronization</span>
              </h3>
              <span className={`text-xs font-medium px-2 py-0.5 rounded ${
                !isRunning
                  ? 'bg-slate-800 text-slate-400 border border-slate-700/50'
                  : isSynced 
                  ? 'bg-emerald-950/60 text-emerald-400 border border-emerald-800/40' 
                  : 'bg-blue-950/60 text-blue-400 border border-blue-800/40'
              }`}>
                {!isRunning 
                  ? 'Offline (Paused)' 
                  : isSynced 
                  ? 'Synchronized' 
                  : `Syncing (${syncPercent.toFixed(2)}%)`}
              </span>
            </div>

            <div className="space-y-3 text-xs">
              <div className="flex justify-between py-1 border-b border-[#262B34]">
                <span className="text-slate-400">{isRunning ? 'Local Height' : 'Last Known Height'}</span>
                <span className="font-mono text-slate-200 font-medium">{formatNumber(blockHeight)}</span>
              </div>
              <div className="flex justify-between py-1 border-b border-[#262B34]">
                <span className="text-slate-400">Network Tip</span>
                <span className="font-mono text-slate-200 font-medium">
                  {networkHeight > 0 ? formatNumber(networkHeight) : isRunning ? 'Connecting...' : '—'}
                </span>
              </div>
              <div className="flex justify-between py-1 border-b border-[#262B34]">
                <span className="text-slate-400">Sync Progress</span>
                <span className="font-mono text-slate-200">
                  {!isRunning
                    ? `Paused (${formatNumber(blocksBehind)} blocks behind tip)`
                    : isSynced 
                    ? 'Up to date' 
                    : `${formatNumber(blocksBehind)} blocks behind`}
                </span>
              </div>
            </div>
          </div>

          {/* Minimal Progress Bar */}
          <div className="mt-5 pt-2">
            <div className="w-full bg-[#0F1115] rounded-full h-2 overflow-hidden border border-[#262B34]">
              <div 
                className={`h-full transition-all duration-500 rounded-full ${
                  !isRunning ? 'bg-slate-600' : isSynced ? 'bg-emerald-500' : 'bg-blue-500'
                }`}
                style={{ width: `${syncPercent}%` }}
              />
            </div>
            <div className="flex justify-between text-[11px] text-slate-400 mt-2 font-mono">
              <span>Genesis</span>
              <span>{syncPercent.toFixed(2)}%</span>
              <span>Tip: {formatNumber(networkHeight || blockHeight)}</span>
            </div>
          </div>
        </section>

        {/* Right Column: Validator State */}
        <section className="bg-[#181B20] border border-[#262B34] rounded-lg p-5 flex flex-col justify-between">
          <div>
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-xs font-semibold tracking-wider uppercase text-slate-400 flex items-center space-x-2">
                <Shield className="w-3.5 h-3.5 text-blue-400" />
                <span>Validator State</span>
              </h3>
              <span className={`text-xs font-medium px-2 py-0.5 rounded ${
                !isRunning
                  ? 'bg-slate-800 text-slate-400 border border-slate-700/50'
                  : config?.isAutoHarvesting 
                  ? 'bg-emerald-950/60 text-emerald-400 border border-emerald-800/40' 
                  : 'bg-slate-800 text-slate-400'
              }`}>
                {!isRunning
                  ? `Offline (${config?.isAutoHarvesting ? 'Configured' : 'Relay'})`
                  : config?.isAutoHarvesting 
                  ? 'Active Harvester' 
                  : 'Relay Only'}
              </span>
            </div>

            <div className="space-y-3 text-xs">
              <div className="flex items-center justify-between py-1 border-b border-[#262B34]">
                <span className="text-slate-400">Harvester Key</span>
                <div className="flex items-center space-x-1.5 font-mono text-slate-200">
                  <span>{truncate(harvestAddress, 6, 6)}</span>
                  {harvestAddress && (
                    <button
                      onClick={() => handleCopy(harvestAddress, 'harvestKey')}
                      className="p-1 hover:bg-[#262B34] rounded text-slate-400 hover:text-white transition-colors"
                      title="Copy Harvester Key"
                    >
                      {copiedKey === 'harvestKey' ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                    </button>
                  )}
                  {harvestAddress && (
                    <a
                      href={getExplorerAddressUrl(harvestAddress)}
                      target="_blank"
                      rel="noreferrer"
                      className="p-1 hover:bg-[#262B34] rounded text-slate-400 hover:text-white transition-colors"
                      title="View in Explorer"
                    >
                      <ExternalLink className="w-3.5 h-3.5" />
                    </a>
                  )}
                </div>
              </div>

              <div className="flex justify-between py-1 border-b border-[#262B34]">
                <span className="text-slate-400">Fast Finality Voting</span>
                <span className={`font-medium ${isRunning ? 'text-slate-200' : 'text-slate-500'}`}>
                  {!isRunning
                    ? 'Offline (Disconnected)'
                    : config?.isAutoHarvesting 
                    ? 'Active Committee Member' 
                    : 'Quorum Listener'}
                </span>
              </div>

              <div className="flex justify-between py-1 border-b border-[#262B34]">
                <span className="text-slate-400">Active Validators (24h)</span>
                <span className="font-mono text-slate-200">
                  {networkValidatorStats?.activeValidators24h ? `${networkValidatorStats.activeValidators24h} nodes` : 'Active'}
                </span>
              </div>
            </div>
          </div>

          <div className="mt-5 pt-3 border-t border-[#262B34] flex items-center justify-between text-xs">
            <span className="text-slate-400">Connected Peers</span>
            {isRunning ? (
              <button
                onClick={() => setPeersModalOpen(true)}
                className="font-mono text-blue-400 hover:underline font-medium inline-flex items-center space-x-1"
              >
                <Users className="w-3.5 h-3.5 mr-1 text-slate-400" />
                <span>{metrics?.peersCount ?? 0} active peers</span>
              </button>
            ) : (
              <span className="text-slate-500 font-mono">0 peers (Offline)</span>
            )}
          </div>
        </section>

      </div>

      {/* 4. Recent Activity: Compact Single-Line Feed Style (Node Liveness Signal) */}
      <section className="bg-[#181B20] border border-[#262B34] rounded-lg p-5">
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-xs font-semibold tracking-wider uppercase text-slate-400 flex items-center space-x-2">
            <Clock className="w-3.5 h-3.5 text-slate-400" />
            <span>Recent Activity</span>
          </h3>
          <button
            onClick={() => setActiveTab('validator')}
            className="text-xs text-blue-400 hover:text-blue-300 hover:underline font-medium inline-flex items-center space-x-1"
          >
            <span>View Full Block History</span>
            <ExternalLink className="w-3 h-3" />
          </button>
        </div>

        {/* Compact Feed Items */}
        {recentBlocks.length > 0 ? (
          <div className="divide-y divide-[#262B34]/60">
            {recentBlocks.slice(0, 4).map((block, idx) => (
              <div key={idx} className="py-2.5 flex flex-col sm:flex-row sm:items-center justify-between gap-2 text-xs hover:bg-[#1E2228]/50 px-1 rounded transition-colors">
                
                {/* Left: Height & Time */}
                <div className="flex items-center space-x-3">
                  <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 flex-shrink-0" />
                  <a
                    href={getExplorerBlockUrl(block.height)}
                    target="_blank"
                    rel="noreferrer"
                    className="font-mono font-semibold text-blue-400 hover:underline inline-flex items-center space-x-1"
                  >
                    <span>Block #{formatNumber(block.height)}</span>
                  </a>
                  <span className="text-slate-500">•</span>
                  <span className="text-slate-400 font-mono text-[11px]" title={`Local Time: ${formatToLocalTime(block.timestamp)}`}>
                    {block.timestamp ? formatToLocalTime(block.timestamp) : 'Recently'}
                  </span>
                  <span className="text-slate-500">•</span>
                  <span className="text-slate-400 font-mono">{block.numTransactions || 0} txs</span>
                </div>

                {/* Right: Hash & Reward */}
                <div className="flex items-center space-x-4 pl-4 sm:pl-0">
                  <div className="flex items-center space-x-1 font-mono text-slate-400 text-[11px]">
                    <span>{truncate(block.hash, 6, 6)}</span>
                    {block.hash && (
                      <button
                        onClick={() => handleCopy(block.hash, `feed-hash-${idx}`)}
                        className="p-0.5 hover:bg-[#262B34] rounded text-slate-500 hover:text-slate-200"
                        title="Copy Hash"
                      >
                        {copiedKey === `feed-hash-${idx}` ? <Check className="w-3 h-3 text-emerald-400" /> : <Copy className="w-3 h-3" />}
                      </button>
                    )}
                  </div>
                  <span className="font-mono font-semibold text-emerald-400 text-right min-w-[90px]">
                    +{formatNumber(block.feeXPX || 0, 2)} XPX
                  </span>
                </div>

              </div>
            ))}
          </div>
        ) : (
          <div className="py-6 text-center text-xs text-slate-400">
            <p>Node is actively validating the network.</p>
            <p className="text-slate-500 text-[11px] mt-0.5">
              Newly minted blocks will appear in this feed automatically.
            </p>
          </div>
        )}
      </section>

      {/* Embedded Detail Modals */}
      <PeersModal isOpen={peersModalOpen} onClose={() => setPeersModalOpen(false)} isRunning={isRunning} />
      <BlocksValidatedModal
        isOpen={blocksModalOpen}
        onClose={() => setBlocksModalOpen(false)}
        harvestStats={harvestStats}
        isRunning={isRunning}
      />

    </div>
  );
};
