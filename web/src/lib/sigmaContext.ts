import { DEFAULT_MAX_NEBULAS } from './nebulas';
import type { SigmaContextState } from '../types/galaxy';

export const sigmaContext: SigmaContextState = {
  sigma: null,
  frameLabelsCount: 0,
  frameCalloutBoxes: [],
  renderedNebulaCenters: [],
  maxNebulas: DEFAULT_MAX_NEBULAS,
  focusedSector: null,
  focusedCluster: null,
  edgeMode: 'focus',
  hoveredNodeId: null,
  selectedNodeId: null,
  constellation: null,
  showRings: true,
  isCosmicMode: true,
  showSectorLabels: true,
  showNebulaLabels: true,
  galaxyCenter: { x: 0, y: 0 },
};
