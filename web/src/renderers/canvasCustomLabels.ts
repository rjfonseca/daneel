import { sigmaContext } from '../lib/sigmaContext';
import { resolveProjectCategory } from '../lib/nebulas';
import {
  clampOutwardCalloutToSafeBounds,
  doesRectOverlapBoxes,
  getSafeHudBounds,
  sanitizeArmsStationLabel,
  shouldShowOverviewRootNebula,
} from '../lib/galaxy/labelLayout';
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
  hidden?: boolean;
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
  sectorKey?: string;
  sectorDisplayName?: string;
  trustScope?: string;
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
  if (!label || data.hidden) return;
  const nodeKey = (key || data.id || '') as string;

  // 1. Contextual Constellation: focal star pill + non-colliding 1-hop neighbor labels
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

      context.strokeStyle = 'rgba(224, 242, 254, 0.95)';
      context.lineWidth = 1.4;
      context.stroke();

      context.fillStyle = '#ffffff';
      context.textBaseline = 'middle';
      context.fillText(fullText, badgeX + 7, y);
      context.restore();

      if (!sigmaContext.frameCalloutBoxes) sigmaContext.frameCalloutBoxes = [];
      sigmaContext.frameCalloutBoxes.push({
        minX: badgeX,
        maxX: badgeX + badgeW,
        minY: badgeY,
        maxY: badgeY + badgeH,
        nodeId: nodeKey,
      });
      return;
    }

    if (constel.hopDistances.get(nodeKey) === 1) {
      const cameraRatio = sigmaContext.sigma ? sigmaContext.sigma.getCamera().getState().ratio : 0.1;
      const neighborCount = sigmaContext.frameConstellationNeighborLabelsCount ?? 0;
      if (cameraRatio <= 0.24 && neighborCount < 10) {
        const rawTitle = label.replace(/\.(md|markdown)$/i, '').split('/').pop() || label;
        const display = rawTitle.length > 22 ? rawTitle.substring(0, 20) + '..' : rawTitle;
        context.save();
        context.font = '10px sans-serif';
        const tw = context.measureText(display).width;
        const badgeW = tw + 10;
        const badgeH = 17;
        const badgeX = x + size + 5;
        const badgeY = y - badgeH / 2;
        const rect = { minX: badgeX, maxX: badgeX + badgeW, minY: badgeY, maxY: badgeY + badgeH, nodeId: nodeKey };
        const boxes = sigmaContext.frameCalloutBoxes || [];
        if (!doesRectOverlapBoxes(rect, boxes, 4)) {
          context.fillStyle = 'rgba(10, 15, 30, 0.78)';
          context.beginPath();
          drawPill(context, badgeX, badgeY, badgeW, badgeH, 3);
          context.fill();
          context.strokeStyle = 'rgba(56, 189, 248, 0.32)';
          context.lineWidth = 1;
          context.stroke();
          context.fillStyle = 'rgba(226, 232, 240, 0.88)';
          context.textBaseline = 'middle';
          context.fillText(display, badgeX + 5, y);

          if (!sigmaContext.frameCalloutBoxes) sigmaContext.frameCalloutBoxes = [];
          sigmaContext.frameCalloutBoxes.push(rect);
          sigmaContext.frameConstellationNeighborLabelsCount = neighborCount + 1;
        }
        context.restore();
      }
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
    const lod = sigmaContext.lodState;
    const nodeTrustScope = data.trustScope || (rawCat.startsWith('sources/') || rawCat === 'sources' ? 'sources' : 'curated');
    const resolvedSectorKey = String(data.sectorKey || data.sector || resolveProjectCategory(rawCat).sector || rawCat).toLowerCase();

    if (sigmaContext.scopeMode === 'curated' && nodeTrustScope === 'sources') {
      return;
    }

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
    // Sector label: rendered at overview zoom or when collapsed in LOD; hidden when unfolded
    let allowLodExpandedClusterLabel = false;
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
      if (!sigmaContext.focusedSector && lod && lod.expandedSectors.has(rawCat)) {
        return;
      }
      const isCollapsedByLod = Boolean(lod && lod.collapsedSectorNodes.has(nodeKey));
      if (!isOverviewZoom && !sigmaContext.focusedSector && !isCollapsedByLod) {
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
      if (!sigmaContext.focusedSector && !sigmaContext.focusedCluster && lod && nodeTrustScope === 'sources') {
        if (lod.collapsedSectorNodes.has(`__SECTOR_${resolvedSectorKey}`)) {
          return;
        }
        if (lod.collapsedClusterNodes.has(nodeKey)) {
          allowLodExpandedClusterLabel = true;
        }
      }
      // Repository / Category label:
      // If it belongs to a project sector that has a sector anchor:
      const isProjectRepo = Boolean(data.isProject ?? resolveProjectCategory(rawCat).isProject);
      if (isProjectRepo && isOverviewZoom && !sigmaContext.focusedSector && !allowLodExpandedClusterLabel) {
        // Suppress individual repository labels in overview zoom (Sector Anchor takes precedence)
        return;
      }

      if (isOverviewZoom && !sigmaContext.focusedSector && !allowLodExpandedClusterLabel) {
        if (!isTop) return;
      }
    }

    const threshold = settings.labelRenderedSizeThreshold || 8;
    if (!isSector && !isTop && !isRoot && !allowLodExpandedClusterLabel && size < threshold - 2) return;

    // Progressive overview budget for root notes/* categories (prevents southern pileup in large vaults)
    const isFocusedView = Boolean(sigmaContext.focusedSector || sigmaContext.focusedCluster);
    const renderedRoots = sigmaContext.frameRootLabelsCount ?? 0;
    if (!isSector && isRoot && !shouldShowOverviewRootNebula(renderedRoots, sigmaContext.maxNebulas, cameraRatio, isFocusedView)) {
      return;
    }

    // Proximity Culling: prevent crowding if centers are too close
    const minCenterDist = isOverviewZoom && !isFocusedView ? 110 : 70;
    const renderedCenters = sigmaContext.renderedNebulaCenters || [];
    const tooClose = renderedCenters.some(c => Math.hypot(c.x - x, c.y - y) < minCenterDist);
    if (tooClose && (!isTop && (!isRoot || renderedRoots >= 2))) return;

    // Direct root note categories under notes/ count toward progressive root cap; sectors/project nebulas share maxNebulas.
    if (isRoot && !isSector) {
      sigmaContext.frameRootLabelsCount = renderedRoots + 1;
    } else {
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

  // 3. Seldon Perimeter Stations (Adaptive LOD + Safe HUD Clamping)
  if (ring === 'skills' || ring === 'routines' || ring === 'harvesters') {
    if (!sigmaContext.showRings) return;

    const camera = sigmaContext.sigma ? sigmaContext.sigma.getCamera() : null;
    const cameraRatio = camera ? camera.getState().ratio : 1;
    const isOverviewZoom = cameraRatio >= 0.65;

    const ringMeta: Record<string, { badge: string; color: string }> = {
      harvesters: { badge: 'HARVESTER', color: '#38bdf8' },
      skills: { badge: 'SKILL', color: '#c084fc' },
      routines: { badge: 'ROUTINE', color: '#fbbf24' },
    };
    const meta = ringMeta[ring] || { badge: 'ARMS', color: '#38bdf8' };
    const fullText = sanitizeArmsStationLabel(label, ring, isOverviewZoom);

    context.font = 'bold 10px monospace';
    const textWidth = context.measureText(fullText).width;

    let centerX = window.innerWidth / 2;
    let centerY = window.innerHeight / 2;
    if (sigmaContext.sigma && sigmaContext.galaxyCenter) {
      const centerV = sigmaContext.sigma.graphToViewport(sigmaContext.galaxyCenter);
      centerX = centerV.x;
      centerY = centerV.y;
    }

    const rawOutward = calculateNonCollidingOutwardCallout(
      x,
      y,
      centerX,
      centerY,
      textWidth,
      sigmaContext.frameCalloutBoxes || [],
      {
        diagDist: 24,
        padding: 12,
        textOffsetBottom: 3,
      },
      { nodeId: nodeKey, isStation: true }
    );
    const safeBounds = getSafeHudBounds(
      window.innerWidth,
      window.innerHeight,
      114,
      window.innerHeight - 92
    );
    const { geom, box } = clampOutwardCalloutToSafeBounds(rawOutward.geom, rawOutward.box, safeBounds);
    if (!sigmaContext.frameCalloutBoxes) sigmaContext.frameCalloutBoxes = [];
    sigmaContext.frameCalloutBoxes.push(box);

    renderNeonCallout(context, { x, y }, geom, fullText, meta.color, {
      lineWidth: 1.4,
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
  hidden?: boolean;
  label?: string | null;
  color?: string;
}

export const drawHover = (context: CanvasRenderingContext2D, data: NodeHoverData): void => {
  const { x, y, size, label, color, hidden } = data;
  if (hidden || !label || label.includes('|CATEGORY') || label.includes('|SECTOR')) return;
  
  // Subtle 2-pass glowing halo around the hovered star (zero shadowBlur for 60fps consistency)
  const r = Math.max(size * 1.6, 5);
  context.save();
  context.beginPath();
  context.arc(x, y, r, 0, Math.PI * 2);
  context.strokeStyle = color || '#6366f1';
  context.globalAlpha = 0.28;
  context.lineWidth = 4.2;
  context.stroke();

  context.globalAlpha = 0.95;
  context.lineWidth = 1.6;
  context.stroke();
  context.restore();
};
