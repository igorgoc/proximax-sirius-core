import React, { useState } from 'react';
import { 
  HardDrive, 
  RefreshCw, 
  ExternalLink, 
  Copy, 
  Check, 
  Trash2, 
  Server, 
  Zap, 
  AlertCircle, 
  FolderOpen, 
  CheckCircle2, 
  HelpCircle, 
  Activity,
  Info,
  TrendingUp,
  Sparkles,
  Settings,
  Eye,
  EyeOff,
  Key,
  Shield
} from 'lucide-react';
import { StorageStatus, OnboardReplicatorResult, PortCheckResult, NodeMetrics } from '../types';
import { getExplorerTxUrl } from '../utils/explorer';
import { formatXPX } from '../utils/format';
import { DirectoryDropdown } from './DirectoryDropdown';
import { SiriusLogo } from './SiriusLogo';

interface StorageTabProps {
  storageStatus: StorageStatus | null;
  portCheck: PortCheckResult | null;
  metrics?: NodeMetrics | null;
  onRefresh: () => void;
  loading: boolean;
  onStartNode?: () => void;
}

export const StorageTab: React.FC<StorageTabProps> = ({
  storageStatus,
  portCheck,
  metrics,
  onRefresh,
  loading,
  onStartNode,
}) => {
  const [copiedField, setCopiedField] = useState<string | null>(null);
  
  // Modals
  const [isConfigModalOpen, setIsConfigModalOpen] = useState(() => typeof window !== 'undefined' && window.location.hash === '#storage-config');
  const [isOnboardModalOpen, setIsOnboardModalOpen] = useState(false);
  const [isPortHelpModalOpen, setIsPortHelpModalOpen] = useState(false);
  const [isUnitsModalOpen, setIsUnitsModalOpen] = useState(false);
  const [simulatedRentedGB, setSimulatedRentedGB] = useState<number>(20);
  
  // Actions
  const [customKey, setCustomKey] = useState('');
  const [showCustomKey, setShowCustomKey] = useState(false);
  const [customStoragePath, setCustomStoragePath] = useState('');
  const [savingKey, setSavingKey] = useState(false);
  const [generatingKey, setGeneratingKey] = useState(false);
  const [cleaningSandboxes, setCleaningSandboxes] = useState(false);
  const [testingPorts, setTestingPorts] = useState(false);

  // Onboard State
  const [capacityGB, setCapacityGB] = useState<number>(50);
  const [feeStrategy, setFeeStrategy] = useState<'low' | 'middle' | 'high'>('middle');
  const [onboarding, setOnboarding] = useState(false);
  const [onboardResult, setOnboardResult] = useState<OnboardReplicatorResult | null>(null);
  const [onboardError, setOnboardError] = useState<string | null>(null);
  const [actionMessage, setActionMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  const copyToClipboard = (text: string, field: string) => {
    navigator.clipboard.writeText(text);
    setCopiedField(field);
    setTimeout(() => setCopiedField(null), 2000);
  };

  const handleGenerateKey = async () => {
    setGeneratingKey(true);
    setActionMessage(null);
    try {
      const res = await fetch('/api/storage/key/generate', { method: 'POST' });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to generate replicator key');
      setActionMessage({ type: 'success', text: 'New Replicator KeyPair generated and saved to configuration!' });
      setCustomKey('');
      setIsConfigModalOpen(false);
      onRefresh();
    } catch (e: any) {
      setActionMessage({ type: 'error', text: e.message });
    } finally {
      setGeneratingKey(false);
    }
  };

  const handleSaveCustomKey = async (e: React.FormEvent) => {
    e.preventDefault();
    setSavingKey(true);
    setActionMessage(null);
    try {
      const res = await fetch('/api/storage/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ 
          key: customKey.trim(),
          storagePath: customStoragePath.trim()
        }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to save replicator configuration');
      setActionMessage({ type: 'success', text: 'Replicator configuration updated successfully!' });
      setIsConfigModalOpen(false);
      onRefresh();
    } catch (e: any) {
      setActionMessage({ type: 'error', text: e.message });
    } finally {
      setSavingKey(false);
    }
  };

  // Clean Sandboxes
  const handleCleanSandboxes = async () => {
    setCleaningSandboxes(true);
    setActionMessage(null);
    try {
      const res = await fetch('/api/storage/sandboxes/clean', { method: 'POST' });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to clean sandboxes');
      setActionMessage({ type: 'success', text: data.message || 'Storage sandboxes cleaned successfully!' });
      onRefresh();
    } catch (e: any) {
      setActionMessage({ type: 'error', text: e.message });
    } finally {
      setCleaningSandboxes(false);
    }
  };

  // Re-check Ports
  const handleTestPorts = async () => {
    setTestingPorts(true);
    setActionMessage(null);
    try {
      await fetch('/api/network/port-check');
      onRefresh();
      setActionMessage({ type: 'success', text: 'Network & Port Reachability test complete!' });
    } catch (e: any) {
      setActionMessage({ type: 'error', text: 'Failed to test port reachability' });
    } finally {
      setTestingPorts(false);
    }
  };

  // Execute Onboarding (Prepare + Channel)
  const handleOnboardReplicator = async (e: React.FormEvent) => {
    e.preventDefault();
    setOnboarding(true);
    setOnboardError(null);
    setOnboardResult(null);

    try {
      const res = await fetch('/api/storage/onboard', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          capacityGB: Number(capacityGB),
          feeStrategy: feeStrategy,
        }),
      });

      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error || 'Failed to announce ReplicatorOnboardingTransaction');
      }

      setOnboardResult(data);
      setActionMessage({
        type: 'success',
        text: `ReplicatorOnboardingTransaction for ${capacityGB} GB announced successfully to Mainnet!`,
      });
      onRefresh();
    } catch (e: any) {
      setOnboardError(e.message);
    } finally {
      setOnboarding(false);
    }
  };

  const isNodeRunning = metrics?.status === 'running';
  const isConfigured = !!storageStatus?.config?.isConfigured;
  const isRegistered = !!storageStatus?.onChain?.isRegistered;
  const address = storageStatus?.config?.address || storageStatus?.onChain?.address || '';
  const balanceXPX = storageStatus?.onChain?.balanceXPX || 0;
  const peers = storageStatus?.peers || [];
  const shardsCount = storageStatus?.metrics?.totalShardsCount || 0;
  const driveSizeStr = storageStatus?.metrics?.driveSizeStr || '0.00 B';

  const truncate = (str?: string, front = 8, back = 8) => {
    if (!str) return '—';
    if (str.length <= front + back) return str;
    return `${str.slice(0, front)}...${str.slice(-back)}`;
  };

  return (
    <div className="space-y-6 select-none">
      {/* 1. Header Bar: Aligned Top Card */}
      <div className="bg-[#181B20] border border-[#262B34] rounded-lg p-5">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div className="flex items-center space-x-3.5 min-w-0">
            <div className="relative flex items-center justify-center w-10 h-10 rounded-lg bg-[#0F1115] border border-[#262B34] text-blue-400 flex-shrink-0">
              <SiriusLogo size={20} />
              <span
                className={`absolute -bottom-0.5 -right-0.5 w-2.5 h-2.5 rounded-full border-2 border-[#181B20] ${
                  !isNodeRunning
                    ? 'bg-slate-500'
                    : isRegistered
                    ? 'bg-emerald-400'
                    : isConfigured
                    ? 'bg-blue-400'
                    : 'bg-amber-400'
                }`}
              />
            </div>
            <div className="min-w-0">
              <div className="flex items-center space-x-2">
                <h3 className="text-sm font-semibold text-white tracking-tight truncate">
                  Storage Replicator
                </h3>
                <span
                  className={`inline-flex items-center px-2 py-0.5 rounded text-[10px] font-mono font-medium uppercase tracking-wider ${
                    !isNodeRunning
                      ? 'bg-slate-800 text-slate-400 border border-slate-700/50'
                      : isRegistered
                      ? 'bg-emerald-950/60 text-emerald-400 border border-emerald-800/40'
                      : isConfigured
                      ? 'bg-blue-950/60 text-blue-400 border border-blue-800/40'
                      : 'bg-amber-950/60 text-amber-400 border border-amber-800/40'
                  }`}
                >
                  {!isNodeRunning
                    ? 'Node Offline'
                    : isRegistered
                    ? 'Active Replicator'
                    : isConfigured
                    ? 'Key Configured'
                    : 'Setup Required'}
                </span>
              </div>

              {/* IP Address & Port Status Indicators */}
              <p className="text-xs text-slate-400 mt-1 truncate font-mono">
                IP {portCheck?.publicIp || 'Auto-IP'}:7904 • {portCheck?.port7904Open ? 'Port 7904 Open & Reachable' : 'Port 7904 Restricted'} • Distributed Storage Protocol
              </p>
            </div>
          </div>

          {/* Action Controls */}
          <div className="flex items-center space-x-2 w-full sm:w-auto flex-shrink-0">
            {!storageStatus?.config?.hasKey && (
              <button
                onClick={handleGenerateKey}
                disabled={generatingKey}
                className="flex items-center space-x-1.5 px-3.5 py-2 text-xs font-semibold bg-blue-600 hover:bg-blue-500 text-white rounded-md transition-colors disabled:opacity-50 shadow-xs"
              >
                <Sparkles className="w-3.5 h-3.5" />
                <span>{generatingKey ? 'Generating...' : 'Auto-Generate Key'}</span>
              </button>
            )}

            <button
              onClick={() => setIsConfigModalOpen(true)}
              className="flex-1 sm:flex-none flex items-center justify-center space-x-1.5 px-3.5 py-2 text-xs font-medium bg-[#0F1115] hover:bg-[#262B34] text-slate-200 rounded-md border border-[#262B34] transition-colors"
            >
              <Settings className="w-3.5 h-3.5 text-slate-400" />
              <span>Settings &amp; Keys</span>
            </button>

            <button
              onClick={onRefresh}
              disabled={loading}
              className="flex items-center justify-center p-2 bg-[#0F1115] hover:bg-[#262B34] text-slate-400 hover:text-white rounded-md border border-[#262B34] transition-colors"
              title="Refresh Storage Status"
            >
              <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
            </button>
          </div>
        </div>
      </div>

      {/* Replicator Key Missing Warning (Unconditionally shown if unconfigured) */}
      {!storageStatus?.config?.hasKey && (
        <div className="bg-[#181B20] border border-amber-900/40 rounded-lg p-5 flex flex-col sm:flex-row sm:items-center justify-between gap-4 text-xs">
          <div className="flex items-center space-x-3.5">
            <AlertCircle className="w-5 h-5 text-amber-400 flex-shrink-0" />
            <div>
              <span className="font-medium text-sm text-slate-200 block">Replicator Key Not Configured</span>
              <span className="text-slate-400 mt-0.5 block">
                Auto-generate or specify a storage replicator key to participate in distributed storage hosting.
              </span>
            </div>
          </div>
          <div className="flex items-center space-x-2 flex-shrink-0">
            <button
              onClick={handleGenerateKey}
              disabled={generatingKey}
              className="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-md text-xs font-semibold transition-colors flex items-center space-x-1.5 disabled:opacity-50 shadow-xs"
            >
              <Sparkles className="w-3.5 h-3.5" />
              <span>{generatingKey ? 'Generating...' : 'Auto-Generate Key'}</span>
            </button>
            <button
              onClick={() => setIsConfigModalOpen(true)}
              className="px-4 py-2 bg-[#0F1115] hover:bg-[#262B34] text-slate-300 rounded-md text-xs font-medium border border-[#262B34] transition-colors"
            >
              Custom Key
            </button>
          </div>
        </div>
      )}

      {/* Honest Offline Callout Banner */}
      {!isNodeRunning && (
        <div className="bg-[#181B20] border border-slate-700/60 rounded-lg p-5 flex flex-col sm:flex-row sm:items-center justify-between gap-4 text-xs">
          <div className="flex items-center space-x-3.5">
            <AlertCircle className="w-5 h-5 text-slate-400 flex-shrink-0" />
            <div>
              <span className="font-medium text-sm text-slate-200 block">Your Sirius node is stopped</span>
              <span className="text-slate-400 mt-0.5 block">
                Storage Replicator service and P2P protocol traffic are inactive until the node is started. Metrics reflect last-known state.
              </span>
            </div>
          </div>
          {onStartNode && (
            <button
              onClick={onStartNode}
              className="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-md text-xs font-semibold transition-colors flex-shrink-0 shadow-xs"
            >
              Start Node
            </button>
          )}
        </div>
      )}

      {/* Action Notification Message */}
      {actionMessage && (
        <div className={`p-4 rounded-lg border text-xs flex items-center justify-between ${
          actionMessage.type === 'success'
            ? 'bg-emerald-950/40 border-emerald-800/50 text-emerald-300'
            : 'bg-rose-950/40 border-rose-800/50 text-rose-300'
        }`}>
          <div className="flex items-center space-x-2.5">
            {actionMessage.type === 'success' ? (
              <CheckCircle2 className="w-4 h-4 text-emerald-400 flex-shrink-0" />
            ) : (
              <AlertCircle className="w-4 h-4 text-rose-400 flex-shrink-0" />
            )}
            <span>{actionMessage.text}</span>
          </div>
          <button
            onClick={() => setActionMessage(null)}
            className="text-slate-400 hover:text-white text-xs px-1.5 py-0.5 rounded hover:bg-[#262B34] transition-colors"
          >
            ✕
          </button>
        </div>
      )}

      {/* 3-Column Cockpit Grid (Flattened Hairline Dividers) */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6 items-stretch">
        
        {/* Left Panel (4 cols): Replicator Identity & On-Chain State */}
        <div className="lg:col-span-4 bg-[#181B20] border border-[#262B34] rounded-lg p-5 flex flex-col justify-between">
          <div>
            <div className="flex items-center justify-between pb-3 border-b border-[#262B34]">
              <h3 className="text-xs font-semibold tracking-wider uppercase text-slate-400 flex items-center space-x-2">
                <Shield className="w-3.5 h-3.5 text-blue-400" />
                <span>Replicator Identity</span>
              </h3>
              <span className="text-xs font-mono text-slate-400">On-Chain</span>
            </div>

            <div className="space-y-3 text-xs pt-1">
              {/* Row 1: Address */}
              <div className="flex justify-between items-center py-1.5 border-b border-[#262B34]">
                <span className="text-slate-400">Address</span>
                <div className="flex items-center space-x-1.5 font-mono">
                  <span className="text-slate-200 font-medium">{address ? truncate(address, 7, 7) : 'Not Configured'}</span>
                  {address && (
                    <button
                      onClick={() => copyToClipboard(address, 'ad')}
                      className="p-1 hover:bg-[#262B34] rounded text-slate-400 hover:text-white transition-colors"
                      title="Copy Full Address"
                    >
                      {copiedField === 'ad' ? <Check className="w-3 h-3 text-emerald-400" /> : <Copy className="w-3 h-3" />}
                    </button>
                  )}
                </div>
              </div>

              {/* Row 2: Balance */}
              <div className="flex justify-between items-center py-1.5 border-b border-[#262B34]">
                <span className="text-slate-400">Balance</span>
                <div className="flex items-center space-x-2">
                  <span className="font-mono font-semibold text-emerald-400">{formatXPX(balanceXPX)}</span>
                  <a
                    href="https://web-wallet.xpxsirius.io/#/"
                    target="_blank"
                    rel="noreferrer"
                    className="p-1 hover:bg-[#262B34] rounded text-slate-400 hover:text-white transition-colors"
                    title="Open Sirius Web Wallet"
                  >
                    <ExternalLink className="w-3 h-3" />
                  </a>
                </div>
              </div>

              {/* Row 3: Storage Units */}
              <div
                onClick={() => setIsUnitsModalOpen(true)}
                className="flex justify-between items-center py-1.5 border-b border-[#262B34] cursor-pointer group"
                title="Click to view Service Units & Rental Economics"
              >
                <span className="text-slate-400 group-hover:text-blue-400 transition-colors flex items-center gap-1">
                  <span>Storage Units (SO)</span>
                  <Info className="w-3 h-3 text-slate-500 group-hover:text-blue-400" />
                </span>
                <span className="font-mono font-medium text-slate-200">
                  {formatXPX(storageStatus?.onChain?.balanceSO || 0, 'SO')}
                </span>
              </div>

              {/* Row 4: Streaming Units */}
              <div
                onClick={() => setIsUnitsModalOpen(true)}
                className="flex justify-between items-center py-1.5 border-b border-[#262B34] cursor-pointer group"
                title="Click to view Service Units & Rental Economics"
              >
                <span className="text-slate-400 group-hover:text-blue-400 transition-colors flex items-center gap-1">
                  <span>Streaming Units (SI)</span>
                  <Info className="w-3 h-3 text-slate-500 group-hover:text-blue-400" />
                </span>
                <span className="font-mono font-medium text-slate-200">
                  {formatXPX(storageStatus?.onChain?.balanceSI || 0, 'SI')}
                </span>
              </div>

              {/* Row 5: Storage Drive Location */}
              <div className="flex justify-between items-center py-1.5">
                <div className="flex items-center gap-1">
                  <span className="text-slate-400">Drive Path</span>
                  <button
                    onClick={() => {
                      setCustomKey('');
                      setCustomStoragePath(storageStatus?.config?.storagePath || '');
                      setIsConfigModalOpen(true);
                    }}
                    className="text-[10px] text-blue-400 hover:underline flex items-center gap-0.5 ml-1 font-sans font-medium"
                  >
                    <FolderOpen className="w-3 h-3 text-slate-400" />
                    <span>Change</span>
                  </button>
                </div>
                <span className="font-mono text-slate-300 max-w-[160px] truncate text-[11px]" title={storageStatus?.config?.resolvedPath}>
                  {storageStatus?.config?.resolvedPath || './chainconfig/data/drives'}
                </span>
              </div>
            </div>
          </div>
        </div>

        {/* Center Panel (4 cols): Interactive Replicator Drive */}
        <div className="lg:col-span-4 bg-[#181B20] border border-[#262B34] rounded-lg p-5 flex flex-col items-center justify-between relative overflow-hidden text-center">
          <div className="w-full flex items-center justify-between pb-3 border-b border-[#262B34]">
            <span className="text-xs font-semibold text-slate-400 uppercase tracking-wider flex items-center space-x-1.5">
              <Activity className="w-3.5 h-3.5 text-blue-400" />
              <span>Drive Capacity</span>
            </span>
            <span className="text-xs font-mono text-slate-400">
              50 GB Target
            </span>
          </div>

          {/* Interactive Circular Storage Drive Graphic with hover effects */}
          <div 
            onClick={() => {
              if (isConfigured) {
                setOnboardResult(null);
                setOnboardError(null);
                setIsOnboardModalOpen(true);
              }
            }}
            className="relative my-4 flex items-center justify-center cursor-pointer group"
            title="Click to Configure / Onboard Capacity"
          >
            {/* SVG Drive Ring */}
            <svg className="w-36 h-36 -rotate-90 group-hover:scale-105 transition-transform duration-300" viewBox="0 0 160 160">
              {/* Background Track */}
              <circle
                cx="80"
                cy="80"
                r="68"
                fill="none"
                stroke="currentColor"
                strokeWidth="8"
                className="text-[#0F1115]"
              />
              {/* Subtle Track 1 */}
              <circle
                cx="80"
                cy="80"
                r="56"
                fill="none"
                stroke="currentColor"
                strokeWidth="1"
                strokeDasharray="4 4"
                className="text-slate-700"
              />
              {/* Active Capacity Fill */}
              <circle
                cx="80"
                cy="80"
                r="68"
                fill="none"
                stroke="url(#emeraldGradient)"
                strokeWidth="8"
                strokeDasharray="427"
                strokeDashoffset={!isNodeRunning || !isRegistered ? "427" : "380"}
                strokeLinecap="round"
                className="transition-all duration-1000"
              />
              <defs>
                <linearGradient id="emeraldGradient" x1="0%" y1="0%" x2="100%" y2="100%">
                  <stop offset="0%" stopColor="#3B82F6" />
                  <stop offset="100%" stopColor="#10B981" />
                </linearGradient>
              </defs>
            </svg>

            {/* Center Spindle & Data Readout */}
            <div className="absolute inset-0 flex flex-col items-center justify-center pointer-events-none">
              <div className="p-2 rounded-full bg-[#0F1115] border border-[#262B34] group-hover:border-blue-500/50 transition-colors duration-300 mb-1">
                <HardDrive className="w-4 h-4 text-slate-400 group-hover:scale-110 group-hover:text-blue-400 transition-all duration-300" />
              </div>
              <span className="text-xl font-bold font-mono text-white leading-tight">
                {driveSizeStr}
              </span>
              <span className="text-[11px] text-blue-400 font-semibold font-mono mt-0.5">
                {shardsCount} Active Shards
              </span>
              <span className="text-[10px] text-slate-400 uppercase tracking-wider mt-0.5 font-medium">
                {!isNodeRunning ? 'Offline' : isRegistered ? 'Replicating' : isConfigured ? 'Standby' : 'Setup Required'}
              </span>
            </div>
          </div>

          {/* Clean Status Footer */}
          <div className="w-full pt-3 border-t border-[#262B34] flex items-center justify-center">
            <span className="text-xs text-slate-400 font-medium flex items-center space-x-1.5">
              <span className={`w-2 h-2 rounded-full ${
                !isNodeRunning ? 'bg-slate-500' : isRegistered ? 'bg-emerald-400' : 'bg-amber-400'
              }`} />
              <span>
                {!isNodeRunning
                  ? 'Node Offline • Storage services inactive'
                  : isRegistered
                  ? 'Active Mainnet Replicator (50 GB)'
                  : isConfigured
                  ? 'Standby • Click disc to onboard'
                  : 'Key required before onboarding'}
              </span>
            </span>
          </div>
        </div>

        {/* Right Panel (4 cols): Drive Metrics & Cache */}
        <div className="lg:col-span-4 bg-[#181B20] border border-[#262B34] rounded-lg p-5 flex flex-col justify-between">
          <div>
            <div className="flex items-center justify-between pb-3 border-b border-[#262B34]">
              <h3 className="text-xs font-semibold tracking-wider uppercase text-slate-400 flex items-center space-x-2">
                <FolderOpen className="w-3.5 h-3.5 text-blue-400" />
                <span>Drive Metrics &amp; Cache</span>
              </h3>
              <span className="text-xs font-mono text-slate-400">Port 7904</span>
            </div>

            <div className="space-y-3 text-xs pt-1">
              <div className="flex justify-between items-center py-1.5 border-b border-[#262B34]">
                <span className="text-slate-400">Active Shards</span>
                <span className="font-mono font-medium text-slate-200">{shardsCount} files</span>
              </div>
              <div className="flex justify-between items-center py-1.5 border-b border-[#262B34]">
                <span className="text-slate-400">Temp Sandboxes</span>
                <span className="font-mono font-medium text-slate-200">{storageStatus?.metrics?.sandboxSizeStr || '0.00 B'}</span>
              </div>
              <div className="flex justify-between items-center py-1.5 border-b border-[#262B34]">
                <span className="text-slate-400">Replicator Protocol</span>
                <span className="font-mono text-slate-200">BitTorrent (DFMS)</span>
              </div>
              <div className="flex justify-between items-center py-1.5">
                <span className="text-slate-400">Drive Target Capacity</span>
                <span className="font-mono font-medium text-slate-200">50 GB</span>
              </div>
            </div>
          </div>

          <div className="pt-4 mt-2 border-t border-[#262B34]">
            <button
              onClick={handleCleanSandboxes}
              disabled={cleaningSandboxes}
              className="w-full py-2 px-3 bg-[#0F1115] hover:bg-[#262B34] text-slate-300 hover:text-white rounded-md text-xs font-medium border border-[#262B34] transition-colors flex items-center justify-center space-x-2"
            >
              <Trash2 className="w-3.5 h-3.5 text-rose-400" />
              <span>{cleaningSandboxes ? 'Cleaning Sandboxes...' : 'Purge Temporary Sandboxes'}</span>
            </button>
          </div>
        </div>
      </div>

      {/* 3. Bottom Horizontal Ribbon: Backbone Replicator Nodes & Test Ports */}
      <div className="bg-[#181B20] border border-[#262B34] rounded-lg p-4 flex flex-col sm:flex-row items-center justify-between gap-3">
        {/* 6 Backbone Replicators */}
        <div className="flex items-center space-x-3 overflow-x-auto max-w-full">
          <div className="flex items-center space-x-1.5 mr-1 flex-shrink-0">
            <Server className="w-3.5 h-3.5 text-blue-400" />
            <span className="text-xs font-semibold text-slate-400 uppercase tracking-wider">
              Backbone Nodes:
            </span>
          </div>
          <div className="flex items-center divide-x divide-[#262B34] text-xs">
            {peers.map((peer, idx) => (
              <div
                key={idx}
                className="px-2.5 flex items-center space-x-1.5 font-mono first:pl-0"
                title={`${peer.name} (${peer.host}): ${peer.latencyMs}ms`}
              >
                <span className={`w-1.5 h-1.5 rounded-full ${peer.isReachable ? 'bg-emerald-400' : 'bg-rose-500'}`} />
                <span className="font-sans font-medium text-slate-300">{peer.name.replace('Mainnet DFMS Node ', 'Node ').replace('Mainnet Storage Node ', 'Node ')}</span>
                <span className={peer.isReachable ? 'text-emerald-400 font-medium' : 'text-slate-500'}>
                  {peer.isReachable ? `${peer.latencyMs}ms` : '—'}
                </span>
              </div>
            ))}
          </div>
        </div>

        <div className="flex items-center space-x-2 flex-shrink-0">
          <button
            onClick={handleTestPorts}
            disabled={testingPorts}
            className="px-3 py-1.5 bg-[#0F1115] hover:bg-[#262B34] text-slate-200 rounded-md text-xs font-medium transition-colors flex items-center space-x-1.5 border border-[#262B34]"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${testingPorts ? 'animate-spin' : ''}`} />
            <span>Test Ports</span>
          </button>
          <button
            onClick={() => setIsPortHelpModalOpen(true)}
            className="p-1.5 text-slate-400 hover:text-white rounded-md hover:bg-[#262B34] transition-colors"
            title="Port Forwarding Help"
          >
            <HelpCircle className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* 4. Modals */}
      {/* Onboard Modal */}
      {isOnboardModalOpen && (
        <div className="fixed inset-0 z-50 bg-black/75 backdrop-blur-xs flex items-center justify-center p-4">
          <div className="bg-[#181B20] border border-[#262B34] w-full max-w-lg rounded-xl shadow-2xl overflow-hidden text-xs">
            <div className="bg-[#0F1115] text-white px-5 py-3.5 flex items-center justify-between border-b border-[#262B34]">
              <div className="flex items-center space-x-2">
                <Zap className="w-4 h-4 text-emerald-400" />
                <span className="font-bold uppercase tracking-wider">Onboard Replicator On-Chain</span>
              </div>
              <button onClick={() => setIsOnboardModalOpen(false)} className="text-slate-400 hover:text-white text-xs px-2 py-0.5 rounded">✕</button>
            </div>

            {onboardResult ? (
              <div className="p-6 space-y-4">
                <div className="p-4 rounded-lg bg-emerald-500/10 border border-emerald-500/30 text-emerald-400 space-y-2">
                  <div className="flex items-center space-x-2">
                    <CheckCircle2 className="w-5 h-5 text-emerald-400 flex-shrink-0" />
                    <span className="font-bold text-sm">Onboarding Broadcasted!</span>
                  </div>
                  <p className="text-xs text-slate-300">{onboardResult.message}</p>
                </div>

                <div className="space-y-2 font-mono text-xs bg-[#0F1115] p-4 rounded-lg border border-[#262B34]">
                  <div className="flex justify-between">
                    <span className="text-slate-400">Allocated Capacity:</span>
                    <span className="font-bold text-white">{onboardResult.capacityGB} GB</span>
                  </div>
                  <div className="flex flex-col space-y-1 pt-1">
                    <span className="text-slate-400">Tx Hash:</span>
                    <div className="flex items-center justify-between bg-[#181B20] border border-[#262B34] p-2 rounded truncate">
                      <span className="text-emerald-400 truncate mr-2">{onboardResult.txHash}</span>
                      <a
                        href={getExplorerTxUrl(onboardResult.txHash)}
                        target="_blank"
                        rel="noreferrer"
                        className="text-blue-400 hover:underline flex items-center space-x-1 flex-shrink-0 font-sans font-semibold"
                      >
                        <span>Explorer</span>
                        <ExternalLink className="w-3 h-3" />
                      </a>
                    </div>
                  </div>
                </div>

                <div className="flex justify-end pt-2">
                  <button
                    onClick={() => setIsOnboardModalOpen(false)}
                    className="px-4 py-2 bg-[#181B20] hover:bg-[#262B34] border border-[#262B34] text-white rounded-md font-semibold transition-colors"
                  >
                    Done
                  </button>
                </div>
              </div>
            ) : (
              <form onSubmit={handleOnboardReplicator} className="p-6 space-y-4">
                {balanceXPX < 1.0 ? (
                  <div className="p-3.5 rounded-lg bg-amber-500/10 border border-amber-500/30 text-amber-400 space-y-2 text-xs">
                    <div className="flex items-center space-x-2 font-bold">
                      <AlertCircle className="w-4 h-4 text-amber-400 flex-shrink-0" />
                      <span>Funding Required: Replicator Has 0.000 XPX</span>
                    </div>
                    <p className="text-[11px] text-slate-300">
                      Send XPX to this Replicator Address to pay network transaction fees and storage collateral deposit:
                    </p>
                    <div className="p-2 bg-[#0F1115] border border-[#262B34] rounded-md flex items-center justify-between font-mono text-[11px] text-white">
                      <span className="truncate mr-2">{address}</span>
                      <button
                        type="button"
                        onClick={() => copyToClipboard(address, 'onboard-addr')}
                        className="p-1 hover:text-blue-400 text-slate-400"
                      >
                        {copiedField === 'onboard-addr' ? <Check className="w-3 h-3 text-emerald-400" /> : <Copy className="w-3 h-3" />}
                      </button>
                    </div>
                  </div>
                ) : (
                  <div className="p-2.5 rounded-lg bg-emerald-500/10 border border-emerald-500/30 text-emerald-400 flex items-center justify-between text-xs font-semibold">
                    <div className="flex items-center space-x-2">
                      <CheckCircle2 className="w-4 h-4 text-emerald-400" />
                      <span>Available Balance: {balanceXPX.toLocaleString()} XPX</span>
                    </div>
                    <span className="text-[10px] text-emerald-300">Ready</span>
                  </div>
                )}

                {onboardError && (
                  <div className="p-3 rounded-lg bg-rose-500/10 border border-rose-500/30 text-rose-400 flex items-center space-x-2 text-xs">
                    <AlertCircle className="w-4 h-4 flex-shrink-0" />
                    <span>{onboardError}</span>
                  </div>
                )}

                <div>
                  <label className="block font-semibold text-slate-300 mb-1">
                    Capacity to Allocate (GB)
                  </label>
                  <div className="grid grid-cols-4 gap-2 mb-2">
                    {[25, 50, 100, 250].map((size) => (
                      <button
                        type="button"
                        key={size}
                        onClick={() => setCapacityGB(size)}
                        className={`py-2 text-xs font-mono font-bold rounded-md border transition-all ${
                          capacityGB === size
                            ? 'bg-blue-600/15 border-blue-500 text-blue-400 font-bold'
                            : 'bg-[#0F1115] border-[#262B34] text-slate-400 hover:text-slate-200'
                        }`}
                      >
                        {size} GB
                      </button>
                    ))}
                  </div>
                  <input
                    type="number"
                    min="1"
                    value={capacityGB}
                    onChange={(e) => setCapacityGB(Math.max(1, Number(e.target.value)))}
                    className="w-full px-3 py-2 bg-[#0F1115] border border-[#262B34] rounded-md font-mono text-xs text-slate-100 placeholder-slate-500 focus:outline-hidden focus:border-blue-500"
                    required
                  />
                </div>

                <div>
                  <label className="block font-semibold text-slate-300 mb-1">
                    Fee Strategy
                  </label>
                  <select
                    value={feeStrategy}
                    onChange={(e: any) => setFeeStrategy(e.target.value)}
                    className="w-full px-3 py-2 bg-[#0F1115] border border-[#262B34] rounded-md text-xs text-slate-100 font-mono focus:outline-hidden focus:border-blue-500"
                  >
                    <option value="low">Slow / Low Fee (Cheapest)</option>
                    <option value="middle">Standard / Balanced (Recommended)</option>
                    <option value="high">Fast / High Priority</option>
                  </select>
                </div>

                <div className="flex justify-end space-x-2 pt-3 border-t border-[#262B34]">
                  <button
                    type="button"
                    onClick={() => setIsOnboardModalOpen(false)}
                    className="px-4 py-2 bg-[#181B20] hover:bg-[#262B34] text-slate-300 rounded-md font-semibold border border-[#262B34] transition-colors"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    disabled={onboarding}
                    className="px-4 py-2 bg-emerald-600 hover:bg-emerald-500 text-white rounded-md font-semibold shadow-xs disabled:opacity-50 flex items-center space-x-1.5 transition-colors"
                  >
                    <Zap className={`w-3.5 h-3.5 ${onboarding ? 'animate-spin' : ''}`} />
                    <span>{onboarding ? 'Broadcasting...' : 'Broadcast Onboarding'}</span>
                  </button>
                </div>
              </form>
            )}
          </div>
        </div>
      )}

      {/* Config / Key Modal */}
      {isConfigModalOpen && (
        <div className="fixed inset-0 z-50 bg-black/75 backdrop-blur-xs flex items-center justify-center p-4">
          <div className="bg-[#181B20] border border-[#262B34] w-full max-w-lg rounded-xl shadow-2xl overflow-hidden text-xs">
            <div className="bg-[#0F1115] text-white px-5 py-3.5 flex items-center justify-between border-b border-[#262B34]">
              <div className="flex items-center space-x-2">
                <HardDrive className="w-4 h-4 text-slate-400" />
                <span className="font-bold uppercase tracking-wider">Storage Replicator Settings</span>
              </div>
              <button onClick={() => setIsConfigModalOpen(false)} className="text-slate-400 hover:text-white text-xs px-2 py-0.5 rounded">✕</button>
            </div>

            <form onSubmit={handleSaveCustomKey} className="p-6 space-y-4">
              {/* Storage Drive Path with DirectoryDropdown */}
              <div className="space-y-1.5">
                <div className="flex items-center justify-between">
                  <label className="block font-semibold text-slate-300">
                    Storage Drive Location
                  </label>
                  {customStoragePath && (
                    <button
                      type="button"
                      onClick={() => setCustomStoragePath('')}
                      className="text-[10px] text-blue-400 hover:underline"
                    >
                      Reset to Default
                    </button>
                  )}
                </div>
                <DirectoryDropdown
                  value={customStoragePath || storageStatus?.config?.defaultStoragePath || './chainconfig/data/drives'}
                  onChange={(newPath) => setCustomStoragePath(newPath)}
                  placeholder={storageStatus?.config?.defaultStoragePath || './chainconfig/data/drives'}
                  prompt="Select Sirius Storage Drive Directory"
                />
                <p className="text-[11px] text-slate-400">
                  By default, shards are stored inside your validator data folder on your external drive. You can choose a custom external folder above.
                </p>
              </div>

              {/* Replicator Key */}
              <div className="pt-2 border-t border-[#262B34] space-y-2">
                <div className="flex items-center justify-between">
                  <label className="block font-semibold text-slate-300">
                    Replicator Private Key (64 hex characters)
                  </label>
                  <button
                    type="button"
                    onClick={handleGenerateKey}
                    disabled={generatingKey}
                    className="text-[11px] text-blue-400 hover:text-blue-300 font-semibold flex items-center space-x-1 transition-colors disabled:opacity-50"
                  >
                    <Sparkles className="w-3 h-3 text-blue-400" />
                    <span>{generatingKey ? 'Generating...' : 'Auto-Generate Key'}</span>
                  </button>
                </div>
                <div className="relative flex items-center">
                  <input
                    type={showCustomKey ? 'text' : 'password'}
                    value={customKey}
                    onChange={(e) => setCustomKey(e.target.value.trim())}
                    placeholder={storageStatus?.config?.hasKey ? "Configured on server (leave blank to keep unchanged)" : "64-character hex key"}
                    className="w-full px-3 py-2 pr-20 bg-[#0F1115] border border-[#262B34] rounded-md font-mono text-xs text-slate-100 placeholder-slate-500 focus:outline-hidden focus:border-blue-500"
                  />
                  <div className="absolute right-1.5 flex items-center space-x-1">
                    <button
                      type="button"
                      onClick={() => setShowCustomKey(!showCustomKey)}
                      className="p-1 text-slate-400 hover:text-white transition-colors"
                      title={showCustomKey ? 'Hide Key' : 'Reveal Key'}
                    >
                      {showCustomKey ? <EyeOff className="w-3.5 h-3.5" /> : <Eye className="w-3.5 h-3.5" />}
                    </button>
                    {customKey && (
                      <button
                        type="button"
                        onClick={() => copyToClipboard(customKey, 'customKey')}
                        className="p-1 text-slate-400 hover:text-white transition-colors"
                        title="Copy Key"
                      >
                        {copiedField === 'customKey' ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5 text-slate-400" />}
                      </button>
                    )}
                  </div>
                </div>
                <p className="text-[11px] text-slate-500">
                  Used for DFMS storage shard replication and P2P storage traffic. Never shared over the network.
                </p>
              </div>

              <div className="flex justify-end space-x-2 pt-3 border-t border-[#262B34]">
                <button
                  type="button"
                  onClick={() => setIsConfigModalOpen(false)}
                  className="px-4 py-2 bg-[#181B20] hover:bg-[#262B34] text-slate-300 rounded-md font-semibold border border-[#262B34] transition-colors"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={savingKey || (customKey.trim().length !== 64 && customKey.trim().length > 0) || (!storageStatus?.config?.hasKey && customKey.trim().length === 0)}
                  className="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-md font-semibold shadow-xs disabled:opacity-50 transition-colors"
                >
                  {savingKey ? 'Saving...' : 'Save Settings'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Port Help Modal */}
      {isPortHelpModalOpen && (
        <div className="fixed inset-0 z-50 bg-black/75 backdrop-blur-xs flex items-center justify-center p-4">
          <div className="bg-[#181B20] border border-[#262B34] w-full max-w-lg rounded-xl shadow-2xl overflow-hidden text-xs">
            <div className="bg-[#0F1115] text-white px-5 py-3.5 flex items-center justify-between border-b border-[#262B34]">
              <span className="font-bold uppercase tracking-wider">Network & Port Forwarding Guide</span>
              <button onClick={() => setIsPortHelpModalOpen(false)} className="text-slate-400 hover:text-white text-xs px-2 py-0.5 rounded">✕</button>
            </div>

            <div className="p-6 space-y-3 text-slate-300 leading-relaxed text-xs">
              <p>
                Our node automatically maps ports <strong>7900</strong> & <strong>7904</strong> via UPnP. If your router has UPnP disabled, add two port forwarding rules pointing to <code className="text-blue-400 font-mono">{portCheck?.localIp || '192.168.1.21'}</code>:
              </p>
              <ul className="list-disc list-inside pl-2 space-y-1 font-mono text-xs text-emerald-400 font-semibold">
                <li>Port 7900 (TCP) → Sirius Consensus</li>
                <li>Port 7904 (TCP & UDP) → Storage Replicator (BitTorrent P2P)</li>
              </ul>
              <div className="flex justify-end pt-3 border-t border-[#262B34]">
                <button
                  onClick={() => setIsPortHelpModalOpen(false)}
                  className="px-4 py-2 bg-[#181B20] hover:bg-[#262B34] text-white rounded-md font-semibold border border-[#262B34] transition-colors"
                >
                  Close
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Service Units & Rental Economics Modal */}
      {isUnitsModalOpen && (
        <div className="fixed inset-0 z-50 bg-black/75 backdrop-blur-xs flex items-center justify-center p-4">
          <div className="bg-[#181B20] border border-[#262B34] w-full max-w-xl rounded-xl shadow-2xl overflow-hidden text-xs">
            <div className="bg-[#0F1115] text-white px-5 py-3.5 flex items-center justify-between border-b border-[#262B34]">
              <div className="flex items-center space-x-2">
                <Sparkles className="w-4 h-4 text-blue-400" />
                <span className="font-bold uppercase tracking-wider">Storage Service Units & Rental Economics</span>
              </div>
              <button onClick={() => setIsUnitsModalOpen(false)} className="text-slate-400 hover:text-white text-xs px-2 py-0.5 rounded">✕</button>
            </div>

            <div className="p-6 space-y-4 max-h-[80vh] overflow-y-auto">
              {/* Active Units Grid */}
              <div className="grid grid-cols-2 gap-3">
                <div className="p-3.5 bg-[#0F1115] border border-[#262B34] rounded-lg space-y-1">
                  <div className="flex items-center justify-between text-slate-400 font-bold uppercase text-[11px]">
                    <span>Storage Units (SO)</span>
                    <HardDrive className="w-3.5 h-3.5 text-slate-400" />
                  </div>
                  <div className="text-xl font-bold font-mono text-white">
                    {(storageStatus?.onChain?.balanceSO || 51200).toLocaleString()} SO
                  </div>
                  <div className="text-[11px] text-slate-400">
                    1 SO = 1 MB &bull; <strong className="text-white">{((storageStatus?.onChain?.balanceSO || 51200) / 1024).toFixed(0)} GB</strong> Total Drive Capacity
                  </div>
                </div>

                <div className="p-3.5 bg-[#0F1115] border border-[#262B34] rounded-lg space-y-1">
                  <div className="flex items-center justify-between text-slate-400 font-bold uppercase text-[11px]">
                    <span>Streaming Units (SI)</span>
                    <Activity className="w-3.5 h-3.5 text-slate-400" />
                  </div>
                  <div className="text-xl font-bold font-mono text-white">
                    {(storageStatus?.onChain?.balanceSI || 102400).toLocaleString()} SI
                  </div>
                  <div className="text-[11px] text-slate-400">
                    1 SI = 1 MB &bull; <strong className="text-white">{((storageStatus?.onChain?.balanceSI || 102400) / 1024).toFixed(0)} GB</strong> Network Bandwidth Throughput
                  </div>
                </div>
              </div>

              {/* How Rental Works Info */}
              <div className="p-3.5 bg-[#0F1115] border border-[#262B34] rounded-lg space-y-2 text-slate-300">
                <div className="flex items-center space-x-2 font-bold text-white">
                  <Info className="w-4 h-4 text-blue-400" />
                  <span>How Do Service Units & Space Rental Work?</span>
                </div>
                <ul className="list-disc list-inside space-y-1 text-[11px] leading-relaxed text-slate-300">
                  <li><strong>Minted Quota:</strong> Your <code className="text-blue-400 font-mono font-bold">SO</code> and <code className="text-blue-400 font-mono font-bold">SI</code> represent your on-chain minted storage rights and bandwidth allowance.</li>
                  <li><strong>Client Rental:</strong> When dApps or users create Drive Contracts to store files, the Sirius network assigns shards to your node.</li>
                  <li><strong>Revenue in XPX:</strong> As your node hosts files and serves download streaming, contract payments and proof-of-storage rewards are paid directly to your Replicator in <strong className="text-emerald-400">XPX</strong>.</li>
                </ul>
              </div>

              {/* Interactive Rental & Space Simulator */}
              <div className="p-4 bg-[#0F1115] border border-[#262B34] rounded-lg space-y-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center space-x-2 font-bold text-white text-xs">
                    <TrendingUp className="w-4 h-4 text-blue-400" />
                    <span>Interactive Network Utilization Simulator</span>
                  </div>
                  <span className="font-mono font-bold text-xs text-blue-400">
                    {simulatedRentedGB} GB Rented ({((simulatedRentedGB / 50) * 100).toFixed(0)}%)
                  </span>
                </div>

                <input
                  type="range"
                  min="0"
                  max="50"
                  step="1"
                  value={simulatedRentedGB}
                  onChange={(e) => setSimulatedRentedGB(Number(e.target.value))}
                  className="w-full h-2 bg-[#181B20] rounded-lg appearance-none cursor-pointer accent-blue-500"
                />

                <div className="grid grid-cols-3 gap-2 pt-1 text-center font-mono">
                  <div className="p-2 bg-[#181B20] rounded-md border border-[#262B34]">
                    <span className="text-[10px] text-slate-400 font-sans block">Active Shards</span>
                    <span className="font-bold text-white text-sm">
                      ~{(simulatedRentedGB * 20).toLocaleString()}
                    </span>
                  </div>
                  <div className="p-2 bg-[#181B20] rounded-md border border-[#262B34]">
                    <span className="text-[10px] text-slate-400 font-sans block">Remaining Quota</span>
                    <span className="font-bold text-emerald-400 text-sm">
                      {50 - simulatedRentedGB} GB
                    </span>
                  </div>
                  <div className="p-2 bg-[#181B20] rounded-md border border-[#262B34]">
                    <span className="text-[10px] text-slate-400 font-sans block">Est. Yield</span>
                    <span className="font-bold text-amber-400 text-sm">
                      +{(simulatedRentedGB * 15).toLocaleString()} XPX/mo
                    </span>
                  </div>
                </div>
              </div>

              {/* Action Buttons */}
              <div className="flex items-center justify-between pt-2 border-t border-[#262B34]">
                <button
                  type="button"
                  onClick={() => {
                    setIsUnitsModalOpen(false);
                    setOnboardResult(null);
                    setOnboardError(null);
                    setIsOnboardModalOpen(true);
                  }}
                  className="px-4 py-2 bg-emerald-600 hover:bg-emerald-500 text-white rounded-md font-bold flex items-center space-x-1.5 shadow-xs transition-colors"
                >
                  <Zap className="w-3.5 h-3.5" />
                  <span>Expand Drive Capacity</span>
                </button>
                <button
                  type="button"
                  onClick={() => setIsUnitsModalOpen(false)}
                  className="px-4 py-2 bg-[#181B20] hover:bg-[#262B34] text-slate-300 rounded-md font-semibold border border-[#262B34] transition-colors"
                >
                  Close
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
