import React, { useState, useMemo, useEffect } from 'react';
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
  HardDrive,
  Sparkles,
  Globe,
  Users,
  CheckCircle,
  RefreshCw,
  Trash2
} from 'lucide-react';
import { NodeMetrics, NodeConfig, HarvestStats, NetworkValidatorStats, StorageStatus, PortCheckResult } from '../types';
import { getExplorerBlockUrl, getExplorerAddressUrl } from '../utils/explorer';
import { formatNumber, formatXPXInMillions } from '../utils/format';
import { formatToLocalTime, formatRelativeTime } from '../utils/date';
import { StorageTab } from './StorageTab';
import { SiriusGalaxyView } from './galaxy/SiriusGalaxyView';

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
  const [activeSubTab, setActiveSubTab] = useState<'harvesting' | 'galaxy' | 'storage'>(() => {
    if (typeof window !== 'undefined') {
      if (window.location.hash === '#galaxy' || window.location.hash === '#constellation') {
        return 'galaxy';
      }
      if (window.location.hash === '#storage' || window.location.hash.startsWith('#storage-') || window.location.hash === '#validator-storage') {
        return 'storage';
      }
    }
    return 'harvesting';
  });

  const handleSubTabChange = (tab: 'harvesting' | 'galaxy' | 'storage') => {
    setActiveSubTab(tab);
    if (typeof window !== 'undefined') {
      const hash = tab === 'galaxy' ? '#galaxy' : tab === 'storage' ? '#storage' : '#validator';
      window.history.replaceState(null, '', hash);
    }
  };
  const [copiedKey, setCopiedKey] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);

  const [onChainStatus, setOnChainStatus] = useState<{ registered: boolean; metadata?: any; loading: boolean }>({
    registered: false,
    loading: true,
  });
  const [isRegisteringOnChain, setIsRegisteringOnChain] = useState(false);
  const [regMessage, setRegMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);
  const [delegatedHarvesters, setDelegatedHarvesters] = useState<Array<{ fileName: string; harvesterPublicKey: string; modifiedAt: string }>>([]);
  const [loadingHarvesters, setLoadingHarvesters] = useState(false);

  const fetchOnChainStatus = async () => {
    try {
      setOnChainStatus(prev => ({ ...prev, loading: true }));
      const res = await fetch('/api/validator/onchain-status');
      const data = await res.json();
      setOnChainStatus({
        registered: Boolean(data.registered),
        metadata: data.metadata,
        loading: false,
      });
    } catch {
      setOnChainStatus({ registered: false, loading: false });
    }
  };

  const fetchDelegatedHarvesters = async () => {
    try {
      setLoadingHarvesters(true);
      const res = await fetch('/api/harvesting/delegated/list');
      const data = await res.json();
      if (Array.isArray(data)) {
        setDelegatedHarvesters(data);
      }
    } catch {
      // ignore
    } finally {
      setLoadingHarvesters(false);
    }
  };

  useEffect(() => {
    fetchOnChainStatus();
    fetchDelegatedHarvesters();
  }, [config?.harvestPublicKey]);

  const handleRegisterOnChain = async () => {
    setIsRegisteringOnChain(true);
    setRegMessage(null);
    try {
      const res = await fetch('/api/validator/register-onchain', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: config?.friendlyName || 'Sirius Validator Node',
          endpoint: `http://${window.location.hostname || 'localhost'}:8080`,
          restEndpoint: `http://${window.location.hostname || 'localhost'}:3000`,
          location: 'Global',
        }),
      });
      const data = await res.json();
      if (res.ok && (data.status === 'SUCCESS' || data.status === 'ALREADY_REGISTERED')) {
        setRegMessage({ type: 'success', text: data.message || 'Successfully registered on-chain!' });
        fetchOnChainStatus();
      } else {
        setRegMessage({ type: 'error', text: data.error || data.message || 'Failed to register on-chain' });
      }
    } catch (err: any) {
      setRegMessage({ type: 'error', text: err.message || 'Network error registering on-chain' });
    } finally {
      setIsRegisteringOnChain(false);
    }
  };

  const handleRemoveHarvester = async (fileName: string) => {
    if (!confirm(`Are you sure you want to remove delegated harvester key ${fileName}?`)) return;
    try {
      await fetch('/api/harvesting/delegated/remove', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ fileName }),
      });
      fetchDelegatedHarvesters();
    } catch (err) {
      console.error('Failed to remove delegated key:', err);
    }
  };

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
          onClick={() => handleSubTabChange('galaxy')}
          className={`flex items-center space-x-2 px-3.5 py-1.5 rounded-md text-xs font-semibold transition-all ${
            activeSubTab === 'galaxy'
              ? 'bg-sky-600/20 text-sky-400 border border-sky-500/40 shadow-xs'
              : 'text-slate-400 hover:text-slate-200 hover:bg-[#181B20] border border-transparent'
          }`}
        >
          <Sparkles className="w-3.5 h-3.5 text-amber-400 animate-pulse" />
          <span>Sirius Galaxy (Constellation)</span>
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

      {activeSubTab === 'galaxy' ? (
        <SiriusGalaxyView
          metrics={metrics}
          harvestStats={harvestStats}
          networkValidatorStats={networkValidatorStats}
        />
      ) : activeSubTab === 'storage' ? (
        <StorageTab
          storageStatus={storageStatus || null}
          portCheck={portCheck || null}
          metrics={metrics}
          onRefresh={onRefresh || (() => {})}
          loading={loading}
          onStartNode={hasHarvestKey ? onStartNode : undefined}
        />
      ) : (
        <>
          {/* 1. Missing Key State Banner (only shown after config loads if key is truly missing) */}
      {config !== null && !loading && !hasHarvestKey && (
        <div className="bg-[#181B20] border border-amber-900/40 rounded-lg p-5 flex items-center justify-between">
          <div className="flex items-center space-x-3.5">
            <AlertCircle className="w-5 h-5 text-amber-400" />
            <div>
              <h3 className="text-sm font-medium text-amber-200">Harvesting Key Not Configured</h3>
              <p className="text-xs text-slate-400 mt-0.5">
                A valid 64-character harvest key is mandatory to participate in POS+ consensus and start the node.
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

      {/* 3.5 Decentralized Directory & Delegated Staking Pool Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        
        {/* On-Chain Directory Registration */}
        <section className="bg-[#181B20] border border-[#262B34] rounded-lg p-5 flex flex-col justify-between">
          <div>
            <div className="flex items-center justify-between mb-3">
              <h3 className="text-xs font-semibold tracking-wider uppercase text-slate-400 flex items-center space-x-2">
                <Globe className="w-3.5 h-3.5 text-sky-400" />
                <span>On-Chain Node Directory</span>
              </h3>
              <span className={`text-[11px] font-medium px-2 py-0.5 rounded flex items-center gap-1 ${
                onChainStatus.registered 
                  ? 'bg-emerald-950/60 text-emerald-400 border border-emerald-800/40' 
                  : 'bg-amber-950/60 text-amber-300 border border-amber-800/40'
              }`}>
                {onChainStatus.registered ? <CheckCircle className="w-3 h-3 text-emerald-400" /> : <AlertCircle className="w-3 h-3 text-amber-400" />}
                {onChainStatus.registered ? 'Registered (Public)' : 'Unregistered'}
              </span>
            </div>

            <p className="text-xs text-slate-400 mb-4 leading-relaxed">
              Publishes this node into Sirius Mainnet's decentralized metadata directory (<code className="text-sky-300 font-mono">sirius.v</code>). Community web wallets discover this node with zero hardcoding.
            </p>

            {onChainStatus.registered && onChainStatus.metadata && (
              <div className="bg-[#111317] border border-[#262B34] rounded p-3 mb-4 space-y-1.5 text-xs font-mono">
                <div className="flex justify-between">
                  <span className="text-slate-500">Name:</span>
                  <span className="text-slate-200">{onChainStatus.metadata.name || '—'}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-slate-500">Public Web:</span>
                  <span className="text-slate-200">{onChainStatus.metadata.endpoint || '—'}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-slate-500">REST API:</span>
                  <span className="text-slate-200">{onChainStatus.metadata.restEndpoint || '—'}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-slate-500">Region:</span>
                  <span className="text-slate-200">{onChainStatus.metadata.location || 'Global'}</span>
                </div>
              </div>
            )}

            {regMessage && (
              <div className={`p-2.5 rounded text-xs mb-3 ${
                regMessage.type === 'success' 
                  ? 'bg-emerald-950/60 border border-emerald-800/60 text-emerald-300' 
                  : 'bg-red-950/60 border border-red-800/60 text-red-300'
              }`}>
                {regMessage.text}
              </div>
            )}
          </div>

          <button
            onClick={handleRegisterOnChain}
            disabled={isRegisteringOnChain || !hasHarvestKey}
            className="w-full mt-2 flex items-center justify-center space-x-2 px-4 py-2.5 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded-md text-xs font-semibold transition-colors cursor-pointer"
          >
            {isRegisteringOnChain ? (
              <>
                <RefreshCw className="w-3.5 h-3.5 animate-spin" />
                <span>Broadcasting to Sirius Mainnet...</span>
              </>
            ) : onChainStatus.registered ? (
              <>
                <Globe className="w-3.5 h-3.5" />
                <span>Update On-Chain Registration</span>
              </>
            ) : (
              <>
                <Globe className="w-3.5 h-3.5" />
                <span>Register Validator On-Chain (1-Click)</span>
              </>
            )}
          </button>
        </section>

        {/* Delegated Staking Pool (Active Harvesters) */}
        <section className="bg-[#181B20] border border-[#262B34] rounded-lg p-5 flex flex-col justify-between">
          <div>
            <div className="flex items-center justify-between mb-3">
              <h3 className="text-xs font-semibold tracking-wider uppercase text-slate-400 flex items-center space-x-2">
                <Users className="w-3.5 h-3.5 text-emerald-400" />
                <span>Delegated Staking Pool</span>
              </h3>
              <span className="text-[11px] font-mono font-medium px-2 py-0.5 rounded bg-slate-800 text-slate-300 border border-slate-700">
                {delegatedHarvesters.length} / {config?.maxUnlockedAccounts || 5} Slots
              </span>
            </div>

            <p className="text-xs text-slate-400 mb-3 leading-relaxed">
              Active remote harvester keys hotloaded onto this validator from web wallet delegators.
            </p>

            <div className="space-y-2 max-h-[160px] overflow-y-auto pr-1">
              {loadingHarvesters ? (
                <div className="text-xs text-slate-500 py-4 text-center">Loading delegated harvesters...</div>
              ) : delegatedHarvesters.length === 0 ? (
                <div className="text-xs text-slate-500 py-6 text-center border border-dashed border-[#262B34] rounded">
                  No delegated accounts currently connected.
                </div>
              ) : (
                delegatedHarvesters.map((item) => (
                  <div key={item.fileName} className="flex items-center justify-between p-2 bg-[#111317] border border-[#262B34] rounded text-xs">
                    <div>
                      <div className="font-mono text-slate-200 font-medium">
                        {truncate(item.harvesterPublicKey || item.fileName, 8, 8)}
                      </div>
                      <div className="text-[10px] text-slate-500">{item.fileName}</div>
                    </div>
                    <button
                      onClick={() => handleRemoveHarvester(item.fileName)}
                      className="p-1.5 hover:bg-red-950/60 hover:text-red-400 text-slate-500 rounded transition-colors"
                      title="Remove Delegated Key"
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </div>
                ))
              )}
            </div>
          </div>

          <div className="mt-3 pt-3 border-t border-[#262B34] flex items-center justify-between text-xs text-slate-400">
            <span>Dynamic Hotload Ready</span>
            <span className="text-emerald-400 font-semibold">Active</span>
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
