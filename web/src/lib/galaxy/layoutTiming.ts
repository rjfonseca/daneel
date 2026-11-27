export interface LayoutSettlePlanInput {
  nodeCount: number;
  cachedCoverageRatio: number;
  reducedMotion: boolean;
  shouldAnimateReset: boolean;
}

export interface LayoutSettlePlan {
  forceAtlasMs: number;
  postStopDelayMs: number;
  resetDurationMs: number;
  readyDelayAfterResetMs: number;
}

const SMALL_VAULT_MAX_NODES = 250;
const LARGE_VAULT_MIN_NODES = 2000;
const SMALL_VAULT_FORCE_ATLAS_MS = 1800;
const LARGE_VAULT_FORCE_ATLAS_MS = 8500;
const FULL_CACHE_FORCE_ATLAS_MS = 400;
const PARTIAL_CACHE_FORCE_ATLAS_MS = 1200;

/**
 * Computes ForceAtlas2 stabilization and post-layout camera settle durations
 * scaled by graph size, layout cache coverage, and motion preferences.
 */
export function computeLayoutSettlePlan(input: LayoutSettlePlanInput): LayoutSettlePlan {
  const { nodeCount, cachedCoverageRatio, reducedMotion, shouldAnimateReset } = input;
  const coverage = Math.max(0, Math.min(1, Number.isFinite(cachedCoverageRatio) ? cachedCoverageRatio : 0));
  const count = Math.max(0, Number.isFinite(nodeCount) ? nodeCount : 0);

  let forceAtlasMs: number;
  if (coverage >= 0.9) {
    forceAtlasMs = FULL_CACHE_FORCE_ATLAS_MS;
  } else if (coverage > 0) {
    forceAtlasMs = PARTIAL_CACHE_FORCE_ATLAS_MS;
  } else if (count <= SMALL_VAULT_MAX_NODES) {
    forceAtlasMs = SMALL_VAULT_FORCE_ATLAS_MS;
  } else if (count >= LARGE_VAULT_MIN_NODES) {
    forceAtlasMs = LARGE_VAULT_FORCE_ATLAS_MS;
  } else {
    const t = (count - SMALL_VAULT_MAX_NODES) / (LARGE_VAULT_MIN_NODES - SMALL_VAULT_MAX_NODES);
    forceAtlasMs = Math.round(
      SMALL_VAULT_FORCE_ATLAS_MS + t * (LARGE_VAULT_FORCE_ATLAS_MS - SMALL_VAULT_FORCE_ATLAS_MS)
    );
  }

  const postStopDelayMs = reducedMotion || !shouldAnimateReset ? 50 : 300;
  const resetDurationMs = shouldAnimateReset ? (reducedMotion ? 1 : 1500) : 0;
  const readyDelayAfterResetMs = shouldAnimateReset ? (reducedMotion ? 50 : 1500) : 50;

  return {
    forceAtlasMs,
    postStopDelayMs,
    resetDurationMs,
    readyDelayAfterResetMs,
  };
}
