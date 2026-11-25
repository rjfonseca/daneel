/**
 * @file hoverController.ts
 * O(deg) hover interaction manager using sigmaContext.hoveredNeighborIds and
 * Sigma partialGraph refreshes with skipIndexation: true instead of full graph rebuilds.
 */

import type Sigma from 'sigma';
import { resolveProjectCategory } from '../nebulas';
import { resolveActiveSuperEdges } from '../superEdges';
import type { HoverTooltipNode, SigmaContextState } from '../../types/galaxy';

export interface HoverControllerOptions {
  sigma: Sigma;
  context: SigmaContextState;
  onHover: (node: HoverTooltipNode | null, pos: { x: number; y: number } | null) => void;
  getColor: (category: string) => string;
}

export class HoverController {
  private readonly sigma: Sigma;
  private readonly context: SigmaContextState;
  private readonly onHover: HoverControllerOptions['onHover'];
  private readonly getColor: (category: string) => string;

  private activeNodes: string[] = [];
  private activeEdges: string[] = [];
  private activeTetherEdges: string[] = [];

  constructor(options: HoverControllerOptions) {
    this.sigma = options.sigma;
    this.context = options.context;
    this.onHover = options.onHover;
    this.getColor = options.getColor;
  }

  private refreshPartial(nodes: string[], edges: string[], forceFull = false): void {
    if (forceFull) {
      this.sigma.refresh({ schedule: true });
      return;
    }
    if (nodes.length === 0 && edges.length === 0) return;

    try {
      this.sigma.refresh({
        partialGraph: { nodes, edges },
        skipIndexation: true,
        schedule: true,
      });
    } catch {
      this.sigma.refresh({ schedule: true });
    }
  }

  private setContainerCursor(cursor: string): void {
    const getContainer = (this.sigma as unknown as { getContainer?: () => HTMLElement | null }).getContainer;
    const container = typeof getContainer === 'function' ? getContainer.call(this.sigma) : null;
    if (container && container.style) {
      container.style.cursor = cursor;
    }
  }

  private clearActiveHoverState(refreshNow: boolean): void {
    const prevNodes = this.activeNodes;
    const prevEdges = this.activeEdges;
    const prevTethers = this.activeTetherEdges;
    const hadTethers = prevTethers.length > 0;

    this.activeNodes = [];
    this.activeEdges = [];
    this.activeTetherEdges = [];
    this.context.hoveredNodeId = null;
    this.context.hoveredNeighborIds?.clear();

    if (hadTethers) {
      const graph = this.sigma.getGraph();
      for (const edge of prevTethers) {
        if (graph.hasEdge(edge)) {
          graph.setEdgeAttribute(edge, 'hidden', true);
          graph.setEdgeAttribute(edge, 'color', 'rgba(0,0,0,0)');
          graph.setEdgeAttribute(edge, 'size', 0.1);
        }
      }
    }

    if (refreshNow) {
      this.refreshPartial(prevNodes, prevEdges, hadTethers);
    }
  }

  public handleEnterNode(nodeId: string): void {
    const graph = this.sigma.getGraph();
    if (!graph.hasNode(nodeId)) return;

    const prevNodes = this.activeNodes;
    const prevEdges = this.activeEdges;
    const prevTethers = this.activeTetherEdges;
    this.clearActiveHoverState(false);

    if (nodeId.startsWith('__SECTOR_')) {
      this.setContainerCursor('pointer');
      const targetSector = nodeId.replace('__SECTOR_', '').toLowerCase();
      const sectorColor = this.getColor(targetSector);
      const isCollapsedSuperNode = Boolean(this.context.lodState?.collapsedSectorNodes.has(nodeId));

      if (isCollapsedSuperNode) {
        const meta = this.context.superNodeRegistry.get(nodeId);
        if (meta) {
          const attrs = graph.getNodeAttributes(nodeId) as { x: number; y: number };
          const pos = this.sigma.graphToViewport(attrs);
          const bridges = resolveActiveSuperEdges(this.context.superEdgeIndex, this.context.lodState, {
            superNodeRegistry: this.context.superNodeRegistry,
            getColor: this.getColor,
          });
          const bridgeCount = bridges.filter(b => b.sourceNodeId === nodeId || b.targetNodeId === nodeId).length;
          this.context.hoveredNodeId = nodeId;
          this.activeNodes = [nodeId];
          this.onHover({
            id: nodeId,
            title: meta.displayName,
            originalCategory: meta.key,
            color: meta.color,
            stellarBadge: '◈ SECTOR SUPER-NODE',
            isSuperNode: true,
            repoCount: meta.repoCount,
            noteCount: meta.noteCount,
            recentCount: meta.recentCount,
            bridgeCount,
          }, pos);
        }
        this.refreshPartial(
          Array.from(new Set([...prevNodes, ...this.activeNodes])),
          prevEdges,
          prevTethers.length > 0
        );
        return;
      }

      const affectedNodes: string[] = [];
      graph.forEachNode((node, nAttrs: Record<string, unknown>) => {
        if (!node.startsWith('__CAT_') && !node.startsWith('__SECTOR_')) {
          const noteCat = String(nAttrs.originalCategory || nAttrs.category || '').toLowerCase();
          const nodeSector = String(nAttrs.sectorKey || resolveProjectCategory(noteCat).sector).toLowerCase();
          if (nodeSector === targetSector || noteCat === targetSector || noteCat.startsWith(targetSector + '/')) {
            this.context.hoveredNeighborIds?.add(node);
            affectedNodes.push(node);
          }
        }
      });

      const tethers: string[] = [];
      graph.forEachEdge((edge, eAttrs: Record<string, unknown>) => {
        if (eAttrs.isSectorTether && eAttrs.sector === targetSector) {
          graph.setEdgeAttribute(edge, 'hidden', false);
          graph.setEdgeAttribute(edge, 'color', sectorColor);
          graph.setEdgeAttribute(edge, 'size', 1.2);
          tethers.push(edge);
        }
      });

      this.activeNodes = affectedNodes;
      this.activeTetherEdges = tethers;
      this.sigma.refresh({ schedule: true });
      return;
    }

    if (nodeId.startsWith('__CAT_')) {
      this.setContainerCursor('pointer');
      const isCollapsedCluster = Boolean(this.context.lodState?.collapsedClusterNodes.has(nodeId));
      if (isCollapsedCluster) {
        const meta = this.context.superNodeRegistry.get(nodeId);
        if (meta) {
          const attrs = graph.getNodeAttributes(nodeId) as { x: number; y: number };
          const pos = this.sigma.graphToViewport(attrs);
          const bridges = resolveActiveSuperEdges(this.context.superEdgeIndex, this.context.lodState, {
            superNodeRegistry: this.context.superNodeRegistry,
            getColor: this.getColor,
          });
          const bridgeCount = bridges.filter(b => b.sourceNodeId === nodeId || b.targetNodeId === nodeId).length;
          this.context.hoveredNodeId = nodeId;
          this.activeNodes = [nodeId];
          this.onHover({
            id: nodeId,
            title: meta.displayName,
            originalCategory: meta.key,
            color: meta.color,
            stellarBadge: '☁️ CLUSTER SUPER-NODE',
            isSuperNode: true,
            repoCount: 0,
            noteCount: meta.noteCount,
            recentCount: meta.recentCount,
            bridgeCount,
          }, pos);
          this.refreshPartial(
            Array.from(new Set([...prevNodes, ...this.activeNodes])),
            prevEdges,
            prevTethers.length > 0
          );
        }
        return;
      }

      const targetCat = nodeId.replace('__CAT_', '').toLowerCase();
      const affectedNodes: string[] = [];
      graph.forEachNode((node, nAttrs: Record<string, unknown>) => {
        const noteCat = String(nAttrs.originalCategory || nAttrs.category || '').toLowerCase();
        if (!node.startsWith('__CAT_') && !node.startsWith('__SECTOR_') && (noteCat === targetCat || noteCat.startsWith(targetCat + '/'))) {
          this.context.hoveredNeighborIds?.add(node);
          affectedNodes.push(node);
        }
      });
      this.activeNodes = affectedNodes;
      this.refreshPartial(
        Array.from(new Set([...prevNodes, ...affectedNodes])),
        prevEdges,
        prevTethers.length > 0
      );
      return;
    }

    const attrs = graph.getNodeAttributes(nodeId) as Record<string, unknown>;
    const pos = this.sigma.graphToViewport(attrs as { x: number; y: number });
    const realConnections = graph.neighbors(nodeId)
      .filter((neighbor) => !neighbor.startsWith('__CAT_') && !neighbor.startsWith('__SECTOR_'))
      .length;
    this.setContainerCursor('pointer');
    this.onHover({
      id: nodeId,
      title: attrs.label as string | undefined,
      originalCategory: attrs.originalCategory as string | undefined,
      color: attrs.color as string | undefined,
      stellarClass: attrs.stellarClass as string | undefined,
      stellarBadge: attrs.stellarBadge as string | undefined,
      stellarLabel: attrs.stellarLabel as string | undefined,
      stellarDesc: attrs.stellarDesc as string | undefined,
      connections: realConnections || (attrs.connections as number | undefined),
      ring: attrs.ring as string | undefined,
      status: attrs.status as string | undefined,
      icon: attrs.icon as string | undefined,
    }, pos);

    this.context.hoveredNodeId = nodeId;
    const affectedNodes = [nodeId];
    graph.forEachNeighbor(nodeId, (neighbor) => {
      if (!neighbor.startsWith('__CAT_') && !neighbor.startsWith('__SECTOR_')) {
        this.context.hoveredNeighborIds?.add(neighbor);
        affectedNodes.push(neighbor);
      }
    });
    const affectedEdges = graph.edges(nodeId);

    this.activeNodes = affectedNodes;
    this.activeEdges = affectedEdges;
    this.refreshPartial(
      Array.from(new Set([...prevNodes, ...affectedNodes])),
      Array.from(new Set([...prevEdges, ...affectedEdges])),
      prevTethers.length > 0
    );
  }

  public handleLeaveNode(): void {
    this.setContainerCursor('');
    this.onHover(null, null);
    this.clearActiveHoverState(true);
  }

  public clear(): void {
    this.setContainerCursor('');
    this.onHover(null, null);
    this.clearActiveHoverState(true);
  }
}
