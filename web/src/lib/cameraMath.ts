/**
 * @file cameraMath.ts
 * Funções puras de cálculo geométrico e enquadramento da câmera Sigma.js.
 *
 * Invariantes Fundamentais do Sistema Cósmico:
 * 1. No Sigma.js, camera.ratio opera como inverso de zoom:
 *    - ratio menor = mais próximo / maior magnificação (ex: 0.04 a 0.08)
 *    - ratio maior = mais distante / menor magnificação (ex: 0.40 a 1.00)
 * 2. Princípio da Âncora Intacta:
 *    - O ponto focal da câmera deve ancorar firmemente na entidade selecionada pelo usuário.
 *    - Vizinhos secundários apenas adaptam a razão de zoom (se estiverem dentro do raio local),
 *      nunca distorcendo o ponto focal para o vácuo interestelar.
 * 3. Compensação do Painel Lateral (Drawer de 440px):
 *    - Quando o drawer está aberto em telas desktop (>= 900px), o centro útil do viewport
 *      é deslocado para a esquerda: openCenterX = (width - 440) / 2.
 */

export interface Point2D {
  x: number;
  y: number;
}

export interface ViewportDimensions {
  width: number;
  height: number;
}

export interface CameraFramingResult {
  x: number;
  y: number;
  ratio: number;
}

/** Preserve the current zoom while moving a selected node to the visible center beside a drawer. */
export function calculateDrawerRecenterX(
  targetFramed: Point2D,
  ratio: number,
  viewportDims: ViewportDimensions,
  drawerWidth: number,
  viewportToFramed?: (pt: Point2D, options: { cameraState: { x: number; y: number; ratio: number; angle: number } }) => Point2D,
): number {
  if (viewportDims.width < 900) return targetFramed.x;
  const openCenterX = (viewportDims.width - drawerWidth) / 2;
  if (viewportToFramed) {
    const cameraState = { ...targetFramed, ratio, angle: 0 };
    const center = viewportToFramed({ x: viewportDims.width / 2, y: viewportDims.height / 2 }, { cameraState });
    const openCenter = viewportToFramed({ x: openCenterX, y: viewportDims.height / 2 }, { cameraState });
    return targetFramed.x + center.x - openCenter.x;
  }
  return targetFramed.x + (viewportDims.width / 2 - openCenterX) * ratio / Math.min(viewportDims.width, viewportDims.height);
}

/**
 * Garante que a razão de zoom permaneça em limites seguros.
 */
export function clampCameraRatio(ratio: number, min: number = 0.020, max: number = 0.55): number {
  if (Number.isNaN(ratio) || !Number.isFinite(ratio)) return min;
  return Math.max(min, Math.min(max, ratio));
}

/**
 * Calcula o enquadramento ideal para que um conjunto de pontos (estrelas de uma nebulosa/setor)
 * ocupe entre 75% e 80% da área visível da tela ("full screen").
 */
export function calculateClusterFraming(
  points: Point2D[],
  viewportDims: ViewportDimensions,
  options: {
    targetCoverageFraction?: number;
    minRatio?: number;
    maxRatio?: number;
    minSpan?: number;
  } = {}
): CameraFramingResult {
  const targetCoverage = options.targetCoverageFraction ?? 0.78;
  const minRatio = options.minRatio ?? (points && points.length <= 2 ? 0.16 : 0.005);
  const maxRatio = options.maxRatio ?? 0.55;

  if (!points || points.length === 0) {
    return { x: 0.5, y: 0.5, ratio: 1.0 };
  }

  let minX = Infinity, maxX = -Infinity;
  let minY = Infinity, maxY = -Infinity;
  let sumX = 0, sumY = 0;

  for (const p of points) {
    minX = Math.min(minX, p.x);
    maxX = Math.max(maxX, p.x);
    minY = Math.min(minY, p.y);
    maxY = Math.max(maxY, p.y);
    sumX += p.x;
    sumY += p.y;
  }

  const centroidX = sumX / points.length;
  const centroidY = sumY / points.length;
  const minSpan = options.minSpan ?? (points.length <= 2 ? 0.08 : 0.001);
  const spanX = Math.max(maxX - minX, minSpan);
  const spanY = Math.max(maxY - minY, minSpan);

  const minDim = Math.min(viewportDims.width, viewportDims.height);
  const ratioX = (spanX * minDim) / (targetCoverage * viewportDims.width);
  const ratioY = (spanY * minDim) / (targetCoverage * viewportDims.height);
  const rawRatio = Math.max(ratioX, ratioY);

  const ratio = clampCameraRatio(rawRatio, minRatio, maxRatio);

  return {
    x: centroidX,
    y: centroidY,
    ratio,
  };
}

/**
 * Calcula a posição de câmera para focar em uma nota individual ou satélite de infraestrutura,
 * aplicando compensação paramétrica do Drawer lateral (440px).
 */
export function calculateNodeFocusCamera(
  targetFramed: Point2D,
  isSatellite: boolean,
  neighborFramedPoints: Point2D[],
  viewportDims: ViewportDimensions,
  options: {
    drawerWidth?: number;
    satelliteRatio?: number;
    maxLocalNeighborhoodRadius?: number;
    viewportToFramed?: (pt: Point2D, options: { cameraState: { x: number; y: number; ratio: number; angle: number } }) => Point2D;
  } = {}
): CameraFramingResult {
  const drawerWidth = options.drawerWidth ?? 440;
  const satelliteRatio = options.satelliteRatio ?? 0.08;
  const maxLocalRadius = options.maxLocalNeighborhoodRadius ?? 0.18;
  const viewportToFramed = options.viewportToFramed;

  const dims = viewportDims;
  const isDesktop = dims.width >= 900;
  const openCenterX = isDesktop ? (dims.width - drawerWidth) / 2 : dims.width / 2;
  const availableWidth = isDesktop ? (dims.width - drawerWidth) : dims.width;
  const availableHeight = Math.max(300, dims.height - 80);
  const minDim = Math.min(dims.width, dims.height);

  const getDx = (ratio: number) => {
    if (viewportToFramed) {
      const tempCam = { x: targetFramed.x, y: targetFramed.y, ratio, angle: 0 };
      const framedCenter = viewportToFramed({ x: dims.width / 2, y: dims.height / 2 }, { cameraState: tempCam });
      const framedTarget = viewportToFramed({ x: openCenterX, y: dims.height / 2 }, { cameraState: tempCam });
      return framedCenter.x - framedTarget.x;
    }
    return (dims.width / 2 - openCenterX) * (ratio / minDim);
  };

  // Satélites ARMS (na órbita externa) ancoram estritamente no próprio satélite
  if (isSatellite || neighborFramedPoints.length === 0) {
    const targetRatio = satelliteRatio;
    const dx = getDx(targetRatio);
    return {
      x: targetFramed.x + dx,
      y: targetFramed.y,
      ratio: targetRatio,
    };
  }

  // Para notas conceituais: filtrar vizinhos locais (descarta links transgalácticos que distorcem o enquadramento)
  const localNeighbors = neighborFramedPoints.filter(p => {
    const dist = Math.hypot(p.x - targetFramed.x, p.y - targetFramed.y);
    return dist <= maxLocalRadius;
  });

  let maxSpanX = 0.02;
  let maxSpanY = 0.02;

  if (localNeighbors.length > 0) {
    for (const n of localNeighbors) {
      maxSpanX = Math.max(maxSpanX, Math.abs(n.x - targetFramed.x) * 2.1);
      maxSpanY = Math.max(maxSpanY, Math.abs(n.y - targetFramed.y) * 2.1);
    }
  }

  const ratioX = (maxSpanX * minDim) / (0.70 * availableWidth);
  const ratioY = (maxSpanY * minDim) / (0.70 * availableHeight);
  const rawRatio = Math.max(ratioX, ratioY);
  const targetRatio = clampCameraRatio(rawRatio, 0.04, 0.14);

  const dx = getDx(targetRatio);

  return {
    x: targetFramed.x + dx,
    y: targetFramed.y,
    ratio: targetRatio,
  };
}
