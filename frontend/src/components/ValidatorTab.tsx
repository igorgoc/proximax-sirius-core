import React, { useState, useMemo } from 'react';
import { 
  Shield, 
  Coins, 
  Copy, 
  Check, 
  ExternalLink, 
  Award, 
  AlertCircle, 
  Search,
  Sliders,
  ChevronLeft,
  ChevronRight,
  HardDrive
} from 'lucide-react';
import { NodeMetrics, NodeConfig, HarvestStats, NetworkValidatorStats, StorageStatus, PortCheckResult } from '../types';
import { getExplorerBlockUrl, getExplorerAddressUrl } from '../utils/explorer';
import { formatNumber, formatXPXInMillions } from '../utils/format';
import { formatToLocalTime, formatRelativeTime } from '../utils/date';
import { StorageTab } from './StorageTab';

interface ValidatorTabProps {
  metrics: NodeMetrics | null;
  config: NodeConfig | null;
  harvestStats: HarvestStats | null;
  networkValidatorStats: NetworkValidatorStats | null;
  storageStatus?: StorageStatus | null;
  portCheck?: PortCheckResult | null;
  loading?: boolean;
  onOpenSettings: () => void;
  onStartNode?: () => void;
  onRefresh?: () => void;
}

export const ValidatorTab: React.FC<ValidatorTabProps> = ({
  metrics,
  config,
  harvestStats,
  networkValidatorStats,
  storageStatus,
  portCheck,
  loading = false,
  onOpenSettings,
  onStartNode,
  onRefresh
}) => {
  const [activeSubTab, setActiveSubTab] = useState<'harvesting' | 'storage'>(() => {
    if (typeof window !== 'undefined' && (window.location.hash === '#storage' || window.location.hash.startsWith('#storage-') || window.location.hash === '#validator-storage')) {
      return 'storage';
    }
    return 'harvesting';
  });

  const handleSubTabChange = (tab: 'harvesting' | 'storage') => {
    setActiveSubTab(tab);
    if (typeof window !== 'undefined') {
      window.history.replaceState(null, '', tab === 'storage' ? '#storage' : '#validator');
    }
  };
  const [copiedKey, setCopiedKey] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);

  const handleCopy = (text: string, id: string) => {
    navigator.clipboard.writeText(text);
    setCopiedKey(id);
    setTimeout(() => setCopiedKey(null), 2000);
  };

  const isRunning = metrics?.status === 'running';
  const isAutoHarvesting = config?.isAutoHarvesting ?? false;
  const hasHarvestKey = Boolean(
    config?.hasHarvestKey ||
    (config?.harvestKey && config.harvestKey !== '') ||
    (config?.harvestPublicKey && config.harvestPublicKey !== '') ||
    (config?.harvestAddress && config.harvestAddress !== '')
  );

  // Truncate long strings
  const truncate = (str?: string, front = 8, back = 8) => {
    if (!str) return '—';
    if (str.length <= front + back) return str;
    return `${str.slice(0, front)}...${str.slice(-back)}`;
  };

  const harvestAddress = config?.harvestAddress || config?.harvestPublicKey || '';
  const totalHarvested = harvestStats?.totalEarnedFeesXPX ?? 0;
  const validatedBlocks = harvestStats?.validatedBlocks || [];

  // Filter blocks by search query
  const filteredBlocks = useMemo(() => {
    return validatedBlocks.filter(block => {
      if (!searchQuery.trim()) return true;
      const q = searchQuery.toLowerCase().trim();
      return (
        block.height.toString().includes(q) ||
        (block.hash && block.hash.toLowerCase().includes(q)) ||
        formatToLocalTime(block.timestamp).toLowerCase().includes(q) ||
        (block.timestamp && block.timestamp.toLowerCase().includes(q))
      );
    });
  }, [validatedBlocks, searchQuery]);

  // Reset pagination on search
  const totalPages = Math.max(1, Math.ceil(filteredBlocks.length / pageSize));
  const safePage = Math.min(currentPage, totalPages);
  const paginatedBlocks = useMemo(() => {
    const start = (safePage - 1) * pageSize;
    return filteredBlocks.slice(start, start + pageSize);
  }, [filteredBlocks, safePage, pageSize]);

  return (
    <div className="space-y-6 max-w-5xl mx-auto px-4 py-6 select-none">
      
      {/* Sub-Navigation Switcher: Block Harvesting (POS+) | Storage Replicator (DFMS) */}
      <div className="flex items-center space-x-2 border-b border-[#262B34] pb-4">
        <button
          onClick={() => handleSubTabChange('harvesting')}
          className={`flex items-center space-x-2 px-3.5 py-1.5 rounded-md text-xs font-semibold transition-all ${
            activeSubTab === 'harvesting'
              ? 'bg-blue-600/20 text-blue-400 border border-blue-500/40 shadow-xs'
              : 'text-slate-400 hover:text-slate-200 hover:bg-[#181B20] border border-transparent'
          }`}
        >
          <Coins className="w-3.5 h-3.5" />
          <span>Block Harvesting (POS+)</span>
        </button>
        <button
          onClick={() => handleSubTabChange('storage')}
          className={`flex items-center space-x-2 px-3.5 py-1.5 rounded-md text-xs font-semibold transition-all ${
            activeSubTab === 'storage'
              ? 'bg-blue-600/20 text-blue-400 border border-blue-500/40 shadow-xs'
              : 'text-slate-400 hover:text-slate-200 hover:bg-[#181B20] border border-transparent'
          }`}
        >
          <HardDrive className="w-3.5 h-3.5" />
          <span>Storage Replicator (DFMS)</span>
        </button>
      </div>

      {activeSubTab === 'storage' ? (
        <StorageTab
          storageStatus={storageStatus || null}
          portCheck={portCheck || null}
          metrics={metrics}
          onRefresh={onRefresh || (() => {})}
          loading={loading}
          onStartNode={onStartNode}
        />
      ) : (
        <>
          {/* 1. Missing Key State Banner (only shown after config loads if key is truly missing) */}
      {config !== null && !loading && !hasHarvestKey && (
        <div className="bg-[#181B20] border border-amber-900/40 rounded-lg p-5 flex items-center justify-between">
          <div className="flex items-center space-x-3.5">
            <AlertCircle className="w-5 h-5 text-amber-400" />
            <div>
              <h3 className="text-sm font-medium text-slate-200">Harvesting Key Not Configured</h3>
              <p className="text-xs text-slate-400 mt-0.5">
                Set up your validator private key to start participating in POS+ block harvesting.
              </p>
            </div>
          </div>
          <button
            onClick={onOpenSettings}
            className="flex items-center space-x-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-md text-xs font-semibold transition-colors"
          >
            <Sliders className="w-3.5 h-3.5" />
            <span>Configure in Settings</span>
          </button>
        </div>
      )}

      {/* 2. Hero Section: Primary Validator Status & Account (Bitcoin Core Style) */}
      <section className="bg-[#181B20] border border-[#262B34] rounded-lg p-6">
        <div className="flex flex-col md:flex-row md:items-baseline justify-between gap-6">
          
          <div>
            <div className="flex items-center space-x-2">
              <span className="text-xs font-medium tracking-wider uppercase text-slate-400">
                POS+ Block Harvesting
              </span>
              <span className={`text-xs font-medium px-2 py-0.5 rounded ${
                !isRunning 
                  ? 'bg-slate-800 text-slate-400 border border-slate-700/50'
                  : isAutoHarvesting 
                  ? 'bg-emerald-950/60 text-emerald-400 border border-emerald-800/40' 
                  : 'bg-slate-800 text-slate-400'
              }`}>
                {!isRunning 
                  ? 'Node Offline' 
                  : isAutoHarvesting 
                  ? 'Active Harvester' 
                  : 'Relay Mode'}
              </span>
            </div>

            {/* Account Details */}
            <div className="mt-3 flex items-center space-x-2">
              <span className="text-sm font-mono text-slate-200 font-medium">
                {harvestAddress ? truncate(harvestAddress, 12, 12) : 'No account configured'}
              </span>
              {harvestAddress && (
                <>
                  <button
                    onClick={() => handleCopy(harvestAddress, 'valAddress')}
                    className="p-1 hover:bg-[#262B34] rounded text-slate-400 hover:text-white transition-colors"
                    title="Copy Full Harvester Address"
                  >
                    {copiedKey === 'valAddress' ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                  </button>
                  <a
                    href={getExplorerAddressUrl(harvestAddress)}
                    target="_blank"
                    rel="noreferrer"
                    className="p-1 hover:bg-[#262B34] rounded text-slate-400 hover:text-white transition-colors"
                    title="View Account on Explorer"
                  >
                    <ExternalLink className="w-3.5 h-3.5" />
                  </a>
                </>
              )}
            </div>
          </div>

          {/* Cumulative Rewards Summary */}
          <div className="flex items-center pt-3 md:pt-0 border-t md:border-t-0 border-[#262B34]">
            <div>
              <span className="text-xs text-slate-400 block">Total Fees Harvested</span>
              <span className="text-xl font-mono font-semibold text-emerald-400">
                +{formatNumber(totalHarvested, 2)} XPX
              </span>
            </div>
          </div>

        </div>
      </section>

      {/* 3. Staking & Committee Parameters Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        
        {/* Consensus & Committee Participation */}
        <section className="bg-[#181B20] border border-[#262B34] rounded-lg p-5">
          <h3 className="text-xs font-semibold tracking-wider uppercase text-slate-400 mb-4 flex items-center space-x-2">
            <Shield className="w-3.5 h-3.5 text-blue-400" />
            <span>Consensus & Voting State</span>
          </h3>

          <div className="space-y-3 text-xs">
            <div className="flex justify-between py-1 border-b border-[#262B34]">
              <span className="text-slate-400">Fast Finality Committee</span>
              <span className={`font-medium ${isRunning ? 'text-slate-200' : 'text-slate-500'}`}>
                {!isRunning 
                  ? 'Offline' 
                  : isAutoHarvesting 
                  ? 'Eligible Round Participant' 
                  : 'Quorum Listener'}
              </span>
            </div>
            <div className="flex justify-between py-1 border-b border-[#262B34]">
              <span className="text-slate-400">Harvester Rotation</span>
              <span className="text-slate-200 font-medium">
                {isRunning ? 'Active / Ready' : '—'}
              </span>
            </div>
            <div className="flex justify-between py-1 border-b border-[#262B34]">
              <span className="text-slate-400">Beneficiary Target</span>
              <span className="font-mono text-slate-200">
                {config?.beneficiary && config.beneficiary !== '0000000000000000000000000000000000000000000000000000000000000000' 
                  ? truncate(config.beneficiary, 6, 6) 
                  : 'Self (Harvester Address)'}
              </span>
            </div>
            <div className="flex justify-between py-1 border-b border-[#262B34]">
              <span className="text-slate-400">Max Unlocked Accounts</span>
              <span className="font-mono text-slate-200">
                {config?.maxUnlockedAccounts || 5}
              </span>
            </div>
          </div>
        </section>

        {/* Network Staking Pool & Importance */}
        <section className="bg-[#181B20] border border-[#262B34] rounded-lg p-5">
          <h3 className="text-xs font-semibold tracking-wider uppercase text-slate-400 mb-4 flex items-center space-x-2">
            <Coins className="w-3.5 h-3.5 text-blue-400" />
            <span>Network Staking Metrics</span>
          </h3>

          <div className="space-y-3 text-xs">
            <div className="flex justify-between py-1 border-b border-[#262B34]">
              <span className="text-slate-400">Active Staked Pool</span>
              <span className="font-mono text-slate-200 font-medium">
                {networkValidatorStats?.estimatedStakedPoolXPX 
                  ? formatXPXInMillions(networkValidatorStats.estimatedStakedPoolXPX) 
                  : 'Sirius Mainnet'}
              </span>
            </div>
            <div className="flex justify-between py-1 border-b border-[#262B34]">
              <span className="text-slate-400">Network Active Harvesters (24h)</span>
              <span className="font-mono text-slate-200">
                {networkValidatorStats?.activeValidators24h ? `${networkValidatorStats.activeValidators24h} nodes` : 'Active'}
              </span>
            </div>
            <div className="flex justify-between py-1 border-b border-[#262B34]">
              <span className="text-slate-400">Average Block Time</span>
              <span className="font-mono text-slate-200">
                {networkValidatorStats?.avgBlockTimeSec ? `${networkValidatorStats.avgBlockTimeSec.toFixed(1)}s` : '15.0s'}
              </span>
            </div>
            <div className="flex justify-between py-1 border-b border-[#262B34]">
              <span className="text-slate-400">Last Harvested Height</span>
              <span className="font-mono text-blue-400">
                {harvestStats?.lastHarvestedHeight ? `#${formatNumber(harvestStats.lastHarvestedHeight)}` : '—'}
              </span>
            </div>
          </div>
        </section>

      </div>

      {/* 4. Complete Blocks Minted Section with Bounded Internal Scroll & Pagination */}
      <section className="bg-[#181B20] border border-[#262B34] rounded-lg p-5">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-3">
          <div>
            <h3 className="text-xs font-semibold tracking-wider uppercase text-slate-400 flex items-center space-x-2">
              <Award className="w-3.5 h-3.5 text-blue-400" />
              <span>Blocks Minted by This Node</span>
            </h3>
            <span className="text-[11px] text-slate-500 mt-0.5 block">
              {filteredBlocks.length} total blocks harvested in current dataset
            </span>
          </div>

          {/* Search Filter & Page Size Selector */}
          <div className="flex items-center space-x-2">
            <div className="relative">
              <Search className="w-3.5 h-3.5 text-slate-500 absolute left-2.5 top-2.5" />
              <input
                type="text"
                placeholder="Search height, hash..."
                value={searchQuery}
                onChange={(e) => {
                  setSearchQuery(e.target.value);
                  setCurrentPage(1);
                }}
                className="bg-[#0F1115] border border-[#262B34] rounded px-2.5 pl-8 py-1.5 text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:border-blue-500 font-mono w-48 sm:w-56"
              />
            </div>

            <select
              value={pageSize}
              onChange={(e) => {
                setPageSize(Number(e.target.value));
                setCurrentPage(1);
              }}
              className="bg-[#0F1115] border border-[#262B34] rounded px-2 py-1.5 text-xs text-slate-400 focus:outline-none focus:border-blue-500 font-mono"
              title="Rows per page"
            >
              <option value={10}>10 rows</option>
              <option value={25}>25 rows</option>
              <option value={50}>50 rows</option>
            </select>
          </div>
        </div>

        {/* Bounded Container with Internal Scrollbar & Sticky Header */}
        {paginatedBlocks.length > 0 ? (
          <div>
            <div className="overflow-x-auto max-h-[380px] overflow-y-auto border border-[#262B34]/60 rounded scrollbar-thin">
              <table className="w-full text-left text-xs">
                <thead className="sticky top-0 bg-[#181B20] border-b border-[#262B34] z-10 shadow-xs">
                  <tr className="text-slate-400">
                    <th className="py-2.5 px-3 font-medium">Height</th>
                    <th className="py-2.5 px-3 font-medium">Date & Time (Local)</th>
                    <th className="py-2.5 px-3 font-medium">Block Hash</th>
                    <th className="py-2.5 px-3 font-medium text-center">Transactions</th>
                    <th className="py-2.5 px-3 font-medium text-right">Fee Earned</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[#262B34]/60 font-mono">
                  {paginatedBlocks.map((block, idx) => (
                    <tr key={idx} className="hover:bg-[#1E2228] transition-colors">
                      <td className="py-2.5 px-3 text-blue-400 font-semibold">
                        <a
                          href={getExplorerBlockUrl(block.height)}
                          target="_blank"
                          rel="noreferrer"
                          className="hover:underline inline-flex items-center space-x-1"
                        >
                          <span>#{formatNumber(block.height)}</span>
                          <ExternalLink className="w-3 h-3 text-slate-500" />
                        </a>
                      </td>
                      <td className="py-2.5 px-3 text-slate-300 font-mono text-xs">
                        {block.timestamp ? (
                          <div className="flex flex-col">
                            <span className="text-slate-200">{formatToLocalTime(block.timestamp)}</span>
                            <span className="text-[10px] text-slate-500 font-sans">{formatRelativeTime(block.timestamp)}</span>
                          </div>
                        ) : (
                          <span className="text-slate-500">Recent</span>
                        )}
                      </td>
                      <td className="py-2.5 px-3 text-slate-300">
                        <div className="flex items-center space-x-1.5">
                          <span>{truncate(block.hash, 8, 8)}</span>
                          {block.hash && (
                            <button
                              onClick={() => handleCopy(block.hash, `val-hash-${block.height}`)}
                              className="p-0.5 hover:bg-[#262B34] rounded text-slate-500 hover:text-slate-200"
                              title="Copy Hash"
                            >
                              {copiedKey === `val-hash-${block.height}` ? <Check className="w-3 h-3 text-emerald-400" /> : <Copy className="w-3 h-3" />}
                            </button>
                          )}
                        </div>
                      </td>
                      <td className="py-2.5 px-3 text-center text-slate-300">
                        {block.numTransactions || 0} txs
                      </td>
                      <td className="py-2.5 px-3 text-right text-emerald-400 font-semibold">
                        +{formatNumber(block.feeXPX || 0, 2)} XPX
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {/* Pagination Controls Bar */}
            <div className="mt-3 flex flex-col sm:flex-row sm:items-center justify-between gap-2 text-xs text-slate-400 pt-2 border-t border-[#262B34]/60">
              <span className="font-mono text-[11px] text-slate-500">
                Showing {((safePage - 1) * pageSize) + 1} – {Math.min(filteredBlocks.length, safePage * pageSize)} of {filteredBlocks.length} blocks
              </span>

              <div className="flex items-center space-x-2">
                <button
                  disabled={safePage <= 1}
                  onClick={() => setCurrentPage(p => Math.max(1, p - 1))}
                  className="px-2.5 py-1 bg-[#0F1115] hover:bg-[#262B34] disabled:opacity-40 disabled:hover:bg-[#0F1115] border border-[#262B34] rounded text-slate-300 font-medium inline-flex items-center space-x-1 transition-colors"
                >
                  <ChevronLeft className="w-3.5 h-3.5" />
                  <span>Prev</span>
                </button>

                <span className="font-mono text-[11px] px-2 text-slate-400">
                  Page {safePage} of {totalPages}
                </span>

                <button
                  disabled={safePage >= totalPages}
                  onClick={() => setCurrentPage(p => Math.min(totalPages, p + 1))}
                  className="px-2.5 py-1 bg-[#0F1115] hover:bg-[#262B34] disabled:opacity-40 disabled:hover:bg-[#0F1115] border border-[#262B34] rounded text-slate-300 font-medium inline-flex items-center space-x-1 transition-colors"
                >
                  <span>Next</span>
                  <ChevronRight className="w-3.5 h-3.5" />
                </button>
              </div>
            </div>
          </div>
        ) : (
          <div className="py-10 text-center text-xs text-slate-400">
            <p>No harvested blocks match the current search filter.</p>
            <p className="text-slate-500 text-[11px] mt-1">
              Your node will automatically record all newly minted blocks here.
            </p>
          </div>
        )}
      </section>
        </>
      )}

    </div>
  );
};
