/**
 * @file labelLayout.ts
 * Pure canvas label sanitization, safe-HUD bounds clamping, progressive
 * overview nebula gating, and shared AABB overlap predicates.
 */

import type { CalloutBoundingBox, OutwardCalloutGeometry } from '../canvasMath';

export interface SafeHudBounds {
  minX: number;
  maxX: number;
  minY: number;
  maxY: number;
}

const ARMS_BADGE_MAP: Record<string, string> = {
  harvesters: 'HARVESTER',
  skills: 'SKILL',
  routines: 'ROUTINE',
};

/**
 * Removes redundant category prefixes/suffixes (e.g. `Skill: Foo` -> `SKILL · Foo`,
 * `GitLab Harvester` -> `HARVESTER · GitLab`) and truncates cleanly.
 */
export function sanitizeArmsStationLabel(
  rawLabel: string,
  ring: string,
  isOverviewZoom: boolean
): string {
  const badge = ARMS_BADGE_MAP[ring] || 'ARMS';
  let name = (rawLabel || '').replace(/\.(md|markdown)$/i, '').split('/').pop() || rawLabel || '';

  if (ring === 'skills') {
    name = name.replace(/^skill\s*:\s*/i, '');
  } else if (ring === 'harvesters') {
    name = name.replace(/^harvester\s*:\s*/i, '').replace(/\s+harvester$/i, '');
  } else if (ring === 'routines') {
    name = name.replace(/^routine\s*:\s*/i, '');
  }
  name = name.trim() || rawLabel;

  const maxLen = isOverviewZoom ? 16 : 26;
  const display = name.length > maxLen ? `${name.slice(0, maxLen - 2)}..` : name;
  return `${badge} · ${display}`;
}

/**
 * Computes the drawable canvas viewport rectangle between TopCosmicBar and BottomTemporalHotbar.
 */
export function getSafeHudBounds(
  viewportWidth: number,
  viewportHeight: number,
  topHudBottomPx: number,
  bottomHudTopPx: number
): SafeHudBounds {
  const minX = 14;
  const maxX = Math.max(minX + 40, viewportWidth - 14);
  const minY = Math.max(24, topHudBottomPx + 12);
  const maxY = Math.max(minY + 40, Math.min(viewportHeight - 14, bottomHudTopPx - 12));
  return { minX, maxX, minY, maxY };
}

/**
 * Clamps an outward station callout geometry and bounding box inside SafeHudBounds
 * so perimeter labels never clip behind top/bottom HUD chrome or screen edges.
 */
export function clampOutwardCalloutToSafeBounds(
  geom: OutwardCalloutGeometry,
  box: CalloutBoundingBox,
  safe: SafeHudBounds
): { geom: OutwardCalloutGeometry; box: CalloutBoundingBox } {
  let dx = 0;
  let dy = 0;

  if (box.minX < safe.minX) {
    dx = safe.minX - box.minX;
  } else if (box.maxX > safe.maxX) {
    dx = safe.maxX - box.maxX;
  }

  if (box.minY < safe.minY) {
    dy = safe.minY - box.minY;
  } else if (box.maxY > safe.maxY) {
    dy = safe.maxY - box.maxY;
  }

  if (dx === 0 && dy === 0) {
    return { geom, box };
  }

  return {
    geom: {
      ...geom,
      midX: geom.midX + dx,
      midY: geom.midY + dy,
      endX: geom.endX + dx,
      endY: geom.endY + dy,
      textAnchorX: geom.textAnchorX + dx,
      textAnchorY: geom.textAnchorY + dy,
    },
    box: {
      ...box,
      minX: box.minX + dx,
      maxX: box.maxX + dx,
      minY: box.minY + dy,
      maxY: box.maxY + dy,
    },
  };
}

/**
 * Progressive overview budget for root `notes/*` nebula callouts:
 * keeps overview zoom uncluttered (~4 pillars max) while revealing all pillars on zoom-in or focus.
 */
export function shouldShowOverviewRootNebula(
  renderedRootCount: number,
  maxNebulas: number,
  cameraRatio: number,
  isFocused: boolean
): boolean {
  if (isFocused || cameraRatio < 0.55) {
    return true;
  }
  const overviewRootBudget = Math.min(4, Math.max(2, maxNebulas));
  return renderedRootCount < overviewRootBudget;
}

/**
 * Shared AABB overlap predicate with configurable margin.
 */
export function doesRectOverlapBoxes(
  rect: { minX: number; maxX: number; minY: number; maxY: number },
  boxes: CalloutBoundingBox[],
  margin = 4
): boolean {
  for (const box of boxes) {
    const separated =
      rect.maxX + margin < box.minX ||
      rect.minX - margin > box.maxX ||
      rect.maxY + margin < box.minY ||
      rect.minY - margin > box.maxY;
    if (!separated) return true;
  }
  return false;
}
