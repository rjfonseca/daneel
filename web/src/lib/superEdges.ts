import { resolveProjectCategory } from './nebulas';
import {
  classifyTrustScope,
  type SemanticLodState,
  type SuperNodeMeta,
  type TrustScope,
} from './semanticZoomLod';

export interface SuperEdgeBucket {
  sourceCluster: string;
  targetCluster: string;
  sourceSector: string;
  targetSector: string;
  sourceScope: TrustScope;
  targetScope: TrustScope;
  weight: number;
}

export interface SuperEdgeIndex {
  buckets: SuperEdgeBucket[];
}

export interface ActiveSuperEdge {
  id: string;
  sourceNodeId: string;
  targetNodeId: string;
  weight: number;
  color: string;
  involvesCurated: boolean;
}

interface NodeMetaEntry {
  clusterKey: string;
  sectorKey: string;
  trustScope: TrustScope;
}

/**
 * Precomputes cross-cluster edge densities once at graph load without adding
 * artificial edges to Graphology (zero ForceAtlas2 physics distortion).
 */
export function buildSuperEdgeIndex(
  nodes: Array<{ id: string; category?: string; originalCategory?: string; ring?: string }>,
  links: Array<{ source: string; target: string }>
): SuperEdgeIndex {
  const nodeMeta = new Map<string, NodeMetaEntry>();
  for (const n of nodes) {
    const trustScope = classifyTrustScope(n);
    if (trustScope === 'arms') continue;
    const rawCat = (n.originalCategory || n.category || 'root').toLowerCase();
    const proj = resolveProjectCategory(n.id, rawCat);
    const clusterKey = (proj.category || rawCat || 'root').toLowerCase();
    const sectorKey = (proj.sector || rawCat || 'root').toLowerCase();
    nodeMeta.set(n.id, { clusterKey, sectorKey, trustScope });
  }

  const bucketMap = new Map<string, SuperEdgeBucket>();

  for (const l of links) {
    const src = nodeMeta.get(l.source);
    const tgt = nodeMeta.get(l.target);
    if (!src || !tgt) continue;
    if (src.clusterKey === tgt.clusterKey) continue;

    // Canonical undirected ordering
    const forward = src.clusterKey <= tgt.clusterKey;
    const a = forward ? src : tgt;
    const b = forward ? tgt : src;
    const key = `${a.clusterKey}::${b.clusterKey}`;

    const existing = bucketMap.get(key);
    if (existing) {
      existing.weight += 1;
    } else {
      bucketMap.set(key, {
        sourceCluster: a.clusterKey,
        targetCluster: b.clusterKey,
        sourceSector: a.sectorKey,
        targetSector: b.sectorKey,
        sourceScope: a.trustScope,
        targetScope: b.trustScope,
        weight: 1,
      });
    }
  }

  return {
    buckets: Array.from(bucketMap.values()),
  };
}

function resolveEndpointAnchor(
  clusterKey: string,
  sectorKey: string,
  trustScope: TrustScope,
  lodState: SemanticLodState
): { anchorId: string; isCollapsed: boolean } {
  if (trustScope === 'sources') {
    const secId = `__SECTOR_${sectorKey}`;
    if (lodState.collapsedSectorNodes.has(secId)) {
      return { anchorId: secId, isCollapsed: true };
    }
    const catId = `__CAT_${clusterKey}`;
    if (lodState.collapsedClusterNodes.has(catId)) {
      return { anchorId: catId, isCollapsed: true };
    }
    return { anchorId: catId, isCollapsed: false };
  }

  return { anchorId: `__CAT_${clusterKey}`, isCollapsed: false };
}

/**
 * Resolves currently visible weighted super-edges between collapsed Super-Nodes
 * and curated clusters for the active Semantic LOD state.
 */
export function resolveActiveSuperEdges(
  index: SuperEdgeIndex | null,
  lodState: SemanticLodState | null,
  options: {
    superNodeRegistry: Map<string, SuperNodeMeta>;
    getColor: (key: string) => string;
    maxSourceOnlyBridges?: number;
  }
): ActiveSuperEdge[] {
  if (!index || !lodState || lodState.scopeMode === 'all' || lodState.scopeMode === 'curated') {
    return [];
  }

  const maxSourceOnlyBridges = options.maxSourceOnlyBridges ?? 80;
  const aggregated = new Map<string, ActiveSuperEdge>();

  for (const bucket of index.buckets) {
    const src = resolveEndpointAnchor(
      bucket.sourceCluster,
      bucket.sourceSector,
      bucket.sourceScope,
      lodState
    );
    const tgt = resolveEndpointAnchor(
      bucket.targetCluster,
      bucket.targetSector,
      bucket.targetScope,
      lodState
    );

    // Only emit an aggregated bridge when at least one side is collapsed and endpoints differ
    if ((!src.isCollapsed && !tgt.isCollapsed) || src.anchorId === tgt.anchorId) {
      continue;
    }

    const forward = src.anchorId <= tgt.anchorId;
    const aId = forward ? src.anchorId : tgt.anchorId;
    const bId = forward ? tgt.anchorId : src.anchorId;
    const edgeId = `${aId}--${bId}`;
    const involvesCurated = bucket.sourceScope === 'curated' || bucket.targetScope === 'curated';

    const existing = aggregated.get(edgeId);
    if (existing) {
      existing.weight += bucket.weight;
      existing.involvesCurated = existing.involvesCurated || involvesCurated;
    } else {
      const colorAnchor = src.isCollapsed ? src.anchorId : tgt.anchorId;
      const registeredColor = options.superNodeRegistry.get(colorAnchor)?.color;
      const cleanKey = colorAnchor.replace(/^__SECTOR_/, '').replace(/^__CAT_/, '');
      aggregated.set(edgeId, {
        id: edgeId,
        sourceNodeId: aId,
        targetNodeId: bId,
        weight: bucket.weight,
        color: registeredColor || options.getColor(cleanKey),
        involvesCurated,
      });
    }
  }

  const curatedBridges: ActiveSuperEdge[] = [];
  const sourceOnlyBridges: ActiveSuperEdge[] = [];
  for (const edge of aggregated.values()) {
    if (edge.involvesCurated) {
      curatedBridges.push(edge);
    } else {
      sourceOnlyBridges.push(edge);
    }
  }

  sourceOnlyBridges.sort((a, b) => b.weight - a.weight);
  const cappedSourceOnly = sourceOnlyBridges.slice(0, maxSourceOnlyBridges);

  return [...curatedBridges, ...cappedSourceOnly];
}
