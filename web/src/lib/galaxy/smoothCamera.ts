/**
 * @file smoothCamera.ts
 * Log-space camera flight interpolation (van Wijk & Nuij, 2003) and
 * altitude-adaptive, critically damped, velocity-capped wheel zoom controller.
 */

import type Sigma from 'sigma';
import type { WheelCoords } from 'sigma/types';

export const MIN_CAMERA_RATIO = 0.004;
export const MAX_CAMERA_RATIO = 1.5;
export const MAX_LOG_LEAD = 0.75;
export const MAX_LOG_SPEED = 2.4;
export const WHEEL_DAMPING_TAU_MS = 110;

export interface CameraPose {
  x: number;
  y: number;
  ratio: number;
  angle: number;
}

export type CameraEasingName = 'cubicOut' | 'cubicInOut';

export function cubicOut(t: number): number {
  const k = Math.max(0, Math.min(1, t)) - 1;
  return k * k * k + 1;
}

export function cubicInOut(t: number): number {
  let k = Math.max(0, Math.min(1, t)) * 2;
  if (k < 1) return 0.5 * k * k * k;
  k -= 2;
  return 0.5 * (k * k * k + 2);
}

/**
 * Interpolates camera state in logarithmic zoom space with zoom-compensated
 * pan weighting so the visual target moves smoothly without end-of-flight snaps.
 */
export function interpolateCameraStateLog(
  from: CameraPose,
  to: CameraPose,
  easedT: number
): CameraPose {
  const p = Math.max(0, Math.min(1, easedT));
  const r0 = Math.max(1e-6, from.ratio);
  const r1 = Math.max(1e-6, to.ratio);

  const ratio = r0 * Math.pow(r1 / r0, p);
  const ratioDelta = r1 - r0;
  const panWeight = Math.abs(ratioDelta) > 1e-6 ? (ratio - r0) / ratioDelta : p;

  return {
    x: from.x + (to.x - from.x) * panWeight,
    y: from.y + (to.y - from.y) * panWeight,
    ratio,
    angle: from.angle + (to.angle - from.angle) * p,
  };
}

/**
 * Computes an altitude-adaptive wheel gain multiplier in log-ratio space:
 * brisk at galaxy overview (~1.0), tapering smoothly via Hermite smoothstep
 * to ~0.45 at deep star inspection for surgical control without whiplash.
 */
export function computeAltitudeGainMultiplier(currentRatio: number): number {
  const safeRatio = Number.isFinite(currentRatio) && currentRatio > 0 ? currentRatio : 1;
  const minLog = Math.log(0.008);
  const maxLog = Math.log(0.85);
  const t = Math.max(0, Math.min(1, (Math.log(safeRatio) - minLog) / (maxLog - minLog)));
  const smooth = t * t * (3 - 2 * t);
  return 0.45 + 0.55 * smooth;
}

/**
 * Normalizes mouse wheel and trackpad/pinch WheelEvent deltas into a bounded,
 * altitude-compensated multiplicative camera.ratio factor (< 1 zooms in, > 1 zooms out).
 */
export function normalizeWheelDelta(
  deltaY: number,
  deltaMode: number = 0,
  ctrlKey: boolean = false,
  currentRatio: number = 1.0
): number {
  if (!Number.isFinite(deltaY) || deltaY === 0) return 1;

  let deltaPx = deltaY;
  if (deltaMode === 1) {
    deltaPx *= 32;
  } else if (deltaMode === 2) {
    deltaPx *= 400;
  }
  if (ctrlKey) {
    deltaPx *= 3.2;
  }

  const clampedPx = Math.max(-180, Math.min(180, deltaPx));
  const effectiveGain = 0.00175 * computeAltitudeGainMultiplier(currentRatio);
  return Math.exp(clampedPx * effectiveGain);
}

export function computeWheelTargetRatio(
  currentTargetRatio: number,
  wheelFactor: number,
  minRatio: number = MIN_CAMERA_RATIO,
  maxRatio: number = MAX_CAMERA_RATIO
): number {
  const base = Number.isFinite(currentTargetRatio) && currentTargetRatio > 0 ? currentTargetRatio : 1;
  const factor = Number.isFinite(wheelFactor) && wheelFactor > 0 ? wheelFactor : 1;
  return Math.max(minRatio, Math.min(maxRatio, base * factor));
}

/**
 * Advances one frame of velocity-capped, critically damped log-space wheel glide.
 */
export function stepSmoothZoomLog(
  currentRatio: number,
  targetRatio: number,
  dtMs: number,
  options?: { tauMs?: number; maxLogSpeed?: number }
): { nextRatio: number; settled: boolean } {
  const safeCur = Math.max(1e-6, currentRatio);
  const safeTarget = Math.max(1e-6, targetRatio);
  const logDiff = Math.log(safeTarget / safeCur);
  if (Math.abs(logDiff) < 0.0005) {
    return { nextRatio: safeTarget, settled: true };
  }

  const tauMs = options?.tauMs ?? WHEEL_DAMPING_TAU_MS;
  const maxLogSpeed = options?.maxLogSpeed ?? MAX_LOG_SPEED;
  const alpha = 1 - Math.exp(-Math.max(1, dtMs) / tauMs);
  const desiredStep = logDiff * alpha;
  const maxStep = maxLogSpeed * (Math.max(1, dtMs) / 1000);
  const clampedStep = Math.max(-maxStep, Math.min(maxStep, desiredStep));
  return {
    nextRatio: safeCur * Math.exp(clampedStep),
    settled: false,
  };
}

export interface AnimateToOptions {
  duration?: number;
  easing?: CameraEasingName;
  onComplete?: () => void;
  onCancel?: () => void;
}

export interface SmoothCameraControllerOptions {
  sigma: Sigma;
  getReducedMotion: () => boolean;
  minRatio?: number;
  maxRatio?: number;
  onUserWheelInterrupt?: () => void;
}

/**
 * Coordinates smooth programmatic flights and momentum-damped cursor-anchored
 * wheel zooming on a Sigma instance without target runaway or whip-crack overshoot.
 */
export class SmoothCameraController {
  private readonly sigma: Sigma;
  private readonly getReducedMotion: () => boolean;
  private readonly minRatio: number;
  private readonly maxRatio: number;
  private readonly onUserWheelInterrupt?: () => void;

  private flightFrameId: number | null = null;
  private activeFlightCancel: (() => void) | null = null;

  private wheelFrameId: number | null = null;
  private wheelTargetRatio: number | null = null;
  private wheelCursor: { x: number; y: number } = { x: 0, y: 0 };
  private lastWheelTickTime = 0;

  constructor(options: SmoothCameraControllerOptions) {
    this.sigma = options.sigma;
    this.getReducedMotion = options.getReducedMotion;
    this.minRatio = options.minRatio ?? MIN_CAMERA_RATIO;
    this.maxRatio = options.maxRatio ?? MAX_CAMERA_RATIO;
    this.onUserWheelInterrupt = options.onUserWheelInterrupt;
  }

  public isAnimating(): boolean {
    return this.flightFrameId !== null || this.wheelFrameId !== null;
  }

  public cancelFlight(): void {
    if (this.flightFrameId !== null) {
      cancelAnimationFrame(this.flightFrameId);
      this.flightFrameId = null;
    }
    if (this.activeFlightCancel) {
      const cancelCb = this.activeFlightCancel;
      this.activeFlightCancel = null;
      cancelCb();
    }
    // Also cancel any external Sigma camera.animate() loop so it cannot overwrite us.
    const cam = this.sigma.getCamera() as unknown as {
      nextFrame?: number | null;
      animationCallback?: (() => void) | undefined;
    };
    if (cam.nextFrame) {
      cancelAnimationFrame(cam.nextFrame);
      cam.nextFrame = null;
      if (cam.animationCallback) {
        const cb = cam.animationCallback;
        cam.animationCallback = undefined;
        cb();
      }
    }
  }

  public cancelWheelGlide(): void {
    if (this.wheelFrameId !== null) {
      cancelAnimationFrame(this.wheelFrameId);
      this.wheelFrameId = null;
    }
    this.wheelTargetRatio = null;
  }

  public animateTo(target: Partial<CameraPose>, options: AnimateToOptions = {}): void {
    this.cancelWheelGlide();
    this.cancelFlight();

    const camera = this.sigma.getCamera();
    const initial = camera.getState();
    const destination: CameraPose = {
      x: typeof target.x === 'number' && Number.isFinite(target.x) ? target.x : initial.x,
      y: typeof target.y === 'number' && Number.isFinite(target.y) ? target.y : initial.y,
      ratio: Math.max(
        this.minRatio,
        Math.min(
          this.maxRatio,
          typeof target.ratio === 'number' && Number.isFinite(target.ratio) ? target.ratio : initial.ratio
        )
      ),
      angle: typeof target.angle === 'number' && Number.isFinite(target.angle) ? target.angle : initial.angle,
    };

    const requestedDuration = options.duration ?? 750;
    const duration = this.getReducedMotion() || requestedDuration <= 1 ? 1 : requestedDuration;

    if (duration <= 1) {
      camera.setState(destination);
      options.onComplete?.();
      return;
    }

    const easingFn = options.easing === 'cubicOut' ? cubicOut : cubicInOut;
    const startTime = performance.now();
    this.activeFlightCancel = options.onCancel ?? null;

    const step = (now: number) => {
      const rawT = Math.min(1, Math.max(0, (now - startTime) / duration));
      if (rawT >= 1) {
        this.flightFrameId = null;
        this.activeFlightCancel = null;
        camera.setState(destination);
        options.onComplete?.();
        return;
      }

      const eased = easingFn(rawT);
      camera.setState(interpolateCameraStateLog(initial, destination, eased));
      this.flightFrameId = requestAnimationFrame(step);
    };

    this.flightFrameId = requestAnimationFrame(step);
  }

  public handleWheel = (e: WheelCoords): void => {
    const orig = e.original as WheelEvent | undefined;
    const rawDeltaY = typeof orig?.deltaY === 'number'
      ? orig.deltaY
      : (typeof e.delta === 'number' ? -e.delta * 100 : 0);
    if (!rawDeltaY) return;

    e.preventSigmaDefault();
    // Work around Sigma.js v3.0.3 getWheelCoords() shallow-spread bug where
    // preventSigmaDefault() mutates the discarded inner getMouseCoords() object
    // instead of the emitted WheelCoords object checked by MouseCaptor.handleWheel.
    (e as WheelCoords & { sigmaDefaultPrevented?: boolean }).sigmaDefaultPrevented = true;
    orig?.preventDefault?.();

    const wasFlightActive = this.flightFrameId !== null;
    this.cancelFlight();
    if (wasFlightActive || this.onUserWheelInterrupt) {
      this.onUserWheelInterrupt?.();
    }

    const camera = this.sigma.getCamera();
    const currentRatio = Math.max(1e-6, camera.getState().ratio);
    const factor = normalizeWheelDelta(rawDeltaY, orig?.deltaMode ?? 0, Boolean(orig?.ctrlKey), currentRatio);

    // Instant reverse-direction braking: discard opposing residual glide immediately
    let baseTarget = currentRatio;
    if (this.wheelTargetRatio !== null) {
      const existingLogDiff = Math.log(this.wheelTargetRatio / currentRatio);
      const isReversing = (existingLogDiff < 0 && factor > 1) || (existingLogDiff > 0 && factor < 1);
      baseTarget = isReversing ? currentRatio : this.wheelTargetRatio;
    }

    // Bound target lead ahead of currentRatio to eliminate whip-crack runaway
    const rawTarget = computeWheelTargetRatio(baseTarget, factor, this.minRatio, this.maxRatio);
    const clampedLogLead = Math.max(
      -MAX_LOG_LEAD,
      Math.min(MAX_LOG_LEAD, Math.log(rawTarget / currentRatio))
    );
    const nextTarget = Math.max(
      this.minRatio,
      Math.min(this.maxRatio, currentRatio * Math.exp(clampedLogLead))
    );

    if (Math.abs(nextTarget - currentRatio) < 1e-7 && this.wheelTargetRatio === null) {
      return;
    }

    this.wheelCursor = { x: e.x, y: e.y };
    this.wheelTargetRatio = nextTarget;

    if (this.getReducedMotion()) {
      const zoomed = this.sigma.getViewportZoomedState(this.wheelCursor, nextTarget);
      camera.setState(zoomed);
      this.wheelTargetRatio = null;
      return;
    }

    if (this.wheelFrameId === null) {
      this.lastWheelTickTime = performance.now();
      const tick = (now: number) => {
        if (this.wheelTargetRatio === null) {
          this.wheelFrameId = null;
          return;
        }
        const dtMs = Math.max(1, Math.min(50, now - this.lastWheelTickTime));
        this.lastWheelTickTime = now;

        const cur = camera.getState().ratio;
        const target = this.wheelTargetRatio;
        const { nextRatio, settled } = stepSmoothZoomLog(cur, target, dtMs);
        if (settled) {
          camera.setState(this.sigma.getViewportZoomedState(this.wheelCursor, target));
          this.wheelTargetRatio = null;
          this.wheelFrameId = null;
          return;
        }

        camera.setState(this.sigma.getViewportZoomedState(this.wheelCursor, nextRatio));
        this.wheelFrameId = requestAnimationFrame(tick);
      };
      this.wheelFrameId = requestAnimationFrame(tick);
    }
  };

  public destroy(): void {
    this.cancelWheelGlide();
    this.cancelFlight();
  }
}
