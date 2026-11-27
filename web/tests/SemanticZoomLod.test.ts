import { describe, it, expect, beforeEach } from 'vitest';
import {
  classifyTrustScope,
  computeZoomTier,
  computeSuperNodeRadius,
  computeSemanticLodState,
  isLodStateEqual,
  isMemoryNodeVisibleInLod,
  loadScopeMode,
  saveScopeMode,
  DEFAULT_SCOPE_MODE,
  STORAGE_KEY_SCOPE_MODE,
  MAX_EXPANDED_SECTORS,
  MAX_EXPANDED_CLUSTERS,
  type SuperNodeMeta,
  type SemanticLodState,
} from '../src/lib/semanticZoomLod';

describe('semanticZoomLod', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  describe('classifyTrustScope', () => {
    it('classifies ARMS orbital stations as arms', () => {
      expect(classifyTrustScope({ id: 'skill:arch', ring: 'skills' })).toBe('arms');
      expect(classifyTrustScope({ id: 'routine:dream', ring: 'routines' })).toBe('arms');
      expect(classifyTrustScope({ id: 'harvester:gitlab', ring: 'harvesters' })).toBe('arms');
      expect(classifyTrustScope({ id: 'custom', category: 'arms/skills' })).toBe('arms');
    });

    it('classifies sources/* notes as sources', () => {
      expect(classifyTrustScope({ id: 'sources/github/acme/orion/README.md', category: 'sources/github/acme/orion' })).toBe('sources');
      expect(classifyTrustScope({ id: 'foo.md', originalCategory: 'sources/gitlab/acme/saas' })).toBe('sources');
      expect(classifyTrustScope({ id: 'sources/jira/INC-1.md' })).toBe('sources');
    });

    it('classifies notes/, daily/, meta/, inbox/ as curated', () => {
      expect(classifyTrustScope({ id: 'notes/architecture/hexagonal.md', category: 'notes/architecture' })).toBe('curated');
      expect(classifyTrustScope({ id: 'daily/2026-10-16.md', category: 'daily' })).toBe('curated');
      expect(classifyTrustScope({ id: 'meta/adrs/0001.md', category: 'meta/adrs' })).toBe('curated');
      expect(classifyTrustScope({ id: 'inbox/draft.md', category: 'inbox' })).toBe('curated');
    });
  });

  describe('computeZoomTier (hysteresis)', () => {
    it('transitions overview -> sector -> cluster cleanly on zoom in', () => {
      expect(computeZoomTier(0.8, 'overview')).toBe('overview');
      expect(computeZoomTier(0.5, 'overview')).toBe('overview');
      expect(computeZoomTier(0.49, 'overview')).toBe('sector');
      expect(computeZoomTier(0.20, 'sector')).toBe('sector');
      expect(computeZoomTier(0.19, 'sector')).toBe('cluster');
      expect(computeZoomTier(0.15, 'overview')).toBe('cluster');
    });

    it('prevents boundary oscillation via hysteresis when zooming out', () => {
      // Between 0.50 and 0.58, stays in sector instead of flickering back to overview
      expect(computeZoomTier(0.54, 'sector')).toBe('sector');
      expect(computeZoomTier(0.59, 'sector')).toBe('overview');

      // Between 0.20 and 0.26, stays in cluster instead of flickering back to sector
      expect(computeZoomTier(0.24, 'cluster')).toBe('cluster');
      expect(computeZoomTier(0.27, 'cluster')).toBe('sector');
      expect(computeZoomTier(0.60, 'cluster')).toBe('overview');
    });
  });

  describe('computeSuperNodeRadius', () => {
    it('scales logarithmically within bounded pixel limits', () => {
      const smallSector = computeSuperNodeRadius('sector', 5);
      const bigSector = computeSuperNodeRadius('sector', 2500);
      expect(smallSector).toBeGreaterThanOrEqual(8);
      expect(bigSector).toBeLessThanOrEqual(22);
      expect(bigSector).toBeGreaterThan(smallSector);

      const smallCluster = computeSuperNodeRadius('cluster', 3);
      const bigCluster = computeSuperNodeRadius('cluster', 1000);
      expect(smallCluster).toBeGreaterThanOrEqual(6);
      expect(bigCluster).toBeLessThanOrEqual(15);
      expect(bigCluster).toBeGreaterThan(smallCluster);
    });
  });

  describe('computeSemanticLodState & viewport culling', () => {
    const sectors: SuperNodeMeta[] = [
      { id: '__SECTOR_acme', kind: 'sector', key: 'acme', sectorKey: 'acme', displayName: 'ACME', color: '#2dd4bf', trustScope: 'sources', noteCount: 120, repoCount: 2, recentCount: 4, x: 100, y: 100 },
      { id: '__SECTOR_far', kind: 'sector', key: 'far', sectorKey: 'far', displayName: 'FAR', color: '#f87171', trustScope: 'sources', noteCount: 80, repoCount: 1, recentCount: 0, x: 5000, y: 5000 },
    ];
    const clusters: SuperNodeMeta[] = [
      { id: '__CAT_sources/github/acme/orion', kind: 'cluster', key: 'sources/github/acme/orion', sectorKey: 'acme', displayName: 'ORION', color: '#2dd4bf', trustScope: 'sources', noteCount: 70, repoCount: 0, recentCount: 2, x: 100, y: 100 },
      { id: '__CAT_sources/github/acme/atlas', kind: 'cluster', key: 'sources/github/acme/atlas', sectorKey: 'acme', displayName: 'ATLAS', color: '#2dd4bf', trustScope: 'sources', noteCount: 50, repoCount: 0, recentCount: 1, x: 1600, y: 1600 },
      { id: '__CAT_sources/github/far/core', kind: 'cluster', key: 'sources/github/far/core', sectorKey: 'far', displayName: 'CORE', color: '#f87171', trustScope: 'sources', noteCount: 80, repoCount: 0, recentCount: 0, x: 5000, y: 5000 },
    ];
    const viewportDims = { width: 1000, height: 800 };
    const project = (x: number, y: number) => ({ x, y });

    it('collapses all source sectors in hybrid overview', () => {
      const lod = computeSemanticLodState({
        scopeMode: 'hybrid',
        cameraRatio: 0.85,
        viewportDims,
        previousState: null,
        sectors,
        clusters,
        curatedStarCount: 50,
        totalStarCount: 250,
        projectToViewport: project,
      });

      expect(lod.tier).toBe('overview');
      expect(lod.expandedSectors.size).toBe(0);
      expect(lod.collapsedSectorNodes.has('__SECTOR_acme')).toBe(true);
      expect(lod.collapsedSectorNodes.has('__SECTOR_far')).toBe(true);
      expect(lod.visibleStarCount).toBe(50);
    });

    it('unfolds in-viewport sector in Tier 1 while keeping off-screen sector collapsed', () => {
      const lod = computeSemanticLodState({
        scopeMode: 'hybrid',
        cameraRatio: 0.35,
        viewportDims,
        previousState: null,
        sectors,
        clusters,
        curatedStarCount: 50,
        totalStarCount: 250,
        projectToViewport: (x, _y) => (x === 100 ? { x: 500, y: 400 } : { x: 2500, y: 2500 }),
      });

      expect(lod.tier).toBe('sector');
      expect(lod.expandedSectors.has('acme')).toBe(true);
      expect(lod.expandedSectors.has('far')).toBe(false);
      expect(lod.collapsedSectorNodes.has('__SECTOR_acme')).toBe(false);
      expect(lod.collapsedSectorNodes.has('__SECTOR_far')).toBe(true);
      expect(lod.collapsedClusterNodes.has('__CAT_sources/github/acme/orion')).toBe(true);
      expect(lod.collapsedClusterNodes.has('__CAT_sources/github/acme/atlas')).toBe(true);
    });

    it('unfolds in-viewport cluster in Tier 2 and auto-collapses on pan-out past exit margin', () => {
      const tier2 = computeSemanticLodState({
        scopeMode: 'hybrid',
        cameraRatio: 0.14,
        viewportDims,
        previousState: null,
        sectors,
        clusters,
        curatedStarCount: 50,
        totalStarCount: 250,
        projectToViewport: (x) => {
          if (x === 100) return { x: 500, y: 400 };
          if (x === 1600) return { x: 1200, y: 400 }; // beyond +10% enter margin (1100)
          return { x: 4000, y: 4000 };
        },
      });

      expect(tier2.tier).toBe('cluster');
      expect(tier2.expandedClusters.has('sources/github/acme/orion')).toBe(true);
      expect(tier2.expandedClusters.has('sources/github/acme/atlas')).toBe(false);
      expect(tier2.collapsedClusterNodes.has('__CAT_sources/github/acme/atlas')).toBe(true);
      expect(tier2.visibleStarCount).toBe(120); // 50 curated + 70 orion

      // Pan slightly to x = 1200 (inside 35% exit margin = 1350): stays expanded (anti-popping)
      const pannedSlightly = computeSemanticLodState({
        scopeMode: 'hybrid',
        cameraRatio: 0.14,
        viewportDims,
        previousState: tier2,
        sectors,
        clusters,
        curatedStarCount: 50,
        totalStarCount: 250,
        projectToViewport: (x) => (x === 100 ? { x: 1200, y: 400 } : { x: 3000, y: 3000 }),
      });
      expect(pannedSlightly.expandedSectors.has('acme')).toBe(true);
      expect(pannedSlightly.expandedClusters.has('sources/github/acme/orion')).toBe(true);

      // Pan further to x = 1500 (past 35% exit margin = 1350): auto-collapses
      const pannedOut = computeSemanticLodState({
        scopeMode: 'hybrid',
        cameraRatio: 0.14,
        viewportDims,
        previousState: tier2,
        sectors,
        clusters,
        curatedStarCount: 50,
        totalStarCount: 250,
        projectToViewport: (x) => (x === 100 ? { x: 1500, y: 400 } : { x: 3000, y: 3000 }),
      });
      expect(pannedOut.expandedSectors.has('acme')).toBe(false);
      expect(pannedOut.expandedClusters.has('sources/github/acme/orion')).toBe(false);
      expect(pannedOut.collapsedSectorNodes.has('__SECTOR_acme')).toBe(true);
    });

    it('caps expanded sectors and clusters by budget prioritized by proximity to center', () => {
      const manySectors: SuperNodeMeta[] = Array.from({ length: 14 }, (_, i) => ({
        id: `__SECTOR_s${i}`,
        kind: 'sector',
        key: `s${i}`,
        sectorKey: `s${i}`,
        displayName: `S${i}`,
        color: '#2dd4bf',
        trustScope: 'sources',
        noteCount: 10,
        repoCount: 1,
        recentCount: 0,
        x: i * 10,
        y: 0,
      }));
      const manyClusters: SuperNodeMeta[] = Array.from({ length: 20 }, (_, i) => ({
        id: `__CAT_sources/s0/c${i}`,
        kind: 'cluster',
        key: `sources/s0/c${i}`,
        sectorKey: 's0',
        displayName: `C${i}`,
        color: '#2dd4bf',
        trustScope: 'sources',
        noteCount: 5,
        repoCount: 0,
        recentCount: 0,
        x: i * 10,
        y: 0,
      }));

      const lod = computeSemanticLodState({
        scopeMode: 'hybrid',
        cameraRatio: 0.12,
        viewportDims: { width: 1000, height: 800 },
        previousState: null,
        sectors: manySectors,
        clusters: manyClusters,
        curatedStarCount: 20,
        totalStarCount: 240,
        projectToViewport: (x) => ({ x: 500 + x, y: 400 }),
      });

      expect(lod.expandedSectors.size).toBe(MAX_EXPANDED_SECTORS);
      expect(lod.expandedSectors.has('s0')).toBe(true);
      expect(lod.expandedClusters.size).toBe(MAX_EXPANDED_CLUSTERS);
      expect(lod.expandedClusters.has('sources/s0/c0')).toBe(true);
    });

    it('keeps sector and cluster expanded via AABB bounds when centroid leaves viewport off-center', () => {
      const boundedSectors: SuperNodeMeta[] = [
        {
          id: '__SECTOR_acme',
          kind: 'sector',
          key: 'acme',
          sectorKey: 'acme',
          displayName: 'ACME',
          color: '#2dd4bf',
          trustScope: 'sources',
          noteCount: 500,
          repoCount: 3,
          recentCount: 4,
          x: 100,
          y: 100,
          minX: 50,
          maxX: 900,
          minY: 50,
          maxY: 700,
        },
        {
          id: '__SECTOR_other',
          kind: 'sector',
          key: 'other',
          sectorKey: 'other',
          displayName: 'OTHER',
          color: '#f87171',
          trustScope: 'sources',
          noteCount: 50,
          repoCount: 1,
          recentCount: 0,
          x: 600,
          y: 400,
          minX: 550,
          maxX: 650,
          minY: 350,
          maxY: 450,
        },
      ];
      const boundedClusters: SuperNodeMeta[] = [
        {
          id: '__CAT_sources/github/acme/orion',
          kind: 'cluster',
          key: 'sources/github/acme/orion',
          sectorKey: 'acme',
          displayName: 'ORION',
          color: '#2dd4bf',
          trustScope: 'sources',
          noteCount: 200,
          repoCount: 0,
          recentCount: 2,
          x: 100,
          y: 100,
          minX: 80,
          maxX: 850,
          minY: 80,
          maxY: 650,
        },
      ];

      // Centroid (100, 100) projects far off-screen (-2000, -1200),
      // while member stars bounding box (minX..maxX, minY..maxY) covers the viewport!
      const projectPan = (x: number, y: number) => ({
        x: x === 100 ? -2000 : x === 50 ? -2100 : x === 900 ? 1200 : x === 80 ? -1950 : x === 850 ? 1100 : x,
        y: y === 100 ? -1200 : y === 50 ? -1300 : y === 700 ? 900 : y === 80 ? -1150 : y === 650 ? 850 : y,
      });

      const lod = computeSemanticLodState({
        scopeMode: 'hybrid',
        cameraRatio: 0.12,
        viewportDims,
        previousState: null,
        sectors: boundedSectors,
        clusters: boundedClusters,
        curatedStarCount: 50,
        totalStarCount: 600,
        projectToViewport: projectPan,
      });

      expect(lod.expandedSectors.has('acme')).toBe(true);
      expect(lod.collapsedSectorNodes.has('__SECTOR_acme')).toBe(false);
      expect(lod.expandedClusters.has('sources/github/acme/orion')).toBe(true);
      expect(lod.collapsedClusterNodes.has('__CAT_sources/github/acme/orion')).toBe(false);
    });

    it('never collapses singleton source sectors or standalone clusters (noteCount <= 1) into Super-Nodes', () => {
      const singletonSectors: SuperNodeMeta[] = [
        { id: '__SECTOR_solo', kind: 'sector', key: 'solo', sectorKey: 'solo', displayName: 'SOLO', color: '#2dd4bf', trustScope: 'sources', noteCount: 1, repoCount: 1, recentCount: 0, x: 100, y: 100 },
        { id: '__SECTOR_multi', kind: 'sector', key: 'multi', sectorKey: 'multi', displayName: 'MULTI', color: '#38bdf8', trustScope: 'sources', noteCount: 25, repoCount: 2, recentCount: 0, x: 300, y: 300 },
      ];
      const singletonClusters: SuperNodeMeta[] = [
        { id: '__CAT_sources/standalone/single', kind: 'cluster', key: 'sources/standalone/single', sectorKey: 'standalone', displayName: 'SINGLE', color: '#a78bfa', trustScope: 'sources', noteCount: 1, repoCount: 0, recentCount: 0, x: 400, y: 400 },
        { id: '__CAT_sources/multi/one', kind: 'cluster', key: 'sources/multi/one', sectorKey: 'multi', displayName: 'ONE', color: '#38bdf8', trustScope: 'sources', noteCount: 1, repoCount: 0, recentCount: 0, x: 320, y: 320 },
      ];

      const overviewLod = computeSemanticLodState({
        scopeMode: 'hybrid',
        cameraRatio: 0.9,
        viewportDims,
        previousState: null,
        sectors: singletonSectors,
        clusters: singletonClusters,
        curatedStarCount: 50,
        totalStarCount: 77,
        projectToViewport: project,
      });

      expect(overviewLod.collapsedSectorNodes.has('__SECTOR_solo')).toBe(false);
      expect(overviewLod.expandedSectors.has('solo')).toBe(true);
      expect(overviewLod.collapsedClusterNodes.has('__CAT_sources/standalone/single')).toBe(false);
      expect(overviewLod.expandedClusters.has('sources/standalone/single')).toBe(true);
      expect(overviewLod.collapsedSectorNodes.has('__SECTOR_multi')).toBe(true);
      // Singleton stars are visible directly in overview without fake 1-star Super-Node aggregation
      expect(overviewLod.visibleStarCount).toBe(52);
      expect(
        isMemoryNodeVisibleInLod(
          { trustScope: 'sources', sectorKey: 'solo', clusterKey: 'sources/solo/leaf' },
          overviewLod
        )
      ).toBe(true);
      expect(
        isMemoryNodeVisibleInLod(
          { trustScope: 'sources', sectorKey: 'standalone', clusterKey: 'sources/standalone/single' },
          overviewLod
        )
      ).toBe(true);

      // Even at sector tier, a singleton child cluster inside an expanded sector should not collapse into a 1-star orb
      const sectorLod = computeSemanticLodState({
        scopeMode: 'hybrid',
        cameraRatio: 0.35,
        viewportDims,
        previousState: overviewLod,
        sectors: singletonSectors,
        clusters: singletonClusters,
        curatedStarCount: 50,
        totalStarCount: 77,
        projectToViewport: project,
      });
      expect(sectorLod.collapsedClusterNodes.has('__CAT_sources/multi/one')).toBe(false);
      expect(sectorLod.expandedClusters.has('sources/multi/one')).toBe(true);
    });

    it('handles curated and all scope modes properly', () => {
      const curatedLod = computeSemanticLodState({
        scopeMode: 'curated',
        cameraRatio: 0.10,
        viewportDims,
        previousState: null,
        sectors,
        clusters,
        curatedStarCount: 50,
        totalStarCount: 250,
        projectToViewport: project,
      });
      expect(curatedLod.visibleStarCount).toBe(50);
      expect(curatedLod.collapsedGroupCount).toBe(0);

      const allLod = computeSemanticLodState({
        scopeMode: 'all',
        cameraRatio: 1.0,
        viewportDims,
        previousState: null,
        sectors,
        clusters,
        curatedStarCount: 50,
        totalStarCount: 250,
        projectToViewport: project,
      });
      expect(allLod.visibleStarCount).toBe(250);
      expect(allLod.collapsedGroupCount).toBe(0);
    });
  });

  describe('isMemoryNodeVisibleInLod & isLodStateEqual', () => {
    const baseState: SemanticLodState = {
      scopeMode: 'hybrid',
      tier: 'sector',
      expandedSectors: new Set(['acme']),
      expandedClusters: new Set(),
      collapsedSectorNodes: new Set(['__SECTOR_far']),
      collapsedClusterNodes: new Set(['__CAT_sources/github/acme/orion']),
      visibleStarCount: 50,
      totalStarCount: 200,
      collapsedGroupCount: 2,
    };

    it('keeps curated and ARMS nodes always visible in hybrid mode', () => {
      expect(isMemoryNodeVisibleInLod({ trustScope: 'curated', sectorKey: 'notes/arch', clusterKey: 'notes/arch' }, baseState)).toBe(true);
      expect(isMemoryNodeVisibleInLod({ trustScope: 'arms', sectorKey: '', clusterKey: '' }, baseState)).toBe(true);
    });

    it('shows only high-signal source stars in Tier 1 expanded sector and hides regular leaf stars', () => {
      expect(isMemoryNodeVisibleInLod({ trustScope: 'sources', sectorKey: 'acme', clusterKey: 'sources/github/acme/orion', isClusterAlpha: true }, baseState)).toBe(true);
      expect(isMemoryNodeVisibleInLod({ trustScope: 'sources', sectorKey: 'acme', clusterKey: 'sources/github/acme/orion', connections: 6 }, baseState)).toBe(true);
      expect(isMemoryNodeVisibleInLod({ trustScope: 'sources', sectorKey: 'acme', clusterKey: 'sources/github/acme/orion', connections: 1 }, baseState)).toBe(false);
      expect(isMemoryNodeVisibleInLod({ trustScope: 'sources', sectorKey: 'far', clusterKey: 'sources/github/far/core', isClusterAlpha: true }, baseState)).toBe(false);
    });

    it('hides all source stars in curated scope mode', () => {
      const curatedOnly: SemanticLodState = { ...baseState, scopeMode: 'curated' };
      expect(isMemoryNodeVisibleInLod({ trustScope: 'sources', sectorKey: 'acme', clusterKey: 'sources/github/acme/orion', isClusterAlpha: true }, curatedOnly)).toBe(false);
      expect(isMemoryNodeVisibleInLod({ trustScope: 'curated', sectorKey: 'notes/arch', clusterKey: 'notes/arch' }, curatedOnly)).toBe(true);
    });

    it('checks structural equality accurately', () => {
      const copy: SemanticLodState = {
        ...baseState,
        expandedSectors: new Set(['acme']),
        expandedClusters: new Set(),
        collapsedSectorNodes: new Set(['__SECTOR_far']),
        collapsedClusterNodes: new Set(['__CAT_sources/github/acme/orion']),
      };
      expect(isLodStateEqual(baseState, copy)).toBe(true);
      copy.expandedSectors.add('other');
      expect(isLodStateEqual(baseState, copy)).toBe(false);
    });
  });

  describe('scope persistence', () => {
    it('defaults to hybrid and persists valid values', () => {
      expect(loadScopeMode()).toBe(DEFAULT_SCOPE_MODE);
      saveScopeMode('curated');
      expect(localStorage.getItem(STORAGE_KEY_SCOPE_MODE)).toBe('curated');
      expect(loadScopeMode()).toBe('curated');
      localStorage.setItem(STORAGE_KEY_SCOPE_MODE, 'invalid');
      expect(loadScopeMode()).toBe(DEFAULT_SCOPE_MODE);
    });
  });
});
