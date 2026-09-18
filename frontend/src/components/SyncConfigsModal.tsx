import React, { useState, useEffect, useCallback } from 'react';
import { 
  X, 
  DownloadCloud, 
  ShieldCheck, 
  RefreshCw, 
  CheckCircle2, 
  FileCode, 
  Terminal, 
  Check,
  AlertTriangle,
  AlertCircle,
  RotateCcw,
  HardDrive,
  ChevronDown,
  ChevronUp
} from 'lucide-react';
import { ConfigDiffReport } from '../types';

interface SyncConfigsModalProps {
  isOpen: boolean;
  onClose: () => void;
  updateInfo: any;
  onRefresh: () => void;
}

const fallbackOfficialFiles = [
  {
    name: 'config-network.properties',
    purpose: 'Network fees, block generation times, and nemesis block hash definitions',
    category: 'Consensus Rules',
  },
  {
    name: 'peers-p2p.json',
    purpose: 'Active Mainnet P2P validator bootstrap peers and seed nodes',
    category: 'P2P Networking',
  },
  {
    name: 'peers-api.json',
    purpose: 'Public REST API node gateway seed directory',
    category: 'REST Gateway',
  },
  {
    name: 'replicators.json',
    purpose: 'DFMS distributed storage replicator bootstrap node network',
    category: 'Storage Replicators',
  },
  {
    name: 'supported-entities.json',
    purpose: 'Supported blockchain transaction entity types and plugin definitions',
    category: 'Transaction Types',
  },
];

export const SyncConfigsModal: React.FC<SyncConfigsModalProps> = ({
  isOpen,
  onClose,
  updateInfo,
  onRefresh,
}) => {
  const [isApplying, setIsApplying] = useState(false);
  const [progressMsg, setProgressMsg] = useState<string>('');
  const [isCompleted, setIsCompleted] = useState(false);
  const [isRolledBack, setIsRolledBack] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  // Diff inspection state
  const [isLoadingDiff, setIsLoadingDiff] = useState(false);
  const [diffReport, setDiffReport] = useState<ConfigDiffReport | null>(null);
  const [diffError, setDiffError] = useState<string | null>(null);
  const [expandedFile, setExpandedFile] = useState<string | null>(null);

  const fetchDiff = useCallback(async () => {
    setIsLoadingDiff(true);
    setDiffError(null);
    try {
      const res = await fetch('/api/maintenance/configs/diff');
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.error || `HTTP ${res.status}: Failed to inspect config differences`);
      }
      const data: ConfigDiffReport = await res.json();
      setDiffReport(data);
    } catch (err: any) {
      console.warn('Failed to fetch config diff:', err);
      setDiffError(err.message || 'Network error fetching configuration diff');
    } finally {
      setIsLoadingDiff(false);
    }
  }, []);

  // Fetch diff on initial modal open
  useEffect(() => {
    if (isOpen) {
      fetchDiff();
      setIsCompleted(false);
      setIsRolledBack(false);
      setErrorMsg(null);
      setProgressMsg('');
    }
  }, [isOpen, fetchDiff]);

  // Poll progress when applying
  useEffect(() => {
    if (!isApplying) return;

    const timer = setInterval(async () => {
      try {
        const res = await fetch('/api/system/updates/check');
        if (res.ok) {
          const data = await res.json();
          if (data.updateMessage) {
            setProgressMsg(data.updateMessage);
          }
          if (data.rollbackOccurred) {
            setIsRolledBack(true);
          }
          if (!data.isApplying) {
            setIsApplying(false);
            if (data.rollbackOccurred) {
              setIsRolledBack(true);
              setErrorMsg(data.updateMessage || 'Node healthcheck failed: Automatically rolled back to previous configs');
            } else if (data.updateMessage && data.updateMessage.toLowerCase().includes('failed')) {
              setErrorMsg(data.updateMessage);
            } else {
              setIsCompleted(true);
              onRefresh();
              fetchDiff();
            }
            clearInterval(timer);
          }
        }
      } catch (e) {
        console.warn('Poll config sync err:', e);
      }
    }, 600);

    return () => clearInterval(timer);
  }, [isApplying, onRefresh, fetchDiff]);

  if (!isOpen) return null;

  const handleStartSync = async () => {
    setIsApplying(true);
    setIsCompleted(false);
    setIsRolledBack(false);
    setErrorMsg(null);
    setProgressMsg('Initiating official configuration synchronization...');
    try {
      const res = await fetch('/api/maintenance/update/apply', { method: 'POST' });
      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error || 'Failed to start configuration synchronization');
      }
      if (data.message) {
        setProgressMsg(data.message);
      }
    } catch (e: any) {
      setIsApplying(false);
      setErrorMsg(e.message || 'Error communicating with supervisor');
    }
  };

  const getStepStatus = (stepIdx: number) => {
    if (isCompleted) return 'completed';
    if (isRolledBack) {
      if (stepIdx <= 1) return 'completed';
      if (stepIdx === 2) return 'failed';
    }
    if (errorMsg && !isRolledBack) {
      return 'failed';
    }
    if (!isApplying) return 'pending';

    const msg = progressMsg.toLowerCase();
    if (msg.includes('downloading') || msg.includes('fetching') || msg.includes('verifying') || msg.includes('in memory')) {
      return stepIdx === 0 ? 'active' : 'pending';
    }
    if (msg.includes('backup') || msg.includes('swap') || msg.includes('atomically')) {
      if (stepIdx === 0) return 'completed';
      if (stepIdx === 1) return 'active';
      return 'pending';
    }
    if (msg.includes('starting node') || msg.includes('healthcheck') || msg.includes('restarting') || msg.includes('stopping node')) {
      if (stepIdx <= 1) return 'completed';
      if (stepIdx === 2) return 'active';
      return 'pending';
    }
    return stepIdx === 0 ? 'active' : 'pending';
  };

  const displayFiles = diffReport && diffReport.files && diffReport.files.length > 0 
    ? diffReport.files 
    : fallbackOfficialFiles.map(f => ({
        ...f,
        localHash: '',
        remoteHash: '',
        status: 'pending',
        localFound: true,
        remoteFound: true,
      }));

  const diffCount = diffReport?.differentCount ?? 0;
  const missingCount = diffReport?.missingCount ?? 0;
  const totalChanged = diffCount + missingCount;
  const hasDifferences = diffReport?.hasDifferences ?? true;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/75 backdrop-blur-xs animate-fadeIn select-none">
      <div 
        className="bg-[#181B20] border border-[#262B34] rounded-xl shadow-2xl w-full max-w-2xl overflow-hidden flex flex-col max-h-[90vh]"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="px-6 py-4 border-b border-[#262B34] bg-[#0F1115] flex items-center justify-between">
          <div className="flex items-center space-x-3">
            <div className="w-10 h-10 rounded-lg bg-blue-500/10 border border-blue-500/30 flex items-center justify-center">
              <DownloadCloud className="w-5 h-5 text-blue-400" />
            </div>
            <div>
              <div className="flex items-center space-x-2">
                <h3 className="text-sm font-semibold text-white">Sync Official Network Configurations</h3>
                <span className="text-[10px] px-2 py-0.5 rounded-full bg-blue-500/20 text-blue-300 font-mono border border-blue-500/30">
                  mainnet
                </span>
              </div>
              <p className="text-xs text-slate-400 mt-0.5">
                Inspect differences against upstream GitHub master, create pre-backup, and auto-rollback on failure
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            disabled={isApplying}
            className="text-slate-400 hover:text-white p-1.5 rounded-lg hover:bg-[#262B34] transition-colors disabled:opacity-40"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Body */}
        <div className="p-6 overflow-y-auto space-y-5 flex-1 text-xs">
          
          {/* Privacy & Auto-Rollback Guarantees Card */}
          <div className="bg-emerald-950/30 border border-emerald-500/40 rounded-lg p-4 flex items-start space-x-3">
            <ShieldCheck className="w-5 h-5 text-emerald-400 flex-shrink-0 mt-0.5" />
            <div className="space-y-1">
              <span className="font-semibold text-emerald-300 block">
                Key Privacy & Zero-Downtime Rollback Guarantee
              </span>
              <p className="text-emerald-200/80 leading-relaxed text-[11px]">
                Your harvesting private key (<code className="font-mono text-white">config-harvesting.properties</code>) and node transport identity (<code className="font-mono text-white">config-user.properties</code>) are strictly protected and <strong>never touched</strong>.
                Before any file is updated, an automated timestamped backup is preserved. If the node fails to initialize post-sync, all files are <strong>immediately rolled back</strong>.
              </p>
            </div>
          </div>

          {/* Upstream Difference Detection Banner */}
          {isLoadingDiff ? (
            <div className="bg-[#13171F] border border-[#262B34] rounded-lg p-4 flex items-center justify-between">
              <div className="flex items-center space-x-3">
                <RefreshCw className="w-4 h-4 text-blue-400 animate-spin" />
                <span className="text-slate-300 font-medium text-xs">
                  Inspecting differences against upstream GitHub repository...
                </span>
              </div>
              <span className="text-[10px] font-mono text-slate-500">Checking SHA-256</span>
            </div>
          ) : diffError ? (
            <div className="bg-amber-950/30 border border-amber-500/40 rounded-lg p-4 flex items-center justify-between">
              <div className="flex items-center space-x-3">
                <AlertTriangle className="w-4 h-4 text-amber-400 flex-shrink-0" />
                <span className="text-amber-200 text-xs">
                  Could not check upstream differences: {diffError}
                </span>
              </div>
              <button
                onClick={fetchDiff}
                className="px-2.5 py-1 text-[11px] rounded bg-amber-900/40 hover:bg-amber-800/50 text-amber-200 border border-amber-700/50 flex items-center space-x-1.5 transition-colors"
              >
                <RefreshCw className="w-3 h-3" />
                <span>Retry</span>
              </button>
            </div>
          ) : diffReport ? (
            hasDifferences ? (
              <div className="bg-amber-950/30 border border-amber-500/40 rounded-lg p-4 flex items-start justify-between">
                <div className="flex items-start space-x-3">
                  <AlertTriangle className="w-5 h-5 text-amber-400 flex-shrink-0 mt-0.5" />
                  <div>
                    <div className="flex items-center space-x-2">
                      <span className="font-semibold text-amber-300">
                        Configuration Differences Detected
                      </span>
                      <span className="text-[10px] px-2 py-0.5 rounded-full bg-amber-500/20 text-amber-300 font-mono border border-amber-500/30">
                        {totalChanged} file{totalChanged === 1 ? '' : 's'} need update
                      </span>
                    </div>
                    <p className="text-amber-200/80 text-[11px] mt-1 leading-relaxed">
                      Your local configurations differ from the latest official GitHub master release. Review the detected changes below before synchronizing.
                    </p>
                  </div>
                </div>
                <button
                  onClick={fetchDiff}
                  disabled={isApplying}
                  title="Re-check differences"
                  className="p-1.5 text-slate-400 hover:text-white rounded hover:bg-[#262B34] transition-colors flex-shrink-0"
                >
                  <RefreshCw className="w-3.5 h-3.5" />
                </button>
              </div>
            ) : (
              <div className="bg-emerald-950/25 border border-emerald-500/30 rounded-lg p-4 flex items-start justify-between">
                <div className="flex items-start space-x-3">
                  <CheckCircle2 className="w-5 h-5 text-emerald-400 flex-shrink-0 mt-0.5" />
                  <div>
                    <span className="font-semibold text-emerald-300 block">
                      All Configurations Up-to-Date
                    </span>
                    <p className="text-emerald-200/80 text-[11px] mt-0.5 leading-relaxed">
                      All 5 configuration files on your machine match upstream GitHub master byte-for-byte (verified via SHA-256).
                    </p>
                  </div>
                </div>
                <button
                  onClick={fetchDiff}
                  disabled={isApplying}
                  title="Re-check differences"
                  className="p-1.5 text-slate-400 hover:text-white rounded hover:bg-[#262B34] transition-colors flex-shrink-0"
                >
                  <RefreshCw className="w-3.5 h-3.5" />
                </button>
              </div>
            )
          ) : null}

          {/* Files List Card */}
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <span className="text-slate-400 font-semibold uppercase tracking-wider text-[11px] block">
                Official Network Configurations (5 Assets)
              </span>
              {diffReport?.checkedAt && (
                <span className="text-[10px] text-slate-500 font-mono">
                  Checked: {new Date(diffReport.checkedAt).toLocaleTimeString()}
                </span>
              )}
            </div>

            <div className="border border-[#262B34] rounded-lg overflow-hidden divide-y divide-[#262B34]">
              {displayFiles.map((f: any, idx) => {
                const fileName = f.name || f.fileName;
                const isDiff = f.status === 'different';
                const isMissing = f.status === 'missing';
                const isIdentical = f.status === 'identical';
                const isExpanded = expandedFile === fileName;

                return (
                  <div key={idx} className="flex flex-col">
                    <div 
                      className={`p-3 transition-colors flex items-center justify-between ${
                        isDiff 
                          ? 'bg-amber-950/10 hover:bg-amber-950/20 cursor-pointer' 
                          : isMissing 
                          ? 'bg-rose-950/10 hover:bg-rose-950/20' 
                          : 'bg-[#13171F] hover:bg-[#181B20]'
                      }`}
                      onClick={() => {
                        if (isDiff && f.diffSnippet) {
                          setExpandedFile(isExpanded ? null : fileName);
                        }
                      }}
                    >
                      <div className="flex items-center space-x-3 min-w-0">
                        <FileCode className={`w-4 h-4 flex-shrink-0 ${
                          isDiff ? 'text-amber-400' : isMissing ? 'text-rose-400' : 'text-blue-400'
                        }`} />
                        <div className="min-w-0">
                          <div className="flex items-center space-x-2">
                            <span className="font-mono font-medium text-white text-xs">{fileName}</span>
                            <span className="text-[10px] font-mono px-1.5 py-0.2 rounded bg-[#0F1115] text-slate-400 border border-[#262B34]">
                              {f.category}
                            </span>
                          </div>
                          <p className="text-[11px] text-slate-400 mt-0.5 truncate">{f.purpose}</p>

                          {/* Hash comparison info */}
                          {f.localHash && f.remoteHash && (
                            <div className="mt-1 flex items-center space-x-2 text-[10px] font-mono text-slate-500">
                              <span>Local: <code className="text-slate-400">{f.localHash.slice(0, 8)}…</code></span>
                              <span>•</span>
                              <span>Upstream: <code className="text-slate-400">{f.remoteHash.slice(0, 8)}…</code></span>
                            </div>
                          )}
                          {isMissing && f.remoteHash && (
                            <div className="mt-1 text-[10px] font-mono text-rose-400/90">
                              <span>Not present on local disk • Upstream: <code className="text-slate-400">{f.remoteHash.slice(0, 8)}…</code></span>
                            </div>
                          )}
                        </div>
                      </div>

                      {/* Status Badge & Expand toggle */}
                      <div className="flex items-center space-x-2 flex-shrink-0 ml-3">
                        {isLoadingDiff ? (
                          <span className="text-[10px] px-2 py-0.5 rounded bg-blue-500/10 text-blue-300 font-mono border border-blue-500/20 flex items-center space-x-1">
                            <RefreshCw className="w-2.5 h-2.5 animate-spin" />
                            <span>Checking</span>
                          </span>
                        ) : isIdentical ? (
                          <span className="text-[10px] px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-300 font-mono border border-emerald-500/20 flex items-center space-x-1">
                            <Check className="w-2.5 h-2.5" />
                            <span>In Sync</span>
                          </span>
                        ) : isDiff ? (
                          <div className="flex items-center space-x-1.5">
                            <span className="text-[10px] px-2 py-0.5 rounded bg-amber-500/15 text-amber-300 font-mono border border-amber-500/30 flex items-center space-x-1">
                              <AlertTriangle className="w-2.5 h-2.5" />
                              <span>Different</span>
                            </span>
                            {f.diffSnippet && (
                              <button 
                                onClick={(e) => {
                                  e.stopPropagation();
                                  setExpandedFile(isExpanded ? null : fileName);
                                }}
                                className="p-1 hover:bg-[#262B34] text-slate-400 hover:text-slate-200 rounded"
                              >
                                {isExpanded ? <ChevronUp className="w-3.5 h-3.5" /> : <ChevronDown className="w-3.5 h-3.5" />}
                              </button>
                            )}
                          </div>
                        ) : isMissing ? (
                          <span className="text-[10px] px-2 py-0.5 rounded bg-rose-500/15 text-rose-300 font-mono border border-rose-500/30 flex items-center space-x-1">
                            <AlertCircle className="w-2.5 h-2.5" />
                            <span>Missing</span>
                          </span>
                        ) : (
                          <span className="text-[10px] px-2 py-0.5 rounded bg-[#0F1115] text-slate-400 font-mono border border-[#262B34]">
                            Pending Check
                          </span>
                        )}
                      </div>
                    </div>

                    {/* Expandable diff snippet */}
                    {isExpanded && f.diffSnippet && (
                      <div className="px-3 pb-3 pt-1.5 bg-[#0F1115] border-t border-[#262B34]/60">
                        <div className="text-[10px] text-amber-300 font-mono mb-1 flex items-center justify-between">
                          <span>Detected Changes (+ Upstream):</span>
                          <span className="text-[9px] text-slate-500">Lines in upstream repository</span>
                        </div>
                        <pre className="text-[10px] font-mono text-slate-300 bg-[#0A0C10] p-2 rounded border border-[#262B34] overflow-x-auto whitespace-pre-wrap leading-relaxed">
                          {f.diffSnippet}
                        </pre>
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          </div>

          {/* Rollback Safety Notice Banner (When Auto-Rollback triggered) */}
          {isRolledBack && (
            <div className="bg-amber-950/40 border border-amber-500/50 rounded-lg p-4 space-y-2 text-amber-200">
              <div className="flex items-center space-x-2 text-xs font-bold text-amber-300 uppercase tracking-wide">
                <RotateCcw className="w-4 h-4 text-amber-400" />
                <span>Automated Rollback Executed</span>
              </div>
              <p className="text-xs text-amber-200/90 leading-relaxed">
                The node engine failed post-update operational verification with the new configuration files. 
                The supervisor immediately aborted the update, restored your previous configurations from backup, and revived the node process cleanly. Your node remains operational with original settings.
              </p>
            </div>
          )}

          {/* Progress / Step Tracker (when active or complete) */}
          {(isApplying || isCompleted || errorMsg) && (
            <div className="space-y-3 pt-2">
              <span className="text-slate-400 font-semibold uppercase tracking-wider text-[11px] block">
                Synchronization & Verification Pipeline
              </span>

              <div className="space-y-2.5">
                {/* Step 1: Download & In-Memory Validation */}
                <div className={`p-3 rounded-lg border flex items-start space-x-3 ${
                  getStepStatus(0) === 'active' 
                    ? 'bg-blue-950/30 border-blue-500/50 text-blue-200' 
                    : getStepStatus(0) === 'completed'
                    ? 'bg-emerald-950/20 border-emerald-800/40 text-emerald-200'
                    : 'bg-[#13171F] border-[#262B34] text-slate-500'
                }`}>
                  <div className="mt-0.5">
                    {getStepStatus(0) === 'active' && <RefreshCw className="w-4 h-4 text-blue-400 animate-spin" />}
                    {getStepStatus(0) === 'completed' && <CheckCircle2 className="w-4 h-4 text-emerald-400" />}
                    {getStepStatus(0) === 'failed' && <AlertCircle className="w-4 h-4 text-rose-400" />}
                    {getStepStatus(0) === 'pending' && <div className="w-4 h-4 rounded-full border border-slate-600" />}
                  </div>
                  <div className="flex-1">
                    <div className="font-medium flex items-center justify-between text-xs">
                      <span>1. In-Memory Download & Syntax Validation</span>
                      <span className="font-mono text-[10px] uppercase opacity-75">{getStepStatus(0)}</span>
                    </div>
                    <p className="text-[11px] opacity-80 mt-0.5">
                      Streams raw config files from official repository and validates JSON/properties structure before disk touch.
                    </p>
                  </div>
                </div>

                {/* Step 2: Pre-Backup & Atomic Swap */}
                <div className={`p-3 rounded-lg border flex items-start space-x-3 ${
                  getStepStatus(1) === 'active' 
                    ? 'bg-blue-950/30 border-blue-500/50 text-blue-200' 
                    : getStepStatus(1) === 'completed'
                    ? 'bg-emerald-950/20 border-emerald-800/40 text-emerald-200'
                    : 'bg-[#13171F] border-[#262B34] text-slate-500'
                }`}>
                  <div className="mt-0.5">
                    {getStepStatus(1) === 'active' && <RefreshCw className="w-4 h-4 text-blue-400 animate-spin" />}
                    {getStepStatus(1) === 'completed' && <CheckCircle2 className="w-4 h-4 text-emerald-400" />}
                    {getStepStatus(1) === 'failed' && <AlertCircle className="w-4 h-4 text-rose-400" />}
                    {getStepStatus(1) === 'pending' && <div className="w-4 h-4 rounded-full border border-slate-600" />}
                  </div>
                  <div className="flex-1">
                    <div className="font-medium flex items-center justify-between text-xs">
                      <span>2. Timestamped Pre-Backup & Atomic Swap</span>
                      <span className="font-mono text-[10px] uppercase opacity-75">{getStepStatus(1)}</span>
                    </div>
                    <p className="text-[11px] opacity-80 mt-0.5">
                      Backs up current configs to <code className="text-slate-300">.backup-configs-*</code> and performs atomic rename swap.
                    </p>
                  </div>
                </div>

                {/* Step 3: Reload & Operational Healthcheck */}
                <div className={`p-3 rounded-lg border flex items-start space-x-3 ${
                  getStepStatus(2) === 'active' 
                    ? 'bg-blue-950/30 border-blue-500/50 text-blue-200' 
                    : getStepStatus(2) === 'completed'
                    ? 'bg-emerald-950/20 border-emerald-800/40 text-emerald-200'
                    : getStepStatus(2) === 'failed'
                    ? 'bg-amber-950/30 border-amber-800/50 text-amber-200'
                    : 'bg-[#13171F] border-[#262B34] text-slate-500'
                }`}>
                  <div className="mt-0.5">
                    {getStepStatus(2) === 'active' && <RefreshCw className="w-4 h-4 text-blue-400 animate-spin" />}
                    {getStepStatus(2) === 'completed' && <CheckCircle2 className="w-4 h-4 text-emerald-400" />}
                    {getStepStatus(2) === 'failed' && <AlertTriangle className="w-4 h-4 text-amber-400" />}
                    {getStepStatus(2) === 'pending' && <div className="w-4 h-4 rounded-full border border-slate-600" />}
                  </div>
                  <div className="flex-1">
                    <div className="font-medium flex items-center justify-between text-xs">
                      <span>3. Node Reload & Operational Healthcheck (Auto-Rollback)</span>
                      <span className="font-mono text-[10px] uppercase opacity-75">{getStepStatus(2)}</span>
                    </div>
                    <p className="text-[11px] opacity-80 mt-0.5">
                      Restarts node and performs operational health check. If node crashes or exits, restores backup automatically.
                    </p>
                  </div>
                </div>
              </div>

              {/* Progress message log */}
              {(progressMsg || errorMsg) && (
                <div className={`p-3.5 rounded-lg border font-mono text-xs ${
                  isRolledBack
                    ? 'bg-amber-950/40 border-amber-700/60 text-amber-200'
                    : errorMsg 
                    ? 'bg-rose-950/40 border-rose-800/60 text-rose-300' 
                    : isCompleted 
                    ? 'bg-emerald-950/40 border-emerald-800/60 text-emerald-300' 
                    : 'bg-[#0F1115] border-[#262B34] text-slate-300'
                }`}>
                  <div className="flex items-center space-x-2 font-semibold mb-1">
                    <Terminal className="w-4 h-4 flex-shrink-0" />
                    <span>
                      {isRolledBack 
                        ? 'Automated Rollback Executed' 
                        : errorMsg 
                        ? 'Sync Failed' 
                        : isCompleted 
                        ? 'Sync Completed' 
                        : 'Supervisor Output'}
                    </span>
                  </div>
                  <p className="leading-relaxed whitespace-pre-wrap">{errorMsg || progressMsg}</p>
                </div>
              )}
            </div>
          )}

          {/* Safety Summary Note when idle */}
          {!isApplying && !isCompleted && !isRolledBack && (
            <div className="bg-[#13171F] border border-[#262B34] rounded-lg p-3.5 flex items-center space-x-3 text-slate-400">
              <HardDrive className="w-4 h-4 text-blue-400 flex-shrink-0" />
              <p className="text-[11px] leading-relaxed">
                Before applying updates, your existing configs are archived in a timestamped backup. If the node fails to initialize, the system restores your previous configurations automatically.
              </p>
            </div>
          )}

        </div>

        {/* Footer */}
        <div className="px-6 py-4 border-t border-[#262B34] bg-[#0F1115] flex items-center justify-between">
          <div className="text-xs text-slate-500">
            {isLoadingDiff 
              ? 'Checking upstream differences...' 
              : isApplying 
              ? 'Sync in progress...' 
              : isRolledBack
              ? 'Restored previous configuration'
              : isCompleted 
              ? 'All configs synchronized' 
              : hasDifferences 
              ? `${totalChanged} configuration file${totalChanged === 1 ? '' : 's'} differ` 
              : 'All 5 configuration files in sync'}
          </div>

          <div className="flex items-center space-x-3">
            <button
              onClick={onClose}
              disabled={isApplying}
              className="px-4 py-2 rounded-lg bg-[#181B20] hover:bg-[#262B34] text-slate-300 text-xs font-medium border border-[#262B34] transition-colors disabled:opacity-40"
            >
              {isCompleted ? 'Close' : 'Cancel'}
            </button>

            {!isCompleted && (
              <>
                {!hasDifferences && !isLoadingDiff && (
                  <button
                    onClick={handleStartSync}
                    disabled={isApplying}
                    className="px-3.5 py-2 rounded-lg bg-[#181B20] hover:bg-[#262B34] text-slate-300 text-xs font-medium border border-[#262B34] transition-colors flex items-center space-x-1.5 disabled:opacity-40"
                  >
                    <RefreshCw className="w-3.5 h-3.5 text-slate-400" />
                    <span>Force Re-sync</span>
                  </button>
                )}

                <button
                  onClick={handleStartSync}
                  disabled={isApplying || isLoadingDiff}
                  className={`px-4 py-2 rounded-lg text-white text-xs font-semibold shadow-md transition-all flex items-center space-x-2 disabled:opacity-40 ${
                    hasDifferences 
                      ? 'bg-blue-600 hover:bg-blue-500 shadow-blue-600/20' 
                      : 'bg-emerald-600 hover:bg-emerald-500 shadow-emerald-600/20'
                  }`}
                >
                  {isApplying ? (
                    <>
                      <RefreshCw className="w-3.5 h-3.5 animate-spin" />
                      <span>Syncing Configurations...</span>
                    </>
                  ) : hasDifferences ? (
                    <>
                      <DownloadCloud className="w-3.5 h-3.5" />
                      <span>Synchronize {totalChanged} Config{totalChanged === 1 ? '' : 's'} (with Auto-Rollback)</span>
                    </>
                  ) : (
                    <>
                      <Check className="w-3.5 h-3.5" />
                      <span>All In Sync</span>
                    </>
                  )}
                </button>
              </>
            )}

            {isCompleted && (
              <button
                onClick={onClose}
                className="px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold shadow-md shadow-emerald-600/20 transition-all flex items-center space-x-2"
              >
                <Check className="w-3.5 h-3.5" />
                <span>Done</span>
              </button>
            )}
          </div>
        </div>

      </div>
    </div>
  );
};
