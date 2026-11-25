import type { EdgeVisibilityMode } from './edgeFiltering';
import type { GalaxyScopeMode } from './semanticZoomLod';

export interface GalaxyViewState {
  edgeMode: EdgeVisibilityMode;
  scopeMode: GalaxyScopeMode;
  showSectorLabels: boolean;
  showNebulaLabels: boolean;
  isCosmicMode: boolean;
  showRings: boolean;
  reducedMotion: boolean;
}

export type GalaxyViewPreset = 'explore' | 'read' | 'present';

export const DEFAULT_GALAXY_VIEW: GalaxyViewState = {
  edgeMode: 'focus',
  scopeMode: 'hybrid',
  showSectorLabels: true,
  showNebulaLabels: true,
  isCosmicMode: true,
  showRings: true,
  reducedMotion: false,
};

const PRESETS: Record<GalaxyViewPreset, Omit<GalaxyViewState, 'reducedMotion'>> = {
  explore: { edgeMode: 'focus', scopeMode: 'hybrid', showSectorLabels: true, showNebulaLabels: true, isCosmicMode: true, showRings: true },
  read: { edgeMode: 'focus', scopeMode: 'curated', showSectorLabels: false, showNebulaLabels: false, isCosmicMode: false, showRings: false },
  present: { edgeMode: 'bridges', scopeMode: 'hybrid', showSectorLabels: true, showNebulaLabels: true, isCosmicMode: true, showRings: true },
};

export function applyViewPreset(view: GalaxyViewState, preset: GalaxyViewPreset): GalaxyViewState {
  return { ...view, ...PRESETS[preset] };
}

export function activeViewPreset(view: GalaxyViewState): GalaxyViewPreset | null {
  return (Object.keys(PRESETS) as GalaxyViewPreset[]).find((preset) =>
    (Object.keys(PRESETS[preset]) as Array<keyof typeof PRESETS.explore>)
      .every((key) => view[key] === PRESETS[preset][key])) ?? null;
}
