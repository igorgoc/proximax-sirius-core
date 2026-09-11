import React, { useState, useEffect } from 'react';
import {
  X,
  Cpu,
  AlertTriangle,
  CheckCircle2,
  Terminal,
  ExternalLink,
  Copy,
  Check,
  RefreshCw,
  HelpCircle,
  ChevronDown,
  ChevronUp,
  ShieldCheck,
  Layers,
  ArrowRight
} from 'lucide-react';
import { WSLStatus } from '../types';

interface WSLSetupModalProps {
  isOpen: boolean;
  onClose: () => void;
  wslStatus: WSLStatus | null;
  onRefreshStatus: () => void;
}

export const WSLSetupModal: React.FC<WSLSetupModalProps> = ({
  isOpen,
  onClose,
  wslStatus,
  onRefreshStatus,
}) => {
  const [isInstalling, setIsInstalling] = useState(false);
  const [installError, setInstallError] = useState<string | null>(null);
  const [copiedCmd, setCopiedCmd] = useState(false);
  const [showManual, setShowManual] = useState(false);

  // Poll WSL status while modal is open and WSL is not ready
  useEffect(() => {
    if (!isOpen) return;
    if (wslStatus?.state === 'WSL2_READY') return;

    const timer = setInterval(() => {
      onRefreshStatus();
    }, 2500);

    return () => clearInterval(timer);
  }, [isOpen, wslStatus?.state, onRefreshStatus]);

  if (!isOpen) return null;

  const state = wslStatus?.state || 'WSL_NOT_INSTALLED';
  const isBiosDisabled = wslStatus?.errorCode === 'BIOS_VIRTUALIZATION_DISABLED' ||
    wslStatus?.errorMessage?.toLowerCase().includes('virtual machine platform') ||
    wslStatus?.rawStatus?.includes('0x80370102');

  const isUacDenied = installError?.includes('UAC_DENIED') ||
    wslStatus?.errorMessage?.includes('UAC_DENIED') ||
    installError?.toLowerCase().includes('canceled by the user');

  const handleEnableWSL = async () => {
    setIsInstalling(true);
    setInstallError(null);
    try {
      const res = await fetch('/api/system/wsl/install', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
      });
      const data = await res.json();
      if (!res.ok) {
        setInstallError(data.error || 'Failed to initiate subsystem installation');
      } else {
        onRefreshStatus();
      }
    } catch (e: any) {
      setInstallError(e.message || 'Network error occurred while requesting subsystem setup');
    } finally {
      setIsInstalling(false);
    }
  };

  const handleSetupDistro = async () => {
    setIsInstalling(true);
    setInstallError(null);
    try {
      const res = await fetch('/api/system/wsl/setup-distro', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ distro: 'Ubuntu-22.04' }),
      });
      const data = await res.json();
      if (!res.ok) {
        setInstallError(data.error || 'Failed to setup Sirius Linux subsystem');
      } else {
        onRefreshStatus();
      }
    } catch (e: any) {
      setInstallError(e.message || 'Network error occurred');
    } finally {
      setIsInstalling(false);
    }
  };

  const manualScript = `dism.exe /online /enable-feature /featurename:Microsoft-Windows-Subsystem-Linux /all /norestart
dism.exe /online /enable-feature /featurename:VirtualMachinePlatform /all /norestart
wsl --set-default-version 2
wsl --install -d Ubuntu-22.04 --no-launch`;

  const handleCopyCmd = () => {
    navigator.clipboard.writeText(manualScript);
    setCopiedCmd(true);
    setTimeout(() => setCopiedCmd(false), 2000);
  };

  const handleDismiss = () => {
    sessionStorage.setItem('wsl_setup_dismissed', 'true');
    onClose();
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm animate-fadeIn">
      <div className="bg-zinc-900 border border-zinc-700/80 rounded-2xl w-full max-w-2xl overflow-hidden shadow-2xl flex flex-col max-h-[90vh]">
        {/* Header */}
        <div className="p-5 border-b border-zinc-800 flex items-center justify-between bg-zinc-950/70">
          <div className="flex items-center space-x-3">
            <div className={`p-2.5 rounded-xl border flex items-center justify-center ${
              state === 'WSL2_READY'
                ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-400'
                : isBiosDisabled
                ? 'bg-red-500/10 border-red-500/30 text-red-400'
                : 'bg-indigo-500/10 border-indigo-500/30 text-indigo-400'
            }`}>
              {state === 'WSL2_READY' ? <ShieldCheck className="w-5 h-5" /> : <Cpu className="w-5 h-5" />}
            </div>
            <div>
              <h2 className="text-base font-bold text-zinc-100 flex items-center gap-2">
                <span>High-Performance Blockchain Subsystem</span>
                {state === 'WSL2_READY' && (
                  <span className="px-2 py-0.5 rounded-full bg-emerald-500/20 text-emerald-400 border border-emerald-500/30 text-[10px] font-semibold">
                    Ready
                  </span>
                )}
              </h2>
              <p className="text-xs text-zinc-400 mt-0.5">
                Native Windows Subsystem for Linux (WSL2) Engine Integration
              </p>
            </div>
          </div>
          <button
            onClick={handleDismiss}
            className="p-1.5 text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 rounded-lg transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Content Body */}
        <div className="p-6 overflow-y-auto space-y-5 text-sm">
          {/* Reboot Resume Banner */}
          {wslStatus?.wslInstallInitiated && state === 'WSL2_READY' && (
            <div className="p-4 rounded-xl bg-emerald-950/40 border border-emerald-500/40 flex items-start space-x-3 animate-fadeIn">
              <CheckCircle2 className="w-5 h-5 text-emerald-400 flex-shrink-0 mt-0.5" />
              <div>
                <h4 className="text-xs font-bold text-emerald-300 uppercase tracking-wider">
                  Setup Successfully Detected
                </h4>
                <p className="text-xs text-zinc-300 mt-1">
                  Blockchain subsystem detected successfully! Finalizing setup...
                </p>
              </div>
            </div>
          )}

          {/* BIOS Virtualization Disabled Alert */}
          {isBiosDisabled && (
            <div className="p-4 rounded-xl bg-red-950/40 border border-red-500/50 space-y-3 animate-fadeIn">
              <div className="flex items-start space-x-3">
                <AlertTriangle className="w-5 h-5 text-red-400 flex-shrink-0 mt-0.5" />
                <div>
                  <h4 className="font-semibold text-red-300">
                    Hardware Virtualization Disabled in BIOS/UEFI
                  </h4>
                  <p className="text-xs text-zinc-300 mt-1 leading-relaxed">
                    WSL2 requires CPU virtualization to run the high-speed RocksDB blockchain engine. Virtualization is currently disabled on your motherboard.
                  </p>
                </div>
              </div>

              <div className="bg-zinc-950/80 p-3 rounded-lg border border-red-500/20 text-xs text-zinc-300 space-y-2">
                <div className="font-semibold text-red-200">How to Enable in 2 Minutes:</div>
                <ol className="list-decimal list-inside space-y-1 text-zinc-400">
                  <li>Restart your computer and press <kbd className="px-1.5 py-0.5 bg-zinc-800 rounded text-zinc-200 font-mono">F2</kbd>, <kbd className="px-1.5 py-0.5 bg-zinc-800 rounded text-zinc-200 font-mono">Del</kbd>, or <kbd className="px-1.5 py-0.5 bg-zinc-800 rounded text-zinc-200 font-mono">Esc</kbd> during startup to enter BIOS.</li>
                  <li>Locate <strong className="text-zinc-200">Intel Virtualization Technology (VT-x)</strong> or <strong className="text-zinc-200">AMD SVM / AMD-V</strong> in Advanced/CPU configuration.</li>
                  <li>Change setting to <strong className="text-emerald-400">Enabled</strong>, press <kbd className="px-1.5 py-0.5 bg-zinc-800 rounded text-zinc-200 font-mono">F10</kbd> to save and restart.</li>
                </ol>
              </div>
            </div>
          )}

          {/* UAC Denial Card */}
          {isUacDenied && !isBiosDisabled && (
            <div className="p-4 rounded-xl bg-amber-950/40 border border-amber-500/40 flex items-start space-x-3 animate-fadeIn">
              <AlertTriangle className="w-5 h-5 text-amber-400 flex-shrink-0 mt-0.5" />
              <div className="space-y-2">
                <h4 className="font-semibold text-amber-300">
                  Administrator Permission Required
                </h4>
                <p className="text-xs text-zinc-300 leading-relaxed">
                  Administrator permissions were declined. ProximaX Sirius requires permission once to enable the Windows virtualization feature.
                </p>
                <button
                  onClick={handleEnableWSL}
                  disabled={isInstalling}
                  className="px-3 py-1.5 bg-amber-600 hover:bg-amber-500 text-white rounded-lg text-xs font-semibold shadow transition-all flex items-center space-x-1.5"
                >
                  <RefreshCw className={`w-3.5 h-3.5 ${isInstalling ? 'animate-spin' : ''}`} />
                  <span>Try Again</span>
                </button>
              </div>
            </div>
          )}

          {/* General Install Error */}
          {installError && !isUacDenied && (
            <div className="p-3.5 rounded-xl bg-red-950/30 border border-red-500/30 text-xs text-red-300 flex items-center space-x-2">
              <AlertTriangle className="w-4 h-4 flex-shrink-0" />
              <span>{installError}</span>
            </div>
          )}

          {/* Main State Panel */}
          <div className="p-4 rounded-xl bg-zinc-950 border border-zinc-800 space-y-4">
            <div className="flex items-center justify-between">
              <div className="flex items-center space-x-2.5">
                <Layers className="w-4 h-4 text-indigo-400" />
                <span className="font-semibold text-zinc-200">Subsystem Status</span>
              </div>
              <button
                onClick={onRefreshStatus}
                className="text-xs text-zinc-400 hover:text-indigo-400 flex items-center space-x-1 transition-colors"
                title="Refresh Status"
              >
                <RefreshCw className="w-3.5 h-3.5" />
                <span>Check Status</span>
              </button>
            </div>

            {state === 'WSL_NOT_INSTALLED' && (
              <div className="space-y-3">
                <p className="text-xs text-zinc-300 leading-relaxed">
                  The Windows Subsystem for Linux (WSL2) is not yet active. ProximaX Sirius runs the C++ Sirius engine inside WSL2 to achieve native ext4 RocksDB throughput with zero Docker overhead.
                </p>
                <div className="pt-1">
                  <button
                    onClick={handleEnableWSL}
                    disabled={isInstalling}
                    className="w-full py-2.5 px-4 rounded-xl bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white font-semibold text-xs shadow-lg shadow-indigo-600/30 transition-all flex items-center justify-center space-x-2"
                  >
                    {isInstalling ? (
                      <>
                        <RefreshCw className="w-4 h-4 animate-spin" />
                        <span>Enabling Blockchain Subsystem...</span>
                      </>
                    ) : (
                      <>
                        <Cpu className="w-4 h-4" />
                        <span>Enable Blockchain Subsystem</span>
                      </>
                    )}
                  </button>
                  <p className="text-[11px] text-zinc-500 text-center mt-2">
                    Windows will display a User Account Control (UAC) prompt to allow enabling the virtualization feature.
                  </p>
                </div>
              </div>
            )}

            {state === 'WSL_V1_ONLY' && (
              <div className="space-y-3">
                <p className="text-xs text-amber-300/90 leading-relaxed">
                  WSL Version 1 is detected. Sirius requires WSL Version 2 for Linux ext4 file system performance and memory-mapped consensus storage.
                </p>
                <button
                  onClick={handleSetupDistro}
                  disabled={isInstalling}
                  className="w-full py-2.5 px-4 rounded-xl bg-amber-600 hover:bg-amber-500 disabled:opacity-50 text-white font-semibold text-xs shadow transition-all flex items-center justify-center space-x-2"
                >
                  {isInstalling ? (
                    <RefreshCw className="w-4 h-4 animate-spin" />
                  ) : (
                    <Layers className="w-4 h-4" />
                  )}
                  <span>Upgrade to WSL2 Subsystem</span>
                </button>
              </div>
            )}

            {state === 'WSL2_NO_DISTRO' && (
              <div className="space-y-3">
                <p className="text-xs text-zinc-300 leading-relaxed">
                  WSL2 core is active! The isolated Linux subsystem environment (Ubuntu-22.04) needs to be initialized to execute the Sirius node engine.
                </p>
                <button
                  onClick={handleSetupDistro}
                  disabled={isInstalling}
                  className="w-full py-2.5 px-4 rounded-xl bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white font-semibold text-xs shadow-lg shadow-indigo-600/30 transition-all flex items-center justify-center space-x-2"
                >
                  {isInstalling ? (
                    <>
                      <RefreshCw className="w-4 h-4 animate-spin" />
                      <span>Setting Up Subsystem Distribution...</span>
                    </>
                  ) : (
                    <>
                      <Cpu className="w-4 h-4" />
                      <span>Install Sirius Linux Subsystem (Ubuntu-22.04)</span>
                    </>
                  )}
                </button>
              </div>
            )}

            {state === 'WSL2_READY' && (
              <div className="space-y-3">
                <div className="flex items-center space-x-3 p-3 rounded-lg bg-emerald-950/30 border border-emerald-500/30">
                  <CheckCircle2 className="w-5 h-5 text-emerald-400 flex-shrink-0" />
                  <div>
                    <div className="font-semibold text-emerald-300 text-xs">
                      Subsystem is Fully Operational
                    </div>
                    <div className="text-[11px] text-zinc-400 font-mono mt-0.5">
                      Distro: {wslStatus?.distroName || 'Ubuntu-22.04'} • Mode: WSL{wslStatus?.defaultVersion || 2}
                    </div>
                  </div>
                </div>
                <button
                  onClick={handleDismiss}
                  className="w-full py-2.5 px-4 rounded-xl bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs shadow-lg shadow-emerald-600/20 transition-all flex items-center justify-center space-x-2"
                >
                  <span>Proceed to Node Manager</span>
                  <ArrowRight className="w-4 h-4" />
                </button>
              </div>
            )}
          </div>

          {/* Linux Kernel Update Link */}
          <div className="p-3.5 rounded-xl bg-zinc-950/60 border border-zinc-800/80 flex items-center justify-between text-xs">
            <div className="flex items-center space-x-2.5">
              <HelpCircle className="w-4 h-4 text-zinc-400" />
              <span className="text-zinc-300">Need the official Microsoft WSL2 Linux Kernel update?</span>
            </div>
            <a
              href="https://wslstorestorage.blob.core.windows.net/wslblob/wsl_update_x64.msi"
              target="_blank"
              rel="noopener noreferrer"
              className="text-indigo-400 hover:text-indigo-300 flex items-center space-x-1 font-medium underline flex-shrink-0"
            >
              <span>Download MSI (Official)</span>
              <ExternalLink className="w-3.5 h-3.5" />
            </a>
          </div>

          {/* Manual PowerShell Accordion */}
          <div className="border border-zinc-800 rounded-xl overflow-hidden">
            <button
              onClick={() => setShowManual(!showManual)}
              className="w-full p-3.5 bg-zinc-950 flex items-center justify-between text-xs text-zinc-400 hover:text-zinc-200 transition-colors"
            >
              <div className="flex items-center space-x-2">
                <Terminal className="w-4 h-4 text-zinc-500" />
                <span className="font-semibold">Manual Setup via Administrator PowerShell</span>
              </div>
              {showManual ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
            </button>
            {showManual && (
              <div className="p-4 bg-zinc-950/90 border-t border-zinc-800 space-y-2">
                <div className="flex items-center justify-between">
                  <span className="text-[11px] text-zinc-400">Run in an elevated PowerShell terminal:</span>
                  <button
                    onClick={handleCopyCmd}
                    className="flex items-center space-x-1 text-xs text-indigo-400 hover:text-indigo-300 transition-colors"
                  >
                    {copiedCmd ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                    <span>{copiedCmd ? 'Copied!' : 'Copy Script'}</span>
                  </button>
                </div>
                <pre className="p-3 bg-zinc-900 rounded-lg border border-zinc-800 font-mono text-[11px] text-zinc-300 overflow-x-auto select-all">
                  {manualScript}
                </pre>
              </div>
            )}
          </div>
        </div>

        {/* Footer */}
        <div className="p-4 border-t border-zinc-800 bg-zinc-950/80 flex items-center justify-between text-xs">
          <span className="text-zinc-500 text-[11px]">
            {wslStatus?.rawStatus ? `System: ${wslStatus.rawStatus.split('\n')[0]}` : 'Windows 10/11 Architecture'}
          </span>
          <button
            onClick={handleDismiss}
            className="px-4 py-1.5 rounded-lg bg-zinc-800 hover:bg-zinc-700 text-zinc-300 transition-colors"
          >
            Close
          </button>
        </div>
      </div>
    </div>
  );
};
