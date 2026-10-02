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
  stakedBalanceXPX: number;
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

export function formatXPXAmount(xpx: number): string {
  if (!xpx || xpx <= 0) return '0 XPX';
  if (xpx >= 1000000) {
    const val = xpx / 1000000;
    return `${val.toFixed(1)}M XPX`;
  }
  if (xpx >= 1000) {
    return `${(xpx / 1000).toFixed(0)}k XPX`;
  }
  return `${Math.round(xpx)} XPX`;
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
 * 100% DETERMINISTIC based purely on publicKey and isSelf:
 * Stars NEVER move their coordinates when blocks are harvested or rankings change!
 */
export function calculateStarPosition(
  publicKey: string,
  isSelf: boolean
): { x: number; y: number } {
  if (isSelf) {
    // Sirius anchor: focal center-left offset
    return { x: -140, y: -45 };
  }

  const hash = hashStringToNumber(publicKey);

  // Arm 0 or Arm 1 based on hash (180 deg offset)
  const arm = (hash >>> 0) % 2;
  const armAngle = arm * Math.PI;

  // Normalized distance along the arm [0..1]
  const u = ((hash >>> 8) % 10000) / 10000;
  // Natural galactic density: clustered around core, trailing outward
  const radius = 175 + Math.pow(u, 0.85) * 350;

  // Logarithmic spiral angle progression with radius + subtle organic jitter
  const spiralAngle = Math.log(radius / 130) * 2.6;
  const jitter = (((hash >>> 16) % 1000) / 1000 - 0.5) * 0.28;
  const theta = armAngle + spiralAngle + jitter;

  // Elliptical coordinate calculation (0.68 celestial tilt)
  const x = Math.round(radius * Math.cos(theta));
  const y = Math.round((radius * 0.68) * Math.sin(theta));

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
 * Uses k-nearest neighbors + Minimum Spanning Tree (MST) bridging to guarantee
 * that all stars (and both spiral arms + Sirius anchor) form a single cohesive,
 * organically connected cosmic constellation mesh without isolated groups.
 */
export function generateConstellationEdges(
  stars: Array<{ publicKey: string; x: number; y: number; isSelf?: boolean }>
): ConstellationEdge[] {
  const n = stars.length;
  if (n < 2) return [];

  const edges: ConstellationEdge[] = [];
  const edgeSet = new Set<string>();

  const addEdge = (i: number, j: number, forcedOpacity?: number) => {
    if (i === j || i < 0 || j < 0 || i >= n || j >= n) return;
    const s1 = stars[i];
    const s2 = stars[j];
    const edgeKey = s1.publicKey < s2.publicKey
      ? `${s1.publicKey}_${s2.publicKey}`
      : `${s2.publicKey}_${s1.publicKey}`;

    if (edgeSet.has(edgeKey)) return;
    edgeSet.add(edgeKey);

    const dx = s1.x - s2.x;
    const dy = s1.y - s2.y;
    const dist = Math.sqrt(dx * dx + dy * dy);

    // Calculate opacity based on distance or forced override
    let opacity = forcedOpacity;
    if (opacity === undefined) {
      const isAnchorEdge = s1.isSelf || s2.isSelf;
      if (isAnchorEdge) {
        opacity = Math.max(0.24, 0.52 - (dist / 650) * 0.3);
      } else {
        opacity = Math.max(0.12, 0.42 - (dist / 550) * 0.3);
      }
    }

    edges.push({
      sourceKey: s1.publicKey,
      targetKey: s2.publicKey,
      x1: s1.x,
      y1: s1.y,
      x2: s2.x,
      y2: s2.y,
      opacity: parseFloat(opacity.toFixed(2)),
    });
  };

  // 1. Calculate all pairwise distances
  interface PairDist {
    i: number;
    j: number;
    dist: number;
  }
  const allPairs: PairDist[] = [];
  for (let i = 0; i < n; i++) {
    for (let j = i + 1; j < n; j++) {
      const dx = stars[i].x - stars[j].x;
      const dy = stars[i].y - stars[j].y;
      const dist = Math.sqrt(dx * dx + dy * dy);
      allPairs.push({ i, j, dist });
    }
  }

  // Sort pairs by ascending Euclidean distance
  allPairs.sort((a, b) => a.dist - b.dist);

  // 2. Disjoint-Set (Union-Find) for MST guaranteed connectivity
  const parent = new Int32Array(n);
  for (let i = 0; i < n; i++) parent[i] = i;

  const find = (i: number): number => {
    let root = i;
    while (root !== parent[root]) root = parent[root];
    let curr = i;
    while (curr !== root) {
      const next = parent[curr];
      parent[curr] = root;
      curr = next;
    }
    return root;
  };

  const union = (i: number, j: number): boolean => {
    const rootI = find(i);
    const rootJ = find(j);
    if (rootI === rootJ) return false;
    parent[rootI] = rootJ;
    return true;
  };

  // 3. Kruskal's MST to guarantee all nodes are connected into a single component
  let componentsCount = n;
  for (let p = 0; p < allPairs.length; p++) {
    const pair = allPairs[p];
    if (union(pair.i, pair.j)) {
      addEdge(pair.i, pair.j);
      componentsCount--;
      if (componentsCount === 1) break;
    }
  }

  // 4. Add k-Nearest Neighbors (k=3) for richer local constellation mesh density
  const k = Math.min(3, n - 1);
  for (let i = 0; i < n; i++) {
    const neighbors: Array<{ j: number; dist: number }> = [];
    for (let j = 0; j < n; j++) {
      if (i === j) continue;
      const dx = stars[i].x - stars[j].x;
      const dy = stars[i].y - stars[j].y;
      neighbors.push({ j, dist: Math.sqrt(dx * dx + dy * dy) });
    }
    neighbors.sort((a, b) => a.dist - b.dist);
    for (let m = 0; m < k; m++) {
      // Connect local neighbors within reasonable galaxy radius
      if (neighbors[m].dist <= 520) {
        addEdge(i, neighbors[m].j);
      }
    }
  }

  // 5. Special Anchor Links: Connect the Sirius Anchor (isSelf) to closest nodes on both sides
  const selfIndex = stars.findIndex((s) => s.isSelf);
  if (selfIndex !== -1) {
    const distancesFromSelf: Array<{ j: number; dist: number }> = [];
    for (let j = 0; j < n; j++) {
      if (j === selfIndex) continue;
      const dx = stars[selfIndex].x - stars[j].x;
      const dy = stars[selfIndex].y - stars[j].y;
      distancesFromSelf.push({ j, dist: Math.sqrt(dx * dx + dy * dy) });
    }
    distancesFromSelf.sort((a, b) => a.dist - b.dist);
    // Connect self to 3 nearest stars
    const anchorConnections = Math.min(3, distancesFromSelf.length);
    for (let m = 0; m < anchorConnections; m++) {
      addEdge(selfIndex, distancesFromSelf[m].j, 0.38);
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
