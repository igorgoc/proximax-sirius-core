import React, { useState, useMemo, useRef, useEffect } from 'react';
import {
  NodeMetrics,
  HarvestStats,
  NetworkValidatorStats,
} from '../../types';
import {
  GalaxyStarData,
  ConstellationEdge,
  calculateStarPosition,
  getStarColorAndRadius,
  generateConstellationEdges,
} from '../../utils/galaxyMath';
import { SiriusGalaxyCanvas } from './SiriusGalaxyCanvas';
import { SiriusGalaxyHUD } from './SiriusGalaxyHUD';

export interface SiriusGalaxyViewProps {
  metrics: NodeMetrics | null;
  harvestStats: HarvestStats | null;
  networkValidatorStats: NetworkValidatorStats | null;
}

export const SiriusGalaxyView: React.FC<SiriusGalaxyViewProps> = ({
  metrics,
  harvestStats,
  networkValidatorStats,
}) => {
  const [selectedStar, setSelectedStar] = useState<GalaxyStarData | null>(null);
  const [showFilaments, setShowFilaments] = useState(true);
  const [showLabels, setShowLabels] = useState(true);
  const [activeNova, setActiveNova] = useState<{ x: number; y: number; text?: string } | null>(null);

  const cameraResetRef = useRef<(() => void) | null>(null);
  const centerStarRef = useRef<((star: GalaxyStarData) => void) | null>(null);
  const lastObservedHeightRef = useRef<number>(metrics?.blockHeight || 0);

  const currentHeight = metrics?.blockHeight || 0;

  // 1. Transform raw validator network data into celestial stars
  const stars: GalaxyStarData[] = useMemo(() => {
    const rawList = networkValidatorStats?.topValidators || [];
    if (rawList.length === 0) return [];

    let maxBlocks = 1;
    for (const v of rawList) {
      if (v.blocksCount > maxBlocks) maxBlocks = v.blocksCount;
    }

    return rawList.map((v, index) => {
      const pos = calculateStarPosition(v.publicKey, index, rawList.length, v.isSelf);
      const visual = getStarColorAndRadius(v, maxBlocks, currentHeight);

      return {
        publicKey: v.publicKey,
        shortKey: v.shortKey || v.publicKey.slice(-8),
        blocksCount: v.blocksCount,
        sharePercent: v.sharePercent,
        lastSeenHeight: v.lastSeenHeight,
        lastSeenTime: v.lastSeenTime,
        isSelf: v.isSelf,
        x: pos.x,
        y: pos.y,
        radius: visual.radius,
        color: visual.color,
        glowColor: visual.glowColor,
        haloSize: visual.haloSize,
      };
    });
  }, [networkValidatorStats, currentHeight]);

  // 2. Generate P2P constellation filaments
  const edges: ConstellationEdge[] = useMemo(() => {
    if (stars.length === 0) return [];
    return generateConstellationEdges(stars);
  }, [stars]);

  // 3. Real-time Block Commitment Listener: Trigger Nova shockwave on new block
  useEffect(() => {
    if (currentHeight > 0 && lastObservedHeightRef.current > 0 && currentHeight > lastObservedHeightRef.current) {
      // Find the winning harvester of the latest block
      const winningStar = stars.find((s) => s.lastSeenHeight === currentHeight) || stars[0];
      if (winningStar) {
        setActiveNova({
          x: winningStar.x,
          y: winningStar.y,
          text: `Block #${currentHeight}`,
        });
      }
    }
    lastObservedHeightRef.current = currentHeight;
  }, [currentHeight, stars]);

  const handleCenterStar = (star: GalaxyStarData) => {
    if (centerStarRef.current) {
      centerStarRef.current(star);
    }
  };

  return (
    <div className="flex flex-col lg:flex-row gap-4 w-full h-[660px]">
      {/* Interactive Galactic Universe Canvas */}
      <div className="flex-1 h-full min-h-[460px]">
        <SiriusGalaxyCanvas
          stars={stars}
          edges={edges}
          selectedStar={selectedStar}
          onSelectStar={setSelectedStar}
          activeNova={activeNova}
          showFilaments={showFilaments}
          showLabels={showLabels}
          onCameraResetRef={cameraResetRef}
          onCenterStarRef={centerStarRef}
        />
      </div>

      {/* Glassmorphic Side Panel HUD */}
      <div className="h-full">
        <SiriusGalaxyHUD
          stars={stars}
          selectedStar={selectedStar}
          onSelectStar={setSelectedStar}
          onCenterStar={handleCenterStar}
          totalNetworkFees={networkValidatorStats?.totalNetworkFees4h || 0}
          activeValidators4h={networkValidatorStats?.activeValidators4h || 0}
          activeValidators24h={networkValidatorStats?.activeValidators24h || 0}
          estimatedStakedPoolXPX={networkValidatorStats?.estimatedStakedPoolXPX || 0}
          currentHeight={currentHeight}
          showFilaments={showFilaments}
          setShowFilaments={setShowFilaments}
          showLabels={showLabels}
          setShowLabels={setShowLabels}
        />
      </div>
    </div>
  );
};
