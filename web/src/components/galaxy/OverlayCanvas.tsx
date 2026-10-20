import React, { useRef, useEffect, useCallback } from 'react';
import { useSigma } from '@react-sigma/core';
import { renderOrbitalRingsOnCanvas, type OrbitalLayoutResult } from '../../lib/orbitalRings';
import type { TrafficSimulator } from '../../lib/trafficSimulation';
import type { ConstellationResult } from '../../lib/constellationFilter';

interface OverlayCanvasProps {
  showRings: boolean;
  isCosmicMode: boolean;
  focusedSector: string | null;
  orbitalLayout: OrbitalLayoutResult | null;
  trafficSimulator: TrafficSimulator;
  isReady: boolean;
  constellation?: ConstellationResult | null;
}

export const OverlayCanvas: React.FC<OverlayCanvasProps> = ({
  showRings,
  isCosmicMode,
  focusedSector,
  orbitalLayout,
  trafficSimulator,
  isReady,
  constellation,
}) => {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const sigma = useSigma();

  const isNodeVisible = useCallback((nodeId: string): boolean => {
    const graph = sigma?.getGraph();
    if (!graph || !graph.hasNode(nodeId)) return false;
    if (graph.getNodeAttribute(nodeId, 'hidden') === true) return false;
    if (constellation && !constellation.visibleNodes.has(nodeId)) {
      return false;
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
      renderOrbitalRingsOnCanvas(ctx, centerV, scaledRings);
    }

    // 2. Render Cosmic Traffic Ships
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
  }, [sigma, showRings, isCosmicMode, focusedSector, orbitalLayout, trafficSimulator, isNodeVisible]);

  // Redraw when ready, rings, or cosmic mode changes
  useEffect(() => {
    renderCanvas();
    const t = setTimeout(() => renderCanvas(), 100);
    return () => clearTimeout(t);
  }, [isReady, showRings, isCosmicMode, renderCanvas]);

  // Redraw on window resize
  useEffect(() => {
    const handleResize = () => renderCanvas();
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, [renderCanvas]);

  // Redraw on camera move
  useEffect(() => {
    if (!sigma) return;
    const camera = sigma.getCamera();
    const handleCam = () => renderCanvas();
    camera.on('updated', handleCam);
    return () => {
      camera.off('updated', handleCam);
    };
  }, [sigma, renderCanvas]);

  // Cosmic animation loop
  useEffect(() => {
    if (!isCosmicMode) {
      renderCanvas();
      return;
    }

    let animationFrameId: number;
    let lastTime = performance.now();

    const loop = (time: number) => {
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

    animationFrameId = requestAnimationFrame(loop);

    return () => {
      cancelAnimationFrame(animationFrameId);
    };
  }, [isCosmicMode, renderCanvas, sigma, trafficSimulator, isNodeVisible]);

  return (
    <canvas 
      ref={canvasRef} 
      className="absolute inset-0 pointer-events-none z-10 w-full h-full" 
    />
  );
};
