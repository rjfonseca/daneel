import React, { useEffect, useCallback, useRef, useState } from 'react';
import Graph from 'graphology';
import { useLoadGraph, useRegisterEvents, useSigma } from '@react-sigma/core';
import { useWorkerLayoutForceAtlas2 } from '@react-sigma/layout-forceatlas2';
import { saveLayout, loadLayout } from '../../lib/layoutCache';
import { calculateNebulaAllocations, getCategoryDisplayName, resolveProjectCategory, isRootNoteCategory } from '../../lib/nebulas';
import { type CalloutBoundingBox, isPointInCalloutBox } from '../../lib/canvasMath';
import { classifyStar } from '../../lib/stellarClassification';
import { calculateOrbitalLayout, type OrbitalLayoutResult } from '../../lib/orbitalRings';
import { TrafficSimulator } from '../../lib/trafficSimulation';
import { calculateClusterFraming, calculateNodeFocusCamera, calculateDrawerRecenterX } from '../../lib/cameraMath';
import { sigmaContext } from '../../lib/sigmaContext';
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
}) => {
  const loadGraph = useLoadGraph();
  const sigma = useSigma();
  const highlightedNodesRef = useRef<Array<{ id: string; size: number }>>([]);
  const lastLodTierRef = useRef<string>('overview');
  const orbitalLayoutRef = useRef<OrbitalLayoutResult | null>(null);
  const trafficSimulatorRef = useRef<TrafficSimulator>(new TrafficSimulator());
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

  // Proportional dynamic cluster framing to fill screen (~78%)
  const zoomToCategoryCluster = useCallback((catId: string) => {
    const graph = sigma.getGraph();
    if (!graph) return;
    const isSectorNode = catId.startsWith('__SECTOR_');
    const rawSectorCat = catId.replace('__SECTOR_', '').replace('__CAT_', '').toLowerCase();
    const cleanSector = rawSectorCat.replace(/^(notes|sources)\//, '');
    // Sigma normalization function converts graph coordinates to normalized framed [0, 1] space
    const normFn = (sigma as unknown as { normalizationFunction: (attrs: unknown) => { x: number; y: number } }).normalizationFunction;
    if (!normFn) return;

    const points: Array<{ x: number; y: number }> = [];

    graph.forEachNode((node, attrs: Record<string, unknown>) => {
      if (node.startsWith('__CAT_') || node.startsWith('__SECTOR_')) return;
      const noteCat = String(attrs.originalCategory || attrs.category || '').toLowerCase();
      const cleanNoteCat = noteCat.replace(/^(notes|sources)\//, '');
      const proj = resolveProjectCategory(noteCat);
      const nodeSector = proj.sector.toLowerCase();
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
      const framing = calculateClusterFraming(points, dims, {
        targetCoverageFraction: 0.78,
        minRatio: points.length <= 2 ? 0.16 : 0.005,
        minSpan: points.length <= 2 ? 0.08 : 0.001,
      });
      sigma.getCamera().animate(
        { x: framing.x, y: framing.y, ratio: framing.ratio, angle: 0 },
        { duration: cameraDuration(850) }
      );
    } else if (graph.hasNode(catId)) {
      const attrs = graph.getNodeAttributes(catId);
      const framed = normFn(attrs);
      sigma.getCamera().animate(
        { x: framed.x, y: framed.y, ratio: 0.22, angle: 0 },
        { duration: cameraDuration(850) }
      );
    }
  }, [sigma, cameraDuration]);

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
      sigma.getCamera().animate(
        { x: framing.x, y: framing.y, ratio: framing.ratio, angle: 0 },
        { duration: cameraDuration(850) }
      );
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

  useEffect(() => {
    if (onMountController) {
      onMountController({
        resetCamera: () => {
          setFocusedSector(null);
          if (setFocusedCluster) setFocusedCluster(null);
          sigma.getCamera().animatedReset({ duration: cameraDuration(900) });
        },
        zoomToCategoryCluster,
        zoomToNodes,
      });
    }
  }, [onMountController, setFocusedSector, setFocusedCluster, sigma, zoomToCategoryCluster, zoomToNodes, cameraDuration]);

  useEffect(() => {
    sigmaContext.sigma = sigma;
    (window as unknown as { __sigmaInstance: unknown }).__sigmaInstance = sigma;
    (window as unknown as { __sigmaContext: unknown }).__sigmaContext = sigmaContext;
    const handleBeforeRender = () => {
      sigmaContext.frameLabelsCount = 0;
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

  useEffect(() => {
    if (!sigma) return;
    const camera = sigma.getCamera();
    const handleCameraUpdate = () => {
      const ratio = camera.ratio;
      let tier = 'overview';
      if (ratio < 0.35) tier = 'deep';
      else if (ratio < 0.65) tier = 'mid';

      if (tier !== lastLodTierRef.current) {
        lastLodTierRef.current = tier;
        sigma.refresh({ schedule: true });
      }
    };
    camera.on('updated', handleCameraUpdate);
    return () => {
      camera.off('updated', handleCameraUpdate);
    };
  }, [sigma]);

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
  
  useEffect(() => {
    if (focusedSector === null && !focusedCluster) {
      sigma.getCamera().animatedReset({ duration: cameraDuration(1000) });
    }
  }, [focusedSector, focusedCluster, sigma, cameraDuration]);

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
        graph.addNode(sectorNodeId, {
          x: cachedLayout[sectorNodeId] ? cachedLayout[sectorNodeId].x : defaultPos.x,
          y: cachedLayout[sectorNodeId] ? cachedLayout[sectorNodeId].y : defaultPos.y,
          size: 24,
          key: sectorNodeId,
          label: `${displayName}|${neonColor}|SECTOR|SECTOR|${topType}|${rank}|${repoCount}|${noteCount}`,
          color: "rgba(0,0,0,0)",
          forceLabel: isTop,
          alwaysRenderLabel: isTop,
          labelDensityWeight: isTop ? 20000 : 0,
          sector: sKey,
          sectorDisplayName: displayName,
          isSector: true,
          repoCount,
          noteCount,
        });
      }
    });

    // Nebula category hubs
    Object.keys(catCounts).forEach(cat => {
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
        const noteCount = catCounts[cat] || 0;
        
        graph.addNode(`__CAT_${cat}`, {
          x: cachedLayout[`__CAT_${cat}`] ? cachedLayout[`__CAT_${cat}`].x : defaultPos.x,
          y: cachedLayout[`__CAT_${cat}`] ? cachedLayout[`__CAT_${cat}`].y : defaultPos.y,
          size: dynSize,
          key: `__CAT_${cat}`,
          label: `${name}|${neonColor}|CATEGORY|${catType}|${topType}|${rank}|0|${noteCount}`, 
          color: "rgba(0,0,0,0)",
          forceLabel: isRoot,
          alwaysRenderLabel: isRoot,
          labelDensityWeight: isRoot ? 35000 : (isTop ? 10000 : 0),
          isRoot,
          noteCount,
        });
      }
    });

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
    safeNodes.forEach((n, nIdx) => {
      nodeCatMap.set(n.id, n.category || 'root');
      nodeDegreeMap.set(n.id, n.connections || 0);

      if (!graph.hasNode(n.id)) {
        const stellar = classifyStar(n);
        const cat = n.category || 'root';
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

      // Align category hub centroids to true mathematical barycenter of their member stars
      const catSums = new Map<string, { sumX: number; sumY: number; count: number }>();
      safeNodes.forEach((n) => {
        const isMemory = !n.ring || n.ring === 'memory';
        if (isMemory && liveGraph.hasNode(n.id)) {
          const cat = n.category || 'root';
          const entry = catSums.get(cat) || { sumX: 0, sumY: 0, count: 0 };
          entry.sumX += liveGraph.getNodeAttribute(n.id, 'x') as number;
          entry.sumY += liveGraph.getNodeAttribute(n.id, 'y') as number;
          entry.count++;
          catSums.set(cat, entry);
        }
      });
      catSums.forEach((entry, cat) => {
        const catNodeId = `__CAT_${cat}`;
        if (liveGraph.hasNode(catNodeId) && entry.count > 0) {
          liveGraph.setNodeAttribute(catNodeId, 'x', entry.sumX / entry.count);
          liveGraph.setNodeAttribute(catNodeId, 'y', entry.sumY / entry.count);
        }
      });

      // Align sector anchor centroids to barycenter of their member category hubs
      sectorMap.forEach((secInfo, sKey) => {
        const sectorNodeId = `__SECTOR_${sKey}`;
        if (liveGraph.hasNode(sectorNodeId)) {
          let sumX = 0;
          let sumY = 0;
          let totalWeight = 0;
          secInfo.categories.forEach(cat => {
            const catNodeId = `__CAT_${cat}`;
            if (liveGraph.hasNode(catNodeId)) {
              const weight = catCounts[cat] || 1;
              sumX += (liveGraph.getNodeAttribute(catNodeId, 'x') as number) * weight;
              sumY += (liveGraph.getNodeAttribute(catNodeId, 'y') as number) * weight;
              totalWeight += weight;
            }
          });
          if (totalWeight > 0) {
            liveGraph.setNodeAttribute(sectorNodeId, 'x', sumX / totalWeight);
            liveGraph.setNodeAttribute(sectorNodeId, 'y', sumY / totalWeight);
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
      // Node coordinates changed in place after ForceAtlas2. Reframe the current
      // selection against the settled layout rather than its initial position.
      setLayoutRevision(revision => revision + 1);
      
      setTimeout(() => {
        if (!sigmaContext.selectedNodeId && !sigmaContext.focusedSector) {
          const camera = sigma.getCamera();
          camera.animatedReset({ duration: cameraDuration(1500) });
        }
        
        setTimeout(() => {
          onReady();
          sigma.refresh({ schedule: true });
        }, 1500); 
      }, 300);
      
    }, waitTime);

    return () => {
      stopped = true;
      clearTimeout(timer);
      try { stop(); } catch (_e) { /* noop on unmount */ }
    };
  }, [safeNodes, safeLinks, catCounts, loadGraph, start, stop, sigma, onReady, maxNebulas, cameraDuration]);

  // Dynamically update category node attributes when maxNebulas changes
  useEffect(() => {
    const graph = sigma.getGraph();
    if (!graph || graph.order === 0) return;
    
    const allocations = calculateNebulaAllocations(Object.keys(catCounts), catCounts, maxNebulas);
    allocations.categoryMap.forEach((info, cat) => {
      const catNodeId = `__CAT_${cat}`;
      if (graph.hasNode(catNodeId)) {
        const neonColor = getColor(cat);
        const isTop = info.isOverviewVisible;
        const isRoot = info.isRoot;
        const catType = isRoot ? 'ROOT' : 'SUB';
        const topType = isTop ? 'TOP' : 'OTHER';
        const noteCount = catCounts[cat] || 0;
        
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

      sigma.getCamera().animate({
        x: camTarget.x,
        y: camTarget.y,
        ratio: camTarget.ratio,
        angle: 0,
      }, { duration: cameraDuration(850) });
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
    registerEvents({
      enterNode: (e) => {
        const graph = sigma.getGraph();
        if (e.node.startsWith('__SECTOR_')) {
          const targetSector = e.node.replace('__SECTOR_', '').toLowerCase();
          const sectorColor = getColor(targetSector);
          graph.forEachNode((node, nAttrs) => {
            if (!node.startsWith('__CAT_') && !node.startsWith('__SECTOR_')) {
              const noteCat = String(nAttrs.originalCategory || nAttrs.category || '').toLowerCase();
              const nodeSector = resolveProjectCategory(noteCat).sector.toLowerCase();
              if (nodeSector === targetSector || noteCat === targetSector || noteCat.startsWith(targetSector + '/')) {
                highlightedNodesRef.current.push({ id: node, size: (nAttrs.size as number) || 2 });
                graph.setNodeAttribute(node, 'size', ((nAttrs.size as number) || 2) * 1.5);
              }
            }
          });
          graph.forEachEdge((edge, eAttrs) => {
            if (eAttrs.isSectorTether && eAttrs.sector === targetSector) {
              graph.setEdgeAttribute(edge, 'hidden', false);
              graph.setEdgeAttribute(edge, 'color', sectorColor);
              graph.setEdgeAttribute(edge, 'size', 1.2);
            }
          });
          sigma.refresh({ schedule: true });
          return;
        }

        if (e.node.startsWith('__CAT_')) {
          const targetCat = e.node.replace('__CAT_', '').toLowerCase();
          graph.forEachNode((node, nAttrs) => {
            const noteCat = String(nAttrs.originalCategory || nAttrs.category || '').toLowerCase();
            if (!node.startsWith('__CAT_') && !node.startsWith('__SECTOR_') && (noteCat === targetCat || noteCat.startsWith(targetCat + '/'))) {
              highlightedNodesRef.current.push({ id: node, size: (nAttrs.size as number) || 2 });
              graph.setNodeAttribute(node, 'size', ((nAttrs.size as number) || 2) * 1.5);
            }
          });
          return;
        }

        const attrs = graph.getNodeAttributes(e.node);
        const pos = sigma.graphToViewport(attrs as { x: number; y: number });
        onHover({ 
          id: e.node, 
          title: attrs.label, 
          originalCategory: attrs.originalCategory,
          color: attrs.color,
          stellarClass: attrs.stellarClass,
          stellarBadge: attrs.stellarBadge,
          stellarLabel: attrs.stellarLabel,
          stellarDesc: attrs.stellarDesc,
          connections: attrs.connections,
          ring: attrs.ring,
          status: attrs.status,
          icon: attrs.icon,
        }, pos);
        
        sigmaContext.hoveredNodeId = e.node;
        sigma.refresh({ schedule: true });

        graph.forEachNeighbor(e.node, (neighbor, nAttrs) => {
          if (!neighbor.startsWith('__CAT_') && !neighbor.startsWith('__SECTOR_')) {
            highlightedNodesRef.current.push({ id: neighbor, size: (nAttrs.size as number) || 2 });
            graph.setNodeAttribute(neighbor, 'size', ((nAttrs.size as number) || 2) * 1.4);
          }
        });
      },
      leaveNode: () => {
        onHover(null, null);
        sigmaContext.hoveredNodeId = null;

        const graph = sigma.getGraph();
        highlightedNodesRef.current.forEach(({ id, size }) => {
          if (graph.hasNode(id)) {
            graph.setNodeAttribute(id, 'size', size);
          }
        });
        highlightedNodesRef.current = [];

        graph.forEachEdge((edge, eAttrs) => {
          if (eAttrs.isSectorTether && !eAttrs.hidden) {
            graph.setEdgeAttribute(edge, 'hidden', true);
            graph.setEdgeAttribute(edge, 'color', 'rgba(0,0,0,0)');
            graph.setEdgeAttribute(edge, 'size', 0.1);
          }
        });

        sigma.refresh({ schedule: true });
      },
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
          const box = bestBox;
          if (box.isStation && box.nodeId) {
            onFocusNode(box.nodeId);
            return;
          }
          if (box.isSector || box.nodeId?.startsWith('__SECTOR_')) {
            const sectorId = box.nodeId || (box.category ? ('__SECTOR_' + box.category.toLowerCase()) : null);
            if (sectorId) {
              zoomToCategoryCluster(sectorId);
              const isCurrent = sectorId.toLowerCase() === (focusedSector || '').toLowerCase();
              setFocusedSector(isCurrent ? null : sectorId);
              if (setFocusedCluster) setFocusedCluster(null);
            }
            return;
          }
          if (box.isCategory) {
            const catId = box.nodeId || (box.category ? ('__CAT_' + box.category.toLowerCase()) : null);
            if (catId) {
              zoomToCategoryCluster(catId);
              const isCurrent = catId.toLowerCase() === (focusedCluster || '').toLowerCase();
              if (isCurrent) {
                if (setFocusedCluster) setFocusedCluster(null);
              } else {
                if (setFocusedCluster) setFocusedCluster(catId);
                const rawCat = catId.replace('__CAT_', '').toLowerCase();
                const proj = resolveProjectCategory(rawCat);
                if (proj.sector && proj.sector !== 'root') {
                  setFocusedSector('__SECTOR_' + proj.sector.toLowerCase());
                }
              }
            }
            return;
          }
        }
      },
      clickNode: (e) => {
        const nodeId = e.node;
        
        if (nodeId.startsWith('__SECTOR_')) {
          zoomToCategoryCluster(nodeId);
          const isCurrent = nodeId.toLowerCase() === (focusedSector || '').toLowerCase();
          setFocusedSector(isCurrent ? null : nodeId);
          if (setFocusedCluster) setFocusedCluster(null);
        } else if (nodeId.startsWith('__CAT_')) {
          zoomToCategoryCluster(nodeId);
          const isCurrent = nodeId.toLowerCase() === (focusedCluster || '').toLowerCase();
          if (isCurrent) {
            if (setFocusedCluster) setFocusedCluster(null);
          } else {
            if (setFocusedCluster) setFocusedCluster(nodeId);
            const rawCat = nodeId.replace('__CAT_', '').toLowerCase();
            const proj = resolveProjectCategory(rawCat);
            if (proj.sector && proj.sector !== 'root') {
              setFocusedSector('__SECTOR_' + proj.sector.toLowerCase());
            }
          }
        } else {
          onFocusNode(nodeId);
        }
      }
    });
  }, [registerEvents, sigma, onHover, onFocusNode, focusedSector, setFocusedSector, focusedCluster, setFocusedCluster, zoomToCategoryCluster]);

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

  // Sector & Cluster Visibility Filter
  useEffect(() => {
    const graph = sigma.getGraph();
    if (!graph || graph.order === 0) return;
    
    if (focusedCluster) {
      const clusterCat = focusedCluster.replace('__CAT_', '').toLowerCase();
      const cleanCluster = clusterCat.replace(/^(notes|sources)\//, '');

      graph.forEachNode((node, attrs) => {
        if (node.startsWith('__SECTOR_')) {
          graph.setNodeAttribute(node, 'hidden', true);
        } else if (node.startsWith('__CAT_')) {
          const thisCat = node.replace('__CAT_', '').toLowerCase();
          const cleanThisCat = thisCat.replace(/^(notes|sources)\//, '');
          const isCurrent = thisCat === clusterCat || cleanThisCat === cleanCluster;
          graph.setNodeAttribute(node, 'hidden', !isCurrent);
        } else {
          const noteCat = String(attrs.originalCategory || attrs.category || 'root').toLowerCase();
          const cleanNoteCat = noteCat.replace(/^(notes|sources)\//, '');
          const belongsToCluster =
            noteCat === clusterCat ||
            noteCat.startsWith(clusterCat + '/') ||
            cleanNoteCat === cleanCluster ||
            cleanNoteCat.startsWith(cleanCluster + '/');
          graph.setNodeAttribute(node, 'hidden', !belongsToCluster);
        }
      });
    } else if (focusedSector) {
      const rawSectorCat = focusedSector.replace('__SECTOR_', '').replace('__CAT_', '').toLowerCase();
      const cleanSector = rawSectorCat.replace(/^(notes|sources)\//, '');

      graph.forEachNode((node, attrs) => {
        if (node.startsWith('__SECTOR_')) {
          const thisSector = node.replace('__SECTOR_', '').toLowerCase();
          const cleanThisSector = thisSector.replace(/^(notes|sources)\//, '');
          const isCurrent = thisSector === rawSectorCat || cleanThisSector === cleanSector;
          graph.setNodeAttribute(node, 'hidden', !isCurrent);
        } else if (node.startsWith('__CAT_')) {
          const thisCat = node.replace('__CAT_', '').toLowerCase();
          const cleanThisCat = thisCat.replace(/^(notes|sources)\//, '');
          const nodeSector = resolveProjectCategory(thisCat).sector.toLowerCase();
          const cleanNodeSector = nodeSector.replace(/^(notes|sources)\//, '');
          const isCurrentSector =
            thisCat === rawSectorCat ||
            cleanThisCat === cleanSector ||
            nodeSector === rawSectorCat ||
            cleanNodeSector === cleanSector;
          const isSubsector =
            thisCat.startsWith(rawSectorCat + '/') ||
            cleanThisCat.startsWith(cleanSector + '/');
          graph.setNodeAttribute(node, 'hidden', !(isCurrentSector || isSubsector));
        } else {
          const noteCat = String(attrs.originalCategory || attrs.category || 'root').toLowerCase();
          const cleanNoteCat = noteCat.replace(/^(notes|sources)\//, '');
          const nodeSector = resolveProjectCategory(noteCat).sector.toLowerCase();
          const cleanNodeSector = nodeSector.replace(/^(notes|sources)\//, '');
          const belongsToSector =
            noteCat === rawSectorCat ||
            noteCat.startsWith(rawSectorCat + '/') ||
            cleanNoteCat === cleanSector ||
            cleanNoteCat.startsWith(cleanSector + '/') ||
            nodeSector === rawSectorCat ||
            cleanNodeSector === cleanSector;
          graph.setNodeAttribute(node, 'hidden', !belongsToSector);
        }
      });
    } else {
      graph.forEachNode((node) => {
        graph.setNodeAttribute(node, 'hidden', false);
      });
    }

    graph.forEachEdge((edge, attrs, source, target) => {
      if (focusedCluster || focusedSector) {
        const sHidden = graph.getNodeAttribute(source, 'hidden');
        const tHidden = graph.getNodeAttribute(target, 'hidden');
        graph.setEdgeAttribute(edge, 'hidden', sHidden || tHidden);
      } else {
        graph.setEdgeAttribute(edge, 'hidden', attrs.originalHidden || false);
      }
    });

    sigma.refresh({ schedule: true });
  }, [focusedSector, focusedCluster, sigma]);

  return (
    <OverlayCanvas
      showRings={showRings}
      isCosmicMode={isCosmicMode && !motionReduced}
      focusedSector={focusedSector}
      orbitalLayout={orbitalLayoutRef.current}
      trafficSimulator={trafficSimulatorRef.current}
      isReady={isReady}
      constellation={constellation}
    />
  );
};
