import React, { useState, useEffect } from 'react';
import { 
  X, 
  ShieldCheck, 
  AlertTriangle, 
  ArrowUpCircle, 
  RefreshCw, 
  CheckCircle2, 
  HardDrive, 
  Terminal, 
  ExternalLink, 
  ChevronDown, 
  ChevronUp, 
  RotateCcw,
  Check
} from 'lucide-react';
import { EngineUpdateStatus } from '../types';

interface EngineUpdateModalProps {
  isOpen: boolean;
  onClose: () => void;
  engineStatus: EngineUpdateStatus | null;
  onRefreshStatus: () => void;
}

export const EngineUpdateModal: React.FC<EngineUpdateModalProps> = ({
  isOpen,
  onClose,
  engineStatus,
  onRefreshStatus,
}) => {
  const [showSimulations, setShowSimulations] = useState(false);
  const [localStatus, setLocalStatus] = useState<EngineUpdateStatus | null>(engineStatus);
  const [pollTimer, setPollTimer] = useState<any>(null);

  useEffect(() => {
    const isTest = new URLSearchParams(window.location.search).get('test_view');
    if (!isTest || !localStatus) {
      setLocalStatus(engineStatus);
    }
  }, [engineStatus]);

  // Fast poll when applying
  useEffect(() => {
    const isTest = new URLSearchParams(window.location.search).get('test_view');
    if (isTest) return; // Preserve test snapshot view without poll overwrite

    if (localStatus?.isApplying) {
      const timer = setInterval(async () => {
        try {
          const res = await fetch('/api/engine/status');
          if (res.ok) {
            const data: EngineUpdateStatus = await res.json();
            setLocalStatus(data);
            if (!data.isApplying) {
              clearInterval(timer);
              onRefreshStatus();
            }
          }
        } catch (e) {
          console.warn('Poll engine status err:', e);
        }
      }, 500);
      setPollTimer(timer);
      return () => clearInterval(timer);
    }
  }, [localStatus?.isApplying, onRefreshStatus]);

  if (!isOpen) return null;

  const currentVer = localStatus?.currentVersion || 'v1.9.7';
  const targetVer = localStatus?.targetVersion || 'v1.9.8';
  const isApplying = localStatus?.isApplying || false;
  const state = localStatus?.state || 'idle';
  const isRolledBack = state === 'rolled_back' || localStatus?.rollbackOccurred;
  const isCompleted = state === 'completed';
  const isFailed = state === 'failed';

  const handleApply = async (scenario: string = 'normal') => {
    try {
      await fetch('/api/engine/apply', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ version: targetVer, scenario }),
      });
      // Trigger status refresh
      const res = await fetch('/api/engine/status');
      if (res.ok) {
        const data = await res.json();
        setLocalStatus(data);
      }
    } catch (e) {
      console.error('Apply engine update error:', e);
    }
  };

  const handleReset = async () => {
    try {
      const res = await fetch('/api/engine/reset', { method: 'POST' });
      if (res.ok) {
        const data = await res.json();
        setLocalStatus(data);
        onRefreshStatus();
      }
    } catch (e) {
      console.error('Reset status error:', e);
    }
  };

  // Step progress calculation
  const getStepStatus = (stepIndex: number) => {
    // Step 0: Verification, Step 1: Swap, Step 2: Healthcheck
    if (isCompleted) return 'completed';
    if (isRolledBack && stepIndex === 2) return 'failed';
    if (isFailed && stepIndex === 0) return 'failed';

    if (state === 'verifying') {
      if (stepIndex === 0) return 'active';
      return 'pending';
    }
    if (state === 'swapping') {
      if (stepIndex === 0) return 'completed';
      if (stepIndex === 1) return 'active';
      return 'pending';
    }
    if (state === 'healthcheck') {
      if (stepIndex <= 1) return 'completed';
      if (stepIndex === 2) return 'active';
      return 'pending';
    }
    return 'pending';
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm animate-fadeIn">
      <div 
        className="bg-zinc-900 border border-zinc-700/80 rounded-xl shadow-2xl w-full max-w-2xl overflow-hidden flex flex-col max-h-[90vh]"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Modal Header */}
        <div className="px-6 py-4 border-b border-zinc-800 bg-zinc-900/90 flex items-center justify-between">
          <div className="flex items-center space-x-3">
            <div className="w-10 h-10 rounded-lg bg-indigo-500/10 border border-indigo-500/30 flex items-center justify-center">
              <ArrowUpCircle className="w-5 h-5 text-indigo-400" />
            </div>
            <div>
              <div className="flex items-center space-x-2">
                <h3 className="text-base font-semibold text-zinc-100">Sirius Core Engine Updater</h3>
                <span className="text-xs px-2 py-0.5 rounded-full bg-indigo-500/20 text-indigo-300 font-mono border border-indigo-500/30">
                  cpp-xpx-chain
                </span>
              </div>
              <p className="text-xs text-zinc-400 mt-0.5">
                Atomic, signed, zero-downtime C++ blockchain engine updates with automated rollback
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            disabled={isApplying}
            className="text-zinc-400 hover:text-zinc-200 p-1.5 rounded-lg hover:bg-zinc-800 transition-colors disabled:opacity-40"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Modal Body */}
        <div className="p-6 overflow-y-auto space-y-6 flex-1 text-sm">
          
          {/* Version Comparison Card */}
          <div className="grid grid-cols-2 gap-4 bg-zinc-800/40 p-4 rounded-lg border border-zinc-800">
            <div>
              <span className="text-xs font-medium text-zinc-400 uppercase tracking-wider block mb-1">
                Active Node Version
              </span>
              <div className="flex items-center space-x-2">
                <span className="font-mono text-lg font-bold text-zinc-200">{currentVer}</span>
                <span className="text-xs px-2 py-0.5 rounded bg-zinc-700/50 text-zinc-300 border border-zinc-600/50">
                  Installed
                </span>
              </div>
            </div>

            <div>
              <span className="text-xs font-medium text-zinc-400 uppercase tracking-wider block mb-1">
                Target Release
              </span>
              <div className="flex items-center space-x-2">
                <span className="font-mono text-lg font-bold text-indigo-400">{targetVer}</span>
                <span className="text-xs px-2 py-0.5 rounded bg-emerald-500/20 text-emerald-300 border border-emerald-500/30 font-medium">
                  Verified Compatible
                </span>
              </div>
            </div>
          </div>

          {/* Release Notes */}
          {localStatus?.releaseNotes && (
            <div className="bg-zinc-950/60 rounded-lg p-4 border border-zinc-800/80">
              <span className="text-xs font-semibold text-zinc-300 uppercase tracking-wider flex items-center justify-between mb-2">
                <span>Release Highlights ({targetVer})</span>
                {localStatus.releaseUrl && (
                  <a 
                    href={localStatus.releaseUrl} 
                    target="_blank" 
                    rel="noreferrer" 
                    className="text-indigo-400 hover:underline inline-flex items-center text-xs space-x-1"
                  >
                    <span>GitHub Release</span>
                    <ExternalLink className="w-3 h-3" />
                  </a>
                )}
              </span>
              <p className="text-xs text-zinc-300 leading-relaxed font-mono whitespace-pre-wrap">
                {localStatus.releaseNotes}
              </p>
            </div>
          )}

          {/* ACTIVE PROGRESS / STEPS (when applying or in result state) */}
          {(isApplying || isCompleted || isRolledBack || isFailed) && (
            <div className="space-y-4 pt-2">
              <span className="text-xs font-semibold text-zinc-400 uppercase tracking-wider block">
                Update Pipeline Status
              </span>

              <div className="space-y-3">
                {/* Step 1: Verification */}
                <div className={`p-3.5 rounded-lg border transition-all flex items-start space-x-3.5 ${
                  getStepStatus(0) === 'active' 
                    ? 'bg-indigo-950/30 border-indigo-500/50 text-indigo-200' 
                    : getStepStatus(0) === 'completed'
                    ? 'bg-emerald-950/20 border-emerald-800/40 text-emerald-200'
                    : getStepStatus(0) === 'failed'
                    ? 'bg-rose-950/30 border-rose-800/50 text-rose-200'
                    : 'bg-zinc-800/20 border-zinc-800/60 text-zinc-500'
                }`}>
                  <div className="mt-0.5">
                    {getStepStatus(0) === 'active' && <RefreshCw className="w-4 h-4 text-indigo-400 animate-spin" />}
                    {getStepStatus(0) === 'completed' && <CheckCircle2 className="w-4 h-4 text-emerald-400" />}
                    {getStepStatus(0) === 'failed' && <AlertTriangle className="w-4 h-4 text-rose-400" />}
                    {getStepStatus(0) === 'pending' && <div className="w-4 h-4 rounded-full border border-zinc-600" />}
                  </div>
                  <div className="flex-1">
                    <div className="text-xs font-semibold flex items-center justify-between">
                      <span>1. Ed25519 Cryptographic Signature & SHA-256 Stream Verification</span>
                      <span className="text-[10px] font-mono uppercase opacity-75">
                        {getStepStatus(0)}
                      </span>
                    </div>
                    <p className="text-[11px] opacity-80 mt-0.5">
                      Validates release manifest against official Sirius release keys before binary bytes touch disk (Fail-Closed).
                    </p>
                  </div>
                </div>

                {/* Step 2: Atomic Swap */}
                <div className={`p-3.5 rounded-lg border transition-all flex items-start space-x-3.5 ${
                  getStepStatus(1) === 'active' 
                    ? 'bg-indigo-950/30 border-indigo-500/50 text-indigo-200' 
                    : getStepStatus(1) === 'completed'
                    ? 'bg-emerald-950/20 border-emerald-800/40 text-emerald-200'
                    : getStepStatus(1) === 'failed'
                    ? 'bg-rose-950/30 border-rose-800/50 text-rose-200'
                    : 'bg-zinc-800/20 border-zinc-800/60 text-zinc-500'
                }`}>
                  <div className="mt-0.5">
                    {getStepStatus(1) === 'active' && <RefreshCw className="w-4 h-4 text-indigo-400 animate-spin" />}
                    {getStepStatus(1) === 'completed' && <CheckCircle2 className="w-4 h-4 text-emerald-400" />}
                    {getStepStatus(1) === 'failed' && <AlertTriangle className="w-4 h-4 text-rose-400" />}
                    {getStepStatus(1) === 'pending' && <div className="w-4 h-4 rounded-full border border-zinc-600" />}
                  </div>
                  <div className="flex-1">
                    <div className="text-xs font-semibold flex items-center justify-between">
                      <span>2. Atomic Binary Swap & Fallback Preservation</span>
                      <span className="text-[10px] font-mono uppercase opacity-75">
                        {getStepStatus(1)}
                      </span>
                    </div>
                    <p className="text-[11px] opacity-80 mt-0.5">
                      Stops active node gracefully, creates <code className="text-zinc-300">sirius.bc.bak</code> backup, and renames verified binary atomically.
                    </p>
                  </div>
                </div>

                {/* Step 3: Healthcheck */}
                <div className={`p-3.5 rounded-lg border transition-all flex items-start space-x-3.5 ${
                  getStepStatus(2) === 'active' 
                    ? 'bg-indigo-950/30 border-indigo-500/50 text-indigo-200' 
                    : getStepStatus(2) === 'completed'
                    ? 'bg-emerald-950/20 border-emerald-800/40 text-emerald-200'
                    : getStepStatus(2) === 'failed'
                    ? 'bg-amber-950/30 border-amber-800/50 text-amber-200'
                    : 'bg-zinc-800/20 border-zinc-800/60 text-zinc-500'
                }`}>
                  <div className="mt-0.5">
                    {getStepStatus(2) === 'active' && <RefreshCw className="w-4 h-4 text-indigo-400 animate-spin" />}
                    {getStepStatus(2) === 'completed' && <CheckCircle2 className="w-4 h-4 text-emerald-400" />}
                    {getStepStatus(2) === 'failed' && <RotateCcw className="w-4 h-4 text-amber-400" />}
                    {getStepStatus(2) === 'pending' && <div className="w-4 h-4 rounded-full border border-zinc-600" />}
                  </div>
                  <div className="flex-1">
                    <div className="text-xs font-semibold flex items-center justify-between">
                      <span>3. Post-Update Operational Healthcheck Probe</span>
                      <span className="text-[10px] font-mono uppercase opacity-75">
                        {getStepStatus(2)}
                      </span>
                    </div>
                    <p className="text-[11px] opacity-80 mt-0.5">
                      Starts new binary and verifies heartbeat & block height progression. Triggers instant auto-rollback on failure.
                    </p>
                  </div>
                </div>
              </div>

              {/* Live Status Message Card */}
              {localStatus?.message && (
                <div className={`p-4 rounded-lg border font-mono text-xs ${
                  isRolledBack 
                    ? 'bg-amber-950/40 border-amber-700/60 text-amber-200'
                    : isCompleted
                    ? 'bg-emerald-950/40 border-emerald-700/60 text-emerald-200'
                    : isFailed
                    ? 'bg-rose-950/40 border-rose-700/60 text-rose-200'
                    : 'bg-zinc-950 border-zinc-800 text-zinc-300'
                }`}>
                  <div className="flex items-center space-x-2 font-semibold mb-1">
                    <Terminal className="w-4 h-4 flex-shrink-0" />
                    <span>
                      {isRolledBack 
                        ? 'Automated Rollback Executed' 
                        : isCompleted 
                        ? 'Engine Operational' 
                        : isFailed
                        ? 'Verification Error'
                        : 'Supervisor Log'}
                    </span>
                  </div>
                  <p className="leading-relaxed whitespace-pre-wrap">{localStatus.message}</p>
                </div>
              )}
            </div>
          )}

          {/* Rollback Safety Notice Card */}
          {isRolledBack && (
            <div className="bg-amber-900/20 border border-amber-600/40 rounded-lg p-4 text-amber-200 space-y-2">
              <div className="flex items-center space-x-2 text-xs font-bold text-amber-300 uppercase tracking-wide">
                <ShieldCheck className="w-4 h-4 text-amber-400" />
                <span>Zero-Downtime Rollback Guarantee Satisfied</span>
              </div>
              <p className="text-xs text-amber-200/90 leading-relaxed">
                The updated engine crashed or failed to establish consensus during the post-update healthcheck. 
                The supervisor immediately aborted the deployment, restored your previous binary (<span className="font-mono font-bold text-amber-300">{currentVer}</span>) from backup, and revived the node process cleanly. No manual file recovery or data resync is needed.
              </p>
            </div>
          )}

          {/* Idle Safeguards Banner (When not applying yet) */}
          {state === 'idle' && (
            <div className="bg-zinc-950/40 rounded-lg p-4 border border-zinc-800/80 space-y-3">
              <span className="text-xs font-semibold text-zinc-400 uppercase tracking-wider block">
                Autonomous Safety Invariants
              </span>
              <ul className="text-xs text-zinc-300 space-y-2">
                <li className="flex items-start space-x-2">
                  <span className="text-indigo-400 font-bold">•</span>
                  <span><strong>Cryptographic Authenticity:</strong> Verified via Ed25519 public key before write.</span>
                </li>
                <li className="flex items-start space-x-2">
                  <span className="text-indigo-400 font-bold">•</span>
                  <span><strong>Atomic Replacement:</strong> Existing binary backed up to <code className="text-zinc-200">.bak</code> prior to swap.</span>
                </li>
                <li className="flex items-start space-x-2">
                  <span className="text-indigo-400 font-bold">•</span>
                  <span><strong>Crash-Loop Protection:</strong> If the updated engine fails healthcheck, supervisor rolls back instantly.</span>
                </li>
              </ul>
            </div>
          )}

          {/* Adversarial Testing & Simulation Accordion */}
          <div className="border border-zinc-800/80 rounded-lg overflow-hidden">
            <button
              onClick={() => setShowSimulations(!showSimulations)}
              className="w-full px-4 py-2.5 bg-zinc-800/30 hover:bg-zinc-800/50 flex items-center justify-between text-xs font-medium text-zinc-400 hover:text-zinc-300 transition-colors"
            >
              <span>Test & Simulation Scenarios (Adversarial Verification)</span>
              {showSimulations ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
            </button>
            {showSimulations && (
              <div className="p-3.5 bg-zinc-950/60 border-t border-zinc-800/80 space-y-2">
                <p className="text-[11px] text-zinc-400 mb-2">
                  Simulate live update outcomes to test UI resilience and automatic rollback handling:
                </p>
                <div className="grid grid-cols-3 gap-2">
                  <button
                    onClick={() => handleApply('normal')}
                    disabled={isApplying}
                    className="px-2.5 py-1.5 rounded bg-emerald-950/40 hover:bg-emerald-900/60 border border-emerald-800/50 text-emerald-300 text-xs font-medium transition-colors text-center disabled:opacity-40"
                  >
                    Simulate Success
                  </button>
                  <button
                    onClick={() => handleApply('rollback')}
                    disabled={isApplying}
                    className="px-2.5 py-1.5 rounded bg-amber-950/40 hover:bg-amber-900/60 border border-amber-800/50 text-amber-300 text-xs font-medium transition-colors text-center disabled:opacity-40"
                  >
                    Simulate Auto-Rollback
                  </button>
                  <button
                    onClick={() => handleApply('signature_fail')}
                    disabled={isApplying}
                    className="px-2.5 py-1.5 rounded bg-rose-950/40 hover:bg-rose-900/60 border border-rose-800/50 text-rose-300 text-xs font-medium transition-colors text-center disabled:opacity-40"
                  >
                    Simulate Bad Signature
                  </button>
                </div>
                {(isRolledBack || isCompleted || isFailed) && (
                  <div className="pt-1">
                    <button
                      onClick={handleReset}
                      className="text-xs text-zinc-400 hover:text-zinc-200 underline"
                    >
                      Reset updater status to idle
                    </button>
                  </div>
                )}
              </div>
            )}
          </div>

        </div>

        {/* Modal Footer */}
        <div className="px-6 py-4 border-t border-zinc-800 bg-zinc-900/90 flex items-center justify-between">
          <div className="text-xs text-zinc-500">
            {isApplying ? 'Applying update...' : isRolledBack ? 'Rollback completed' : 'Safe to proceed'}
          </div>

          <div className="flex items-center space-x-3">
            <button
              onClick={onClose}
              disabled={isApplying}
              className="px-4 py-2 rounded-lg bg-zinc-800 hover:bg-zinc-700 text-zinc-300 text-xs font-medium transition-colors disabled:opacity-40"
            >
              {isCompleted || isRolledBack ? 'Close' : 'Cancel'}
            </button>

            {!isCompleted && !isRolledBack && (
              <button
                onClick={() => handleApply('normal')}
                disabled={isApplying}
                className="px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold shadow-md shadow-indigo-600/20 transition-all flex items-center space-x-2 disabled:opacity-40"
              >
                {isApplying ? (
                  <>
                    <RefreshCw className="w-3.5 h-3.5 animate-spin" />
                    <span>Applying Engine Update...</span>
                  </>
                ) : (
                  <>
                    <ArrowUpCircle className="w-3.5 h-3.5" />
                    <span>Apply Engine Update ({targetVer})</span>
                  </>
                )}
              </button>
            )}

            {isRolledBack && (
              <button
                onClick={handleReset}
                className="px-4 py-2 rounded-lg bg-amber-600 hover:bg-amber-500 text-white text-xs font-semibold shadow-md shadow-amber-600/20 transition-all flex items-center space-x-2"
              >
                <RotateCcw className="w-3.5 h-3.5" />
                <span>Acknowledge & Clear</span>
              </button>
            )}
          </div>
        </div>

      </div>
    </div>
  );
};
