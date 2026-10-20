export function calculateLabelBounds(nodeX: number, nodeY: number, size: number, text: string, ctx: { measureText(text: string): Pick<TextMetrics, 'width'> }) {
  const textWidth = ctx.measureText(text).width;
  const paddingX = 8;
  const paddingY = 6;
  const fontSize = 12; // Assuming fixed font size for labels
  
  // The box should be placed to the right of the node
  const boxX = Math.round(nodeX + size + 5);
  const boxY = Math.round(nodeY - fontSize / 2 - paddingY);
  
  const width = Math.round(textWidth + paddingX * 2);
  const height = Math.round(fontSize + paddingY * 2);
  
  // Text should be strictly inside the box (left-aligned)
  const textX = boxX + paddingX;
  const textY = Math.round(nodeY + fontSize / 3);
  
  return { x: boxX, y: boxY, width, height, textX, textY };
}

export interface NebulaCalloutGeometry {
  isLeft: boolean;
  dir: 1 | -1;
  midX: number;
  midY: number;
  endX: number;
  endY: number;
  textAnchorX: number;
  textAnchorY: number;
  textAlign: CanvasTextAlign;
  textBaseline: CanvasTextBaseline;
  textStartX: number;
  textEndX: number;
  shelfMinX: number;
  shelfMaxX: number;
}

export interface CalculateNebulaCalloutOptions {
  diagDist?: number;
  padding?: number;
  textOffsetBottom?: number;
}

export function calculateNebulaCalloutGeometry(
  x: number,
  y: number,
  textWidth: number,
  isLeft: boolean,
  options?: CalculateNebulaCalloutOptions
): NebulaCalloutGeometry {
  const diagDist = options?.diagDist ?? 35;
  const padding = options?.padding ?? 16;
  const textOffsetBottom = options?.textOffsetBottom ?? 4;

  const dir: 1 | -1 = isLeft ? -1 : 1;
  const horizDist = textWidth + padding;

  const midX = x + (dir * diagDist);
  const midY = y - diagDist;
  const endX = midX + (dir * horizDist);
  const endY = midY;

  const shelfMinX = Math.min(midX, endX);
  const shelfMaxX = Math.max(midX, endX);

  const textAnchorX = (midX + endX) / 2;
  const textAnchorY = endY - textOffsetBottom;
  const textAlign: CanvasTextAlign = 'center';
  const textBaseline: CanvasTextBaseline = 'bottom';

  const textStartX = textAnchorX - textWidth / 2;
  const textEndX = textAnchorX + textWidth / 2;

  return {
    isLeft,
    dir,
    midX,
    midY,
    endX,
    endY,
    textAnchorX,
    textAnchorY,
    textAlign,
    textBaseline,
    textStartX,
    textEndX,
    shelfMinX,
    shelfMaxX,
  };
}

export interface CalloutBoundingBox {
  minX: number;
  maxX: number;
  minY: number;
  maxY: number;
  nodeId?: string;
  category?: string;
  isStation?: boolean;
  isCategory?: boolean;
  isSector?: boolean;
}

/**
 * Returns true if a canvas coordinate (px, py) lies within a callout bounding box.
 */
export function isPointInCalloutBox(
  px: number,
  py: number,
  box: CalloutBoundingBox,
  tolerance = 6
): boolean {
  return (
    px >= box.minX - tolerance &&
    px <= box.maxX + tolerance &&
    py >= box.minY - tolerance &&
    py <= box.maxY + tolerance
  );
}

/**
 * Returns true if two bounding boxes overlap, accounting for an optional safety margin.
 */
export function doCalloutBoxesOverlap(a: CalloutBoundingBox, b: CalloutBoundingBox, margin = 6): boolean {
  return !(
    a.maxX + margin < b.minX ||
    a.minX - margin > b.maxX ||
    a.maxY + margin < b.minY ||
    a.minY - margin > b.maxY
  );
}

export function drawPill(
  ctx: CanvasRenderingContext2D,
  px: number,
  py: number,
  pw: number,
  ph: number,
  r: number
): void {
  if (typeof ctx.roundRect === 'function') {
    ctx.roundRect(px, py, pw, ph, r);
  } else {
    ctx.rect(px, py, pw, ph);
  }
}

/**
 * Derives the Axis-Aligned Bounding Box (AABB) occupied by a cybernetic Sector HUD card.
 */
export function getSectorCalloutBoundingBox(
  geom: NebulaCalloutGeometry,
  cardH = 36
): CalloutBoundingBox {
  const minX = Math.min(geom.shelfMinX, geom.textStartX) - 4;
  const maxX = Math.max(geom.shelfMaxX, geom.textEndX) + 4;
  const minY = geom.midY - cardH - 4;
  const maxY = geom.midY + 2;

  return { minX, maxX, minY, maxY };
}

/**
 * Derives the Axis-Aligned Bounding Box (AABB) occupied by the callout shelf and text.
 */
export function getCalloutBoundingBox(
  geom: NebulaCalloutGeometry,
  fontSize = 14
): CalloutBoundingBox {
  const minX = Math.min(geom.shelfMinX, geom.textStartX);
  const maxX = Math.max(geom.shelfMaxX, geom.textEndX);
  const minY = Math.min(geom.endY, geom.textAnchorY - fontSize);
  const maxY = Math.max(geom.endY, geom.textAnchorY);

  return { minX, maxX, minY, maxY };
}

/**
 * Calculates nebula callout geometry avoiding collisions with previously placed callouts in the current frame.
 * Tries increasing vertical elevation (e.g. diagDist 35, 65, 95) or inverted direction if needed.
 */
export function calculateNonCollidingNebulaCallout(
  x: number,
  y: number,
  textWidth: number,
  isLeft: boolean,
  existingBoxes: CalloutBoundingBox[] = [],
  options?: CalculateNebulaCalloutOptions,
  meta?: { category?: string; nodeId?: string; isCategory?: boolean; isSector?: boolean }
): { geom: NebulaCalloutGeometry; box: CalloutBoundingBox } | null {
  const baseDiagDist = options?.diagDist ?? 35;
  const elevationCandidates = [
    baseDiagDist,
    baseDiagDist + 30, // 65
    baseDiagDist + 60, // 95
    baseDiagDist + 90, // 125
  ];

  // 1. Try standard direction with ascending elevations
  for (const diagDist of elevationCandidates) {
    const candidateGeom = calculateNebulaCalloutGeometry(x, y, textWidth, isLeft, {
      ...options,
      diagDist,
    });
    const rawBox = meta?.isSector
      ? getSectorCalloutBoundingBox(candidateGeom)
      : getCalloutBoundingBox(candidateGeom);
    const candidateBox: CalloutBoundingBox = {
      ...rawBox,
      category: meta?.category,
      nodeId: meta?.nodeId,
      isCategory: meta?.isCategory ?? true,
      isSector: meta?.isSector,
    };

    const hasCollision = existingBoxes.some(box => doCalloutBoxesOverlap(candidateBox, box, 8));
    if (!hasCollision) {
      return { geom: candidateGeom, box: candidateBox };
    }
  }

  // 2. Try inverted direction with ascending elevations
  for (const diagDist of elevationCandidates) {
    const invertedGeom = calculateNebulaCalloutGeometry(x, y, textWidth, !isLeft, {
      ...options,
      diagDist,
    });
    const rawInvertedBox = meta?.isSector
      ? getSectorCalloutBoundingBox(invertedGeom)
      : getCalloutBoundingBox(invertedGeom);
    const invertedBox: CalloutBoundingBox = {
      ...rawInvertedBox,
      category: meta?.category,
      nodeId: meta?.nodeId,
      isCategory: meta?.isCategory ?? true,
      isSector: meta?.isSector,
    };
    if (!existingBoxes.some(box => doCalloutBoxesOverlap(invertedBox, box, 8))) {
      return { geom: invertedGeom, box: invertedBox };
    }
  }

  // If all elevations and orientations collide with existing boxes, return null
  // to avoid visual clutter of overlapping labels on the screen.
  return null;
}

export interface BaseCalloutGeometry {
  midX: number;
  midY: number;
  endX: number;
  endY: number;
  textAnchorX: number;
  textAnchorY: number;
  textAlign: CanvasTextAlign;
  textBaseline: CanvasTextBaseline;
}

export interface OutwardCalloutGeometry extends BaseCalloutGeometry {
  dir: 1 | -1;
  vDir: 1 | -1;
}

export interface NeonCalloutOptions {
  lineWidth?: number;
  glowBlur?: number;
  drawTerminalPoint?: boolean;
}

/**
 * Renders a neon cybernetic callout leader line and glowing text.
 * Provides a unified visual identity for category nebulas and satellite stations.
 */
export function renderNeonCallout(
  ctx: CanvasRenderingContext2D,
  origin: { x: number; y: number },
  geom: BaseCalloutGeometry,
  text: string,
  neonColor: string,
  options?: NeonCalloutOptions
): void {
  const lineWidth = options?.lineWidth ?? 1.5;
  const glowBlur = options?.glowBlur ?? 8;

  ctx.save();

  // 1. Leader line: origin -> diagonal mid -> horizontal shelf end
  ctx.beginPath();
  ctx.moveTo(origin.x, origin.y);
  ctx.lineTo(geom.midX, geom.midY);
  ctx.lineTo(geom.endX, geom.endY);

  ctx.strokeStyle = neonColor;
  ctx.lineWidth = lineWidth;
  ctx.shadowColor = neonColor;
  ctx.shadowBlur = glowBlur;
  ctx.stroke();

  // 2. Terminal anchor point at elbow joint
  if (options?.drawTerminalPoint) {
    ctx.beginPath();
    ctx.arc(geom.midX, geom.midY, 2, 0, Math.PI * 2);
    ctx.fillStyle = neonColor;
    ctx.shadowBlur = glowBlur * 1.5;
    ctx.fill();
  }

  // 3. Glowing neon text along shelf
  ctx.shadowBlur = glowBlur * 1.8;
  ctx.fillStyle = neonColor;
  ctx.textBaseline = geom.textBaseline;
  ctx.textAlign = geom.textAlign;
  ctx.fillText(text, geom.textAnchorX, geom.textAnchorY);

  ctx.restore();
}

export interface CosmicCardCalloutOptions {
  subHeader?: string;
  telemetry?: string;
  lineWidth?: number;
  glowBlur?: number;
  repoCount?: number;
  noteCount?: number;
}

export type SectorCalloutOptions = CosmicCardCalloutOptions;

/**
 * Draws a subtle cybernetic beacon reticle at the sector anchor origin.
 */
export function drawSectorBeacon(
  ctx: CanvasRenderingContext2D,
  origin: { x: number; y: number },
  neonColor: string
): void {
  ctx.save();
  const { x, y } = origin;

  // Outer faint radar circle
  ctx.beginPath();
  ctx.arc(x, y, 9, 0, Math.PI * 2);
  ctx.strokeStyle = neonColor;
  ctx.lineWidth = 1;
  ctx.globalAlpha = 0.35;
  ctx.shadowColor = neonColor;
  ctx.shadowBlur = 6;
  ctx.stroke();

  // Diamond reticle
  ctx.beginPath();
  ctx.moveTo(x, y - 5);
  ctx.lineTo(x + 5, y);
  ctx.lineTo(x, y + 5);
  ctx.lineTo(x - 5, y);
  ctx.closePath();
  ctx.fillStyle = neonColor;
  ctx.globalAlpha = 0.85;
  ctx.shadowBlur = 8;
  ctx.fill();

  // Center core pinprick
  ctx.beginPath();
  ctx.arc(x, y, 1.5, 0, Math.PI * 2);
  ctx.fillStyle = '#ffffff';
  ctx.globalAlpha = 1.0;
  ctx.fill();

  ctx.restore();
}

/**
 * Renders a unified cybernetic HUD glass badge for Sectors and Nebulas.
 */
export function renderCosmicCardCallout(
  ctx: CanvasRenderingContext2D,
  origin: { x: number; y: number },
  geom: BaseCalloutGeometry & { shelfMinX: number; shelfMaxX: number },
  title: string,
  neonColor: string,
  options?: CosmicCardCalloutOptions
): void {
  const lineWidth = options?.lineWidth ?? 1.5;
  const glowBlur = options?.glowBlur ?? 8;
  const subHeader = options?.subHeader ?? '◈ SECTOR';

  let telemetry = options?.telemetry ?? '';
  if (!telemetry) {
    const repoCount = options?.repoCount ?? 0;
    const noteCount = options?.noteCount ?? 0;
    if (subHeader.includes('SECTOR')) {
      telemetry = repoCount > 0
        ? (noteCount > 0 ? `${repoCount} nebulas · ${noteCount} stars` : `${repoCount} nebulas`)
        : (noteCount > 0 ? `${noteCount} stars` : '');
    } else {
      telemetry = noteCount > 0 ? `${noteCount} stars` : '';
    }
  }

  const cardW = geom.shelfMaxX - geom.shelfMinX;
  const cardH = 34;
  const cardX = geom.shelfMinX;
  const cardY = geom.midY - cardH - 2;

  ctx.save();

  // 1. Leader Line from origin to card shelf
  ctx.beginPath();
  ctx.moveTo(origin.x, origin.y);
  ctx.lineTo(geom.midX, geom.midY);
  ctx.lineTo(geom.endX, geom.endY);
  ctx.strokeStyle = neonColor;
  ctx.lineWidth = lineWidth;
  ctx.shadowColor = neonColor;
  ctx.shadowBlur = glowBlur;
  ctx.stroke();

  // 2. Terminal anchor point at elbow joint
  ctx.beginPath();
  ctx.arc(geom.midX, geom.midY, 2.5, 0, Math.PI * 2);
  ctx.fillStyle = neonColor;
  ctx.shadowBlur = glowBlur * 1.5;
  ctx.fill();

  // 3. Cybernetic HUD Glass Card Background
  ctx.fillStyle = 'rgba(10, 15, 30, 0.92)';
  ctx.beginPath();
  drawPill(ctx, cardX, cardY, cardW, cardH, 4);
  ctx.fill();

  // Card Border
  ctx.strokeStyle = neonColor;
  ctx.lineWidth = 1.2;
  ctx.shadowColor = neonColor;
  ctx.shadowBlur = glowBlur;
  ctx.stroke();

  // Corner Accents (Cybernetic brackets at corners)
  const tick = 4;
  ctx.strokeStyle = '#ffffff';
  ctx.lineWidth = 1.5;
  ctx.shadowBlur = 4;
  ctx.shadowColor = '#ffffff';

  // Top-left corner tick
  ctx.beginPath();
  ctx.moveTo(cardX, cardY + tick);
  ctx.lineTo(cardX, cardY);
  ctx.lineTo(cardX + tick, cardY);
  ctx.stroke();

  // Top-right corner tick
  ctx.beginPath();
  ctx.moveTo(cardX + cardW - tick, cardY);
  ctx.lineTo(cardX + cardW, cardY);
  ctx.lineTo(cardX + cardW, cardY + tick);
  ctx.stroke();

  // Bottom-left corner tick
  ctx.beginPath();
  ctx.moveTo(cardX, cardY + cardH - tick);
  ctx.lineTo(cardX, cardY + cardH);
  ctx.lineTo(cardX + tick, cardY + cardH);
  ctx.stroke();

  // Bottom-right corner tick
  ctx.beginPath();
  ctx.moveTo(cardX + cardW - tick, cardY + cardH);
  ctx.lineTo(cardX + cardW, cardY + cardH);
  ctx.lineTo(cardX + cardW, cardY + cardH - tick);
  ctx.stroke();

  // 4. Text Contents
  ctx.shadowBlur = 0; // reset blur for crisp text

  // Top Sub-header: [◈ SECTOR] or [◈ NEBULA] on left
  ctx.font = 'bold 8px monospace';
  ctx.fillStyle = neonColor;
  ctx.textAlign = 'left';
  ctx.textBaseline = 'top';
  ctx.fillText(subHeader, cardX + 8, cardY + 5);

  // Telemetry on right: N nebulas · M stars
  if (telemetry) {
    ctx.font = '8px monospace';
    ctx.fillStyle = 'rgba(148, 163, 184, 0.85)';
    ctx.textAlign = 'right';
    ctx.fillText(telemetry, cardX + cardW - 8, cardY + 5);
  }

  // Primary Title (Sector or Nebula Name in bold)
  ctx.font = 'bold 13px monospace';
  ctx.fillStyle = '#ffffff';
  ctx.textAlign = 'left';
  ctx.textBaseline = 'bottom';
  ctx.shadowColor = neonColor;
  ctx.shadowBlur = 6;
  ctx.fillText(title, cardX + 8, cardY + cardH - 4);

  ctx.restore();
}

/**
 * Re-export renderSectorCallout as alias to renderCosmicCardCallout
 */
export const renderSectorCallout = renderCosmicCardCallout;

/**
 * Calculates a callout leader line that ALWAYS points radially OUTWARD from the galaxy center (centerX, centerY).
 * This ensures satellite labels never obstruct internal galaxy stars.
 */
export function calculateOutwardCalloutGeometry(
  x: number,
  y: number,
  centerX: number,
  centerY: number,
  textWidth: number,
  options?: { diagDist?: number; padding?: number; textOffsetBottom?: number }
): OutwardCalloutGeometry {
  const diagDist = options?.diagDist ?? 28;
  const padding = options?.padding ?? 16;
  const textOffset = options?.textOffsetBottom ?? 4;

  const dx = x - centerX;
  const dy = y - centerY;

  const dir: 1 | -1 = dx >= 0 ? 1 : -1;
  const vDir: 1 | -1 = dy >= 0 ? 1 : -1;

  const horizDist = textWidth + padding;

  const midX = x + dir * diagDist;
  const midY = y + vDir * diagDist;
  const endX = midX + dir * horizDist;
  const endY = midY;

  const textAnchorX = (midX + endX) / 2;
  const textAnchorY = endY - textOffset;
  const textAlign: CanvasTextAlign = 'center';
  const textBaseline: CanvasTextBaseline = 'bottom';

  return {
    dir,
    vDir,
    midX,
    midY,
    endX,
    endY,
    textAnchorX,
    textAnchorY,
    textAlign,
    textBaseline,
  };
}

export function getOutwardCalloutBoundingBox(
  geom: OutwardCalloutGeometry,
  textWidth: number,
  fontSize = 11
): CalloutBoundingBox {
  const shelfMinX = Math.min(geom.midX, geom.endX);
  const shelfMaxX = Math.max(geom.midX, geom.endX);
  const textStartX = geom.textAnchorX - textWidth / 2;
  const textEndX = geom.textAnchorX + textWidth / 2;

  const minX = Math.min(shelfMinX, textStartX);
  const maxX = Math.max(shelfMaxX, textEndX);
  const minY = geom.textAnchorY - fontSize;
  const maxY = geom.endY + 2;

  return { minX, maxX, minY, maxY };
}

/**
 * Calculates outward callout geometry avoiding collisions with other callout boxes in the current frame.
 * Adjusts radial extension distance (e.g. diagDist 26 -> 50 -> 74) if collision is detected.
 */
export function calculateNonCollidingOutwardCallout(
  x: number,
  y: number,
  centerX: number,
  centerY: number,
  textWidth: number,
  existingBoxes: CalloutBoundingBox[] = [],
  options?: { diagDist?: number; padding?: number; textOffsetBottom?: number },
  meta?: { nodeId?: string; isStation?: boolean }
): { geom: OutwardCalloutGeometry; box: CalloutBoundingBox } {
  const baseDist = options?.diagDist ?? 26;
  const candidateDists = [baseDist, baseDist + 24, baseDist + 48];

  let bestGeom: OutwardCalloutGeometry | null = null;
  let bestBox: CalloutBoundingBox | null = null;

  for (const dist of candidateDists) {
    const candidateGeom = calculateOutwardCalloutGeometry(x, y, centerX, centerY, textWidth, {
      ...options,
      diagDist: dist,
    });
    const rawBox = getOutwardCalloutBoundingBox(candidateGeom, textWidth);
    const candidateBox: CalloutBoundingBox = {
      ...rawBox,
      nodeId: meta?.nodeId,
      isStation: meta?.isStation ?? true,
    };

    const hasCollision = existingBoxes.some(box => doCalloutBoxesOverlap(candidateBox, box, 4));
    if (!hasCollision) {
      return { geom: candidateGeom, box: candidateBox };
    }

    if (!bestGeom) {
      bestGeom = candidateGeom;
      bestBox = candidateBox;
    }
  }

  return { geom: bestGeom!, box: bestBox! };
}

