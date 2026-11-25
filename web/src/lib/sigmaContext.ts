import { DEFAULT_MAX_NEBULAS } from './nebulas';
import { DEFAULT_SCOPE_MODE } from './semanticZoomLod';
import type { SigmaContextState } from '../types/galaxy';

export const sigmaContext: SigmaContextState = {
  sigma: null,
  frameLabelsCount: 0,
  frameRootLabelsCount: 0,
  frameConstellationNeighborLabelsCount: 0,
  frameCalloutBoxes: [],
  renderedNebulaCenters: [],
  maxNebulas: DEFAULT_MAX_NEBULAS,
  focusedSector: null,
  focusedCluster: null,
  edgeMode: 'focus',
  hoveredNodeId: null,
  hoveredNeighborIds: new Set<string>(),
  selectedNodeId: null,
  constellation: null,
  showRings: true,
  isCosmicMode: true,
  showSectorLabels: true,
  showNebulaLabels: true,
  galaxyCenter: { x: 0, y: 0 },
  scopeMode: DEFAULT_SCOPE_MODE,
  lodState: null,
  superNodeRegistry: new Map(),
  superEdgeIndex: null,
};
