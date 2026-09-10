import React, { useState, useEffect } from 'react';
import { 
  X, 
  DownloadCloud, 
  ShieldCheck, 
  RefreshCw, 
  CheckCircle2, 
  FileCode, 
  Terminal, 
  Check
} from 'lucide-react';

interface SyncConfigsModalProps {
  isOpen: boolean;
  onClose: () => void;
  updateInfo: any;
  onRefresh: () => void;
}

export const SyncConfigsModal: React.FC<SyncConfigsModalProps> = ({
  isOpen,
  onClose,
  updateInfo,
  onRefresh,
}) => {
  const [isApplying, setIsApplying] = useState(false);
  const [progressMsg, setProgressMsg] = useState<string>('');
  const [isCompleted, setIsCompleted] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const officialFiles = [
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
          if (!data.isApplying && isApplying) {
            setIsApplying(false);
            if (data.updateMessage && data.updateMessage.toLowerCase().includes('failed')) {
              setErrorMsg(data.updateMessage);
            } else {
              setIsCompleted(true);
              onRefresh();
            }
            clearInterval(timer);
          }
        }
      } catch (e) {
        console.warn('Poll config sync err:', e);
      }
    }, 600);

    return () => clearInterval(timer);
  }, [isApplying, onRefresh]);

  if (!isOpen) return null;

  const handleStartSync = async () => {
    setIsApplying(true);
    setIsCompleted(false);
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
    if (errorMsg) return 'failed';
    if (!isApplying) return 'pending';

    const msg = progressMsg.toLowerCase();
    if (msg.includes('fetching') || msg.includes('downloading') || msg.includes('updating network')) {
      if (stepIdx === 0) return 'active';
      return 'pending';
    }
    if (msg.includes('installed') || msg.includes('validated') || msg.includes('sha-256')) {
      if (stepIdx === 0) return 'completed';
      if (stepIdx === 1) return 'active';
      return 'pending';
    }
    if (msg.includes('restarting') || msg.includes('restart')) {
      if (stepIdx <= 1) return 'completed';
      if (stepIdx === 2) return 'active';
      return 'pending';
    }
    return stepIdx === 0 ? 'active' : 'pending';
  };

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
                Pulls verified seed nodes, peer lists, and protocol network properties from official GitHub
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
          
          {/* Safety Invariant Notice */}
          <div className="bg-emerald-950/30 border border-emerald-500/40 rounded-lg p-4 flex items-start space-x-3">
            <ShieldCheck className="w-5 h-5 text-emerald-400 flex-shrink-0 mt-0.5" />
            <div className="space-y-1">
              <span className="font-semibold text-emerald-300 block">
                Zero Key Exposure & Preservation Guarantee
              </span>
              <p className="text-emerald-200/80 leading-relaxed text-[11px]">
                Your local harvesting private key (<code className="font-mono text-white">config-harvesting.properties</code>), 
                transport identity (<code className="font-mono text-white">config-user.properties</code>), and custom storage data path 
                are strictly isolated. They are <strong>never overwritten</strong> during network configuration sync.
              </p>
            </div>
          </div>

          {/* Files List Card */}
          <div className="space-y-2">
            <span className="text-slate-400 font-semibold uppercase tracking-wider text-[11px] block">
              Configurations To Synchronize (5 Official Assets)
            </span>
            <div className="border border-[#262B34] rounded-lg overflow-hidden divide-y divide-[#262B34]">
              {officialFiles.map((f, idx) => (
                <div key={idx} className="p-3 bg-[#13171F] flex items-center justify-between hover:bg-[#181B20] transition-colors">
                  <div className="flex items-center space-x-2.5">
                    <FileCode className="w-4 h-4 text-blue-400 flex-shrink-0" />
                    <div>
                      <span className="font-mono font-medium text-white">{f.name}</span>
                      <p className="text-[11px] text-slate-400 mt-0.5">{f.purpose}</p>
                    </div>
                  </div>
                  <span className="text-[10px] font-mono px-2 py-0.5 rounded bg-[#0F1115] text-slate-400 border border-[#262B34] flex-shrink-0">
                    {f.category}
                  </span>
                </div>
              ))}
            </div>
          </div>

          {/* Progress / Step Tracker (when active or complete) */}
          {(isApplying || isCompleted || errorMsg) && (
            <div className="space-y-3 pt-2">
              <span className="text-slate-400 font-semibold uppercase tracking-wider text-[11px] block">
                Synchronization Pipeline Progress
              </span>

              <div className="space-y-2.5">
                {/* Step 1 */}
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
                    {getStepStatus(0) === 'pending' && <div className="w-4 h-4 rounded-full border border-slate-600" />}
                  </div>
                  <div className="flex-1">
                    <div className="font-medium flex items-center justify-between text-xs">
                      <span>1. Download Official Configurations from GitHub</span>
                      <span className="font-mono text-[10px] uppercase opacity-75">{getStepStatus(0)}</span>
                    </div>
                    <p className="text-[11px] opacity-80 mt-0.5">
                      Streams raw config files from <code className="text-slate-300">proximax-storage/cpp-xpx-chain/master/resources</code>.
                    </p>
                  </div>
                </div>

                {/* Step 2 */}
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
                    {getStepStatus(1) === 'pending' && <div className="w-4 h-4 rounded-full border border-slate-600" />}
                  </div>
                  <div className="flex-1">
                    <div className="font-medium flex items-center justify-between text-xs">
                      <span>2. Structural Format Validation & SHA-256 Checksums</span>
                      <span className="font-mono text-[10px] uppercase opacity-75">{getStepStatus(1)}</span>
                    </div>
                    <p className="text-[11px] opacity-80 mt-0.5">
                      Ensures valid JSON & property syntax and calculates SHA-256 hashes to prevent corrupted downloads.
                    </p>
                  </div>
                </div>

                {/* Step 3 */}
                <div className={`p-3 rounded-lg border flex items-start space-x-3 ${
                  getStepStatus(2) === 'active' 
                    ? 'bg-blue-950/30 border-blue-500/50 text-blue-200' 
                    : getStepStatus(2) === 'completed'
                    ? 'bg-emerald-950/20 border-emerald-800/40 text-emerald-200'
                    : 'bg-[#13171F] border-[#262B34] text-slate-500'
                }`}>
                  <div className="mt-0.5">
                    {getStepStatus(2) === 'active' && <RefreshCw className="w-4 h-4 text-blue-400 animate-spin" />}
                    {getStepStatus(2) === 'completed' && <CheckCircle2 className="w-4 h-4 text-emerald-400" />}
                    {getStepStatus(2) === 'pending' && <div className="w-4 h-4 rounded-full border border-slate-600" />}
                  </div>
                  <div className="flex-1">
                    <div className="font-medium flex items-center justify-between text-xs">
                      <span>3. Atomic Replacement & Seamless Node Reload</span>
                      <span className="font-mono text-[10px] uppercase opacity-75">{getStepStatus(2)}</span>
                    </div>
                    <p className="text-[11px] opacity-80 mt-0.5">
                      Writes verified files with 0600 permissions, clears locks, and cleanly restarts node engine.
                    </p>
                  </div>
                </div>
              </div>

              {/* Progress message log */}
              {(progressMsg || errorMsg) && (
                <div className={`p-3.5 rounded-lg border font-mono text-xs ${
                  errorMsg 
                    ? 'bg-rose-950/40 border-rose-800/60 text-rose-300' 
                    : isCompleted 
                    ? 'bg-emerald-950/40 border-emerald-800/60 text-emerald-300' 
                    : 'bg-[#0F1115] border-[#262B34] text-slate-300'
                }`}>
                  <div className="flex items-center space-x-2 font-semibold mb-1">
                    <Terminal className="w-4 h-4 flex-shrink-0" />
                    <span>{errorMsg ? 'Sync Failed' : isCompleted ? 'Sync Completed' : 'Updater Output'}</span>
                  </div>
                  <p className="leading-relaxed whitespace-pre-wrap">{errorMsg || progressMsg}</p>
                </div>
              )}
            </div>
          )}

        </div>

        {/* Footer */}
        <div className="px-6 py-4 border-t border-[#262B34] bg-[#0F1115] flex items-center justify-between">
          <div className="text-xs text-slate-500">
            {isApplying ? 'Sync in progress...' : isCompleted ? 'All configs synchronized' : 'Ready to synchronize'}
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
              <button
                onClick={handleStartSync}
                disabled={isApplying}
                className="px-4 py-2 rounded-lg bg-blue-600 hover:bg-blue-500 text-white text-xs font-semibold shadow-md shadow-blue-600/20 transition-all flex items-center space-x-2 disabled:opacity-40"
              >
                {isApplying ? (
                  <>
                    <RefreshCw className="w-3.5 h-3.5 animate-spin" />
                    <span>Syncing Configurations...</span>
                  </>
                ) : (
                  <>
                    <DownloadCloud className="w-3.5 h-3.5" />
                    <span>Synchronize 5 Official Configs</span>
                  </>
                )}
              </button>
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
