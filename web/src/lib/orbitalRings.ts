/**
 * @file orbitalRings.ts
 * Polar geometry calculation and rendering of ARMS concentric orbital rings
 * (Applications, Routines, Memory, Skills).
 */

export interface OrbitalNodeInput {
  id: string;
  ring?: string; // 'memory' | 'skills' | 'routines' | 'harvesters' | 'hooks'
  x?: number;
  y?: number;
}

export interface OrbitRingDef {
  id: 'skills' | 'routines' | 'harvesters' | 'perimeter';
  name: string;
  shortLabel: string;
  radius: number;
  color: string;
  dashPattern: number[];
}

export interface OrbitalLayoutResult {
  center: { x: number; y: number };
  coreRadius: number;
  rings: OrbitRingDef[];
  nodePositions: Map<string, { x: number; y: number; angle: number; ring: string }>;
}

/**
 * Calculates galactic core radius and distributes ARMS satellites along
 * the unified Seldon Perimeter (First Foundation & Second Foundation sectors).
 */
export function calculateOrbitalLayout(
  nodes: OrbitalNodeInput[],
  options: {
    baseMargin?: number;
    defaultRadius?: number;
  } = {}
): OrbitalLayoutResult {
  const baseMargin = options.baseMargin ?? 500;
  const defaultRadius = options.defaultRadius ?? 6500;

  // 1. Calculate centroid and maximum radius of core memory envelope
  let sumX = 0;
  let sumY = 0;
  let memoryCount = 0;

  nodes.forEach((n) => {
    if (!n.ring || n.ring === 'memory') {
      if (typeof n.x === 'number' && typeof n.y === 'number' && !Number.isNaN(n.x) && !Number.isNaN(n.y)) {
        sumX += n.x;
        sumY += n.y;
        memoryCount++;
      }
    }
  });

  const centerX = memoryCount > 0 ? sumX / memoryCount : 0;
  const centerY = memoryCount > 0 ? sumY / memoryCount : 0;

  const distances: number[] = [];
  nodes.forEach((n) => {
    if (!n.ring || n.ring === 'memory') {
      if (typeof n.x === 'number' && typeof n.y === 'number' && !Number.isNaN(n.x) && !Number.isNaN(n.y)) {
        const dx = n.x - centerX;
        const dy = n.y - centerY;
        distances.push(Math.sqrt(dx * dx + dy * dy));
      }
    }
  });

  let rawCoreRadius = 0;
  if (distances.length > 0) {
    distances.sort((a, b) => a - b);
    const pIndex = Math.min(distances.length - 1, Math.floor(distances.length * 0.995));
    const p99 = distances[pIndex];
    const maxDist = distances[distances.length - 1];
    rawCoreRadius = Math.max(p99, maxDist * 0.92);
  }

  const galaxyEnvelopeMargin = 450;
  const coreRadius = Math.max(defaultRadius, rawCoreRadius + galaxyEnvelopeMargin);
  const perimeterRadius = coreRadius + baseMargin;

  const rings: OrbitRingDef[] = [
    {
      id: 'perimeter',
      name: "Seldon Perimeter // Two Foundations Orbit",
      shortLabel: "SELDON PERIMETER // FOUNDATIONS ORBIT",
      radius: perimeterRadius,
      color: "rgba(168, 85, 247, 0.65)",
      dashPattern: [10, 10],
    },
  ];

  // Group and order satellites by Foundation
  // First Foundation: Harvesters (5) + Skills (7)
  // Second Foundation: Routines (3)
  const harvesters: OrbitalNodeInput[] = [];
  const skills: OrbitalNodeInput[] = [];
  const routines: OrbitalNodeInput[] = [];
  const hooks: OrbitalNodeInput[] = [];

  nodes.forEach((n) => {
    if (n.ring === 'harvesters') harvesters.push(n);
    else if (n.ring === 'skills') skills.push(n);
    else if (n.ring === 'routines') routines.push(n);
    else if (n.ring === 'hooks') hooks.push(n);
  });

  const orderedSatellites = [...harvesters, ...skills, ...routines, ...hooks];
  const totalCount = orderedSatellites.length;
  const nodePositions = new Map<string, { x: number; y: number; angle: number; ring: string }>();

  if (totalCount > 0) {
    const angleStep = (2 * Math.PI) / totalCount;
    // Small initial phase to avoid aligning directly with 0 rad
    const initialPhase = Math.PI / 12;

    orderedSatellites.forEach((sat, idx) => {
      const angle = initialPhase + idx * angleStep;
      const x = centerX + perimeterRadius * Math.cos(angle);
      const y = centerY + perimeterRadius * Math.sin(angle);
      nodePositions.set(sat.id, {
        x,
        y,
        angle,
        ring: sat.ring || 'perimeter',
      });
    });
  }

  return {
    center: { x: centerX, y: centerY },
    coreRadius,
    rings,
    nodePositions,
  };
}

/**
 * Renders the unified Seldon Perimeter and Foundation Sector markers on the Canvas 2D overlay.
 */
export function renderOrbitalRingsOnCanvas(
  ctx: CanvasRenderingContext2D,
  centerViewport: { x: number; y: number },
  scaledRings: Array<{ radiusViewport: number; color: string; dashPattern: number[]; shortLabel: string }>
): void {
  ctx.save();

  scaledRings.forEach((ring) => {
    if (ring.radiusViewport <= 10) return;

    // 1. Primary Seldon Perimeter Circle with cybernetic glow
    ctx.beginPath();
    ctx.arc(centerViewport.x, centerViewport.y, ring.radiusViewport, 0, 2 * Math.PI);
    ctx.setLineDash(ring.dashPattern);
    ctx.lineWidth = 1.8;
    ctx.strokeStyle = ring.color;
    ctx.shadowColor = ring.color;
    ctx.shadowBlur = 10;
    ctx.stroke();

    // 2. Geodesic astrolabe coordinate ticks every 15 degrees
    ctx.shadowBlur = 0;
    ctx.setLineDash([]);
    for (let angle = 0; angle < 2 * Math.PI; angle += Math.PI / 12) {
      const innerR = ring.radiusViewport - 4;
      const outerR = ring.radiusViewport + 4;
      ctx.beginPath();
      ctx.moveTo(centerViewport.x + innerR * Math.cos(angle), centerViewport.y + innerR * Math.sin(angle));
      ctx.lineTo(centerViewport.x + outerR * Math.cos(angle), centerViewport.y + outerR * Math.sin(angle));
      ctx.strokeStyle = 'rgba(168, 85, 247, 0.40)';
      ctx.lineWidth = 1.0;
      ctx.stroke();
    }

    // 3. Two Foundations Holographic Arc Badges
    ctx.font = 'bold 10px "JetBrains Mono", monospace';

    // First Foundation (Terminus Operational Sector) - Top Arc
    ctx.fillStyle = 'rgba(56, 189, 248, 0.90)';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'bottom';
    ctx.fillText(
      '✦ FIRST FOUNDATION // TERMINUS OPERATIONAL SECTOR ✦',
      centerViewport.x,
      centerViewport.y - ring.radiusViewport - 8
    );

    // Second Foundation (Star\'s End Mentalics) - Bottom Arc
    ctx.fillStyle = 'rgba(251, 191, 36, 0.90)';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'top';
    ctx.fillText(
      '✦ SECOND FOUNDATION // STAR\'S END MENTALICS ✦',
      centerViewport.x,
      centerViewport.y + ring.radiusViewport + 8
    );
  });

  ctx.restore();
}
