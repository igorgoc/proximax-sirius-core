/**
 * Sirius Galaxy: Celestial Math & Spatial Positioning Engine
 * Provides deterministic logarithmic spiral coordinates, star luminosity calculations,
 * and P2P constellation edge generation for the ProximaX Sirius validator universe.
 */

export interface StarCoord {
  x: number;
  y: number;
  radius: number;
  color: string;
  glowColor: string;
  haloSize: number;
}

export interface ConstellationEdge {
  sourceKey: string;
  targetKey: string;
  x1: number;
  y1: number;
  x2: number;
  y2: number;
  opacity: number;
}

export interface GalaxyStarData {
  publicKey: string;
  shortKey: string;
  blocksCount: number;
  sharePercent: number;
  lastSeenHeight: number;
  lastSeenTime: string;
  isSelf: boolean;
  x: number;
  y: number;
  radius: number;
  color: string;
  glowColor: string;
  haloSize: number;
}

export interface BackgroundParticle {
  x: number;
  y: number;
  size: number;
  baseAlpha: number;
  twinkleSpeed: number;
  phase: number;
  color: string;
}

/**
 * Deterministic 32-bit FNV-1a hash of a public key string.
 */
export function hashStringToNumber(str: string): number {
  let hash = 2166136261;
  for (let i = 0; i < str.length; i++) {
    hash ^= str.charCodeAt(i);
    hash = Math.imul(hash, 16777619);
  }
  return hash >>> 0;
}

/**
 * Calculates persistent 2D galactic coordinates for a validator star.
 * Uses a centered galactic coordinate system where origin (0, 0) is the center of the galaxy.
 * Places 'isSelf' as the radiant Sirius anchor star near center-left (-120, -40).
 * Places peer stars in a dual-arm logarithmic spiral galaxy with golden ratio spacing.
 */
export function calculateStarPosition(
  publicKey: string,
  index: number,
  totalCount: number,
  isSelf: boolean
): { x: number; y: number } {
  if (isSelf) {
    // Sirius anchor: focal center-left offset
    return { x: -140, y: -45 };
  }

  const hash = hashStringToNumber(publicKey);
  const normalizedHash = (hash % 10000) / 10000;
  const hashJitter = ((hash % 1000) / 1000 - 0.5) * 45;

  // Dual spiral arms separated by PI radians
  const arm = index % 2;
  const armOffset = arm * Math.PI;

  // Step progression along the arm
  const t = Math.floor(index / 2);
  const goldenAngle = 2.39996323; // Golden angle in radians

  // Logarithmic spiral progression: r = a * e^(b * theta)
  const theta = t * 0.42 + armOffset + (normalizedHash * 0.35);
  const baseRadius = 160 + Math.pow(t + 1, 1.35) * 32;
  const r = Math.min(850, baseRadius + hashJitter);

  const x = Math.round(r * Math.cos(theta));
  const y = Math.round((r * 0.68) * Math.sin(theta)); // Elliptical galactic tilt (0.68 aspect)

  return { x, y };
}

/**
 * Determines star color, glow aura, and logarithmic radius based on harvesting performance.
 */
export function getStarColorAndRadius(
  validator: {
    blocksCount: number;
    sharePercent: number;
    isSelf: boolean;
    lastSeenHeight: number;
  },
  maxBlocks: number,
  currentHeight: number
): { radius: number; color: string; glowColor: string; haloSize: number } {
  // 1. Sirius Self Node: Brilliant Emerald & Cyan Corona
  if (validator.isSelf) {
    return {
      radius: 18,
      color: '#10B981',      // Emerald Green
      glowColor: '#06B6D4',  // Radiant Cyan Aura
      haloSize: 42,
    };
  }

  // Logarithmic radius scaling between 6px and 20px
  const safeMax = Math.max(1, maxBlocks);
  const safeBlocks = Math.max(0, validator.blocksCount);
  const logFactor = Math.log(1 + safeBlocks) / Math.log(1 + safeMax);
  const clampedLog = Math.max(0, Math.min(1, logFactor));
  const radius = Math.round(6 + clampedLog * 14);

  // 2. Top Supernodes (> 10% share or > 20% max blocks): Blazing Stellar Cyan-White
  if (validator.sharePercent >= 10 || safeBlocks >= safeMax * 0.5) {
    return {
      radius: Math.max(14, radius),
      color: '#F0F9FF',      // Pure Star White
      glowColor: '#38BDF8',  // Cyan Plasma
      haloSize: 28,
    };
  }

  // Blocks since last harvest
  const heightDelta = currentHeight > 0 && validator.lastSeenHeight > 0
    ? Math.max(0, currentHeight - validator.lastSeenHeight)
    : 0;

  // 3. Recently Active Harvester (< 120 blocks / ~30 minutes): Amber Gold
  if (heightDelta <= 120 && safeBlocks > 0) {
    return {
      radius: Math.max(10, radius),
      color: '#FCD34D',      // Light Gold
      glowColor: '#F59E0B',  // Amber
      haloSize: 22,
    };
  }

  // 4. Moderate Harvester (< 480 blocks / ~2 hours): Nebula Violet
  if (heightDelta <= 480 && safeBlocks > 0) {
    return {
      radius: Math.max(8, radius),
      color: '#C4B5FD',      // Soft Lavender
      glowColor: '#818CF8',  // Violet Nebula
      haloSize: 16,
    };
  }

  // 5. Standby / Syncing Peer: Cosmic Slate Silver
  return {
    radius: Math.max(6, radius),
    color: '#94A3B8',        // Silver Slate
    glowColor: '#475569',    // Dim Nebula
    haloSize: 10,
  };
}

/**
 * Generates constellation filaments connecting nodes.
 * Connects each node to its 2 nearest celestial neighbors or active P2P peers.
 */
export function generateConstellationEdges(
  stars: Array<{ publicKey: string; x: number; y: number; isSelf?: boolean }>
): ConstellationEdge[] {
  const edges: ConstellationEdge[] = [];
  const edgeSet = new Set<string>();

  for (let i = 0; i < stars.length; i++) {
    const s1 = stars[i];

    // Find nearest neighbor distances
    const distances: Array<{ index: number; dist: number }> = [];
    for (let j = 0; j < stars.length; j++) {
      if (i === j) continue;
      const s2 = stars[j];
      const dx = s1.x - s2.x;
      const dy = s1.y - s2.y;
      const dist = Math.sqrt(dx * dx + dy * dy);
      distances.push({ index: j, dist });
    }

    distances.sort((a, b) => a.dist - b.dist);

    // Connect to 2 nearest neighbors if within visual range
    const maxFilamentDist = 320;
    const connectionsCount = Math.min(2, distances.length);

    for (let k = 0; k < connectionsCount; k++) {
      const neighbor = distances[k];
      if (neighbor.dist <= maxFilamentDist) {
        const s2 = stars[neighbor.index];
        const edgeKey = s1.publicKey < s2.publicKey
          ? `${s1.publicKey}_${s2.publicKey}`
          : `${s2.publicKey}_${s1.publicKey}`;

        if (!edgeSet.has(edgeKey)) {
          edgeSet.add(edgeKey);
          const opacity = Math.max(0.12, 0.45 - (neighbor.dist / maxFilamentDist) * 0.35);
          edges.push({
            sourceKey: s1.publicKey,
            targetKey: s2.publicKey,
            x1: s1.x,
            y1: s1.y,
            x2: s2.x,
            y2: s2.y,
            opacity: parseFloat(opacity.toFixed(2)),
          });
        }
      }
    }
  }

  return edges;
}

/**
 * Pre-computes background stardust particles for deep space ambiance.
 */
export function generateBackgroundParticles(count = 140, spread = 1200): BackgroundParticle[] {
  const particles: BackgroundParticle[] = [];
  const colors = ['#E2E8F0', '#93C5FD', '#C4B5FD', '#FDE68A'];

  for (let i = 0; i < count; i++) {
    const angle = Math.random() * Math.PI * 2;
    const dist = Math.pow(Math.random(), 0.7) * spread;
    particles.push({
      x: Math.cos(angle) * dist,
      y: Math.sin(angle) * (dist * 0.7),
      size: Math.random() * 1.8 + 0.6,
      baseAlpha: Math.random() * 0.5 + 0.2,
      twinkleSpeed: Math.random() * 0.03 + 0.01,
      phase: Math.random() * Math.PI * 2,
      color: colors[Math.floor(Math.random() * colors.length)],
    });
  }

  return particles;
}
