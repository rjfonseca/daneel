import { lazy, useState, useCallback, useMemo, useEffect, useRef } from 'react';
import type { Settings } from 'sigma/settings';
import { DeferredPanel } from './components/DeferredPanel';
import { SigmaContainer } from '@react-sigma/core';
import '@react-sigma/core/lib/style.css';

// Libs & Algorithms
import {
  loadMaxNebulas,
  saveMaxNebulas,
  resolveProjectCategory,
  getCategoryDisplayName,
  getSectorDisplayName,
} from './lib/nebulas';
import { preprocessWikiLinks, resolveInternalLink } from './lib/linkResolver';
import { classifyStar } from './lib/stellarClassification';
import { type EdgeVisibilityMode } from './lib/edgeFiltering';
import { computeConstellation, type ConstellationResult } from './lib/constellationFilter';
import { NodeStellarProgram } from './lib/nodeStellarProgram';
import { sigmaContext } from './lib/sigmaContext';
import { client } from './lib/apiClient';
import {
  loadScopeMode,
  saveScopeMode,
  type GalaxyScopeMode,
  type LodTelemetry,
} from './lib/semanticZoomLod';
import { createGalaxyReducers } from './lib/galaxy/galaxyReducers';
import { MAX_CAMERA_RATIO, MIN_CAMERA_RATIO } from './lib/galaxy/smoothCamera';

// Renderers
import { drawCustomLabel, drawHover } from './renderers/canvasCustomLabels';

// Hooks
import { useGalaxyData } from './hooks/useGalaxyData';
import { useNoteSearch } from './hooks/useNoteSearch';
import { useKeyboardShortcuts } from './hooks/useKeyboardShortcuts';
import { useUIEventSync } from './hooks/useUIEventSync';
import { usePinnedAnchors } from './hooks/usePinnedAnchors';
import { useRoutineExecution } from './hooks/useRoutineExecution';
import { isNavigationSnapshot, useNavigationHistory, type NavigationSnapshot } from './hooks/useNavigationHistory';
import { decodeNavigationHash, getDirectedRelations, getExplorerNodes, isMemoryNote, normalizeRecentNotes, normalizeSavedSearches, readLocalList, shouldPushNavigation, writeLocalList, type NavigationTarget } from './lib/navigationHelpers';

// Domain Types
import type {
  GalaxyNode,
  HoverTooltipNode,
  HoverTooltipState,
  SearchResultItem,
  GalaxyApi,
} from './types/galaxy';

// UI & HUD Components
import { TopCosmicBar } from './components/TopCosmicBar';
import { type HeaderDisclosure } from './lib/headerLayout';
import { SectorResetIndicator } from './components/SectorResetIndicator';
import { CosmicLoadingOverlay } from './components/CosmicLoadingOverlay';
import { NodeHoverTooltip } from './components/NodeHoverTooltip';
import { CosmicNotificationToast } from './components/CosmicNotificationToast';
import { BottomTemporalHotbar } from './components/galaxy/BottomTemporalHotbar';
import { GalaxyController } from './components/galaxy/GalaxyController';
import { ViewControlsPanel } from './components/ViewControlsPanel';
import type { GalaxyViewState } from './lib/galaxyView';
import { loadLatestNote } from './lib/latestNoteRequest';

const FloatingNoteCard = lazy(() => import('./components/FloatingNoteCard').then(module => ({ default: module.FloatingNoteCard })));
const ArmsInspectorCard = lazy(() => import('./components/ArmsInspectorCard').then(module => ({ default: module.ArmsInspectorCard })));
const CommandSearchDialog = lazy(() => import('./components/CommandSearchDialog').then(module => ({ default: module.CommandSearchDialog })));
const ShortcutsModal = lazy(() => import('./components/hud/ShortcutsModal').then(module => ({ default: module.ShortcutsModal })));
const CockpitModal = lazy(() => import('./components/hud/CockpitModal').then(module => ({ default: module.CockpitModal })));
const KnowledgeExplorer = lazy(() => import('./components/KnowledgeExplorer').then(module => ({ default: module.KnowledgeExplorer })));
const QuickCaptureDialog = lazy(() => import('./components/QuickCaptureDialog').then(module => ({ default: module.QuickCaptureDialog })));
const KnowledgeReviewPanel = lazy(() => import('./components/KnowledgeReviewPanel').then(module => ({ default: module.KnowledgeReviewPanel })));

const RECENT_NOTES_KEY = 'daneel:recent-notes:v1';
const SAVED_SEARCHES_KEY = 'daneel:saved-searches:v1';
const HUD_ROOT_STYLE = { '--hud-content-top': '128px' } as React.CSSProperties;
const INITIAL_NAVIGATION: NavigationSnapshot = {
  target: null, camera: null, activeFilter: 'All', edgeMode: 'focus', scopeMode: 'hybrid',
  showRings: true, showSectorLabels: true, showNebulaLabels: true,
  isCosmicMode: true, reducedMotion: false, drawerWidth: 440, scrollTop: 0,
};

export default function App() {
  // 1. Galaxy Data Hook
  const { data, stats, catCounts, pinnedAnchors, weeklyPattern, unindexedLinks, isLoading: isGalaxyLoading, reloadGalaxy } = useGalaxyData();
  const { isPinned, togglePin } = usePinnedAnchors({ backendAnchors: pinnedAnchors });

  // 2. Note Search Hook
  const {
    searchQuery,
    activeFilter,
    setActiveFilter,
    filteredResults,
    isLoading: isSearchLoading,
    searchError,
    hasSearched,
    handleSearchChange,
    resetSearch,
  } = useNoteSearch({ nodes: data.nodes, catCounts });

  // 3. Selection & Navigation States
  const [activeNote, setActiveNote] = useState<GalaxyNode | null>(null);
  const [noteContent, setNoteContent] = useState<string>('');
  const [isReady, setIsReady] = useState(false);
  const [targetNodeId, setTargetNodeId] = useState<string | null>(null);
  const [focusedSector, setFocusedSector] = useState<string | null>(null);
  const [focusedCluster, setFocusedCluster] = useState<string | null>(null);
  const [hoverTooltip, setHoverTooltip] = useState<HoverTooltipState | null>(null);
  const [maxNebulas, setMaxNebulas] = useState<number>(() => loadMaxNebulas());
  const [edgeMode, setEdgeMode] = useState<EdgeVisibilityMode>('focus');
  const [scopeMode, setScopeMode] = useState<GalaxyScopeMode>(() => loadScopeMode());
  const [lodTelemetry, setLodTelemetry] = useState<LodTelemetry | null>(null);
  const [isMaximized, setIsMaximized] = useState(false);
  const [copied, setCopied] = useState(false);
  const [galaxyApi, setGalaxyApi] = useState<GalaxyApi | null>(null);

  // 4. HUD & Visibility Controls
  const [isCosmicMode, setIsCosmicMode] = useState(true);
  const [showRings, setShowRings] = useState(true);
  const [showSectorLabels, setShowSectorLabels] = useState(true);
  const [showNebulaLabels, setShowNebulaLabels] = useState(true);
  const [reducedMotion, setReducedMotion] = useState(false);
  const [drawerWidth, setDrawerWidth] = useState(440);
  const [headerDisclosure, setHeaderDisclosure] = useState<HeaderDisclosure>(null);
  const isControlsOpen = headerDisclosure === 'view';
  const [isCaptureOpen, setIsCaptureOpen] = useState(false);
  const [pendingCapturedPath, setPendingCapturedPath] = useState<string | null>(null);
  const [isReviewOpen, setIsReviewOpen] = useState(false);
  const [isSearchOpen, setIsSearchOpen] = useState(false);
  const [isShortcutsOpen, setIsShortcutsOpen] = useState(false);
  const [isCockpitOpen, setIsCockpitOpen] = useState(false);
  const [cockpitPreselectedRoutine, setCockpitPreselectedRoutine] = useState<string | null>(null);
  const routineExecution = useRoutineExecution();
  const [constellation, setConstellation] = useState<ConstellationResult | null>(null);
  const [isExplorerOpen, setIsExplorerOpen] = useState(false);
  const [recentNotes, setRecentNotes] = useState(() => readLocalList(RECENT_NOTES_KEY, normalizeRecentNotes));
  const [savedSearches, setSavedSearches] = useState(() => readLocalList(SAVED_SEARCHES_KEY, normalizeSavedSearches));
  const [readerInitialScrollTop, setReaderInitialScrollTop] = useState(0);
  const readerScrollTopRef = useRef(0);
  const pendingReaderScroll = useRef<number | null>(null);
  const noteRequestId = useRef(0);
  const historyTransition = useRef({ lastKey: '', suppressKey: null as string | null });
  const currentSnapshot = useRef<NavigationSnapshot>(INITIAL_NAVIGATION);
  const deepLinkHandled = useRef(false);
  const pendingCameraRestore = useRef<number | null>(null);
  const restoreNavigationRef = useRef<(snapshot: NavigationSnapshot) => void>(() => {});
  const navigation = useNavigationHistory(INITIAL_NAVIGATION, snapshot => restoreNavigationRef.current(snapshot));

  // 5. Actions & Handlers
  const handleDrawerWidthChange = useCallback((width: number) => {
    if (pendingCameraRestore.current !== null) window.clearTimeout(pendingCameraRestore.current);
    pendingCameraRestore.current = null;
    setDrawerWidth(width);
  }, []);

  const handleResetCamera = useCallback(() => {
    if (pendingCameraRestore.current !== null) window.clearTimeout(pendingCameraRestore.current);
    pendingCameraRestore.current = null;
    noteRequestId.current += 1;
    setActiveNote(null);
    setNoteContent('');
    if (galaxyApi) {
      galaxyApi.resetCamera();
    }
    setFocusedSector(null);
    setFocusedCluster(null);
    setTargetNodeId(null);
    setConstellation(null);
    sigmaContext.constellation = null;
    sigmaContext.selectedNodeId = null;
    sigmaContext.focusedSector = null;
    sigmaContext.focusedCluster = null;
    sigmaContext.sigma?.refresh({ schedule: true });
  }, [galaxyApi]);

  const handleSelectSector = useCallback(() => {
    if (pendingCameraRestore.current !== null) window.clearTimeout(pendingCameraRestore.current);
    pendingCameraRestore.current = null;
    noteRequestId.current += 1;
    let sectorToZoom = focusedSector;
    if (!sectorToZoom && activeNote) {
      const cat = activeNote.category || activeNote.id || '';
      const proj = resolveProjectCategory(cat);
      sectorToZoom = '__SECTOR_' + proj.sector.toLowerCase();
      setFocusedSector(sectorToZoom);
    }
    if (sectorToZoom && galaxyApi?.zoomToCategoryCluster) {
      galaxyApi.zoomToCategoryCluster(sectorToZoom);
    }
    setTargetNodeId(sectorToZoom);
    setFocusedCluster(null);
    setActiveNote(null);
    sigmaContext.focusedCluster = null;
    sigmaContext.selectedNodeId = null;
    sigmaContext.sigma?.refresh({ schedule: true });
  }, [focusedSector, activeNote, galaxyApi]);

  const handleAutoUnfocus = useCallback((opts: { clearCluster: boolean; clearSector: boolean; clearConstellation?: boolean }) => {
    if (opts.clearConstellation) {
      setConstellation(null);
      sigmaContext.constellation = null;
      setTargetNodeId(prev => (prev && !prev.startsWith('__SECTOR_') && !prev.startsWith('__CAT_') ? null : prev));
    }
    if (opts.clearCluster) {
      setFocusedCluster(null);
      sigmaContext.focusedCluster = null;
      setTargetNodeId(prev => (prev && prev.startsWith('__CAT_') ? null : prev));
    }
    if (opts.clearSector) {
      setFocusedSector(null);
      sigmaContext.focusedSector = null;
      setTargetNodeId(prev => (prev && (prev.startsWith('__SECTOR_') || prev.startsWith('__CAT_')) ? null : prev));
    }
  }, []);

  const activeSectorName = useMemo(() => {
    if (activeNote) {
      const cat = activeNote.category || activeNote.id || '';
      return getSectorDisplayName(cat);
    }
    if (focusedSector) {
      return getSectorDisplayName(focusedSector);
    }
    if (focusedCluster) {
      return getSectorDisplayName(focusedCluster);
    }
    return null;
  }, [focusedSector, focusedCluster, activeNote]);

  const activeClusterName = useMemo(() => {
    if (activeNote) {
      const cat = activeNote.category || activeNote.id || '';
      const catDisplay = getCategoryDisplayName(cat);
      if (catDisplay && catDisplay !== activeSectorName) {
        return catDisplay;
      }
    }
    if (focusedCluster) {
      const clusterDisplay = getCategoryDisplayName(focusedCluster);
      if (clusterDisplay && clusterDisplay !== activeSectorName) {
        return clusterDisplay;
      }
    }
    return null;
  }, [focusedCluster, activeNote, activeSectorName]);

  const handleEdgeModeChange = useCallback((mode: EdgeVisibilityMode) => {
    setEdgeMode(mode);
    sigmaContext.edgeMode = mode;
    sigmaContext.sigma?.refresh({ schedule: true });
  }, []);

  const handleScopeModeChange = useCallback((mode: GalaxyScopeMode) => {
    setScopeMode(mode);
    saveScopeMode(mode);
    sigmaContext.scopeMode = mode;
  }, []);

  const processedMarkdown = useMemo(() => {
    return preprocessWikiLinks(noteContent);
  }, [noteContent]);

  const handleCopyNote = useCallback(() => {
    if (noteContent) {
      navigator.clipboard.writeText(noteContent);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  }, [noteContent]);

  const updateMaxNebulas = useCallback((val: number) => {
    const clamped = Math.min(32, Math.max(1, val));
    setMaxNebulas(clamped);
    saveMaxNebulas(clamped);
    sigmaContext.maxNebulas = clamped;
  }, []);

  const handleReady = useCallback(() => {
    setIsReady(true);
  }, []);

  // Constellation isolation + note activation
  const focusNode = useCallback((res: GalaxyNode | SearchResultItem | string | null) => {
    if (!res) return;
    if (pendingCameraRestore.current !== null) window.clearTimeout(pendingCameraRestore.current);
    const targetPath = typeof res === 'string' ? res : (res.relPath || res.id || '');
    const isCat = targetPath.startsWith('__CAT_') || targetPath.startsWith('__SECTOR_') || (typeof res === 'object' && Boolean(res.isCat));

    setTargetNodeId(targetPath);
    resetSearch();
    setIsSearchOpen(false);

    if (!isCat) {
      setHeaderDisclosure(null);
      setNoteContent("Loading telemetry...");
      readerScrollTopRef.current = 0;
      pendingReaderScroll.current = null;
      setReaderInitialScrollTop(0);
      const matched = data.nodes.find(n => n.id === targetPath);
      const rawNode: GalaxyNode = matched
        ? { ...matched, ...(typeof res === 'object' ? res : {}) }
        : (typeof res === 'string' ? { id: res, title: res.split('/').pop() } : { ...res, id: targetPath });
      if (!rawNode.id) rawNode.id = targetPath;
      const stellar = classifyStar(rawNode);
      setActiveNote({ ...rawNode, ...stellar, stellarBadge: stellar.badge });
      sigmaContext.selectedNodeId = targetPath;
      if (isMemoryNote(rawNode)) {
        setRecentNotes(previous => normalizeRecentNotes([
          { id: targetPath, title: rawNode.title || targetPath.split('/').pop() || targetPath },
          ...previous.filter(item => item.id !== targetPath),
        ]));
      }

      // Compute Constellation Isolation (k-hop = 2)
      if (sigmaContext.sigma) {
        const graph = sigmaContext.sigma.getGraph();
        const constResult = computeConstellation(graph, targetPath, 2);
        setConstellation(constResult);
        sigmaContext.constellation = constResult;
        sigmaContext.sigma.refresh({ schedule: true });
      }

      void loadLatestNote(noteRequestId,
        () => client.getNote({ relPath: targetPath }).then(result => result.contentMarkdown),
        setNoteContent,
        () => setNoteContent("> **Error loading note or satellite content**"));
    } else {
      noteRequestId.current += 1;
      setActiveNote(null);
      setNoteContent('');
      setIsMaximized(false);
      setConstellation(null);
      sigmaContext.constellation = null;
      sigmaContext.selectedNodeId = null;
      if (targetPath.startsWith('__CAT_')) {
        setFocusedCluster(targetPath);
        const category = targetPath.replace('__CAT_', '');
        const sector = resolveProjectCategory(category).sector;
        setFocusedSector(sector && sector !== 'root' ? `__SECTOR_${sector.toLowerCase()}` : null);
      } else if (targetPath.startsWith('__SECTOR_')) {
        setFocusedCluster(null);
        setFocusedSector(targetPath);
      }
      sigmaContext.sigma?.refresh({ schedule: true });
    }
  }, [data.nodes, resetSearch]);

  const handleNavigateInternalLink = useCallback((href: string) => {
    const currentPath = activeNote?.relPath || activeNote?.id || '';
    const resolved = resolveInternalLink(href, currentPath, data.nodes);
    if (resolved) focusNode(resolved);
  }, [activeNote, data.nodes, focusNode]);

  const handleResetConstellation = useCallback(() => {
    setConstellation(null);
    sigmaContext.constellation = null;
    sigmaContext.sigma?.refresh({ schedule: true });
  }, []);

  const handleCloseNote = useCallback(() => {
    if (pendingCameraRestore.current !== null) window.clearTimeout(pendingCameraRestore.current);
    pendingCameraRestore.current = null;
    noteRequestId.current += 1;
    setActiveNote(null);
    setNoteContent('');
    setTargetNodeId(null);
    setIsMaximized(false);
    setConstellation(null);
    sigmaContext.constellation = null;
    sigmaContext.selectedNodeId = null;
    sigmaContext.sigma?.refresh({ schedule: true });
  }, []);

  const selectedNavigationTarget: NavigationTarget | null = useMemo(() => activeNote
    ? { kind: 'note', id: activeNote.id }
    : focusedCluster
      ? { kind: 'cluster', id: focusedCluster.replace(/^__CAT_/, '') }
      : focusedSector
        ? { kind: 'sector', id: focusedSector.replace(/^__SECTOR_/, '') }
        : null, [activeNote, focusedCluster, focusedSector]);
  const selectedTargetKey = selectedNavigationTarget ? `${selectedNavigationTarget.kind}:${selectedNavigationTarget.id}` : '';

  useEffect(() => { writeLocalList(RECENT_NOTES_KEY, recentNotes); }, [recentNotes]);
  useEffect(() => { writeLocalList(SAVED_SEARCHES_KEY, savedSearches); }, [savedSearches]);
  useEffect(() => {
    if (pendingReaderScroll.current === null || !noteContent || noteContent === 'Loading telemetry...') return;
    setReaderInitialScrollTop(pendingReaderScroll.current);
    pendingReaderScroll.current = null;
  }, [noteContent]);

  restoreNavigationRef.current = snapshot => {
    historyTransition.current.suppressKey = snapshot.target ? `${snapshot.target.kind}:${snapshot.target.id}` : '';
    currentSnapshot.current = snapshot;
    setActiveFilter(snapshot.activeFilter);
    handleEdgeModeChange(snapshot.edgeMode);
    if (snapshot.scopeMode) handleScopeModeChange(snapshot.scopeMode);
    setShowRings(snapshot.showRings);
    setShowSectorLabels(snapshot.showSectorLabels);
    setShowNebulaLabels(snapshot.showNebulaLabels);
    setIsCosmicMode(snapshot.isCosmicMode);
    setReducedMotion(snapshot.reducedMotion);
    setDrawerWidth(snapshot.drawerWidth);
    if (snapshot.target?.kind === 'note') {
      focusNode(snapshot.target.id);
    } else if (snapshot.target?.kind === 'cluster') {
      focusNode(`__CAT_${snapshot.target.id}`);
    } else if (snapshot.target?.kind === 'sector') {
      focusNode(`__SECTOR_${snapshot.target.id}`);
    } else {
      handleCloseNote();
      setFocusedCluster(null);
      setFocusedSector(null);
      sigmaContext.focusedCluster = null;
      sigmaContext.focusedSector = null;
    }
    readerScrollTopRef.current = snapshot.scrollTop;
    pendingReaderScroll.current = snapshot.target?.kind === 'note' ? snapshot.scrollTop : null;
    if (pendingCameraRestore.current !== null) window.clearTimeout(pendingCameraRestore.current);
    if (snapshot.camera) {
      pendingCameraRestore.current = window.setTimeout(() => {
        sigmaContext.sigma?.getCamera().animate(snapshot.camera!, { duration: 1 });
        pendingCameraRestore.current = null;
      }, 900);
    }
  };

  useEffect(() => {
    if (!isReady || isGalaxyLoading || deepLinkHandled.current) return;
    deepLinkHandled.current = true;
    const target = decodeNavigationHash(window.location.hash);
    if (!target) return;
    const browserState = window.history.state as { __daneelNavigation?: unknown; snapshot?: unknown } | null;
    if (browserState?.__daneelNavigation === true && isNavigationSnapshot(browserState.snapshot) &&
      browserState.snapshot.target?.kind === target.kind && browserState.snapshot.target.id === target.id) {
      restoreNavigationRef.current(browserState.snapshot);
      return;
    }
    const openInitialTarget = (nodeId: string) => {
      historyTransition.current.suppressKey = `${target.kind}:${target.id}`;
      const initial = { ...currentSnapshot.current, target };
      currentSnapshot.current = initial;
      navigation.replace(initial);
      focusNode(nodeId);
    };
    if (target.kind === 'note') {
      openInitialTarget(target.id);
    } else if (target.kind === 'cluster') {
      if (Object.keys(catCounts).some(category => category === target.id || category.startsWith(`${target.id}/`))) {
        openInitialTarget(`__CAT_${target.id}`);
      }
    } else if (data.nodes.some(node => resolveProjectCategory(node.originalCategory || node.category || '').sector.toLowerCase() === target.id.toLowerCase())) {
      openInitialTarget(`__SECTOR_${target.id}`);
    }
  }, [isReady, isGalaxyLoading, data.nodes, catCounts, focusNode, navigation]);

  useEffect(() => {
    if (!deepLinkHandled.current) return;
    if (!shouldPushNavigation(historyTransition.current, selectedTargetKey)) return;
    const camera = sigmaContext.sigma?.getCamera().getState() || null;
    const next: NavigationSnapshot = { ...currentSnapshot.current, target: selectedNavigationTarget, camera, scrollTop: 0 };
    currentSnapshot.current = next;
    navigation.push(next);
  }, [selectedTargetKey, selectedNavigationTarget, navigation]);

  useEffect(() => {
    if (!deepLinkHandled.current) return;
    currentSnapshot.current = {
      ...currentSnapshot.current, activeFilter, edgeMode, scopeMode, showRings,
      showSectorLabels, showNebulaLabels, isCosmicMode, reducedMotion, drawerWidth,
    };
    navigation.replace(currentSnapshot.current);
  }, [activeFilter, edgeMode, scopeMode, showRings, showSectorLabels, showNebulaLabels, isCosmicMode, reducedMotion, drawerWidth, navigation]);

  useEffect(() => {
    const camera = sigmaContext.sigma?.getCamera();
    if (!camera) return;
    let replaceTimer: number | null = null;
    const updated = () => {
      if (!deepLinkHandled.current || pendingCameraRestore.current !== null) return;
      currentSnapshot.current = { ...currentSnapshot.current, camera: camera.getState() };
      if (replaceTimer !== null) window.clearTimeout(replaceTimer);
      replaceTimer = window.setTimeout(() => {
        replaceTimer = null;
        navigation.replace(currentSnapshot.current);
      }, 250);
    };
    camera.on('updated', updated);
    return () => {
      camera.off('updated', updated);
      if (replaceTimer !== null) window.clearTimeout(replaceTimer);
    };
  }, [galaxyApi, navigation]);

  useEffect(() => () => {
    noteRequestId.current += 1;
    if (pendingCameraRestore.current !== null) window.clearTimeout(pendingCameraRestore.current);
  }, []);

  useEffect(() => {
    if (!pendingCapturedPath) return;
    let frame = 0;
    let cancelled = false;
    const openSavedNote = () => {
      if (cancelled) return;
      focusNode(pendingCapturedPath);
      setPendingCapturedPath(null);
    };
    const fallback = window.setTimeout(openSavedNote, 4000);
    const waitForGraph = () => {
      if (cancelled) return;
      if (sigmaContext.sigma?.getGraph().hasNode(pendingCapturedPath)) {
        window.clearTimeout(fallback);
        openSavedNote();
      } else {
        frame = window.requestAnimationFrame(waitForGraph);
      }
    };
    if (data.nodes.some(node => node.id === pendingCapturedPath)) waitForGraph();
    return () => { cancelled = true; window.clearTimeout(fallback); window.cancelAnimationFrame(frame); };
  }, [pendingCapturedPath, data.nodes, focusNode]);

  // 6. Keyboard Shortcuts Hook
  useKeyboardShortcuts({
    onToggleSearch: () => { setHeaderDisclosure(null); setIsSearchOpen(prev => !prev); },
    isSearchOpen,
    onCloseSearch: () => setIsSearchOpen(false),
    isControlsOpen,
    onCloseControls: () => setHeaderDisclosure(null),
    isExplorerOpen,
    onCloseExplorer: () => setIsExplorerOpen(false),
    isReviewOpen,
    onCloseReview: () => setIsReviewOpen(false),
    isCaptureOpen,
    onCloseCapture: () => setIsCaptureOpen(false),
    hasActiveNote: Boolean(activeNote),
    onCloseNote: handleCloseNote,
    isShortcutsOpen,
    onToggleShortcuts: () => { setHeaderDisclosure(null); setIsShortcutsOpen(prev => !prev); },
    onCloseShortcuts: () => setIsShortcutsOpen(false),
    isCockpitOpen,
    onToggleCockpit: () => { setHeaderDisclosure(null); setIsCockpitOpen(prev => !prev); },
    onCloseCockpit: () => setIsCockpitOpen(false),
  });

  const handleFocusNode = useCallback((nodeId: string) => {
    focusNode(data.nodes.find(n => n.id === nodeId) || nodeId);
  }, [focusNode, data.nodes]);

  const handleSetFocusedSector = useCallback((sector: string | null) => {
    if (sector) {
      noteRequestId.current += 1;
      setActiveNote(null);
      setConstellation(null);
      sigmaContext.constellation = null;
      sigmaContext.selectedNodeId = null;
    }
    setTargetNodeId(sector);
    setFocusedSector(sector);
  }, []);

  const handleSetFocusedCluster = useCallback((cluster: string | null) => {
    if (cluster) {
      noteRequestId.current += 1;
      setActiveNote(null);
      setConstellation(null);
      sigmaContext.constellation = null;
      sigmaContext.selectedNodeId = null;
    }
    setTargetNodeId(cluster);
    setFocusedCluster(cluster);
  }, []);

  const handleHover = useCallback((node: HoverTooltipNode | null, pos: { x: number; y: number } | null) => {
    setHoverTooltip(node && pos ? { node, pos } : null);
  }, []);

  // Direct stellar neighbors for active note HUD
  const activeNeighbors = useMemo(() => {
    if (!activeNote || !sigmaContext.sigma) return [];
    const graph = sigmaContext.sigma.getGraph();
    if (!graph || !graph.hasNode(activeNote.id)) return [];

    const list: Array<{ id: string; title: string; color?: string; stellarBadge?: string }> = [];
    graph.forEachNeighbor(activeNote.id, (neighbor: string, attrs: Record<string, unknown>) => {
      if (!neighbor.startsWith('__CAT_')) {
        list.push({
          id: neighbor,
          title: String(attrs.label || neighbor.split('/').pop() || neighbor),
          color: attrs.color as string | undefined,
          stellarBadge: attrs.stellarBadge as string | undefined,
        });
      }
    });
    return list;
  }, [activeNote]);

  const activeRelations = useMemo(() => activeNote
    ? getDirectedRelations(activeNote.id, data.nodes, data.links)
    : { incoming: [], outgoing: [] }, [activeNote, data.nodes, data.links]);

  const explorerNodes = useMemo(() => getExplorerNodes(data.nodes, {
    visibleNodes: constellation?.visibleNodes,
    focusedCluster,
    focusedSector,
  }), [data.nodes, constellation, focusedCluster, focusedSector]);
  const explorerScopeLabel = constellation ? 'Constellation' : focusedCluster
    ? getCategoryDisplayName(focusedCluster) : focusedSector
      ? getSectorDisplayName(focusedSector) : 'Galaxy';

  const handleReaderScroll = useCallback((top: number) => {
    if (noteContent === 'Loading telemetry...' || pendingReaderScroll.current !== null || !Number.isFinite(top)) return;
    readerScrollTopRef.current = Math.max(0, top);
    currentSnapshot.current = { ...currentSnapshot.current, scrollTop: readerScrollTopRef.current };
    navigation.replace(currentSnapshot.current);
  }, [noteContent, navigation]);

  const handleSaveSearch = useCallback(() => {
    const query = searchQuery.trim();
    if (query.length < 2) return;
    setSavedSearches(previous => normalizeSavedSearches([{ query }, ...previous]));
  }, [searchQuery]);

  const viewState: GalaxyViewState = useMemo(() => ({
    edgeMode, scopeMode, showSectorLabels, showNebulaLabels, isCosmicMode, showRings, reducedMotion,
  }), [edgeMode, scopeMode, showSectorLabels, showNebulaLabels, isCosmicMode, showRings, reducedMotion]);
  const handleViewChange = useCallback((next: GalaxyViewState) => {
    handleEdgeModeChange(next.edgeMode);
    if (next.scopeMode !== scopeMode) {
      handleScopeModeChange(next.scopeMode);
    }
    setShowSectorLabels(next.showSectorLabels);
    setShowNebulaLabels(next.showNebulaLabels);
    setIsCosmicMode(next.isCosmicMode);
    setShowRings(next.showRings);
    setReducedMotion(next.reducedMotion);
  }, [handleEdgeModeChange, handleScopeModeChange, scopeMode]);

  const { toastNotification: remoteToast, clearToast: clearRemoteToast } = useUIEventSync({
    setSelectedNodeId: (id: string | null) => { if (id) focusNode(id); },
    setIsDrawerOpen: () => {},
    setIsConstellationIsolated: () => {},
    setFocusedSector: setFocusedSector,
    reloadGalaxy: reloadGalaxy,
    selectedNodeId: targetNodeId,
    focusedSector: focusedSector
  });

  const [localToast, setLocalToast] = useState<{ title: string; message: string; id: number } | null>(null);
  const toastNotification = remoteToast || localToast;
  const handleClearToast = useCallback(() => {
    clearRemoteToast();
    setLocalToast(null);
  }, [clearRemoteToast]);

  const handleToast = useCallback((title: string, message: string) => {
    setLocalToast({ title, message, id: Date.now() });
  }, []);

  const handleFocusWeek = useCallback((weekNodeIds: string[]) => {
    if (galaxyApi?.zoomToNodes && weekNodeIds.length > 0) {
      galaxyApi.zoomToNodes(weekNodeIds);
    } else if (galaxyApi?.zoomToCategoryCluster) {
      galaxyApi.zoomToCategoryCluster('daily');
    }
  }, [galaxyApi]);

  const activePath = activeNote ? (activeNote.relPath || activeNote.id) : '';
  const isCurrentNotePinned = isPinned(activePath);
  const handleTogglePinCurrentNote = useCallback(() => {
    if (!activeNote) return;
    const path = activeNote.relPath || activeNote.id;
    const label = activeNote.title || path.split('/').pop()?.replace(/\.md$/, '') || path;
    const nowPinned = togglePin({ path, label });
    handleToast('Seldon Anchors', nowPinned ? `Pinned to hotbar: ${label}` : `Unpinned: ${label}`);
  }, [activeNote, togglePin, handleToast]);

  const isArmsStation = Boolean(
    activeNote && (
      activeNote.ring === 'skills' ||
      activeNote.ring === 'routines' ||
      activeNote.ring === 'harvesters' ||
      activeNote.id?.startsWith('harvester:') ||
      activeNote.id?.startsWith('routine:') ||
      activeNote.id?.startsWith('skill:') ||
      (activeNote.category || '').startsWith('arms')
    )
  );

  const sigmaStyle = useMemo(() => ({
    width: "100vw",
    height: "100vh",
    position: "absolute" as const,
    top: 0,
    left: 0,
    zIndex: 0,
    background: "#050508",
    opacity: isReady ? 1 : 0,
    transition: "opacity 1.5s ease-in-out"
  }), [isReady]);

  // Sigma Reducers with Constellation Isolation Support
  const sigmaSettings = useMemo<Partial<Settings>>(() => ({
    defaultDrawNodeLabel: drawCustomLabel,
    defaultDrawNodeHover: drawHover,
    renderEdgeLabels: false,
    hideEdgesOnMove: false,
    hideLabelsOnMove: false,
    minCameraRatio: MIN_CAMERA_RATIO,
    maxCameraRatio: MAX_CAMERA_RATIO,
    defaultNodeType: "circle",
    nodeProgramClasses: {
      circle: NodeStellarProgram,
    },
    labelRenderedSizeThreshold: 8,
    labelDensityWeight: 1,
    labelGridCellSize: 120,
    minEdgeThickness: 0.1,
    zIndex: true,
    ...createGalaxyReducers(() => sigmaContext),
  }), []);

  return (
    <div data-hud-root style={HUD_ROOT_STYLE} className="w-screen h-screen bg-void flex relative text-white font-sans overflow-hidden dark select-none">

      <CosmicNotificationToast
        notification={toastNotification}
        onClose={handleClearToast}
      />

      {/* Unified Command Bridge */}
      <TopCosmicBar
        stats={stats}
        visibleStarsCount={lodTelemetry?.visibleStars}
        onResetCamera={handleResetCamera}
        onOpenSearch={() => { setHeaderDisclosure(null); setIsSearchOpen(true); }}
        onToggleControls={() => setHeaderDisclosure(current => current === 'view' ? null : 'view')}
        isControlsOpen={isControlsOpen}
        activeSectorName={activeSectorName}
        activeClusterName={activeClusterName}
        onSelectSector={handleSelectSector}
        onOpenShortcuts={() => { setHeaderDisclosure(null); setIsShortcutsOpen(true); }}
        isCockpitOpen={isCockpitOpen}
        onToggleCockpit={() => { setHeaderDisclosure(null); setIsCockpitOpen(prev => !prev); }}
        disclosure={headerDisclosure}
        onDisclosureChange={setHeaderDisclosure}
        navigation={{
          canBack: navigation.canBack,
          canForward: navigation.canForward,
          onBack: navigation.back,
          onForward: navigation.forward,
          isExplorerOpen,
          onToggleExplorer: () => { setHeaderDisclosure(null); setIsReviewOpen(false); setIsExplorerOpen(open => !open); },
          recentNotes,
          savedSearches,
          searchQuery,
          onSelectRecent: handleFocusNode,
          onSelectSavedSearch: query => { void handleSearchChange(query); setHeaderDisclosure(null); setIsSearchOpen(true); },
          onSaveSearch: handleSaveSearch,
          onRemoveSavedSearch: query => setSavedSearches(previous => previous.filter(item => item.query !== query)),
          onOpenCapture: () => { setHeaderDisclosure(null); setIsCaptureOpen(true); },
          onOpenReview: () => { setHeaderDisclosure(null); setIsExplorerOpen(false); setIsReviewOpen(true); },
        }}
      />

      <DeferredPanel isOpen={isExplorerOpen} label="Loading notes…">
        {isExplorerOpen && <KnowledgeExplorer
          nodes={explorerNodes}
          scopeLabel={explorerScopeLabel}
          onSelectNode={handleFocusNode}
          onClose={() => setIsExplorerOpen(false)}
        />}
      </DeferredPanel>

      <DeferredPanel isOpen={isReviewOpen} label="Loading review…">
        <KnowledgeReviewPanel
          isOpen={isReviewOpen}
          onClose={() => setIsReviewOpen(false)}
          nodes={data.nodes}
          links={data.links}
          unindexedLinks={unindexedLinks}
          onSelectNode={handleFocusNode}
        />
      </DeferredPanel>

      <DeferredPanel isOpen={isCaptureOpen} label="Loading capture…">
        <QuickCaptureDialog
          isOpen={isCaptureOpen}
          onClose={() => setIsCaptureOpen(false)}
          onCaptured={(path, warning) => {
            reloadGalaxy();
            if (warning) {
              setPendingCapturedPath(null);
              focusNode(path);
            } else {
              setPendingCapturedPath(path);
            }
            handleToast('Saved to Inbox', warning || 'Your note is ready to read.');
            setIsCaptureOpen(false);
          }}
        />
      </DeferredPanel>

      {/* Keyboard Shortcuts Cheatsheet Modal */}
      <DeferredPanel isOpen={isShortcutsOpen} label="Loading keyboard shortcuts…">
        <ShortcutsModal
          isOpen={isShortcutsOpen}
          onClose={() => setIsShortcutsOpen(false)}
        />
      </DeferredPanel>

      {/* Operational Cockpit & Action Deck Modal */}
      <DeferredPanel isOpen={isCockpitOpen} label="Loading operational cockpit…">
        <CockpitModal
          isOpen={isCockpitOpen}
          onClose={() => setIsCockpitOpen(false)}
          preselectedRoutineName={cockpitPreselectedRoutine}
          execution={routineExecution}
        />
      </DeferredPanel>

      {/* Floating Controls Dropdown */}
      <ViewControlsPanel
        isOpen={isControlsOpen}
        onClose={() => setHeaderDisclosure(null)}
        view={viewState}
        onChange={handleViewChange}
        maxNebulas={maxNebulas}
        onUpdateMaxNebulas={updateMaxNebulas}
        lodTelemetry={lodTelemetry}
      />

      {/* Floating Command Search Dialog */}
      <DeferredPanel isOpen={isSearchOpen} label="Loading search…">
        <CommandSearchDialog
          isOpen={isSearchOpen}
          onClose={() => setIsSearchOpen(false)}
          searchQuery={searchQuery}
          onSearchChange={handleSearchChange}
          activeFilter={activeFilter}
          onSelectFilter={setActiveFilter}
          filteredResults={filteredResults}
          isLoading={isSearchLoading}
          searchError={searchError}
          hasSearched={hasSearched}
          onSaveSearch={handleSaveSearch}
          onSelectResult={(res) => focusNode(res)}
        />
      </DeferredPanel>

      {/* Sector Reset Indicator */}
      <SectorResetIndicator
        focusedSector={focusedSector || focusedCluster}
        onReset={handleResetCamera}
        isConstellationActive={Boolean(constellation)}
        onResetConstellation={handleResetConstellation}
        hasActiveDrawer={Boolean(activeNote)}
        drawerWidth={drawerWidth}
        isObstructedBySidePanel={isExplorerOpen || isReviewOpen}
      />

      {/* Bottom Cybernetic HUD Hotbar */}
      <BottomTemporalHotbar
        nodes={data.nodes}
        onSelectNode={handleFocusNode}
        onFocusWeek={handleFocusWeek}
        onToast={handleToast}
        isMaximized={isMaximized}
        hasActiveDrawer={Boolean(activeNote)}
        drawerWidth={drawerWidth}
        backendAnchors={pinnedAnchors}
        weeklyPattern={weeklyPattern}
      />

      {/* Loading Overlay */}
      <CosmicLoadingOverlay isReady={isReady} />

      {/* Node Hover Tooltip */}
      <NodeHoverTooltip hoverTooltip={hoverTooltip} />

      {/* Active Note or ARMS Inspector HUD */}
      <DeferredPanel isOpen={Boolean(activeNote)} label="Loading note…">
        {activeNote && (
          isArmsStation ? (
            <ArmsInspectorCard
              station={activeNote}
              contentMarkdown={noteContent}
              onClose={handleCloseNote}
              isConstellationActive={Boolean(constellation)}
              onResetConstellation={handleResetConstellation}
              onNavigateInternalLink={handleNavigateInternalLink}
              execution={routineExecution}
              onOpenCockpit={(routineName) => {
                if (routineName) {
                  setCockpitPreselectedRoutine(routineName);
                }
                setIsCockpitOpen(true);
              }}
            />
          ) : (
            <FloatingNoteCard
              activeNote={activeNote}
              noteContent={noteContent}
              processedMarkdown={processedMarkdown}
              isMaximized={isMaximized}
              onToggleMaximize={() => setIsMaximized(prev => !prev)}
              onClose={handleCloseNote}
              onCopy={handleCopyNote}
              copied={copied}
              onNavigateInternalLink={handleNavigateInternalLink}
              isConstellationActive={Boolean(constellation)}
              onResetConstellation={handleResetConstellation}
              neighbors={activeNeighbors}
              incomingLinks={activeRelations.incoming}
              outgoingLinks={activeRelations.outgoing}
              initialScrollTop={readerInitialScrollTop}
              onScrollPositionChange={handleReaderScroll}
              drawerWidth={drawerWidth}
              onDrawerWidthChange={handleDrawerWidthChange}
              isPinned={isCurrentNotePinned}
              onTogglePin={handleTogglePinCurrentNote}
            />
          )
        )}

      </DeferredPanel>

      {/* Main 2D WebGL/Canvas Sigma Container */}
      <SigmaContainer style={sigmaStyle} settings={sigmaSettings}>
        <GalaxyController
          safeNodes={data.nodes}
          safeLinks={data.links}
          catCounts={catCounts}
          onFocusNode={handleFocusNode}
          onReady={handleReady}
          targetNodeId={targetNodeId}
          focusedSector={focusedSector}
          setFocusedSector={handleSetFocusedSector}
          focusedCluster={focusedCluster}
          setFocusedCluster={handleSetFocusedCluster}
          onHover={handleHover}
          maxNebulas={maxNebulas}
          onMountController={setGalaxyApi}
          edgeMode={edgeMode}
          isCosmicMode={isCosmicMode}
          showRings={showRings}
          constellation={constellation}
          isReady={isReady}
          showSectorLabels={showSectorLabels}
          showNebulaLabels={showNebulaLabels}
          drawerWidth={drawerWidth}
          reducedMotion={reducedMotion}
          scopeMode={scopeMode}
          onLodTelemetryChange={setLodTelemetry}
          onAutoUnfocus={handleAutoUnfocus}
        />
      </SigmaContainer>
    </div>
  );
}
