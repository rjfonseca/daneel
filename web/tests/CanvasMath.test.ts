import { describe, it, expect, vi } from 'vitest';
import {
  calculateOutwardCalloutGeometry,
  renderNeonCallout,
  drawSectorBeacon,
  renderSectorCallout,
  calculateNebulaCalloutGeometry,
} from '../src/lib/canvasMath';

describe('CanvasMath Outward & Neon Callout Tests', () => {
  const center = { x: 500, y: 500 };

  describe('calculateOutwardCalloutGeometry', () => {
    it('points right and down when satellite is in bottom-right quadrant (x > cx, y > cy)', () => {
      const geom = calculateOutwardCalloutGeometry(600, 650, center.x, center.y, 80);
      expect(geom.dir).toBe(1);
      expect(geom.vDir).toBe(1);
      expect(geom.midX).toBeGreaterThan(600);
      expect(geom.midY).toBeGreaterThan(650);
      expect(geom.endX).toBeGreaterThan(geom.midX);
      expect(geom.endY).toBe(geom.midY);
      expect(geom.textAlign).toBe('center');
      expect(geom.textBaseline).toBe('bottom');
      expect(geom.textAnchorY).toBeLessThan(geom.endY);
    });

    it('points left and down when satellite is in bottom-left quadrant (x < cx, y > cy)', () => {
      const geom = calculateOutwardCalloutGeometry(400, 650, center.x, center.y, 80);
      expect(geom.dir).toBe(-1);
      expect(geom.vDir).toBe(1);
      expect(geom.midX).toBeLessThan(400);
      expect(geom.midY).toBeGreaterThan(650);
      expect(geom.endX).toBeLessThan(geom.midX);
      expect(geom.textAlign).toBe('center');
      expect(geom.textBaseline).toBe('bottom');
      expect(geom.textAnchorY).toBeLessThan(geom.endY);
    });

    it('points left and up when satellite is in top-left quadrant (x < cx, y < cy)', () => {
      const geom = calculateOutwardCalloutGeometry(350, 350, center.x, center.y, 80);
      expect(geom.dir).toBe(-1);
      expect(geom.vDir).toBe(-1);
      expect(geom.midX).toBeLessThan(350);
      expect(geom.midY).toBeLessThan(350);
      expect(geom.endX).toBeLessThan(geom.midX);
      expect(geom.textAlign).toBe('center');
      expect(geom.textBaseline).toBe('bottom');
      expect(geom.textAnchorY).toBeLessThan(geom.endY);
    });

    it('points right and up when satellite is in top-right quadrant (x > cx, y < cy)', () => {
      const geom = calculateOutwardCalloutGeometry(650, 350, center.x, center.y, 80);
      expect(geom.dir).toBe(1);
      expect(geom.vDir).toBe(-1);
      expect(geom.midX).toBeGreaterThan(650);
      expect(geom.midY).toBeLessThan(350);
      expect(geom.endX).toBeGreaterThan(geom.midX);
      expect(geom.textAlign).toBe('center');
      expect(geom.textBaseline).toBe('bottom');
    });
  });

  describe('renderNeonCallout', () => {
    it('draws leader line, glow, and text on CanvasRenderingContext2D', () => {
      const mockCtx = {
        save: vi.fn(),
        restore: vi.fn(),
        beginPath: vi.fn(),
        moveTo: vi.fn(),
        lineTo: vi.fn(),
        stroke: vi.fn(),
        arc: vi.fn(),
        fill: vi.fn(),
        fillText: vi.fn(),
        strokeStyle: '',
        fillStyle: '',
        lineWidth: 0,
        shadowColor: '',
        shadowBlur: 0,
        textAlign: 'left',
        textBaseline: 'alphabetic',
      } as unknown as CanvasRenderingContext2D;

      const geom = calculateOutwardCalloutGeometry(600, 350, center.x, center.y, 70);
      renderNeonCallout(mockCtx, { x: 600, y: 350 }, geom, '[SKILL] AGY', '#c084fc', {
        lineWidth: 1.4,
        glowBlur: 6,
        drawTerminalPoint: true,
      });

      expect(mockCtx.save).toHaveBeenCalled();
      expect(mockCtx.restore).toHaveBeenCalled();
      expect(mockCtx.beginPath).toHaveBeenCalled();
      expect(mockCtx.moveTo).toHaveBeenCalledWith(600, 350);
      expect(mockCtx.lineTo).toHaveBeenCalledWith(geom.midX, geom.midY);
      expect(mockCtx.lineTo).toHaveBeenCalledWith(geom.endX, geom.endY);
      expect(mockCtx.stroke).toHaveBeenCalled();
      expect(mockCtx.arc).toHaveBeenCalledWith(geom.midX, geom.midY, 2, 0, Math.PI * 2);
      expect(mockCtx.fill).toHaveBeenCalled();
      expect(mockCtx.fillText).toHaveBeenCalledWith('[SKILL] AGY', geom.textAnchorX, geom.textAnchorY);
    });
  });

  describe('drawSectorBeacon and renderSectorCallout', () => {
    it('draws a pulsar beacon reticle at sector anchor coordinates', () => {
      const mockCtx = {
        save: vi.fn(),
        restore: vi.fn(),
        beginPath: vi.fn(),
        moveTo: vi.fn(),
        lineTo: vi.fn(),
        closePath: vi.fn(),
        stroke: vi.fn(),
        arc: vi.fn(),
        fill: vi.fn(),
        strokeStyle: '',
        fillStyle: '',
        lineWidth: 0,
        shadowColor: '',
        shadowBlur: 0,
        globalAlpha: 1,
      } as unknown as CanvasRenderingContext2D;

      drawSectorBeacon(mockCtx, { x: 300, y: 400 }, '#38bdf8');
      expect(mockCtx.save).toHaveBeenCalled();
      expect(mockCtx.restore).toHaveBeenCalled();
      expect(mockCtx.beginPath).toHaveBeenCalled();
      expect(mockCtx.arc).toHaveBeenCalledWith(300, 400, 9, 0, Math.PI * 2);
      expect(mockCtx.stroke).toHaveBeenCalled();
      expect(mockCtx.fill).toHaveBeenCalled();
    });

    it('renders cybernetic HUD glass badge with micro-header, title, and telemetry', () => {
      const mockCtx = {
        save: vi.fn(),
        restore: vi.fn(),
        beginPath: vi.fn(),
        moveTo: vi.fn(),
        lineTo: vi.fn(),
        stroke: vi.fn(),
        arc: vi.fn(),
        fill: vi.fn(),
        fillText: vi.fn(),
        rect: vi.fn(),
        roundRect: vi.fn(),
        strokeStyle: '',
        fillStyle: '',
        lineWidth: 0,
        shadowColor: '',
        shadowBlur: 0,
        textAlign: 'left',
        textBaseline: 'alphabetic',
        font: '',
      } as unknown as CanvasRenderingContext2D;

      const geom = calculateNebulaCalloutGeometry(200, 200, 100, false);
      renderSectorCallout(mockCtx, { x: 200, y: 200 }, geom, 'SAAS', '#10b981', {
        repoCount: 4,
        noteCount: 110,
      });

      expect(mockCtx.save).toHaveBeenCalled();
      expect(mockCtx.restore).toHaveBeenCalled();
      expect(mockCtx.stroke).toHaveBeenCalled();
      expect(mockCtx.fill).toHaveBeenCalled();

      // Verifies micro-header '◈ SECTOR', title 'SAAS', and telemetry '4 nebulas · 110 stars' were drawn
      expect(mockCtx.fillText).toHaveBeenCalledWith('◈ SECTOR', expect.any(Number), expect.any(Number));
      expect(mockCtx.fillText).toHaveBeenCalledWith('4 nebulas · 110 stars', expect.any(Number), expect.any(Number));
      expect(mockCtx.fillText).toHaveBeenCalledWith('SAAS', expect.any(Number), expect.any(Number));
    });

    it('renders cybernetic HUD nebula badge with ◈ NEBULA subheader and star count', () => {
      const mockCtx = {
        save: vi.fn(),
        restore: vi.fn(),
        beginPath: vi.fn(),
        moveTo: vi.fn(),
        lineTo: vi.fn(),
        arc: vi.fn(),
        stroke: vi.fn(),
        fill: vi.fn(),
        rect: vi.fn(),
        roundRect: vi.fn(),
        fillText: vi.fn(),
        measureText: vi.fn().mockReturnValue({ width: 60 }),
        strokeStyle: '',
        fillStyle: '',
        lineWidth: 1,
        shadowColor: '',
        shadowBlur: 0,
        textAlign: 'left',
        textBaseline: 'alphabetic',
        font: '',
      } as unknown as CanvasRenderingContext2D;

      const geom = calculateNebulaCalloutGeometry(150, 150, 80, true);
      renderSectorCallout(mockCtx, { x: 150, y: 150 }, geom, 'GATEWAY', '#38bdf8', {
        subHeader: '◈ NEBULA',
        noteCount: 42,
      });

      expect(mockCtx.fillText).toHaveBeenCalledWith('◈ NEBULA', expect.any(Number), expect.any(Number));
      expect(mockCtx.fillText).toHaveBeenCalledWith('42 stars', expect.any(Number), expect.any(Number));
      expect(mockCtx.fillText).toHaveBeenCalledWith('GATEWAY', expect.any(Number), expect.any(Number));
    });
  });
});
