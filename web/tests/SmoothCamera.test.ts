import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  MIN_CAMERA_RATIO,
  MAX_CAMERA_RATIO,
  MAX_LOG_LEAD,
  MAX_LOG_SPEED,
  cubicOut,
  cubicInOut,
  interpolateCameraStateLog,
  computeAltitudeGainMultiplier,
  normalizeWheelDelta,
  computeWheelTargetRatio,
  stepSmoothZoomLog,
  SmoothCameraController,
} from '../src/lib/galaxy/smoothCamera';

describe('smoothCamera', () => {
  describe('easings', () => {
    it('satisfies boundary conditions at t=0 and t=1', () => {
      expect(cubicOut(0)).toBe(0);
      expect(cubicOut(1)).toBe(1);
      expect(cubicInOut(0)).toBe(0);
      expect(cubicInOut(1)).toBe(1);
      expect(cubicInOut(0.5)).toBeCloseTo(0.5, 6);
    });
  });

  describe('interpolateCameraStateLog', () => {
    it('interpolates ratio exponentially in log-space (geometric mean at midpoint)', () => {
      const from = { x: 0.5, y: 0.5, ratio: 1.0, angle: 0 };
      const to = { x: 0.8, y: 0.2, ratio: 0.04, angle: 0 };

      const mid = interpolateCameraStateLog(from, to, 0.5);
      // Geometric mean of 1.0 and 0.04 is sqrt(0.04) = 0.20 (linear would be 0.52)
      expect(mid.ratio).toBeCloseTo(0.20, 6);

      // Zoom-compensated pan weight w = (0.20 - 1.0) / (0.04 - 1.0) = 0.8333...
      const expectedW = (0.20 - 1.0) / (0.04 - 1.0);
      expect(mid.x).toBeCloseTo(0.5 + (0.8 - 0.5) * expectedW, 5);
      expect(mid.y).toBeCloseTo(0.5 + (0.2 - 0.5) * expectedW, 5);
    });

    it('falls back cleanly to linear pan when ratio is unchanged', () => {
      const from = { x: 0.2, y: 0.3, ratio: 0.25, angle: 0 };
      const to = { x: 0.6, y: 0.7, ratio: 0.25, angle: 0 };

      const mid = interpolateCameraStateLog(from, to, 0.5);
      expect(mid.ratio).toBeCloseTo(0.25, 6);
      expect(mid.x).toBeCloseTo(0.4, 6);
      expect(mid.y).toBeCloseTo(0.5, 6);
    });
  });

  describe('normalizeWheelDelta & computeWheelTargetRatio', () => {
    it('normalizes pixel, line, page, and pinch wheel events into bounded factors', () => {
      const mouseTick = normalizeWheelDelta(-100, 0, false); // zoom in (negative deltaY)
      expect(mouseTick).toBeLessThan(1.0);
      expect(mouseTick).toBeGreaterThan(0.70);

      const trackpadMicro = normalizeWheelDelta(-10, 0, false);
      expect(trackpadMicro).toBeLessThan(1.0);
      expect(trackpadMicro).toBeGreaterThan(mouseTick);

      const lineTick = normalizeWheelDelta(3, 1, false); // zoom out (positive deltaY)
      expect(lineTick).toBeGreaterThan(1.0);

      const pageTick = normalizeWheelDelta(1, 2, false);
      expect(pageTick).toBeGreaterThan(1.0);

      const pinchTick = normalizeWheelDelta(-20, 0, true);
      expect(pinchTick).toBeLessThan(1.0);
    });

    it('accumulates wheel factors smoothly and clamps to [minRatio, maxRatio]', () => {
      const f1 = normalizeWheelDelta(-100, 0, false);
      const r1 = computeWheelTargetRatio(1.0, f1, MIN_CAMERA_RATIO, MAX_CAMERA_RATIO);
      const r2 = computeWheelTargetRatio(r1, f1, MIN_CAMERA_RATIO, MAX_CAMERA_RATIO);
      expect(r2).toBeLessThan(r1);
      expect(r1).toBeLessThan(1.0);

      expect(computeWheelTargetRatio(0.004, 0.5, MIN_CAMERA_RATIO, MAX_CAMERA_RATIO)).toBe(MIN_CAMERA_RATIO);
      expect(computeWheelTargetRatio(1.4, 1.5, MIN_CAMERA_RATIO, MAX_CAMERA_RATIO)).toBe(MAX_CAMERA_RATIO);
    });
  });

  describe('SmoothCameraController', () => {
    beforeEach(() => {
      vi.useFakeTimers();
    });

    afterEach(() => {
      vi.useRealTimers();
    });

    function createMockSigma(initial = { x: 0.5, y: 0.5, ratio: 1.0, angle: 0 }) {
      let state = { ...initial };
      const camera = {
        getState: () => ({ ...state }),
        setState: vi.fn((next: Partial<typeof state>) => {
          state = { ...state, ...next };
        }),
        get ratio() {
          return state.ratio;
        },
      };
      const sigma = {
        getCamera: () => camera,
        getViewportZoomedState: vi.fn((pos: { x: number; y: number }, newRatio: number) => ({
          x: state.x + (pos.x - 500) * 0.0001,
          y: state.y + (pos.y - 400) * 0.0001,
          ratio: newRatio,
          angle: state.angle,
        })),
      };
      return { sigma, camera };
    }

    it('executes reduced-motion animateTo synchronously when duration <= 1', () => {
      const { sigma, camera } = createMockSigma();
      const ctrl = new SmoothCameraController({
        sigma: sigma as never,
        getReducedMotion: () => true,
      });
      const onComplete = vi.fn();

      ctrl.animateTo({ x: 0.7, y: 0.3, ratio: 0.15, angle: 0 }, { duration: 850, onComplete });

      expect(camera.setState).toHaveBeenCalledWith({ x: 0.7, y: 0.3, ratio: 0.15, angle: 0 });
      expect(onComplete).toHaveBeenCalledTimes(1);
      ctrl.destroy();
    });

    it('cancels active programmatic flight when user scrolls with wheel and accumulates momentum', () => {
      const { sigma } = createMockSigma();
      const onFlightCancel = vi.fn();
      const onUserWheelInterrupt = vi.fn();
      const ctrl = new SmoothCameraController({
        sigma: sigma as never,
        getReducedMotion: () => false,
        onUserWheelInterrupt,
      });

      ctrl.animateTo({ x: 0.8, y: 0.8, ratio: 0.1, angle: 0 }, {
        duration: 850,
        onCancel: onFlightCancel,
      });

      const preventSigmaDefault = vi.fn();
      const preventDefault = vi.fn();
      ctrl.handleWheel({
        x: 600,
        y: 350,
        delta: 1,
        sigmaDefaultPrevented: false,
        preventSigmaDefault,
        original: { deltaY: -120, deltaMode: 0, ctrlKey: false, preventDefault } as unknown as WheelEvent,
      });

      expect(preventSigmaDefault).toHaveBeenCalledTimes(1);
      expect(preventDefault).toHaveBeenCalledTimes(1);
      expect(onFlightCancel).toHaveBeenCalledTimes(1);
      expect(onUserWheelInterrupt).toHaveBeenCalledTimes(1);

      // Simulate a couple of animation frames converging toward target ratio
      vi.advanceTimersByTime(32);
      const afterFirstTick = sigma.getCamera().ratio;
      expect(afterFirstTick).toBeLessThan(1.0);

      // Second rapid wheel event accumulates even deeper target ratio without resetting
      ctrl.handleWheel({
        x: 600,
        y: 350,
        delta: 1,
        sigmaDefaultPrevented: false,
        preventSigmaDefault,
        original: { deltaY: -120, deltaMode: 0, ctrlKey: false, preventDefault } as unknown as WheelEvent,
      });
      vi.advanceTimersByTime(500);
      expect(sigma.getCamera().ratio).toBeLessThan(afterFirstTick);
      ctrl.destroy();
    });

    it('tapers altitude gain smoothly, caps velocity & target lead, and brakes instantly on scroll reversal', () => {
      // 1. Altitude deceleration curve
      expect(computeAltitudeGainMultiplier(1.0)).toBeCloseTo(1.0, 2);
      expect(computeAltitudeGainMultiplier(0.2)).toBeLessThan(0.90);
      expect(computeAltitudeGainMultiplier(0.05)).toBeLessThan(0.70);
      expect(computeAltitudeGainMultiplier(0.02)).toBeLessThan(0.60);
      expect(computeAltitudeGainMultiplier(0.004)).toBeCloseTo(0.45, 2);

      // 2. Capped log-velocity per step (no whip-crack overshoot)
      const step = stepSmoothZoomLog(1.0, 0.05, 16, { tauMs: 110, maxLogSpeed: MAX_LOG_SPEED });
      expect(step.settled).toBe(false);
      expect(step.nextRatio).toBeLessThan(1.0);
      expect(Math.abs(Math.log(step.nextRatio / 1.0))).toBeLessThanOrEqual(MAX_LOG_SPEED * 0.016 + 1e-6);

      // 3. Lead clamping & instant reversal braking
      const { sigma } = createMockSigma({ x: 0.5, y: 0.5, ratio: 0.5, angle: 0 });
      const ctrl = new SmoothCameraController({
        sigma: sigma as never,
        getReducedMotion: () => false,
      });
      const preventSigmaDefault = vi.fn();
      const preventDefault = vi.fn();

      // Burst 20 inward wheel ticks -> target lead must stay bounded by MAX_LOG_LEAD
      for (let i = 0; i < 20; i++) {
        ctrl.handleWheel({
          x: 500,
          y: 400,
          delta: 1,
          sigmaDefaultPrevented: false,
          preventSigmaDefault,
          original: { deltaY: -120, deltaMode: 0, ctrlKey: false, preventDefault } as unknown as WheelEvent,
        });
      }
      const maxZoomInTarget = 0.5 * Math.exp(-MAX_LOG_LEAD);
      vi.advanceTimersByTime(16);
      const afterBurstRatio = sigma.getCamera().ratio;
      expect(afterBurstRatio).toBeGreaterThanOrEqual(maxZoomInTarget * 0.99);

      // Reverse scroll direction ONCE -> should brake immediately & move outward (> afterBurstRatio)
      ctrl.handleWheel({
        x: 500,
        y: 400,
        delta: -1,
        sigmaDefaultPrevented: false,
        preventSigmaDefault,
        original: { deltaY: 120, deltaMode: 0, ctrlKey: false, preventDefault } as unknown as WheelEvent,
      });
      vi.advanceTimersByTime(80);
      expect(sigma.getCamera().ratio).toBeGreaterThan(afterBurstRatio);
      ctrl.destroy();
    });

    it('sets sigmaDefaultPrevented directly on the emitted WheelCoords object (Sigma 3.0.3 shallow-spread workaround)', () => {
      const { sigma } = createMockSigma({ x: 0.5, y: 0.5, ratio: 1.0, angle: 0 });
      const ctrl = new SmoothCameraController({
        sigma: sigma as never,
        getReducedMotion: () => false,
      });

      const discardedInner = { sigmaDefaultPrevented: false };
      const brokenWheelEvent = {
        x: 600,
        y: 350,
        delta: 1,
        sigmaDefaultPrevented: false,
        preventSigmaDefault: vi.fn(() => {
          discardedInner.sigmaDefaultPrevented = true;
        }),
        original: { deltaY: -120, deltaMode: 0, ctrlKey: false, preventDefault: vi.fn() } as unknown as WheelEvent,
      };

      ctrl.handleWheel(brokenWheelEvent);

      expect(brokenWheelEvent.preventSigmaDefault).toHaveBeenCalledTimes(1);
      expect(brokenWheelEvent.sigmaDefaultPrevented).toBe(true);
      ctrl.destroy();
    });
  });
});
