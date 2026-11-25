import { describe, it, expect } from 'vitest';
import {
  STELLAR_FRAGMENT_SHADER,
  computeStellarPixelMetrics,
} from '../src/lib/galaxy/stellarShader';

describe('NodeStellarProgram', () => {
  describe('computeStellarPixelMetrics', () => {
    it('converts Sigma framed-graph v_radius to true CSS pixels via u_correctionRatio', () => {
      // In Sigma.js v3, 1 CSS pixel in framed-graph units equals u_correctionRatio * 2.0
      const metrics = computeStellarPixelMetrics(0.0006, 0.00001);
      expect(metrics.radiusPx).toBeCloseTo(30, 5);
      expect(metrics.coronaPx).toBeCloseTo(3.5, 5);
      expect(metrics.discRadiusPx).toBeCloseTo(26.5, 5);
      expect(metrics.crispFraction).toBeGreaterThan(0.88);
    });

    it('keeps coronal halo strictly clamped in [1.5, 3.5] pixels across all zoom levels', () => {
      const radii = [4, 15, 30, 45, 120];
      const correctionRatio = 0.00001;
      const onePx = correctionRatio * 2;

      const metricsList = radii.map(px => computeStellarPixelMetrics(px * onePx, correctionRatio));

      for (const m of metricsList) {
        expect(m.coronaPx).toBeGreaterThanOrEqual(1.5);
        expect(m.coronaPx).toBeLessThanOrEqual(3.5);
      }

      // Crisp disc fraction must increase monotonically on deeper zoom (never pinned at 0.50)
      for (let i = 1; i < metricsList.length; i++) {
        expect(metricsList[i].crispFraction).toBeGreaterThanOrEqual(metricsList[i - 1].crispFraction);
      }
      expect(metricsList[3].crispFraction).toBeGreaterThan(0.90); // 45px
      expect(metricsList[4].crispFraction).toBeGreaterThan(0.96); // 120px
    });

    it('guards against zero or negative correctionRatio', () => {
      const m = computeStellarPixelMetrics(0.01, 0);
      expect(Number.isFinite(m.radiusPx)).toBe(true);
      expect(m.coronaPx).toBeGreaterThanOrEqual(1.5);
      expect(m.coronaPx).toBeLessThanOrEqual(3.5);
    });
  });

  describe('STELLAR_FRAGMENT_SHADER WebGL1 invariants', () => {
    it('uses u_correctionRatio for pixel conversion and preserves PICKING_MODE', () => {
      expect(STELLAR_FRAGMENT_SHADER).toContain('uniform float u_correctionRatio;');
      expect(STELLAR_FRAGMENT_SHADER).toContain('u_correctionRatio * 2.0');
      expect(STELLAR_FRAGMENT_SHADER).toContain('#ifdef PICKING_MODE');
      expect(STELLAR_FRAGMENT_SHADER).toContain('gl_FragColor');
    });

    it('contains zero WebGL2-only intrinsics', () => {
      expect(STELLAR_FRAGMENT_SHADER).not.toMatch(/\bfwidth\s*\(/);
      expect(STELLAR_FRAGMENT_SHADER).not.toMatch(/\bdFdx\s*\(/);
      expect(STELLAR_FRAGMENT_SHADER).not.toMatch(/\bdFdy\s*\(/);
      expect(STELLAR_FRAGMENT_SHADER).not.toMatch(/\btexture\s*\(/);
    });
  });
});
