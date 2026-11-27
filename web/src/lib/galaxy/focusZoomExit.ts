/**
 * @file focusZoomExit.ts
 * Pure hierarchical zoom-out state machine for releasing Constellations,
 * Clusters, and Sectors in place without resetting the user's camera position.
 */

export type FocusExitAction =
  | { type: 'none' }
  | { type: 'exit-constellation' }
  | { type: 'exit-cluster' }
  | { type: 'exit-all' };

export interface FocusZoomExitInput {
  focusedSector: string | null;
  focusedCluster: string | null;
  hasActiveConstellation: boolean;
  isProgrammaticFocusFlight: boolean;
  currentRatio: number;
  previousRatio: number;
  sectorAnchorRatio: number | null;
  clusterAnchorRatio: number | null;
  constellationAnchorRatio?: number | null;
}

/**
 * Computes the adaptive camera.ratio threshold at which a focused constellation,
 * cluster, or sector should auto-release focus on zoom-out.
 */
export function calculateFocusExitRatio(
  focusRatio: number,
  kind: 'sector' | 'cluster' | 'constellation'
): number {
  const safeRatio = Number.isFinite(focusRatio) && focusRatio > 0 ? focusRatio : 0.25;
  if (kind === 'constellation') {
    return Math.min(
      Math.max(safeRatio * 2.2, 0.24),
      Math.max(0.42, safeRatio * 1.4)
    );
  }
  const minExit = kind === 'cluster' ? 0.16 : 0.18;
  const maxCap = kind === 'cluster' ? 0.58 : 0.68;
  return Math.min(
    Math.max(safeRatio * 1.65, minExit),
    Math.max(maxCap, safeRatio * 1.25)
  );
}

/**
 * Evaluates whether a zoom-out camera update should step up hierarchically:
 *   Constellation -> Cluster -> Sector -> Full Galaxy
 * without interfering with entry flights.
 */
export function evaluateFocusZoomExit(input: FocusZoomExitInput): FocusExitAction {
  const {
    focusedSector,
    focusedCluster,
    hasActiveConstellation,
    isProgrammaticFocusFlight,
    currentRatio,
    previousRatio,
    sectorAnchorRatio,
    clusterAnchorRatio,
    constellationAnchorRatio,
  } = input;

  if (!focusedSector && !focusedCluster && !hasActiveConstellation) {
    return { type: 'none' };
  }
  if (isProgrammaticFocusFlight) {
    return { type: 'none' };
  }
  if (!Number.isFinite(currentRatio) || currentRatio <= previousRatio + 1e-4) {
    return { type: 'none' };
  }

  if (hasActiveConstellation) {
    const constellationExitRatio = calculateFocusExitRatio(
      constellationAnchorRatio ?? 0.08,
      'constellation'
    );
    if (currentRatio >= constellationExitRatio) {
      return { type: 'exit-constellation' };
    }
    return { type: 'none' };
  }

  const baseSectorExitRatio = focusedSector
    ? calculateFocusExitRatio(sectorAnchorRatio ?? 0.30, 'sector')
    : Infinity;
  const clusterExitRatio = focusedCluster
    ? calculateFocusExitRatio(clusterAnchorRatio ?? 0.18, 'cluster')
    : Infinity;
  const sectorExitRatio = focusedSector && focusedCluster
    ? Math.max(baseSectorExitRatio, Math.min(0.78, clusterExitRatio * 1.5))
    : baseSectorExitRatio;

  if (focusedSector && currentRatio >= sectorExitRatio) {
    return { type: 'exit-all' };
  }

  if (focusedCluster && !focusedSector && currentRatio >= clusterExitRatio) {
    return { type: 'exit-all' };
  }

  if (focusedCluster && focusedSector && currentRatio >= clusterExitRatio) {
    return { type: 'exit-cluster' };
  }

  return { type: 'none' };
}
