/**
 * @file galaxyReducers.ts
 * Deep module encapsulating Sigma nodeReducer and edgeReducer with precomputed
 * sector/cluster keys and zero-allocation fast paths for hidden edges.
 */

import type { NodeDisplayData } from 'sigma/types';
import { computeEdgeDisplay, type EdgeData, type ReducedEdgeAttributes } from '../edgeFiltering';
import { isRootNoteCategory } from '../nebulas';
import { computeSuperNodeRadius, isMemoryNodeVisibleInLod, type TrustScope } from '../semanticZoomLod';
import type { GalaxyLink, GalaxyNode, SigmaContextState } from '../../types/galaxy';
import { matchesClusterFilter, matchesSectorFilter, normalizeFocusTargets } from './sectorVisibility';

export const HIDDEN_EDGE: ReducedEdgeAttributes & {
  label: string;
  forceLabel: boolean;
  type: string;
} = {
  hidden: true,
  color: 'rgba(0,0,0,0)',
  size: 0.1,
  zIndex: 0,
  label: '',
  forceLabel: false,
  type: 'line',
};

type RawNodeData = Partial<GalaxyNode> & Partial<NodeDisplayData> & {
  isRoot?: boolean;
  alwaysRenderLabel?: boolean;
  neonColor?: string;
  noteCount?: number;
};

type RawEdgeData = Partial<GalaxyLink> & {
  sourceSectorKey?: string;
  targetSectorKey?: string;
  sourceClusterKey?: string;
  targetClusterKey?: string;
};

function isNodeInActiveScope(nodeData: RawNodeData, ctx: SigmaContextState): boolean {
  if (ctx.focusedCluster) {
    const { clusterCat, cleanCluster } = normalizeFocusTargets(null, ctx.focusedCluster);
    const noteCat = String(nodeData.originalCategory || nodeData.category || 'root').toLowerCase();
    const clusterKey = typeof nodeData.clusterKey === 'string' ? nodeData.clusterKey.toLowerCase() : undefined;
    return matchesClusterFilter(noteCat, clusterKey, clusterCat, cleanCluster);
  }
  if (ctx.focusedSector) {
    const { sectorCat, cleanSector } = normalizeFocusTargets(ctx.focusedSector, null);
    const noteCat = String(nodeData.originalCategory || nodeData.category || 'root').toLowerCase();
    const preSector = typeof nodeData.sectorKey === 'string' ? nodeData.sectorKey.toLowerCase() : undefined;
    return matchesSectorFilter(noteCat, preSector, sectorCat, cleanSector);
  }
  if (!ctx.lodState) return true;
  return isMemoryNodeVisibleInLod(
    {
      trustScope: (nodeData.trustScope as TrustScope) || 'curated',
      sectorKey: String(nodeData.sectorKey || ''),
      clusterKey: String(nodeData.clusterKey || ''),
      isClusterAlpha: Boolean(nodeData.isClusterAlpha),
      stellarClass: nodeData.stellarClass as string | undefined,
      connections: Number(nodeData.connections) || 0,
    },
    ctx.lodState
  );
}

export function createGalaxyReducers(getContext: () => SigmaContextState) {
  const nodeReducer = (node: string, dataAttrs: RawNodeData) => {
    const ctx = getContext();
    const base = { ...dataAttrs, key: node };
    const isHoveredNeighbor = Boolean(ctx.hoveredNeighborIds?.has(node));
    const applyHoverBoost = (size: number) => (isHoveredNeighbor ? size * 1.4 : size);

    // 1. ARMS Toggle: hide foundation stations & rings when disabled
    if (!ctx.showRings && (base.ring === 'skills' || base.ring === 'routines' || base.ring === 'harvesters')) {
      return { ...base, hidden: true, label: '' };
    }

    // 2. On-demand Constellation focus with Soft Cosmic Dimming for context stars
    const constel = ctx.constellation;
    if (constel) {
      if (node.startsWith('__CAT_') || node.startsWith('__SECTOR_')) {
        return { ...base, hidden: true, label: '' };
      }
      if (node === constel.centerNodeId) {
        return {
          ...base,
          hidden: false,
          zIndex: 35,
          size: ((base.size as number) || 1) * 1.35,
          highlighted: true,
          forceLabel: true,
        };
      }
      if (constel.visibleNodes.has(node)) {
        const hop = constel.hopDistances.get(node);
        const hopSize = hop === 1 ? ((base.size as number) || 1) * 1.15 : ((base.size as number) || 1);
        return {
          ...base,
          hidden: false,
          zIndex: hop === 1 ? 25 : 15,
          size: applyHoverBoost(hopSize),
          forceLabel: hop === 1,
        };
      }
      if (!isNodeInActiveScope(base, ctx)) {
        return { ...base, hidden: true, label: '' };
      }
      return {
        ...base,
        hidden: false,
        color: 'rgba(148, 163, 184, 0.18)',
        size: Math.max(1.1, ((base.size as number) || 1.5) * 0.65),
        zIndex: 1,
        label: '',
        forceLabel: false,
      };
    }

    // 3. Sector & Category Super-Nodes
    if (node.startsWith('__CAT_') || node.startsWith('__SECTOR_')) {
      if (ctx.scopeMode === 'curated' && base.trustScope === 'sources') {
        return { ...base, hidden: true, label: '' };
      }
      const lod = ctx.lodState;
      if (node.startsWith('__SECTOR_')) {
        if (!ctx.focusedSector && !ctx.focusedCluster && lod?.collapsedSectorNodes.has(node)) {
          return {
            ...base,
            hidden: false,
            color: base.neonColor || '#38bdf8',
            size: computeSuperNodeRadius('sector', Number(base.noteCount) || 1),
            forceLabel: base.forceLabel ?? true,
            alwaysRenderLabel: base.alwaysRenderLabel ?? true,
            zIndex: 25,
          };
        }
        return {
          ...base,
          forceLabel: base.forceLabel ?? true,
          alwaysRenderLabel: base.alwaysRenderLabel ?? true,
          zIndex: 25,
        };
      }
      if (node.startsWith('__CAT_')) {
        const catName = node.replace('__CAT_', '');
        const isRoot = isRootNoteCategory(catName) || Boolean(base.isRoot);
        if (!ctx.focusedSector && !ctx.focusedCluster && lod?.collapsedClusterNodes.has(node)) {
          return {
            ...base,
            hidden: false,
            color: base.neonColor || '#38bdf8',
            size: computeSuperNodeRadius('cluster', Number(base.noteCount) || 1),
            zIndex: 22,
          };
        }
        if (isRoot) {
          return {
            ...base,
            forceLabel: true,
            alwaysRenderLabel: true,
            labelDensityWeight: 35000,
            zIndex: 30,
          };
        }
      }
      return base;
    }

    // 4. Focused Cluster
    if (ctx.focusedCluster) {
      const { clusterCat, cleanCluster } = normalizeFocusTargets(null, ctx.focusedCluster);
      const noteCat = String(base.originalCategory || base.category || 'root').toLowerCase();
      const clusterKey = typeof base.clusterKey === 'string' ? base.clusterKey.toLowerCase() : undefined;
      if (!matchesClusterFilter(noteCat, clusterKey, clusterCat, cleanCluster)) {
        return { ...base, hidden: true, label: '' };
      }
      return {
        ...base,
        size: applyHoverBoost(Math.max(3.5, ((base.size as number) || 2) * 1.5)),
        zIndex: isHoveredNeighbor ? 25 : 15,
      };
    }

    // 5. Focused Sector
    if (ctx.focusedSector) {
      const { sectorCat, cleanSector } = normalizeFocusTargets(ctx.focusedSector, null);
      const noteCat = String(base.originalCategory || base.category || 'root').toLowerCase();
      const preSector = typeof base.sectorKey === 'string' ? base.sectorKey.toLowerCase() : undefined;
      if (!matchesSectorFilter(noteCat, preSector, sectorCat, cleanSector)) {
        return { ...base, hidden: true, label: '' };
      }
      return {
        ...base,
        size: applyHoverBoost(Math.max(3.5, ((base.size as number) || 2) * 1.5)),
        zIndex: isHoveredNeighbor ? 25 : 15,
      };
    }

    // 6. ARMS Stations
    if (base.ring === 'skills' || base.ring === 'routines' || base.ring === 'harvesters') {
      return {
        ...base,
        size: applyHoverBoost((base.size as number) || 2),
        forceLabel: true,
        zIndex: 18,
      };
    }

    // 7. Semantic Zoom LOD (O(1) lookup)
    if (node !== ctx.hoveredNodeId && node !== ctx.selectedNodeId) {
      const visibleInLod = isMemoryNodeVisibleInLod(
        {
          trustScope: (base.trustScope as TrustScope) || 'curated',
          sectorKey: String(base.sectorKey || ''),
          clusterKey: String(base.clusterKey || ''),
          isClusterAlpha: Boolean(base.isClusterAlpha),
          stellarClass: base.stellarClass as string | undefined,
          connections: Number(base.connections) || 0,
        },
        ctx.lodState
      );
      if (!visibleInLod) {
        return { ...base, hidden: true, label: '' };
      }
    }

    if (isHoveredNeighbor) {
      return {
        ...base,
        size: ((base.size as number) || 2) * 1.4,
        zIndex: 25,
      };
    }

    return base;
  };

  const edgeReducer = (edge: string, dataAttrs: RawEdgeData) => {
    if (dataAttrs.originalHidden === true || edge.startsWith('__CAT_') || edge.startsWith('__SECTOR_')) {
      return HIDDEN_EDGE;
    }

    const ctx = getContext();

    if (!ctx.showRings && (dataAttrs.isOrbital || dataAttrs.ring)) {
      return HIDDEN_EDGE;
    }

    const constel = ctx.constellation;
    if (constel) {
      const s = dataAttrs.source as string;
      const t = dataAttrs.target as string;
      if (constel.visibleNodes.has(s) && constel.visibleNodes.has(t)) {
        const isDirectCenter = s === constel.centerNodeId || t === constel.centerNodeId;
        const cameraRatio = ctx.sigma?.getCamera()?.ratio ?? 0.1;
        const zoomScale = Math.max(0.28, Math.min(1.0, cameraRatio / 0.12));
        return {
          hidden: false,
          color: isDirectCenter ? 'rgba(186, 230, 253, 0.28)' : 'rgba(56, 189, 248, 0.09)',
          size: isDirectCenter ? Math.max(0.06, 0.22 * zoomScale) : Math.max(0.04, 0.09 * zoomScale),
          zIndex: isDirectCenter ? 20 : 10,
          label: '',
          forceLabel: false,
          type: 'line',
        };
      }
      return HIDDEN_EDGE;
    }

    // Zero-allocation fast path when focus mode has no active hover/selection/sector
    if (
      ctx.edgeMode === 'focus' &&
      !ctx.hoveredNodeId &&
      !ctx.selectedNodeId &&
      !ctx.focusedSector &&
      !ctx.focusedCluster
    ) {
      return HIDDEN_EDGE;
    }

    // Cluster focus
    if (ctx.focusedCluster) {
      const { clusterCat, cleanCluster } = normalizeFocusTargets(null, ctx.focusedCluster);
      const sCat = String(dataAttrs.sourceCategory || '').toLowerCase();
      const tCat = String(dataAttrs.targetCategory || '').toLowerCase();
      const sClusterKey = typeof dataAttrs.sourceClusterKey === 'string' ? dataAttrs.sourceClusterKey.toLowerCase() : undefined;
      const tClusterKey = typeof dataAttrs.targetClusterKey === 'string' ? dataAttrs.targetClusterKey.toLowerCase() : undefined;
      if (
        !matchesClusterFilter(sCat, sClusterKey, clusterCat, cleanCluster) ||
        !matchesClusterFilter(tCat, tClusterKey, clusterCat, cleanCluster)
      ) {
        return HIDDEN_EDGE;
      }
    } else if (ctx.focusedSector) {
      const { sectorCat, cleanSector } = normalizeFocusTargets(ctx.focusedSector, null);
      const sCat = String(dataAttrs.sourceCategory || '').toLowerCase();
      const tCat = String(dataAttrs.targetCategory || '').toLowerCase();
      const sSectorKey = typeof dataAttrs.sourceSectorKey === 'string' ? dataAttrs.sourceSectorKey.toLowerCase() : undefined;
      const tSectorKey = typeof dataAttrs.targetSectorKey === 'string' ? dataAttrs.targetSectorKey.toLowerCase() : undefined;
      if (
        !matchesSectorFilter(sCat, sSectorKey, sectorCat, cleanSector) ||
        !matchesSectorFilter(tCat, tSectorKey, sectorCat, cleanSector)
      ) {
        return HIDDEN_EDGE;
      }
    }

    // Semantic Zoom LOD: hide edges if either endpoint star is collapsed/hidden
    if (!ctx.focusedCluster && !ctx.focusedSector && ctx.lodState) {
      const graph = ctx.sigma?.getGraph();
      const s = dataAttrs.source as string;
      const t = dataAttrs.target as string;
      if (graph && s && t && graph.hasNode(s) && graph.hasNode(t)) {
        const sAttrs = graph.getNodeAttributes(s) as Record<string, unknown>;
        const tAttrs = graph.getNodeAttributes(t) as Record<string, unknown>;
        const sVisible = s === ctx.hoveredNodeId || s === ctx.selectedNodeId || isMemoryNodeVisibleInLod({
          trustScope: (sAttrs.trustScope as TrustScope) || 'curated',
          sectorKey: String(sAttrs.sectorKey || ''),
          clusterKey: String(sAttrs.clusterKey || ''),
          isClusterAlpha: Boolean(sAttrs.isClusterAlpha),
          stellarClass: sAttrs.stellarClass as string | undefined,
          connections: Number(sAttrs.connections) || 0,
        }, ctx.lodState);
        const tVisible = t === ctx.hoveredNodeId || t === ctx.selectedNodeId || isMemoryNodeVisibleInLod({
          trustScope: (tAttrs.trustScope as TrustScope) || 'curated',
          sectorKey: String(tAttrs.sectorKey || ''),
          clusterKey: String(tAttrs.clusterKey || ''),
          isClusterAlpha: Boolean(tAttrs.isClusterAlpha),
          stellarClass: tAttrs.stellarClass as string | undefined,
          connections: Number(tAttrs.connections) || 0,
        }, ctx.lodState);
        if (!sVisible || !tVisible) {
          return HIDDEN_EDGE;
        }
      }
    }

    const result = computeEdgeDisplay(
      dataAttrs as unknown as EdgeData,
      dataAttrs.source as string,
      dataAttrs.target as string,
      {
        cameraRatio: ctx.sigma?.getCamera()?.ratio ?? 1.0,
        mode: ctx.edgeMode || 'auto',
        hoveredNodeId: ctx.hoveredNodeId,
        selectedNodeId: ctx.selectedNodeId,
        focusedCategory: ctx.focusedSector,
      }
    );
    if (result.hidden) {
      return HIDDEN_EDGE;
    }
    return { ...dataAttrs, ...result };
  };

  return { nodeReducer, edgeReducer };
}
