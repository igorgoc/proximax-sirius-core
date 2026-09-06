import React, { useState, useEffect } from 'react';
import {
  Wrench,
  RefreshCw,
  CheckCircle2,
  ShieldAlert,
  ShieldCheck,
  Lock,
  Eye,
  EyeOff,
  DownloadCloud,
  AlertTriangle,
  Trash2,
  StopCircle,
  FolderInput,
  HardDrive,
  Archive,
  Layers,
  Zap,
} from 'lucide-react';
import { SnapshotStatus, DataBackupStatus, StorageConvertStatus, UpdateInfo } from '../types';
import { DirectoryDropdown } from './DirectoryDropdown';

export const MaintenanceTab: React.FC = () => {
  const [resetting, setResetting] = useState(false);
  const [resetDone, setResetDone] = useState(false);

  // Fast-sync snapshot state (Remote vs Local)
  const [snapshotMode, setSnapshotMode] = useState<'remote' | 'local'>('remote');
  const [snapshotUrl, setSnapshotUrl] = useState('http://207.180.195.181/snapshot.tar.xz');
  const [localSnapshotPath, setLocalSnapshotPath] = useState('');
  const [snapshotStatus, setSnapshotStatus] = useState<SnapshotStatus | null>(null);
  const [cancellingSnapshot, setCancellingSnapshot] = useState(false);

  // Official updater state
  const [updateInfo, setUpdateInfo] = useState<UpdateInfo | null>(null);
  const [checkingUpdate, setCheckingUpdate] = useState(false);
  const [applyingUpdate, setApplyingUpdate] = useState(false);


  // Clean logs & cache state
  const [cleaning, setCleaning] = useState(false);
  const [cleanMessage, setCleanMessage] = useState<string | null>(null);

  // Watchdog state
  const [autoRecovery, setAutoRecovery] = useState(true);

  // Blockchain Data Backup state
  const [dataBackupSource, setDataBackupSource] = useState('./chainconfig/data');
  const [dataBackupTarget, setDataBackupTarget] = useState('');
  const [dataBackupFormat, setDataBackupFormat] = useState<'zst' | 'gz'>('zst');
  const [dataBackupStatus, setDataBackupStatus] = useState<DataBackupStatus | null>(null);
  const [startingDataBackup, setStartingDataBackup] = useState(false);
  const [cancellingDataBackup, setCancellingDataBackup] = useState(false);

  // Storage Conversion state
  const [convertSourcePath, setConvertSourcePath] = useState('./chainconfig/data');
  const [convertStatus, setConvertStatus] = useState<StorageConvertStatus | null>(null);
  const [startingConvert, setStartingConvert] = useState(false);
  const [cancellingConvert, setCancellingConvert] = useState(false);

  // Fetch live snapshot & data backup status periodically
  useEffect(() => {
    const checkStatus = async () => {
      try {
        const [snapRes, backupRes, convertRes, statusRes] = await Promise.all([
          fetch('/api/maintenance/snapshot/status'),
          fetch('/api/maintenance/data-backup/status'),
          fetch('/api/maintenance/storage-convert/status'),
          fetch('/api/status')
        ]);
        if (snapRes.ok) {
          const data: SnapshotStatus = await snapRes.json();
          setSnapshotStatus(data);
        }
        if (backupRes.ok) {
          const bData: DataBackupStatus = await backupRes.json();
          setDataBackupStatus(bData);
        }
        if (convertRes.ok) {
          const cData: StorageConvertStatus = await convertRes.json();
          setConvertStatus(cData);
        }
        if (statusRes.ok) {
          const sData = await statusRes.json();
          if (sData.updateInfo) setUpdateInfo(sData.updateInfo);
          if (typeof sData.autoRecovery === 'boolean') setAutoRecovery(sData.autoRecovery);
        }
      } catch (e) {
        // ignore network glitches
      }
    };

    checkStatus();
    const interval = setInterval(checkStatus, 1000);
    return () => clearInterval(interval);
  }, []);

  const handleResetChain = async () => {
    if (!window.confirm('Are you sure you want to reset the blockchain data to the genesis nemesis block? This will delete local sync state and restart synchronization from block 1.')) {
      return;
    }

    setResetting(true);
    setResetDone(false);

    try {
      const res = await fetch('/api/maintenance/reset', { method: 'POST' });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to reset chain');

      setResetDone(true);
      alert('Blockchain data successfully reset to genesis nemesis block. You can now start the node.');
    } catch (e: any) {
      alert(e.message);
    } finally {
      setResetting(false);
    }
  };

  const handleStartSnapshot = async () => {
    const isLocal = snapshotMode === 'local';
    const targetDesc = isLocal ? `local archive "${localSnapshotPath || 'specified path'}"` : `remote server "${snapshotUrl}"`;

    if (!window.confirm(`Restore blockchain data from ${targetDesc}? Existing blocks will be fast-forwarded. Continue?`)) {
      return;
    }

    try {
      const res = await fetch('/api/maintenance/snapshot', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          mode: snapshotMode,
          url: snapshotUrl.trim(),
          sourcePath: localSnapshotPath.trim(),
        }),
      });

      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to start snapshot process');
    } catch (e: any) {
      alert(e.message);
    }
  };

  const handleCancelSnapshot = async () => {
    if (!window.confirm('Are you sure you want to cancel the snapshot extraction? Any incomplete extraction will be halted.')) {
      return;
    }

    setCancellingSnapshot(true);
    try {
      const res = await fetch('/api/maintenance/snapshot/cancel', { method: 'POST' });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to cancel snapshot');
    } catch (e: any) {
      alert(e.message);
    } finally {
      setCancellingSnapshot(false);
    }
  };

  const handleCleanLogs = async () => {
    setCleaning(true);
    setCleanMessage(null);
    try {
      const res = await fetch('/api/maintenance/clean-logs', { method: 'POST' });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to clean logs');
      setCleanMessage(`Freed ${(data.freedMB || 0).toFixed(2)} MB of disk space by purging historical logs and temporary cache.`);
    } catch (e: any) {
      alert(e.message);
    } finally {
      setCleaning(false);
    }
  };

  const handleStartDataBackup = async () => {
    const formatLabel = dataBackupFormat === 'zst' ? '.tar.zst (Zstandard)' : '.tar.gz (Gzip)';
    if (!window.confirm(`Create a full ${formatLabel} compressed backup archive of blockchain data from "${dataBackupSource}" to "${dataBackupTarget || 'default directory'}"?`)) {
      return;
    }

    setStartingDataBackup(true);
    try {
      const res = await fetch('/api/maintenance/data-backup', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          sourcePath: dataBackupSource.trim(),
          targetPath: dataBackupTarget.trim(),
          format: dataBackupFormat,
        }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to start data backup');
    } catch (e: any) {
      alert(e.message);
    } finally {
      setStartingDataBackup(false);
    }
  };

  const handleCancelDataBackup = async () => {
    if (!window.confirm('Are you sure you want to cancel the ongoing blockchain data backup? Incomplete archive will be removed.')) {
      return;
    }

    setCancellingDataBackup(true);
    try {
      const res = await fetch('/api/maintenance/data-backup/cancel', { method: 'POST' });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to cancel data backup');
    } catch (e: any) {
      alert(e.message);
    } finally {
      setCancellingDataBackup(false);
    }
  };

  // Settings & statistics (.json) export/import
  const [restoringSettings, setRestoringSettings] = useState(false);
  const [settingsMessage, setSettingsMessage] = useState<string | null>(null);

  const handleExportSettings = async () => {
    try {
      const res = await fetch('/api/system/backup/export', { method: 'POST' });
      if (!res.ok) throw new Error('Settings export failed');
      const blob = await res.blob();
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `sirius-settings-${new Date().toISOString().slice(0, 10)}.json`;
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
      document.body.removeChild(a);
      setSettingsMessage('Settings and statistics exported successfully.');
    } catch (e: any) {
      alert(e.message);
    }
  };

  const handleImportSettings = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    if (!window.confirm(`Import settings from "${file.name}"? Note: This updates non-sensitive configuration parameters and stats only. It does not restore private keys or blockchain state.`)) {
      return;
    }

    setRestoringSettings(true);
    setSettingsMessage(null);
    try {
      const reader = new FileReader();
      reader.onload = async (event) => {
        try {
          const content = event.target?.result as string;
          const res = await fetch('/api/system/backup/restore', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: content,
          });
          const data = await res.json();
          if (!res.ok) throw new Error(data.error || 'Failed to restore settings');
          setSettingsMessage('Settings imported successfully. Please restart node to apply changes.');
        } catch (err: any) {
          alert(err.message);
        } finally {
          setRestoringSettings(false);
        }
      };
      reader.readAsText(file);
    } catch (err: any) {
      alert(err.message);
      setRestoringSettings(false);
    }
  };

  // Full Encrypted Disaster Recovery Package (.drpkg) state
  const [drMode, setDrMode] = useState<'dr' | 'settings'>('dr');
  const [drPassphrase, setDrPassphrase] = useState('');
  const [showDrPassphrase, setShowDrPassphrase] = useState(false);
  const [drExporting, setDrExporting] = useState(false);
  const [drRestoring, setDrRestoring] = useState(false);
  const [drMessage, setDrMessage] = useState<string | null>(null);
  const [drError, setDrError] = useState<string | null>(null);

  const handleExportDR = async () => {
    if (!drPassphrase || drPassphrase.length < 8) {
      alert('Passphrase must be at least 8 characters long to encrypt the disaster recovery package.');
      return;
    }
    setDrExporting(true);
    setDrError(null);
    setDrMessage(null);
    try {
      const res = await fetch('/api/system/recovery/export', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ passphrase: drPassphrase }),
      });
      if (!res.ok) {
        const errData = await res.json().catch(() => ({}));
        throw new Error(errData.error || 'Failed to export disaster recovery package');
      }
      const blob = await res.blob();
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `sirius-recovery-package-${new Date().toISOString().slice(0, 10)}.drpkg`;
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
      document.body.removeChild(a);
      setDrMessage('Full Disaster Recovery Package exported and encrypted with Argon2id + AES-256-GCM.');
      setDrPassphrase(''); // Clear plaintext passphrase from state
    } catch (err: any) {
      setDrError(err.message);
    } finally {
      setDrExporting(false);
    }
  };

  const handleImportDR = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    if (!drPassphrase || drPassphrase.length < 8) {
      alert('Please enter the decryption passphrase in the Encryption Passphrase field before selecting a recovery package file.');
      e.target.value = '';
      return;
    }

    if (!window.confirm(`Restore ALL 21 Catapult configurations, certificates, and harvesting keys from "${file.name}"? This will overwrite existing configuration on this machine.`)) {
      e.target.value = '';
      return;
    }

    setDrRestoring(true);
    setDrError(null);
    setDrMessage(null);
    const pass = drPassphrase;
    try {
      const reader = new FileReader();
      reader.onload = async (event) => {
        try {
          const rawContent = event.target?.result as string;
          let pkgObj: any;
          try {
            pkgObj = JSON.parse(rawContent);
          } catch {
            throw new Error('Invalid recovery package file (not valid JSON format)');
          }

          const res = await fetch('/api/system/recovery/restore', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
              passphrase: pass,
              package: pkgObj,
            }),
          });
          const data = await res.json();
          if (!res.ok) throw new Error(data.error || 'Failed to restore disaster recovery package');
          setDrMessage('Disaster recovery package decrypted and restored! Please restart node to apply.');
          setDrPassphrase('');
        } catch (err: any) {
          setDrError(err.message);
        } finally {
          setDrRestoring(false);
        }
      };
      reader.readAsText(file);
    } catch (err: any) {
      setDrError(err.message);
      setDrRestoring(false);
    }
  };

  const handleCheckUpdate = async () => {
    setCheckingUpdate(true);
    try {
      const res = await fetch('/api/maintenance/update/check');
      const data = await res.json();
      if (res.ok && data) {
        setUpdateInfo(data);
      }
    } catch (e) {
      // ignore
    } finally {
      setCheckingUpdate(false);
    }
  };

  const handleApplyUpdate = async () => {
    if (!window.confirm('Apply official network updates from GitHub? This will sync network seed peers and restart the node while preserving custom native patches.')) {
      return;
    }

    setApplyingUpdate(true);
    try {
      const res = await fetch('/api/maintenance/update/apply', { method: 'POST' });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to apply official update');
      alert('Official network configuration updated successfully.');
      handleCheckUpdate();
    } catch (e: any) {
      alert(e.message);
    } finally {
      setApplyingUpdate(false);
    }
  };

  const handleToggleWatchdog = async () => {
    try {
      const res = await fetch('/api/maintenance/auto-recovery/toggle', { method: 'POST' });
      const data = await res.json();
      if (res.ok) {
        setAutoRecovery(data.autoRecovery);
      }
    } catch (e) {
      // ignore
    }
  };

  const handleStartConvert = async () => {
    if (!window.confirm('Convert legacy loose block files into compact 4-file Bitcoin-style chunks? This will stop the node, pack blocks.dat / statements.dat / blocks.idx, and delete old loose files to free inodes.')) {
      return;
    }

    setStartingConvert(true);
    try {
      const res = await fetch('/api/maintenance/storage-convert', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ dataPath: convertSourcePath })
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to start storage migration');
    } catch (e: any) {
      alert(e.message);
    } finally {
      setStartingConvert(false);
    }
  };

  const handleCancelConvert = async () => {
    setCancellingConvert(true);
    try {
      await fetch('/api/maintenance/storage-convert/cancel', { method: 'POST' });
    } catch (e) {
      // ignore
    } finally {
      setCancellingConvert(false);
    }
  };

  const isSnapshotRunning = snapshotStatus?.stage === 'downloading' || snapshotStatus?.stage === 'extracting';
  const isDataBackupRunning = dataBackupStatus?.stage === 'backing_up';
  const isConvertRunning = convertStatus?.status === 'running';

  return (
    <div className="h-full flex flex-col justify-between p-3 sm:p-4 space-y-2.5 max-w-7xl mx-auto overflow-hidden">
      {/* Header Banner */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between pb-2 border-b border-[#262B34] gap-2 flex-shrink-0">
        <div className="flex items-center space-x-2.5">
          <div className="w-8 h-8 rounded-lg bg-[#181B20] border border-[#262B34] flex items-center justify-center flex-shrink-0">
            <Wrench className="w-4 h-4 text-blue-400" />
          </div>
          <div>
            <div className="flex items-center space-x-2">
              <h2 className="text-sm font-bold text-white tracking-wide uppercase">
                Maintenance & Synchronization Center
              </h2>
              <span className="px-1.5 py-0.5 rounded text-[10px] font-mono font-bold bg-emerald-950/40 text-emerald-400 border border-emerald-500/40">
                Native v1.9.7 (Patches 1 & 2 Active)
              </span>
            </div>
            <p className="text-xs text-slate-400 font-mono">
              High-speed streaming fast-sync, multi-threaded .tar.zst backup, protocol sync, and node self-healing watchdog.
            </p>
          </div>
        </div>

        {/* Watchdog Status Indicator */}
        <button
          onClick={handleToggleWatchdog}
          className={`px-3 py-1.5 rounded-lg text-xs font-semibold border transition-all flex items-center space-x-1.5 flex-shrink-0 ${
            autoRecovery
              ? 'bg-emerald-950/30 border-emerald-500/40 text-emerald-400 hover:bg-emerald-950/50'
              : 'bg-[#181B20] border-[#262B34] text-slate-400 hover:bg-[#262B34]'
          }`}
          title="Toggle Auto-Recovery Process Watchdog"
        >
          <span className={`w-2 h-2 rounded-full ${autoRecovery ? 'bg-emerald-400 shadow-[0_0_6px_#34d399]' : 'bg-slate-500'}`} />
          <span>Watchdog: {autoRecovery ? 'ACTIVE' : 'DISABLED'}</span>
        </button>
      </div>

      {/* Top Row: 3 Modular Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-2.5 flex-shrink-0">
        {/* Card 1: Official Protocol Updater */}
        <div className="operator-card rounded-xl p-3 flex flex-col justify-between space-y-2">
          <div className="space-y-1.5">
            <div className="flex items-center justify-between pb-1.5 border-b border-[#262B34]">
              <div className="flex items-center space-x-1.5">
                <DownloadCloud className="w-3.5 h-3.5 text-slate-400" />
                <h3 className="text-xs font-semibold uppercase tracking-wider text-white">
                  Official Protocol Updater
                </h3>
              </div>
              <span className="text-[10px] font-mono font-bold bg-blue-600/15 text-blue-400 px-1.5 py-0.5 rounded border border-blue-500/30">
                {updateInfo?.currentVersion || 'v1.9.7'}
              </span>
            </div>
            <p className="text-[11px] text-slate-400 leading-tight">
              Synchronizes official seeds, peers, and network configurations from GitHub while preserving local C++ native performance patches.
            </p>
            <div className="p-2 bg-[#0F1115] rounded-lg border border-[#262B34] text-[11px] flex items-center justify-between font-mono">
              <span className="text-slate-400">Release Status:</span>
              <span className={updateInfo?.hasUpdate ? 'text-amber-400 font-bold' : 'text-emerald-400 font-bold'}>
                {updateInfo?.hasUpdate ? `Update ${updateInfo.latestVersion} Available` : `Up to Date (${updateInfo?.currentVersion || 'v1.9.7'})`}
              </span>
            </div>
          </div>

          <div className="pt-1.5 border-t border-[#262B34] flex items-center justify-between gap-2">
            <button
              onClick={handleCheckUpdate}
              disabled={checkingUpdate}
              className="px-2.5 py-1 bg-[#181B20] hover:bg-[#262B34] text-slate-300 hover:text-white rounded-lg text-xs font-semibold border border-[#262B34] transition-colors flex items-center space-x-1"
            >
              <RefreshCw className={`w-3 h-3 ${checkingUpdate ? 'animate-spin' : ''}`} />
              <span>{checkingUpdate ? 'Checking...' : 'Check GitHub'}</span>
            </button>
            <button
              onClick={handleApplyUpdate}
              disabled={applyingUpdate}
              className="px-3 py-1 bg-[#2563eb] hover:bg-[#1d4ed8] text-white rounded-lg text-xs font-semibold shadow-xs transition-all flex items-center space-x-1"
            >
              <DownloadCloud className="w-3 h-3" />
              <span>{applyingUpdate ? 'Updating...' : 'Sync Configs'}</span>
            </button>
          </div>
        </div>

        {/* Card 2: Disaster Recovery Package (Encrypted) & Settings Export */}
        <div className="operator-card rounded-xl p-3 flex flex-col justify-between space-y-2">
          <div className="space-y-1.5">
            <div className="flex items-center justify-between pb-1.5 border-b border-[#262B34]">
              <div className="flex items-center space-x-1.5">
                {drMode === 'dr' ? (
                  <ShieldCheck className="w-3.5 h-3.5 text-emerald-400" />
                ) : (
                  <FolderInput className="w-3.5 h-3.5 text-slate-400" />
                )}
                <h3 className="text-xs font-semibold uppercase tracking-wider text-white">
                  {drMode === 'dr' ? 'Disaster Recovery' : 'Node Settings'}
                </h3>
              </div>
              <div className="flex items-center space-x-1">
                <button
                  type="button"
                  onClick={() => setDrMode('dr')}
                  className={`px-1.5 py-0.5 rounded text-[10px] font-mono font-bold transition-all ${
                    drMode === 'dr' ? 'bg-emerald-600 text-white' : 'bg-[#181B20] text-slate-400 hover:text-slate-200'
                  }`}
                >
                  Full DR (Encrypted)
                </button>
                <button
                  type="button"
                  onClick={() => setDrMode('settings')}
                  className={`px-1.5 py-0.5 rounded text-[10px] font-mono font-bold transition-all ${
                    drMode === 'settings' ? 'bg-amber-600 text-white' : 'bg-[#181B20] text-slate-400 hover:text-slate-200'
                  }`}
                >
                  Settings Only
                </button>
              </div>
            </div>

            {drMode === 'dr' ? (
              <>
                <p className="text-[11px] text-slate-400 leading-tight">
                  Full disaster recovery archive: all 21 Catapult configs, TLS certificates, and unredacted keys encrypted with Argon2id + AES-256-GCM.
                </p>

                <div className="space-y-1 pt-0.5">
                  <div className="flex items-center justify-between text-[11px] text-slate-300">
                    <span className="flex items-center space-x-1">
                      <Lock className="w-3 h-3 text-emerald-400" />
                      <span>Encryption Passphrase:</span>
                    </span>
                    <span className="text-[9.5px] text-slate-500 font-mono">Min 8 chars</span>
                  </div>
                  <div className="relative">
                    <input
                      type={showDrPassphrase ? 'text' : 'password'}
                      value={drPassphrase}
                      onChange={(e) => setDrPassphrase(e.target.value)}
                      placeholder="Enter strong passphrase..."
                      className="w-full bg-[#0F1115] border border-[#262B34] focus:border-emerald-500 rounded-lg px-2.5 py-1 text-xs text-white placeholder-slate-500 pr-8"
                    />
                    <button
                      type="button"
                      onClick={() => setShowDrPassphrase(!showDrPassphrase)}
                      className="absolute right-2 top-1.5 text-slate-500 hover:text-slate-300"
                    >
                      {showDrPassphrase ? <EyeOff className="w-3.5 h-3.5" /> : <Eye className="w-3.5 h-3.5" />}
                    </button>
                  </div>
                </div>

                {drMessage && (
                  <div className="p-2 bg-emerald-950/30 border border-emerald-500/30 text-emerald-400 rounded-lg text-[11px] flex items-center space-x-1 truncate">
                    <CheckCircle2 className="w-3 h-3 flex-shrink-0" />
                    <span className="truncate">{drMessage}</span>
                  </div>
                )}
                {drError && (
                  <div className="p-2 bg-red-950/30 border border-red-500/30 text-red-400 rounded-lg text-[11px] flex items-center space-x-1 truncate">
                    <AlertTriangle className="w-3 h-3 flex-shrink-0" />
                    <span className="truncate">{drError}</span>
                  </div>
                )}
              </>
            ) : (
              <>
                <p className="text-[11px] text-slate-400 leading-tight">
                  Export portable non-sensitive settings (friendly name, ports, custom peer lists, and harvest stats).
                </p>

                {/* Explicit Disaster Recovery Limitation Warning */}
                <div className="p-2 bg-amber-950/20 border border-amber-500/30 rounded-lg text-[10.5px] leading-relaxed text-amber-200/90 flex items-start space-x-1.5">
                  <AlertTriangle className="w-3.5 h-3.5 text-amber-400 flex-shrink-0 mt-0.5" />
                  <span>
                    <strong className="text-amber-300 font-semibold">Not a Disaster Recovery Backup:</strong> Private keys (bootKey/harvestKey), blockchain data, certificates, and Catapult templates are <strong className="text-amber-200">not included</strong>.
                  </span>
                </div>

                {settingsMessage ? (
                  <div className="p-2 bg-emerald-950/30 border border-emerald-500/30 text-emerald-400 rounded-lg text-[11px] flex items-center space-x-1 truncate">
                    <CheckCircle2 className="w-3 h-3 flex-shrink-0" />
                    <span className="truncate">{settingsMessage}</span>
                  </div>
                ) : null}
              </>
            )}
          </div>

          <div className="pt-1.5 border-t border-[#262B34] flex items-center justify-between gap-2">
            {drMode === 'dr' ? (
              <>
                <label className="px-2.5 py-1 bg-[#181B20] hover:bg-[#262B34] text-slate-300 hover:text-white rounded-lg text-xs font-semibold border border-[#262B34] transition-colors cursor-pointer flex items-center space-x-1">
                  <FolderInput className="w-3 h-3" />
                  <span>{drRestoring ? 'Restoring...' : 'Restore Package'}</span>
                  <input type="file" accept=".drpkg,.json" onChange={handleImportDR} className="hidden" disabled={drRestoring} />
                </label>
                <button
                  onClick={handleExportDR}
                  disabled={drExporting}
                  className="px-3 py-1 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg text-xs font-semibold shadow-xs border border-emerald-500/30 transition-all flex items-center space-x-1"
                >
                  <DownloadCloud className="w-3 h-3" />
                  <span>{drExporting ? 'Encrypting...' : 'Export Package'}</span>
                </button>
              </>
            ) : (
              <>
                <label className="px-2.5 py-1 bg-[#181B20] hover:bg-[#262B34] text-slate-300 hover:text-white rounded-lg text-xs font-semibold border border-[#262B34] transition-colors cursor-pointer flex items-center space-x-1">
                  <FolderInput className="w-3 h-3" />
                  <span>{restoringSettings ? 'Importing...' : 'Import Settings'}</span>
                  <input type="file" accept=".json" onChange={handleImportSettings} className="hidden" disabled={restoringSettings} />
                </label>
                <button
                  onClick={handleExportSettings}
                  className="px-3 py-1 bg-[#262B34] hover:bg-[#323844] text-white rounded-lg text-xs font-semibold shadow-xs border border-[#3A4250] transition-all flex items-center space-x-1"
                >
                  <DownloadCloud className="w-3 h-3" />
                  <span>Export Settings</span>
                </button>
              </>
            )}
          </div>
        </div>

        {/* Card 3: Fast-Sync Snapshot Stream (Remote vs Local Archive) */}
        <div className="operator-card rounded-xl p-3 flex flex-col justify-between space-y-2">
          <div className="space-y-1.5">
            <div className="flex items-center justify-between pb-1.5 border-b border-[#262B34]">
              <div className="flex items-center space-x-1.5">
                <DownloadCloud className="w-3.5 h-3.5 text-slate-400" />
                <h3 className="text-xs font-semibold uppercase tracking-wider text-white">
                  Fast-Sync Snapshot
                </h3>
              </div>
              <div className="flex items-center space-x-1">
                <button
                  type="button"
                  onClick={() => setSnapshotMode('remote')}
                  className={`px-1.5 py-0.5 rounded text-[10px] font-mono font-bold transition-all ${
                    snapshotMode === 'remote' ? 'bg-blue-600 text-white' : 'bg-[#181B20] text-slate-400 hover:text-slate-200'
                  }`}
                >
                  Remote Server
                </button>
                <button
                  type="button"
                  onClick={() => setSnapshotMode('local')}
                  className={`px-1.5 py-0.5 rounded text-[10px] font-mono font-bold transition-all ${
                    snapshotMode === 'local' ? 'bg-emerald-600 text-white' : 'bg-[#181B20] text-slate-400 hover:text-slate-200'
                  }`}
                >
                  Local Backup
                </button>
              </div>
            </div>
            <p className="text-[11px] text-slate-400 leading-tight">
              {snapshotMode === 'remote'
                ? 'Direct streaming decompression from official mainnet snapshot server.'
                : 'Restores locally created backup archives (.tar.zst, .tar.xz, .tar.gz) using fast multi-threaded codecs.'}
            </p>
            <div>
              {snapshotMode === 'remote' ? (
                <input
                  type="text"
                  value={snapshotUrl}
                  onChange={(e) => setSnapshotUrl(e.target.value)}
                  disabled={isSnapshotRunning}
                  placeholder="http://207.180.195.181/snapshot.tar.xz"
                  className="w-full px-2.5 py-1 bg-[#0F1115] border border-[#262B34] rounded-lg text-[11px] font-mono text-slate-100 placeholder-slate-500 focus:outline-hidden focus:border-blue-500 disabled:opacity-50"
                />
              ) : (
                <DirectoryDropdown
                  mode="file"
                  value={localSnapshotPath}
                  onChange={setLocalSnapshotPath}
                  placeholder="~/sirius-data-backup-*.tar.zst"
                  prompt="Select Sirius Snapshot Archive"
                />
              )}
            </div>
          </div>

          <div className="pt-1.5 border-t border-[#262B34] flex items-center justify-between">
            <span className="text-[10px] text-slate-400 font-mono truncate mr-2">
              {snapshotStatus?.stage && snapshotStatus.stage !== 'idle' ? snapshotStatus.stage.toUpperCase() : 'Ready'}
            </span>
            {!isSnapshotRunning ? (
              <button
                onClick={handleStartSnapshot}
                className="px-3 py-1 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-semibold shadow-xs border border-blue-500/30 transition-all flex items-center space-x-1"
              >
                <DownloadCloud className="w-3 h-3" />
                <span>{snapshotMode === 'remote' ? 'Start Remote Sync' : 'Restore Local File'}</span>
              </button>
            ) : (
              <button
                onClick={handleCancelSnapshot}
                disabled={cancellingSnapshot}
                className="px-3 py-1 bg-rose-600 hover:bg-rose-500 text-white rounded-lg text-xs font-semibold shadow-xs transition-all flex items-center space-x-1"
              >
                <StopCircle className="w-3 h-3" />
                <span>Cancel Sync</span>
              </button>
            )}
          </div>
        </div>
      </div>

      {/* Bottom Section: 2 Columns (Left: Blockchain Backup & Chunk Migration; Right: Maintenance Utilities) */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-2.5 flex-1 items-stretch">
        {/* Card 4: Blockchain Data Backup */}
        <div className="operator-card rounded-xl p-3.5 flex flex-col justify-between space-y-2.5">
          <div className="space-y-2">
            <div className="flex items-center justify-between pb-1.5 border-b border-[#262B34]">
              <div className="flex items-center space-x-2">
                <Archive className="w-4 h-4 text-emerald-400" />
                <h3 className="text-xs font-semibold uppercase tracking-wider text-white">
                  Blockchain Data Backup (Full Archive)
                </h3>
              </div>
              {dataBackupStatus && dataBackupStatus.stage !== 'idle' && (
                <span className={`px-2 py-0.5 rounded font-mono text-[10px] font-bold uppercase border ${
                  dataBackupStatus.stage === 'backing_up'
                    ? 'bg-amber-950/30 text-amber-400 border-amber-500/40'
                    : dataBackupStatus.stage === 'complete'
                    ? 'bg-emerald-950/30 text-emerald-400 border-emerald-500/40'
                    : 'bg-blue-600/15 text-blue-400 border-blue-500/30'
                }`}>
                  {dataBackupStatus.stage === 'backing_up' ? 'BACKING UP' : dataBackupStatus.stage}
                </span>
              )}
            </div>

            <p className="text-xs text-slate-400 leading-tight">
              Create a full point-in-time compressed archive of all block directories (<code className="font-mono text-emerald-300">00000/</code>, RocksDB state cache, index) directly to internal or external storage.
            </p>

            {/* Format Selection with Clear Explanation */}
            <div className="p-2 rounded-lg bg-[#0F1115] border border-[#262B34] space-y-1.5">
              <div className="flex items-center justify-between">
                <span className="text-xs font-semibold text-slate-300">Compression Format:</span>
                <div className="flex items-center space-x-1.5">
                  <button
                    type="button"
                    onClick={() => setDataBackupFormat('zst')}
                    className={`px-2.5 py-0.5 rounded text-[11px] font-mono font-bold transition-all ${
                      dataBackupFormat === 'zst'
                        ? 'bg-emerald-600 text-white shadow-xs'
                        : 'bg-[#181B20] text-slate-400 hover:text-slate-200 border border-[#262B34]'
                    }`}
                  >
                    .tar.zst (Zstandard)
                  </button>
                  <button
                    type="button"
                    onClick={() => setDataBackupFormat('gz')}
                    className={`px-2.5 py-0.5 rounded text-[11px] font-mono font-bold transition-all ${
                      dataBackupFormat === 'gz'
                        ? 'bg-blue-600 text-white shadow-xs'
                        : 'bg-[#181B20] text-slate-400 hover:text-slate-200 border border-[#262B34]'
                    }`}
                  >
                    .tar.gz (Gzip)
                  </button>
                </div>
              </div>
              <p className="text-[11px] text-slate-400 leading-relaxed font-sans">
                {dataBackupFormat === 'zst' ? (
                  <>
                    <strong className="text-emerald-400">Recommended (Ultra-Fast):</strong> Multi-threaded Zstandard uses all CPU cores for 5x–10x faster backup.
                  </>
                ) : (
                  <>
                    <strong className="text-blue-400">Standard Compatibility:</strong> Universal Gzip archive compatible with older legacy tools.
                  </>
                )}
              </p>
            </div>

            {/* Roomy Directory Selectors */}
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 mt-1">
              <DirectoryDropdown label="Source (data.path)" value={dataBackupSource} onChange={setDataBackupSource} placeholder="./chainconfig/data" prompt="Select Backup Source Directory" />
              <DirectoryDropdown label="Destination Folder" value={dataBackupTarget} onChange={setDataBackupTarget} placeholder="/Volumes/SSD/backups" prompt="Select Backup Destination Directory" />
            </div>

            {/* Live Progress or Status Message */}
            {dataBackupStatus?.stage === 'backing_up' && (
              <div className="p-2 bg-[#0F1115] rounded-lg border border-amber-500/30 space-y-1 mt-1">
                <div className="flex justify-between text-[11px] font-mono">
                  <span className="text-amber-400 font-bold">{dataBackupStatus.percentage ? `${dataBackupStatus.percentage.toFixed(1)}%` : 'Archiving...'}</span>
                  <span className="text-slate-400 truncate max-w-[280px]">{dataBackupStatus.currentFile || 'Scanning files...'}</span>
                </div>
                <div className="w-full bg-[#181B20] h-1.5 rounded-full overflow-hidden">
                  <div
                    className="bg-gradient-to-r from-amber-500 to-emerald-400 h-full rounded-full transition-all duration-300"
                    style={{ width: `${Math.max(5, dataBackupStatus.percentage || 0)}%` }}
                  />
                </div>
                <p className="text-[10px] text-slate-400 truncate">{dataBackupStatus.message}</p>
              </div>
            )}

            {dataBackupStatus?.stage === 'complete' && (
              <div className="p-2 bg-emerald-950/30 border border-emerald-500/30 text-emerald-400 rounded-lg text-xs flex items-center space-x-1.5 mt-1">
                <CheckCircle2 className="w-3.5 h-3.5 flex-shrink-0" />
                <span className="truncate">{dataBackupStatus.message}</span>
              </div>
            )}

            {dataBackupStatus?.stage === 'error' && (
              <div className="p-2 bg-rose-950/30 border border-rose-500/30 text-rose-400 rounded-lg text-xs flex items-center space-x-1.5 mt-1">
                <AlertTriangle className="w-3.5 h-3.5 flex-shrink-0" />
                <span className="truncate">{dataBackupStatus.errorMessage || dataBackupStatus.message}</span>
              </div>
            )}
          </div>

          <div className="pt-2 border-t border-[#262B34] flex items-center justify-between">
            <span className="text-[11px] text-slate-400 font-mono truncate mr-2">
              {dataBackupStatus?.stage === 'backing_up' ? 'Archiving data.path...' : `Selected Format: .tar.${dataBackupFormat}`}
            </span>
            {!isDataBackupRunning ? (
              <button
                onClick={handleStartDataBackup}
                disabled={startingDataBackup}
                className="px-3.5 py-1.5 bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white rounded-lg text-xs font-semibold shadow-xs transition-all flex items-center space-x-1.5"
              >
                <Archive className="w-3.5 h-3.5" />
                <span>{startingDataBackup ? 'Starting...' : 'Create Data Backup'}</span>
              </button>
            ) : (
              <button
                onClick={handleCancelDataBackup}
                disabled={cancellingDataBackup}
                className="px-3.5 py-1.5 bg-rose-600 hover:bg-rose-500 text-white rounded-lg text-xs font-semibold shadow-xs transition-all flex items-center space-x-1.5"
              >
                <StopCircle className="w-3.5 h-3.5" />
                <span>{cancellingDataBackup ? 'Cancelling...' : 'Cancel Backup'}</span>
              </button>
            )}
          </div>
        </div>

        {/* Card 5: Migrate Legacy Storage to Chunked Format */}
        <div className="operator-card rounded-xl p-3.5 flex flex-col justify-between space-y-2.5">
          <div className="space-y-2">
            <div className="flex items-center justify-between pb-1.5 border-b border-[#262B34]">
              <div className="flex items-center space-x-2">
                <Layers className="w-4 h-4 text-slate-400" />
                <h3 className="text-xs font-semibold uppercase tracking-wider text-white">
                  Migrate Legacy Storage to Chunked Format
                </h3>
              </div>
              {convertStatus && convertStatus.status !== 'idle' && (
                <span className={`px-2 py-0.5 rounded font-mono text-[10px] font-bold uppercase border ${
                  convertStatus.status === 'running'
                    ? 'bg-amber-950/30 text-amber-400 border-amber-500/40'
                    : convertStatus.status === 'completed'
                    ? 'bg-emerald-950/30 text-emerald-400 border-emerald-500/40'
                    : 'bg-rose-950/30 text-rose-400 border-rose-500/40'
                }`}>
                  {convertStatus.status === 'running' ? 'CONVERTING' : convertStatus.status}
                </span>
              )}
            </div>

            <p className="text-xs text-slate-400 leading-tight">
              Packs 27.6M individual legacy <code className="font-mono text-blue-400">.dat</code> / <code className="font-mono text-blue-400">.stmt</code> files into compact 4-file chunks (<code className="font-mono text-emerald-300">blocks.dat</code>, <code className="font-mono text-emerald-300">statements.dat</code>, <code className="font-mono text-emerald-300">blocks.idx</code>) per 65k-block folder. Saves millions of filesystem inodes instantly.
            </p>

            <div className="mt-1">
              <DirectoryDropdown label="Blockchain Data Path" value={convertSourcePath} onChange={setConvertSourcePath} placeholder="./chainconfig/data" prompt="Select Blockchain Data Directory" />
            </div>

            {/* Live Progress Bar */}
            {convertStatus?.status === 'running' && (
              <div className="p-2 bg-[#0F1115] rounded-lg border border-amber-500/30 space-y-1 mt-1">
                <div className="flex justify-between text-[11px] font-mono">
                  <span className="text-amber-400 font-bold">{convertStatus.percent}% ({convertStatus.convertedBlocks.toLocaleString()} blocks)</span>
                  <span className="text-slate-400 truncate max-w-[200px]">Folder: {convertStatus.currentDir}</span>
                </div>
                <div className="w-full bg-[#181B20] h-1.5 rounded-full overflow-hidden">
                  <div
                    className="bg-gradient-to-r from-amber-500 via-blue-500 to-emerald-400 h-full rounded-full transition-all duration-300"
                    style={{ width: `${Math.max(5, convertStatus.percent)}%` }}
                  />
                </div>
                <div className="flex justify-between text-[10px] text-slate-400 font-mono">
                  <span>{convertStatus.message}</span>
                  <span className="text-rose-400">Deleted: {convertStatus.deletedFiles.toLocaleString()} files</span>
                </div>
              </div>
            )}

            {convertStatus?.status === 'completed' && (
              <div className="p-2 bg-emerald-950/30 border border-emerald-500/30 text-emerald-400 rounded-lg text-xs flex items-center space-x-1.5 mt-1">
                <CheckCircle2 className="w-3.5 h-3.5 flex-shrink-0" />
                <span className="truncate">{convertStatus.message}</span>
              </div>
            )}

            {convertStatus?.status === 'failed' && (
              <div className="p-2 bg-rose-950/30 border border-rose-500/30 text-rose-400 rounded-lg text-xs flex items-center space-x-1.5 mt-1">
                <AlertTriangle className="w-3.5 h-3.5 flex-shrink-0" />
                <span className="truncate">{convertStatus.error || convertStatus.message}</span>
              </div>
            )}
          </div>

          <div className="pt-2 border-t border-[#262B34] flex items-center justify-between">
            <span className="text-[11px] text-slate-400 font-mono truncate mr-2">
              {convertStatus?.status === 'running' ? 'Migrating files in background...' : 'Zero-loss in-place consolidation'}
            </span>
            {!isConvertRunning ? (
              <button
                onClick={handleStartConvert}
                disabled={startingConvert}
                className="px-3.5 py-1.5 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded-lg text-xs font-semibold shadow-xs border border-blue-500/30 transition-all flex items-center space-x-1.5"
              >
                <Zap className="w-3.5 h-3.5" />
                <span>{startingConvert ? 'Starting...' : 'Start Migration'}</span>
              </button>
            ) : (
              <button
                onClick={handleCancelConvert}
                disabled={cancellingConvert}
                className="px-3.5 py-1.5 bg-rose-600 hover:bg-rose-500 text-white rounded-lg text-xs font-semibold shadow-xs transition-all flex items-center space-x-1.5"
              >
                <StopCircle className="w-3.5 h-3.5" />
                <span>{cancellingConvert ? 'Cancelling...' : 'Cancel Migration'}</span>
              </button>
            )}
          </div>
        </div>
      </div>

      {/* Footer Utilities (Clean Logs & Nemesis Reset) */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5 pt-1">
        {/* Utility 1: Clean Cache & Logs */}
        <div className="operator-card rounded-xl p-3 flex items-center justify-between space-x-3">
          <div className="space-y-0.5">
            <div className="flex items-center space-x-1.5">
              <Trash2 className="w-3.5 h-3.5 text-amber-400" />
              <h3 className="text-xs font-semibold uppercase tracking-wider text-white">Clean Cache & Logs</h3>
            </div>
            <p className="text-[11px] text-slate-400">Purges historical rotated log files and clears stale server.lock handles.</p>
            {cleanMessage && <span className="text-[10px] text-emerald-400 font-mono block truncate">{cleanMessage}</span>}
          </div>
          <button
            onClick={handleCleanLogs}
            disabled={cleaning}
            className="px-3 py-1.5 bg-amber-600 hover:bg-amber-500 disabled:opacity-50 text-white rounded-lg text-xs font-semibold shadow-xs transition-all flex items-center space-x-1 flex-shrink-0"
          >
            <Trash2 className="w-3 h-3" />
            <span>{cleaning ? 'Cleaning...' : 'Purge Logs'}</span>
          </button>
        </div>

        {/* Utility 2: Nemesis Reset */}
        <div className="operator-card rounded-xl p-3 flex items-center justify-between space-x-3">
          <div className="space-y-0.5">
            <div className="flex items-center space-x-1.5">
              <ShieldAlert className="w-3.5 h-3.5 text-rose-400" />
              <h3 className="text-xs font-semibold uppercase tracking-wider text-white">Nemesis Reset (Block 1)</h3>
            </div>
            <p className="text-[11px] text-slate-400">Preserves Nemesis genesis (00001.dat) while wiping local state to re-sync from Block 1.</p>
            {resetDone && <span className="text-[10px] text-emerald-400 font-mono block">Reset complete. Ready to start.</span>}
          </div>
          <button
            onClick={handleResetChain}
            disabled={resetting}
            className="px-3 py-1.5 bg-rose-600 hover:bg-rose-500 disabled:opacity-50 text-white rounded-lg text-xs font-semibold shadow-xs transition-all flex items-center space-x-1 flex-shrink-0"
          >
            <RefreshCw className={`w-3 h-3 ${resetting ? 'animate-spin' : ''}`} />
            <span>{resetting ? 'Resetting...' : 'Reset Chain'}</span>
          </button>
        </div>
      </div>
    </div>
  );
};
