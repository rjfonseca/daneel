import React, { useRef, useEffect, useCallback } from 'react';
import { useSigma } from '@react-sigma/core';
import { renderOrbitalRingsOnCanvas, type OrbitalLayoutResult } from '../../lib/orbitalRings';
import type { TrafficSimulator } from '../../lib/trafficSimulation';
import type { ConstellationResult } from '../../lib/constellationFilter';
import type { EdgeVisibilityMode } from '../../lib/edgeFiltering';
import { sigmaContext } from '../../lib/sigmaContext';
import { getSafeHudBounds } from '../../lib/galaxy/labelLayout';
import { isMemoryNodeVisibleInLod, type SemanticLodState, type SuperNodeMeta, type TrustScope } from '../../lib/semanticZoomLod';
import { resolveActiveSuperEdges, type ActiveSuperEdge, type SuperEdgeIndex } from '../../lib/superEdges';
import { renderAggregatedSuperEdges, renderCollapsedSuperNodes } from '../../lib/superNodeRenderer';

interface OverlayCanvasProps {
  showRings: boolean;
  isCosmicMode: boolean;
  focusedSector: string | null;
  orbitalLayout: OrbitalLayoutResult | null;
  trafficSimulator: TrafficSimulator;
  isReady: boolean;
  constellation?: ConstellationResult | null;
  edgeMode?: EdgeVisibilityMode;
  getColor?: (category: string) => string;
}

export const OverlayCanvas: React.FC<OverlayCanvasProps> = ({
  showRings,
  isCosmicMode,
  focusedSector,
  orbitalLayout,
  trafficSimulator,
  isReady,
  constellation,
  edgeMode = 'focus',
  getColor = () => '#38bdf8',
}) => {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const pendingRenderFrameRef = useRef<number | null>(null);
  const superEdgeCacheRef = useRef<{
    index: SuperEdgeIndex | null;
    lod: SemanticLodState | null;
    bridges: ActiveSuperEdge[];
  }>({ index: null, lod: null, bridges: [] });
  const sigma = useSigma();

  const isNodeVisible = useCallback((nodeId: string): boolean => {
    const graph = sigma?.getGraph();
    if (!graph || !graph.hasNode(nodeId)) return false;
    if (graph.getNodeAttribute(nodeId, 'hidden') === true) return false;
    if (constellation && !constellation.visibleNodes.has(nodeId)) {
      return false;
    }
    if (!constellation && !sigmaContext.focusedSector && !sigmaContext.focusedCluster && sigmaContext.lodState) {
      const attrs = graph.getNodeAttributes(nodeId) as Record<string, unknown>;
      return isMemoryNodeVisibleInLod(
        {
          trustScope: (attrs.trustScope as TrustScope) || 'curated',
          sectorKey: String(attrs.sectorKey || ''),
          clusterKey: String(attrs.clusterKey || ''),
          isClusterAlpha: Boolean(attrs.isClusterAlpha),
          stellarClass: attrs.stellarClass as string | undefined,
          connections: Number(attrs.connections) || 0,
        },
        sigmaContext.lodState
      );
    }
    return true;
  }, [constellation, sigma]);

  const renderCanvas = useCallback(() => {
    const canvas = canvasRef.current;
    if (!canvas || !sigma) return;

    const width = window.innerWidth;
    const height = window.innerHeight;
    const dpr = window.devicePixelRatio || 1;

    if (canvas.width !== width * dpr || canvas.height !== height * dpr) {
      canvas.width = width * dpr;
      canvas.height = height * dpr;
    }

    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    ctx.resetTransform();
    ctx.scale(dpr, dpr);
    ctx.clearRect(0, 0, width, height);

    const occupiedCalloutBoxes = sigmaContext.frameCalloutBoxes || [];
    const safeBounds = getSafeHudBounds(width, height, 114, height - 92);

    // 1. Render Orbital Rings (ARMS / Seldon Perimeter)
    if (showRings && !focusedSector && orbitalLayout) {
      const centerV = sigma.graphToViewport(orbitalLayout.center);
      const scaledRings = orbitalLayout.rings.map(ring => {
        const edgeV = sigma.graphToViewport({
          x: orbitalLayout.center.x + ring.radius,
          y: orbitalLayout.center.y,
        });
        const radiusViewport = Math.hypot(edgeV.x - centerV.x, edgeV.y - centerV.y);
        return {
          radiusViewport,
          color: ring.color,
          dashPattern: ring.dashPattern,
          shortLabel: ring.shortLabel,
        };
      });
      renderOrbitalRingsOnCanvas(ctx, centerV, scaledRings, {
        safeBounds,
        occupiedCalloutBoxes,
      });
    }

    // 2. Render Aggregated Super-Edges & Collapsed Super-Nodes (Semantic Zoom LOD)
    if (!constellation && !focusedSector && !sigmaContext.focusedCluster && sigmaContext.lodState) {
      const graph = sigma.getGraph();
      const getViewportPos = (nodeId: string) => {
        if (!graph || !graph.hasNode(nodeId)) return null;
        const attrs = graph.getNodeAttributes(nodeId) as { x: number; y: number };
        return sigma.graphToViewport(attrs);
      };

      const cache = superEdgeCacheRef.current;
      if (cache.index !== sigmaContext.superEdgeIndex || cache.lod !== sigmaContext.lodState) {
        cache.index = sigmaContext.superEdgeIndex;
        cache.lod = sigmaContext.lodState;
        cache.bridges = resolveActiveSuperEdges(
          sigmaContext.superEdgeIndex,
          sigmaContext.lodState,
          {
            superNodeRegistry: sigmaContext.superNodeRegistry,
            getColor,
          }
        );
      }
      renderAggregatedSuperEdges(ctx, cache.bridges, getViewportPos, {
        hoveredNodeId: sigmaContext.hoveredNodeId,
        edgeMode,
      });

      const collapsedNodes: SuperNodeMeta[] = [];
      sigmaContext.lodState.collapsedSectorNodes.forEach((id) => {
        const meta = sigmaContext.superNodeRegistry.get(id);
        if (meta) collapsedNodes.push(meta);
      });
      sigmaContext.lodState.collapsedClusterNodes.forEach((id) => {
        const meta = sigmaContext.superNodeRegistry.get(id);
        if (meta) collapsedNodes.push(meta);
      });

      renderCollapsedSuperNodes(ctx, collapsedNodes, getViewportPos, {
        hoveredNodeId: sigmaContext.hoveredNodeId,
        viewportWidth: width,
        viewportHeight: height,
        cameraRatio: sigma.getCamera().ratio,
      });
    }

    // 3. Render Cosmic Traffic Ships
    if (isCosmicMode && trafficSimulator) {
      const graph = sigma.getGraph();
      trafficSimulator.render(
        ctx,
        (nodeId) => {
          if (!graph || !graph.hasNode(nodeId)) return null;
          const attrs = graph.getNodeAttributes(nodeId) as { x: number; y: number };
          return sigma.graphToViewport(attrs);
        },
        isNodeVisible
      );
    }
  }, [sigma, showRings, isCosmicMode, focusedSector, orbitalLayout, trafficSimulator, isNodeVisible, constellation, edgeMode, getColor]);

  // Coalesce non-cosmic redraws to at most once per animation frame
  const scheduleCoalescedRender = useCallback(() => {
    if (isCosmicMode) return; // Cosmic rAF loop renders every frame already
    if (pendingRenderFrameRef.current !== null) return;
    pendingRenderFrameRef.current = requestAnimationFrame(() => {
      pendingRenderFrameRef.current = null;
      renderCanvas();
    });
  }, [isCosmicMode, renderCanvas]);

  // Redraw when ready, rings, or cosmic mode changes
  useEffect(() => {
    renderCanvas();
    const t = setTimeout(() => renderCanvas(), 100);
    return () => {
      clearTimeout(t);
      if (pendingRenderFrameRef.current !== null) {
        cancelAnimationFrame(pendingRenderFrameRef.current);
        pendingRenderFrameRef.current = null;
      }
    };
  }, [isReady, showRings, isCosmicMode, renderCanvas]);

  // Redraw in lockstep after Sigma completes any scheduled render when cosmic loop is off
  useEffect(() => {
    if (!sigma) return;
    const handleAfterRender = () => scheduleCoalescedRender();
    sigma.on('afterRender', handleAfterRender);
    return () => {
      sigma.off('afterRender', handleAfterRender);
    };
  }, [sigma, scheduleCoalescedRender]);

  // Redraw on window resize
  useEffect(() => {
    const handleResize = () => {
      if (isCosmicMode) renderCanvas();
      else scheduleCoalescedRender();
    };
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, [isCosmicMode, renderCanvas, scheduleCoalescedRender]);

  // Redraw on camera move when cosmic loop is off
  useEffect(() => {
    if (!sigma) return;
    const camera = sigma.getCamera();
    const handleCam = () => scheduleCoalescedRender();
    camera.on('updated', handleCam);
    return () => {
      camera.off('updated', handleCam);
    };
  }, [sigma, scheduleCoalescedRender]);

  // Cosmic animation loop (pauses automatically when tab is hidden)
  useEffect(() => {
    if (!isCosmicMode) {
      renderCanvas();
      return;
    }

    let animationFrameId: number | null = null;
    let lastTime = performance.now();

    const loop = (time: number) => {
      if (document.hidden) {
        animationFrameId = null;
        return;
      }
      const dt = Math.min(0.1, (time - lastTime) / 1000);
      lastTime = time;
      const graph = sigma.getGraph();
      trafficSimulator.update(
        dt,
        (nodeId) => {
          if (!graph || !graph.hasNode(nodeId)) return null;
          const attrs = graph.getNodeAttributes(nodeId) as { x: number; y: number };
          return sigma.graphToViewport(attrs);
        },
        window.innerWidth,
        window.innerHeight,
        isNodeVisible
      );
      renderCanvas();

      animationFrameId = requestAnimationFrame(loop);
    };

    const startLoop = () => {
      if (animationFrameId === null && !document.hidden) {
        lastTime = performance.now();
        animationFrameId = requestAnimationFrame(loop);
      }
    };

    const handleVisibility = () => {
      if (document.hidden) {
        if (animationFrameId !== null) {
          cancelAnimationFrame(animationFrameId);
          animationFrameId = null;
        }
      } else {
        startLoop();
      }
    };

    document.addEventListener('visibilitychange', handleVisibility);
    startLoop();

    return () => {
      document.removeEventListener('visibilitychange', handleVisibility);
      if (animationFrameId !== null) {
        cancelAnimationFrame(animationFrameId);
      }
    };
  }, [isCosmicMode, renderCanvas, sigma, trafficSimulator, isNodeVisible]);

  return (
    <canvas
      ref={canvasRef}
      className="absolute inset-0 pointer-events-none z-10 w-full h-full"
    />
  );
};
