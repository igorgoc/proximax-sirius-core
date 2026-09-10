import React, { useState, useEffect } from 'react';
import {
  Save,
  CheckCircle2,
  AlertTriangle,
  RotateCcw,
  Sliders,
  Folder,
  Globe,
  Key,
  Archive,
  ShieldCheck,
} from 'lucide-react';
import { DirectoryDropdown } from './DirectoryDropdown';

interface SnapshotSubTabProps {
  currentDataPath: string;
  blockHeight?: number;
  onRefreshConfig?: () => void;
}

export interface SnapshotPreferences {
  defaultDataPath: string;
  defaultSnapshotFolder: string;
  defaultCompressionFormat: 'tar.zst' | 'tar.gz';
  defaultRemoteUrl: string;
  releasePubKey: string;
}

const STORAGE_KEY = 'sirius_snapshot_prefs';

export const DEFAULT_SNAPSHOT_PREFS: SnapshotPreferences = {
  defaultDataPath: './chainconfig/data',
  defaultSnapshotFolder: '/Volumes/SSD/snapshots',
  defaultCompressionFormat: 'tar.zst',
  defaultRemoteUrl: 'https://huggingface.co/datasets/igorgoc/sirius-snapshot/resolve/main/manifest.json',
  releasePubKey: '68b1a927c47850a5d4244305b2670960d9fe6f048e5a458d801959b606f3e917',
};

export const loadSnapshotPreferences = (fallbackDataPath?: string): SnapshotPreferences => {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw);
      return {
        ...DEFAULT_SNAPSHOT_PREFS,
        ...parsed,
        defaultDataPath: parsed.defaultDataPath || fallbackDataPath || DEFAULT_SNAPSHOT_PREFS.defaultDataPath,
      };
    }
  } catch (e) {
    console.error('Failed to load snapshot preferences from storage:', e);
  }
  return {
    ...DEFAULT_SNAPSHOT_PREFS,
    defaultDataPath: fallbackDataPath || DEFAULT_SNAPSHOT_PREFS.defaultDataPath,
  };
};

export const SnapshotSubTab: React.FC<SnapshotSubTabProps> = ({ currentDataPath }) => {
  const [prefs, setPrefs] = useState<SnapshotPreferences>(() => loadSnapshotPreferences(currentDataPath));
  const [savedSuccess, setSavedSuccess] = useState(false);
  const [isDirty, setIsDirty] = useState(false);

  useEffect(() => {
    if (currentDataPath && !isDirty && prefs.defaultDataPath !== currentDataPath) {
      setPrefs((prev) => ({ ...prev, defaultDataPath: currentDataPath }));
    }
  }, [currentDataPath, isDirty, prefs.defaultDataPath]);

  const handleUpdate = <K extends keyof SnapshotPreferences>(key: K, value: SnapshotPreferences[K]) => {
    setPrefs((prev) => ({ ...prev, [key]: value }));
    setIsDirty(true);
    setSavedSuccess(false);
  };

  const handleSave = () => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(prefs));
      window.dispatchEvent(new CustomEvent('sirius-snapshot-prefs-updated', { detail: prefs }));
      setIsDirty(false);
      setSavedSuccess(true);
      setTimeout(() => setSavedSuccess(false), 3000);
    } catch (e) {
      console.error('Failed to save snapshot preferences:', e);
      alert('Failed to save snapshot preferences to browser storage.');
    }
  };

  const handleResetDefaults = () => {
    if (window.confirm('Reset all snapshot and backup settings to default values?')) {
      const resetVals = {
        ...DEFAULT_SNAPSHOT_PREFS,
        defaultDataPath: currentDataPath || DEFAULT_SNAPSHOT_PREFS.defaultDataPath,
      };
      setPrefs(resetVals);
      setIsDirty(true);
      setSavedSuccess(false);
    }
  };

  return (
    <div className="space-y-6 animate-in fade-in duration-150">
      {/* Section Header & Subtitle */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between pb-3 border-b border-[#262B34] gap-2">
        <div className="space-y-1">
          <div className="flex items-center space-x-2">
            <Sliders className="w-4 h-4 text-slate-300" />
            <h3 className="text-sm font-semibold uppercase tracking-wider text-white">
              Snapshot & Fast-Sync Defaults
            </h3>
          </div>
          <p className="text-xs text-slate-400">
            Configure system-wide default storage locations, compression algorithms, and remote sync endpoints.
          </p>
        </div>

        {/* Security & Engine Badges */}
        <div className="flex items-center space-x-2">
          <span className="px-2 py-0.5 rounded text-[11px] font-mono bg-zinc-800/80 text-zinc-300 border border-zinc-700">
            Engine: Native Fast-Sync
          </span>
          <span className="px-2 py-0.5 rounded text-[11px] font-mono bg-emerald-950/40 text-emerald-400 border border-emerald-500/40 flex items-center space-x-1">
            <ShieldCheck className="w-3 h-3" />
            <span>Ed25519 Background Verified</span>
          </span>
        </div>
      </div>

      {/* Main Configuration Card */}
      <div className="bg-[#13171F] border border-[#262B34] rounded-xl p-5 space-y-5">
        
        {/* Row 1: Default Paths */}
        <div className="space-y-3">
          <div className="flex items-center space-x-2 text-slate-200 font-medium text-xs">
            <Folder className="w-4 h-4 text-slate-400" />
            <span>Filesystem Directories</span>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <DirectoryDropdown
                label="Default Blockchain Data Directory (Source/Target)"
                value={prefs.defaultDataPath}
                onChange={(val) => handleUpdate('defaultDataPath', val)}
                placeholder="./chainconfig/data"
                prompt="Select Default Blockchain Data Directory"
              />
              <span className="text-[11px] text-slate-500 mt-1 block">
                Primary data folder scanned during backups and populated during fast-sync.
              </span>
            </div>

            <div>
              <DirectoryDropdown
                label="Default Snapshot & Backup Folder"
                value={prefs.defaultSnapshotFolder}
                onChange={(val) => handleUpdate('defaultSnapshotFolder', val)}
                placeholder="/Volumes/SSD/snapshots"
                prompt="Select Default Snapshot Storage Directory"
              />
              <span className="text-[11px] text-slate-500 mt-1 block">
                Default location where new archives and downloaded snapshots will be saved.
              </span>
            </div>
          </div>
        </div>

        <div className="h-px bg-[#262B34]" />

        {/* Row 2: Default Compression Format */}
        <div className="space-y-3">
          <div className="flex items-center space-x-2 text-slate-200 font-medium text-xs">
            <Archive className="w-4 h-4 text-slate-400" />
            <span>Default Compression Format</span>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 max-w-xl">
            <button
              type="button"
              onClick={() => handleUpdate('defaultCompressionFormat', 'tar.zst')}
              className={`p-3 rounded-lg border text-left transition-all ${
                prefs.defaultCompressionFormat === 'tar.zst'
                  ? 'bg-emerald-950/20 border-emerald-500/50 text-white'
                  : 'bg-[#181B20] border-[#262B34] text-slate-400 hover:text-white'
              }`}
            >
              <div className="flex items-center justify-between mb-1">
                <span className="text-xs font-mono font-bold">.tar.zst (Zstandard)</span>
                {prefs.defaultCompressionFormat === 'tar.zst' && (
                  <span className="px-1.5 py-0.5 rounded text-[10px] font-mono bg-emerald-950/40 text-emerald-400 border border-emerald-500/40">
                    Active Default
                  </span>
                )}
              </div>
              <p className="text-[11px] text-slate-400 leading-relaxed">
                Multi-threaded Zstandard compression. 5x faster backup and decompression with high ratio.
              </p>
            </button>

            <button
              type="button"
              onClick={() => handleUpdate('defaultCompressionFormat', 'tar.gz')}
              className={`p-3 rounded-lg border text-left transition-all ${
                prefs.defaultCompressionFormat === 'tar.gz'
                  ? 'bg-emerald-950/20 border-emerald-500/50 text-white'
                  : 'bg-[#181B20] border-[#262B34] text-slate-400 hover:text-white'
              }`}
            >
              <div className="flex items-center justify-between mb-1">
                <span className="text-xs font-mono font-bold">.tar.gz (Gzip)</span>
                {prefs.defaultCompressionFormat === 'tar.gz' && (
                  <span className="px-1.5 py-0.5 rounded text-[10px] font-mono bg-emerald-950/40 text-emerald-400 border border-emerald-500/40">
                    Active Default
                  </span>
                )}
              </div>
              <p className="text-[11px] text-slate-400 leading-relaxed">
                Universal standard gzip format. Broad compatibility with legacy archive extractors.
              </p>
            </button>
          </div>
        </div>

        <div className="h-px bg-[#262B34]" />

        {/* Row 3: Remote Snapshot & Signature Verification */}
        <div className="space-y-3">
          <div className="flex items-center space-x-2 text-slate-200 font-medium text-xs">
            <Globe className="w-4 h-4 text-slate-400" />
            <span>Remote Sync & Verification</span>
          </div>

          <div className="space-y-3">
            <div>
              <label className="text-xs font-medium text-slate-300 block mb-1">
                Default Snapshot / Manifest URL
              </label>
              <input
                type="text"
                value={prefs.defaultRemoteUrl}
                onChange={(e) => handleUpdate('defaultRemoteUrl', e.target.value)}
                placeholder="https://huggingface.co/datasets/igorgoc/sirius-snapshot/resolve/main/manifest.json"
                className="w-full px-3 py-2 bg-[#0F1115] border border-[#262B34] focus:border-zinc-500 rounded-lg text-xs font-mono text-white placeholder-slate-500 focus:outline-none transition-all"
              />
              <span className="text-[11px] text-slate-500 mt-1 block">
                Direct archive URL (.tar.xz, .tar.zst) or release manifest (.json) with checksums and signatures.
              </span>
            </div>

            <div className="p-3 bg-[#0F1115] rounded-lg border border-[#262B34] flex items-center justify-between text-xs">
              <div className="flex items-center space-x-2.5">
                <div className="w-7 h-7 rounded bg-emerald-950/50 border border-emerald-500/30 flex items-center justify-center text-emerald-400 shrink-0">
                  <ShieldCheck className="w-4 h-4" />
                </div>
                <div>
                  <div className="font-medium text-slate-200">Cryptographic Signature Verification</div>
                  <div className="text-[11px] text-slate-400">
                    Release manifests and snapshot archives are cryptographically verified in the background using official Ed25519 signatures and SHA-256 before extraction.
                  </div>
                </div>
              </div>
              <span className="px-2 py-0.5 rounded text-[10px] font-mono bg-emerald-950/40 text-emerald-400 border border-emerald-500/30 shrink-0 ml-3">
                Fail-Closed Enforced
              </span>
            </div>
          </div>
        </div>

      </div>

      {/* Action Bar (Save & Reset Defaults) */}
      <div className="bg-[#181B20] border border-[#262B34] rounded-xl p-4 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div className="flex items-center space-x-2 text-xs">
          {savedSuccess ? (
            <div className="flex items-center space-x-1.5 text-emerald-400 font-medium">
              <CheckCircle2 className="w-4 h-4" />
              <span>Snapshot preferences saved successfully.</span>
            </div>
          ) : isDirty ? (
            <div className="flex items-center space-x-1.5 text-amber-400 font-medium">
              <AlertTriangle className="w-4 h-4" />
              <span>You have unsaved snapshot configuration changes.</span>
            </div>
          ) : (
            <div className="flex items-center space-x-1.5 text-slate-500">
              <CheckCircle2 className="w-4 h-4" />
              <span>Snapshot preferences are current.</span>
            </div>
          )}
        </div>

        <div className="flex items-center space-x-3">
          <button
            type="button"
            onClick={handleResetDefaults}
            className="px-3 py-1.5 bg-[#13171F] hover:bg-[#262B34] text-slate-400 hover:text-white rounded-lg text-xs font-semibold border border-[#262B34] transition-all flex items-center space-x-1.5"
          >
            <RotateCcw className="w-3.5 h-3.5" />
            <span>Reset Defaults</span>
          </button>

          <button
            type="button"
            onClick={handleSave}
            disabled={!isDirty}
            className="px-4 py-1.5 bg-emerald-600 hover:bg-emerald-500 disabled:opacity-40 disabled:cursor-not-allowed text-white rounded-lg text-xs font-semibold shadow-xs transition-all flex items-center space-x-1.5"
          >
            <Save className="w-3.5 h-3.5" />
            <span>Save Snapshot Settings</span>
          </button>
        </div>
      </div>
    </div>
  );
};
