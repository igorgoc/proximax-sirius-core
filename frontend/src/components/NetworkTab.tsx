import React, { useState, useEffect, useMemo } from 'react';
import { 
  Globe2, 
  Radio, 
  RefreshCw, 
  Copy, 
  Check, 
  ExternalLink, 
  HardDrive, 
  Search, 
  AlertCircle
} from 'lucide-react';
import { NodeMetrics, NodeConfig, StorageStatus, PortCheckResult, PeersDetailResponse, PeerInfo, SeedPingInfo } from '../types';
import { getExplorerPublicKeyUrl } from '../utils/explorer';

interface NetworkTabProps {
  metrics: NodeMetrics | null;
  config: NodeConfig | null;
  storageStatus?: StorageStatus | null;
  portCheck?: PortCheckResult | null;
  onRefresh: () => void;
  loading?: boolean;
}

export const NetworkTab: React.FC<NetworkTabProps> = ({
  metrics,
  config,
  storageStatus,
  onRefresh
}) => {
  const [peerDetails, setPeerDetails] = useState<PeersDetailResponse | null>(null);
  const [fetchingPeers, setFetchingPeers] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [copiedKey, setCopiedKey] = useState<string | null>(null);
  const [activeSubSection, setActiveSubSection] = useState<'p2p' | 'storage'>('p2p');

  const handleCopy = (text: string, id: string) => {
    navigator.clipboard.writeText(text);
    setCopiedKey(id);
    setTimeout(() => setCopiedKey(null), 2000);
  };

  const isRunning = metrics?.status === 'running';

  // Fetch full peer details from backend
  const fetchPeerDetails = async () => {
    if (!isRunning) return;
    setFetchingPeers(true);
    try {
      const res = await fetch('/api/network/peers-detail');
      if (res.ok) {
        const json = await res.json();
        setPeerDetails(json);
      }
    } catch (e) {
      console.error('Failed to fetch peer details:', e);
    } finally {
      setFetchingPeers(false);
    }
  };

  useEffect(() => {
    fetchPeerDetails();
    const interval = setInterval(fetchPeerDetails, 15000);
    return () => clearInterval(interval);
  }, [isRunning]);

  // Truncate long strings
  const truncate = (str?: string, front = 8, back = 8) => {
    if (!str) return '—';
    if (str.length <= front + back) return str;
    return `${str.slice(0, front)}...${str.slice(-back)}`;
  };

  const peersList: PeerInfo[] = peerDetails?.peers || [];
  const seedNodes: SeedPingInfo[] = peerDetails?.seedNodes || [];
  const replicatorPeers = storageStatus?.peers || [];

  // Filter P2P peers
  const filteredPeers = useMemo(() => {
    return peersList.filter(peer => {
      if (!searchQuery.trim()) return true;
      const q = searchQuery.toLowerCase().trim();
      return (
        (peer.host && peer.host.toLowerCase().includes(q)) ||
        (peer.friendlyName && peer.friendlyName.toLowerCase().includes(q)) ||
        (peer.publicKey && peer.publicKey.toLowerCase().includes(q)) ||
        peer.port.toString().includes(q)
      );
    });
  }, [peersList, searchQuery]);

  return (
    <div className="space-y-6 max-w-5xl mx-auto px-4 py-6 select-none">
      
      {/* 1. Offline State Banner */}
      {!isRunning && (
        <div className="bg-[#181B20] border border-[#262B34] rounded-lg p-5 flex items-center justify-between">
          <div className="flex items-center space-x-3.5">
            <AlertCircle className="w-5 h-5 text-slate-400" />
            <div>
              <h3 className="text-sm font-medium text-slate-200">P2P Networking is Offline</h3>
              <p className="text-xs text-slate-400 mt-0.5">
                Start your Sirius node to establish peer connections and participate in gossip propagation.
              </p>
            </div>
          </div>
        </div>
      )}

      {/* 2. Hero Section: Primary Network Overview & Transport Identity */}
      <section className="bg-[#181B20] border border-[#262B34] rounded-lg p-6">
        <div className="flex flex-col md:flex-row md:items-baseline justify-between gap-6">
          
          <div>
            <div className="flex items-center space-x-2">
              <span className="text-xs font-medium tracking-wider uppercase text-slate-400">
                P2P Network Mesh
              </span>
              <span className={`text-xs font-medium px-2 py-0.5 rounded ${
                !isRunning 
                  ? 'bg-slate-800 text-slate-400 border border-slate-700/50' 
                  : 'bg-emerald-950/60 text-emerald-400 border border-emerald-800/40'
              }`}>
                {isRunning ? 'Connected & Listening' : 'Offline'}
              </span>
            </div>

            {/* Local Host & Transport Identity */}
            <div className="mt-3 flex items-center space-x-2">
              <span className="text-xs text-slate-400">Node Public Key:</span>
              <span className="text-sm font-mono text-slate-200 font-medium">
                {config?.bootPublicKey ? truncate(config.bootPublicKey, 10, 10) : 'Generating transport key...'}
              </span>
              {config?.bootPublicKey && (
                <button
                  onClick={() => handleCopy(config.bootPublicKey!, 'bootKey')}
                  className="p-1 hover:bg-[#262B34] rounded text-slate-400 hover:text-white transition-colors"
                  title="Copy Transport Key"
                >
                  {copiedKey === 'bootKey' ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                </button>
              )}
            </div>
          </div>

          {/* Peer Count Metrics */}
          <div className="flex items-center space-x-6 pt-3 md:pt-0 border-t md:border-t-0 border-[#262B34]">
            <div>
              <span className="text-xs text-slate-400 block">Connected P2P Peers</span>
              <span className="text-xl font-mono font-semibold text-white">
                {isRunning ? (metrics?.peersCount ?? 0) : 0}
              </span>
            </div>
            <div className="h-8 w-px bg-[#262B34]" />
            <div>
              <div className="flex items-center space-x-1.5">
                <span className="text-xs text-slate-400 block">Storage Replicators</span>
                <span className="text-[9px] bg-slate-800 text-slate-400 px-1 py-0.2 rounded font-sans border border-slate-700/50">
                  Public Net
                </span>
              </div>
              <span className="text-xl font-mono font-semibold text-slate-200">
                {replicatorPeers.length}
              </span>
            </div>
            <div className="h-8 w-px bg-[#262B34]" />
            <div>
              <span className="text-xs text-slate-400 block">P2P Port</span>
              <span className={`text-sm font-mono font-medium ${isRunning ? 'text-emerald-400' : 'text-slate-500'}`}>
                :{config?.port || 7900} {isRunning ? '(Open)' : '(Closed)'}
              </span>
            </div>
          </div>

        </div>
      </section>

      {/* 3. Unified Network List (Tabs for P2P Peers vs DFMS Replicators) */}
      <section className="bg-[#181B20] border border-[#262B34] rounded-lg p-5">
        
        {/* Sub-Header & Controls */}
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-4 pb-3 border-b border-[#262B34]/60">
          
          <div className="flex items-center space-x-2">
            <button
              onClick={() => setActiveSubSection('p2p')}
              className={`px-3 py-1.5 text-xs font-medium rounded-md transition-colors ${
                activeSubSection === 'p2p'
                  ? 'bg-[#262B34] text-white'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-[#262B34]/40'
              }`}
            >
              Connected P2P Peers ({peersList.length})
            </button>
            <button
              onClick={() => setActiveSubSection('storage')}
              className={`px-3 py-1.5 text-xs font-medium rounded-md transition-colors ${
                activeSubSection === 'storage'
                  ? 'bg-[#262B34] text-white'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-[#262B34]/40'
              }`}
            >
              DFMS Storage Replicators ({replicatorPeers.length})
            </button>
          </div>

          <div className="flex items-center space-x-2">
            {activeSubSection === 'p2p' && (
              <div className="relative">
                <Search className="w-3.5 h-3.5 text-slate-500 absolute left-2.5 top-2.5" />
                <input
                  type="text"
                  placeholder="Search host, key..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="bg-[#0F1115] border border-[#262B34] rounded px-2.5 pl-8 py-1.5 text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:border-blue-500 font-mono w-48 sm:w-56"
                />
              </div>
            )}

            <button
              onClick={() => {
                fetchPeerDetails();
                onRefresh();
              }}
              disabled={fetchingPeers}
              className="p-1.5 bg-[#0F1115] hover:bg-[#262B34] border border-[#262B34] rounded text-slate-400 hover:text-white transition-colors"
              title="Refresh Network"
            >
              <RefreshCw className={`w-3.5 h-3.5 ${fetchingPeers ? 'animate-spin text-blue-400' : ''}`} />
            </button>
          </div>

        </div>

        {/* View A: P2P Connected Peers Table */}
        {activeSubSection === 'p2p' && (
          <div>
            {filteredPeers.length > 0 ? (
              <div className="overflow-x-auto max-h-[400px] overflow-y-auto border border-[#262B34]/60 rounded scrollbar-thin">
                <table className="w-full text-left text-xs">
                  <thead className="sticky top-0 bg-[#181B20] border-b border-[#262B34] z-10 shadow-xs">
                    <tr className="text-slate-400">
                      <th className="py-2.5 px-3 font-medium">Node Endpoint</th>
                      <th className="py-2.5 px-3 font-medium">Type</th>
                      <th className="py-2.5 px-3 font-medium">Latency</th>
                      <th className="py-2.5 px-3 font-medium">Public Key</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-[#262B34]/60 font-mono">
                    {filteredPeers.map((peer, idx) => (
                      <tr key={idx} className="hover:bg-[#1E2228] transition-colors">
                        <td className="py-2.5 px-3 text-slate-200 font-medium">
                          <div className="flex items-center space-x-2">
                            <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 flex-shrink-0" />
                            <span>{peer.host || 'Unknown'}:{peer.port}</span>
                            {peer.friendlyName && (
                              <span className="text-[10px] text-slate-400 font-sans">
                                ({peer.friendlyName})
                              </span>
                            )}
                          </div>
                        </td>
                        <td className="py-2.5 px-3 text-slate-400 font-sans">
                          {peer.roles === 3 ? 'Dual / API Node' : peer.roles === 1 ? 'Peer Validator' : 'Relay Node'}
                        </td>
                        <td className="py-2.5 px-3 text-slate-300 font-mono">
                          {peer.latencyMs ? `${peer.latencyMs} ms` : '< 25 ms'}
                        </td>
                        <td className="py-2.5 px-3 text-slate-400">
                          <div className="flex items-center space-x-1.5">
                            <span>{truncate(peer.publicKey, 8, 8)}</span>
                            {peer.publicKey && (
                              <>
                                <button
                                  onClick={() => handleCopy(peer.publicKey, `peer-key-${idx}`)}
                                  className="p-0.5 hover:bg-[#262B34] rounded text-slate-500 hover:text-slate-200"
                                  title="Copy Public Key"
                                >
                                  {copiedKey === `peer-key-${idx}` ? <Check className="w-3 h-3 text-emerald-400" /> : <Copy className="w-3 h-3" />}
                                </button>
                                <a
                                  href={getExplorerPublicKeyUrl(peer.publicKey)}
                                  target="_blank"
                                  rel="noreferrer"
                                  className="p-0.5 hover:bg-[#262B34] rounded text-slate-500 hover:text-slate-200"
                                  title="View in Explorer"
                                >
                                  <ExternalLink className="w-3 h-3" />
                                </a>
                              </>
                            )}
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <div className="py-12 text-center text-xs text-slate-400">
                <Globe2 className="w-6 h-6 text-slate-500 mx-auto mb-2" />
                <p>
                  {isRunning 
                    ? 'Discovering network peers via seed nodes...' 
                    : 'Node is offline. Start node to discover active peers.'}
                </p>
              </div>
            )}
          </div>
        )}

        {/* View B: Storage Replicators Table */}
        {activeSubSection === 'storage' && (
          <div>
            <div className="flex flex-col sm:flex-row sm:items-center justify-between mb-3 text-xs text-slate-400 gap-1">
              <span className="flex items-center space-x-1.5">
                <span className="w-1.5 h-1.5 rounded-full bg-emerald-400" />
                <span className="font-medium text-slate-300">DFMS Storage Mesh</span>
                <span className="text-[10px] bg-slate-800 text-slate-400 border border-slate-700/60 px-1.5 py-0.2 rounded font-sans">
                  Public Network Telemetry
                </span>
              </span>
              <span className="text-[11px] text-slate-500 font-sans">
                Direct internet pings (Port :7904) — independent of local node status
              </span>
            </div>
            {replicatorPeers.length > 0 ? (
              <div className="overflow-x-auto max-h-[400px] overflow-y-auto border border-[#262B34]/60 rounded scrollbar-thin">
                <table className="w-full text-left text-xs">
                  <thead className="sticky top-0 bg-[#181B20] border-b border-[#262B34] z-10 shadow-xs">
                    <tr className="text-slate-400">
                      <th className="py-2.5 px-3 font-medium">Replicator Node</th>
                      <th className="py-2.5 px-3 font-medium">Public Status</th>
                      <th className="py-2.5 px-3 font-medium">Latency</th>
                      <th className="py-2.5 px-3 font-medium">Public Key</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-[#262B34]/60 font-mono">
                    {replicatorPeers.map((rep, idx) => (
                      <tr key={idx} className="hover:bg-[#1E2228] transition-colors">
                        <td className="py-2.5 px-3 text-slate-200 font-medium">
                          <div className="flex items-center space-x-2">
                            <span className={`w-1.5 h-1.5 rounded-full ${rep.isReachable ? 'bg-emerald-400' : 'bg-slate-500'}`} />
                            <span>{rep.host}:{rep.port}</span>
                            {rep.name && <span className="text-[10px] text-slate-400 font-sans">({rep.name})</span>}
                          </div>
                        </td>
                        <td className="py-2.5 px-3 text-slate-400 font-sans">
                          {rep.isReachable ? (
                            <div className="flex items-center space-x-1.5">
                              <span className="text-emerald-400 font-medium">Online</span>
                              <span className="text-[9px] bg-emerald-950/60 text-emerald-400 border border-emerald-800/40 px-1 py-0.2 rounded">
                                Public Peer
                              </span>
                            </div>
                          ) : (
                            <span className="text-slate-500">Unreachable</span>
                          )}
                        </td>
                        <td className="py-2.5 px-3 text-slate-300 font-mono">
                          {rep.latencyMs ? `${rep.latencyMs} ms` : '—'}
                        </td>
                        <td className="py-2.5 px-3 text-slate-400">
                          <div className="flex items-center space-x-1.5">
                            <span>{truncate(rep.publicKey, 8, 8)}</span>
                            {rep.publicKey && (
                              <button
                                onClick={() => handleCopy(rep.publicKey, `rep-key-${idx}`)}
                                className="p-0.5 hover:bg-[#262B34] rounded text-slate-500 hover:text-slate-200"
                                title="Copy Key"
                              >
                                {copiedKey === `rep-key-${idx}` ? <Check className="w-3 h-3 text-emerald-400" /> : <Copy className="w-3 h-3" />}
                              </button>
                            )}
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <div className="py-12 text-center text-xs text-slate-400">
                <HardDrive className="w-6 h-6 text-slate-500 mx-auto mb-2" />
                <p>No storage replicator peers detected on the DFMS mesh.</p>
                <p className="text-slate-500 text-[11px] mt-1">
                  Storage nodes will automatically appear here when connected to port 7904.
                </p>
              </div>
            )}
          </div>
        )}

      </section>

      {/* 4. Seed Nodes RTT Ping Telemetry */}
      {seedNodes.length > 0 && (
        <section className="bg-[#181B20] border border-[#262B34] rounded-lg p-5">
          <h3 className="text-xs font-semibold tracking-wider uppercase text-slate-400 mb-3 flex items-center space-x-2">
            <Radio className="w-3.5 h-3.5 text-blue-400" />
            <span>Official Mainnet Seed Nodes Ping</span>
          </h3>

          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
            {seedNodes.map((seed, idx) => (
              <div key={idx} className="bg-[#0F1115] border border-[#262B34] rounded p-2.5 flex items-center justify-between text-xs">
                <div className="min-w-0 pr-2">
                  <span className="text-slate-200 font-medium block truncate font-mono">{seed.endpoint}</span>
                  <span className="text-slate-500 text-[11px] font-sans block">Mainnet Seed</span>
                </div>
                <div className="flex items-center space-x-1.5 font-mono flex-shrink-0">
                  <span className={`w-1.5 h-1.5 rounded-full ${seed.latencyMs < 100 ? 'bg-emerald-400' : seed.latencyMs < 300 ? 'bg-amber-400' : 'bg-rose-400'}`} />
                  <span className="text-slate-300 font-semibold">{seed.latencyMs} ms</span>
                </div>
              </div>
            ))}
          </div>
        </section>
      )}

    </div>
  );
};
