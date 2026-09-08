import React from 'react';
import { 
  CheckCircle2, 
  AlertTriangle, 
  Loader2, 
  ShieldCheck, 
  DownloadCloud, 
  Archive, 
  XCircle, 
  Hash, 
  Lock 
} from 'lucide-react';
import { SnapshotManagerStatus } from '../types';

interface SnapshotModalProps {
  status: SnapshotManagerStatus | null;
  onCancel: () => void;
  onClose: () => void;
}

export const SnapshotModal: React.FC<SnapshotModalProps> = ({ status, onCancel, onClose }) => {
  if (!status || status.stage === 'idle') return null;

  const isCompleted = status.stage === 'completed';
  const isError = status.stage === 'error';
  const isCancelled = status.stage === 'cancelled';
  const isRunning = !isCompleted && !isError && !isCancelled;
  const isRemote = status.operation === 'restore_remote';
  const isCreate = status.operation === 'create';

  const steps = isRemote
    ? [
        { key: 'fetching_manifest', label: '1. Manifest', icon: DownloadCloud },
        { key: 'verifying_signature', label: '2. Ed25519 Sig', icon: Lock },
        { key: 'downloading', label: '3. Stream & Extract', icon: DownloadCloud },
        { key: 'verifying_checksum', label: '4. SHA-256 Check', icon: Hash },
        { key: 'completed', label: '5. Ready', icon: CheckCircle2 },
      ]
    : [
        { key: 'archiving', label: isCreate ? '1. Scanning Data' : '1. Preparing', icon: Archive },
        { key: 'compressing', label: isCreate ? '2. Compressing .tar.zst' : '2. Decompressing', icon: Hash },
        { key: 'completed', label: '3. Completed', icon: CheckCircle2 },
      ];

  const getStepState = (stepKey: string) => {
    if (isCompleted) return 'completed';
    if (isError) return 'error';

    const stage = status.stage;
    if (stepKey === stage) return 'active';

    if (isRemote) {
      const order = ['fetching_manifest', 'verifying_signature', 'downloading', 'verifying_checksum', 'completed'];
      const currentIndex = order.indexOf(stage);
      const stepIndex = order.indexOf(stepKey);
      if (currentIndex > stepIndex) return 'completed';
      return 'pending';
    } else {
      if (stage === 'archiving' || stage === 'extracting') {
        if (stepKey === 'completed') return 'pending';
        return 'active';
      }
      return 'pending';
    }
  };

  const formatBytes = (bytes: number): string => {
    if (!bytes || bytes <= 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
  };

  const formatTime = (secs: number): string => {
    if (!secs || secs <= 0) return '--:--';
    const m = Math.floor(secs / 60);
    const s = Math.floor(secs % 60);
    return `${m}m ${s < 10 ? '0' : ''}${s}s`;
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm animate-in fade-in duration-200">
      <div className="w-full max-w-xl bg-[#13171F] border border-[#262B34] rounded-2xl shadow-2xl overflow-hidden flex flex-col">
        
        {/* Header */}
        <div className="px-6 py-4 border-b border-[#262B34] flex items-center justify-between bg-[#181B20]/60">
          <div className="flex items-center space-x-3">
            <div className={`p-2 rounded-lg ${
              isCompleted ? 'bg-emerald-500/10 text-emerald-400' :
              isError ? 'bg-rose-500/10 text-rose-400' :
              'bg-blue-500/10 text-blue-400'
            }`}>
              {isCompleted ? <ShieldCheck className="w-5 h-5" /> :
               isError ? <AlertTriangle className="w-5 h-5" /> :
               <DownloadCloud className="w-5 h-5 animate-pulse" />}
            </div>
            <div>
              <h3 className="text-sm font-semibold text-white">
                {isCreate ? 'Creating Blockchain Snapshot' :
                 isRemote ? 'Restoring Verified Remote Snapshot' :
                 'Restoring Local Snapshot'}
              </h3>
              <p className="text-xs text-slate-400 font-mono">
                {status.manifest?.archiveName || status.targetFile || 'Blockchain state operation'}
              </p>
            </div>
          </div>
          
          <div className="flex items-center space-x-2">
            <span className={`px-2.5 py-0.5 rounded text-[11px] font-mono font-medium ${
              isCompleted ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30' :
              isError ? 'bg-rose-500/20 text-rose-300 border border-rose-500/30' :
              isCancelled ? 'bg-slate-700 text-slate-300' :
              'bg-blue-500/20 text-blue-300 border border-blue-500/30 animate-pulse'
            }`}>
              {status.stage.toUpperCase()}
            </span>
          </div>
        </div>

        {/* Multi-Step Pipeline Indicator */}
        <div className="px-6 pt-5 pb-3 border-b border-[#262B34]/60 bg-[#0F1115]/50">
          <div className="flex items-center justify-between relative">
            {steps.map((step, idx) => {
              const state = getStepState(step.key);
              const StepIcon = step.icon;

              return (
                <div key={step.key} className="flex flex-col items-center space-y-1.5 flex-1 z-10">
                  <div className={`w-8 h-8 rounded-full flex items-center justify-center transition-all ${
                    state === 'completed' ? 'bg-emerald-600 text-white shadow-xs shadow-emerald-500/30' :
                    state === 'active' ? 'bg-blue-600 text-white ring-4 ring-blue-500/20 shadow-xs shadow-blue-500/40' :
                    state === 'error' ? 'bg-rose-600 text-white' :
                    'bg-[#181B20] text-slate-500 border border-[#262B34]'
                  }`}>
                    {state === 'active' ? (
                      <Loader2 className="w-4 h-4 animate-spin" />
                    ) : state === 'completed' ? (
                      <CheckCircle2 className="w-4 h-4" />
                    ) : (
                      <StepIcon className="w-3.5 h-3.5" />
                    )}
                  </div>
                  <span className={`text-[10px] font-medium text-center truncate max-w-[80px] ${
                    state === 'active' ? 'text-blue-400 font-bold' :
                    state === 'completed' ? 'text-emerald-400' :
                    'text-slate-500'
                  }`}>
                    {step.label}
                  </span>
                </div>
              );
            })}
          </div>
        </div>

        {/* Progress & Live Metrics */}
        <div className="p-6 space-y-5">
          {/* Main Progress Bar */}
          <div className="space-y-2">
            <div className="flex justify-between items-baseline text-xs font-mono">
              <span className="text-slate-300 font-medium truncate max-w-[340px]">
                {status.message}
              </span>
              <span className="text-blue-400 font-bold text-sm">
                {(status.progress?.percentage || 0).toFixed(1)}%
              </span>
            </div>

            <div className="w-full bg-[#181B20] h-2.5 rounded-full overflow-hidden border border-[#262B34]">
              <div 
                className={`h-full rounded-full transition-all duration-300 ${
                  isCompleted ? 'bg-emerald-500' :
                  isError ? 'bg-rose-500' :
                  'bg-gradient-to-r from-blue-500 via-indigo-500 to-emerald-400'
                }`}
                style={{ width: `${Math.max(isCompleted ? 100 : 3, status.progress?.percentage || 0)}%` }}
              />
            </div>
          </div>

          {/* Metrics Grid */}
          <div className="grid grid-cols-3 gap-3 p-3 bg-[#0F1115] rounded-xl border border-[#262B34] text-center font-mono">
            <div>
              <div className="text-[10px] text-slate-500 uppercase tracking-wider">Processed</div>
              <div className="text-xs font-semibold text-slate-200 mt-0.5">
                {formatBytes(status.progress?.processedBytes || 0)}
                {status.progress?.totalBytes ? ` / ${formatBytes(status.progress.totalBytes)}` : ''}
              </div>
            </div>
            <div>
              <div className="text-[10px] text-slate-500 uppercase tracking-wider">Throughput</div>
              <div className="text-xs font-semibold text-emerald-400 mt-0.5">
                {(status.progress?.speedMbs || 0).toFixed(1)} MB/s
              </div>
            </div>
            <div>
              <div className="text-[10px] text-slate-500 uppercase tracking-wider">Est. Remaining</div>
              <div className="text-xs font-semibold text-slate-200 mt-0.5">
                {formatTime(status.progress?.etaSeconds || 0)}
              </div>
            </div>
          </div>

          {/* Manifest & Cryptographic Verification Card */}
          {status.manifest && (
            <div className="p-3 bg-[#181B20]/80 rounded-xl border border-[#262B34] space-y-1.5 text-xs font-mono">
              <div className="flex items-center justify-between text-slate-400 pb-1 border-b border-[#262B34]">
                <span className="flex items-center space-x-1.5 text-slate-300">
                  <ShieldCheck className="w-3.5 h-3.5 text-emerald-400" />
                  <span>Verified Manifest Details</span>
                </span>
                <span className="text-[10px] text-emerald-400">Ed25519 Signed</span>
              </div>
              <div className="grid grid-cols-2 gap-2 text-[11px] pt-1">
                <div>
                  <span className="text-slate-500">Chain Height: </span>
                  <span className="text-white font-bold">{status.manifest.chainHeight.toLocaleString()}</span>
                </div>
                <div>
                  <span className="text-slate-500">Format: </span>
                  <span className="text-white font-bold">{status.manifest.format}</span>
                </div>
                <div className="col-span-2 truncate">
                  <span className="text-slate-500">SHA-256: </span>
                  <span className="text-emerald-400 font-bold">{status.manifest.sha256}</span>
                </div>
              </div>
            </div>
          )}

          {/* Error Message */}
          {isError && (
            <div className="p-3 bg-rose-500/10 border border-rose-500/30 rounded-xl flex items-start space-x-2 text-rose-300 text-xs">
              <AlertTriangle className="w-4 h-4 flex-shrink-0 mt-0.5 text-rose-400" />
              <div>
                <strong className="font-semibold block">Operation Failed</strong>
                <span className="text-slate-300 font-mono text-[11px]">{status.errorMessage || status.message}</span>
              </div>
            </div>
          )}

          {/* Success Message */}
          {isCompleted && (
            <div className="p-3 bg-emerald-500/10 border border-emerald-500/30 rounded-xl flex items-start space-x-2 text-emerald-300 text-xs">
              <CheckCircle2 className="w-4 h-4 flex-shrink-0 mt-0.5 text-emerald-400" />
              <div>
                <strong className="font-semibold block">Operation Succeeded</strong>
                <span className="text-slate-300 text-[11px]">{status.message}</span>
              </div>
            </div>
          )}
        </div>

        {/* Footer Actions */}
        <div className="px-6 py-4 bg-[#0F1115] border-t border-[#262B34] flex items-center justify-between">
          <div className="text-[11px] text-slate-500 font-mono">
            {isRunning ? 'Decompressing directly into data.path (zero disk overhead)' : 'Operation ended'}
          </div>

          <div className="flex space-x-2">
            {isRunning ? (
              <button
                type="button"
                onClick={onCancel}
                className="px-4 py-2 bg-rose-600/20 hover:bg-rose-600/30 text-rose-300 border border-rose-500/30 rounded-xl text-xs font-semibold transition-all flex items-center space-x-1.5"
              >
                <XCircle className="w-3.5 h-3.5" />
                <span>Cancel Operation</span>
              </button>
            ) : (
              <button
                type="button"
                onClick={onClose}
                className="px-5 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-xl text-xs font-semibold shadow-xs transition-all"
              >
                Close
              </button>
            )}
          </div>
        </div>

      </div>
    </div>
  );
};
