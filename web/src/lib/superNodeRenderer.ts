import type { EdgeVisibilityMode } from './edgeFiltering';
import type { ActiveSuperEdge } from './superEdges';
import { computeSuperNodeRadius, type SuperNodeMeta } from './semanticZoomLod';

export function formatCompactCount(count: number): string {
  const safe = Math.max(0, Math.round(count));
  if (safe < 1000) return String(safe);
  if (safe < 10000) return `${(safe / 1000).toFixed(1)}k`;
  return `${Math.round(safe / 1000)}k`;
}

function hexToRgba(hex: string, alpha: number): string {
  const match = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(hex);
  if (!match) return `rgba(56, 189, 248, ${alpha})`;
  const r = parseInt(match[1], 16);
  const g = parseInt(match[2], 16);
  const b = parseInt(match[3], 16);
  return `rgba(${r}, ${g}, ${b}, ${alpha})`;
}

function drawRoundedPill(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  w: number,
  h: number,
  r: number
): void {
  ctx.beginPath();
  if (typeof ctx.roundRect === 'function') {
    ctx.roundRect(x, y, w, h, r);
  } else {
    ctx.rect(x, y, w, h);
  }
}

/**
 * Renders curved quadratic Bezier aggregated super-edges between collapsed
 * Super-Nodes and curated clusters on the 2D OverlayCanvas.
 */
export function renderAggregatedSuperEdges(
  ctx: CanvasRenderingContext2D,
  bridges: ActiveSuperEdge[],
  getViewportPos: (nodeId: string) => { x: number; y: number } | null,
  options: {
    hoveredNodeId?: string | null;
    edgeMode: EdgeVisibilityMode;
  }
): void {
  if (bridges.length === 0) return;
  const hoveredId = options.hoveredNodeId || null;

  ctx.save();
  for (const bridge of bridges) {
    const isIncidentHover = Boolean(
      hoveredId && (bridge.sourceNodeId === hoveredId || bridge.targetNodeId === hoveredId)
    );

    if (options.edgeMode === 'focus' && !isIncidentHover) {
      continue;
    }

    const p1 = getViewportPos(bridge.sourceNodeId);
    const p2 = getViewportPos(bridge.targetNodeId);
    if (!p1 || !p2) continue;

    const dx = p2.x - p1.x;
    const dy = p2.y - p1.y;
    const len = Math.hypot(dx, dy);
    if (len < 6) continue;

    const mx = (p1.x + p2.x) / 2;
    const my = (p1.y + p2.y) / 2;
    const bend = Math.min(48, len * 0.14);
    const cx = mx - (dy / len) * bend;
    const cy = my + (dx / len) * bend;

    const baseWidth = Math.max(0.8, Math.min(3.8, 0.8 + Math.log2(bridge.weight + 1) * 0.65));

    ctx.beginPath();
    ctx.moveTo(p1.x, p1.y);
    ctx.quadraticCurveTo(cx, cy, p2.x, p2.y);
    if (isIncidentHover) {
      ctx.strokeStyle = hexToRgba(bridge.color, 0.22);
      ctx.lineWidth = baseWidth + 3.6;
      ctx.stroke();
    }
    ctx.strokeStyle = hexToRgba(
      bridge.color,
      isIncidentHover ? 0.82 : bridge.involvesCurated ? 0.24 : 0.14
    );
    ctx.lineWidth = isIncidentHover ? baseWidth + 0.8 : baseWidth;
    ctx.stroke();

    if (isIncidentHover) {
      const bx = 0.25 * p1.x + 0.5 * cx + 0.25 * p2.x;
      const by = 0.25 * p1.y + 0.5 * cy + 0.25 * p2.y;
      const label = `${bridge.weight} ${bridge.weight === 1 ? 'link' : 'links'}`;
      ctx.font = 'bold 9px monospace';
      const tw = ctx.measureText(label).width;
      const pw = tw + 12;
      const ph = 16;

      ctx.fillStyle = 'rgba(10, 15, 30, 0.94)';
      drawRoundedPill(ctx, bx - pw / 2, by - ph / 2, pw, ph, 4);
      ctx.fill();
      ctx.strokeStyle = hexToRgba(bridge.color, 0.75);
      ctx.lineWidth = 1;
      ctx.stroke();

      ctx.fillStyle = '#e0f2fe';
      ctx.textAlign = 'center';
      ctx.textBaseline = 'middle';
      ctx.fillText(label, bx, by + 0.5);
    }
  }
  ctx.restore();
}

/**
 * Renders radial nebula halos, concentric dashed rings, activity arcs, and
 * compact star-count badges around collapsed Sector and Cluster Super-Nodes.
 */
export function renderCollapsedSuperNodes(
  ctx: CanvasRenderingContext2D,
  collapsedNodes: SuperNodeMeta[],
  getViewportPos: (nodeId: string) => { x: number; y: number } | null,
  options: {
    hoveredNodeId?: string | null;
    viewportWidth: number;
    viewportHeight: number;
    cameraRatio?: number;
  }
): void {
  if (collapsedNodes.length === 0) return;

  const { hoveredNodeId, viewportWidth, viewportHeight, cameraRatio } = options;
  const margin = 80;

  ctx.save();
  for (const node of collapsedNodes) {
    const pos = getViewportPos(node.id);
    if (!pos) continue;
    if (
      pos.x < -margin ||
      pos.x > viewportWidth + margin ||
      pos.y < -margin ||
      pos.y > viewportHeight + margin
    ) {
      continue;
    }

    const isHovered = hoveredNodeId === node.id;
    const isSector = node.kind === 'sector';
    const dissolveAlpha = isSector && typeof cameraRatio === 'number'
      ? Math.max(0.35, Math.min(1, (cameraRatio - 0.46) / 0.14))
      : 1;
    ctx.globalAlpha = dissolveAlpha;
    const baseRadius = computeSuperNodeRadius(node.kind, node.noteCount);
    // Physical pixel clamping [18px, 64px] prevents excessive blur on zoom
    const haloRadius = Math.max(18, Math.min(64, baseRadius * (isHovered ? 2.85 : 2.35)));

    // 1. Radial nebula gas halo
    const grad = ctx.createRadialGradient(pos.x, pos.y, 2, pos.x, pos.y, haloRadius);
    grad.addColorStop(0, hexToRgba(node.color, isHovered ? 0.38 : 0.22));
    grad.addColorStop(0.55, hexToRgba(node.color, isHovered ? 0.14 : 0.07));
    grad.addColorStop(1, hexToRgba(node.color, 0));
    ctx.fillStyle = grad;
    ctx.beginPath();
    ctx.arc(pos.x, pos.y, haloRadius, 0, Math.PI * 2);
    ctx.fill();

    // 2. Dashed concentric orbital ring(s)
    const ringR = baseRadius + 6;
    ctx.beginPath();
    ctx.setLineDash([4, 4]);
    ctx.arc(pos.x, pos.y, ringR, 0, Math.PI * 2);
    ctx.strokeStyle = hexToRgba(node.color, isHovered ? 0.85 : 0.45);
    ctx.lineWidth = isHovered ? 1.5 : 1.05;
    ctx.stroke();

    if (isSector) {
      ctx.beginPath();
      ctx.setLineDash([2, 5]);
      ctx.arc(pos.x, pos.y, ringR + 6, 0, Math.PI * 2);
      ctx.strokeStyle = hexToRgba(node.color, isHovered ? 0.55 : 0.22);
      ctx.lineWidth = 0.95;
      ctx.stroke();
    }
    ctx.setLineDash([]);

    // 3. Recent 7-day activity arc (2-pass translucent glow, zero shadowBlur)
    if (node.recentCount > 0) {
      const ratio = Math.min(0.85, (node.recentCount / Math.max(1, node.noteCount)) * 4);
      ctx.beginPath();
      ctx.arc(pos.x, pos.y, ringR, -Math.PI * 0.65, -Math.PI * 0.65 + Math.PI * 2 * ratio);
      ctx.strokeStyle = 'rgba(56, 189, 248, 0.22)';
      ctx.lineWidth = 4.2;
      ctx.stroke();
      ctx.strokeStyle = '#67e8f9';
      ctx.lineWidth = 1.8;
      ctx.stroke();
    }
  }
  ctx.restore();
}
