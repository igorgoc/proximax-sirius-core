import React, { useState, useEffect } from 'react';
import { Globe2, X, RefreshCw, Radio, Server, Activity, ShieldCheck } from 'lucide-react';
import { PeersDetailResponse } from '../types';

interface PeersModalProps {
  isOpen: boolean;
  onClose: () => void;
  isRunning?: boolean;
}

export const PeersModal: React.FC<PeersModalProps> = ({ isOpen, onClose, isRunning = true }) => {
  const [data, setData] = useState<PeersDetailResponse | null>(null);
  const [loading, setLoading] = useState(false);

  const fetchPeers = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/network/peers-detail');
      if (res.ok) {
        const json = await res.json();
        setData(json);
      }
    } catch (e) {
      console.error(e);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (isOpen) {
      fetchPeers();
      const interval = setInterval(fetchPeers, 10000);
      return () => clearInterval(interval);
    }
  }, [isOpen]);

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-3 sm:p-5 bg-black/80 backdrop-blur-xs overflow-y-auto animate-fadeIn select-none">
      <div className="bg-[#181B20] rounded-xl border border-[#262B34] shadow-2xl w-full max-w-4xl max-h-[90vh] my-auto flex flex-col overflow-hidden text-slate-200">
        {/* Modal Header */}
        <div className="p-4 sm:p-5 border-b border-[#262B34] flex items-center justify-between bg-[#0F1115] flex-shrink-0">
          <div className="flex items-center space-x-3">
            <div className="p-1.5 bg-[#181B20] text-slate-300 rounded-md border border-[#262B34]">
              <Globe2 className="w-4 h-4" />
            </div>
            <div>
              <div className="flex items-center space-x-2">
                <h2 className="text-sm font-semibold text-white tracking-tight">
                  Global Peer Network &amp; Topology
                </h2>
                <span className={`text-[10px] font-mono px-2 py-0.5 rounded border ${
                  isRunning
                    ? 'bg-slate-800 text-slate-400 border-slate-700/50'
                    : 'bg-slate-800 text-slate-500 border-slate-700/30'
                }`}>
                  {isRunning ? 'P2P Mesh Active' : 'P2P Offline'}
                </span>
              </div>
              <p className="text-[11px] text-slate-400 mt-0.5">
                Sirius Mainnet bootstrap seeds health and connected P2P mesh peers.
              </p>
            </div>
          </div>

          <div className="flex items-center space-x-2">
            <button
              onClick={fetchPeers}
              disabled={loading}
              className="p-1.5 text-slate-400 hover:text-white rounded hover:bg-[#262B34] transition-colors"
              title="Refresh peer latency"
            >
              <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
            </button>
            <button
              onClick={onClose}
              className="p-1.5 text-slate-400 hover:text-white rounded hover:bg-[#262B34] transition-colors"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>

        {/* Modal Body */}
        <div className="p-5 space-y-4 overflow-y-auto flex-1 text-xs scrollbar-thin">
          {/* 1. Official Bootstrap Seed Nodes Latency Bar */}
          {(() => {
            const seedList = data?.seedNodes || [];
            const reachableCount = seedList.filter(s => s.status === 'Online').length;
            const reachablePercent = seedList.length > 0 ? Math.round((reachableCount / seedList.length) * 100) : 0;

            return (
              <div className="p-4 bg-[#0F1115] rounded-lg border border-[#262B34] space-y-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center space-x-2 font-medium text-white text-xs">
                    <Radio className="w-3.5 h-3.5 text-blue-400 animate-pulse" />
                    <span>Sirius Mainnet Bootstrap Seed Nodes Health</span>
                  </div>
                  <span className={`text-[10px] font-mono font-medium px-2 py-0.5 rounded border ${
                    reachablePercent >= 66
                      ? 'text-emerald-400 bg-emerald-950/40 border-emerald-800/40'
                      : reachablePercent > 0
                      ? 'text-amber-300 bg-amber-950/40 border-amber-800/40'
                      : 'text-rose-400 bg-rose-950/40 border-rose-800/40'
                  }`}>
                    ● {reachableCount}/{seedList.length} Reachable ({reachablePercent}%)
                  </span>
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-3 gap-2.5">
                  {seedList.map((seed) => {
                    const name = seed.endpoint.replace('http://', '').replace(':3000', '');
                    const isOnline = seed.status === 'Online';
                    return (
                      <div
                        key={seed.endpoint}
                        className="p-2.5 bg-[#181B20] rounded-lg border border-[#262B34] hover:border-slate-600 transition-colors flex items-center justify-between"
                      >
                        <div>
                          <span className="font-semibold text-white capitalize block text-xs">{name}</span>
                          <span className="text-[10px] text-slate-400 font-mono block truncate max-w-[140px]">{seed.endpoint}</span>
                        </div>
                        <div className="text-right">
                          <span className={`font-mono font-semibold text-xs ${
                            !isOnline
                              ? 'text-rose-400'
                              : seed.latencyMs < 50
                              ? 'text-emerald-400'
                              : seed.latencyMs < 120
                              ? 'text-blue-400'
                              : 'text-amber-400'
                          }`}>
                            {isOnline && seed.latencyMs > 0 ? `${seed.latencyMs} ms` : '—'}
                          </span>
                          <span className={`text-[10px] font-medium block font-mono ${isOnline ? 'text-emerald-400' : 'text-rose-400'}`}>
                            {isOnline ? 'Online' : 'Offline'}
                          </span>
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
            );
          })()}

          {/* 2. Connected Mainnet Peers List */}
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <div className="flex items-center space-x-2 font-medium text-slate-300 text-xs">
                <Server className="w-3.5 h-3.5 text-slate-400" />
                <span>Connected P2P Nodes ({isRunning ? (data?.peers?.length || 0) : 0})</span>
              </div>
              <span className="text-[10px] text-slate-500 font-mono">Port 7900 / 7903 Protocol</span>
            </div>

            {isRunning && data?.peers && data.peers.length > 0 ? (
              <div className="border border-[#262B34] rounded-lg overflow-hidden shadow-xs bg-[#181B20]">
                <div className="overflow-x-auto">
                  <table className="w-full text-left border-collapse text-xs">
                    <thead>
                      <tr className="bg-[#0F1115] text-[10px] text-slate-400 font-medium uppercase border-b border-[#262B34]">
                        <th className="p-2.5">Node Host / IP</th>
                        <th className="p-2.5">Friendly Name</th>
                        <th className="p-2.5">Port</th>
                        <th className="p-2.5">Roles</th>
                        <th className="p-2.5">Version</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-[#262B34]/60 font-mono text-[11px]">
                      {data.peers.map((peer, idx) => (
                        <tr key={idx} className="hover:bg-[#1E2228] transition-colors">
                          <td className="p-2.5 font-medium text-white flex items-center space-x-1.5">
                            <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 inline-block"></span>
                            <span>{peer.host || 'P2P Node'}</span>
                          </td>
                          <td className="p-2.5 text-blue-400 font-medium">
                            {peer.friendlyName || 'mainnet-peer'}
                          </td>
                          <td className="p-2.5 text-slate-400">{peer.port || 7900}</td>
                          <td className="p-2.5">
                            <span className="px-1.5 py-0.5 rounded bg-slate-800 text-slate-300 border border-slate-700/50 text-[10px] font-sans">
                              {peer.roles === 3 ? 'Peer + API' : peer.roles === 2 ? 'API Node' : 'Peer'}
                            </span>
                          </td>
                          <td className="p-2.5 text-slate-400 text-[10px]">
                            v{peer.version ? `${(peer.version >> 16) & 0xFF}.${(peer.version >> 8) & 0xFF}.${peer.version & 0xFF}` : '1.9.6'}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            ) : (
              <div className="p-8 text-center bg-[#0F1115] rounded-lg border border-[#262B34] text-slate-400">
                {!isRunning ? (
                  <p className="text-slate-400">Local node is currently offline. Start the node to connect to peers.</p>
                ) : (
                  <>
                    <Activity className="w-5 h-5 mx-auto mb-2 opacity-50 animate-pulse text-blue-400" />
                    <p>Connected to P2P network, discovering mesh peers...</p>
                  </>
                )}
              </div>
            )}
          </div>
        </div>

        {/* Modal Footer */}
        <div className="p-3 bg-[#0F1115] border-t border-[#262B34] flex justify-between items-center text-[11px] text-slate-400 flex-shrink-0">
          <div className="flex items-center space-x-1.5 font-mono">
            <ShieldCheck className={`w-3.5 h-3.5 ${isRunning ? 'text-emerald-400' : 'text-slate-500'}`} />
            <span className={isRunning ? 'text-emerald-400' : 'text-slate-500'}>
              {isRunning ? 'Encrypted Dual TLS & Curve25519 Transport Active' : 'P2P Transport Stopped • Node Offline'}
            </span>
          </div>
          <button
            onClick={onClose}
            className="px-3.5 py-1.5 bg-[#181B20] hover:bg-[#262B34] text-slate-200 rounded font-medium text-xs border border-[#262B34] transition-colors"
          >
            Close
          </button>
        </div>
      </div>
    </div>
  );
};
