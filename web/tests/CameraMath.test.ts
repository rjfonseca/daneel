import { describe, it, expect } from 'vitest';
import {
  clampCameraRatio,
  calculateClusterFraming,
  calculateNodeFocusCamera,
  calculateDrawerRecenterX,
} from '../src/lib/cameraMath';

describe('cameraMath module tests', () => {

  const viewport720p = { width: 1280, height: 720 };

  it('recenters an open note as its drawer grows without changing zoom', () => {
    const target = { x: 0.65, y: 0.55 };
    const currentRatio = 0.23;
    const x = calculateDrawerRecenterX(target, currentRatio, viewport720p, 600);
    expect(x).toBeCloseTo(target.x + (600 / 2) * currentRatio / 720);
  });

  describe('clampCameraRatio', () => {
    it('clamps values below min', () => {
      expect(clampCameraRatio(0.01, 0.04, 0.55)).toBe(0.04);
    });

    it('clamps values above max', () => {
      expect(clampCameraRatio(0.80, 0.04, 0.55)).toBe(0.55);
    });

    it('preserves valid values in range', () => {
      expect(clampCameraRatio(0.25, 0.04, 0.55)).toBe(0.25);
    });

    it('handles NaN or non-finite values safely', () => {
      expect(clampCameraRatio(NaN, 0.04, 0.55)).toBe(0.04);
      expect(clampCameraRatio(Infinity, 0.04, 0.55)).toBe(0.04);
    });
  });

  describe('calculateClusterFraming', () => {
    it('returns default camera state for empty points', () => {
      const res = calculateClusterFraming([], viewport720p);
      expect(res).toEqual({ x: 0.5, y: 0.5, ratio: 1.0 });
    });

    it('calculates true centroid and bounded ratio for single point', () => {
      const points = [{ x: 0.45, y: 0.60 }];
      const res = calculateClusterFraming(points, viewport720p);
      expect(res.x).toBe(0.45);
      expect(res.y).toBe(0.60);
      expect(res.ratio).toBeGreaterThanOrEqual(0.020);
      expect(res.ratio).toBeLessThanOrEqual(0.55);
    });

    it('fits cluster of 30 nodes (e.g. Arquitetura) comfortably within 75-80% of screen', () => {
      // Simulates real notes/arquitetura span: spanX ~ 0.33, spanY ~ 0.32
      const points: Array<{ x: number; y: number }> = [];
      for (let i = 0; i < 30; i++) {
        points.push({
          x: 0.30 + (i / 30) * 0.33,
          y: 0.40 + (Math.sin(i) * 0.16),
        });
      }
      const res = calculateClusterFraming(points, viewport720p, { targetCoverageFraction: 0.78 });

      // Centroid must be centered within the cluster points
      expect(res.x).toBeGreaterThan(0.30);
      expect(res.x).toBeLessThan(0.65);

      // Ratio must be in medium zoom range (~0.35 to 0.48), NOT crushed to 0.085
      expect(res.ratio).toBeGreaterThanOrEqual(0.25);
      expect(res.ratio).toBeLessThanOrEqual(0.55);

      // Verify pixel span at this ratio
      const minDim = Math.min(viewport720p.width, viewport720p.height);
      const pixelWidth = (0.33 * minDim) / res.ratio;
      const pixelHeight = (0.32 * minDim) / res.ratio;
      expect(pixelWidth).toBeLessThanOrEqual(viewport720p.width);
      expect(pixelHeight).toBeLessThanOrEqual(viewport720p.height);
    });
  });

  describe('calculateNodeFocusCamera', () => {
    it('centers satellite strictly on itself with drawer compensation', () => {
      const satellite = { x: 0.65, y: 0.55 };
      const res = calculateNodeFocusCamera(satellite, true, [], viewport720p, { drawerWidth: 440 });

      // Satellite should have ratio 0.08
      expect(res.ratio).toBe(0.08);

      // y must match target exactly
      expect(res.y).toBe(satellite.y);

      // x must be shifted right in framed space (so viewport shifts left to openCenterX)
      const openCenterX = (viewport720p.width - 440) / 2; // 420
      const minDim = Math.min(viewport720p.width, viewport720p.height); // 720
      const expectedDx = (viewport720p.width / 2 - openCenterX) * (0.08 / minDim);
      expect(res.x).toBeCloseTo(satellite.x + expectedDx, 5);
    });

    it('ignores cross-galaxy outlier neighbors and focuses on local constellation', () => {
      const target = { x: 0.50, y: 0.50 };
      const neighbors = [
        { x: 0.52, y: 0.51 }, // local neighbor (dist ~ 0.02)
        { x: 0.48, y: 0.49 }, // local neighbor (dist ~ 0.02)
        { x: 0.95, y: 0.90 }, // cross-galaxy outlier (dist ~ 0.60)
      ];

      const res = calculateNodeFocusCamera(target, false, neighbors, viewport720p, { drawerWidth: 440 });

      // Anchor y remains firmly at target.y
      expect(res.y).toBe(target.y);

      // Ratio fits local neighborhood, not the extreme 0.95 outlier
      expect(res.ratio).toBeLessThanOrEqual(0.12);
      expect(res.ratio).toBeGreaterThanOrEqual(0.04);
    });
  });
});
