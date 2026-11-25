import React, { useEffect, useCallback, useRef, useState } from 'react';
import Graph from 'graphology';
import { useLoadGraph, useRegisterEvents, useSigma } from '@react-sigma/core';
import { useWorkerLayoutForceAtlas2 } from '@react-sigma/layout-forceatlas2';
import { saveLayout, loadLayout } from '../../lib/layoutCache';
import { calculateNebulaAllocations, getCategoryDisplayName, resolveCanonicalSectorKey, resolveProjectCategory, isRootNoteCategory } from '../../lib/nebulas';
import { type CalloutBoundingBox, isPointInCalloutBox } from '../../lib/canvasMath';
import { classifyStar } from '../../lib/stellarClassification';
import { calculateOrbitalLayout, type OrbitalLayoutResult } from '../../lib/orbitalRings';
import { TrafficSimulator } from '../../lib/trafficSimulation';
import { calculateClusterFraming, calculateNodeFocusCamera, calculateDrawerRecenterX } from '../../lib/cameraMath';
import { sigmaContext } from '../../lib/sigmaContext';
import {
  classifyTrustScope,
  computeSemanticLodState,
  isLodStateEqual,
  type SuperNodeMeta,
  type TrustScope,
} from '../../lib/semanticZoomLod';
import { buildSuperEdgeIndex } from '../../lib/superEdges';
import { SmoothCameraController, MIN_CAMERA_RATIO, MAX_CAMERA_RATIO } from '../../lib/galaxy/smoothCamera';
import { evaluateFocusZoomExit } from '../../lib/galaxy/focusZoomExit';
import { applySectorVisibilityFilter } from '../../lib/galaxy/sectorVisibility';
import { HoverController } from '../../lib/galaxy/hoverController';
import { OverlayCanvas } from './OverlayCanvas';
import type { GalaxyControllerProps } from '../../types/galaxy';

const CANONICAL_SECTOR_COLORS: Record<string, string> = {
  // Knowledge Vault thematic sectors
  arquitetura: '#818cf8', // Electric Indigo
  ia: '#a78bfa',          // Nebula Lavender
  pessoas: '#fb923c',     // Plasma Orange
  carreira: '#fcd34d',    // Solar Gold
  cultura: '#e879f9',     // Supernova Magenta
  sistemas: '#38bdf8',    // Neon Sky Blue
  seguranca: '#f87171',   // Coral Red
  daily: '#34d399',       // Emerald Green

  // Engineering, Platforms & Infrastructure
  saas: '#2dd4bf',        // Hyper Teal
  apis: '#60a5fa',        // Cobalt Blue
  stores: '#f472b6',      // Cosmic Pink
  platform: '#a78bfa',    // Lavender
  infra: '#38bdf8',       // Neon Blue
  core: '#c084fc',        // Stellar Violet
  data: '#fb923c',        // Plasma Orange
  tools: '#4ade80',       // Aurora Mint
};

const COSMIC_PALETTE = [
  '#38bdf8', // Neon Sky Blue
  '#34d399', // Emerald Green
  '#f472b6', // Cosmic Pink
  '#fcd34d', // Solar Gold
  '#2dd4bf', // Hyper Teal
  '#818cf8', // Electric Indigo
  '#f87171', // Coral Red
  '#c084fc', // Stellar Violet
  '#fb923c', // Plasma Orange
  '#a78bfa', // Nebula Lavender
  '#4ade80', // Aurora Mint
  '#e879f9', // Supernova Magenta
];

export const getColor = (category: string): string => {
  if (!category) return '#818cf8';
  const proj = resolveProjectCategory(category);
  const sectorKey = (proj.sector || category)
    .toLowerCase()
    .replace(/^(notes|sources)\//, '')
    .trim();
  const key = sectorKey.split('/')[0] || sectorKey;

  if (CANONICAL_SECTOR_COLORS[key]) {
    return CANONICAL_SECTOR_COLORS[key];
  }

  let hash = 5381;
  for (let i = 0; i < key.length; i++) {
    hash = ((hash << 5) + hash) + key.charCodeAt(i);
  }
  return COSMIC_PALETTE[Math.abs(hash) % COSMIC_PALETTE.length];
};

export const GalaxyController: React.FC<GalaxyControllerProps> = ({ 
  safeNodes, 
  safeLinks, 
  catCounts, 
  onFocusNode, 
  onReady, 
  targetNodeId, 
  focusedSector, 
  setFocusedSector, 
  focusedCluster,
  setFocusedCluster,
  onHover, 
  maxNebulas, 
  onMountController, 
  edgeMode,
  isCosmicMode,
  showRings,
  constellation,
  isReady,
  showSectorLabels = true,
  showNebulaLabels = true,
  drawerWidth = 440,
  reducedMotion = false,
  scopeMode,
  onLodTelemetryChange,
  onAutoUnfocus,
}) => {
  const loadGraph = useLoadGraph();
  const sigma = useSigma();
  const lastLodTierRef = useRef<string>('overview');
  const lastFrustumLodTimeRef = useRef<number>(0);
  const trailingLodTimerRef = useRef<number | null>(null);
  const orbitalLayoutRef = useRef<OrbitalLayoutResult | null>(null);
  const trafficSimulatorRef = useRef<TrafficSimulator>(new TrafficSimulator());
  const sectorMetasRef = useRef<SuperNodeMeta[]>([]);
  const clusterMetasRef = useRef<SuperNodeMeta[]>([]);
  const curatedStarCountRef = useRef<number>(0);
  const totalStarCountRef = useRef<number>(0);
  const onLodTelemetryChangeRef = useRef(onLodTelemetryChange);
  onLodTelemetryChangeRef.current = onLodTelemetryChange;
  const onAutoUnfocusRef = useRef(onAutoUnfocus);
  onAutoUnfocusRef.current = onAutoUnfocus;
  const smoothCameraRef = useRef<SmoothCameraController | null>(null);
  const hoverControllerRef = useRef<HoverController | null>(null);
  const skipNextTargetZoomRef = useRef<string | null>(null);
  const lastLayoutRevisionRef = useRef<number>(0);
  const userHasInteractedRef = useRef<boolean>(false);
  const focusAnchorRef = useRef<{
    sectorAnchorRatio: number | null;
    clusterAnchorRatio: number | null;
    constellationAnchorRatio: number | null;
    isProgrammaticFocusFlight: boolean;
    previousRatio: number;
  }>({
    sectorAnchorRatio: null,
    clusterAnchorRatio: null,
    constellationAnchorRatio: null,
    isProgrammaticFocusFlight: false,
    previousRatio: 1.0,
  });
  const drawerWidthRef = useRef(drawerWidth);
  drawerWidthRef.current = drawerWidth;
  const lastDrawerRef = useRef({ width: drawerWidth, target: targetNodeId });
  const [layoutRevision, setLayoutRevision] = useState(0);
  const [systemReducedMotion, setSystemReducedMotion] = useState(() =>
    typeof window !== 'undefined' && Boolean(window.matchMedia?.('(prefers-reduced-motion: reduce)').matches));
  const motionReduced = reducedMotion || systemReducedMotion;
  const motionReducedRef = useRef(motionReduced);
  motionReducedRef.current = motionReduced;
  // Sigma divides elapsed time by duration on its first synchronous frame.
  const cameraDuration = useCallback((normalDuration: number) => motionReducedRef.current ? 1 : normalDuration, []);

  useEffect(() => {
    if (!window.matchMedia) return;
    const media = window.matchMedia('(prefers-reduced-motion: reduce)');
    const update = () => setSystemReducedMotion(media.matches);
    update();
    media.addEventListener?.('change', update);
    return () => media.removeEventListener?.('change', update);
  }, []);

  const computeCategoryFraming = useCallback((catId: string) => {
    const graph = sigma.getGraph();
    if (!graph) return null;
    const isSectorNode = catId.startsWith('__SECTOR_');
    const rawSectorCat = catId.replace('__SECTOR_', '').replace('__CAT_', '').toLowerCase();
    const cleanSector = rawSectorCat.replace(/^(notes|sources)\//, '');
    const normFn = (sigma as unknown as { normalizationFunction: (attrs: unknown) => { x: number; y: number } }).normalizationFunction;
    if (!normFn) return null;

    const points: Array<{ x: number; y: number }> = [];

    graph.forEachNode((node, attrs: Record<string, unknown>) => {
      if (node.startsWith('__CAT_') || node.startsWith('__SECTOR_')) return;
      const noteCat = String(attrs.originalCategory || attrs.category || '').toLowerCase();
      const cleanNoteCat = noteCat.replace(/^(notes|sources)\//, '');
      const nodeSector = String(attrs.sectorKey || resolveCanonicalSectorKey(noteCat)).toLowerCase();
      const cleanNodeSector = nodeSector.replace(/^(notes|sources)\//, '');

      const isMatch =
        noteCat === rawSectorCat ||
        noteCat.startsWith(rawSectorCat + '/') ||
        cleanNoteCat === cleanSector ||
        cleanNoteCat.startsWith(cleanSector + '/') ||
        nodeSector === rawSectorCat ||
        cleanNodeSector === cleanSector ||
        (isSectorNode && (nodeSector === rawSectorCat || cleanNodeSector === cleanSector));

      if (isMatch) {
        const f = normFn(attrs);
        if (f && typeof f.x === 'number' && typeof f.y === 'number') {
          points.push(f);
        }
      }
    });

    if (points.length > 0) {
      const dims = sigma.getDimensions();
      return calculateClusterFraming(points, dims, {
        targetCoverageFraction: 0.78,
        minRatio: points.length <= 2 ? 0.16 : 0.005,
        minSpan: points.length <= 2 ? 0.08 : 0.001,
      });
    }
    if (graph.hasNode(catId)) {
      const attrs = graph.getNodeAttributes(catId);
      const framed = normFn(attrs);
      return { x: framed.x, y: framed.y, ratio: 0.22 };
    }
    return null;
  }, [sigma]);

  // Proportional dynamic cluster framing to fill screen (~78%)
  const zoomToCategoryCluster = useCallback((catId: string) => {
    const framing = computeCategoryFraming(catId);
    if (!framing) return;

    const isClusterNode = catId.startsWith('__CAT_');
    if (isClusterNode) {
      focusAnchorRef.current.clusterAnchorRatio = framing.ratio;
      const rawCat = catId.replace('__CAT_', '').toLowerCase();
      const proj = resolveProjectCategory(rawCat);
      if (proj.sector && proj.sector !== 'root') {
        const parentSectorId = `__SECTOR_${proj.sector.toLowerCase()}`;
        const parentFraming = computeCategoryFraming(parentSectorId);
        focusAnchorRef.current.sectorAnchorRatio = parentFraming?.ratio ?? Math.min(0.45, Math.max(framing.ratio * 1.4, 0.28));
      } else {
        focusAnchorRef.current.sectorAnchorRatio = null;
      }
    } else {
      focusAnchorRef.current.sectorAnchorRatio = framing.ratio;
      focusAnchorRef.current.clusterAnchorRatio = null;
    }
    focusAnchorRef.current.isProgrammaticFocusFlight = true;

    const clearFlight = () => {
      focusAnchorRef.current.isProgrammaticFocusFlight = false;
    };
    const animator = smoothCameraRef.current;
    if (animator) {
      animator.animateTo(
        { x: framing.x, y: framing.y, ratio: framing.ratio, angle: 0 },
        { duration: cameraDuration(850), easing: 'cubicInOut', onComplete: clearFlight, onCancel: clearFlight }
      );
    } else {
      sigma.getCamera().animate(
        { x: framing.x, y: framing.y, ratio: framing.ratio, angle: 0 },
        { duration: cameraDuration(850) },
        clearFlight
      );
    }
  }, [sigma, computeCategoryFraming, cameraDuration]);

  const zoomToNodes = useCallback((nodeIds: string[]) => {
    const graph = sigma.getGraph();
    if (!graph) return;
    const normFn = (sigma as unknown as { normalizationFunction: (attrs: unknown) => { x: number; y: number } }).normalizationFunction;
    if (!normFn) return;

    const points: Array<{ x: number; y: number }> = [];
    nodeIds.forEach(id => {
      if (graph.hasNode(id)) {
        const attrs = graph.getNodeAttributes(id);
        const f = normFn(attrs);
        if (f && typeof f.x === 'number' && typeof f.y === 'number') {
          points.push(f);
        }
      }
    });

    if (points.length > 0) {
      const dims = sigma.getDimensions();
      const framing = calculateClusterFraming(points, dims, { targetCoverageFraction: 0.72 });
      const animator = smoothCameraRef.current;
      if (animator) {
        animator.animateTo(
          { x: framing.x, y: framing.y, ratio: framing.ratio, angle: 0 },
          { duration: cameraDuration(850), easing: 'cubicInOut' }
        );
      } else {
        sigma.getCamera().animate(
          { x: framing.x, y: framing.y, ratio: framing.ratio, angle: 0 },
          { duration: cameraDuration(850) }
        );
      }
    } else {
      zoomToCategoryCluster('daily');
    }
  }, [sigma, zoomToCategoryCluster, cameraDuration]);
  
  const { start, stop } = useWorkerLayoutForceAtlas2({
    settings: {
      barnesHutOptimize: true,
      scalingRatio: 25,
      strongGravityMode: false,
      gravity: 0.05,
      edgeWeightInfluence: 0.5,
      slowDown: 1.5,
      linLogMode: true,
    },
    outputReducer: (key, attrs) => {
      const orbPos = orbitalLayoutRef.current?.nodePositions.get(key);
      if (orbPos) {
        return { ...attrs, x: orbPos.x, y: orbPos.y };
      }
      return attrs;
    },
  });
  
  const registerEvents = useRegisterEvents();

  const resetCamera = useCallback(() => {
    hoverControllerRef.current?.clear();
    focusAnchorRef.current.sectorAnchorRatio = null;
    focusAnchorRef.current.clusterAnchorRatio = null;
    focusAnchorRef.current.constellationAnchorRatio = null;
    focusAnchorRef.current.isProgrammaticFocusFlight = false;
    sigmaContext.focusedSector = null;
    sigmaContext.focusedCluster = null;
    setFocusedSector(null);
    if (setFocusedCluster) setFocusedCluster(null);
    const animator = smoothCameraRef.current;
    if (animator) {
      animator.animateTo({ x: 0.5, y: 0.5, ratio: 1, angle: 0 }, { duration: cameraDuration(900), easing: 'cubicInOut' });
    } else {
      sigma.getCamera().animatedReset({ duration: cameraDuration(900) });
    }
  }, [setFocusedSector, setFocusedCluster, sigma, cameraDuration]);

  useEffect(() => {
    if (onMountController) {
      onMountController({
        resetCamera,
        zoomToCategoryCluster,
        zoomToNodes,
      });
    }
  }, [onMountController, resetCamera, zoomToCategoryCluster, zoomToNodes]);

  useEffect(() => {
    sigmaContext.sigma = sigma;
    (window as unknown as { __sigmaInstance: unknown }).__sigmaInstance = sigma;
    (window as unknown as { __sigmaContext: unknown }).__sigmaContext = sigmaContext;
    const handleBeforeRender = () => {
      sigmaContext.frameLabelsCount = 0;
      sigmaContext.frameRootLabelsCount = 0;
      sigmaContext.frameConstellationNeighborLabelsCount = 0;
      sigmaContext.frameCalloutBoxes = [];
      sigmaContext.renderedNebulaCenters = [];
    };
    sigma.on('beforeRender', handleBeforeRender);
    return () => {
      sigma.off('beforeRender', handleBeforeRender);
      if (sigmaContext.sigma === sigma) {
        sigmaContext.sigma = null;
        (window as unknown as { __sigmaInstance: unknown }).__sigmaInstance = null;
      }
    };
  }, [sigma]);

  const recomputeLod = useCallback((forceRefresh = false) => {
    if (!sigma) return;
    const dims = sigma.getDimensions();
    if (!dims || dims.width <= 0 || dims.height <= 0) return;
    const camera = sigma.getCamera();
    const nextLod = computeSemanticLodState({
      scopeMode: sigmaContext.scopeMode,
      cameraRatio: camera.ratio,
      viewportDims: dims,
      previousState: sigmaContext.lodState,
      sectors: sectorMetasRef.current,
      clusters: clusterMetasRef.current,
      curatedStarCount: curatedStarCountRef.current,
      totalStarCount: totalStarCountRef.current,
      projectToViewport: (x, y) => sigma.graphToViewport({ x, y }),
    });

    const changed = !isLodStateEqual(sigmaContext.lodState, nextLod);
    const hasFocus = Boolean(sigmaContext.focusedSector || sigmaContext.focusedCluster);
    if (changed || forceRefresh || hasFocus) {
      sigmaContext.lodState = nextLod;
      let visibleStars = nextLod.visibleStarCount;
      if (hasFocus) {
        const graph = sigma.getGraph();
        let focusedVisibleCount = 0;
        graph.forEachNode((id, attrs) => {
          if (!id.startsWith('__CAT_') && !id.startsWith('__SECTOR_') && (!attrs.ring || attrs.ring === 'memory') && !attrs.hidden) {
            focusedVisibleCount++;
          }
        });
        visibleStars = focusedVisibleCount;
      }
      onLodTelemetryChangeRef.current?.({
        tier: nextLod.tier,
        visibleStars,
        totalStars: nextLod.totalStarCount,
        groupedStars: Math.max(0, nextLod.totalStarCount - visibleStars),
        collapsedGroups: hasFocus ? 0 : nextLod.collapsedGroupCount,
      });
      if (changed || forceRefresh) {
        sigma.refresh({ schedule: true });
      }
    }
  }, [sigma]);

  useEffect(() => {
    if (!sigma) return;
    const smoothCamera = new SmoothCameraController({
      sigma,
      getReducedMotion: () => motionReducedRef.current,
      minRatio: MIN_CAMERA_RATIO,
      maxRatio: MAX_CAMERA_RATIO,
      onUserWheelInterrupt: () => {
        userHasInteractedRef.current = true;
        focusAnchorRef.current.isProgrammaticFocusFlight = false;
      },
    });
    smoothCameraRef.current = smoothCamera;

    const mouseCaptor = sigma.getMouseCaptor();
    const handleMouseDown = () => {
      userHasInteractedRef.current = true;
      smoothCamera.cancelWheelGlide();
    };
    mouseCaptor.on('wheel', smoothCamera.handleWheel);
    mouseCaptor.on('mousedown', handleMouseDown);

    return () => {
      mouseCaptor.off('wheel', smoothCamera.handleWheel);
      mouseCaptor.off('mousedown', handleMouseDown);
      smoothCamera.destroy();
      if (smoothCameraRef.current === smoothCamera) {
        smoothCameraRef.current = null;
      }
    };
  }, [sigma]);

  useEffect(() => {
    if (!sigma) return;
    const camera = sigma.getCamera();
    focusAnchorRef.current.previousRatio = camera.ratio;
    let rafId: number | null = null;
    const handleCameraUpdate = () => {
      if (rafId !== null) return;
      rafId = requestAnimationFrame(() => {
        rafId = null;
        const ratio = camera.ratio;

        // 1. Hierarchical auto-unfocus on zoom-out (Constellation -> Cluster -> Sector -> Full Galaxy)
        const exitAction = evaluateFocusZoomExit({
          focusedSector: sigmaContext.focusedSector,
          focusedCluster: sigmaContext.focusedCluster ?? null,
          hasActiveConstellation: Boolean(sigmaContext.constellation),
          isProgrammaticFocusFlight: focusAnchorRef.current.isProgrammaticFocusFlight,
          currentRatio: ratio,
          previousRatio: focusAnchorRef.current.previousRatio,
          sectorAnchorRatio: focusAnchorRef.current.sectorAnchorRatio,
          clusterAnchorRatio: focusAnchorRef.current.clusterAnchorRatio,
          constellationAnchorRatio: focusAnchorRef.current.constellationAnchorRatio,
        });
        focusAnchorRef.current.previousRatio = ratio;

        if (exitAction.type === 'exit-constellation') {
          smoothCameraRef.current?.cancelWheelGlide();
          hoverControllerRef.current?.clear();
          sigmaContext.constellation = null;
          focusAnchorRef.current.constellationAnchorRatio = null;
          onAutoUnfocusRef.current?.({ clearCluster: false, clearSector: false, clearConstellation: true });
          recomputeLod(true);
          sigma.refresh({ schedule: true });
        } else if (exitAction.type === 'exit-cluster') {
          smoothCameraRef.current?.cancelWheelGlide();
          hoverControllerRef.current?.clear();
          sigmaContext.focusedCluster = null;
          focusAnchorRef.current.clusterAnchorRatio = null;
          onAutoUnfocusRef.current?.({ clearCluster: true, clearSector: false });
          applySectorVisibilityFilter(sigma.getGraph(), sigmaContext.focusedSector, null);
          recomputeLod(true);
        } else if (exitAction.type === 'exit-all') {
          hoverControllerRef.current?.clear();
          sigmaContext.focusedCluster = null;
          sigmaContext.focusedSector = null;
          focusAnchorRef.current.clusterAnchorRatio = null;
          focusAnchorRef.current.sectorAnchorRatio = null;
          onAutoUnfocusRef.current?.({ clearCluster: true, clearSector: true });
          applySectorVisibilityFilter(sigma.getGraph(), null, null);
          recomputeLod(true);
        }

        // 2. Semantic Zoom LOD with throttled frustum checks during smooth camera motion
        let tier = 'overview';
        if (ratio < 0.35) tier = 'deep';
        else if (ratio < 0.65) tier = 'mid';

        const edgeTierChanged = tier !== lastLodTierRef.current;
        if (edgeTierChanged) {
          lastLodTierRef.current = tier;
        }

        const now = performance.now();
        const isMoving = Boolean(smoothCameraRef.current?.isAnimating());
        if (edgeTierChanged || !isMoving || now - lastFrustumLodTimeRef.current >= 120) {
          lastFrustumLodTimeRef.current = now;
          if (trailingLodTimerRef.current !== null) {
            window.clearTimeout(trailingLodTimerRef.current);
            trailingLodTimerRef.current = null;
          }
          recomputeLod(edgeTierChanged);
        } else if (trailingLodTimerRef.current === null) {
          trailingLodTimerRef.current = window.setTimeout(() => {
            trailingLodTimerRef.current = null;
            lastFrustumLodTimeRef.current = performance.now();
            recomputeLod(false);
          }, 140);
        }
      });
    };
    camera.on('updated', handleCameraUpdate);
    return () => {
      camera.off('updated', handleCameraUpdate);
      if (rafId !== null) cancelAnimationFrame(rafId);
      if (trailingLodTimerRef.current !== null) {
        window.clearTimeout(trailingLodTimerRef.current);
        trailingLodTimerRef.current = null;
      }
    };
  }, [sigma, recomputeLod]);

  useEffect(() => {
    sigmaContext.scopeMode = scopeMode;
    recomputeLod(true);
  }, [scopeMode, recomputeLod]);

  useEffect(() => {
    sigmaContext.edgeMode = edgeMode;
    if (sigma) {
      sigma.refresh({ schedule: true });
    }
  }, [edgeMode, sigma]);

  useEffect(() => {
    sigmaContext.focusedSector = focusedSector;
    sigmaContext.focusedCluster = focusedCluster || null;
    sigmaContext.maxNebulas = maxNebulas;
  }, [focusedSector, focusedCluster, maxNebulas]);

  useEffect(() => {
    sigmaContext.constellation = constellation;
    if (sigma) {
      sigma.refresh({ schedule: true });
    }
  }, [constellation, sigma]);

  useEffect(() => {
    sigmaContext.showRings = showRings;
    if (sigma) {
      sigma.refresh({ schedule: true });
    }
  }, [showRings, sigma]);

  useEffect(() => {
    sigmaContext.isCosmicMode = isCosmicMode;
  }, [isCosmicMode]);

  useEffect(() => {
    sigmaContext.showSectorLabels = showSectorLabels;
    if (sigma) {
      sigma.refresh({ schedule: true });
    }
  }, [showSectorLabels, sigma]);

  useEffect(() => {
    sigmaContext.showNebulaLabels = showNebulaLabels;
    if (sigma) {
      sigma.refresh({ schedule: true });
    }
  }, [showNebulaLabels, sigma]);

  // Load Graph & Calculate Orbital Positions
  useEffect(() => {
    if (!safeNodes.length) return;

    const graph = new Graph();
    const cachedLayout = loadLayout() || {};

    const allocations = calculateNebulaAllocations(Object.keys(catCounts), catCounts, maxNebulas);

    // Group project categories by sector for hierarchical clustering
    const sectorMap = new Map<string, { displayName: string; categories: string[] }>();
    Object.keys(catCounts).forEach(cat => {
      const proj = resolveProjectCategory(cat);
      if (proj.isProject && proj.sector) {
        const sKey = proj.sector.toLowerCase();
        const existing = sectorMap.get(sKey) || { displayName: proj.sectorDisplayName, categories: [] };
        existing.categories.push(cat);
        sectorMap.set(sKey, existing);
      }
    });

    // Determine initial positions for macro centers (sectors and standalone/root categories)
    // so ForceAtlas2 starts with an organic, well-distributed galactic disk rather than an imploding central blob
    const macroClusterKeys: string[] = [];
    const initialMacroPos = new Map<string, { x: number; y: number }>();

    sectorMap.forEach((_, sKey) => {
      macroClusterKeys.push(`__SECTOR_${sKey}`);
    });

    const sectorCategories = new Set<string>();
    sectorMap.forEach(sec => sec.categories.forEach(c => sectorCategories.add(c)));

    Object.keys(catCounts).forEach(cat => {
      if (!sectorCategories.has(cat)) {
        macroClusterKeys.push(`__CAT_${cat}`);
      }
    });

    const numMacro = Math.max(1, macroClusterKeys.length);
    const baseDiskRadius = 2200;
    macroClusterKeys.forEach((key, idx) => {
      if (cachedLayout[key]) {
        initialMacroPos.set(key, cachedLayout[key]);
      } else {
        const angle = (idx / numMacro) * Math.PI * 2;
        const radiusJitter = ((idx % 3) - 1) * 350;
        const r = baseDiskRadius + radiusJitter;
        initialMacroPos.set(key, {
          x: Math.cos(angle) * r,
          y: Math.sin(angle) * r,
        });
      }
    });

    // Category hub initial positions
    const initialCatPos = new Map<string, { x: number; y: number }>();
    sectorMap.forEach((secGroup, sKey) => {
      const secCenter = initialMacroPos.get(`__SECTOR_${sKey}`) || { x: 0, y: 0 };
      const totalInSec = Math.max(1, secGroup.categories.length);
      secGroup.categories.forEach((cat, cIdx) => {
        const catKey = `__CAT_${cat}`;
        if (cachedLayout[catKey]) {
          initialCatPos.set(cat, cachedLayout[catKey]);
        } else {
          const cAngle = (cIdx / totalInSec) * Math.PI * 2;
          const cDist = 220 + ((cIdx % 3) * 100);
          initialCatPos.set(cat, {
            x: secCenter.x + Math.cos(cAngle) * cDist,
            y: secCenter.y + Math.sin(cAngle) * cDist,
          });
        }
      });
    });

    Object.keys(catCounts).forEach(cat => {
      if (!sectorCategories.has(cat)) {
        const catKey = `__CAT_${cat}`;
        if (cachedLayout[catKey]) {
          initialCatPos.set(cat, cachedLayout[catKey]);
        } else {
          const macroPos = initialMacroPos.get(catKey) || { x: 0, y: 0 };
          initialCatPos.set(cat, macroPos);
        }
      }
    });

    const initialNodesWithCoords = safeNodes.map(n => {
      const isMemory = !n.ring || n.ring === 'memory';
      if (!isMemory) return n;
      if (cachedLayout[n.id]) {
        return { ...n, x: cachedLayout[n.id].x, y: cachedLayout[n.id].y };
      }
      const cat = n.category || 'root';
      const hubPos = initialCatPos.get(cat) || { x: 0, y: 0 };
      return {
        ...n,
        x: hubPos.x,
        y: hubPos.y,
      };
    });
    const orbitalLayout = calculateOrbitalLayout(initialNodesWithCoords);
    orbitalLayoutRef.current = orbitalLayout;
    sigmaContext.galaxyCenter = orbitalLayout.center;

    // Precompute trust scopes & 7-day activity counters per sector and cluster
    const recentCutoffSec = Date.now() / 1000 - 7 * 86400;
    const sectorRecentCounts = new Map<string, number>();
    const clusterRecentCounts = new Map<string, number>();
    const sectorTrustScopes = new Map<string, TrustScope>();
    const clusterTrustScopes = new Map<string, TrustScope>();
    let curatedStarCount = 0;
    let totalStarCount = 0;

    safeNodes.forEach((n) => {
      const scope = classifyTrustScope(n);
      if (scope === 'arms') return;
      totalStarCount += 1;
      if (scope === 'curated') curatedStarCount += 1;

      const cat = n.category || 'root';
      const proj = resolveProjectCategory(n.id, cat, n.domain || n.sector, n.group, n.repository);
      const cKey = (proj.category || cat).toLowerCase();
      const sKey = resolveCanonicalSectorKey(cKey, proj.sector);

      if (scope === 'sources') {
        clusterTrustScopes.set(cKey, 'sources');
        if (proj.isProject && proj.sector) {
          sectorTrustScopes.set(sKey, 'sources');
        }
      } else {
        if (!clusterTrustScopes.has(cKey)) clusterTrustScopes.set(cKey, 'curated');
        if (proj.isProject && proj.sector && !sectorTrustScopes.has(sKey)) {
          sectorTrustScopes.set(sKey, 'curated');
        }
      }

      const rawMod = Number(n.modTime || 0);
      const modSec = rawMod > 1e11 ? rawMod / 1000 : rawMod;
      if (modSec >= recentCutoffSec) {
        clusterRecentCounts.set(cKey, (clusterRecentCounts.get(cKey) || 0) + 1);
        if (proj.isProject && proj.sector) {
          sectorRecentCounts.set(sKey, (sectorRecentCounts.get(sKey) || 0) + 1);
        }
      }
    });

    curatedStarCountRef.current = curatedStarCount;
    totalStarCountRef.current = totalStarCount;

    const superNodeRegistry = new Map<string, SuperNodeMeta>();
    const sectorMetas: SuperNodeMeta[] = [];
    const clusterMetas: SuperNodeMeta[] = [];

    // Sector Anchors
    sectorMap.forEach((secGroup, sKey) => {
      const sectorNodeId = `__SECTOR_${sKey}`;
      if (!graph.hasNode(sectorNodeId)) {
        const neonColor = getColor(sKey);
        const secInfo = allocations.sectorMap.get(sKey);
        const displayName = secInfo ? secInfo.displayName : secGroup.displayName;
        const isTop = secInfo ? secInfo.isOverviewVisible : false;
        const topType = isTop ? 'TOP' : 'OTHER';
        const rank = secInfo ? secInfo.priorityRank : 999;
        const repoCount = secInfo ? secInfo.categories.length : secGroup.categories.length;
        const noteCount = secInfo ? secInfo.totalNotes : 0;
        const defaultPos = initialMacroPos.get(sectorNodeId) || { x: 0, y: 0 };
        const posX = cachedLayout[sectorNodeId] ? cachedLayout[sectorNodeId].x : defaultPos.x;
        const posY = cachedLayout[sectorNodeId] ? cachedLayout[sectorNodeId].y : defaultPos.y;
        const hasSourceCat = secGroup.categories.some(c => c.toLowerCase().startsWith('sources/'));
        const trustScope: TrustScope = sectorTrustScopes.get(sKey) || (hasSourceCat ? 'sources' : 'curated');
        const recentCount = sectorRecentCounts.get(sKey) || 0;

        graph.addNode(sectorNodeId, {
          x: posX,
          y: posY,
          size: 24,
          key: sectorNodeId,
          label: `${displayName}|${neonColor}|SECTOR|SECTOR|${topType}|${rank}|${repoCount}|${noteCount}`,
          color: "rgba(0,0,0,0)",
          neonColor,
          forceLabel: isTop,
          alwaysRenderLabel: isTop,
          labelDensityWeight: isTop ? 20000 : 0,
          sector: sKey,
          sectorKey: sKey,
          sectorDisplayName: displayName,
          trustScope,
          isSector: true,
          repoCount,
          noteCount,
          recentCount,
        });

        const meta: SuperNodeMeta = {
          id: sectorNodeId,
          kind: 'sector',
          key: sKey,
          sectorKey: sKey,
          displayName,
          color: neonColor,
          trustScope,
          noteCount,
          repoCount,
          recentCount,
          x: posX,
          y: posY,
        };
        superNodeRegistry.set(sectorNodeId, meta);
        sectorMetas.push(meta);
      }
    });

    // Nebula category hubs (compute recursive counts & skip parent source index dirs already covered by sector orbs)
    const catKeys = Object.keys(catCounts);
    const recursiveCatCounts: Record<string, number> = {};
    const hasChildCatSet = new Set<string>();
    for (const cat of catKeys) {
      let sum = 0;
      for (const other of catKeys) {
        if (other === cat || other.startsWith(`${cat}/`)) {
          sum += catCounts[other] || 0;
          if (other !== cat) {
            hasChildCatSet.add(cat);
          }
        }
      }
      recursiveCatCounts[cat] = sum;
    }

    catKeys.forEach(cat => {
      if (!graph.hasNode(`__CAT_${cat}`)) {
        const neonColor = getColor(cat);
        const info = allocations.categoryMap.get(cat);
        const name = info ? info.displayName : getCategoryDisplayName(cat);
        const isTop = info ? info.isOverviewVisible : false;
        const isRoot = info ? info.isRoot : isRootNoteCategory(cat);
        const dynSize = isRoot ? 26 : (info ? info.dynSize : 1);
        const catType = isRoot ? 'ROOT' : 'SUB';
        const topType = isTop ? 'TOP' : 'OTHER';
        const rank = info ? info.priorityRank : 999;
        const defaultPos = initialCatPos.get(cat) || { x: 0, y: 0 };
        const noteCount = recursiveCatCounts[cat] ?? (catCounts[cat] || 0);
        const catKey = cat.toLowerCase();
        const posX = cachedLayout[`__CAT_${cat}`] ? cachedLayout[`__CAT_${cat}`].x : defaultPos.x;
        const posY = cachedLayout[`__CAT_${cat}`] ? cachedLayout[`__CAT_${cat}`].y : defaultPos.y;
        const proj = resolveProjectCategory(cat);
        const sectorKey = (info?.sector || proj.sector || cat).toLowerCase();
        const trustScope: TrustScope = clusterTrustScopes.get(catKey) || (catKey.startsWith('sources/') ? 'sources' : 'curated');
        const recentCount = clusterRecentCounts.get(catKey) || 0;
        
        graph.addNode(`__CAT_${cat}`, {
          x: posX,
          y: posY,
          size: dynSize,
          key: `__CAT_${cat}`,
          label: `${name}|${neonColor}|CATEGORY|${catType}|${topType}|${rank}|0|${noteCount}`, 
          color: "rgba(0,0,0,0)",
          neonColor,
          forceLabel: isRoot,
          alwaysRenderLabel: isRoot,
          labelDensityWeight: isRoot ? 35000 : (isTop ? 10000 : 0),
          isRoot,
          noteCount,
          recentCount,
          trustScope,
          sectorKey,
          clusterKey: catKey,
        });

        const meta: SuperNodeMeta = {
          id: `__CAT_${cat}`,
          kind: 'cluster',
          key: catKey,
          sectorKey,
          displayName: name,
          color: neonColor,
          trustScope,
          noteCount,
          repoCount: 0,
          recentCount,
          x: posX,
          y: posY,
        };
        superNodeRegistry.set(`__CAT_${cat}`, meta);
        const isParentSourceFolderCoveredBySector =
          trustScope === 'sources' &&
          hasChildCatSet.has(cat) &&
          sectorMap.has(resolveCanonicalSectorKey(cat, sectorKey));
        if (!isParentSourceFolderCoveredBySector) {
          clusterMetas.push(meta);
        }
      }
    });

    sigmaContext.superNodeRegistry = superNodeRegistry;
    sectorMetasRef.current = sectorMetas;
    clusterMetasRef.current = clusterMetas;
    sigmaContext.superEdgeIndex = buildSuperEdgeIndex(safeNodes, safeLinks);

    // Connect category hubs to their sector anchors with calibrated gravitational edges
    sectorMap.forEach((secInfo, sKey) => {
      const sectorNodeId = `__SECTOR_${sKey}`;
      secInfo.categories.forEach(cat => {
        const catNodeId = `__CAT_${cat}`;
        if (graph.hasNode(catNodeId) && graph.hasNode(sectorNodeId) && !graph.hasEdge(catNodeId, sectorNodeId)) {
          graph.addEdge(catNodeId, sectorNodeId, {
            weight: 3.0,
            hidden: true,
            size: 0.1,
            color: "rgba(0,0,0,0)",
            originalHidden: true,
            isSectorTether: true,
            sector: sKey,
          });
        }
      });
    });

    const nodeCatMap = new Map<string, string>();
    const nodeDegreeMap = new Map<string, number>();
    const nodeSectorMap = new Map<string, string>();
    const nodeClusterMap = new Map<string, string>();
    safeNodes.forEach((n, nIdx) => {
      nodeCatMap.set(n.id, n.category || 'root');
      nodeDegreeMap.set(n.id, n.connections || 0);

      const cat = n.category || 'root';
      const proj = resolveProjectCategory(n.id, cat, n.domain || n.sector, n.group, n.repository);
      const clusterKey = (proj.category || cat).toLowerCase();
      const sectorKey = resolveCanonicalSectorKey(clusterKey, proj.sector);
      nodeClusterMap.set(n.id, clusterKey);
      nodeSectorMap.set(n.id, sectorKey);

      if (!graph.hasNode(n.id)) {
        const stellar = classifyStar(n);
        const hubPos = initialCatPos.get(cat) || { x: 0, y: 0 };
        const nAngle = (nIdx * 2.39996) % (Math.PI * 2);
        const nDist = 30 + ((nIdx * 17) % 220);
        let posX = hubPos.x + Math.cos(nAngle) * nDist;
        let posY = hubPos.y + Math.sin(nAngle) * nDist;

        if (orbitalLayout.nodePositions.has(n.id)) {
          const orbPos = orbitalLayout.nodePositions.get(n.id)!;
          posX = orbPos.x;
          posY = orbPos.y;
        } else if (cachedLayout[n.id]) {
          posX = cachedLayout[n.id].x;
          posY = cachedLayout[n.id].y;
        }

        const trustScope = classifyTrustScope(n);
        const isClusterAlpha = stellar.spectralClass === 'α-CORE' || n.noteType === 'repository';

        graph.addNode(n.id, {
          x: posX,
          y: posY,
          size: stellar.size,
          label: n.title || n.id,
          color: stellar.color,
          originalCategory: n.category,
          stellarClass: stellar.classification,
          stellarBadge: stellar.badge,
          stellarLabel: stellar.label,
          stellarDesc: stellar.description,
          spectralClass: stellar.spectralClass,
          connections: n.connections || 0,
          modTime: n.modTime || 0,
          ring: n.ring,
          status: n.status,
          icon: n.icon,
          trustScope,
          sectorKey,
          clusterKey,
          isClusterAlpha,
        });
      }
      
      const isMemory = !n.ring || n.ring === 'memory';
      if (isMemory) {
        const catNode = `__CAT_${n.category || 'root'}`;
        if (graph.hasNode(n.id) && graph.hasNode(catNode) && !graph.hasEdge(n.id, catNode)) {
          graph.addEdge(n.id, catNode, {
            weight: 1.5,
            hidden: true,
            size: 0.1,
            color: "rgba(0,0,0,0)",
            originalHidden: true,
          });
        }
      }
    });

    safeLinks.forEach(l => {
      if (graph.hasNode(l.source) && graph.hasNode(l.target) && !graph.hasEdge(l.source, l.target)) {
        const srcCat = nodeCatMap.get(l.source) || 'root';
        const tgtCat = nodeCatMap.get(l.target) || 'root';
        const isCrossCluster = srcCat !== tgtCat;
        graph.addEdge(l.source, l.target, { 
          source: l.source,
          target: l.target,
          weight: isCrossCluster ? 0.2 : 0.5, 
          size: 0.25, 
          color: "rgba(148, 163, 184, 0.05)", 
          originalHidden: false,
          isCrossCluster,
          sourceCategory: srcCat,
          targetCategory: tgtCat,
          sourceSectorKey: nodeSectorMap.get(l.source) || 'root',
          targetSectorKey: nodeSectorMap.get(l.target) || 'root',
          sourceClusterKey: nodeClusterMap.get(l.source) || srcCat.toLowerCase(),
          targetClusterKey: nodeClusterMap.get(l.target) || tgtCat.toLowerCase(),
          sourceDegree: nodeDegreeMap.get(l.source) || 0,
          targetDegree: nodeDegreeMap.get(l.target) || 0,
        });
      }
    });

    loadGraph(graph);
    trafficSimulatorRef.current.init(graph, 10);
    
    let stopped = false;
    try { start(); } catch (_e) { /* noop if layout already running */ }
    
    const waitTime = Object.keys(cachedLayout).length > 0 ? 1500 : 8500;
    
    const timer = setTimeout(() => {
      if (stopped) return;
      stopped = true;
      try { stop(); } catch (_e) { /* noop if layout already stopped */ }
      
      // Stabilize ARMS stations to their exact polar rings around outer memory envelope
      const liveGraph = sigma.getGraph();
      const nodesWithCoords = safeNodes.map((n) => ({
        ...n,
        x: liveGraph.hasNode(n.id) ? (liveGraph.getNodeAttribute(n.id, 'x') as number) : n.x,
        y: liveGraph.hasNode(n.id) ? (liveGraph.getNodeAttribute(n.id, 'y') as number) : n.y,
      }));
      const updatedLayout = calculateOrbitalLayout(nodesWithCoords);
      orbitalLayoutRef.current = updatedLayout;
      sigmaContext.galaxyCenter = updatedLayout.center;

      updatedLayout.nodePositions.forEach((pos, id) => {
        if (liveGraph.hasNode(id)) {
          liveGraph.setNodeAttribute(id, 'x', pos.x);
          liveGraph.setNodeAttribute(id, 'y', pos.y);
        }
      });

      // Align category hub centroids & bounding boxes to member stars
      const catSums = new Map<string, {
        sumX: number;
        sumY: number;
        count: number;
        minX: number;
        maxX: number;
        minY: number;
        maxY: number;
      }>();
      safeNodes.forEach((n) => {
        const isMemory = !n.ring || n.ring === 'memory';
        if (isMemory && liveGraph.hasNode(n.id)) {
          const cat = n.category || 'root';
          const nx = liveGraph.getNodeAttribute(n.id, 'x') as number;
          const ny = liveGraph.getNodeAttribute(n.id, 'y') as number;
          const entry = catSums.get(cat) || {
            sumX: 0,
            sumY: 0,
            count: 0,
            minX: Infinity,
            maxX: -Infinity,
            minY: Infinity,
            maxY: -Infinity,
          };
          entry.sumX += nx;
          entry.sumY += ny;
          entry.count++;
          entry.minX = Math.min(entry.minX, nx);
          entry.maxX = Math.max(entry.maxX, nx);
          entry.minY = Math.min(entry.minY, ny);
          entry.maxY = Math.max(entry.maxY, ny);
          catSums.set(cat, entry);
        }
      });
      catSums.forEach((entry, cat) => {
        const catNodeId = `__CAT_${cat}`;
        if (liveGraph.hasNode(catNodeId) && entry.count > 0) {
          const cx = entry.sumX / entry.count;
          const cy = entry.sumY / entry.count;
          liveGraph.setNodeAttribute(catNodeId, 'x', cx);
          liveGraph.setNodeAttribute(catNodeId, 'y', cy);
          const meta = sigmaContext.superNodeRegistry.get(catNodeId);
          if (meta) {
            meta.x = cx;
            meta.y = cy;
            meta.minX = entry.minX;
            meta.maxX = entry.maxX;
            meta.minY = entry.minY;
            meta.maxY = entry.maxY;
          }
        }
      });

      // Align sector anchor centroids & aggregate AABB bounds across member category hubs
      sectorMap.forEach((secInfo, sKey) => {
        const sectorNodeId = `__SECTOR_${sKey}`;
        if (liveGraph.hasNode(sectorNodeId)) {
          let sumX = 0;
          let sumY = 0;
          let totalWeight = 0;
          let minX = Infinity;
          let maxX = -Infinity;
          let minY = Infinity;
          let maxY = -Infinity;
          secInfo.categories.forEach(cat => {
            const catNodeId = `__CAT_${cat}`;
            if (liveGraph.hasNode(catNodeId)) {
              const weight = catCounts[cat] || 1;
              const cx = liveGraph.getNodeAttribute(catNodeId, 'x') as number;
              const cy = liveGraph.getNodeAttribute(catNodeId, 'y') as number;
              sumX += cx * weight;
              sumY += cy * weight;
              totalWeight += weight;
              const cBounds = catSums.get(cat);
              if (cBounds && cBounds.count > 0) {
                minX = Math.min(minX, cBounds.minX);
                maxX = Math.max(maxX, cBounds.maxX);
                minY = Math.min(minY, cBounds.minY);
                maxY = Math.max(maxY, cBounds.maxY);
              } else {
                minX = Math.min(minX, cx);
                maxX = Math.max(maxX, cx);
                minY = Math.min(minY, cy);
                maxY = Math.max(maxY, cy);
              }
            }
          });
          if (totalWeight > 0) {
            const sx = sumX / totalWeight;
            const sy = sumY / totalWeight;
            liveGraph.setNodeAttribute(sectorNodeId, 'x', sx);
            liveGraph.setNodeAttribute(sectorNodeId, 'y', sy);
            const meta = sigmaContext.superNodeRegistry.get(sectorNodeId);
            if (meta) {
              meta.x = sx;
              meta.y = sy;
              if (Number.isFinite(minX)) {
                meta.minX = minX;
                meta.maxX = maxX;
                meta.minY = minY;
                meta.maxY = maxY;
              }
            }
          }
        }
      });

      const currentPositions: Record<string, { x: number; y: number }> = {};
      liveGraph.forEachNode((n, attrs) => {
        currentPositions[n] = { x: attrs.x, y: attrs.y };
      });
      saveLayout(currentPositions);
      // Sigma recomputes graph bounds and normalization during a full refresh.
      // Update them before the selected node is reframed against this layout.
      sigma.refresh();
      recomputeLod(true);
      // Node coordinates changed in place after ForceAtlas2. Reframe the current
      // selection against the settled layout rather than its initial position.
      setLayoutRevision(revision => revision + 1);
      
      setTimeout(() => {
        if (!sigmaContext.selectedNodeId && !sigmaContext.focusedSector && !sigmaContext.focusedCluster && !userHasInteractedRef.current) {
          const animator = smoothCameraRef.current;
          if (animator) {
            animator.animateTo({ x: 0.5, y: 0.5, ratio: 1, angle: 0 }, { duration: cameraDuration(1500), easing: 'cubicInOut' });
          } else {
            sigma.getCamera().animatedReset({ duration: cameraDuration(1500) });
          }
        }
        
        setTimeout(() => {
          onReady();
          recomputeLod(true);
          sigma.refresh({ schedule: true });
        }, 1500); 
      }, 300);
      
    }, waitTime);

    return () => {
      stopped = true;
      clearTimeout(timer);
      try { stop(); } catch (_e) { /* noop on unmount */ }
    };
  }, [safeNodes, safeLinks, catCounts, loadGraph, start, stop, sigma, onReady, maxNebulas, cameraDuration, recomputeLod]);

  // Dynamically update category node attributes when maxNebulas changes
  useEffect(() => {
    const graph = sigma.getGraph();
    if (!graph || graph.order === 0) return;
    
    const catKeys = Object.keys(catCounts);
    const allocations = calculateNebulaAllocations(catKeys, catCounts, maxNebulas);
    allocations.categoryMap.forEach((info, cat) => {
      const catNodeId = `__CAT_${cat}`;
      if (graph.hasNode(catNodeId)) {
        const neonColor = getColor(cat);
        const isTop = info.isOverviewVisible;
        const isRoot = info.isRoot;
        const catType = isRoot ? 'ROOT' : 'SUB';
        const topType = isTop ? 'TOP' : 'OTHER';
        const noteCount = catKeys.reduce(
          (sum, other) => (other === cat || other.startsWith(`${cat}/`) ? sum + (catCounts[other] || 0) : sum),
          0
        );
        
        graph.setNodeAttribute(catNodeId, 'size', isRoot ? 26 : info.dynSize);
        graph.setNodeAttribute(
          catNodeId, 
          'label', 
          `${info.displayName}|${neonColor}|CATEGORY|${catType}|${topType}|${info.priorityRank}|0|${noteCount}`
        );
        graph.setNodeAttribute(catNodeId, 'forceLabel', isRoot);
        graph.setNodeAttribute(catNodeId, 'alwaysRenderLabel', isRoot);
        graph.setNodeAttribute(catNodeId, 'labelDensityWeight', isRoot ? 35000 : (isTop ? 10000 : 0));
        graph.setNodeAttribute(catNodeId, 'isRoot', isRoot);
        graph.setNodeAttribute(catNodeId, 'noteCount', noteCount);
      }
    });

    allocations.sectorMap.forEach((secInfo, sKey) => {
      const sectorNodeId = `__SECTOR_${sKey}`;
      if (graph.hasNode(sectorNodeId)) {
        const neonColor = getColor(sKey);
        const isTop = secInfo.isOverviewVisible;
        const topType = isTop ? 'TOP' : 'OTHER';

        const repoCount = secInfo.categories.length;
        const noteCount = secInfo.totalNotes;

        graph.setNodeAttribute(
          sectorNodeId,
          'label',
          `${secInfo.displayName}|${neonColor}|SECTOR|SECTOR|${topType}|${secInfo.priorityRank}|${repoCount}|${noteCount}`
        );
        graph.setNodeAttribute(sectorNodeId, 'forceLabel', isTop);
        graph.setNodeAttribute(sectorNodeId, 'alwaysRenderLabel', isTop);
        graph.setNodeAttribute(sectorNodeId, 'labelDensityWeight', isTop ? 20000 : 0);
        graph.setNodeAttribute(sectorNodeId, 'repoCount', repoCount);
        graph.setNodeAttribute(sectorNodeId, 'noteCount', noteCount);
      }
    });
    sigma.refresh();
  }, [maxNebulas, catCounts, sigma]);

  // Target Node Camera Zoom & Centering (Constellation-aware + Viewport drawer compensation)
  useEffect(() => {
    const layoutChanged = lastLayoutRevisionRef.current !== layoutRevision;
    lastLayoutRevisionRef.current = layoutRevision;

    if (targetNodeId && !layoutChanged && skipNextTargetZoomRef.current === targetNodeId) {
      skipNextTargetZoomRef.current = null;
      return;
    }

    if (targetNodeId && sigma.getGraph().hasNode(targetNodeId)) {
      const graph = sigma.getGraph();
      const attrs = graph.getNodeAttributes(targetNodeId);
      const normFn = (sigma as unknown as { normalizationFunction: (a: unknown) => { x: number; y: number } }).normalizationFunction;
      if (!normFn) return;

      if (targetNodeId.startsWith('__CAT_') || targetNodeId.startsWith('__SECTOR_')) {
        zoomToCategoryCluster(targetNodeId);
        return;
      }

      const centerFramed = normFn(attrs);
      if (!centerFramed) return;

      const isSatellite = Boolean(
        attrs.ring === 'skills' || 
        attrs.ring === 'routines' || 
        attrs.ring === 'harvesters' ||
        targetNodeId.startsWith('harvester:') ||
        targetNodeId.startsWith('routine:') ||
        targetNodeId.startsWith('skill:') ||
        String(attrs.originalCategory || attrs.category || '').startsWith('arms')
      );

      const neighborFramedPoints: Array<{ x: number; y: number }> = [];
      if (!isSatellite && constellation?.visibleNodes) {
        constellation.visibleNodes.forEach(id => {
          if (id !== targetNodeId && graph.hasNode(id)) {
            const hop = constellation.hopDistances?.get(id) ?? 1;
            if (hop === 1) {
              const nAttrs = graph.getNodeAttributes(id);
              const f = normFn(nAttrs);
              if (f && typeof f.x === 'number' && typeof f.y === 'number') {
                neighborFramedPoints.push(f);
              }
            }
          }
        });
      }

      const dims = sigma.getDimensions();
      const camTarget = calculateNodeFocusCamera(centerFramed, isSatellite, neighborFramedPoints, dims, {
        drawerWidth: drawerWidthRef.current,
        viewportToFramed: (pt, opt) => sigma.viewportToFramedGraph(pt, opt),
      });

      focusAnchorRef.current.constellationAnchorRatio = camTarget.ratio;
      focusAnchorRef.current.isProgrammaticFocusFlight = true;
      const clearFocusFlight = () => {
        focusAnchorRef.current.isProgrammaticFocusFlight = false;
      };

      const animator = smoothCameraRef.current;
      if (animator) {
        animator.animateTo(
          { x: camTarget.x, y: camTarget.y, ratio: camTarget.ratio, angle: 0 },
          { duration: cameraDuration(850), easing: 'cubicInOut', onComplete: clearFocusFlight, onCancel: clearFocusFlight }
        );
      } else {
        sigma.getCamera().animate({
          x: camTarget.x,
          y: camTarget.y,
          ratio: camTarget.ratio,
          angle: 0,
        }, { duration: cameraDuration(850) }, clearFocusFlight);
      }
    }
  }, [targetNodeId, constellation, safeNodes, layoutRevision, sigma, zoomToCategoryCluster, cameraDuration]);

  // A drawer resize shifts the visible center, but preserves the user's current zoom.
  useEffect(() => {
    const previous = lastDrawerRef.current;
    lastDrawerRef.current = { width: drawerWidth, target: targetNodeId };
    if (previous.width === drawerWidth || previous.target !== targetNodeId || !targetNodeId) return;
    const graph = sigma.getGraph();
    if (!graph?.hasNode(targetNodeId) || targetNodeId.startsWith('__CAT_') || targetNodeId.startsWith('__SECTOR_')) return;
    const normFn = (sigma as unknown as { normalizationFunction?: (attrs: unknown) => { x: number; y: number } }).normalizationFunction;
    const target = normFn?.(graph.getNodeAttributes(targetNodeId));
    if (!target) return;
    const camera = sigma.getCamera();
    const x = calculateDrawerRecenterX(target, camera.ratio, sigma.getDimensions(), drawerWidth,
      (pt, options) => sigma.viewportToFramedGraph(pt, options));
    camera.setState({ ...camera.getState(), x });
  }, [drawerWidth, targetNodeId, sigma]);

  // Events
  useEffect(() => {
    const hoverCtrl = new HoverController({
      sigma,
      context: sigmaContext,
      onHover,
      getColor,
    });
    hoverControllerRef.current = hoverCtrl;

    const handleSectorActivate = (sectorId: string) => {
      hoverControllerRef.current?.clear();
      const isCurrent = sectorId.toLowerCase() === (focusedSector || '').toLowerCase() && !focusedCluster;
      if (isCurrent) {
        resetCamera();
        return;
      }
      skipNextTargetZoomRef.current = sectorId;
      zoomToCategoryCluster(sectorId);
      if (setFocusedCluster) setFocusedCluster(null);
      setFocusedSector(sectorId);
    };

    const handleClusterActivate = (catId: string) => {
      hoverControllerRef.current?.clear();
      const isCurrent = catId.toLowerCase() === (focusedCluster || '').toLowerCase();
      const rawCat = catId.replace('__CAT_', '').toLowerCase();
      const proj = resolveProjectCategory(rawCat);
      const hasParentSector = Boolean(proj.sector && proj.sector !== 'root');
      const parentSectorId = hasParentSector ? `__SECTOR_${proj.sector.toLowerCase()}` : null;

      if (isCurrent) {
        if (parentSectorId) {
          skipNextTargetZoomRef.current = parentSectorId;
          zoomToCategoryCluster(parentSectorId);
          if (setFocusedCluster) setFocusedCluster(null);
          setFocusedSector(parentSectorId);
        } else {
          resetCamera();
        }
        return;
      }

      skipNextTargetZoomRef.current = catId;
      zoomToCategoryCluster(catId);
      if (parentSectorId) {
        setFocusedSector(parentSectorId);
      }
      if (setFocusedCluster) setFocusedCluster(catId);
    };

    registerEvents({
      enterNode: (e) => hoverCtrl.handleEnterNode(e.node),
      leaveNode: () => hoverCtrl.handleLeaveNode(),
      clickStage: (e) => {
        const mouseEvent = e.event as unknown as { x?: number; clientX?: number; y?: number; clientY?: number };
        const mouseX = mouseEvent.x ?? mouseEvent.clientX ?? 0;
        const mouseY = mouseEvent.y ?? mouseEvent.clientY ?? 0;
        const boxes = sigmaContext.frameCalloutBoxes || [];
        let bestBox: CalloutBoundingBox | null = null;
        let bestDist = Infinity;
        for (const box of boxes) {
          if (isPointInCalloutBox(mouseX, mouseY, box, 12)) {
            const centerX = (box.minX + box.maxX) / 2;
            const centerY = (box.minY + box.maxY) / 2;
            const dist = Math.hypot(mouseX - centerX, mouseY - centerY);
            if (dist < bestDist) {
              bestDist = dist;
              bestBox = box;
            }
          }
        }
        if (bestBox) {
          userHasInteractedRef.current = true;
          hoverControllerRef.current?.clear();
          const box = bestBox;
          if (box.isStation && box.nodeId) {
            onFocusNode(box.nodeId);
            return;
          }
          if (box.isSector || box.nodeId?.startsWith('__SECTOR_')) {
            const sectorId = box.nodeId || (box.category ? ('__SECTOR_' + box.category.toLowerCase()) : null);
            if (sectorId) handleSectorActivate(sectorId);
            return;
          }
          if (box.isCategory) {
            const catId = box.nodeId || (box.category ? ('__CAT_' + box.category.toLowerCase()) : null);
            if (catId) handleClusterActivate(catId);
            return;
          }
        }
      },
      clickNode: (e) => {
        userHasInteractedRef.current = true;
        hoverControllerRef.current?.clear();
        const nodeId = e.node;
        if (nodeId.startsWith('__SECTOR_')) {
          handleSectorActivate(nodeId);
        } else if (nodeId.startsWith('__CAT_')) {
          handleClusterActivate(nodeId);
        } else {
          onFocusNode(nodeId);
        }
      }
    });
  }, [registerEvents, sigma, onHover, onFocusNode, focusedSector, setFocusedSector, focusedCluster, setFocusedCluster, zoomToCategoryCluster, resetCamera]);

  // Canvas Callouts Hover Pointer Affordance
  useEffect(() => {
    const container = sigma.getContainer();
    if (!container) return;

    const handleMouseMove = (e: MouseEvent) => {
      const rect = container.getBoundingClientRect();
      const mouseX = e.clientX - rect.left;
      const mouseY = e.clientY - rect.top;
      const boxes = sigmaContext.frameCalloutBoxes || [];
      const isOver = boxes.some(box => isPointInCalloutBox(mouseX, mouseY, box, 12));
      if (isOver) {
        container.style.cursor = 'pointer';
      } else if (container.style.cursor === 'pointer') {
        container.style.cursor = '';
      }
    };

    container.addEventListener('mousemove', handleMouseMove);
    return () => {
      container.removeEventListener('mousemove', handleMouseMove);
      if (container.style.cursor === 'pointer') {
        container.style.cursor = '';
      }
    };
  }, [sigma]);

  // Sector & Cluster Visibility Filter (batched O(N+E) update + immediate LOD sync)
  useEffect(() => {
    const graph = sigma.getGraph();
    if (!graph || graph.order === 0) return;

    applySectorVisibilityFilter(graph, focusedSector, focusedCluster ?? null);
    if (!focusedSector && !focusedCluster) {
      recomputeLod(true);
    } else {
      sigma.refresh({ schedule: true });
    }
  }, [focusedSector, focusedCluster, sigma, recomputeLod]);

  return (
    <OverlayCanvas
      showRings={showRings}
      isCosmicMode={isCosmicMode && !motionReduced}
      focusedSector={focusedSector}
      orbitalLayout={orbitalLayoutRef.current}
      trafficSimulator={trafficSimulatorRef.current}
      isReady={isReady}
      constellation={constellation}
      edgeMode={edgeMode}
      getColor={getColor}
    />
  );
};
