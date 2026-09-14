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
  ArrowRight,
  Globe,
  ArrowUpCircle,
  FileText
} from 'lucide-react';
import { WSLStatus } from '../types';
import { ErrorBoundary } from './ErrorBoundary';

interface WSLSetupModalProps {
  isOpen: boolean;
  onClose: () => void;
  wslStatus: WSLStatus | null;
  onRefreshStatus: () => void;
}

// Safe string matching helper: returns false for non-string / null / undefined without throwing
const strContains = (haystack: unknown, needle: string): boolean => {
  if (typeof haystack !== 'string') return false;
  return haystack.toLowerCase().includes(needle.toLowerCase());
};

const WSLSetupModalContent: React.FC<WSLSetupModalProps> = ({
  isOpen,
  onClose,
  wslStatus,
  onRefreshStatus,
}) => {
  const [isInstalling, setIsInstalling] = useState(false);
  const [isUpdatingWSL, setIsUpdatingWSL] = useState(false);
  const [updateMsg, setUpdateMsg] = useState<string | null>(null);
  const [installError, setInstallError] = useState<string | null>(null);
  const [copiedCmd, setCopiedCmd] = useState(false);
  const [showManual, setShowManual] = useState(false);
  const [showLogDetails, setShowLogDetails] = useState(false);
  const [distroInstalling, setDistroInstalling] = useState(() => {
    return sessionStorage.getItem('sirius_wsl_distro_installing') === 'true';
  });

  // Poll WSL status while modal is open and WSL is not ready
  useEffect(() => {
    if (!isOpen) return;
    if (wslStatus?.state === 'WSL2_READY') return;

    const timer = setInterval(() => {
      onRefreshStatus();
    }, 2500);

    return () => clearInterval(timer);
  }, [isOpen, wslStatus?.state, onRefreshStatus]);

  useEffect(() => {
    if (wslStatus?.state === 'WSL2_READY') {
      sessionStorage.removeItem('sirius_wsl_distro_installing');
      setDistroInstalling(false);
    }
  }, [wslStatus?.state]);

  if (!isOpen) return null;

  const state = wslStatus?.state || 'WSL_NOT_INSTALLED';
  const isStep1Ready = state !== 'WSL_NOT_INSTALLED' && state !== 'WSL_V1_ONLY' && !wslStatus?.isOutdated;
  const isStep2Ready = state === 'WSL2_READY';
  const isFullyReady = isStep1Ready && isStep2Ready;

  const isBiosDisabled = wslStatus?.errorCode === 'BIOS_VIRTUALIZATION_DISABLED' ||
    strContains(wslStatus?.errorMessage, 'virtual machine platform') ||
    strContains(wslStatus?.rawStatus, '0x80370102') ||
    strContains(wslStatus?.installLog, '0x80370102');

  const isUacDenied = strContains(installError, 'UAC_DENIED') ||
    strContains(wslStatus?.errorMessage, 'UAC_DENIED') ||
    strContains(installError, 'canceled by the user');

  const isNetworkTimeout = wslStatus?.errorCode === 'NETWORK_TIMEOUT' ||
    strContains(installError, 'NETWORK_TIMEOUT') ||
    strContains(installError, '0x80072ee7') ||
    strContains(wslStatus?.rawStatus, '0x80072ee7') ||
    strContains(wslStatus?.installLog, '0x80072ee7');

  const isGroupPolicyBlocked = wslStatus?.errorCode === 'GROUP_POLICY_BLOCKED' ||
    strContains(installError, 'GROUP_POLICY_BLOCKED') ||
    strContains(installError, '0x8024500c') ||
    strContains(wslStatus?.rawStatus, '0x8024500c') ||
    strContains(wslStatus?.installLog, '0x8024500c');

  const isDistroNotFound = wslStatus?.errorCode === 'DISTRO_NOT_FOUND' ||
    strContains(installError, 'DISTRO_NOT_FOUND') ||
    strContains(wslStatus?.errorMessage, 'not found') ||
    strContains(wslStatus?.installLog, 'not found');

  const handleEnableWSL = async () => {
    setIsInstalling(true);
    setInstallError(null);
    setUpdateMsg(null);
    try {
      const res = await fetch('/api/system/wsl/install', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
      });
      const data = await res.json();
      if (!res.ok) {
        setInstallError(data.error || 'Failed to initiate subsystem installation');
      } else {
        setUpdateMsg(data.message || 'WSL subsystem installation initiated. Please check the elevated PowerShell window on your screen.');
        onRefreshStatus();
      }
    } catch (e: any) {
      setInstallError(e.message || 'Network error occurred while requesting subsystem setup');
    } finally {
      setIsInstalling(false);
    }
  };

  const handleUpdateWSL = async () => {
    setIsUpdatingWSL(true);
    setInstallError(null);
    setUpdateMsg(null);
    try {
      const res = await fetch('/api/system/wsl/update', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
      });
      const data = await res.json();
      if (!res.ok) {
        setInstallError(data.error || 'Failed to initiate WSL update');
      } else {
        setUpdateMsg(data.message || 'WSL update initiated. Please check the elevated PowerShell window on your screen.');
        onRefreshStatus();
      }
    } catch (e: any) {
      setInstallError(e.message || 'Network error occurred while requesting WSL update');
    } finally {
      setIsUpdatingWSL(false);
    }
  };

  const handleSetupDistro = async () => {
    setIsInstalling(true);
    setDistroInstalling(true);
    sessionStorage.setItem('sirius_wsl_distro_installing', 'true');
    setInstallError(null);
    setUpdateMsg(null);
    try {
      const res = await fetch('/api/system/wsl/setup-distro', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ distro: 'Ubuntu-22.04' }),
      });
      const data = await res.json();
      if (!res.ok) {
        setInstallError(data.error || 'Failed to setup Sirius Linux subsystem');
        sessionStorage.removeItem('sirius_wsl_distro_installing');
        setDistroInstalling(false);
      } else {
        setUpdateMsg(data.message || 'WSL distribution setup initiated. Please check the elevated PowerShell window on your screen.');
        onRefreshStatus();
      }
    } catch (e: any) {
      setInstallError(e.message || 'Network error occurred');
      sessionStorage.removeItem('sirius_wsl_distro_installing');
      setDistroInstalling(false);
    } finally {
      setIsInstalling(false);
    }
  };

  const manualScript = `dism.exe /online /enable-feature /featurename:Microsoft-Windows-Subsystem-Linux /all /norestart
dism.exe /online /enable-feature /featurename:VirtualMachinePlatform /all /norestart
wsl --set-default-version 2
wsl --update
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
              <h2 className="text-base font-bold text-zinc-100 flex items-center gap-2 flex-wrap">
                <span>High-Performance Blockchain Subsystem</span>
                {state === 'WSL2_READY' && (
                  <span className="px-2 py-0.5 rounded-full bg-emerald-500/20 text-emerald-400 border border-emerald-500/30 text-[10px] font-semibold">
                    Ready
                  </span>
                )}
                {wslStatus?.wslVersion && (
                  <span className="px-2 py-0.5 rounded-full bg-zinc-800 border border-zinc-700 text-zinc-300 font-mono text-[10px]">
                    WSL v{wslStatus.wslVersion}
                  </span>
                )}
                {wslStatus?.isOutdated && state !== 'WSL2_READY' && (
                  <span className="px-2 py-0.5 rounded-full bg-amber-500/20 text-amber-300 border border-amber-500/30 text-[10px] font-semibold">
                    Update Recommended
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

          {/* Distro Not Found in Catalog Alert */}
          {isDistroNotFound && (
            <div className="p-4 rounded-xl bg-amber-950/40 border border-amber-500/50 space-y-3 animate-fadeIn">
              <div className="flex items-start space-x-3">
                <AlertTriangle className="w-5 h-5 text-amber-400 flex-shrink-0 mt-0.5" />
                <div>
                  <h4 className="font-semibold text-amber-300">
                    Distribution Catalog Requires WSL Update
                  </h4>
                  <p className="text-xs text-zinc-300 mt-1 leading-relaxed">
                    Ubuntu-22.04 is not listed in your system's current WSL catalog. Updating WSL enables the modern Microsoft distribution catalog containing Ubuntu-22.04 LTS.
                  </p>
                </div>
              </div>
              <div className="flex items-center space-x-3 pt-1">
                <button
                  onClick={handleUpdateWSL}
                  disabled={isUpdatingWSL}
                  className="px-3 py-1.5 bg-amber-600 hover:bg-amber-500 text-white rounded-lg text-xs font-semibold shadow transition-all flex items-center space-x-1.5"
                >
                  <RefreshCw className={`w-3.5 h-3.5 ${isUpdatingWSL ? 'animate-spin' : ''}`} />
                  <span>Update WSL (wsl --update)</span>
                </button>
              </div>
            </div>
          )}

          {/* Network Timeout Card */}
          {isNetworkTimeout && (
            <div className="p-4 rounded-xl bg-amber-950/40 border border-amber-500/40 space-y-3 animate-fadeIn">
              <div className="flex items-start space-x-3">
                <Globe className="w-5 h-5 text-amber-400 flex-shrink-0 mt-0.5" />
                <div>
                  <h4 className="font-semibold text-amber-300">
                    Network Connection Timeout (0x80072ee7)
                  </h4>
                  <p className="text-xs text-zinc-300 mt-1 leading-relaxed">
                    WSL installer could not reach Microsoft Store / CDN distribution servers. Please check your internet connection, proxy settings, or VPN.
                  </p>
                </div>
              </div>
              <div className="flex items-center space-x-3 pt-1">
                <button
                  onClick={state === 'WSL2_NO_DISTRO' ? handleSetupDistro : handleEnableWSL}
                  disabled={isInstalling}
                  className="px-3 py-1.5 bg-amber-600 hover:bg-amber-500 text-white rounded-lg text-xs font-semibold shadow transition-all flex items-center space-x-1.5"
                >
                  <RefreshCw className={`w-3.5 h-3.5 ${isInstalling ? 'animate-spin' : ''}`} />
                  <span>Retry Download</span>
                </button>
              </div>
            </div>
          )}

          {/* Group Policy Blocked Card */}
          {isGroupPolicyBlocked && (
            <div className="p-4 rounded-xl bg-red-950/40 border border-red-500/50 space-y-3 animate-fadeIn">
              <div className="flex items-start space-x-3">
                <AlertTriangle className="w-5 h-5 text-red-400 flex-shrink-0 mt-0.5" />
                <div>
                  <h4 className="font-semibold text-red-300">
                    Installation Blocked by Group Policy (0x8024500c)
                  </h4>
                  <p className="text-xs text-zinc-300 mt-1 leading-relaxed">
                    Windows Update / Microsoft Store downloads are restricted by your system administrator or corporate Group Policy.
                  </p>
                </div>
              </div>
              <div className="bg-zinc-950/80 p-3 rounded-lg border border-red-500/20 text-xs text-zinc-300 space-y-1.5">
                <div className="font-semibold text-red-200">Recommended Next Steps:</div>
                <ul className="list-disc list-inside space-y-1 text-zinc-400">
                  <li>Contact your IT administrator to allow Windows Subsystem for Linux packages.</li>
                  <li>Alternatively, install Ubuntu manually using an offline rootfs package.</li>
                </ul>
              </div>
            </div>
          )}

          {/* Active Process / Window Notification */}
          {updateMsg && (
            <div className="p-3.5 rounded-xl bg-emerald-950/40 border border-emerald-500/40 text-xs text-emerald-300 flex items-center space-x-2.5 shadow-lg animate-fadeIn">
              <CheckCircle2 className="w-4 h-4 flex-shrink-0 text-emerald-400 animate-pulse" />
              <div className="flex-1 font-medium">{updateMsg}</div>
            </div>
          )}

          {/* General Install Error */}
          {installError && !isUacDenied && !isBiosDisabled && !isNetworkTimeout && !isGroupPolicyBlocked && !isDistroNotFound && (
            <div className="p-3.5 rounded-xl bg-red-950/30 border border-red-500/30 text-xs text-red-300 flex items-center space-x-2">
              <AlertTriangle className="w-4 h-4 flex-shrink-0" />
              <span>{installError}</span>
            </div>
          )}

          {/* Clean 2-Step Subsystem Setup Overview */}
          <div className="p-4 rounded-xl bg-zinc-950 border border-zinc-800 space-y-4">
            <div className="flex items-center justify-between">
              <div className="flex items-center space-x-2.5">
                <Layers className="w-4 h-4 text-indigo-400" />
                <span className="font-semibold text-zinc-200">Subsystem Setup Overview</span>
              </div>
              <button
                onClick={onRefreshStatus}
                className="text-xs text-zinc-400 hover:text-indigo-400 flex items-center space-x-1.5 transition-colors"
                title="Refresh Status"
              >
                <RefreshCw className="w-3.5 h-3.5" />
                <span>Check Status</span>
              </button>
            </div>

            {/* STEP 1 CARD: WSL2 Core Engine */}
            <div className={`p-4 rounded-xl border transition-all ${
              isStep1Ready
                ? 'bg-zinc-900/50 border-emerald-500/30'
                : wslStatus?.isOutdated || state === 'WSL_V1_ONLY'
                ? 'bg-zinc-900/50 border-amber-500/40'
                : 'bg-zinc-900/50 border-zinc-800'
            }`}>
              <div className="flex items-start justify-between gap-4">
                <div className="flex items-start space-x-3">
                  <div className={`w-7 h-7 rounded-lg flex items-center justify-center font-bold text-xs flex-shrink-0 mt-0.5 ${
                    isStep1Ready
                      ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30'
                      : 'bg-indigo-500/20 text-indigo-400 border border-indigo-500/30'
                  }`}>
                    1
                  </div>
                  <div>
                    <div className="font-semibold text-xs text-zinc-200 flex items-center space-x-2 flex-wrap gap-y-1">
                      <span>Windows Subsystem for Linux (WSL2)</span>
                      {isStep1Ready ? (
                        <span className="px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/30 text-[10px] font-medium flex items-center space-x-1">
                          <Check className="w-3 h-3" />
                          <span>Ready</span>
                        </span>
                      ) : wslStatus?.isOutdated ? (
                        <span className="px-2 py-0.5 rounded-full bg-amber-500/10 text-amber-300 border border-amber-500/30 text-[10px] font-medium flex items-center space-x-1">
                          <AlertTriangle className="w-3 h-3" />
                          <span>Update Recommended</span>
                        </span>
                      ) : state === 'WSL_V1_ONLY' ? (
                        <span className="px-2 py-0.5 rounded-full bg-amber-500/10 text-amber-300 border border-amber-500/30 text-[10px] font-medium">
                          WSL1 (Upgrade Needed)
                        </span>
                      ) : (
                        <span className="px-2 py-0.5 rounded-full bg-zinc-800 text-zinc-400 border border-zinc-700 text-[10px] font-medium">
                          Not Enabled
                        </span>
                      )}
                    </div>
                    <p className="text-[11px] text-zinc-400 mt-1 leading-relaxed">
                      {isStep1Ready
                        ? `WSL2 Core Engine active (${wslStatus?.wslVersion ? `WSL v${wslStatus.wslVersion}` : 'WSL2 Default'}). Linux ext4 RocksDB consensus storage enabled.`
                        : wslStatus?.isOutdated
                        ? `Installed WSL version (${wslStatus?.wslVersion ? `v${wslStatus.wslVersion}` : 'Inbox / Legacy'}) requires update to support the modern Ubuntu-22.04 LTS catalog.`
                        : state === 'WSL_V1_ONLY'
                        ? 'WSL version 1 is active. Sirius requires WSL Version 2 for Linux ext4 RocksDB performance.'
                        : 'Virtual Machine Platform and Windows Subsystem for Linux optional features must be enabled.'}
                    </p>
                  </div>
                </div>

                <div className="flex items-center space-x-2 flex-shrink-0">
                  {state === 'WSL_NOT_INSTALLED' && (
                    <button
                      onClick={handleEnableWSL}
                      disabled={isInstalling}
                      className="px-3 py-1.5 bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white rounded-lg text-xs font-semibold shadow transition-all flex items-center space-x-1.5"
                    >
                      <Cpu className={`w-3.5 h-3.5 ${isInstalling ? 'animate-spin' : ''}`} />
                      <span>Enable WSL2</span>
                    </button>
                  )}

                  {(wslStatus?.isOutdated || state === 'WSL_V1_ONLY') && (
                    <button
                      onClick={handleUpdateWSL}
                      disabled={isUpdatingWSL}
                      className="px-3 py-1.5 bg-amber-600 hover:bg-amber-500 disabled:opacity-50 text-white rounded-lg text-xs font-semibold shadow transition-all flex items-center space-x-1.5"
                    >
                      <RefreshCw className={`w-3.5 h-3.5 ${isUpdatingWSL ? 'animate-spin' : ''}`} />
                      <span>Update WSL Subsystem</span>
                    </button>
                  )}

                  {isStep1Ready && (
                    <button
                      onClick={handleUpdateWSL}
                      disabled={isUpdatingWSL}
                      title="Re-check or force update WSL kernel"
                      className="px-2.5 py-1 bg-zinc-800 hover:bg-zinc-700 text-zinc-300 rounded-lg text-[11px] font-medium border border-zinc-700 transition-colors flex items-center space-x-1"
                    >
                      <RefreshCw className={`w-3 h-3 ${isUpdatingWSL ? 'animate-spin' : ''}`} />
                      <span>Check / Update</span>
                    </button>
                  )}
                </div>
              </div>
            </div>

            {/* STEP 2 CARD: Sirius Linux Subsystem (Ubuntu 22.04 LTS) */}
            <div className={`p-4 rounded-xl border transition-all ${
              isStep2Ready
                ? 'bg-zinc-900/50 border-emerald-500/30'
                : distroInstalling
                ? 'bg-zinc-900/50 border-indigo-500/40'
                : 'bg-zinc-900/50 border-zinc-800'
            }`}>
              <div className="flex items-start justify-between gap-4">
                <div className="flex items-start space-x-3">
                  <div className={`w-7 h-7 rounded-lg flex items-center justify-center font-bold text-xs flex-shrink-0 mt-0.5 ${
                    isStep2Ready
                      ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30'
                      : 'bg-indigo-500/20 text-indigo-400 border border-indigo-500/30'
                  }`}>
                    2
                  </div>
                  <div>
                    <div className="font-semibold text-xs text-zinc-200 flex items-center space-x-2 flex-wrap gap-y-1">
                      <span>Sirius Linux Subsystem (Ubuntu 22.04 LTS)</span>
                      {isStep2Ready ? (
                        <span className="px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/30 text-[10px] font-medium flex items-center space-x-1">
                          <Check className="w-3 h-3" />
                          <span>Installed & Ready</span>
                        </span>
                      ) : distroInstalling ? (
                        <span className="px-2 py-0.5 rounded-full bg-indigo-500/10 text-indigo-400 border border-indigo-500/30 text-[10px] font-medium flex items-center space-x-1">
                          <RefreshCw className="w-3 h-3 animate-spin" />
                          <span>Installing...</span>
                        </span>
                      ) : !isStep1Ready ? (
                        <span className="px-2 py-0.5 rounded-full bg-zinc-800 text-zinc-500 border border-zinc-700 text-[10px] font-medium">
                          Waiting for Step 1
                        </span>
                      ) : (
                        <span className="px-2 py-0.5 rounded-full bg-amber-500/10 text-amber-300 border border-amber-500/30 text-[10px] font-medium">
                          Not Installed
                        </span>
                      )}
                    </div>
                    <p className="text-[11px] text-zinc-400 mt-1 leading-relaxed">
                      {isStep2Ready
                        ? `Distro: ${wslStatus?.distroName || 'Ubuntu-22.04'} (WSL2) • FastFinality runtime (libatomic1) verified.`
                        : distroInstalling
                        ? 'Downloading distribution package (~500 MB) and configuring libatomic1 in terminal window...'
                        : 'Runs the C++ Sirius Catapult engine ELF binary with RocksDB and libatomic1 dependencies.'}
                    </p>
                  </div>
                </div>

                <div className="flex items-center space-x-2 flex-shrink-0">
                  {distroInstalling ? (
                    <span className="px-3 py-1.5 bg-indigo-900/40 text-indigo-300 border border-indigo-500/30 rounded-lg text-xs font-semibold flex items-center space-x-1.5">
                      <RefreshCw className="w-3.5 h-3.5 animate-spin" />
                      <span>Installing...</span>
                    </span>
                  ) : isStep2Ready ? (
                    <div className="flex items-center space-x-1.5 text-emerald-400 text-xs font-semibold px-3 py-1.5 bg-emerald-950/30 border border-emerald-500/20 rounded-lg">
                      <CheckCircle2 className="w-3.5 h-3.5" />
                      <span>Configured</span>
                    </div>
                  ) : (
                    <button
                      onClick={handleSetupDistro}
                      disabled={isInstalling || !isStep1Ready}
                      className="px-3 py-1.5 bg-indigo-600 hover:bg-indigo-500 disabled:opacity-40 disabled:cursor-not-allowed text-white rounded-lg text-xs font-semibold shadow transition-all flex items-center space-x-1.5"
                    >
                      <Cpu className="w-3.5 h-3.5" />
                      <span>Install Ubuntu 22.04</span>
                    </button>
                  )}
                </div>
              </div>
            </div>

            {/* FULLY OPERATIONAL PROCEED BANNER */}
            {isFullyReady && (
              <div className="p-4 rounded-xl bg-gradient-to-r from-emerald-950/50 via-zinc-900 to-zinc-950 border border-emerald-500/40 flex items-center justify-between gap-4 animate-fadeIn">
                <div className="flex items-center space-x-3">
                  <div className="p-2 rounded-lg bg-emerald-500/20 text-emerald-400 flex-shrink-0">
                    <ShieldCheck className="w-6 h-6" />
                  </div>
                  <div>
                    <h4 className="text-xs font-bold text-emerald-300 uppercase tracking-wider">
                      Subsystem is Fully Operational
                    </h4>
                    <p className="text-xs text-zinc-300 mt-0.5">
                      WSL2 and Ubuntu 22.04 LTS (with libatomic1) are ready to start the Sirius Node engine.
                    </p>
                  </div>
                </div>
                <button
                  onClick={handleDismiss}
                  className="px-4 py-2 bg-emerald-600 hover:bg-emerald-500 text-white rounded-xl text-xs font-semibold shadow-lg shadow-emerald-600/30 transition-all flex items-center space-x-2 flex-shrink-0"
                >
                  <span>Proceed to Node Manager</span>
                  <ArrowRight className="w-4 h-4" />
                </button>
              </div>
            )}

            {/* Collapsible Install Log Drawer if present */}
            {wslStatus?.installLog && (
              <div className="pt-1">
                <button
                  onClick={() => setShowLogDetails(!showLogDetails)}
                  className="text-[11px] text-zinc-400 hover:text-zinc-200 flex items-center space-x-1 transition-colors"
                >
                  <FileText className="w-3 h-3" />
                  <span>{showLogDetails ? 'Hide' : 'View'} Recent Subsystem Log</span>
                  {showLogDetails ? <ChevronUp className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />}
                </button>
                {showLogDetails && (
                  <div className="mt-2 p-2.5 bg-zinc-950 rounded-lg border border-zinc-800 font-mono text-[10px] text-zinc-400 max-h-32 overflow-y-auto whitespace-pre-wrap">
                    {wslStatus.installLog}
                  </div>
                )}
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
            {typeof wslStatus?.rawStatus === 'string' && wslStatus.rawStatus.trim()
              ? `System: ${wslStatus.rawStatus.trim().split('\n')[0]}`
              : `Windows 10/11 Architecture${wslStatus?.wslVersion ? ` • WSL v${wslStatus.wslVersion}` : ''}`}
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

export const WSLSetupModal: React.FC<WSLSetupModalProps> = (props) => {
  if (!props.isOpen) return null;
  return (
    <ErrorBoundary fallbackTitle="WSL Subsystem Modal Error">
      <WSLSetupModalContent {...props} />
    </ErrorBoundary>
  );
};
