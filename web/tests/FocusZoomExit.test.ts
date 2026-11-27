import { describe, it, expect } from 'vitest';
import {
  calculateFocusExitRatio,
  evaluateFocusZoomExit,
} from '../src/lib/galaxy/focusZoomExit';

describe('focusZoomExit', () => {
  describe('calculateFocusExitRatio', () => {
    it('scales proportionally with focus ratio while respecting bounds', () => {
      // Cluster bounds: minExit = 0.16, maxCap = 0.58
      expect(calculateFocusExitRatio(0.06, 'cluster')).toBeCloseTo(0.16, 4);
      expect(calculateFocusExitRatio(0.15, 'cluster')).toBeCloseTo(0.2475, 4);
      expect(calculateFocusExitRatio(0.40, 'cluster')).toBeCloseTo(0.58, 4);

      // Sector bounds: minExit = 0.18, maxCap = 0.68
      expect(calculateFocusExitRatio(0.10, 'sector')).toBeCloseTo(0.18, 4);
      expect(calculateFocusExitRatio(0.20, 'sector')).toBeCloseTo(0.33, 4);
      expect(calculateFocusExitRatio(0.35, 'sector')).toBeCloseTo(0.5775, 4);
      expect(calculateFocusExitRatio(0.45, 'sector')).toBeCloseTo(0.68, 4);
      expect(calculateFocusExitRatio(0.55, 'sector')).toBeCloseTo(0.6875, 4);
    });
  });

  describe('evaluateFocusZoomExit', () => {
    it('returns none when no sector/cluster is focused, during entry flight, when zooming in, or with active constellation', () => {
      expect(evaluateFocusZoomExit({
        focusedSector: null,
        focusedCluster: null,
        hasActiveConstellation: false,
        isProgrammaticFocusFlight: false,
        currentRatio: 0.90,
        previousRatio: 0.50,
        sectorAnchorRatio: 0.30,
        clusterAnchorRatio: null,
      })).toEqual({ type: 'none' });

      expect(evaluateFocusZoomExit({
        focusedSector: '__SECTOR_acme',
        focusedCluster: null,
        hasActiveConstellation: false,
        isProgrammaticFocusFlight: true,
        currentRatio: 0.90,
        previousRatio: 0.50,
        sectorAnchorRatio: 0.30,
        clusterAnchorRatio: null,
      })).toEqual({ type: 'none' });

      expect(evaluateFocusZoomExit({
        focusedSector: '__SECTOR_acme',
        focusedCluster: null,
        hasActiveConstellation: true,
        isProgrammaticFocusFlight: false,
        currentRatio: 0.16,
        previousRatio: 0.10,
        sectorAnchorRatio: 0.30,
        clusterAnchorRatio: null,
        constellationAnchorRatio: 0.08,
      })).toEqual({ type: 'none' });

      // Zooming in (ratio decreasing) must never unfocus
      expect(evaluateFocusZoomExit({
        focusedSector: '__SECTOR_acme',
        focusedCluster: null,
        hasActiveConstellation: false,
        isProgrammaticFocusFlight: false,
        currentRatio: 0.50,
        previousRatio: 0.60,
        sectorAnchorRatio: 0.30,
        clusterAnchorRatio: null,
      })).toEqual({ type: 'none' });
    });

    it('steps up hierarchically: Cluster -> Sector -> Full Galaxy', () => {
      // Cluster anchor 0.14 -> exit at 0.231; Sector anchor 0.35 -> exit at 0.5775
      expect(evaluateFocusZoomExit({
        focusedSector: '__SECTOR_acme',
        focusedCluster: '__CAT_sources/github/acme/orion',
        hasActiveConstellation: false,
        isProgrammaticFocusFlight: false,
        currentRatio: 0.20,
        previousRatio: 0.16,
        sectorAnchorRatio: 0.35,
        clusterAnchorRatio: 0.14,
      })).toEqual({ type: 'none' });

      // Step 1: zooming out past 0.231 exits cluster, keeps sector
      expect(evaluateFocusZoomExit({
        focusedSector: '__SECTOR_acme',
        focusedCluster: '__CAT_sources/github/acme/orion',
        hasActiveConstellation: false,
        isProgrammaticFocusFlight: false,
        currentRatio: 0.28,
        previousRatio: 0.22,
        sectorAnchorRatio: 0.35,
        clusterAnchorRatio: 0.14,
      })).toEqual({ type: 'exit-cluster' });

      // Step 2: zooming out further past 0.5775 exits sector
      expect(evaluateFocusZoomExit({
        focusedSector: '__SECTOR_acme',
        focusedCluster: null,
        hasActiveConstellation: false,
        isProgrammaticFocusFlight: false,
        currentRatio: 0.60,
        previousRatio: 0.45,
        sectorAnchorRatio: 0.35,
        clusterAnchorRatio: null,
      })).toEqual({ type: 'exit-all' });
    });

    it('exits both cluster and sector on a single large zoom-out past sector threshold', () => {
      expect(evaluateFocusZoomExit({
        focusedSector: '__SECTOR_acme',
        focusedCluster: '__CAT_sources/github/acme/orion',
        hasActiveConstellation: false,
        isProgrammaticFocusFlight: false,
        currentRatio: 0.70,
        previousRatio: 0.20,
        sectorAnchorRatio: 0.35,
        clusterAnchorRatio: 0.14,
      })).toEqual({ type: 'exit-all' });
    });

    it('keeps clear separation between cluster exit and sector exit even for tight sectors', () => {
      expect(evaluateFocusZoomExit({
        focusedSector: '__SECTOR_acme',
        focusedCluster: '__CAT_sources/github/acme/orion',
        hasActiveConstellation: false,
        isProgrammaticFocusFlight: false,
        currentRatio: 0.19,
        previousRatio: 0.14,
        sectorAnchorRatio: 0.12,
        clusterAnchorRatio: 0.10,
      })).toEqual({ type: 'exit-cluster' });

      expect(evaluateFocusZoomExit({
        focusedSector: '__SECTOR_acme',
        focusedCluster: null,
        hasActiveConstellation: false,
        isProgrammaticFocusFlight: false,
        currentRatio: 0.25,
        previousRatio: 0.19,
        sectorAnchorRatio: 0.12,
        clusterAnchorRatio: null,
      })).toEqual({ type: 'exit-all' });
    });

    it('exits standalone cluster without parent sector directly via exit-all', () => {
      expect(evaluateFocusZoomExit({
        focusedSector: null,
        focusedCluster: '__CAT_daily',
        hasActiveConstellation: false,
        isProgrammaticFocusFlight: false,
        currentRatio: 0.30,
        previousRatio: 0.16,
        sectorAnchorRatio: null,
        clusterAnchorRatio: 0.16,
      })).toEqual({ type: 'exit-all' });
    });

    it('steps up from active constellation via exit-constellation without leaving underlying sector on first step', () => {
      // Below threshold: stay in constellation
      expect(evaluateFocusZoomExit({
        focusedSector: '__SECTOR_acme',
        focusedCluster: null,
        hasActiveConstellation: true,
        isProgrammaticFocusFlight: false,
        currentRatio: 0.16,
        previousRatio: 0.08,
        sectorAnchorRatio: 0.40,
        clusterAnchorRatio: null,
        constellationAnchorRatio: 0.08,
      })).toEqual({ type: 'none' });

      // Crossing constellation threshold releases only the constellation
      expect(evaluateFocusZoomExit({
        focusedSector: '__SECTOR_acme',
        focusedCluster: null,
        hasActiveConstellation: true,
        isProgrammaticFocusFlight: false,
        currentRatio: 0.28,
        previousRatio: 0.18,
        sectorAnchorRatio: 0.40,
        clusterAnchorRatio: null,
        constellationAnchorRatio: 0.08,
      })).toEqual({ type: 'exit-constellation' });
    });
  });
});
