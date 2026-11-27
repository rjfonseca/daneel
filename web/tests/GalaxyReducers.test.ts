import { describe, it, expect } from 'vitest';
import { createGalaxyReducers, HIDDEN_EDGE } from '../src/lib/galaxy/galaxyReducers';
import type { SigmaContextState } from '../src/types/galaxy';

function createBaseContext(): SigmaContextState {
  return {
    sigma: null,
    frameLabelsCount: 0,
    frameCalloutBoxes: [],
    renderedNebulaCenters: [],
    maxNebulas: 6,
    focusedSector: null,
    focusedCluster: null,
    edgeMode: 'focus',
    hoveredNodeId: null,
    hoveredNeighborIds: new Set<string>(),
    selectedNodeId: null,
    constellation: null,
    showRings: true,
    isCosmicMode: true,
    showSectorLabels: true,
    showNebulaLabels: true,
    galaxyCenter: { x: 0, y: 0 },
    scopeMode: 'hybrid',
    lodState: null,
    superNodeRegistry: new Map(),
    superEdgeIndex: null,
  };
}

describe('galaxyReducers', () => {
  it('returns the exact shared HIDDEN_EDGE reference in focus mode with no hover/selection/focus (zero allocation)', () => {
    const ctx = createBaseContext();
    const { edgeReducer } = createGalaxyReducers(() => ctx);

    const r1 = edgeReducer('e1', { source: 'a', target: 'b' });
    const r2 = edgeReducer('e2', { source: 'c', target: 'd' });
    expect(r1).toBe(HIDDEN_EDGE);
    expect(r2).toBe(HIDDEN_EDGE);
    expect(r1.hidden).toBe(true);

    // Sigma v3 applyEdgeDefaults mutates label/type/zIndex/forceLabel in strict mode
    expect(() => {
      const mutable = r1 as typeof r1 & { label: string; type: string; forceLabel: boolean };
      mutable.label = '';
      mutable.type = 'line';
      mutable.zIndex = 0;
      mutable.forceLabel = false;
    }).not.toThrow();
  });

  it('highlights hovered neighbors via sigmaContext.hoveredNeighborIds without mutating graph attributes', () => {
    const ctx = createBaseContext();
    ctx.hoveredNodeId = 'n1';
    ctx.hoveredNeighborIds = new Set(['n2']);
    const { nodeReducer } = createGalaxyReducers(() => ctx);

    const n2 = nodeReducer('n2', { size: 2, trustScope: 'curated', sectorKey: 'notes/arch', clusterKey: 'notes/arch' });
    const n3 = nodeReducer('n3', { size: 2, trustScope: 'curated', sectorKey: 'notes/arch', clusterKey: 'notes/arch' });

    expect(n2.size).toBeCloseTo(2.8, 5);
    expect(n3.size).toBe(2);
  });

  it('filters edges inside focusedSector using precomputed sector keys in O(1)', () => {
    const ctx = createBaseContext();
    ctx.focusedSector = '__SECTOR_acme';
    const { edgeReducer } = createGalaxyReducers(() => ctx);

    const internalEdge = edgeReducer('e1', {
      source: 'sources/github/acme/orion/a.md',
      target: 'sources/github/acme/atlas/b.md',
      sourceSectorKey: 'acme',
      targetSectorKey: 'acme',
      sourceCategory: 'sources/github/acme/orion',
      targetCategory: 'sources/github/acme/atlas',
    });
    expect(internalEdge.hidden).toBe(false);

    const externalEdge = edgeReducer('e2', {
      source: 'sources/github/acme/orion/a.md',
      target: 'notes/architecture/x.md',
      sourceSectorKey: 'acme',
      targetSectorKey: 'notes/architecture',
      sourceCategory: 'sources/github/acme/orion',
      targetCategory: 'notes/architecture',
    });
    expect(externalEdge).toBe(HIDDEN_EDGE);
  });

  it('enforces ARMS toggle, soft cosmic dimming during constellation focus, and super-node hiding in nodeReducer', () => {
    const ctx = createBaseContext();
    ctx.showRings = false;
    const { nodeReducer, edgeReducer } = createGalaxyReducers(() => ctx);

    const hiddenSkill = nodeReducer('skill:foo', { ring: 'skills', size: 3 });
    expect(hiddenSkill.hidden).toBe(true);

    ctx.showRings = true;
    ctx.constellation = {
      centerNodeId: 'notes/a.md',
      visibleNodes: new Set(['notes/a.md', 'notes/b.md']),
      visibleEdges: new Set<string>(),
      hopDistances: new Map([['notes/a.md', 0], ['notes/b.md', 1]]),
      isNodeInConstellation: () => true,
      isEdgeInConstellation: () => true,
      getNodeDimmingAlpha: () => 1,
    };
    expect(nodeReducer('notes/a.md', { size: 2 }).highlighted).toBe(true);
    expect(nodeReducer('notes/b.md', { size: 2 }).forceLabel).toBe(true);

    // Non-constellation LOD-visible curated star stays softly dimmed instead of blacked out
    const dimmedCurated = nodeReducer('notes/c.md', { size: 2, trustScope: 'curated', sectorKey: 'notes/arch', clusterKey: 'notes/arch' });
    expect(dimmedCurated.hidden).toBe(false);
    expect(dimmedCurated.label).toBe('');
    expect(dimmedCurated.color).toBe('rgba(148, 163, 184, 0.18)');
    expect(dimmedCurated.size).toBeLessThan(2);

    // LOD-collapsed sources star remains hidden to preserve 12k-node performance
    ctx.lodState = {
      scopeMode: 'hybrid',
      tier: 'overview',
      expandedSectors: new Set(),
      expandedClusters: new Set(),
      collapsedSectorNodes: new Set(['__SECTOR_acme']),
      collapsedClusterNodes: new Set(),
      visibleStarCount: 100,
      totalStarCount: 1000,
      collapsedGroupCount: 1,
    };
    const collapsedSource = nodeReducer('sources/gitlab/acme/saas/app/readme.md', {
      size: 2,
      trustScope: 'sources',
      sectorKey: 'saas',
      clusterKey: 'sources/gitlab/acme/saas/app',
    });
    expect(collapsedSource.hidden).toBe(true);

    expect(nodeReducer('__SECTOR_acme', { size: 24 }).hidden).toBe(true);

    // Deep-zoom edge attenuation in constellation mode
    ctx.sigma = { getCamera: () => ({ ratio: 0.03 }) } as never;
    const deepEdge = edgeReducer('e1', { source: 'notes/a.md', target: 'notes/b.md' });
    expect(deepEdge.hidden).toBe(false);
    expect(deepEdge.size).toBeLessThan(0.15);
  });
});
