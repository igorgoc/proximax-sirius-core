import React, { useState, useEffect, useRef } from 'react';
import { 
  Sliders, 
  Key, 
  Eye, 
  EyeOff, 
  Check, 
  Copy, 
  HardDrive, 
  Network, 
  FileCode, 
  Save, 
  AlertCircle, 
  CheckCircle2, 
  ShieldCheck, 
  FolderOpen,
  ChevronDown,
  RefreshCw,
  Zap,
  X,
  Loader2
} from 'lucide-react';
import { NodeConfig, HarvestStats, HarvesterStatus, NodeMetrics } from '../types';
import { formatXPXInMillions, formatSiriusAddress } from '../utils/format';
import { DirectoryDropdown } from './DirectoryDropdown';
import { SnapshotSubTab } from './SnapshotSubTab';

interface ConfigTabProps {
  config: NodeConfig | null;
  harvestStats?: HarvestStats | null;
  metrics?: NodeMetrics | null;
  onRefreshConfig: () => void;
}

export const ConfigTab: React.FC<ConfigTabProps> = ({ config, harvestStats, metrics, onRefreshConfig }) => {
  const [activeSubTab, setActiveSubTab] = useState<'general' | 'keys' | 'snapshots' | 'raw'>(() => {
    const param = typeof window !== 'undefined' ? new URLSearchParams(window.location.search).get('subtab') : null;
    if (param === 'keys' || param === 'snapshots' || param === 'raw' || param === 'general') {
      return param;
    }
    if (typeof window !== 'undefined' && window.location.hash === '#snapshots') {
      return 'snapshots';
    }
    return 'general';
  });

  useEffect(() => {
    const handleHash = () => {
      const h = window.location.hash.replace('#', '');
      if (h === 'snapshots' || h === 'keys' || h === 'raw' || h === 'general') {
        setActiveSubTab(h as any);
      } else if (!h) {
        setActiveSubTab('general');
      }
    };
    window.addEventListener('hashchange', handleHash);
    return () => window.removeEventListener('hashchange', handleHash);
  }, []);

  const [showAdvancedPorts, setShowAdvancedPorts] = useState(false);
  const [showBootKey, setShowBootKey] = useState(false);
  const [showHarvestKey, setShowHarvestKey] = useState(false);
  const [copiedField, setCopiedField] = useState<string | null>(null);

  // On-Chain Harvester Verification State
  const [verifyingHarvest, setVerifyingHarvest] = useState(false);
  const [verifyResult, setVerifyResult] = useState<HarvesterStatus | null>(null);
  const [verifyError, setVerifyError] = useState<string | null>(null);

  // Link / Create Harvester Modal State
  const [showLinkModal, setShowLinkModal] = useState(false);
  const [accountPrivKey, setAccountPrivKey] = useState('');
  const [showAccountPrivKey, setShowAccountPrivKey] = useState(false);
  const [actionType, setActionType] = useState<'link_and_register' | 'register_only' | 'unlink'>('link_and_register');
  const [apiNode, setApiNode] = useState('https://aldebaran.xpxsirius.io');
  const [linking, setLinking] = useState(false);
  const [linkResult, setLinkResult] = useState<{ message?: string; txHash?: string; remotePublicKey?: string } | null>(null);
  const [linkError, setLinkError] = useState<string | null>(null);

  // Form State
  const [formData, setFormData] = useState<NodeConfig>(() => ({
    friendlyName: config?.friendlyName || 'Mainnet Peer Node',
    host: config?.host || '',
    bootKey: config?.bootKey || '',
    bootKeySource: 'generated',
    harvestKey: config?.harvestKey || '',
    beneficiary: config?.beneficiary || '0000000000000000000000000000000000000000000000000000000000000000',
    isAutoHarvesting: config?.isAutoHarvesting ?? true,
    maxUnlockedAccounts: config?.maxUnlockedAccounts || 5,
    port: config?.port || 7900,
    apiPort: config?.apiPort || 7901,
    dbrbPort: config?.dbrbPort || 7903,
    dataDirectory: config?.dataDirectory || '/data',
    dataPath: config?.dataPath || './chainconfig/data',
    migrateData: config?.migrateData || false,
    isConfigured: config?.isConfigured ?? true,
  }));

  // Track user edits
  const [isDirty, setIsDirty] = useState(false);
  const [saving, setSaving] = useState(false);
  const [saveStatus, setSaveStatus] = useState<{ type: 'success' | 'error'; message: string } | null>(null);

  // Raw config editor state
  const [rawFile, setRawFile] = useState('config-user.properties');
  const [availableRawFiles, setAvailableRawFiles] = useState<string[]>([
    'config-user.properties',
    'config-harvesting.properties',
    'config-storage.properties',
    'config-node.properties',
    'config-network.properties',
    'config-extensions-server.properties',
    'config-logging-server.properties',
  ]);
  const [rawContent, setRawContent] = useState('');
  const [loadingRaw, setLoadingRaw] = useState(false);
  const [savingRaw, setSavingRaw] = useState(false);


  // Sync external config updates when not dirty, or fetch directly if not provided
  useEffect(() => {
    if (config && !isDirty) {
      setFormData({
        friendlyName: config.friendlyName || 'Mainnet Peer Node',
        host: config.host || '',
        bootKey: config.bootKey || '',
        bootKeySource: 'generated',
        harvestKey: config.harvestKey || '',
        beneficiary: config.beneficiary || '0000000000000000000000000000000000000000000000000000000000000000',
        isAutoHarvesting: config.isAutoHarvesting ?? true,
        maxUnlockedAccounts: config.maxUnlockedAccounts || 5,
        port: config.port || 7900,
        apiPort: config.apiPort || 7901,
        dbrbPort: config.dbrbPort || 7903,
        dataDirectory: config.dataDirectory || '/data',
        dataPath: config.dataPath || './chainconfig/data',
        migrateData: config.migrateData || false,
        isConfigured: config.isConfigured ?? true,
      });
    } else if (!config) {
      fetch(`/api/config?_t=${Date.now()}`)
        .then((res) => res.json())
        .then((data) => {
          if (data && typeof data === 'object' && data.dataPath) {
            setFormData((prev) => {
              if (isDirty) return prev;
              return {
                friendlyName: data.friendlyName || 'Mainnet Peer Node',
                host: data.host || '',
                bootKey: data.bootKey || '',
                bootKeySource: 'generated',
                harvestKey: data.harvestKey || '',
                beneficiary: data.beneficiary || '0000000000000000000000000000000000000000000000000000000000000000',
                isAutoHarvesting: data.isAutoHarvesting ?? true,
                maxUnlockedAccounts: data.maxUnlockedAccounts || 5,
                port: data.port || 7900,
                apiPort: data.apiPort || 7901,
                dbrbPort: data.dbrbPort || 7903,
                dataDirectory: data.dataDirectory || '/data',
                dataPath: data.dataPath || './chainconfig/data',
                migrateData: data.migrateData || false,
                isConfigured: data.isConfigured ?? true,
              };
            });
            onRefreshConfig();
          }
        })
        .catch(console.error);
    }
  }, [config, isDirty]);

  const handleCopy = (text: string, fieldId: string) => {
    navigator.clipboard.writeText(text);
    setCopiedField(fieldId);
    setTimeout(() => setCopiedField(null), 2000);
  };

  const handleFieldChange = (field: keyof NodeConfig, value: any) => {
    setFormData(prev => ({ ...prev, [field]: value }));
    setIsDirty(true);
    setSaveStatus(null);
  };

  const handleDiscard = () => {
    if (config) {
      setFormData({
        friendlyName: config.friendlyName || '',
        host: config.host || '',
        bootKey: config.bootKey || '',
        bootKeySource: 'generated',
        harvestKey: config.harvestKey || '',
        beneficiary: config.beneficiary || '0000000000000000000000000000000000000000000000000000000000000000',
        isAutoHarvesting: config.isAutoHarvesting ?? true,
        maxUnlockedAccounts: config.maxUnlockedAccounts || 5,
        port: config.port || 7900,
        apiPort: config.apiPort || 7901,
        dbrbPort: config.dbrbPort || 7903,
        dataDirectory: config.dataDirectory || '/data',
        dataPath: config.dataPath || './chainconfig/data',
        migrateData: config.migrateData || false,
        isConfigured: config.isConfigured ?? true,
      });
    }
    setIsDirty(false);
    setSaveStatus(null);
  };

  const handleSave = async () => {
    if (!formData.dataPath) return;
    setSaving(true);
    setSaveStatus(null);
    try {
      const res = await fetch('/api/config/save', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(formData),
      });
      const data = await res.json();
      if (!res.ok || (data.status !== 'success' && !data.success)) {
        throw new Error(data.message || 'Failed to save configuration settings');
      }

      // Read-After-Write Verification: re-read directly from disk via server endpoint
      const verifyRes = await fetch(`/api/config?_t=${Date.now()}`);
      if (!verifyRes.ok) {
        throw new Error('Settings saved, but read-after-write verification request failed');
      }
      const verified = await verifyRes.json();
      if (formData.dataPath && verified.dataPath !== formData.dataPath) {
        throw new Error(`Read-after-write mismatch: On-disk path is "${verified.dataPath}", expected "${formData.dataPath}".`);
      }

      setSaveStatus({ type: 'success', message: 'Settings saved and verified on disk. Restart node to apply changes.' });
      setIsDirty(false);
      onRefreshConfig();
    } catch (e: any) {
      setSaveStatus({ type: 'error', message: e.message || 'Network error saving configuration.' });
    } finally {
      setSaving(false);
    }
  };

  // Fetch live on-chain harvest verification
  const fetchLiveHarvestStatus = async (targetKey?: string) => {
    const keyToQuery = targetKey || config?.harvestPublicKey || config?.harvestAddress || formData.harvestKey;
    if (!keyToQuery) return;

    setVerifyingHarvest(true);
    setVerifyError(null);
    try {
      const res = await fetch(`/api/harvesting/check?account=${encodeURIComponent(keyToQuery.trim())}&apiNode=${encodeURIComponent(apiNode)}`);
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to query harvester status');
      setVerifyResult(data);
    } catch (err: any) {
      setVerifyError(err.message || 'Error querying on-chain harvesting status');
    } finally {
      setVerifyingHarvest(false);
    }
  };

  const hasCheckedHarvestRef = useRef(false);

  // Query on-chain status strictly once when opening the app
  useEffect(() => {
    if (!hasCheckedHarvestRef.current && (config?.harvestPublicKey || config?.harvestAddress || formData.harvestKey)) {
      hasCheckedHarvestRef.current = true;
      fetchLiveHarvestStatus();
    }
  }, [config?.harvestPublicKey, config?.harvestAddress, formData.harvestKey]);

  // Generate random boot key (P2P mesh identity)
  const handleGenerateBootKey = async () => {
    try {
      const res = await fetch('/api/keys/generate', { method: 'POST' });
      const data = await res.json();
      if (data && data.privateKey) {
        handleFieldChange('bootKey', data.privateKey);
      }
    } catch (e) {
      console.error(e);
    }
  };

  // Open modal with smart default selection based on current on-chain link status
  const handleOpenLinkModal = () => {
    setLinkError(null);
    setLinkResult(null);
    setAccountPrivKey('');
    if (verifyResult?.isLinked) {
      setActionType('register_only');
    } else {
      setActionType('link_and_register');
    }
    setShowLinkModal(true);
  };

  // Execute Harvester Link Transaction
  const handleExecuteHarvestLink = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!accountPrivKey || accountPrivKey.trim().length !== 64) {
      setLinkError('Staked account private key must be exactly 64 hexadecimal characters.');
      return;
    }

    setLinking(true);
    setLinkError(null);
    setLinkResult(null);

    try {
      const res = await fetch('/api/harvesting/link', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          accountPrivateKey: accountPrivKey.trim(),
          remoteHarvestKey: formData.harvestKey ? formData.harvestKey.trim() : undefined,
          action: actionType,
          apiNode: apiNode,
        }),
      });

      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error || 'Failed to execute harvesting link action');
      }

      setLinkResult(data);
      if (data.remotePrivateKey) {
        handleFieldChange('harvestKey', data.remotePrivateKey);
      }
      onRefreshConfig();
      fetchLiveHarvestStatus(data.remotePublicKey || config?.harvestPublicKey);
    } catch (err: any) {
      setLinkError(err.message || 'Error broadcasting link transaction');
    } finally {
      setLinking(false);
    }
  };

  // Fetch raw file
  const fetchRawFile = async (filename: string) => {
    setLoadingRaw(true);
    try {
      const res = await fetch(`/api/config/raw?file=${encodeURIComponent(filename)}`);
      if (res.ok) {
        const data = await res.json();
        setRawContent(data.content || '');
      }
    } catch (e) {
      console.error(e);
    } finally {
      setLoadingRaw(false);
    }
  };

  const handleSaveRaw = async () => {
    setSavingRaw(true);
    try {
      const res = await fetch(`/api/config/raw?file=${encodeURIComponent(rawFile)}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ file: rawFile, content: rawContent }),
      });
      const data = await res.json();
      if (res.ok && (data.status === 'success' || data.success)) {
        setSaveStatus({ type: 'success', message: `Saved ${rawFile} successfully.` });
        onRefreshConfig();
      } else {
        setSaveStatus({ type: 'error', message: data.message || data.error || `Failed to save ${rawFile}.` });
      }
    } catch (e: any) {
      setSaveStatus({ type: 'error', message: e.message || 'Error saving raw config.' });
    } finally {
      setSavingRaw(false);
    }
  };

  useEffect(() => {
    if (activeSubTab === 'raw') {
      fetch('/api/config/files')
        .then((res) => res.json())
        .then((files: string[]) => {
          if (Array.isArray(files) && files.length > 0) {
            const filtered = files.filter(
              (f) => !f.endsWith('.template') && (f.endsWith('.properties') || f.endsWith('.json'))
            );
            if (!filtered.includes('config-storage.properties')) {
              filtered.push('config-storage.properties');
            }
            setAvailableRawFiles(filtered);
          }
        })
        .catch(console.error);
    }
  }, [activeSubTab]);

  useEffect(() => {
    if (activeSubTab === 'raw') {
      fetchRawFile(rawFile);
    }
  }, [activeSubTab, rawFile]);


  return (
    <div className="space-y-6 max-w-5xl mx-auto px-4 py-6 select-none">
      
      {/* 1. Header & Sub-Navigation */}
      <section className="bg-[#181B20] border border-[#262B34] rounded-lg p-5">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div>
            <h2 className="text-sm font-semibold text-white tracking-tight flex items-center space-x-2">
              <Sliders className="w-4 h-4 text-blue-400" />
              <span>Node & Validator Settings</span>
            </h2>
            <p className="text-xs text-slate-400 mt-0.5">
              Manage blockchain data paths, harvesting credentials, and network communication ports.
            </p>
          </div>

          {/* Sub-Tab Switcher */}
          <div className="flex items-center space-x-1 bg-[#0F1115] p-1 rounded-md border border-[#262B34]">
            <button
              onClick={() => setActiveSubTab('general')}
              className={`px-3 py-1 text-xs font-medium rounded transition-colors ${
                activeSubTab === 'general' ? 'bg-[#262B34] text-white' : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              General & Data Path
            </button>
            <button
              onClick={() => setActiveSubTab('keys')}
              className={`px-3 py-1 text-xs font-medium rounded transition-colors ${
                activeSubTab === 'keys' ? 'bg-[#262B34] text-white' : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              Validator Keys
            </button>
            <button
              onClick={() => setActiveSubTab('snapshots')}
              className={`px-3 py-1 text-xs font-medium rounded transition-colors ${
                activeSubTab === 'snapshots' ? 'bg-[#262B34] text-white' : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              Snapshots
            </button>
            <button
              onClick={() => setActiveSubTab('raw')}
              className={`px-3 py-1 text-xs font-mono rounded transition-colors ${
                activeSubTab === 'raw' ? 'bg-[#262B34] text-white' : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              Raw Config
            </button>
          </div>
        </div>
      </section>

      {/* 2. Feedback Notification Banner */}
      {saveStatus && (
        <div className={`p-4 rounded-lg border text-xs flex items-center space-x-2.5 ${
          saveStatus.type === 'success' 
            ? 'bg-emerald-950/40 border-emerald-800/60 text-emerald-300' 
            : 'bg-rose-950/40 border-rose-800/60 text-rose-300'
        }`}>
          {saveStatus.type === 'success' ? <CheckCircle2 className="w-4 h-4 text-emerald-400 flex-shrink-0" /> : <AlertCircle className="w-4 h-4 text-rose-400 flex-shrink-0" />}
          <span>{saveStatus.message}</span>
        </div>
      )}

      {/* Structural Loading Guard: Lock form and Save action until real config is loaded from disk */}
      {!config ? (
        <div className="bg-[#181B20] border border-[#262B34] rounded-lg p-12 flex flex-col items-center justify-center text-center space-y-3">
          <Loader2 className="w-6 h-6 animate-spin text-blue-400" />
          <div className="space-y-1">
            <p className="text-sm font-semibold text-slate-200">Loading Node Configuration</p>
            <p className="text-xs text-slate-500">Retrieving configuration directly from disk. Form inputs and save actions are locked.</p>
          </div>
        </div>
      ) : (
        <>
          {/* 3. Sub-Tab 1: General & Data Path */}
      {activeSubTab === 'general' && (
        <div className="space-y-6">
          <section className="bg-[#181B20] border border-[#262B34] rounded-lg p-5 space-y-4">
            <h3 className="text-xs font-semibold tracking-wider uppercase text-slate-400 flex items-center space-x-2">
              <HardDrive className="w-3.5 h-3.5 text-blue-400" />
              <span>Blockchain Data Directory</span>
            </h3>

            <div className="space-y-1.5">
              <DirectoryDropdown
                label="Blockchain Data Location"
                value={formData.dataPath}
                onChange={(val) => handleFieldChange('dataPath', val)}
                placeholder="/Volumes/SSD/Sirius_data or ./chainconfig/data"
                prompt="Select Sirius Blockchain Data Directory"
              />
              <p className="text-[11px] text-slate-500">
                Stores blockchain binary block containers (`blocks.dat`, `blocks.idx`, `statements.dat`) on your fast external SSD.
              </p>
            </div>

            <div className="pt-3 border-t border-[#262B34]/60">
              <div className="bg-[#0F1115] border border-[#262B34] rounded p-3 text-xs text-slate-400 space-y-1">
                <div className="flex justify-between">
                  <span>Active Architecture:</span>
                  <span className="font-mono text-slate-200">Chunked binary</span>
                </div>
                <div className="flex justify-between">
                  <span>Total Inodes Used:</span>
                  <span className="font-mono text-emerald-400 font-semibold">
                    {metrics?.blockHeight ? Math.ceil(metrics.blockHeight / 65536) : (harvestStats?.lastHarvestedHeight ? Math.ceil(harvestStats.lastHarvestedHeight / 65536) : 212)} folders, {(metrics?.blockHeight ? Math.ceil(metrics.blockHeight / 65536) : (harvestStats?.lastHarvestedHeight ? Math.ceil(harvestStats.lastHarvestedHeight / 65536) : 212)) * 4} files
                  </span>
                </div>
              </div>
            </div>
          </section>

          <section className="bg-[#181B20] border border-[#262B34] rounded-lg p-5 space-y-4">
            <h3 className="text-xs font-semibold tracking-wider uppercase text-slate-400">
              Node Identification
            </h3>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label className="text-xs text-slate-300 font-medium block mb-1.5">
                  Friendly Node Name
                </label>
                <input
                  type="text"
                  value={formData.friendlyName}
                  onChange={(e) => handleFieldChange('friendlyName', e.target.value)}
                  placeholder="My Sirius Validator"
                  className="w-full bg-[#0F1115] border border-[#262B34] rounded px-3 py-2 text-xs text-slate-200 focus:outline-none focus:border-blue-500"
                />
              </div>

              <div>
                <label className="text-xs text-slate-300 font-medium block mb-1.5">
                  Public Host / IP Address
                </label>
                <input
                  type="text"
                  value={formData.host}
                  onChange={(e) => handleFieldChange('host', e.target.value)}
                  placeholder="Auto-detected or custom IP"
                  className="w-full bg-[#0F1115] border border-[#262B34] rounded px-3 py-2 text-xs text-slate-200 font-mono focus:outline-none focus:border-blue-500"
                />
              </div>
            </div>
          </section>

          {/* Collapsible Advanced Section: Network Communication Ports */}
          <section className="bg-[#181B20] border border-[#262B34] rounded-lg overflow-hidden">
            <button
              type="button"
              onClick={() => setShowAdvancedPorts(!showAdvancedPorts)}
              className="w-full p-4 flex items-center justify-between text-left hover:bg-[#262B34]/30 transition-colors"
            >
              <div className="flex items-center space-x-2">
                <Network className="w-3.5 h-3.5 text-slate-400" />
                <span className="text-xs font-semibold tracking-wider uppercase text-slate-400">
                  Advanced: Network Communication Ports
                </span>
              </div>
              <div className="flex items-center space-x-2">
                <span className="text-[11px] text-slate-500 font-mono">
                  :{formData.port} / :{formData.apiPort} / :{formData.dbrbPort}
                </span>
                <ChevronDown className={`w-4 h-4 text-slate-400 transition-transform ${showAdvancedPorts ? 'rotate-180' : ''}`} />
              </div>
            </button>

            {showAdvancedPorts && (
              <div className="p-5 pt-2 border-t border-[#262B34] space-y-4">
                <p className="text-[11px] text-slate-500">
                  Custom port configuration for specialized network firewalls or multi-instance nodes. Typically left at defaults.
                </p>
                <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                  <div>
                    <label className="text-xs text-slate-300 font-medium block mb-1.5">
                      P2P Gossip Port
                    </label>
                    <input
                      type="number"
                      value={formData.port}
                      onChange={(e) => handleFieldChange('port', parseInt(e.target.value) || 7900)}
                      className="w-full bg-[#0F1115] border border-[#262B34] rounded px-3 py-2 text-xs text-slate-200 font-mono focus:outline-none focus:border-blue-500"
                    />
                    <span className="text-[11px] text-slate-500 mt-1 block">Default: 7900</span>
                  </div>

                  <div>
                    <label className="text-xs text-slate-300 font-medium block mb-1.5">
                      REST API Gateway Port
                    </label>
                    <input
                      type="number"
                      value={formData.apiPort}
                      onChange={(e) => handleFieldChange('apiPort', parseInt(e.target.value) || 7901)}
                      className="w-full bg-[#0F1115] border border-[#262B34] rounded px-3 py-2 text-xs text-slate-200 font-mono focus:outline-none focus:border-blue-500"
                    />
                    <span className="text-[11px] text-slate-500 mt-1 block">Default: 7901 / 3000</span>
                  </div>

                  <div>
                    <label className="text-xs text-slate-300 font-medium block mb-1.5">
                      Fast Finality (DBRB) Port
                    </label>
                    <input
                      type="number"
                      value={formData.dbrbPort}
                      onChange={(e) => handleFieldChange('dbrbPort', parseInt(e.target.value) || 7903)}
                      className="w-full bg-[#0F1115] border border-[#262B34] rounded px-3 py-2 text-xs text-slate-200 font-mono focus:outline-none focus:border-blue-500"
                    />
                    <span className="text-[11px] text-slate-500 mt-1 block">Default: 7903</span>
                  </div>
                </div>
              </div>
            )}
          </section>
        </div>
      )}

      {/* 4. Sub-Tab 2: Validator Keys */}
      {activeSubTab === 'keys' && (
        <div className="space-y-6">
          <section className="bg-[#181B20] border border-[#262B34] rounded-lg p-5 space-y-5">
            <div className="flex items-center justify-between">
              <h3 className="text-xs font-semibold tracking-wider uppercase text-slate-400 flex items-center space-x-2">
                <Key className="w-3.5 h-3.5 text-blue-400" />
                <span>Harvesting Credentials (POS+)</span>
              </h3>
              <span className="text-[11px] text-slate-500">
                Encrypted in `config-harvesting.properties`
              </span>
            </div>

            {/* Live On-Chain Harvester Status & Verification */}
            <div className="bg-[#0F1115] border border-[#262B34] rounded-lg p-4 space-y-3">
              <div className="flex items-center justify-between">
                <div className="flex items-center space-x-2">
                  <ShieldCheck className="w-4 h-4 text-blue-400" />
                  <span className="text-xs font-semibold text-slate-200">On-Chain Verification Status</span>
                </div>
                <button
                  type="button"
                  onClick={() => fetchLiveHarvestStatus()}
                  disabled={verifyingHarvest}
                  className="px-2.5 py-1 bg-[#181B20] hover:bg-[#262B34] border border-[#262B34] rounded text-slate-400 hover:text-white text-xs flex items-center space-x-1.5 transition-colors"
                >
                  <RefreshCw className={`w-3 h-3 ${verifyingHarvest ? 'animate-spin text-blue-400' : ''}`} />
                  <span>{verifyingHarvest ? 'Verifying...' : 'Re-verify'}</span>
                </button>
              </div>

              {verifyResult ? (
                <div className="space-y-2 text-xs">
                  <div className="flex items-center justify-between">
                    <span className="text-slate-400">Mainnet Status</span>
                    <span className={`font-medium px-2 py-0.5 rounded text-[11px] ${
                      verifyResult.isLinked 
                        ? 'bg-emerald-950/60 text-emerald-400 border border-emerald-800/40' 
                        : 'bg-amber-950/60 text-amber-400 border border-amber-800/40'
                    }`}>
                      {verifyResult.isLinked ? '✓ On-Chain Linked & Active' : '⚠ Remote Key Not Linked on Mainnet'}
                    </span>
                  </div>
                  {/* Staked Owner Account (Holding Funds) */}
                  {(verifyResult.linkedAddress || verifyResult.accountAddress) && (
                    <div className="flex flex-col sm:flex-row sm:items-center justify-between border-t border-[#262B34]/60 pt-2 gap-1">
                      <span className="text-slate-400">Staked Account</span>
                      <span className="font-mono text-slate-200 text-xs font-medium">
                        {formatSiriusAddress(verifyResult.accountType === 2 ? (verifyResult.linkedAddress || verifyResult.accountAddress) : verifyResult.accountAddress)}
                      </span>
                    </div>
                  )}

                  {/* Remote Harvester Block Signer (Proxy Key) */}
                  {verifyResult.accountType === 2 && verifyResult.accountAddress && (
                    <div className="flex flex-col sm:flex-row sm:items-center justify-between border-t border-[#262B34]/60 pt-2 gap-1">
                      <span className="text-slate-400">Remote Harvester Signer</span>
                      <span className="font-mono text-slate-400 text-xs">
                        {formatSiriusAddress(verifyResult.accountAddress)}
                      </span>
                    </div>
                  )}
                  {(verifyResult.linkedBalanceXPX || verifyResult.balanceXPX) && (
                    <div className="flex items-center justify-between border-t border-[#262B34]/60 pt-2">
                      <span className="text-slate-400">Staked Balance</span>
                      <span className="font-mono text-emerald-400 font-semibold">
                        {formatXPXInMillions(verifyResult.linkedBalanceXPX || verifyResult.balanceXPX)}
                      </span>
                    </div>
                  )}
                  {verifyResult.isCommitteeHarvester !== undefined && (
                    <div className="flex items-center justify-between border-t border-[#262B34]/60 pt-2">
                      <span className="text-slate-400">POS+ Committee Standing</span>
                      <span className="text-slate-200 font-medium">
                        {verifyResult.isCommitteeHarvester ? 'Eligible Voting Harvester' : 'Standard Node'}
                      </span>
                    </div>
                  )}
                </div>
              ) : verifyError ? (
                <div className="flex items-start space-x-2.5 text-xs bg-[#14161B] border border-[#262B34] rounded-lg p-3">
                  <AlertCircle className="w-4 h-4 text-slate-400 mt-0.5 flex-shrink-0" />
                  <div className="space-y-1">
                    <p className="text-slate-300 font-medium">Remote Verification Unavailable</p>
                    <p className="text-slate-400 text-[11px] leading-relaxed">
                      Your Sirius node is running normally. The public REST endpoint used for on-chain verification is temporarily unreachable — this does not affect syncing or harvesting.
                    </p>
                  </div>
                </div>
              ) : (
                <p className="text-xs text-slate-500">
                  Click Re-verify to query on-chain account standing and POS+ staking eligibility from the live Sirius network.
                </p>
              )}
            </div>

            <div>
              <div className="flex items-center justify-between mb-1.5">
                <label className="text-xs text-slate-300 font-medium">
                  Harvester Private Key (Mandatory)
                </label>
                <div className="flex items-center space-x-3">
                  <button
                    type="button"
                    onClick={async () => {
                      try {
                        const res = await fetch('/api/keys/generate', { method: 'POST' });
                        const data = await res.json();
                        if (data && data.privateKey) {
                          handleFieldChange('harvestKey', data.privateKey);
                        }
                      } catch (err) {
                        console.error('Failed to generate key', err);
                      }
                    }}
                    className="text-xs text-slate-400 hover:text-white font-medium flex items-center space-x-1 transition-colors"
                  >
                    <span>+ Generate Key</span>
                  </button>
                  <button
                    type="button"
                    onClick={handleOpenLinkModal}
                    className="text-xs text-blue-400 hover:text-blue-300 font-semibold flex items-center space-x-1 transition-colors"
                  >
                    <Zap className="w-3.5 h-3.5 text-blue-400" />
                    <span>Create Delegated Harvester Account</span>
                  </button>
                </div>
              </div>
              <div className="relative flex items-center">
                <input
                  type={showHarvestKey ? 'text' : 'password'}
                  value={formData.harvestKey}
                  onChange={(e) => handleFieldChange('harvestKey', e.target.value.trim())}
                  placeholder={config?.hasHarvestKey ? "Configured on server (leave blank to keep unchanged)" : "64-character hexadecimal private key"}
                  className="w-full bg-[#0F1115] border border-[#262B34] rounded px-3 py-2 pr-20 text-xs text-slate-200 font-mono focus:outline-none focus:border-blue-500"
                />
                <div className="absolute right-1.5 flex items-center space-x-1">
                  <button
                    type="button"
                    onClick={() => setShowHarvestKey(!showHarvestKey)}
                    className="p-1 text-slate-400 hover:text-white transition-colors"
                    title={showHarvestKey ? 'Hide Key' : 'Reveal Key'}
                  >
                    {showHarvestKey ? <EyeOff className="w-3.5 h-3.5" /> : <Eye className="w-3.5 h-3.5" />}
                  </button>
                  {formData.harvestKey && (
                    <button
                      type="button"
                      onClick={() => handleCopy(formData.harvestKey, 'harvestKey')}
                      className="p-1 text-slate-400 hover:text-white transition-colors"
                      title="Copy Key"
                    >
                      {copiedField === 'harvestKey' ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                    </button>
                  )}
                </div>
              </div>
            </div>

          </section>

          <section className="bg-[#181B20] border border-[#262B34] rounded-lg p-5 space-y-4">
            <div className="flex items-center justify-between">
              <h3 className="text-xs font-semibold tracking-wider uppercase text-slate-400 flex items-center space-x-2">
                <Key className="w-3.5 h-3.5 text-blue-400" />
                <span>Node boot private key</span>
              </h3>
              <span className="text-[11px] text-slate-500">
                Stored in `config-user.properties`
              </span>
            </div>

            <div>
              <div className="flex items-center justify-between mb-1.5">
                <label className="text-xs text-slate-300 font-medium">
                  Node boot private key
                </label>
                <button
                  type="button"
                  onClick={handleGenerateBootKey}
                  className="text-xs text-blue-400 hover:text-blue-300 font-semibold flex items-center space-x-1 transition-colors"
                >
                  <RefreshCw className="w-3 h-3 text-blue-400" />
                  <span>Generate New Boot Key</span>
                </button>
              </div>
              <div className="relative flex items-center">
                <input
                  type={showBootKey ? 'text' : 'password'}
                  value={formData.bootKey}
                  onChange={(e) => handleFieldChange('bootKey', e.target.value.trim())}
                  placeholder={config?.hasBootKey ? "Configured on server (leave blank to keep unchanged)" : "64-character transport private key"}
                  className="w-full bg-[#0F1115] border border-[#262B34] rounded px-3 py-2 pr-20 text-xs text-slate-200 font-mono focus:outline-none focus:border-blue-500"
                />
                <div className="absolute right-1.5 flex items-center space-x-1">
                  <button
                    type="button"
                    onClick={() => setShowBootKey(!showBootKey)}
                    className="p-1 text-slate-400 hover:text-white transition-colors"
                  >
                    {showBootKey ? <EyeOff className="w-3.5 h-3.5" /> : <Eye className="w-3.5 h-3.5" />}
                  </button>
                  {formData.bootKey && (
                    <button
                      type="button"
                      onClick={() => handleCopy(formData.bootKey, 'bootKey')}
                      className="p-1 text-slate-400 hover:text-white transition-colors"
                    >
                      {copiedField === 'bootKey' ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                    </button>
                  )}
                </div>
              </div>
              <p className="text-[11px] text-slate-500 mt-1">
                Strict separation invariant: The transport identity is distinct from your harvesting staking key.
              </p>
            </div>
          </section>
        </div>
      )}


      {/* 6. Sub-Tab 4: Raw Configuration Files */}
      {activeSubTab === 'raw' && (
        <section className="bg-[#181B20] border border-[#262B34] rounded-lg p-5 space-y-4">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
            <h3 className="text-xs font-semibold tracking-wider uppercase text-slate-400 flex items-center space-x-2">
              <FileCode className="w-3.5 h-3.5 text-blue-400" />
              <span>Direct Configuration Editor</span>
            </h3>

            <div className="flex items-center space-x-2">
              <select
                value={rawFile}
                onChange={(e) => setRawFile(e.target.value)}
                className="bg-[#0F1115] border border-[#262B34] rounded px-2.5 py-1.5 text-xs text-slate-200 font-mono focus:outline-none focus:border-blue-500"
              >
                {availableRawFiles.map((f) => (
                  <option key={f} value={f}>
                    {f}
                  </option>
                ))}
              </select>


              <button
                onClick={handleSaveRaw}
                disabled={savingRaw || loadingRaw}
                className="px-3 py-1.5 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded text-xs font-medium flex items-center space-x-1"
              >
                <Save className="w-3.5 h-3.5" />
                <span>{savingRaw ? 'Saving...' : 'Save File'}</span>
              </button>
            </div>
          </div>

          <textarea
            value={rawContent}
            onChange={(e) => setRawContent(e.target.value)}
            disabled={loadingRaw}
            rows={14}
            className="w-full bg-[#0F1115] border border-[#262B34] rounded p-3 text-xs text-slate-200 font-mono focus:outline-none focus:border-blue-500 resize-none leading-relaxed"
          />
        </section>
      )}

      {/* 6. Snapshots Management Sub-Tab */}
      {activeSubTab === 'snapshots' && (
        <SnapshotSubTab
          currentDataPath={formData.dataPath || config?.dataPath || './chainconfig/data'}
          blockHeight={metrics?.blockHeight}
          onRefreshConfig={onRefreshConfig}
        />
      )}

      {/* 7. Bottom Action Bar (Permanently accessible on General and Keys) */}
      {(activeSubTab === 'general' || activeSubTab === 'keys') && (
        <section className="bg-[#181B20] border border-[#262B34] shadow-md rounded-lg p-4 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <div className="flex items-center space-x-2">
            {isDirty ? (
              <div className="flex items-center space-x-2 text-amber-400 text-xs font-medium">
                <AlertCircle className="w-4 h-4 text-amber-400" />
                <span>You have unsaved changes.</span>
              </div>
            ) : (
              <div className="flex items-center space-x-2 text-slate-500 text-xs">
                <CheckCircle2 className="w-4 h-4 text-slate-500" />
                <span>Configuration is saved and up to date.</span>
              </div>
            )}
          </div>

          <div className="flex items-center space-x-2.5">
            <button
              onClick={handleDiscard}
              disabled={!config || !isDirty || saving}
              className="px-3 py-1.5 bg-[#0F1115] hover:bg-[#262B34] disabled:opacity-40 disabled:hover:bg-[#0F1115] text-slate-300 rounded text-xs font-medium border border-[#262B34] transition-colors"
            >
              Discard Changes
            </button>
            <button
              onClick={handleSave}
              disabled={!config || !isDirty || saving}
              className="px-4 py-1.5 bg-blue-600 hover:bg-blue-500 disabled:opacity-40 disabled:hover:bg-blue-600 text-white rounded text-xs font-semibold shadow-xs flex items-center space-x-1.5 transition-colors"
            >
              <Save className="w-3.5 h-3.5" />
              <span>{saving ? 'Saving...' : 'Save Settings'}</span>
            </button>
          </div>
        </section>
      )}
        </>
      )}

      {/* 8. Link / Create Harvester Key Modal (Bitcoin Core Modal Style) */}
      {showLinkModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-3 sm:p-5 bg-black/80 backdrop-blur-xs overflow-y-auto animate-fadeIn">
          <div className="bg-[#181B20] border border-[#262B34] rounded-xl max-w-lg w-full max-h-[90vh] shadow-2xl flex flex-col my-auto text-slate-200 overflow-hidden">
            {/* Modal Header (Fixed at top) */}
            <div className="p-4 border-b border-[#262B34] flex items-center justify-between bg-[#0F1115] flex-shrink-0">
              <div className="flex items-center space-x-2.5">
                <div className="p-1.5 bg-blue-950/60 text-blue-400 border border-blue-800/40 rounded-lg">
                  <Key className="w-4 h-4" />
                </div>
                <div>
                  <h3 className="text-sm font-bold text-white tracking-tight">
                    Create Delegated Harvester Account
                  </h3>
                  <p className="text-[11px] text-slate-400 font-mono">
                    Announce on-chain AccountLink &amp; Harvester transactions to Sirius Mainnet
                  </p>
                </div>
              </div>
              <button
                type="button"
                onClick={() => setShowLinkModal(false)}
                className="p-1.5 text-slate-400 hover:text-white rounded-lg hover:bg-[#262B34] transition-colors"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            {/* Modal Form */}
            <form onSubmit={handleExecuteHarvestLink} className="flex flex-col flex-1 min-h-0 overflow-hidden">
              <div className="p-5 space-y-4 text-xs overflow-y-auto scrollbar-thin flex-1 min-h-0">
              
              {/* 1. Action Mode Selection Cards with Plain-Language Descriptions */}
              <div className="space-y-2">
                <label className="block font-medium text-slate-300">
                  Select Action Mode
                </label>
                <div className="grid grid-cols-1 gap-2">
                  
                  {/* Mode A: Create Delegated Harvester */}
                  <div
                    onClick={() => setActionType('link_and_register')}
                    className={`p-3 rounded-lg border cursor-pointer transition-all ${
                      actionType === 'link_and_register'
                        ? 'bg-blue-950/30 border-blue-500 text-white'
                        : 'bg-[#0F1115] border-[#262B34] text-slate-300 hover:border-slate-600'
                    }`}
                  >
                    <div className="flex items-center justify-between">
                      <span className="font-semibold text-xs flex items-center space-x-2">
                        <span className={`w-3.5 h-3.5 rounded-full border flex items-center justify-center ${
                          actionType === 'link_and_register' ? 'border-blue-400 bg-blue-500' : 'border-slate-600'
                        }`}>
                          {actionType === 'link_and_register' && <span className="w-1.5 h-1.5 rounded-full bg-white" />}
                        </span>
                        <span>Create Delegated Harvester</span>
                      </span>
                      {verifyResult?.isLinked && (
                        <span className="text-[10px] bg-amber-950/60 text-amber-400 border border-amber-800/40 px-1.5 py-0.5 rounded">
                          Will replace current linked harvester
                        </span>
                      )}
                    </div>
                    <p className="text-[11px] text-slate-400 mt-1 pl-5.5 leading-relaxed">
                      <strong>For new nodes:</strong> Generates a fresh remote harvester keypair locally and broadcasts an on-chain link transaction from your staking account.
                    </p>
                    {verifyResult?.isLinked && actionType === 'link_and_register' && (
                      <p className="text-[10px] text-amber-300/90 mt-1.5 pl-5.5">
                        ⚠ Key Rotation Notice: Your account is currently linked on-chain. Proceeding will broadcast an on-chain link transaction that replaces your active harvester with a newly generated keypair.
                      </p>
                    )}
                  </div>

                  {/* Mode B: Register Harvester as Valid */}
                  <div
                    onClick={() => setActionType('register_only')}
                    className={`p-3 rounded-lg border cursor-pointer transition-all ${
                      actionType === 'register_only'
                        ? 'bg-blue-950/30 border-blue-500 text-white'
                        : 'bg-[#0F1115] border-[#262B34] text-slate-300 hover:border-slate-600'
                    }`}
                  >
                    <div className="flex items-center justify-between">
                      <span className="font-semibold text-xs flex items-center space-x-2">
                        <span className={`w-3.5 h-3.5 rounded-full border flex items-center justify-center ${
                          actionType === 'register_only' ? 'border-blue-400 bg-blue-500' : 'border-slate-600'
                        }`}>
                          {actionType === 'register_only' && <span className="w-1.5 h-1.5 rounded-full bg-white" />}
                        </span>
                        <span>Register Harvester as Valid</span>
                      </span>
                      {verifyResult?.isCommitteeHarvester ? (
                        <span className="text-[10px] bg-amber-950/60 text-amber-400 border border-amber-800/40 px-1.5 py-0.5 rounded">
                          Already Registered On-Chain
                        </span>
                      ) : verifyResult?.isLinked ? (
                        <span className="text-[10px] bg-emerald-950/60 text-emerald-400 border border-emerald-800/40 px-1.5 py-0.5 rounded">
                          Recommended
                        </span>
                      ) : null}
                    </div>
                    <p className="text-[11px] text-slate-400 mt-1 pl-5.5 leading-relaxed">
                      <strong>For existing setups:</strong> Sets an already-linked remote harvester key into this node's configuration without broadcasting a new link transaction.
                    </p>
                    {verifyResult?.isCommitteeHarvester && actionType === 'register_only' && (
                      <p className="text-[10px] text-amber-300/90 mt-1.5 pl-5.5">
                        ⚠ Already Registered Notice: This harvester key is already registered as an active validator in the Sirius POS+ consensus committee. Re-registering is not required unless you are re-applying local node configuration.
                      </p>
                    )}
                  </div>

                  {/* Mode C: Unlink Delegated Harvester */}
                  <div
                    onClick={() => setActionType('unlink')}
                    className={`p-3 rounded-lg border cursor-pointer transition-all ${
                      actionType === 'unlink'
                        ? 'bg-rose-950/30 border-rose-500 text-white'
                        : 'bg-[#0F1115] border-[#262B34] text-slate-300 hover:border-slate-600'
                    }`}
                  >
                    <div className="flex items-center justify-between">
                      <span className="font-semibold text-xs flex items-center space-x-2">
                        <span className={`w-3.5 h-3.5 rounded-full border flex items-center justify-center ${
                          actionType === 'unlink' ? 'border-rose-400 bg-rose-500' : 'border-slate-600'
                        }`}>
                          {actionType === 'unlink' && <span className="w-1.5 h-1.5 rounded-full bg-white" />}
                        </span>
                        <span>Unlink Delegated Harvester</span>
                      </span>
                    </div>
                    <p className="text-[11px] text-slate-400 mt-1 pl-5.5 leading-relaxed">
                      <strong>For revoking:</strong> Broadcasts an on-chain revocation transaction to disconnect this node from your staking account and stop harvesting.
                    </p>
                  </div>

                </div>
              </div>

              {/* 2. Staked Account Private Key Field */}
              <div className="space-y-1.5 pt-1">
                <label className="block font-medium text-slate-300">
                  Main Staked Account Private Key (64 Hex Characters)
                </label>
                <div className="relative">
                  <input
                    type={showAccountPrivKey ? 'text' : 'password'}
                    value={accountPrivKey}
                    onChange={(e) => setAccountPrivKey(e.target.value)}
                    placeholder="Enter 64-character private key of your main staking account..."
                    className="w-full font-mono px-3 py-2 pr-9 bg-[#0F1115] border border-[#262B34] text-slate-100 rounded focus:outline-none focus:border-blue-500"
                  />
                  <button
                    type="button"
                    onClick={() => setShowAccountPrivKey(!showAccountPrivKey)}
                    className="absolute right-2.5 top-2 text-slate-400 hover:text-white"
                  >
                    {showAccountPrivKey ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                  </button>
                </div>
              </div>

              {/* Explicit Local Signing Guarantee */}
              <div className="bg-[#0F1115] border border-[#262B34] rounded-lg p-3 space-y-1">
                <div className="flex items-center space-x-1.5 text-slate-200 font-medium">
                  <ShieldCheck className="w-3.5 h-3.5 text-emerald-400 flex-shrink-0" />
                  <span>Local Signing Guarantee</span>
                </div>
                <p className="text-[11px] text-slate-400 leading-relaxed">
                  Your private key is processed exclusively on this local machine by the local Sirius daemon to cryptographically sign the transaction. Only the resulting pre-signed transaction payload is broadcast to the network. <strong>Your private key never leaves this machine.</strong>
                </p>
              </div>

              {/* 3. Broadcast API Node with TLS */}
              <div className="space-y-1.5">
                <label className="block font-medium text-slate-300">
                  Broadcast API Node (TLS Encrypted)
                </label>
                <input
                  type="text"
                  value={apiNode}
                  onChange={(e) => setApiNode(e.target.value)}
                  placeholder="https://aldebaran.xpxsirius.io"
                  className="w-full font-mono px-3 py-2 bg-[#0F1115] border border-[#262B34] text-slate-100 rounded focus:outline-none focus:border-blue-500"
                />
                <span className="text-[11px] text-slate-500 block">
                  HTTPS Port 443 — encrypted transport channel for public signed transaction broadcast.
                </span>
              </div>

              {linkError && (
                <div className="p-3 bg-rose-950/40 border border-rose-800/60 rounded text-rose-300 flex items-center space-x-2">
                  <AlertCircle className="w-4 h-4 flex-shrink-0" />
                  <span>{linkError}</span>
                </div>
              )}

              {linkResult && (
                <div className="p-3 bg-emerald-950/40 border border-emerald-800/60 rounded text-emerald-300 space-y-1">
                  <div className="flex items-center space-x-2">
                    <CheckCircle2 className="w-4 h-4 flex-shrink-0 text-emerald-400" />
                    <span className="font-medium">{linkResult.message || 'Harvester link broadcast successfully!'}</span>
                  </div>
                  {linkResult.txHash && (
                    <p className="text-[11px] text-slate-400 font-mono pl-6">
                      Tx Hash: {linkResult.txHash}
                    </p>
                  )}
                  {linkResult.remotePublicKey && (
                    <p className="text-[11px] text-slate-400 font-mono pl-6">
                      Remote Harvester Key: {linkResult.remotePublicKey}
                    </p>
                  )}
                </div>
              )}

              </div>

              {/* Modal Sticky Footer (Always visible at bottom) */}
              <div className="p-4 border-t border-[#262B34] bg-[#0F1115] flex items-center justify-end space-x-2 flex-shrink-0">
                <button
                  type="button"
                  onClick={() => setShowLinkModal(false)}
                  className="px-3.5 py-1.5 font-medium text-slate-300 hover:text-white bg-[#0F1115] hover:bg-[#262B34] rounded transition-colors border border-[#262B34]"
                >
                  Close
                </button>
                <button
                  type="submit"
                  disabled={linking || !accountPrivKey.trim()}
                  className="px-4 py-1.5 bg-blue-600 hover:bg-blue-500 disabled:opacity-40 text-white rounded font-semibold transition-all flex items-center space-x-1.5 shadow-xs"
                >
                  <Zap className="w-3.5 h-3.5" />
                  <span>
                    {linking
                      ? 'Broadcasting...'
                      : actionType === 'unlink'
                      ? 'Unlink Delegated Harvester'
                      : actionType === 'register_only'
                      ? 'Register Harvester as Valid'
                      : 'Create Delegated Harvester'}
                  </span>
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

    </div>
  );
};
