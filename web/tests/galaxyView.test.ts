import { describe, expect, it } from 'vitest';
import { applyViewPreset, DEFAULT_GALAXY_VIEW, type GalaxyViewState } from '../src/lib/galaxyView';

describe('galaxy view presets', () => {
  it('uses the existing galaxy settings as the Explore default', () => {
    expect(DEFAULT_GALAXY_VIEW).toEqual({
      edgeMode: 'focus',
      showSectorLabels: true,
      showNebulaLabels: true,
      isCosmicMode: true,
      showRings: true,
      reducedMotion: false,
    });
  });

  it('sets a quiet reading view without changing a motion preference', () => {
    const current: GalaxyViewState = { ...DEFAULT_GALAXY_VIEW, reducedMotion: true };
    expect(applyViewPreset(current, 'read')).toEqual({
      edgeMode: 'focus',
      showSectorLabels: false,
      showNebulaLabels: false,
      isCosmicMode: false,
      showRings: false,
      reducedMotion: true,
    });
  });

  it('restores a presentation view with navigational context', () => {
    expect(applyViewPreset(applyViewPreset(DEFAULT_GALAXY_VIEW, 'read'), 'present')).toMatchObject({
      edgeMode: 'bridges',
      showSectorLabels: true,
      showNebulaLabels: true,
      isCosmicMode: true,
      showRings: true,
    });
  });
});
