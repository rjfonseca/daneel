import { describe, expect, it } from 'vitest';
import { computeLayoutSettlePlan } from '../src/lib/galaxy/layoutTiming';

describe('computeLayoutSettlePlan', () => {
  it('uses fast 1800ms ForceAtlas2 settle time for small cold-start vaults (<= 250 nodes)', () => {
    const plan = computeLayoutSettlePlan({
      nodeCount: 82,
      cachedCoverageRatio: 0,
      reducedMotion: false,
      shouldAnimateReset: true,
    });
    expect(plan).toEqual({
      forceAtlasMs: 1800,
      postStopDelayMs: 300,
      resetDurationMs: 1500,
      readyDelayAfterResetMs: 1500,
    });
  });

  it('preserves full 8500ms ForceAtlas2 settle time for large cold-start vaults (>= 2000 nodes)', () => {
    const large = computeLayoutSettlePlan({
      nodeCount: 12263,
      cachedCoverageRatio: 0,
      reducedMotion: false,
      shouldAnimateReset: true,
    });
    const boundary = computeLayoutSettlePlan({
      nodeCount: 2000,
      cachedCoverageRatio: 0,
      reducedMotion: false,
      shouldAnimateReset: true,
    });
    expect(large.forceAtlasMs).toBe(8500);
    expect(boundary.forceAtlasMs).toBe(8500);
  });

  it('interpolates monotonically between small and large vaults', () => {
    const smallBoundary = computeLayoutSettlePlan({
      nodeCount: 250,
      cachedCoverageRatio: 0,
      reducedMotion: false,
      shouldAnimateReset: true,
    });
    const lowMid = computeLayoutSettlePlan({
      nodeCount: 600,
      cachedCoverageRatio: 0,
      reducedMotion: false,
      shouldAnimateReset: true,
    });
    const exactMid = computeLayoutSettlePlan({
      nodeCount: 1125,
      cachedCoverageRatio: 0,
      reducedMotion: false,
      shouldAnimateReset: true,
    });
    const highMid = computeLayoutSettlePlan({
      nodeCount: 1600,
      cachedCoverageRatio: 0,
      reducedMotion: false,
      shouldAnimateReset: true,
    });

    expect(smallBoundary.forceAtlasMs).toBe(1800);
    expect(lowMid.forceAtlasMs).toBeGreaterThan(1800);
    expect(exactMid.forceAtlasMs).toBe(5150);
    expect(highMid.forceAtlasMs).toBeGreaterThan(exactMid.forceAtlasMs);
    expect(highMid.forceAtlasMs).toBeLessThan(8500);
  });

  it('shortens ForceAtlas2 settle time when layout cache covers nodes', () => {
    const fullCache = computeLayoutSettlePlan({
      nodeCount: 5000,
      cachedCoverageRatio: 0.95,
      reducedMotion: false,
      shouldAnimateReset: true,
    });
    const thresholdCache = computeLayoutSettlePlan({
      nodeCount: 5000,
      cachedCoverageRatio: 0.9,
      reducedMotion: false,
      shouldAnimateReset: true,
    });
    const partialCache = computeLayoutSettlePlan({
      nodeCount: 5000,
      cachedCoverageRatio: 0.5,
      reducedMotion: false,
      shouldAnimateReset: true,
    });

    expect(fullCache.forceAtlasMs).toBe(400);
    expect(thresholdCache.forceAtlasMs).toBe(400);
    expect(partialCache.forceAtlasMs).toBe(1200);
  });

  it('skips dead animation delays when reducedMotion is enabled or reset is skipped', () => {
    const reduced = computeLayoutSettlePlan({
      nodeCount: 82,
      cachedCoverageRatio: 0,
      reducedMotion: true,
      shouldAnimateReset: true,
    });
    expect(reduced).toEqual({
      forceAtlasMs: 1800,
      postStopDelayMs: 50,
      resetDurationMs: 1,
      readyDelayAfterResetMs: 50,
    });

    const noReset = computeLayoutSettlePlan({
      nodeCount: 82,
      cachedCoverageRatio: 0,
      reducedMotion: false,
      shouldAnimateReset: false,
    });
    expect(noReset).toEqual({
      forceAtlasMs: 1800,
      postStopDelayMs: 50,
      resetDurationMs: 0,
      readyDelayAfterResetMs: 50,
    });
  });

  it('handles out-of-range or NaN inputs safely', () => {
    const plan = computeLayoutSettlePlan({
      nodeCount: -10,
      cachedCoverageRatio: Number.NaN,
      reducedMotion: false,
      shouldAnimateReset: true,
    });
    expect(plan.forceAtlasMs).toBe(1800);

    const overCached = computeLayoutSettlePlan({
      nodeCount: 0,
      cachedCoverageRatio: 2.5,
      reducedMotion: false,
      shouldAnimateReset: true,
    });
    expect(overCached.forceAtlasMs).toBe(400);
  });
});
