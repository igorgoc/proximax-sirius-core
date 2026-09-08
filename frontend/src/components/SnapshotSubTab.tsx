import React, { useState, useEffect } from 'react';
import { 
  Archive, 
  DownloadCloud, 
  UploadCloud, 
  ShieldCheck, 
  AlertTriangle, 
  RefreshCw, 
  HardDrive, 
  CheckCircle2, 
  Layers, 
  Lock, 
  FileText, 
  Play, 
  Flame,
  Check
} from 'lucide-react';
import { DirectoryDropdown } from './DirectoryDropdown';
import { SnapshotModal } from './SnapshotModal';
import { SnapshotManagerStatus } from '../types';

interface SnapshotSubTabProps {
  currentDataPath: string;
  blockHeight?: number;
  onRefreshConfig?: () => void;
}

export const SnapshotSubTab: React.FC<SnapshotSubTabProps> = ({ currentDataPath, blockHeight = 0 }) => {
  // Part 1: Local Create State
  const [createSourcePath, setCreateSourcePath] = useState(currentDataPath || './chainconfig/data');
  const [createTargetPath, setCreateTargetPath] = useState('');
  const [createFormat, setCreateFormat] = useState<'tar.zst' | 'tar.gz'>('tar.zst');
  const [creating, setCreating] = useState(false);

  // Part 1: Local Restore State
  const [localArchivePath, setLocalArchivePath] = useState('');
  const [localRestoreTarget, setLocalRestoreTarget] = useState(currentDataPath || './chainconfig/data');
  const [restoringLocal, setRestoringLocal] = useState(false);

  // Part 2: Pluggable Remote Restore State
  const [manifestUrl, setManifestUrl] = useState('');
  const [releasePubKey, setReleasePubKey] = useState('538eefb498971db790422d53d24aa1ed2623e37298ef6c9dfd436b739cf5aa3c');
  const [remoteRestoreTarget, setRemoteRestoreTarget] = useState(currentDataPath || './chainconfig/data');
  const [restoringRemote, setRestoringRemote] = useState(false);

  // Live Manager Status
  const [snapshotStatus, setSnapshotStatus] = useState<SnapshotManagerStatus | null>(null);
  const [showModal, setShowModal] = useState(false);

  // Poll live snapshot status
  useEffect(() => {
    let timer: any;
    const fetchStatus = async () => {
      try {
        const res = await fetch('/api/snapshot/status');
        if (res.ok) {
          const data: SnapshotManagerStatus = await res.json();
          setSnapshotStatus(data);
          if (data && data.stage !== 'idle') {
            setShowModal(true);
          }
        }
      } catch (e) {
        console.error(e);
      }
    };

    fetchStatus();
    timer = setInterval(fetchStatus, 1000);
    return () => clearInterval(timer);
  }, []);

  // Handler: Part 1 Local Create
  const handleCreateSnapshot = async () => {
    if (!createSourcePath) {
      alert('Source data directory cannot be empty');
      return;
    }
    setCreating(true);
    try {
      const res = await fetch('/api/snapshot/create', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          sourcePath: createSourcePath.trim(),
          targetPath: createTargetPath.trim(),
          format: createFormat,
        }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to start snapshot creation');
      setShowModal(true);
    } catch (err: any) {
      alert(err.message);
    } finally {
      setCreating(false);
    }
  };

  // Handler: Part 1 Local Restore
  const handleRestoreLocal = async () => {
    if (!localArchivePath) {
      alert('Please select a local snapshot archive file (.tar.zst, .tar.gz, .tar.xz)');
      return;
    }
    if (!window.confirm(`Warning: Restoring this snapshot will overwrite blockchain state in:\n${localRestoreTarget}\n\nThe node will be stopped during restoration. Continue?`)) {
      return;
    }

    setRestoringLocal(true);
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
      if (!res.ok) throw new Error(data.error || 'Failed to start local snapshot restore');
      setShowModal(true);
    } catch (err: any) {
      alert(err.message);
    } finally {
      setRestoringLocal(false);
    }
  };

  // Handler: Part 2 Pluggable Remote Restore
  const handleRestoreRemote = async () => {
    if (!manifestUrl) {
      alert('Please enter a valid snapshot manifest.json URL');
      return;
    }
    if (!window.confirm(`Restore remote snapshot into:\n${remoteRestoreTarget}\n\nThe manifest signature will be verified before streaming. Node will stop. Proceed?`)) {
      return;
    }

    setRestoringRemote(true);
    try {
      const res = await fetch('/api/snapshot/restore/remote', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          manifestUrl: manifestUrl.trim(),
          releasePublicKeyHex: releasePubKey.trim(),
          targetDataPath: remoteRestoreTarget.trim(),
        }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to start remote snapshot restoration');
      setShowModal(true);
    } catch (err: any) {
      alert(err.message);
    } finally {
      setRestoringRemote(false);
    }
  };

  // Handler: Test with Mock Server
  const handleLoadMockServer = async () => {
    try {
      const res = await fetch('/api/snapshot/mock/info');
      if (!res.ok) throw new Error('Failed to load mock server info');
      const data = await res.json();
      setManifestUrl(data.manifestUrl);
      if (data.mockPublicKeyHex) {
        setReleasePubKey(data.mockPublicKeyHex);
      }
    } catch (err: any) {
      alert(err.message);
    }
  };

  // Cancel running operation
  const handleCancel = async () => {
    try {
      await fetch('/api/snapshot/cancel', { method: 'POST' });
    } catch (e) {
      console.error(e);
    }
  };

  return (
    <div className="space-y-6 animate-in fade-in duration-150">
      
      {/* Visual Status Badges */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
        <div className="p-3.5 bg-[#13171F] border border-emerald-500/30 rounded-xl flex items-start space-x-3">
          <div className="p-2 bg-emerald-500/10 text-emerald-400 rounded-lg flex-shrink-0 mt-0.5">
            <CheckCircle2 className="w-4 h-4" />
          </div>
          <div className="space-y-0.5">
            <div className="flex items-center space-x-2">
              <h4 className="text-xs font-semibold text-white">Part 1: Local Snapshot Engine</h4>
              <span className="px-2 py-0.2 bg-emerald-500/20 text-emerald-300 border border-emerald-500/30 rounded text-[10px] font-mono">
                Operational & Verified
              </span>
            </div>
            <p className="text-[11px] text-slate-400 leading-relaxed">
              Multi-threaded Zstandard compression/decompression directly to/from disk with zero remote dependencies.
            </p>
          </div>
        </div>

        <div className="p-3.5 bg-[#13171F] border border-amber-500/30 rounded-xl flex items-start space-x-3">
          <div className="p-2 bg-amber-500/10 text-amber-400 rounded-lg flex-shrink-0 mt-0.5">
            <AlertTriangle className="w-4 h-4" />
          </div>
          <div className="space-y-0.5">
            <div className="flex items-center space-x-2">
              <h4 className="text-xs font-semibold text-white">Part 2: Pluggable Remote Sync</h4>
              <span className="px-2 py-0.2 bg-amber-500/20 text-amber-300 border border-amber-500/30 rounded text-[10px] font-mono">
                Mock Tested (Unverified on Cloud)
              </span>
            </div>
            <p className="text-[11px] text-slate-400 leading-relaxed">
              Backend-agnostic manifest (R2 / S3 / B2) with Ed25519 signature & streaming SHA-256 verification.
            </p>
          </div>
        </div>
      </div>

      {/* 3 Main Functional Cards */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        
        {/* Card 1: Create Local Snapshot */}
        <div className="bg-[#13171F] border border-[#262B34] rounded-xl p-4 flex flex-col justify-between space-y-4">
          <div className="space-y-3">
            <div className="flex items-center justify-between pb-2 border-b border-[#262B34]">
              <div className="flex items-center space-x-2">
                <UploadCloud className="w-4 h-4 text-emerald-400" />
                <h4 className="text-xs font-semibold text-white">Create Local Snapshot</h4>
              </div>
              <span className="text-[10px] font-mono text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded">
                Height: {blockHeight ? blockHeight.toLocaleString() : 'Live Tip'}
              </span>
            </div>

            <p className="text-[11px] text-slate-400 leading-relaxed">
              Package your active blockchain data into an archive. Generates a SHA-256 manifest alongside the file.
            </p>

            <div className="space-y-2 pt-1">
              <DirectoryDropdown
                label="Source Data Directory"
                value={createSourcePath}
                onChange={setCreateSourcePath}
                placeholder="./chainconfig/data"
                prompt="Select Source Blockchain Directory"
              />

              <DirectoryDropdown
                label="Destination Folder"
                value={createTargetPath}
                onChange={setCreateTargetPath}
                placeholder="/Volumes/SSD/snapshots"
                prompt="Select Snapshot Save Directory"
              />

              <div>
                <label className="text-[11px] font-medium text-slate-400 block mb-1">Compression Format</label>
                <div className="grid grid-cols-2 gap-2">
                  <button
                    type="button"
                    onClick={() => setCreateFormat('tar.zst')}
                    className={`py-1.5 px-2 rounded-lg text-xs font-mono font-medium transition-all ${
                      createFormat === 'tar.zst'
                        ? 'bg-emerald-600 text-white shadow-xs'
                        : 'bg-[#181B20] text-slate-400 hover:text-white border border-[#262B34]'
                    }`}
                  >
                    .tar.zst (Zstandard)
                  </button>
                  <button
                    type="button"
                    onClick={() => setCreateFormat('tar.gz')}
                    className={`py-1.5 px-2 rounded-lg text-xs font-mono font-medium transition-all ${
                      createFormat === 'tar.gz'
                        ? 'bg-emerald-600 text-white shadow-xs'
                        : 'bg-[#181B20] text-slate-400 hover:text-white border border-[#262B34]'
                    }`}
                  >
                    .tar.gz (Gzip)
                  </button>
                </div>
              </div>
            </div>
          </div>

          <button
            type="button"
            onClick={handleCreateSnapshot}
            disabled={creating || snapshotStatus?.stage === 'archiving'}
            className="w-full py-2 bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white rounded-xl text-xs font-semibold shadow-xs transition-all flex items-center justify-center space-x-1.5"
          >
            <Archive className="w-3.5 h-3.5" />
            <span>{creating ? 'Initializing...' : 'Generate Snapshot Archive'}</span>
          </button>
        </div>

        {/* Card 2: Restore from Local Archive */}
        <div className="bg-[#13171F] border border-[#262B34] rounded-xl p-4 flex flex-col justify-between space-y-4">
          <div className="space-y-3">
            <div className="flex items-center justify-between pb-2 border-b border-[#262B34]">
              <div className="flex items-center space-x-2">
                <HardDrive className="w-4 h-4 text-blue-400" />
                <h4 className="text-xs font-semibold text-white">Restore Local Archive</h4>
              </div>
              <span className="text-[10px] font-mono text-blue-400 bg-blue-500/10 px-2 py-0.5 rounded">
                Direct Unpack
              </span>
            </div>

            <p className="text-[11px] text-slate-400 leading-relaxed">
              Extract an existing archive (<code className="text-slate-300">.tar.zst</code>, <code className="text-slate-300">.tar.gz</code>, <code className="text-slate-300">.tar.xz</code>) into your active data directory.
            </p>

            <div className="space-y-2 pt-1">
              <DirectoryDropdown
                label="Select Local Archive"
                value={localArchivePath}
                onChange={setLocalArchivePath}
                placeholder="/Volumes/SSD/snapshot.tar.zst"
                mode="file"
                prompt="Select Snapshot Archive File"
              />

              <DirectoryDropdown
                label="Target Data Directory"
                value={localRestoreTarget}
                onChange={setLocalRestoreTarget}
                placeholder="./chainconfig/data"
                prompt="Select Target Data Directory"
              />
            </div>
          </div>

          <button
            type="button"
            onClick={handleRestoreLocal}
            disabled={restoringLocal || snapshotStatus?.stage === 'extracting'}
            className="w-full py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded-xl text-xs font-semibold shadow-xs transition-all flex items-center justify-center space-x-1.5"
          >
            <Play className="w-3.5 h-3.5" />
            <span>{restoringLocal ? 'Extracting...' : 'Restore from Local File'}</span>
          </button>
        </div>

        {/* Card 3: Pluggable Remote Snapshot Sync */}
        <div className="bg-[#13171F] border border-[#262B34] rounded-xl p-4 flex flex-col justify-between space-y-4">
          <div className="space-y-3">
            <div className="flex items-center justify-between pb-2 border-b border-[#262B34]">
              <div className="flex items-center space-x-2">
                <DownloadCloud className="w-4 h-4 text-indigo-400" />
                <h4 className="text-xs font-semibold text-white">Restore Remote Snapshot</h4>
              </div>
              <button
                type="button"
                onClick={handleLoadMockServer}
                className="text-[10px] font-mono text-indigo-400 hover:text-indigo-300 bg-indigo-500/10 px-2 py-0.5 rounded flex items-center space-x-1 transition-all"
                title="Populate test URL pointing to embedded mock server"
              >
                <RefreshCw className="w-2.5 h-2.5" />
                <span>Test Mock URL</span>
              </button>
            </div>

            <p className="text-[11px] text-slate-400 leading-relaxed">
              Streams verified archive from generic HTTPS URL (Cloudflare R2, S3, B2). Verifies Ed25519 signature and SHA-256 on the fly.
            </p>

            <div className="space-y-2 pt-1">
              <div>
                <label className="text-[11px] font-medium text-slate-400 block mb-1">Manifest URL (.json)</label>
                <input
                  type="text"
                  value={manifestUrl}
                  onChange={(e) => setManifestUrl(e.target.value)}
                  placeholder="https://pub-xxx.r2.dev/manifest.json"
                  className="w-full px-3 py-1.5 bg-[#181B20] border border-[#262B34] rounded-lg text-xs text-white font-mono focus:outline-none focus:border-indigo-500 transition-all"
                />
              </div>

              <div>
                <label className="text-[11px] font-medium text-slate-400 block mb-1">Release Public Key (Ed25519)</label>
                <input
                  type="text"
                  value={releasePubKey}
                  onChange={(e) => setReleasePubKey(e.target.value)}
                  className="w-full px-3 py-1.5 bg-[#181B20] border border-[#262B34] rounded-lg text-[11px] text-slate-300 font-mono focus:outline-none focus:border-indigo-500 transition-all"
                />
              </div>

              <DirectoryDropdown
                label="Target Data Directory"
                value={remoteRestoreTarget}
                onChange={setRemoteRestoreTarget}
                placeholder="./chainconfig/data"
                prompt="Select Target Data Directory"
              />
            </div>
          </div>

          <button
            type="button"
            onClick={handleRestoreRemote}
            disabled={restoringRemote || snapshotStatus?.stage === 'downloading'}
            className="w-full py-2 bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white rounded-xl text-xs font-semibold shadow-xs transition-all flex items-center justify-center space-x-1.5"
          >
            <ShieldCheck className="w-3.5 h-3.5" />
            <span>{restoringRemote ? 'Verifying & Streaming...' : 'Verify & Restore Remote Snapshot'}</span>
          </button>
        </div>

      </div>

      {/* Progress & Verification Modal */}
      {showModal && (
        <SnapshotModal
          status={snapshotStatus}
          onCancel={handleCancel}
          onClose={() => setShowModal(false)}
        />
      )}

    </div>
  );
};
