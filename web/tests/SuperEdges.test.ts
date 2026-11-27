import { describe, it, expect } from 'vitest';
import { buildSuperEdgeIndex, resolveActiveSuperEdges } from '../src/lib/superEdges';
import type { SemanticLodState, SuperNodeMeta } from '../src/lib/semanticZoomLod';

describe('superEdges', () => {
  const nodes = [
    { id: 'notes/architecture/a.md', category: 'notes/architecture' },
    { id: 'notes/architecture/b.md', category: 'notes/architecture' },
    { id: 'sources/github/acme/orion/1.md', category: 'sources/github/acme/orion' },
    { id: 'sources/github/acme/orion/2.md', category: 'sources/github/acme/orion' },
    { id: 'sources/github/acme/atlas/1.md', category: 'sources/github/acme/atlas' },
    { id: 'sources/github/beta/nebula/1.md', category: 'sources/github/beta/nebula' },
    { id: 'skill:arch', ring: 'skills', category: 'arms/skills' },
  ];

  const links = [
    // Intra-cluster link (should be ignored)
    { source: 'notes/architecture/a.md', target: 'notes/architecture/b.md' },
    // Curated <-> Sources/acme/orion (weight 2)
    { source: 'notes/architecture/a.md', target: 'sources/github/acme/orion/1.md' },
    { source: 'notes/architecture/b.md', target: 'sources/github/acme/orion/2.md' },
    // Curated <-> Sources/acme/atlas (weight 1)
    { source: 'notes/architecture/a.md', target: 'sources/github/acme/atlas/1.md' },
    // Intra-sector cross-cluster inside acme: orion <-> atlas (weight 1)
    { source: 'sources/github/acme/orion/1.md', target: 'sources/github/acme/atlas/1.md' },
    // Cross-sector: acme/orion <-> beta/nebula (weight 1)
    { source: 'sources/github/acme/orion/2.md', target: 'sources/github/beta/nebula/1.md' },
    // ARMS link (should be ignored)
    { source: 'skill:arch', target: 'notes/architecture/a.md' },
  ];

  const superNodeRegistry = new Map<string, SuperNodeMeta>([
    ['__SECTOR_acme', { id: '__SECTOR_acme', kind: 'sector', key: 'acme', sectorKey: 'acme', displayName: 'ACME', color: '#2dd4bf', trustScope: 'sources', noteCount: 3, repoCount: 2, recentCount: 1, x: 100, y: 100 }],
    ['__SECTOR_beta', { id: '__SECTOR_beta', kind: 'sector', key: 'beta', sectorKey: 'beta', displayName: 'BETA', color: '#60a5fa', trustScope: 'sources', noteCount: 1, repoCount: 1, recentCount: 0, x: 500, y: 500 }],
    ['__CAT_sources/github/acme/orion', { id: '__CAT_sources/github/acme/orion', kind: 'cluster', key: 'sources/github/acme/orion', sectorKey: 'acme', displayName: 'ORION', color: '#2dd4bf', trustScope: 'sources', noteCount: 2, repoCount: 0, recentCount: 1, x: 90, y: 90 }],
    ['__CAT_sources/github/acme/atlas', { id: '__CAT_sources/github/acme/atlas', kind: 'cluster', key: 'sources/github/acme/atlas', sectorKey: 'acme', displayName: 'ATLAS', color: '#2dd4bf', trustScope: 'sources', noteCount: 1, repoCount: 0, recentCount: 0, x: 110, y: 110 }],
  ]);

  const getColor = (k: string) => (k.includes('acme') ? '#2dd4bf' : '#818cf8');

  it('builds cross-cluster buckets ignoring ARMS and intra-cluster links', () => {
    const index = buildSuperEdgeIndex(nodes, links);
    expect(index.buckets.length).toBe(4);
    const totalWeight = index.buckets.reduce((sum, b) => sum + b.weight, 0);
    expect(totalWeight).toBe(5);
  });

  it('aggregates bridges to sector super-nodes in Tier 0 overview and skips self-loops', () => {
    const index = buildSuperEdgeIndex(nodes, links);
    const overviewLod: SemanticLodState = {
      scopeMode: 'hybrid',
      tier: 'overview',
      expandedSectors: new Set(),
      expandedClusters: new Set(),
      collapsedSectorNodes: new Set(['__SECTOR_acme', '__SECTOR_beta']),
      collapsedClusterNodes: new Set(),
      visibleStarCount: 2,
      totalStarCount: 6,
      collapsedGroupCount: 2,
    };

    const active = resolveActiveSuperEdges(index, overviewLod, { superNodeRegistry, getColor });
    // Expected:
    // 1. __CAT_notes/architecture <-> __SECTOR_acme with combined weight 3 (2 orion + 1 atlas)
    // 2. __SECTOR_acme <-> __SECTOR_beta with weight 1
    // Intra-sector orion <-> atlas collapses into self-loop __SECTOR_acme <-> __SECTOR_acme and is omitted!
    expect(active).toHaveLength(2);
    const archToAcme = active.find(e => e.involvesCurated);
    expect(archToAcme).toBeDefined();
    expect(archToAcme!.weight).toBe(3);

    const acmeToBeta = active.find(e => !e.involvesCurated);
    expect(acmeToBeta).toBeDefined();
    expect(acmeToBeta!.weight).toBe(1);
  });

  it('unbundles bridges to cluster super-nodes when acme unfolds in Tier 1', () => {
    const index = buildSuperEdgeIndex(nodes, links);
    const sectorLod: SemanticLodState = {
      scopeMode: 'hybrid',
      tier: 'sector',
      expandedSectors: new Set(['acme']),
      expandedClusters: new Set(),
      collapsedSectorNodes: new Set(['__SECTOR_beta']),
      collapsedClusterNodes: new Set(['__CAT_sources/github/acme/orion', '__CAT_sources/github/acme/atlas']),
      visibleStarCount: 2,
      totalStarCount: 6,
      collapsedGroupCount: 3,
    };

    const active = resolveActiveSuperEdges(index, sectorLod, { superNodeRegistry, getColor });
    // Now orion <-> atlas is no longer a self-loop! Plus arch->orion (2), arch->atlas (1), orion->beta (1) = 4 bridges
    expect(active).toHaveLength(4);
    expect(active.some(e =>
      (e.sourceNodeId === '__CAT_sources/github/acme/orion' && e.targetNodeId === '__CAT_sources/github/acme/atlas') ||
      (e.targetNodeId === '__CAT_sources/github/acme/orion' && e.sourceNodeId === '__CAT_sources/github/acme/atlas')
    )).toBe(true);
  });

  it('returns empty bridges in curated and all scope modes', () => {
    const index = buildSuperEdgeIndex(nodes, links);
    const lodCurated: SemanticLodState = {
      scopeMode: 'curated',
      tier: 'overview',
      expandedSectors: new Set(),
      expandedClusters: new Set(),
      collapsedSectorNodes: new Set(),
      collapsedClusterNodes: new Set(),
      visibleStarCount: 2,
      totalStarCount: 6,
      collapsedGroupCount: 0,
    };
    expect(resolveActiveSuperEdges(index, lodCurated, { superNodeRegistry, getColor })).toEqual([]);
    expect(resolveActiveSuperEdges(index, { ...lodCurated, scopeMode: 'all' }, { superNodeRegistry, getColor })).toEqual([]);
  });
});
