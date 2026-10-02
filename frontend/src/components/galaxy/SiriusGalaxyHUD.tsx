import React, { useState, useMemo } from 'react';
import {
  Search,
  Crosshair,
  Copy,
  Check,
  ExternalLink,
  Shield,
  Eye,
  EyeOff,
  Share2,
  Sparkles,
  Award,
  Zap,
} from 'lucide-react';
import { GalaxyStarData, formatXPXAmount } from '../../utils/galaxyMath';
import { getExplorerPublicKeyUrl } from '../../utils/explorer';
import { formatNumber } from '../../utils/format';

export type FilterMode = 'all' | 'self' | 'top10' | 'active4h';

export interface SiriusGalaxyHUDProps {
  stars: GalaxyStarData[];
  selectedStar: GalaxyStarData | null;
  onSelectStar: (star: GalaxyStarData | null) => void;
  onCenterStar: (star: GalaxyStarData) => void;
  totalNetworkFees: number;
  activeValidators4h: number;
  activeValidators24h: number;
  estimatedStakedPoolXPX: number;
  currentHeight: number;
  showFilaments: boolean;
  setShowFilaments: (val: boolean) => void;
  showLabels: boolean;
  setShowLabels: (val: boolean) => void;
}

export const SiriusGalaxyHUD: React.FC<SiriusGalaxyHUDProps> = ({
  stars,
  selectedStar,
  onSelectStar,
  onCenterStar,
  totalNetworkFees,
  activeValidators4h,
  activeValidators24h,
  estimatedStakedPoolXPX,
  currentHeight,
  showFilaments,
  setShowFilaments,
  showLabels,
  setShowLabels,
}) => {
  const [searchQuery, setSearchQuery] = useState('');
  const [filterMode, setFilterMode] = useState<FilterMode>('all');
  const [copiedKey, setCopiedKey] = useState<string | null>(null);

  const handleCopy = (text: string, id: string) => {
    navigator.clipboard.writeText(text);
    setCopiedKey(id);
    setTimeout(() => setCopiedKey(null), 2000);
  };

  // Filtered and sorted validators list
  const filteredStars = useMemo(() => {
    let list = [...stars];

    // Filter by mode
    if (filterMode === 'self') {
      list = list.filter((s) => s.isSelf);
    } else if (filterMode === 'top10') {
      list = list.slice(0, 10);
    } else if (filterMode === 'active4h') {
      list = list.filter((s) => {
        const delta = currentHeight > 0 && s.lastSeenHeight > 0
          ? currentHeight - s.lastSeenHeight
          : 999999;
        return delta <= 960 && s.blocksCount > 0;
      });
    }

    // Filter by search query
    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase().trim();
      list = list.filter(
        (s) =>
          s.publicKey.toLowerCase().includes(q) ||
          s.shortKey.toLowerCase().includes(q)
      );
    }

    return list;
  }, [stars, filterMode, searchQuery, currentHeight]);

  return (
    <div className="flex flex-col w-full lg:w-80 h-full max-h-[640px] bg-[#0E121A]/90 backdrop-blur-md rounded-xl border border-[#232936] shadow-2xl overflow-hidden font-sans text-slate-200">
      
      {/* 1. Header & Live Universe Metrics */}
      <div className="p-3.5 border-b border-[#1E2533] bg-[#121622]/60">
        <div className="flex items-center justify-between mb-2">
          <div className="flex items-center space-x-1.5">
            <Sparkles className="w-4 h-4 text-emerald-400 animate-pulse" />
            <span className="text-xs font-bold tracking-wider uppercase text-slate-100">
              Sirius Constellation
            </span>
          </div>
          <span className="text-[10px] font-mono px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/30">
            Block #{formatNumber(currentHeight)}
          </span>
        </div>

        {/* Metric 2x2 Grid */}
        <div className="grid grid-cols-2 gap-2 text-[11px] font-mono">
          <div className="bg-[#171D29]/70 rounded p-1.5 border border-[#263042]">
            <span className="text-slate-400 block text-[9px] uppercase tracking-wider">Network Fees</span>
            <span className="text-amber-400 font-bold">{formatNumber(Math.round(totalNetworkFees))} XPX</span>
          </div>
          <div className="bg-[#171D29]/70 rounded p-1.5 border border-[#263042]">
            <span className="text-slate-400 block text-[9px] uppercase tracking-wider">Active Harvesters</span>
            <span className="text-emerald-400 font-bold">{activeValidators4h} <span className="text-[10px] text-slate-500 font-normal">/ {activeValidators24h} (24h)</span></span>
          </div>
        </div>

        {/* 2. Visual Layer Toggles */}
        <div className="flex items-center justify-between mt-2.5 pt-2 border-t border-[#1C2230] text-[11px]">
          <button
            onClick={() => setShowFilaments(!showFilaments)}
            className={`flex items-center space-x-1 px-2 py-1 rounded transition-colors ${
              showFilaments ? 'text-sky-400 bg-sky-950/40 border border-sky-800/40' : 'text-slate-500 hover:text-slate-300'
            }`}
            title="Toggle P2P Constellation Filaments"
          >
            <Share2 className="w-3 h-3" />
            <span>Mesh Lines</span>
          </button>

          <button
            onClick={() => setShowLabels(!showLabels)}
            className={`flex items-center space-x-1 px-2 py-1 rounded transition-colors ${
              showLabels ? 'text-emerald-400 bg-emerald-950/40 border border-emerald-800/40' : 'text-slate-500 hover:text-slate-300'
            }`}
            title="Toggle Star Names & Monikers"
          >
            {showLabels ? <Eye className="w-3 h-3" /> : <EyeOff className="w-3 h-3" />}
            <span>Star Labels</span>
          </button>
        </div>
      </div>

      {/* 3. Search and Quick Filters */}
      <div className="p-2.5 border-b border-[#1A202C] bg-[#0F141F]/40 space-y-2">
        <div className="relative">
          <Search className="w-3.5 h-3.5 absolute left-2.5 top-2.5 text-slate-500" />
          <input
            type="text"
            placeholder="Search public key..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="w-full pl-8 pr-3 py-1.5 text-xs bg-[#151B26] border border-[#263042] rounded-md text-slate-200 placeholder-slate-500 focus:outline-none focus:border-sky-500"
          />
        </div>

        <div className="flex items-center space-x-1 text-[10px] overflow-x-auto scrollbar-none">
          {(['all', 'self', 'top10', 'active4h'] as FilterMode[]).map((mode) => (
            <button
              key={mode}
              onClick={() => setFilterMode(mode)}
              className={`px-2 py-0.5 rounded capitalize whitespace-nowrap transition-colors ${
                filterMode === mode
                  ? 'bg-sky-500/20 text-sky-300 border border-sky-500/40 font-bold'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-[#1A2230]'
              }`}
            >
              {mode === 'all'
                ? 'All Stars'
                : mode === 'self'
                ? '★ My Node'
                : mode === 'top10'
                ? 'Top 10'
                : 'Active (4h)'}
            </button>
          ))}
        </div>
      </div>

      {/* 4. Ranked Scrollable Validator Directory */}
      <div className="flex-1 overflow-y-auto divide-y divide-[#181F2B] scrollbar-thin scrollbar-thumb-slate-700">
        {filteredStars.length === 0 ? (
          <div className="p-6 text-center text-xs text-slate-500">
            No validator stars found matching filter.
          </div>
        ) : (
          filteredStars.map((star, idx) => {
            const isSelected = selectedStar?.publicKey === star.publicKey;
            return (
              <div
                key={star.publicKey}
                onClick={() => onSelectStar(star)}
                className={`flex items-center justify-between p-2.5 hover:bg-[#161D2B] transition-colors cursor-pointer text-xs ${
                  isSelected ? 'bg-sky-950/30 border-l-2 border-sky-400' : ''
                } ${star.isSelf ? 'bg-emerald-950/20' : ''}`}
              >
                <div className="flex items-center space-x-2 min-w-0">
                  <span className="w-5 text-[10px] font-mono text-slate-500 text-right">
                    #{idx + 1}
                  </span>
                  <div
                    className="w-2.5 h-2.5 rounded-full flex-shrink-0"
                    style={{ backgroundColor: star.color, boxShadow: `0 0 8px ${star.glowColor}` }}
                  />
                  <div className="min-w-0">
                    <div className="flex items-center space-x-1.5">
                      <span className="font-mono font-medium truncate text-slate-200">
                        {star.shortKey}
                      </span>
                      {star.isSelf && (
                        <span className="text-[9px] bg-emerald-500/20 text-emerald-400 px-1 rounded font-bold">
                          YOU
                        </span>
                      )}
                    </div>
                    <div className="text-[10px] text-slate-400 font-mono">
                      <span className="text-amber-400/90 font-medium">{formatXPXAmount(star.stakedBalanceXPX)}</span> • {star.blocksCount} blks ({star.sharePercent.toFixed(1)}%)
                    </div>
                  </div>
                </div>

                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    onSelectStar(star);
                    onCenterStar(star);
                  }}
                  className="p-1 text-slate-500 hover:text-sky-300 hover:bg-slate-700/40 rounded transition-colors"
                  title="Target in Galaxy"
                >
                  <Crosshair className="w-3.5 h-3.5" />
                </button>
              </div>
            );
          })
        )}
      </div>

      {/* 5. Selected Star Inspector Card */}
      {selectedStar && (
        <div className="p-3 border-t border-[#232936] bg-[#121722] text-xs font-mono">
          <div className="flex items-center justify-between mb-1.5">
            <span className="text-[10px] uppercase text-slate-400 font-bold flex items-center space-x-1">
              <Zap className="w-3 h-3 text-amber-400" />
              <span>Star Details</span>
            </span>
            <div className="flex items-center space-x-1">
              <button
                onClick={() => handleCopy(selectedStar.publicKey, 'inspector')}
                className="p-1 hover:text-white rounded hover:bg-slate-700/50"
                title="Copy Public Key"
              >
                {copiedKey === 'inspector' ? (
                  <Check className="w-3 h-3 text-emerald-400" />
                ) : (
                  <Copy className="w-3 h-3 text-slate-400" />
                )}
              </button>
              <a
                href={getExplorerPublicKeyUrl(selectedStar.publicKey)}
                target="_blank"
                rel="noopener noreferrer"
                className="p-1 hover:text-white rounded hover:bg-slate-700/50"
                title="View on Explorer"
              >
                <ExternalLink className="w-3 h-3 text-slate-400" />
              </a>
            </div>
          </div>

          <div className="text-[11px] truncate text-slate-300 mb-2">
            {selectedStar.publicKey}
          </div>

          <div className="grid grid-cols-3 gap-1.5 text-[10px]">
            <div className="bg-[#171F2C] p-1.5 rounded">
              <span className="text-slate-500 block text-[9px]">STAKED</span>
              <span className="text-amber-400 font-bold">{formatXPXAmount(selectedStar.stakedBalanceXPX)}</span>
            </div>
            <div className="bg-[#171F2C] p-1.5 rounded">
              <span className="text-slate-500 block text-[9px]">BLOCKS</span>
              <span className="text-white font-bold">{selectedStar.blocksCount}</span>
            </div>
            <div className="bg-[#171F2C] p-1.5 rounded">
              <span className="text-slate-500 block text-[9px]">SHARE</span>
              <span className="text-sky-300 font-bold">{selectedStar.sharePercent.toFixed(1)}%</span>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
