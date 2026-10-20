import { sigmaContext } from '../lib/sigmaContext';
import { resolveProjectCategory } from '../lib/nebulas';
import {
  calculateNonCollidingNebulaCallout,
  calculateNonCollidingOutwardCallout,
  renderNeonCallout,
  renderCosmicCardCallout,
  drawSectorBeacon,
  isPointInCalloutBox,
} from '../lib/canvasMath';

export const drawPill = (
  ctx: CanvasRenderingContext2D,
  px: number,
  py: number,
  pw: number,
  ph: number,
  r: number
): void => {
  if (typeof ctx.roundRect === 'function') {
    ctx.roundRect(px, py, pw, ph, r);
  } else {
    ctx.rect(px, py, pw, ph);
  }
};

export interface NodeLabelData {
  repoCount?: number;
  noteCount?: number;
  x: number;
  y: number;
  size: number;
  label?: string | null;
  ring?: string;
  key?: string;
  id?: string;
  originalCategory?: string;
  category?: string;
  sector?: string;
  sectorDisplayName?: string;
  isSector?: boolean;
  isProject?: boolean;
}

export interface NodeLabelSettings {
  labelRenderedSizeThreshold?: number;
}

export const drawCustomLabel = (
  context: CanvasRenderingContext2D,
  data: NodeLabelData,
  settings: NodeLabelSettings
): void => {
  const { x, y, size, label, ring, key } = data;
  if (!label) return;
  const nodeKey = (key || data.id || '') as string;

  // 1. Isolated Constellation: stellar focus, suppresses outer labels and secondary nodes
  if (sigmaContext.constellation) {
    const constel = sigmaContext.constellation;
    if (nodeKey === constel.centerNodeId) {
      const rawTitle = label.replace(/\.(md|markdown)$/i, '').split('/').pop() || label;
      const title = rawTitle.length > 24 ? rawTitle.substring(0, 22) + '..' : rawTitle;
      const fullText = `★ ${title}`;

      context.save();
      context.font = 'bold 11px monospace';
      const textMetrics = context.measureText(fullText);
      const badgeW = textMetrics.width + 14;
      const badgeH = 20;
      const badgeX = x + size + 6;
      const badgeY = y - badgeH / 2;

      context.fillStyle = 'rgba(15, 23, 42, 0.94)';
      context.beginPath();
      drawPill(context, badgeX, badgeY, badgeW, badgeH, 4);
      context.fill();

      const borderColor = 'rgba(224, 242, 254, 0.95)';
      context.strokeStyle = borderColor;
      context.lineWidth = 1.4;
      context.shadowColor = borderColor;
      context.shadowBlur = 6;
      context.stroke();

      context.shadowBlur = 0;
      context.fillStyle = '#ffffff';
      context.textBaseline = 'middle';
      context.fillText(fullText, badgeX + 7, y);
      context.restore();
    }
    return;
  }

  // 2. Sector Anchors & Category Nebulas (Hierarchical LOD)
  if (label.includes('|SECTOR') || label.includes('|CATEGORY')) {
    const isSector = label.includes('|SECTOR') || nodeKey.startsWith('__SECTOR_');

    // Visual Cloak Filters
    if (isSector && sigmaContext.showSectorLabels === false) return;
    if (!isSector && sigmaContext.showNebulaLabels === false) return;

    const [realLabel, neonColor, _tag, catType, overviewStatus, _rank, rawRepoCount, rawNoteCount] = label.split('|');
    const repoCount = Number(rawRepoCount || data.repoCount || 0);
    const noteCount = Number(rawNoteCount || data.noteCount || 0);
    const isTop = overviewStatus === 'TOP';
    const isRoot = catType === 'ROOT';
    const rawCat = (nodeKey.startsWith('__CAT_')
      ? nodeKey.replace('__CAT_', '')
      : nodeKey.startsWith('__SECTOR_')
        ? nodeKey.replace('__SECTOR_', '')
        : (data.originalCategory || data.category || '')).toLowerCase();

    const camera = sigmaContext.sigma ? sigmaContext.sigma.getCamera() : null;
    const cameraRatio = camera ? camera.getState().ratio : 1;
    const isOverviewZoom = cameraRatio >= 0.40;

    // If a sector is focused, isolate it and sub-branches exclusively
    if (sigmaContext.focusedSector) {
      const focusedCat = sigmaContext.focusedSector.replace('__SECTOR_', '').replace('__CAT_', '').toLowerCase();
      const nodeSector = String(data.sector || resolveProjectCategory(rawCat).sector).toLowerCase();
      const matchesFocus =
        rawCat === focusedCat ||
        rawCat.startsWith(focusedCat + '/') ||
        focusedCat.startsWith(rawCat + '/') ||
        nodeSector === focusedCat;
      if (!matchesFocus) return;
    }

    // Hierarchical LOD switching:
    // Sector label: rendered at overview zoom (ratio >= 0.40); hidden at medium/deep zoom
    if (isSector) {
      // If this sector is already focused, hide its sector callout so user sees the inner nebulas!
      if (sigmaContext.focusedSector) {
        const focusedKey = sigmaContext.focusedSector.replace('__SECTOR_', '').replace('__CAT_', '').toLowerCase();
        const cleanFocusedKey = focusedKey.replace(/^(notes|sources)\//, '');
        const cleanRawCat = rawCat.replace(/^(notes|sources)\//, '');
        if (rawCat === focusedKey || cleanRawCat === cleanFocusedKey) {
          return;
        }
      }
      if (!isOverviewZoom && !sigmaContext.focusedSector) {
        return;
      }
      if (isOverviewZoom && !sigmaContext.focusedSector) {
        if (!isTop) return;
      }
    } else {
      // If this category cluster is already focused, hide its callout so user sees individual stars!
      if (sigmaContext.focusedCluster) {
        const focusedClusterKey = sigmaContext.focusedCluster.replace('__CAT_', '').toLowerCase();
        if (rawCat === focusedClusterKey) {
          return;
        }
      }
      // Repository / Category label:
      // If it belongs to a project sector that has a sector anchor:
      const isProjectRepo = Boolean(data.isProject ?? resolveProjectCategory(rawCat).isProject);
      if (isProjectRepo && isOverviewZoom && !sigmaContext.focusedSector) {
        // Suppress individual repository labels in overview zoom (Sector Anchor takes precedence)
        return;
      }

      if (isOverviewZoom && !sigmaContext.focusedSector) {
        if (!isTop) return;
      }
    }

    const threshold = settings.labelRenderedSizeThreshold || 8;
    if (!isSector && !isTop && !isRoot && size < threshold - 2) return;

    // Proximity Culling: prevent crowding if centers are too close (< 70px)
    const renderedCenters = sigmaContext.renderedNebulaCenters || [];
    const tooClose = renderedCenters.some(c => Math.hypot(c.x - x, c.y - y) < 70);
    if (tooClose && !isTop && !isRoot) return;

    // Direct root note categories under notes/ (ROOT) are strategic core pillars and always rendered.
    // Sectors and additional project nebulas count towards maxNebulas.
    if (!isRoot) {
      if (sigmaContext.frameLabelsCount >= sigmaContext.maxNebulas) {
        return;
      }
      sigmaContext.frameLabelsCount++;
    }

    const isLeft = x < window.innerWidth / 2;

    let textWidth: number;
    if (isSector) {
      context.font = 'bold 13px monospace';
      const titleW = context.measureText(realLabel).width;
      context.font = '8px monospace';
      const teleStr = repoCount > 0
        ? (noteCount > 0 ? `${repoCount} nebulas · ${noteCount} stars` : `${repoCount} nebulas`)
        : (noteCount > 0 ? `${noteCount} stars` : '');
      const teleW = context.measureText(`◈ SECTOR   ${teleStr}`).width;
      textWidth = Math.max(titleW + 24, teleW + 20, 100);
    } else {
      context.font = 'bold 13px monospace';
      const titleW = context.measureText(realLabel).width;
      context.font = '8px monospace';
      const teleStr = noteCount > 0 ? `${noteCount} stars` : '';
      const teleW = context.measureText(`◈ NEBULA   ${teleStr}`).width;
      textWidth = Math.max(titleW + 24, teleW + 20, 100);
    }
    
    const callout = calculateNonCollidingNebulaCallout(
      x,
      y,
      textWidth,
      isLeft,
      sigmaContext.frameCalloutBoxes || [],
      undefined,
      {
        category: rawCat,
        nodeId: nodeKey || ((isSector ? '__SECTOR_' : '__CAT_') + rawCat),
        isCategory: true,
        isSector,
      }
    );
    if (!callout) return;

    if (!sigmaContext.frameCalloutBoxes) sigmaContext.frameCalloutBoxes = [];
    sigmaContext.frameCalloutBoxes.push(callout.box);
    if (!sigmaContext.renderedNebulaCenters) sigmaContext.renderedNebulaCenters = [];
    sigmaContext.renderedNebulaCenters.push({ x, y });
    
    if (isSector) {
      drawSectorBeacon(context, { x, y }, neonColor);
      renderCosmicCardCallout(context, { x, y }, callout.geom, realLabel, neonColor, {
        subHeader: '◈ SECTOR',
        lineWidth: 1.8,
        glowBlur: 10,
        repoCount,
        noteCount,
      });
    } else {
      renderCosmicCardCallout(context, { x, y }, callout.geom, realLabel, neonColor, {
        subHeader: '◈ NEBULA',
        lineWidth: 1.5,
        glowBlur: 8,
        noteCount,
      });
    }
    return;
  }

  // 3. Seldon Perimeter Stations (Adaptive LOD)
  if (ring === 'skills' || ring === 'routines' || ring === 'harvesters') {
    if (!sigmaContext.showRings) return;

    const camera = sigmaContext.sigma ? sigmaContext.sigma.getCamera() : null;
    const cameraRatio = camera ? camera.getState().ratio : 1;
    const isOverviewZoom = cameraRatio >= 0.65;

    const ringMeta: Record<string, { foundation: string; badge: string; color: string }> = {
      harvesters: { foundation: 'Iª FOUNDATION', badge: 'HARVESTER', color: '#38bdf8' },
      skills: { foundation: 'Iª FOUNDATION', badge: 'SKILL', color: '#c084fc' },
      routines: { foundation: 'IIª FOUNDATION', badge: 'ROUTINE', color: '#fbbf24' },
    };
    const meta = ringMeta[ring] || { foundation: 'FOUNDATION', badge: 'ARMS', color: '#38bdf8' };
    const rawTitle = label.replace(/\.(md|markdown)$/i, '').split('/').pop() || label;

    let fullText = '';
    if (isOverviewZoom) {
      const compactTitle = rawTitle.length > 12 ? rawTitle.substring(0, 10) + '..' : rawTitle;
      fullText = `[${meta.badge}] ${compactTitle}`;
    } else {
      const title = rawTitle.length > 20 ? rawTitle.substring(0, 18) + '..' : rawTitle;
      fullText = `[${meta.foundation} · ${meta.badge}] ${title}`;
    }

    context.font = 'bold 10px monospace';
    const textWidth = context.measureText(fullText).width;

    let centerX = window.innerWidth / 2;
    let centerY = window.innerHeight / 2;
    if (sigmaContext.sigma && sigmaContext.galaxyCenter) {
      const centerV = sigmaContext.sigma.graphToViewport(sigmaContext.galaxyCenter);
      centerX = centerV.x;
      centerY = centerV.y;
    }

    const { geom, box } = calculateNonCollidingOutwardCallout(
      x,
      y,
      centerX,
      centerY,
      textWidth,
      sigmaContext.frameCalloutBoxes || [],
      {
        diagDist: 26,
        padding: 14,
        textOffsetBottom: 3,
      },
      { nodeId: nodeKey, isStation: true }
    );
    if (!sigmaContext.frameCalloutBoxes) sigmaContext.frameCalloutBoxes = [];
    sigmaContext.frameCalloutBoxes.push(box);

    renderNeonCallout(context, { x, y }, geom, fullText, meta.color, {
      lineWidth: 1.4,
      glowBlur: 6,
      drawTerminalPoint: true,
    });
    return;
  }

  // 4. Default notes
  const isSectorFocused = Boolean(sigmaContext.focusedSector);
  const threshold = isSectorFocused ? 2 : (settings.labelRenderedSizeThreshold || 8);
  if (size >= threshold) {
    const rawTitle = label.replace(/\.(md|markdown)$/i, '').split('/').pop() || label;
    const displayLabel = rawTitle.length > 20 ? rawTitle.substring(0, 18) + '..' : rawTitle;
    const labelX = x + size + 4;
    const labelY = y;

    const collidesWithCallout = (sigmaContext.frameCalloutBoxes || []).some(box =>
      isPointInCalloutBox(labelX, labelY, box, 4)
    );
    if (!collidesWithCallout) {
      context.save();
      context.font = '10px sans-serif';
      context.fillStyle = 'rgba(203, 213, 225, 0.75)';
      context.textBaseline = 'middle';
      context.fillText(displayLabel, labelX, labelY);
      context.restore();
    }
  }
};

export interface NodeHoverData {
  x: number;
  y: number;
  size: number;
  label?: string | null;
  color?: string;
}

export const drawHover = (context: CanvasRenderingContext2D, data: NodeHoverData): void => {
  const { x, y, size, label, color } = data;
  if (!label || label.includes('|CATEGORY') || label.includes('|SECTOR')) return;
  
  // Subtle glowing halo around the hovered star (the rich HTML NodeHoverTooltip handles all text)
  context.save();
  context.beginPath();
  context.arc(x, y, Math.max(size * 1.6, 5), 0, Math.PI * 2);
  context.strokeStyle = color || '#6366f1';
  context.lineWidth = 1.8;
  context.shadowColor = color || '#6366f1';
  context.shadowBlur = 10;
  context.stroke();
  context.restore();
};
