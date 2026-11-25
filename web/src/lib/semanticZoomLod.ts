/**
 * Semantic Zoom Level-of-Detail (LOD) & Progressive Disclosure Engine.
 *
 * Implements 3-tier hierarchical collapsing with camera ratio hysteresis,
 * asymmetric viewport frustum culling (anti-popping), and ADR-aligned
 * Trust Scope separation (`curated` vs `sources` vs `arms`).
 */

export type GalaxyScopeMode = 'curated' | 'hybrid' | 'all';
export type SemanticZoomTier = 'overview' | 'sector' | 'cluster';
export type TrustScope = 'curated' | 'sources' | 'arms';

export const DEFAULT_SCOPE_MODE: GalaxyScopeMode = 'hybrid';
export const STORAGE_KEY_SCOPE_MODE = 'daneel_scope_mode';
export const MAX_EXPANDED_SECTORS = 8;
export const MAX_EXPANDED_CLUSTERS = 12;

export interface SuperNodeMeta {
  id: string;
  kind: 'sector' | 'cluster';
  key: string;
  sectorKey: string;
  displayName: string;
  color: string;
  trustScope: TrustScope;
  noteCount: number;
  repoCount: number;
  recentCount: number;
  x: number;
  y: number;
  minX?: number;
  maxX?: number;
  minY?: number;
  maxY?: number;
}

export interface LodNodeMeta {
  trustScope: TrustScope;
  sectorKey: string;
  clusterKey: string;
  isClusterAlpha?: boolean;
  stellarClass?: string;
  connections?: number;
}

export interface SemanticLodState {
  scopeMode: GalaxyScopeMode;
  tier: SemanticZoomTier;
  expandedSectors: Set<string>;
  expandedClusters: Set<string>;
  collapsedSectorNodes: Set<string>;
  collapsedClusterNodes: Set<string>;
  visibleStarCount: number;
  totalStarCount: number;
  collapsedGroupCount: number;
}

export interface LodTelemetry {
  tier: SemanticZoomTier;
  visibleStars: number;
  totalStars: number;
  groupedStars: number;
  collapsedGroups: number;
}

export interface ComputeSemanticLodInput {
  scopeMode: GalaxyScopeMode;
  cameraRatio: number;
  viewportDims: { width: number; height: number };
  previousState: SemanticLodState | null;
  sectors: SuperNodeMeta[];
  clusters: SuperNodeMeta[];
  curatedStarCount: number;
  totalStarCount: number;
  projectToViewport: (x: number, y: number) => { x: number; y: number } | null;
}

/**
 * Classifies a node into its canonical trust scope aligned with ADR 0042.
 */
export function classifyTrustScope(node: {
  id?: string;
  category?: string;
  originalCategory?: string;
  ring?: string;
}): TrustScope {
  const ring = (node.ring || '').toLowerCase();
  const id = (node.id || '').replace(/^\/+/, '').toLowerCase();
  const cat = (node.originalCategory || node.category || '').replace(/^\/+/, '').toLowerCase();

  if (
    ring === 'skills' ||
    ring === 'routines' ||
    ring === 'harvesters' ||
    id.startsWith('harvester:') ||
    id.startsWith('routine:') ||
    id.startsWith('skill:') ||
    cat.startsWith('arms')
  ) {
    return 'arms';
  }

  if (
    id === 'sources' ||
    id.startsWith('sources/') ||
    cat === 'sources' ||
    cat.startsWith('sources/')
  ) {
    return 'sources';
  }

  return 'curated';
}

/**
 * Computes the semantic zoom tier from Sigma camera.ratio with hysteresis
 * to avoid flickering near threshold boundaries.
 */
export function computeZoomTier(
  cameraRatio: number,
  previousTier: SemanticZoomTier = 'overview'
): SemanticZoomTier {
  const ratio = Number.isFinite(cameraRatio) && cameraRatio > 0 ? cameraRatio : 1.0;

  if (previousTier === 'overview') {
    if (ratio < 0.20) return 'cluster';
    if (ratio < 0.50) return 'sector';
    return 'overview';
  }

  if (previousTier === 'sector') {
    if (ratio < 0.20) return 'cluster';
    if (ratio > 0.58) return 'overview';
    return 'sector';
  }

  // previousTier === 'cluster'
  if (ratio > 0.58) return 'overview';
  if (ratio > 0.26) return 'sector';
  return 'cluster';
}

/**
 * Computes bounded logarithmic WebGL radius for collapsed Super-Nodes.
 */
export function computeSuperNodeRadius(kind: 'sector' | 'cluster', noteCount: number): number {
  const safeCount = Math.max(1, noteCount || 1);
  if (kind === 'sector') {
    const raw = 7 + Math.log2(safeCount + 1) * 1.9;
    return Math.max(8, Math.min(22, raw));
  }
  const raw = 5 + Math.log2(safeCount + 1) * 1.35;
  return Math.max(6, Math.min(15, raw));
}

/**
 * Identifies high-signal anchor stars within an expanded Tier 1 sector.
 */
export function isHighSignalSourceStar(node: LodNodeMeta): boolean {
  return Boolean(
    node.isClusterAlpha ||
    node.stellarClass === 'QUASAR' ||
    (node.connections ?? 0) >= 5
  );
}

interface ViewportBox {
  minX: number;
  maxX: number;
  minY: number;
  maxY: number;
}

function projectSuperNodeViewportBounds(
  meta: SuperNodeMeta,
  projectToViewport: (x: number, y: number) => { x: number; y: number } | null
): ViewportBox | null {
  if (
    typeof meta.minX === 'number' &&
    typeof meta.maxX === 'number' &&
    typeof meta.minY === 'number' &&
    typeof meta.maxY === 'number'
  ) {
    const pMin = projectToViewport(meta.minX, meta.minY);
    const pMax = projectToViewport(meta.maxX, meta.maxY);
    if (pMin && pMax) {
      return {
        minX: Math.min(pMin.x, pMax.x),
        maxX: Math.max(pMin.x, pMax.x),
        minY: Math.min(pMin.y, pMax.y),
        maxY: Math.max(pMin.y, pMax.y),
      };
    }
  }
  const pt = projectToViewport(meta.x, meta.y);
  if (!pt) return null;
  return { minX: pt.x, maxX: pt.x, minY: pt.y, maxY: pt.y };
}

function isBoundsIntersectingViewport(
  bounds: ViewportBox | null,
  dims: { width: number; height: number },
  marginFraction: number
): boolean {
  if (!bounds || dims.width <= 0 || dims.height <= 0) return false;
  const mx = dims.width * marginFraction;
  const my = dims.height * marginFraction;
  return (
    bounds.maxX >= -mx &&
    bounds.minX <= dims.width + mx &&
    bounds.maxY >= -my &&
    bounds.minY <= dims.height + my
  );
}

function distanceFromBoundsToViewportCenter(
  bounds: ViewportBox | null,
  dims: { width: number; height: number }
): number {
  if (!bounds) return Infinity;
  const cx = dims.width / 2;
  const cy = dims.height / 2;
  const clampedX = Math.max(bounds.minX, Math.min(bounds.maxX, cx));
  const clampedY = Math.max(bounds.minY, Math.min(bounds.maxY, cy));
  return Math.hypot(cx - clampedX, cy - clampedY);
}

/**
 * Evaluates which sectors and clusters should be unfolded vs. collapsed for the
 * current camera ratio, viewport frustum, and Knowledge Scope mode.
 */
export function computeSemanticLodState(input: ComputeSemanticLodInput): SemanticLodState {
  const {
    scopeMode,
    cameraRatio,
    viewportDims,
    previousState,
    sectors,
    clusters,
    curatedStarCount,
    totalStarCount,
    projectToViewport,
  } = input;

  const tier = computeZoomTier(cameraRatio, previousState?.tier ?? 'overview');

  if (scopeMode === 'all') {
    return {
      scopeMode,
      tier,
      expandedSectors: new Set(sectors.map(s => s.key)),
      expandedClusters: new Set(clusters.map(c => c.key)),
      collapsedSectorNodes: new Set(),
      collapsedClusterNodes: new Set(),
      visibleStarCount: totalStarCount,
      totalStarCount,
      collapsedGroupCount: 0,
    };
  }

  if (scopeMode === 'curated') {
    return {
      scopeMode,
      tier,
      expandedSectors: new Set(),
      expandedClusters: new Set(),
      collapsedSectorNodes: new Set(),
      collapsedClusterNodes: new Set(),
      visibleStarCount: curatedStarCount,
      totalStarCount,
      collapsedGroupCount: 0,
    };
  }

  // Hybrid mode: progressive disclosure for `sources` groups
  const sourceSectors = sectors.filter(s => s.trustScope === 'sources');
  const sourceSectorKeys = new Set(sourceSectors.map(s => s.key));
  const sourceClusters = clusters.filter(c => c.trustScope === 'sources');

  const expandedSectors = new Set<string>();
  const expandedClusters = new Set<string>();
  const collapsedSectorNodes = new Set<string>();
  const collapsedClusterNodes = new Set<string>();

  if (tier === 'overview') {
    let singletonStars = 0;
    for (const sec of sourceSectors) {
      if (sec.noteCount <= 1) {
        expandedSectors.add(sec.key);
        singletonStars += sec.noteCount;
      } else {
        collapsedSectorNodes.add(sec.id);
      }
    }
    for (const cl of sourceClusters) {
      if (!sourceSectorKeys.has(cl.sectorKey)) {
        if (cl.noteCount <= 1) {
          expandedClusters.add(cl.key);
          singletonStars += cl.noteCount;
        } else {
          collapsedClusterNodes.add(cl.id);
        }
      }
    }
    return {
      scopeMode,
      tier,
      expandedSectors,
      expandedClusters,
      collapsedSectorNodes,
      collapsedClusterNodes,
      visibleStarCount: Math.min(totalStarCount, curatedStarCount + singletonStars),
      totalStarCount,
      collapsedGroupCount: collapsedSectorNodes.size + collapsedClusterNodes.size,
    };
  }

  // Tier 1 (sector) and Tier 2 (cluster): select visible sectors intersecting viewport frustum (AABB bounds)
  const candidateSectors: Array<{ meta: SuperNodeMeta; dist: number }> = [];
  let singletonStars = 0;
  for (const sec of sourceSectors) {
    if (sec.noteCount <= 1) {
      expandedSectors.add(sec.key);
      singletonStars += sec.noteCount;
      continue;
    }
    const bounds = projectSuperNodeViewportBounds(sec, projectToViewport);
    const wasExpanded = previousState?.expandedSectors.has(sec.key) ?? false;
    const inside = isBoundsIntersectingViewport(bounds, viewportDims, wasExpanded ? 0.35 : 0.10);
    if (inside) {
      candidateSectors.push({ meta: sec, dist: distanceFromBoundsToViewportCenter(bounds, viewportDims) });
    } else {
      collapsedSectorNodes.add(sec.id);
    }
  }

  candidateSectors.sort((a, b) => a.dist - b.dist);
  candidateSectors.forEach((item, idx) => {
    if (idx < MAX_EXPANDED_SECTORS) {
      expandedSectors.add(item.meta.key);
    } else {
      collapsedSectorNodes.add(item.meta.id);
    }
  });

  // Standalone source clusters without a parent sector anchor
  const standaloneClusters = sourceClusters.filter(cl => !sourceSectorKeys.has(cl.sectorKey));
  const activeSectorClusters = sourceClusters.filter(cl => expandedSectors.has(cl.sectorKey));

  if (tier === 'sector') {
    for (const cl of [...activeSectorClusters, ...standaloneClusters]) {
      if (cl.noteCount <= 1) {
        expandedClusters.add(cl.key);
        singletonStars += cl.noteCount;
      } else {
        collapsedClusterNodes.add(cl.id);
      }
    }
    return {
      scopeMode,
      tier,
      expandedSectors,
      expandedClusters,
      collapsedSectorNodes,
      collapsedClusterNodes,
      visibleStarCount: Math.min(totalStarCount, curatedStarCount + singletonStars),
      totalStarCount,
      collapsedGroupCount: collapsedSectorNodes.size + collapsedClusterNodes.size,
    };
  }

  // Tier 2 (cluster): unfold clusters whose AABB intersects viewport within budget
  const candidateClusters: Array<{ meta: SuperNodeMeta; dist: number }> = [];
  let expandedSourceStars = singletonStars;
  for (const cl of [...activeSectorClusters, ...standaloneClusters]) {
    if (cl.noteCount <= 1) {
      expandedClusters.add(cl.key);
      expandedSourceStars += cl.noteCount;
      continue;
    }
    const bounds = projectSuperNodeViewportBounds(cl, projectToViewport);
    const wasExpanded = previousState?.expandedClusters.has(cl.key) ?? false;
    const inside = isBoundsIntersectingViewport(bounds, viewportDims, wasExpanded ? 0.35 : 0.10);
    if (inside) {
      candidateClusters.push({ meta: cl, dist: distanceFromBoundsToViewportCenter(bounds, viewportDims) });
    } else {
      collapsedClusterNodes.add(cl.id);
    }
  }

  candidateClusters.sort((a, b) => a.dist - b.dist);
  candidateClusters.forEach((item, idx) => {
    if (idx < MAX_EXPANDED_CLUSTERS) {
      expandedClusters.add(item.meta.key);
      expandedSourceStars += item.meta.noteCount;
    } else {
      collapsedClusterNodes.add(item.meta.id);
    }
  });

  return {
    scopeMode,
    tier,
    expandedSectors,
    expandedClusters,
    collapsedSectorNodes,
    collapsedClusterNodes,
    visibleStarCount: Math.min(totalStarCount, curatedStarCount + expandedSourceStars),
    totalStarCount,
    collapsedGroupCount: collapsedSectorNodes.size + collapsedClusterNodes.size,
  };
}

function areSetsEqual(a: Set<string>, b: Set<string>): boolean {
  if (a.size !== b.size) return false;
  for (const item of a) {
    if (!b.has(item)) return false;
  }
  return true;
}

/**
 * Structural equality guard to avoid redundant Sigma refreshes on smooth camera motion.
 */
export function isLodStateEqual(a: SemanticLodState | null, b: SemanticLodState | null): boolean {
  if (a === b) return true;
  if (!a || !b) return false;
  return (
    a.scopeMode === b.scopeMode &&
    a.tier === b.tier &&
    areSetsEqual(a.expandedSectors, b.expandedSectors) &&
    areSetsEqual(a.expandedClusters, b.expandedClusters) &&
    areSetsEqual(a.collapsedSectorNodes, b.collapsedSectorNodes) &&
    areSetsEqual(a.collapsedClusterNodes, b.collapsedClusterNodes)
  );
}

/**
 * O(1) visibility predicate used by nodeReducer, edgeReducer, and TrafficSimulator.
 */
export function isMemoryNodeVisibleInLod(
  meta: LodNodeMeta,
  lod: SemanticLodState | null
): boolean {
  if (!lod || lod.scopeMode === 'all' || meta.trustScope === 'arms') {
    return true;
  }
  if (lod.scopeMode === 'curated') {
    return meta.trustScope !== 'sources';
  }

  // Hybrid mode
  if (meta.trustScope !== 'sources') {
    return true;
  }
  if (lod.tier === 'overview') {
    return lod.expandedSectors.has(meta.sectorKey) || lod.expandedClusters.has(meta.clusterKey);
  }
  if (lod.tier === 'sector') {
    return (
      lod.expandedClusters.has(meta.clusterKey) ||
      (lod.expandedSectors.has(meta.sectorKey) && isHighSignalSourceStar(meta))
    );
  }
  // Tier 2 (cluster)
  return (
    lod.expandedClusters.has(meta.clusterKey) ||
    (lod.expandedSectors.has(meta.sectorKey) && isHighSignalSourceStar(meta))
  );
}

export function loadScopeMode(): GalaxyScopeMode {
  try {
    const raw = localStorage.getItem(STORAGE_KEY_SCOPE_MODE);
    if (raw === 'curated' || raw === 'hybrid' || raw === 'all') {
      return raw;
    }
    return DEFAULT_SCOPE_MODE;
  } catch {
    return DEFAULT_SCOPE_MODE;
  }
}

export function saveScopeMode(mode: GalaxyScopeMode): void {
  try {
    localStorage.setItem(STORAGE_KEY_SCOPE_MODE, mode);
  } catch (e) {
    console.error('Failed to save scope mode to localStorage', e);
  }
}
