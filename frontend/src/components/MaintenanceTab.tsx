import React, { useState, useEffect, useCallback } from 'react';
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
  ArrowUpCircle,
  Database,
  Radio,
  Sliders,
  Check,
  Activity,
  Key,
  RotateCcw,
} from 'lucide-react';
import {
  SnapshotStatus,
  DataBackupStatus,
  StorageConvertStatus,
  UpdateInfo,
  NodeMetrics,
  SnapshotManagerStatus,
  LogStats,
} from '../types';
import { DirectoryDropdown } from './DirectoryDropdown';
import { ConfirmDestructiveModal } from './ConfirmDestructiveModal';
import { SyncConfigsModal } from './SyncConfigsModal';
import { loadSnapshotPreferences, SnapshotPreferences } from './SnapshotSubTab';

interface MaintenanceTabProps {
  onOpenSettings?: (subtab?: string) => void;
}

export const MaintenanceTab: React.FC<MaintenanceTabProps> = ({ onOpenSettings }) => {
  // Sub-Tab Navigation State: 'snapshots' | 'system'
  const [activeSubTab, setActiveSubTab] = useState<'snapshots' | 'system'>('snapshots');

  // 1. Single Source of Truth: Saved Preferences from Settings -> Snapshots
  const [prefs, setPrefs] = useState<SnapshotPreferences>(() => loadSnapshotPreferences());

  // Track session overrides per field (this run only - does NOT write back to Settings)
  const [isOverridden, setIsOverridden] = useState<{
    backupSource: boolean;
    backupTarget: boolean;
    backupFormat: boolean;
    remoteSyncUrl: boolean;
    remoteSyncTarget: boolean;
    localRestoreTarget: boolean;
  }>({
    backupSource: false,
    backupTarget: false,
    backupFormat: false,
    remoteSyncUrl: false,
    remoteSyncTarget: false,
    localRestoreTarget: false,
  });

  // Backup Card State
  const [backupSource, setBackupSource] = useState(() => prefs.defaultDataPath);
  const [backupTarget, setBackupTarget] = useState(() => prefs.defaultSnapshotFolder);
  const [backupFormat, setBackupFormat] = useState<'zst' | 'gz'>(() =>
    prefs.defaultCompressionFormat === 'tar.gz' ? 'gz' : 'zst'
  );
  const [dataBackupStatus, setDataBackupStatus] = useState<DataBackupStatus | null>(null);
  const [startingBackup, setStartingBackup] = useState(false);
  const [cancellingBackup, setCancellingBackup] = useState(false);

  // Remote Sync Card State
  const [remoteSyncUrl, setRemoteSyncUrl] = useState(() => prefs.defaultRemoteUrl);
  const [remoteSyncTarget, setRemoteSyncTarget] = useState(() => prefs.defaultDataPath);
  const [remoteSnapshotStatus, setRemoteSnapshotStatus] = useState<SnapshotStatus | null>(null);
  const [remoteManagerStatus, setRemoteManagerStatus] = useState<SnapshotManagerStatus | null>(null);
  const [startingRemoteSync, setStartingRemoteSync] = useState(false);
  const [cancellingRemoteSync, setCancellingRemoteSync] = useState(false);

  // Restore Local Archive Card State
  const [localArchivePath, setLocalArchivePath] = useState('');
  const [localRestoreTarget, setLocalRestoreTarget] = useState(() => prefs.defaultDataPath);
  const [startingLocalRestore, setStartingLocalRestore] = useState(false);
  const [cancellingLocalRestore, setCancellingLocalRestore] = useState(false);

  // Bidirectional listener for Settings updates (Requirement 6)
  useEffect(() => {
    const handlePrefsUpdated = (e: any) => {
      if (!e.detail) return;
      const newPrefs: SnapshotPreferences = e.detail;
      setPrefs(newPrefs);

      // Only update fields that the user has NOT manually overridden in this session
      setIsOverridden((current) => {
        if (!current.backupSource) setBackupSource(newPrefs.defaultDataPath);
        if (!current.backupTarget) setBackupTarget(newPrefs.defaultSnapshotFolder);
        if (!current.backupFormat) {
          setBackupFormat(newPrefs.defaultCompressionFormat === 'tar.gz' ? 'gz' : 'zst');
        }
        if (!current.remoteSyncUrl) setRemoteSyncUrl(newPrefs.defaultRemoteUrl);
        if (!current.remoteSyncTarget) setRemoteSyncTarget(newPrefs.defaultDataPath);
        if (!current.localRestoreTarget) setLocalRestoreTarget(newPrefs.defaultDataPath);
        return current;
      });
    };

    window.addEventListener('sirius-snapshot-prefs-updated', handlePrefsUpdated);
    return () => window.removeEventListener('sirius-snapshot-prefs-updated', handlePrefsUpdated);
  }, []);

  // Global Node State & Metrics (Status Strip)
  const [nodeStatus, setNodeStatus] = useState<string>('stopped');
  const [blockHeight, setBlockHeight] = useState<number>(0);
  const [networkHeight, setNetworkHeight] = useState<number>(0);
  const [peersCount, setPeersCount] = useState<number>(0);
  const [metrics, setMetrics] = useState<NodeMetrics | null>(null);
  const [autoRecovery, setAutoRecovery] = useState<boolean>(true);

  // Free Disk Space Readout
  const [diskSpaceData, setDiskSpaceData] = useState<{ free: string; total: string; used: string } | null>(null);

  // Official Updater State
  const [updateInfo, setUpdateInfo] = useState<UpdateInfo | null>(null);
  const [checkingUpdate, setCheckingUpdate] = useState(false);
  const [applyingUpdate, setApplyingUpdate] = useState(false);
  const [syncModalOpen, setSyncModalOpen] = useState(false);
  const [checkFeedback, setCheckFeedback] = useState<{
    message: string;
    time: string;
    isError: boolean;
    repo?: string;
    version?: string;
  } | null>(null);

  // Clean Logs & Cache State
  const [cleaning, setCleaning] = useState(false);
  const [cleanMessage, setCleanMessage] = useState<string | null>(null);
  const [logStats, setLogStats] = useState<LogStats | null>(null);

  // Storage Conversion State
  const [convertSourcePath, setConvertSourcePath] = useState(() => prefs.defaultDataPath);
  const [convertStatus, setConvertStatus] = useState<StorageConvertStatus | null>(null);
  const [startingConvert, setStartingConvert] = useState(false);
  const [cancellingConvert, setCancellingConvert] = useState(false);

  // Disaster Recovery Package (.drpkg) & Settings Export State
  const [drMode, setDrMode] = useState<'dr' | 'settings'>('dr');
  const [drPassphrase, setDrPassphrase] = useState('');
  const [showDrPassphrase, setShowDrPassphrase] = useState(false);
  const [drExporting, setDrExporting] = useState(false);
  const [drRestoring, setDrRestoring] = useState(false);
  const [drMessage, setDrMessage] = useState<string | null>(null);
  const [drError, setDrError] = useState<string | null>(null);
  const [settingsMessage, setSettingsMessage] = useState<string | null>(null);
  const [restoringSettings, setRestoringSettings] = useState(false);

  // Danger Zone: Nemesis Reset State
  const [resetting, setResetting] = useState(false);
  const [resetDone, setResetDone] = useState(false);

  // Type-to-Confirm Modal State
  const [confirmModal, setConfirmModal] = useState<{
    isOpen: boolean;
    title: string;
    description: string;
    expectedText: string;
    confirmButtonText: string;
    action: () => Promise<void>;
  }>({
    isOpen: false,
    title: '',
    description: '',
    expectedText: '',
    confirmButtonText: '',
    action: async () => {},
  });

  // Navigate to Settings -> Snapshots handler
  const navigateToSettingsSnapshots = () => {
    if (onOpenSettings) {
      onOpenSettings('snapshots');
    } else {
      window.location.hash = '#snapshots';
      window.dispatchEvent(new CustomEvent('switch-tab', { detail: { tab: 'config', subtab: 'snapshots' } }));
    }
  };

  // Reset individual field to Settings default
  const handleResetField = (field: keyof typeof isOverridden) => {
    setIsOverridden((prev) => ({ ...prev, [field]: false }));
    switch (field) {
      case 'backupSource':
        setBackupSource(prefs.defaultDataPath);
        break;
      case 'backupTarget':
        setBackupTarget(prefs.defaultSnapshotFolder);
        break;
      case 'backupFormat':
        setBackupFormat(prefs.defaultCompressionFormat === 'tar.gz' ? 'gz' : 'zst');
        break;
      case 'remoteSyncUrl':
        setRemoteSyncUrl(prefs.defaultRemoteUrl);
        break;
      case 'remoteSyncTarget':
        setRemoteSyncTarget(prefs.defaultDataPath);
        break;
      case 'localRestoreTarget':
        setLocalRestoreTarget(prefs.defaultDataPath);
        break;
    }
  };

  // Check if a field is currently matching default
  const isFieldDefault = (
    field: keyof typeof isOverridden,
    currentVal: string,
    defaultVal: string
  ) => {
    return !isOverridden[field] || currentVal.trim() === defaultVal.trim();
  };

  // Reusable Field Header with (default) vs custom (this run) indicator & Reset button
  const renderFieldHeader = (
    label: string,
    field: keyof typeof isOverridden,
    currentVal: string,
    defaultVal: string
  ) => {
    const isDef = isFieldDefault(field, currentVal, defaultVal);

    return (
      <div className="flex items-center justify-between mb-1">
        <div className="flex items-center space-x-1.5">
          <span className="text-[11px] font-semibold text-slate-300">{label}</span>
          {isDef ? (
            <span
              className="px-1.5 py-0.2 rounded text-[10px] font-mono text-zinc-400 bg-zinc-800/80 border border-zinc-700"
              title="Using default value from Settings → Snapshots"
            >
              (default)
            </span>
          ) : (
            <span
              className="px-1.5 py-0.2 rounded text-[10px] font-mono text-amber-400 bg-amber-950/40 border border-amber-500/40"
              title="Overridden for this run only (not saved to Settings)"
            >
              custom (this run)
            </span>
          )}
        </div>

        {!isDef && (
          <button
            type="button"
            onClick={() => handleResetField(field)}
            className="text-[10px] font-mono text-zinc-400 hover:text-white flex items-center space-x-1 transition-colors group"
            title={`Reset to default (${defaultVal})`}
          >
            <RotateCcw className="w-2.5 h-2.5 text-zinc-400 group-hover:-rotate-90 transition-transform" />
            <span>Reset to default</span>
          </button>
        )}
      </div>
    );
  };

  // Polling Function
  const fetchStatus = useCallback(async () => {
    try {
      const [statusRes, backupRes, snapRes, snapMgrRes, convertRes] = await Promise.all([
        fetch('/api/status'),
        fetch('/api/maintenance/data-backup/status'),
        fetch('/api/maintenance/snapshot/status'),
        fetch('/api/snapshot/status'),
        fetch('/api/maintenance/storage-convert/status'),
      ]);

      if (statusRes.ok) {
        const sData = await statusRes.json();
        setNodeStatus(sData.status || 'stopped');
        setBlockHeight(sData.blockHeight || 0);
        setNetworkHeight(sData.networkHeight || 0);
        setPeersCount(sData.peersCount || 0);
        setMetrics(sData.metrics || null);
        if (sData.updateInfo) setUpdateInfo(sData.updateInfo);
        if (typeof sData.autoRecovery === 'boolean') setAutoRecovery(sData.autoRecovery);
      }

      if (backupRes.ok) {
        const bData: DataBackupStatus = await backupRes.json();
        setDataBackupStatus(bData);
      }

      if (snapRes.ok) {
        const snapData: SnapshotStatus = await snapRes.json();
        setRemoteSnapshotStatus(snapData);
      }

      if (snapMgrRes.ok) {
        const mgrData: SnapshotManagerStatus = await snapMgrRes.json();
        setRemoteManagerStatus(mgrData);
      }

      if (convertRes.ok) {
        const cData: StorageConvertStatus = await convertRes.json();
        setConvertStatus(cData);
      }
    } catch (e) {
      // Ignore network polling glitches
    }
  }, []);

  // Poll disk space for backup destination / target data path
  useEffect(() => {
    const fetchDiskSpace = async () => {
      try {
        const checkPath = backupTarget || backupSource || './chainconfig/data';
        const res = await fetch(`/api/system/disk-space?path=${encodeURIComponent(checkPath)}`);
        if (res.ok) {
          const dData = await res.json();
          setDiskSpaceData(dData);
        }
      } catch (e) {
        // ignore
      }
    };

    fetchDiskSpace();
    const interval = setInterval(fetchDiskSpace, 5000);
    return () => clearInterval(interval);
  }, [backupTarget, backupSource]);

  useEffect(() => {
    fetchStatus();
    const interval = setInterval(fetchStatus, 1000);
    return () => clearInterval(interval);
  }, [fetchStatus]);

  // Log Stats Fetcher
  const fetchLogStats = useCallback(async () => {
    try {
      const res = await fetch('/api/maintenance/clean-logs');
      if (res.ok) {
        const data = await res.json();
        setLogStats(data);
      }
    } catch {
      // Ignore polling errors
    }
  }, []);

  useEffect(() => {
    if (activeSubTab === 'system') {
      fetchLogStats();
      const interval = setInterval(fetchLogStats, 5000);
      return () => clearInterval(interval);
    }
  }, [activeSubTab, fetchLogStats]);

  // Watchdog Toggle
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

  // 1. Backup Handler
  const handleStartBackup = async () => {
    setStartingBackup(true);
    try {
      const res = await fetch('/api/maintenance/data-backup', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          sourcePath: backupSource.trim(),
          targetPath: backupTarget.trim(),
          format: backupFormat,
        }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to start data backup');
      fetchStatus();
    } catch (e: any) {
      alert(e.message);
    } finally {
      setStartingBackup(false);
    }
  };

  const handleCancelBackup = async () => {
    setCancellingBackup(true);
    try {
      const res = await fetch('/api/maintenance/data-backup/cancel', { method: 'POST' });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to cancel data backup');
      fetchStatus();
    } catch (e: any) {
      alert(e.message);
    } finally {
      setCancellingBackup(false);
    }
  };

  // 2. Remote Sync Handlers (Protected by Type-to-Confirm Modal)
  const triggerRemoteSyncModal = () => {
    if (!remoteSyncUrl.trim()) {
      alert('Please specify a snapshot or manifest URL');
      return;
    }
    setConfirmModal({
      isOpen: true,
      title: 'Confirm Remote Snapshot Fast-Sync',
      description: `Fast-syncing from remote server will stop the node engine and overwrite existing blockchain blocks in "${remoteSyncTarget}".`,
      expectedText: 'RESTORE-REMOTE',
      confirmButtonText: 'Start Remote Sync',
      action: executeRemoteSync,
    });
  };

  const executeRemoteSync = async () => {
    setStartingRemoteSync(true);
    try {
      const url = remoteSyncUrl.trim();
      const isManifest = url.endsWith('.json');

      let res: Response;
      if (isManifest) {
        res = await fetch('/api/snapshot/restore/remote', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            manifestUrl: url,
            targetDataPath: remoteSyncTarget.trim(),
          }),
        });
      } else {
        res = await fetch('/api/maintenance/snapshot', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            mode: 'remote',
            url: url,
            sourcePath: '',
          }),
        });
      }

      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to start remote snapshot restoration');
      setConfirmModal((prev) => ({ ...prev, isOpen: false }));
      fetchStatus();
    } catch (e: any) {
      alert(e.message);
    } finally {
      setStartingRemoteSync(false);
    }
  };

  const handleCancelRemoteSync = async () => {
    setCancellingRemoteSync(true);
    try {
      await Promise.all([
        fetch('/api/maintenance/snapshot/cancel', { method: 'POST' }).catch(() => {}),
        fetch('/api/snapshot/cancel', { method: 'POST' }).catch(() => {}),
      ]);
      fetchStatus();
    } catch (e: any) {
      alert(e.message);
    } finally {
      setCancellingRemoteSync(false);
    }
  };

  // 3. Local Restore Handlers (Protected by Type-to-Confirm Modal)
  const triggerLocalRestoreModal = () => {
    if (!localArchivePath.trim()) {
      alert('Please select a local snapshot archive file (.tar.zst, .tar.gz, .tar.xz)');
      return;
    }
    setConfirmModal({
      isOpen: true,
      title: 'Confirm Local Archive Restoration',
      description: `Extracting "${localArchivePath}" will pause the node and replace existing blockchain data in "${localRestoreTarget}".`,
      expectedText: 'RESTORE-LOCAL',
      confirmButtonText: 'Restore from Local File',
      action: executeLocalRestore,
    });
  };

  const executeLocalRestore = async () => {
    setStartingLocalRestore(true);
    try {
      const res = await fetch('/api/snapshot/restore/local', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          archivePath: localArchivePath.trim(),
          targetDataPath: localRestoreTarget.trim(),
        }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to start local archive restore');
      setConfirmModal((prev) => ({ ...prev, isOpen: false }));
      fetchStatus();
    } catch (e: any) {
      alert(e.message);
    } finally {
      setStartingLocalRestore(false);
    }
  };

  // 4. Danger Zone: Nemesis Reset (Protected by Type-to-Confirm Modal)
  const triggerNemesisResetModal = () => {
    setConfirmModal({
      isOpen: true,
      title: 'Confirm Genesis Nemesis Reset (Block 1)',
      description: 'This will completely wipe local block and transactional databases, keeping only the genesis Nemesis block (00001.dat). All local blockchain sync progress will be lost and the node will re-sync from Block 1.',
      expectedText: 'RESET-CHAIN',
      confirmButtonText: 'Reset Blockchain to Block 1',
      action: executeNemesisReset,
    });
  };

  const executeNemesisReset = async () => {
    setResetting(true);
    setResetDone(false);
    try {
      const res = await fetch('/api/maintenance/reset', { method: 'POST' });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to reset chain');
      setResetDone(true);
      setConfirmModal((prev) => ({ ...prev, isOpen: false }));
      alert('Blockchain data successfully reset to genesis nemesis block (Block 1). Ready to restart.');
      fetchStatus();
    } catch (e: any) {
      alert(e.message);
    } finally {
      setResetting(false);
    }
  };

  // Protocol Updater Handlers
  const handleCheckUpdate = async () => {
    setCheckingUpdate(true);
    try {
      const res = await fetch('/api/maintenance/update/check');
      const data = await res.json();
      if (res.ok && data) {
        setUpdateInfo(data);
        const timeStr = new Date().toLocaleTimeString();
        if (data.hasUpdate) {
          setCheckFeedback({
            message: `New release ${data.latestVersion} available upstream (current: ${data.currentVersion || 'v1.9.8'}).`,
            time: timeStr,
            isError: false,
            repo: 'proximax-storage/cpp-xpx-chain',
            version: data.latestVersion,
          });
        } else {
          setCheckFeedback({
            message: `Configurations and protocol are up to date with official upstream (${data.currentVersion || 'v1.9.8'}).`,
            time: timeStr,
            isError: false,
            repo: 'proximax-storage/cpp-xpx-chain',
            version: data.currentVersion || 'v1.9.8',
          });
        }
      } else {
        setCheckFeedback({
          message: data?.error || 'Failed to check GitHub releases',
          time: new Date().toLocaleTimeString(),
          isError: true,
        });
      }
    } catch (e: any) {
      setCheckFeedback({
        message: e?.message || 'Network error while querying GitHub API',
        time: new Date().toLocaleTimeString(),
        isError: true,
      });
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

  // Clean Cache & Logs Handler
  const handleCleanLogs = async () => {
    setCleaning(true);
    setCleanMessage(null);
    try {
      const res = await fetch('/api/maintenance/clean-logs', { method: 'POST' });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to clean logs');
      setCleanMessage(`Freed ${(data.freedMB || 0).toFixed(2)} MB of disk space by purging historical logs and stale server locks.`);
      await fetchLogStats();
    } catch (e: any) {
      alert(e.message);
    } finally {
      setCleaning(false);
    }
  };

  // Storage Convert Handlers
  const handleStartConvert = async () => {
    if (!window.confirm('Convert legacy loose block files into compact 4-file Bitcoin-style chunks? This will pause the node, pack blocks.dat / statements.dat / blocks.idx, and delete old loose files to free inodes.')) {
      return;
    }
    setStartingConvert(true);
    try {
      const res = await fetch('/api/maintenance/storage-convert', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ dataPath: convertSourcePath }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to start storage migration');
      fetchStatus();
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
      fetchStatus();
    } catch (e) {
      // ignore
    } finally {
      setCancellingConvert(false);
    }
  };

  // Disaster Recovery Handlers
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
      setDrPassphrase('');
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

    if (!window.confirm(`Restore ALL Catapult configurations, certificates, and harvesting keys from "${file.name}"? This will overwrite existing node credentials on this host.`)) {
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
            body: JSON.stringify({ passphrase: pass, package: pkgObj }),
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
      setSettingsMessage('Settings exported successfully.');
    } catch (e: any) {
      alert(e.message);
    }
  };

  const handleImportSettings = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

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

  // State calculations for live operations
  const isBackupRunning = dataBackupStatus?.stage === 'backing_up';
  const isRemoteSyncRunning =
    remoteSnapshotStatus?.stage === 'downloading' ||
    remoteSnapshotStatus?.stage === 'extracting' ||
    (remoteManagerStatus?.stage &&
      ['fetching_manifest', 'verifying_signature', 'downloading', 'verifying_checksum', 'extracting'].includes(
        remoteManagerStatus.stage
      ));
  const isLocalRestoreRunning =
    remoteManagerStatus?.stage === 'extracting' && remoteManagerStatus?.operation === 'restore_local';
  const isConvertRunning = convertStatus?.status === 'running';

  const syncPercent =
    networkHeight > 0 ? Math.min(100, Math.max(0, (blockHeight / networkHeight) * 100)) : 0;
  const isSynced = networkHeight > 0 && Math.abs(networkHeight - blockHeight) <= 10;

  // Free disk readout helper
  const availableDiskFree = diskSpaceData?.free || metrics?.diskFree || 'Checking...';

  // Format toggle helper
  const isFormatDefault =
    !isOverridden.backupFormat ||
    backupFormat === (prefs.defaultCompressionFormat === 'tar.gz' ? 'gz' : 'zst');

  return (
    <div className="space-y-5 animate-in fade-in duration-150 pb-8 max-w-6xl mx-auto">
      
      {/* LEVEL 1: Page Title & Top Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between pb-3 border-b border-[#262B34] gap-2">
        <div className="flex items-center space-x-3">
          <div className="w-8 h-8 rounded-lg bg-[#181B20] border border-[#262B34] flex items-center justify-center text-slate-300 flex-shrink-0">
            <Wrench className="w-4 h-4" />
          </div>
          <div>
            <div className="flex items-center space-x-2">
              <h1 className="text-base font-bold text-white tracking-wider uppercase">
                Maintenance & Synchronization Center
              </h1>
              {/* Status Badge: Gray (Info) */}
              <span className="px-2 py-0.5 rounded text-[10px] font-mono bg-zinc-800 text-zinc-300 border border-zinc-700">
                Native {updateInfo?.currentVersion || 'v1.9.8'}
              </span>
            </div>
            <p className="text-xs text-slate-400">
              Operations center for blockchain backups, fast-sync snapshot streams, data migration, and recovery tools.
            </p>
          </div>
        </div>

        {/* Watchdog Toggle Button */}
        <button
          type="button"
          onClick={handleToggleWatchdog}
          className={`px-3 py-1.5 rounded-lg text-xs font-semibold border transition-all flex items-center space-x-2 flex-shrink-0 ${
            autoRecovery
              ? 'bg-emerald-950/40 border-emerald-500/40 text-emerald-400 hover:bg-emerald-950/60'
              : 'bg-zinc-800/80 border-zinc-700 text-zinc-400 hover:bg-zinc-700'
          }`}
          title="Toggle Auto-Recovery Process Watchdog"
        >
          <span
            className={`w-2 h-2 rounded-full ${
              autoRecovery ? 'bg-emerald-400 shadow-[0_0_6px_#34d399]' : 'bg-zinc-500'
            }`}
          />
          <span className="font-mono">Watchdog: {autoRecovery ? 'ACTIVE' : 'DISABLED'}</span>
        </button>
      </div>

      {/* COMPACT STATUS STRIP - Always visible above sub-tabs (Requirement) */}
      <div className="bg-[#13171F] border border-[#262B34] rounded-xl p-3 grid grid-cols-2 md:grid-cols-4 gap-3 text-xs">
        {/* Strip 1: Node State */}
        <div className="flex items-center space-x-2.5">
          <div
            className={`w-2.5 h-2.5 rounded-full flex-shrink-0 ${
              nodeStatus === 'running'
                ? 'bg-emerald-400 shadow-[0_0_8px_#34d399]'
                : nodeStatus === 'starting'
                ? 'bg-amber-400 shadow-[0_0_8px_#fbbf24]'
                : 'bg-zinc-500'
            }`}
          />
          <div>
            <span className="text-slate-400 text-[11px] block">Node State</span>
            <span className="font-bold text-white font-mono uppercase tracking-wide">
              {nodeStatus}
            </span>
          </div>
        </div>

        {/* Strip 2: Connected Peers */}
        <div className="flex items-center space-x-2.5">
          <Radio className="w-4 h-4 text-slate-400 flex-shrink-0" />
          <div>
            <span className="text-slate-400 text-[11px] block">Peer Connectivity</span>
            <span className="font-bold text-white font-mono">
              {peersCount} {peersCount === 1 ? 'Peer' : 'Peers'} Connected
            </span>
          </div>
        </div>

        {/* Strip 3: Height & Sync Progress */}
        <div className="flex items-center space-x-2.5">
          <Activity className="w-4 h-4 text-slate-400 flex-shrink-0" />
          <div>
            <span className="text-slate-400 text-[11px] block">Height / Consensus</span>
            <div className="flex items-center space-x-1.5 font-mono">
              <span className="text-white font-bold">{blockHeight.toLocaleString()}</span>
              {networkHeight > 0 && (
                <span className={`text-[10px] font-bold ${isSynced ? 'text-emerald-400' : 'text-amber-400'}`}>
                  ({syncPercent.toFixed(1)}%)
                </span>
              )}
            </div>
          </div>
        </div>

        {/* Strip 4: Drive Free Space */}
        <div className="flex items-center space-x-2.5">
          <HardDrive className="w-4 h-4 text-slate-400 flex-shrink-0" />
          <div>
            <span className="text-slate-400 text-[11px] block">Storage Available</span>
            <span className="font-bold text-white font-mono">{availableDiskFree}</span>
          </div>
        </div>
      </div>

      {/* SUB-TAB NAVIGATION BAR - Following the exact pattern used in Settings */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between pb-1 gap-2">
        <div className="flex items-center space-x-1 bg-[#0F1115] p-1 rounded-lg border border-[#262B34]">
          <button
            type="button"
            onClick={() => setActiveSubTab('snapshots')}
            className={`px-3 py-1.5 text-xs font-semibold rounded-md transition-all flex items-center space-x-2 ${
              activeSubTab === 'snapshots'
                ? 'bg-[#262B34] text-white shadow-xs'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            <Archive className="w-3.5 h-3.5 text-slate-300" />
            <span>Snapshots & Sync</span>
            <span className="text-[10px] font-mono px-1.5 py-0.2 rounded bg-zinc-800 text-zinc-400 border border-zinc-700">
              3
            </span>
          </button>

          <button
            type="button"
            onClick={() => setActiveSubTab('system')}
            className={`px-3 py-1.5 text-xs font-semibold rounded-md transition-all flex items-center space-x-2 ${
              activeSubTab === 'system'
                ? 'bg-[#262B34] text-white shadow-xs'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            <Sliders className="w-3.5 h-3.5 text-slate-300" />
            <span>System & Health</span>
            <span className="text-[10px] font-mono px-1.5 py-0.2 rounded bg-zinc-800 text-zinc-400 border border-zinc-700">
              4
            </span>
          </button>
        </div>

        <div className="text-[11px] text-slate-500 font-mono hidden sm:block">
          {activeSubTab === 'snapshots'
            ? 'Blockchain backup archives, fast-sync snapshot streams, and local unpack'
            : 'Protocol updating, encrypted disaster recovery, chunk migration, and log cache cleaning'}
        </div>
      </div>

      {/* SUB-TAB 1: Snapshots & Sync (Backup, Remote Sync, Restore Local Archive) */}
      {activeSubTab === 'snapshots' && (
        <div className="space-y-4 animate-in fade-in duration-150">
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
            
            {/* CARD 1: BACKUP (Merged Create Snapshot + Data Backup) */}
            <div className="bg-[#13171F] border border-[#262B34] rounded-xl p-4 flex flex-col justify-between space-y-4">
              <div className="space-y-3">
                <div className="flex items-center justify-between pb-2 border-b border-[#262B34]">
                  <div className="flex items-center space-x-2">
                    <Archive className="w-4 h-4 text-slate-300" />
                    <h3 className="text-xs font-semibold uppercase tracking-wider text-white">
                      Backup
                    </h3>
                  </div>
                  {/* 3-State Status Badge: Green (Ready/Done) / Amber (Backing up) */}
                  {isBackupRunning ? (
                    <span className="px-2 py-0.5 rounded text-[10px] font-mono font-bold bg-amber-950/40 text-amber-400 border border-amber-500/40 uppercase">
                      Backing Up
                    </span>
                  ) : dataBackupStatus?.stage === 'complete' ? (
                    <span className="px-2 py-0.5 rounded text-[10px] font-mono font-bold bg-emerald-950/40 text-emerald-400 border border-emerald-500/40 uppercase">
                      Completed
                    </span>
                  ) : (
                    <span className="px-2 py-0.5 rounded text-[10px] font-mono bg-zinc-800 text-zinc-300 border border-zinc-700">
                      Ready
                    </span>
                  )}
                </div>

                <p className="text-[11px] text-slate-400 leading-relaxed">
                  Create a full point-in-time compressed archive of all blockchain block and state directories directly to local or external SSD storage.
                </p>

                {/* Form: Source path, Destination path, Compression format */}
                <div className="space-y-2.5 pt-1">
                  <div>
                    {renderFieldHeader(
                      'Source Blockchain Directory (data.path)',
                      'backupSource',
                      backupSource,
                      prefs.defaultDataPath
                    )}
                    <DirectoryDropdown
                      value={backupSource}
                      onChange={(val) => {
                        setIsOverridden((prev) => ({ ...prev, backupSource: true }));
                        setBackupSource(val);
                      }}
                      placeholder="./chainconfig/data"
                      prompt="Select Source Blockchain Directory"
                    />
                  </div>

                  <div>
                    {renderFieldHeader(
                      'Destination Folder',
                      'backupTarget',
                      backupTarget,
                      prefs.defaultSnapshotFolder
                    )}
                    <DirectoryDropdown
                      value={backupTarget}
                      onChange={(val) => {
                        setIsOverridden((prev) => ({ ...prev, backupTarget: true }));
                        setBackupTarget(val);
                      }}
                      placeholder="/Volumes/SSD/snapshots"
                      prompt="Select Backup Save Directory"
                    />
                  </div>

                  <div>
                    <div className="flex items-center justify-between mb-1">
                      <div className="flex items-center space-x-1.5">
                        <span className="text-[11px] font-semibold text-slate-300">
                          Compression Format
                        </span>
                        {isFormatDefault ? (
                          <span
                            className="px-1.5 py-0.2 rounded text-[10px] font-mono text-zinc-400 bg-zinc-800/80 border border-zinc-700"
                            title="Using default configured in Settings → Snapshots"
                          >
                            (default)
                          </span>
                        ) : (
                          <span
                            className="px-1.5 py-0.2 rounded text-[10px] font-mono text-amber-400 bg-amber-950/40 border border-amber-500/40"
                            title="Overridden for this run only"
                          >
                            custom (this run)
                          </span>
                        )}
                      </div>

                      {!isFormatDefault && (
                        <button
                          type="button"
                          onClick={() => handleResetField('backupFormat')}
                          className="text-[10px] font-mono text-zinc-400 hover:text-white flex items-center space-x-1 transition-colors group"
                          title={`Reset to default (.tar.${prefs.defaultCompressionFormat === 'tar.gz' ? 'gz' : 'zst'})`}
                        >
                          <RotateCcw className="w-2.5 h-2.5 text-zinc-400 group-hover:-rotate-90 transition-transform" />
                          <span>Reset to default</span>
                        </button>
                      )}
                    </div>

                    <div className="grid grid-cols-2 gap-2">
                      <button
                        type="button"
                        onClick={() => {
                          setIsOverridden((prev) => ({ ...prev, backupFormat: true }));
                          setBackupFormat('zst');
                        }}
                        disabled={isBackupRunning}
                        className={`py-1.5 px-2 rounded-lg text-xs font-mono font-medium transition-all flex items-center justify-between ${
                          backupFormat === 'zst'
                            ? 'bg-emerald-950/40 text-emerald-400 border border-emerald-500/50'
                            : 'bg-[#181B20] text-slate-400 hover:text-white border border-[#262B34]'
                        }`}
                      >
                        <span>.tar.zst (Zstandard)</span>
                        {prefs.defaultCompressionFormat === 'tar.zst' && (
                          <span className="text-[9.5px] px-1 py-0.2 rounded bg-zinc-800 text-zinc-400 border border-zinc-700 font-sans">
                            default
                          </span>
                        )}
                      </button>
                      <button
                        type="button"
                        onClick={() => {
                          setIsOverridden((prev) => ({ ...prev, backupFormat: true }));
                          setBackupFormat('gz');
                        }}
                        disabled={isBackupRunning}
                        className={`py-1.5 px-2 rounded-lg text-xs font-mono font-medium transition-all flex items-center justify-between ${
                          backupFormat === 'gz'
                            ? 'bg-emerald-950/40 text-emerald-400 border border-emerald-500/50'
                            : 'bg-[#181B20] text-slate-400 hover:text-white border border-[#262B34]'
                        }`}
                      >
                        <span>.tar.gz (Gzip)</span>
                        {prefs.defaultCompressionFormat === 'tar.gz' && (
                          <span className="text-[9.5px] px-1 py-0.2 rounded bg-zinc-800 text-zinc-400 border border-zinc-700 font-sans">
                            default
                          </span>
                        )}
                      </button>
                    </div>
                  </div>

                  {/* "Last backup" readout and Disk space readout */}
                  <div className="p-2.5 bg-[#0F1115] rounded-lg border border-[#262B34] space-y-1 text-[11px] font-mono">
                    <div className="flex items-center justify-between text-slate-400">
                      <span>Free Disk Space:</span>
                      <span className="text-slate-200 font-bold">{availableDiskFree}</span>
                    </div>
                    <div className="flex items-center justify-between text-slate-400">
                      <span>Last Backup:</span>
                      <span className="text-slate-200 font-bold truncate max-w-[180px]">
                        {dataBackupStatus?.lastBackupTime
                          ? `${dataBackupStatus.lastBackupTime} (${dataBackupStatus.lastBackupSize || ''})`
                          : 'No backups recorded'}
                      </span>
                    </div>
                  </div>

                  {/* Live Progress Bar */}
                  {isBackupRunning && (
                    <div className="p-2.5 bg-[#0F1115] rounded-lg border border-amber-500/40 space-y-1.5 animate-in fade-in duration-150">
                      <div className="flex justify-between text-[11px] font-mono">
                        <span className="text-amber-400 font-bold">
                          {dataBackupStatus?.percentage ? `${dataBackupStatus.percentage.toFixed(1)}%` : 'Archiving...'}
                        </span>
                        <span className="text-slate-400 truncate max-w-[160px]">
                          {dataBackupStatus?.currentFile || 'Scanning files...'}
                        </span>
                      </div>
                      <div className="w-full bg-[#181B20] h-1.5 rounded-full overflow-hidden">
                        <div
                          className="bg-amber-400 h-full rounded-full transition-all duration-300"
                          style={{ width: `${Math.max(5, dataBackupStatus?.percentage || 0)}%` }}
                        />
                      </div>
                      <p className="text-[10px] text-slate-400 truncate">{dataBackupStatus?.message}</p>
                    </div>
                  )}
                </div>
              </div>

              {/* Action Buttons */}
              <div className="pt-2 border-t border-[#262B34]">
                {!isBackupRunning ? (
                  <button
                    type="button"
                    onClick={handleStartBackup}
                    disabled={startingBackup}
                    className="w-full py-2 bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white rounded-xl text-xs font-semibold shadow-xs transition-all flex items-center justify-center space-x-1.5"
                  >
                    <Archive className="w-3.5 h-3.5" />
                    <span>{startingBackup ? 'Initializing...' : 'Create Backup Archive'}</span>
                  </button>
                ) : (
                  <button
                    type="button"
                    onClick={handleCancelBackup}
                    disabled={cancellingBackup}
                    className="w-full py-2 bg-rose-600 hover:bg-rose-500 disabled:opacity-50 text-white rounded-xl text-xs font-semibold shadow-xs transition-all flex items-center justify-center space-x-1.5"
                  >
                    <StopCircle className="w-3.5 h-3.5" />
                    <span>{cancellingBackup ? 'Cancelling...' : 'Cancel Backup'}</span>
                  </button>
                )}
              </div>
            </div>

            {/* CARD 2: REMOTE SYNC (Merged Fast-Sync Snapshot + Remote Restore) */}
            <div className="bg-[#13171F] border border-[#262B34] rounded-xl p-4 flex flex-col justify-between space-y-4">
              <div className="space-y-3">
                <div className="flex items-center justify-between pb-2 border-b border-[#262B34]">
                  <div className="flex items-center space-x-2">
                    <DownloadCloud className="w-4 h-4 text-slate-300" />
                    <h3 className="text-xs font-semibold uppercase tracking-wider text-white">
                      Remote Sync
                    </h3>
                  </div>
                  {/* 3-State Status Badge */}
                  {isRemoteSyncRunning ? (
                    <span className="px-2 py-0.5 rounded text-[10px] font-mono font-bold bg-amber-950/40 text-amber-400 border border-amber-500/40 uppercase">
                      Streaming
                    </span>
                  ) : remoteSnapshotStatus?.stage === 'complete' ? (
                    <span className="px-2 py-0.5 rounded text-[10px] font-mono font-bold bg-emerald-950/40 text-emerald-400 border border-emerald-500/40 uppercase">
                      Synced
                    </span>
                  ) : (
                    <span className="px-2 py-0.5 rounded text-[10px] font-mono bg-zinc-800 text-zinc-300 border border-zinc-700">
                      Ready
                    </span>
                  )}
                </div>

                <p className="text-[11px] text-slate-400 leading-relaxed">
                  Direct streaming fast-sync decompression from remote snapshot servers (Cloudflare R2, S3, B2, or HTTP). Direct streaming unpack with zero intermediate disk overhead.
                </p>

                {/* Form: URL, Target path, and Read-Only Verification PubKey with link */}
                <div className="space-y-2.5 pt-1">
                  <div>
                    {renderFieldHeader(
                      'Manifest or Snapshot URL',
                      'remoteSyncUrl',
                      remoteSyncUrl,
                      prefs.defaultRemoteUrl
                    )}
                    <input
                      type="text"
                      value={remoteSyncUrl}
                      onChange={(e) => {
                        setIsOverridden((prev) => ({ ...prev, remoteSyncUrl: true }));
                        setRemoteSyncUrl(e.target.value);
                      }}
                      disabled={isRemoteSyncRunning}
                      placeholder="https://huggingface.co/datasets/igorgoc/sirius-snapshot/resolve/main/manifest.json"
                      className="w-full px-3 py-1.5 bg-[#0F1115] border border-[#262B34] focus:border-zinc-500 rounded-lg text-xs font-mono text-white placeholder-slate-500 focus:outline-none transition-all disabled:opacity-50"
                    />
                  </div>

                  <div>
                    {renderFieldHeader(
                      'Target Data Directory (data.path)',
                      'remoteSyncTarget',
                      remoteSyncTarget,
                      prefs.defaultDataPath
                    )}
                    <DirectoryDropdown
                      value={remoteSyncTarget}
                      onChange={(val) => {
                        setIsOverridden((prev) => ({ ...prev, remoteSyncTarget: true }));
                        setRemoteSyncTarget(val);
                      }}
                      placeholder="./chainconfig/data"
                      prompt="Select Target Blockchain Data Directory"
                    />
                  </div>

                  {/* Cryptographic Authenticity Invariant */}
                  <div className="p-2.5 bg-[#0F1115] rounded-lg border border-[#262B34] flex items-center justify-between text-[11px]">
                    <div className="flex items-center space-x-2.5 min-w-0">
                      <div className="w-6 h-6 rounded bg-emerald-950/50 border border-emerald-500/30 flex items-center justify-center text-emerald-400 flex-shrink-0">
                        <ShieldCheck className="w-3.5 h-3.5" />
                      </div>
                      <div className="min-w-0">
                        <div className="flex items-center space-x-1.5 text-slate-400 text-[10.5px]">
                          <span>Authenticity Verification</span>
                        </div>
                        <div className="text-slate-200 text-xs truncate">
                          Verified via official Ed25519 signature & SHA-256 before unpack
                        </div>
                      </div>
                    </div>
                    <span className="text-[10px] font-mono text-emerald-400 bg-emerald-950/40 px-2 py-0.5 rounded border border-emerald-500/30 flex-shrink-0 ml-2">
                      Fail-Closed
                    </span>
                  </div>

                  {/* Target Disk Space Readout */}
                  <div className="p-2.5 bg-[#0F1115] rounded-lg border border-[#262B34] flex items-center justify-between text-[11px] font-mono text-slate-400">
                    <span>Target Free Space:</span>
                    <span className="text-slate-200 font-bold">{availableDiskFree}</span>
                  </div>

                  {/* Live Progress Bar */}
                  {isRemoteSyncRunning && (
                    <div className="p-2.5 bg-[#0F1115] rounded-lg border border-amber-500/40 space-y-1.5 animate-in fade-in duration-150">
                      <div className="flex justify-between text-[11px] font-mono">
                        <span className="text-amber-400 font-bold">
                          {remoteSnapshotStatus?.download?.percentage
                            ? `${remoteSnapshotStatus.download.percentage.toFixed(1)}%`
                            : remoteManagerStatus?.progress?.percentage
                            ? `${remoteManagerStatus.progress.percentage.toFixed(1)}%`
                            : 'Streaming...'}
                        </span>
                        <span className="text-slate-400 truncate max-w-[160px]">
                          {remoteSnapshotStatus?.stage?.toUpperCase() || remoteManagerStatus?.stage?.toUpperCase()}
                        </span>
                      </div>
                      <div className="w-full bg-[#181B20] h-1.5 rounded-full overflow-hidden">
                        <div
                          className="bg-amber-400 h-full rounded-full transition-all duration-300"
                          style={{
                            width: `${Math.max(
                              5,
                              remoteSnapshotStatus?.download?.percentage ||
                                remoteManagerStatus?.progress?.percentage ||
                                0
                            )}%`,
                          }}
                        />
                      </div>
                      <p className="text-[10px] text-slate-400 truncate">
                        {remoteSnapshotStatus?.message || remoteManagerStatus?.message}
                      </p>
                    </div>
                  )}
                </div>
              </div>

              {/* Action Buttons */}
              <div className="pt-2 border-t border-[#262B34]">
                {!isRemoteSyncRunning ? (
                  <button
                    type="button"
                    onClick={triggerRemoteSyncModal}
                    disabled={startingRemoteSync}
                    className="w-full py-2 bg-zinc-700 hover:bg-zinc-600 disabled:opacity-50 text-white rounded-xl text-xs font-semibold shadow-xs transition-all flex items-center justify-center space-x-1.5"
                  >
                    <DownloadCloud className="w-3.5 h-3.5" />
                    <span>{startingRemoteSync ? 'Initializing...' : 'Start Remote Sync'}</span>
                  </button>
                ) : (
                  <button
                    type="button"
                    onClick={handleCancelRemoteSync}
                    disabled={cancellingRemoteSync}
                    className="w-full py-2 bg-rose-600 hover:bg-rose-500 disabled:opacity-50 text-white rounded-xl text-xs font-semibold shadow-xs transition-all flex items-center justify-center space-x-1.5"
                  >
                    <StopCircle className="w-3.5 h-3.5" />
                    <span>{cancellingRemoteSync ? 'Cancelling...' : 'Cancel Remote Sync'}</span>
                  </button>
                )}
              </div>
            </div>

            {/* CARD 3: RESTORE LOCAL ARCHIVE */}
            <div className="bg-[#13171F] border border-[#262B34] rounded-xl p-4 flex flex-col justify-between space-y-4">
              <div className="space-y-3">
                <div className="flex items-center justify-between pb-2 border-b border-[#262B34]">
                  <div className="flex items-center space-x-2">
                    <HardDrive className="w-4 h-4 text-slate-300" />
                    <h3 className="text-xs font-semibold uppercase tracking-wider text-white">
                      Restore Local Archive
                    </h3>
                  </div>
                  {/* 3-State Status Badge */}
                  {isLocalRestoreRunning ? (
                    <span className="px-2 py-0.5 rounded text-[10px] font-mono font-bold bg-amber-950/40 text-amber-400 border border-amber-500/40 uppercase">
                      Extracting
                    </span>
                  ) : (
                    <span className="px-2 py-0.5 rounded text-[10px] font-mono bg-zinc-800 text-zinc-300 border border-zinc-700">
                      Direct Unpack
                    </span>
                  )}
                </div>

                <p className="text-[11px] text-slate-400 leading-relaxed">
                  Extract an existing archive (<code className="text-slate-300">.tar.zst</code>, <code className="text-slate-300">.tar.gz</code>, <code className="text-slate-300">.tar.xz</code>) directly into your active data directory.
                </p>

                {/* Form: Select Archive File, Target data path */}
                <div className="space-y-2.5 pt-1">
                  <div>
                    <label className="text-[11px] font-semibold text-slate-300 block mb-1">
                      Select Local Archive File
                    </label>
                    <DirectoryDropdown
                      value={localArchivePath}
                      onChange={setLocalArchivePath}
                      placeholder="/Volumes/SSD/snapshots/snapshot.tar.zst"
                      mode="file"
                      prompt="Select Sirius Snapshot Archive File"
                    />
                  </div>

                  <div>
                    {renderFieldHeader(
                      'Target Data Directory (data.path)',
                      'localRestoreTarget',
                      localRestoreTarget,
                      prefs.defaultDataPath
                    )}
                    <DirectoryDropdown
                      value={localRestoreTarget}
                      onChange={(val) => {
                        setIsOverridden((prev) => ({ ...prev, localRestoreTarget: true }));
                        setLocalRestoreTarget(val);
                      }}
                      placeholder="./chainconfig/data"
                      prompt="Select Target Data Directory"
                    />
                  </div>

                  {/* Target Disk Space Readout */}
                  <div className="p-2.5 bg-[#0F1115] rounded-lg border border-[#262B34] flex items-center justify-between text-[11px] font-mono text-slate-400">
                    <span>Target Free Space:</span>
                    <span className="text-slate-200 font-bold">{availableDiskFree}</span>
                  </div>

                  {/* Live Progress Bar */}
                  {isLocalRestoreRunning && (
                    <div className="p-2.5 bg-[#0F1115] rounded-lg border border-amber-500/40 space-y-1.5 animate-in fade-in duration-150">
                      <div className="flex justify-between text-[11px] font-mono">
                        <span className="text-amber-400 font-bold">
                          {remoteManagerStatus?.progress?.percentage
                            ? `${remoteManagerStatus.progress.percentage.toFixed(1)}%`
                            : 'Extracting...'}
                        </span>
                        <span className="text-slate-400 truncate max-w-[160px]">
                          {remoteManagerStatus?.progress?.currentItem || 'Unpacking files...'}
                        </span>
                      </div>
                      <div className="w-full bg-[#181B20] h-1.5 rounded-full overflow-hidden">
                        <div
                          className="bg-amber-400 h-full rounded-full transition-all duration-300"
                          style={{
                            width: `${Math.max(5, remoteManagerStatus?.progress?.percentage || 0)}%`,
                          }}
                        />
                      </div>
                      <p className="text-[10px] text-slate-400 truncate">{remoteManagerStatus?.message}</p>
                    </div>
                  )}
                </div>
              </div>

              {/* Action Buttons */}
              <div className="pt-2 border-t border-[#262B34]">
                {!isLocalRestoreRunning ? (
                  <button
                    type="button"
                    onClick={triggerLocalRestoreModal}
                    disabled={startingLocalRestore}
                    className="w-full py-2 bg-zinc-700 hover:bg-zinc-600 disabled:opacity-50 text-white rounded-xl text-xs font-semibold shadow-xs transition-all flex items-center justify-center space-x-1.5"
                  >
                    <HardDrive className="w-3.5 h-3.5" />
                    <span>{startingLocalRestore ? 'Extracting...' : 'Restore from Local File'}</span>
                  </button>
                ) : (
                  <button
                    type="button"
                    onClick={handleCancelRemoteSync}
                    disabled={cancellingLocalRestore}
                    className="w-full py-2 bg-rose-600 hover:bg-rose-500 disabled:opacity-50 text-white rounded-xl text-xs font-semibold shadow-xs transition-all flex items-center justify-center space-x-1.5"
                  >
                    <StopCircle className="w-3.5 h-3.5" />
                    <span>{cancellingLocalRestore ? 'Cancelling...' : 'Cancel Restore'}</span>
                  </button>
                )}
              </div>
            </div>

          </div>
        </div>
      )}

      {/* SUB-TAB 2: System & Health (Protocol Updater, Disaster Recovery, Storage Migration, Clean Cache & Logs) */}
      {activeSubTab === 'system' && (
        <div className="space-y-4 animate-in fade-in duration-150">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            
            {/* UTILITY 1: Protocol & Engine Updater */}
            <div className="bg-[#13171F] border border-[#262B34] rounded-xl p-4 flex flex-col justify-between space-y-3">
              <div className="space-y-2">
                <div className="flex items-center justify-between pb-1.5 border-b border-[#262B34]">
                  <div className="flex items-center space-x-2">
                    <DownloadCloud className="w-4 h-4 text-slate-300" />
                    <h3 className="text-xs font-semibold uppercase tracking-wider text-white">
                      Official Protocol Updater
                    </h3>
                  </div>
                  {/* 3-State Badge: Gray (Version) */}
                  <span className="text-[10px] font-mono bg-zinc-800 text-zinc-300 px-2 py-0.5 rounded border border-zinc-700">
                    {updateInfo?.currentVersion || 'v1.9.8'}
                  </span>
                </div>
                <p className="text-[11px] text-slate-400 leading-relaxed">
                  Synchronizes official seeds, peers, and network configurations from GitHub while preserving local C++ native performance patches.
                </p>
                <div className="p-2.5 bg-[#0F1115] rounded-lg border border-[#262B34] text-[11px] flex items-center justify-between font-mono">
                  <span className="text-slate-400">Release Status:</span>
                  {/* 3-State Status: Amber (Update) or Green (Up to Date) */}
                  <span className={updateInfo?.hasUpdate ? 'text-amber-400 font-bold' : 'text-emerald-400 font-bold'}>
                    {updateInfo?.hasUpdate
                      ? `Update ${updateInfo.latestVersion} Available`
                      : `Up to Date (${updateInfo?.currentVersion || 'v1.9.8'})`}
                  </span>
                </div>

                {/* GitHub Check Feedback Notice */}
                {checkFeedback && (
                  <div className={`p-2.5 rounded-lg border text-[11px] leading-snug flex items-start space-x-2 animate-in fade-in duration-150 ${
                    checkFeedback.isError
                      ? 'bg-rose-950/30 border-rose-800/40 text-rose-300'
                      : 'bg-emerald-950/30 border-emerald-800/40 text-emerald-300'
                  }`}>
                    {checkFeedback.isError ? (
                      <AlertTriangle className="w-3.5 h-3.5 shrink-0 mt-0.5 text-rose-400" />
                    ) : (
                      <CheckCircle2 className="w-3.5 h-3.5 shrink-0 mt-0.5 text-emerald-400" />
                    )}
                    <div className="flex-1 min-w-0">
                      <div className="font-semibold flex items-center justify-between">
                        <span className="truncate">GitHub Checked ({checkFeedback.repo || 'Upstream'})</span>
                        <span className="text-[10px] opacity-75 font-mono ml-2 shrink-0">{checkFeedback.time}</span>
                      </div>
                      <div className="mt-0.5 text-[10.5px] opacity-90">{checkFeedback.message}</div>
                    </div>
                  </div>
                )}
              </div>

              <div className="pt-2 border-t border-[#262B34] flex items-center justify-between gap-2">
                <button
                  type="button"
                  onClick={handleCheckUpdate}
                  disabled={checkingUpdate}
                  className="px-3 py-1.5 bg-[#181B20] hover:bg-[#262B34] text-slate-300 hover:text-white rounded-lg text-xs font-semibold border border-[#262B34] transition-colors flex items-center space-x-1"
                >
                  <RefreshCw className={`w-3 h-3 ${checkingUpdate ? 'animate-spin' : ''}`} />
                  <span>{checkingUpdate ? 'Checking...' : 'Check GitHub'}</span>
                </button>
                <button
                  type="button"
                  onClick={() => window.dispatchEvent(new CustomEvent('open-engine-updater'))}
                  className="px-3 py-1.5 bg-zinc-800 hover:bg-zinc-700 text-zinc-200 rounded-lg text-xs font-semibold border border-zinc-600 transition-all flex items-center space-x-1"
                >
                  <ArrowUpCircle className="w-3 h-3" />
                  <span>Engine Binary</span>
                </button>
                <button
                  type="button"
                  onClick={() => setSyncModalOpen(true)}
                  className="px-3 py-1.5 bg-zinc-700 hover:bg-zinc-600 text-white rounded-lg text-xs font-semibold shadow-xs transition-all flex items-center space-x-1"
                >
                  <DownloadCloud className="w-3 h-3" />
                  <span>Sync Configs</span>
                </button>
              </div>
            </div>

            {/* UTILITY 2: Disaster Recovery Package & Settings */}
            <div className="bg-[#13171F] border border-[#262B34] rounded-xl p-4 flex flex-col justify-between space-y-3">
              <div className="space-y-2">
                <div className="flex items-center justify-between pb-1.5 border-b border-[#262B34]">
                  <div className="flex items-center space-x-2">
                    {drMode === 'dr' ? (
                      <ShieldCheck className="w-4 h-4 text-emerald-400" />
                    ) : (
                      <FolderInput className="w-4 h-4 text-slate-300" />
                    )}
                    <h3 className="text-xs font-semibold uppercase tracking-wider text-white">
                      {drMode === 'dr' ? 'Disaster Recovery' : 'Node Settings'}
                    </h3>
                  </div>
                  <div className="flex items-center space-x-1">
                    <button
                      type="button"
                      onClick={() => setDrMode('dr')}
                      className={`px-2 py-0.5 rounded text-[10px] font-mono font-bold transition-all ${
                        drMode === 'dr'
                          ? 'bg-emerald-950/40 text-emerald-400 border border-emerald-500/40'
                          : 'bg-zinc-800 text-zinc-400 border border-zinc-700 hover:text-zinc-200'
                      }`}
                    >
                      Full DR
                    </button>
                    <button
                      type="button"
                      onClick={() => setDrMode('settings')}
                      className={`px-2 py-0.5 rounded text-[10px] font-mono font-bold transition-all ${
                        drMode === 'settings'
                          ? 'bg-zinc-700 text-zinc-200 border border-zinc-600'
                          : 'bg-zinc-800 text-zinc-400 border border-zinc-700 hover:text-zinc-200'
                      }`}
                    >
                      Settings Only
                    </button>
                  </div>
                </div>

                {drMode === 'dr' ? (
                  <>
                    <p className="text-[11px] text-slate-400 leading-relaxed">
                      Full disaster recovery archive: all 21 Catapult configs, TLS certificates, and unredacted keys encrypted with Argon2id + AES-256-GCM.
                    </p>

                    <div className="space-y-1">
                      <div className="flex items-center justify-between text-[11px] text-slate-300">
                        <span className="flex items-center space-x-1">
                          <Lock className="w-3 h-3 text-emerald-400" />
                          <span>Passphrase:</span>
                        </span>
                        <span className="text-[9.5px] text-slate-500 font-mono">Min 8 chars</span>
                      </div>
                      <div className="relative">
                        <input
                          type={showDrPassphrase ? 'text' : 'password'}
                          value={drPassphrase}
                          onChange={(e) => setDrPassphrase(e.target.value)}
                          placeholder="Enter encryption passphrase..."
                          className="w-full bg-[#0F1115] border border-[#262B34] focus:border-zinc-500 rounded-lg px-2.5 py-1 text-xs text-white placeholder-slate-500 pr-8"
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
                      <div className="p-2 bg-emerald-950/30 border border-emerald-500/30 text-emerald-400 rounded-lg text-[11px] flex items-center space-x-1.5 truncate">
                        <CheckCircle2 className="w-3 h-3 flex-shrink-0" />
                        <span className="truncate">{drMessage}</span>
                      </div>
                    )}
                    {drError && (
                      <div className="p-2 bg-amber-950/30 border border-amber-500/30 text-amber-400 rounded-lg text-[11px] flex items-center space-x-1.5 truncate">
                        <AlertTriangle className="w-3 h-3 flex-shrink-0" />
                        <span className="truncate">{drError}</span>
                      </div>
                    )}
                  </>
                ) : (
                  <>
                    <p className="text-[11px] text-slate-400 leading-relaxed">
                      Export portable non-sensitive settings (friendly name, ports, custom peer lists, and harvest stats).
                    </p>
                    <div className="p-2 bg-zinc-800/60 border border-zinc-700 rounded-lg text-[10.5px] leading-relaxed text-zinc-300 flex items-start space-x-1.5">
                      <AlertTriangle className="w-3.5 h-3.5 text-amber-400 flex-shrink-0 mt-0.5" />
                      <span>
                        Private keys (bootKey/harvestKey), certificates, and blockchain state are omitted from settings export.
                      </span>
                    </div>
                    {settingsMessage && (
                      <div className="p-2 bg-emerald-950/30 border border-emerald-500/30 text-emerald-400 rounded-lg text-[11px] flex items-center space-x-1.5 truncate">
                        <CheckCircle2 className="w-3 h-3 flex-shrink-0" />
                        <span className="truncate">{settingsMessage}</span>
                      </div>
                    )}
                  </>
                )}
              </div>

              <div className="pt-2 border-t border-[#262B34] flex items-center justify-between gap-2">
                {drMode === 'dr' ? (
                  <>
                    <label className="px-3 py-1.5 bg-[#181B20] hover:bg-[#262B34] text-slate-300 hover:text-white rounded-lg text-xs font-semibold border border-[#262B34] transition-colors cursor-pointer flex items-center space-x-1">
                      <FolderInput className="w-3 h-3" />
                      <span>{drRestoring ? 'Restoring...' : 'Restore Package'}</span>
                      <input type="file" accept=".drpkg,.json" onChange={handleImportDR} className="hidden" disabled={drRestoring} />
                    </label>
                    <button
                      type="button"
                      onClick={handleExportDR}
                      disabled={drExporting}
                      className="px-3.5 py-1.5 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg text-xs font-semibold shadow-xs border border-emerald-500/30 transition-all flex items-center space-x-1"
                    >
                      <DownloadCloud className="w-3 h-3" />
                      <span>{drExporting ? 'Encrypting...' : 'Export Package'}</span>
                    </button>
                  </>
                ) : (
                  <>
                    <label className="px-3 py-1.5 bg-[#181B20] hover:bg-[#262B34] text-slate-300 hover:text-white rounded-lg text-xs font-semibold border border-[#262B34] transition-colors cursor-pointer flex items-center space-x-1">
                      <FolderInput className="w-3 h-3" />
                      <span>{restoringSettings ? 'Importing...' : 'Import Settings'}</span>
                      <input type="file" accept=".json" onChange={handleImportSettings} className="hidden" disabled={restoringSettings} />
                    </label>
                    <button
                      type="button"
                      onClick={handleExportSettings}
                      className="px-3.5 py-1.5 bg-zinc-700 hover:bg-zinc-600 text-white rounded-lg text-xs font-semibold shadow-xs border border-zinc-600 transition-all flex items-center space-x-1"
                    >
                      <DownloadCloud className="w-3 h-3" />
                      <span>Export Settings</span>
                    </button>
                  </>
                )}
              </div>
            </div>

            {/* UTILITY 3: Storage Migration to Chunked Format */}
            <div className="bg-[#13171F] border border-[#262B34] rounded-xl p-4 flex flex-col justify-between space-y-3">
              <div className="space-y-2">
                <div className="flex items-center justify-between pb-1.5 border-b border-[#262B34]">
                  <div className="flex items-center space-x-2">
                    <Layers className="w-4 h-4 text-slate-300" />
                    <h3 className="text-xs font-semibold uppercase tracking-wider text-white">
                      Migrate Legacy Storage to Chunked Format
                    </h3>
                  </div>
                  {/* 3-State Badge */}
                  {isConvertRunning ? (
                    <span className="px-2 py-0.5 rounded font-mono text-[10px] font-bold uppercase bg-amber-950/40 text-amber-400 border border-amber-500/40">
                      Converting
                    </span>
                  ) : convertStatus?.status === 'completed' ? (
                    <span className="px-2 py-0.5 rounded font-mono text-[10px] font-bold uppercase bg-emerald-950/40 text-emerald-400 border border-emerald-500/40">
                      Complete
                    </span>
                  ) : (
                    <span className="px-2 py-0.5 rounded font-mono text-[10px] bg-zinc-800 text-zinc-300 border border-zinc-700">
                      Chunked
                    </span>
                  )}
                </div>

                <p className="text-[11px] text-slate-400 leading-relaxed">
                  Packs millions of loose legacy block files into compact 4-file chunks (<code className="font-mono text-emerald-400">blocks.dat</code>, <code className="font-mono text-emerald-400">statements.dat</code>, <code className="font-mono text-emerald-400">blocks.idx</code>) per 65k-block folder to drastically free filesystem inodes.
                </p>

                <DirectoryDropdown
                  label="Blockchain Data Path"
                  value={convertSourcePath}
                  onChange={setConvertSourcePath}
                  placeholder="./chainconfig/data"
                  prompt="Select Blockchain Data Directory"
                />

                {/* Progress Bar */}
                {isConvertRunning && (
                  <div className="p-2.5 bg-[#0F1115] rounded-lg border border-amber-500/40 space-y-1.5 animate-in fade-in duration-150">
                    <div className="flex justify-between text-[11px] font-mono">
                      <span className="text-amber-400 font-bold">
                        {convertStatus?.percent}% ({convertStatus?.convertedBlocks?.toLocaleString()} blocks)
                      </span>
                      <span className="text-slate-400 truncate max-w-[160px]">
                        Folder: {convertStatus?.currentDir}
                      </span>
                    </div>
                    <div className="w-full bg-[#181B20] h-1.5 rounded-full overflow-hidden">
                      <div
                        className="bg-amber-400 h-full rounded-full transition-all duration-300"
                        style={{ width: `${Math.max(5, convertStatus?.percent || 0)}%` }}
                      />
                    </div>
                    <div className="flex justify-between text-[10px] text-slate-400 font-mono">
                      <span>{convertStatus?.message}</span>
                      <span className="text-rose-400">Deleted: {convertStatus?.deletedFiles?.toLocaleString()} files</span>
                    </div>
                  </div>
                )}
              </div>

              <div className="pt-2 border-t border-[#262B34] flex items-center justify-between">
                <span className="text-[11px] text-slate-400 font-mono">
                  {isConvertRunning ? 'Migrating files in background...' : 'Zero-loss consolidation'}
                </span>
                {!isConvertRunning ? (
                  <button
                    type="button"
                    onClick={handleStartConvert}
                    disabled={startingConvert}
                    className="px-3.5 py-1.5 bg-zinc-700 hover:bg-zinc-600 disabled:opacity-50 text-white rounded-lg text-xs font-semibold shadow-xs border border-zinc-600 transition-all flex items-center space-x-1.5"
                  >
                    <Zap className="w-3.5 h-3.5" />
                    <span>{startingConvert ? 'Starting...' : 'Start Migration'}</span>
                  </button>
                ) : (
                  <button
                    type="button"
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

            {/* UTILITY 4: Clean Logs & Cache */}
            <div className="bg-[#13171F] border border-[#262B34] rounded-xl p-4 flex flex-col justify-between space-y-3">
              <div className="space-y-2">
                <div className="flex items-center justify-between pb-1.5 border-b border-[#262B34]">
                  <div className="flex items-center space-x-2">
                    <Trash2 className="w-4 h-4 text-slate-300" />
                    <h3 className="text-xs font-semibold uppercase tracking-wider text-white">
                      Clean Cache & Logs
                    </h3>
                  </div>
                  {/* 3-State Badge: Gray (Maintenance), Amber (Purging / High usage / Stale lock) */}
                  {cleaning ? (
                    <span className="px-2 py-0.5 rounded text-[10px] font-mono font-bold uppercase bg-amber-950/40 text-amber-400 border border-amber-500/40">
                      Purging...
                    </span>
                  ) : logStats?.serverLockFound ? (
                    <span className="px-2 py-0.5 rounded text-[10px] font-mono font-bold uppercase bg-amber-950/40 text-amber-400 border border-amber-500/40">
                      Stale Lock
                    </span>
                  ) : (logStats?.totalMB ?? 0) > 100 ? (
                    <span className="px-2 py-0.5 rounded text-[10px] font-mono font-bold uppercase bg-amber-950/40 text-amber-400 border border-amber-500/40">
                      {(logStats?.totalMB ?? 0).toFixed(0)} MB
                    </span>
                  ) : (
                    <span className="px-2 py-0.5 rounded text-[10px] font-mono bg-zinc-800 text-zinc-300 border border-zinc-700">
                      Maintenance
                    </span>
                  )}
                </div>

                <p className="text-[11px] text-slate-400 leading-relaxed">
                  Purges historical rotated log files and removes stale <code className="text-slate-300 font-mono">data/server.lock</code> handles to reclaim disk space and prevent restart lockouts.
                </p>

                {/* Telemetry & Purge Preview Box */}
                <div className="p-2.5 bg-[#0F1115] rounded-lg border border-[#262B34] space-y-1.5 text-[11px] font-mono">
                  <div className="flex items-center justify-between">
                    <span className="text-slate-400">Log Footprint:</span>
                    <span className="text-slate-200 font-medium">
                      {logStats ? `${logStats.logCount} file${logStats.logCount === 1 ? '' : 's'} (${logStats.totalMB.toFixed(1)} MB)` : 'Scanning logs...'}
                    </span>
                  </div>

                  <div className="flex items-center justify-between">
                    <span className="text-slate-400">Server Lockfile:</span>
                    {logStats?.serverLockFound ? (
                      <span className="text-amber-400 font-bold flex items-center space-x-1">
                        <AlertTriangle className="w-3 h-3" />
                        <span>stale server.lock found</span>
                      </span>
                    ) : (
                      <span className="text-emerald-400 font-medium flex items-center space-x-1">
                        <CheckCircle2 className="w-3 h-3" />
                        <span>clean (unlocked)</span>
                      </span>
                    )}
                  </div>

                  <div className="pt-1.5 border-t border-[#1C2028] flex items-center justify-between text-[10px]">
                    <span className="text-slate-400">Last Purge:</span>
                    <span className={logStats?.lastPurgeTime ? 'text-emerald-400' : 'text-slate-400'}>
                      {logStats?.lastPurgeTime
                        ? `${logStats.lastPurgeTime} (${(logStats.lastPurgeFreedMB ?? 0).toFixed(2)} MB freed)`
                        : 'Ready to purge'}
                    </span>
                  </div>
                </div>

                {cleanMessage && (
                  <div className="p-2 bg-emerald-950/30 border border-emerald-500/30 text-emerald-400 rounded-lg text-[11px] font-mono flex items-center space-x-1.5 truncate animate-in fade-in duration-150">
                    <CheckCircle2 className="w-3 h-3 flex-shrink-0" />
                    <span className="truncate">{cleanMessage}</span>
                  </div>
                )}
              </div>

              <div className="pt-2 border-t border-[#262B34] flex items-center justify-between">
                <span className="text-[11px] text-slate-400 font-mono">
                  Preserves active current log
                </span>
                <button
                  type="button"
                  onClick={handleCleanLogs}
                  disabled={cleaning}
                  className="px-3.5 py-1.5 bg-zinc-700 hover:bg-zinc-600 disabled:opacity-50 text-white rounded-lg text-xs font-semibold shadow-xs border border-zinc-600 transition-all flex items-center space-x-1.5"
                >
                  <Trash2 className="w-3.5 h-3.5" />
                  <span>{cleaning ? 'Cleaning...' : 'Purge Logs'}</span>
                </button>
              </div>
            </div>

          </div>
        </div>
      )}

      {/* PERSISTENT DANGER ZONE - Visible and accessible at all times regardless of active sub-tab (Requirement) */}
      <div className="pt-2 space-y-3">
        <div className="h-px bg-rose-900/40" />
        
        <div className="flex items-center space-x-2">
          <ShieldAlert className="w-4 h-4 text-rose-400" />
          <h2 className="text-xs font-bold uppercase tracking-wider text-rose-400">
            Danger Zone
          </h2>
        </div>

        {/* Distinct Destructive Card with Red/Danger Accent Styling */}
        <div className="border border-rose-900/50 bg-rose-950/10 rounded-xl p-4 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
          <div className="space-y-1 max-w-2xl">
            <div className="flex items-center space-x-2">
              <h3 className="text-xs font-semibold uppercase tracking-wider text-white">
                Nemesis Reset (Block 1)
              </h3>
              <span className="px-2 py-0.5 rounded text-[10px] font-mono font-bold uppercase bg-rose-950/50 text-rose-400 border border-rose-800/60">
                Irreversible Action
              </span>
            </div>
            <p className="text-[11px] text-slate-300 leading-relaxed">
              Preserves genesis Nemesis block (<code className="text-rose-300 font-mono">00001.dat</code>) while completely erasing all subsequent synced blockchain blocks, cache indexes, and transaction history. Forces node to restart synchronization from scratch at Block 1.
            </p>
            {resetDone && (
              <span className="text-[11px] text-emerald-400 font-mono block font-semibold pt-1">
                ✓ Reset complete. Blockchain is at Block 1 genesis state.
              </span>
            )}
          </div>

          <button
            type="button"
            onClick={triggerNemesisResetModal}
            disabled={resetting}
            className="px-4 py-2 bg-rose-600 hover:bg-rose-500 disabled:opacity-50 text-white rounded-lg text-xs font-semibold shadow-xs transition-all flex items-center space-x-1.5 flex-shrink-0"
          >
            <ShieldAlert className="w-3.5 h-3.5" />
            <span>{resetting ? 'Resetting...' : 'Reset to Genesis Block 1'}</span>
          </button>
        </div>
      </div>

      {/* Type-to-Confirm Modal for All Destructive Actions */}
      <ConfirmDestructiveModal
        isOpen={confirmModal.isOpen}
        onClose={() => setConfirmModal((prev) => ({ ...prev, isOpen: false }))}
        onConfirm={confirmModal.action}
        title={confirmModal.title}
        description={confirmModal.description}
        expectedText={confirmModal.expectedText}
        confirmButtonText={confirmModal.confirmButtonText}
        inProgress={resetting || startingRemoteSync || startingLocalRestore}
      />

      {/* Official Network Configurations Sync Modal */}
      <SyncConfigsModal
        isOpen={syncModalOpen}
        onClose={() => setSyncModalOpen(false)}
        updateInfo={updateInfo}
        onRefresh={fetchStatus}
      />

    </div>
  );
};
