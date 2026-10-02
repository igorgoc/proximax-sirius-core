import { describe, it, expect } from 'vitest';
import {
  formatCompactXPX,
  formatXPXAmount,
  calculateStarPosition,
  generateConstellationEdges,
  getStarColorAndRadius,
  GalaxyStarData,
} from './galaxyMath';

describe('Sirius Galaxy Math & Formatting Engine', () => {
  describe('formatCompactXPX', () => {
    it('formats millions of XPX cleanly without suffix mark (e.g. 7.7M)', () => {
      expect(formatCompactXPX(7727963.75)).toBe('7.7M');
      expect(formatCompactXPX(7700000)).toBe('7.7M');
      expect(formatCompactXPX(1000000)).toBe('1.0M');
      expect(formatCompactXPX(2500000)).toBe('2.5M');
      expect(formatCompactXPX(2600000)).toBe('2.6M');
    });

    it('formats thousands of XPX with k suffix', () => {
      expect(formatCompactXPX(500000)).toBe('500k');
      expect(formatCompactXPX(1000)).toBe('1k');
      expect(formatCompactXPX(1500)).toBe('2k');
    });

    it('formats small balances and zero correctly', () => {
      expect(formatCompactXPX(42)).toBe('42');
      expect(formatCompactXPX(0)).toBe('0');
      expect(formatCompactXPX(-10)).toBe('0');
      expect(formatCompactXPX(undefined as unknown as number)).toBe('0');
    });
  });

  describe('formatXPXAmount', () => {
    it('formats amounts with XPX suffix for HUD display', () => {
      expect(formatXPXAmount(7727963.75)).toBe('7.7M XPX');
      expect(formatXPXAmount(1000)).toBe('1k XPX');
      expect(formatXPXAmount(50)).toBe('50 XPX');
      expect(formatXPXAmount(0)).toBe('0 XPX');
    });
  });

  describe('calculateStarPosition', () => {
    it('positions self node at celestial Sirius anchor coordinates (-140, -45)', () => {
      const pos = calculateStarPosition('1D339BA5E197D7AB2E4BFA9312B5C115040740F9F00C5E3BD7EA6F911B5827F2', true);
      expect(pos.x).toBe(-140);
      expect(pos.y).toBe(-45);
    });

    it('is strictly deterministic: same public key yields identical coordinates', () => {
      const pubKey = 'B96F52511C250E59DF1CA9D1A751684560B1923527EC1D2ACDB2E32AAAB2A825';
      const pos1 = calculateStarPosition(pubKey, false);
      const pos2 = calculateStarPosition(pubKey, false);
      const pos3 = calculateStarPosition(pubKey, false);

      expect(pos1.x).toBe(pos2.x);
      expect(pos1.y).toBe(pos2.y);
      expect(pos2.x).toBe(pos3.x);
      expect(pos2.y).toBe(pos3.y);
    });

    it('distributes different validator public keys to different coordinates', () => {
      const keyA = 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA';
      const keyB = 'BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB';

      const posA = calculateStarPosition(keyA, false);
      const posB = calculateStarPosition(keyB, false);

      expect(posA.x).not.toBe(posB.x);
      expect(posA.y).not.toBe(posB.y);
      expect(Number.isNaN(posA.x)).toBe(false);
      expect(Number.isNaN(posB.y)).toBe(false);
    });
  });

  describe('generateConstellationEdges', () => {
    it('returns empty edges if stars count is less than 2', () => {
      expect(generateConstellationEdges([])).toEqual([]);
      const singleStar: GalaxyStarData = {
        publicKey: 'KEY1',
        shortKey: 'KEY1',
        blocksCount: 1,
        sharePercent: 100,
        stakedBalanceXPX: 1000000,
        lastSeenHeight: 100,
        lastSeenTime: 'Recently',
        isSelf: true,
        x: 0,
        y: 0,
        radius: 16,
        color: '#10B981',
        glowColor: '#06B6D4',
        haloSize: 40,
      };
      expect(generateConstellationEdges([singleStar])).toEqual([]);
    });

    it('builds connected constellation mesh for validator cluster without NaN', () => {
      const stars = [
        {
          publicKey: 'KEY_CENTER',
          isSelf: true,
          x: -140,
          y: -45,
        },
        {
          publicKey: 'KEY_PEER_1',
          isSelf: false,
          x: 150,
          y: 80,
        },
        {
          publicKey: 'KEY_PEER_2',
          isSelf: false,
          x: -120,
          y: 110,
        },
      ];

      const edges = generateConstellationEdges(stars);
      expect(edges.length).toBeGreaterThanOrEqual(2); // At least N-1 edges for 3 nodes

      for (const edge of edges) {
        expect(Number.isNaN(edge.x1)).toBe(false);
        expect(Number.isNaN(edge.y1)).toBe(false);
        expect(Number.isNaN(edge.x2)).toBe(false);
        expect(Number.isNaN(edge.y2)).toBe(false);
        expect(edge.opacity).toBeGreaterThan(0);
        expect(edge.opacity).toBeLessThanOrEqual(0.65);
      }
    });
  });

  describe('getStarColorAndRadius', () => {
    it('assigns self emerald palette regardless of block count', () => {
      const visual = getStarColorAndRadius(
        {
          blocksCount: 0,
          sharePercent: 0,
          isSelf: true,
          lastSeenHeight: 100,
        },
        10,
        105
      );
      expect(visual.color).toBe('#10B981');
      expect(visual.glowColor).toBe('#06B6D4');
      expect(visual.radius).toBeGreaterThanOrEqual(14);
    });

    it('assigns active peer appropriate visual sizing based on maxBlocks', () => {
      const visual = getStarColorAndRadius(
        {
          blocksCount: 10,
          sharePercent: 50,
          isSelf: false,
          lastSeenHeight: 100,
        },
        10,
        101
      );
      expect(visual.radius).toBeGreaterThan(8);
      expect(visual.haloSize).toBeGreaterThan(visual.radius);
    });
  });
});
