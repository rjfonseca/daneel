import type Sigma from 'sigma';
import type { ConstellationResult } from '../lib/constellationFilter';
import type { EdgeVisibilityMode } from '../lib/edgeFiltering';
import type { CalloutBoundingBox } from '../lib/canvasMath';

export interface GalaxyNode {
  id: string;
  title?: string;
  category?: string;
  originalCategory?: string;
  connections?: number;
  modTime?: number | bigint;
  ring?: string;
  status?: string;
  icon?: string;
  description?: string;
  size?: number;
  x?: number;
  y?: number;
  color?: string;
  stellarClass?: string;
  stellarBadge?: string;
  stellarLabel?: string;
  stellarDesc?: string;
  spectralClass?: string;
  relPath?: string;
  domain?: string;
  sector?: string;
  group?: string;
  repository?: string;
  isArms?: boolean;
  isCat?: boolean;
}

export interface GalaxyLink {
  source: string;
  target: string;
  weight?: number;
  size?: number;
  color?: string;
  hidden?: boolean;
  originalHidden?: boolean;
  isCrossCluster?: boolean;
  isOrbital?: boolean;
  ring?: string;
  sourceCategory?: string;
  targetCategory?: string;
  sourceDegree?: number;
  targetDegree?: number;
}

export interface HoverTooltipNode {
  id: string;
  title?: string;
  originalCategory?: string;
  color?: string;
  stellarClass?: string;
  stellarBadge?: string;
  stellarLabel?: string;
  stellarDesc?: string;
  connections?: number;
  ring?: string;
  status?: string;
  icon?: string;
}

export interface HoverTooltipState {
  node: HoverTooltipNode;
  pos: { x: number; y: number };
}

export interface SigmaContextState {
  sigma: Sigma | null;
  frameLabelsCount: number;
  frameCalloutBoxes: CalloutBoundingBox[];
  renderedNebulaCenters: Array<{ x: number; y: number }>;
  maxNebulas: number;
  focusedSector: string | null;
  focusedCluster?: string | null;
  edgeMode: EdgeVisibilityMode;
  hoveredNodeId: string | null;
  selectedNodeId: string | null;
  constellation: ConstellationResult | null;
  showRings: boolean;
  isCosmicMode?: boolean;
  showSectorLabels?: boolean;
  showNebulaLabels?: boolean;
  galaxyCenter: { x: number; y: number };
}

export interface NeighborRef {
  id: string;
  title: string;
  color?: string;
  stellarBadge?: string;
}

export interface GalaxyApi {
  resetCamera: () => void;
  zoomToCategoryCluster?: (catId: string) => void;
  zoomToNodes?: (nodeIds: string[]) => void;
}

export interface GalaxyControllerProps {
  safeNodes: GalaxyNode[];
  safeLinks: GalaxyLink[];
  catCounts: Record<string, number>;
  onFocusNode: (nodeId: string) => void;
  onReady: () => void;
  targetNodeId: string | null;
  focusedSector: string | null;
  setFocusedSector: (sector: string | null) => void;
  focusedCluster?: string | null;
  setFocusedCluster?: (cluster: string | null) => void;
  onHover: (node: HoverTooltipNode | null, pos: { x: number; y: number } | null) => void;
  maxNebulas: number;
  onMountController?: (api: GalaxyApi) => void;
  edgeMode: EdgeVisibilityMode;
  isCosmicMode: boolean;
  showRings: boolean;
  constellation: ConstellationResult | null;
  isReady: boolean;
  showSectorLabels?: boolean;
  showNebulaLabels?: boolean;
  drawerWidth?: number;
  reducedMotion?: boolean;
}

export interface GalaxyDataState {
  nodes: GalaxyNode[];
  links: GalaxyLink[];
}

export interface SearchResultItem {
  id?: string;
  relPath?: string;
  title: string;
  snippet: string;
  isCat?: boolean;
  isArms?: boolean;
  category?: string;
  ring?: string;
  status?: string;
  icon?: string;
}
