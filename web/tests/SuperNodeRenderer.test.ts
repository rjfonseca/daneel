import { describe, it, expect, vi } from 'vitest';
import { formatCompactCount, renderAggregatedSuperEdges, renderCollapsedSuperNodes } from '../src/lib/superNodeRenderer';
import type { SuperNodeMeta } from '../src/lib/semanticZoomLod';
import type { ActiveSuperEdge } from '../src/lib/superEdges';

function createMockCtx() {
  return {
    save: vi.fn(),
    restore: vi.fn(),
    beginPath: vi.fn(),
    arc: vi.fn(),
    moveTo: vi.fn(),
    lineTo: vi.fn(),
    quadraticCurveTo: vi.fn(),
    stroke: vi.fn(),
    fill: vi.fn(),
    fillText: vi.fn(),
    setLineDash: vi.fn(),
    roundRect: vi.fn(),
    rect: vi.fn(),
    measureText: vi.fn((text: string) => ({ width: text.length * 6 })),
    createRadialGradient: vi.fn(() => ({ addColorStop: vi.fn() })),
    fillStyle: '',
    strokeStyle: '',
    lineWidth: 1,
    shadowColor: '',
    shadowBlur: 0,
    font: '',
    textAlign: 'center' as CanvasTextAlign,
    textBaseline: 'middle' as CanvasTextBaseline,
  } as unknown as CanvasRenderingContext2D;
}

describe('superNodeRenderer', () => {
  it('formats compact counts cleanly', () => {
    expect(formatCompactCount(42)).toBe('42');
    expect(formatCompactCount(964)).toBe('964');
    expect(formatCompactCount(2418)).toBe('2.4k');
    expect(formatCompactCount(12218)).toBe('12k');
  });

  it('renders collapsed super-nodes within viewport without drawing floating count pills', () => {
    const ctx = createMockCtx();
    const nodes: SuperNodeMeta[] = [
      { id: '__SECTOR_acme', kind: 'sector', key: 'acme', sectorKey: 'acme', displayName: 'ACME', color: '#2dd4bf', trustScope: 'sources', noteCount: 2418, repoCount: 42, recentCount: 12, x: 100, y: 100 },
      { id: '__SECTOR_off', kind: 'sector', key: 'off', sectorKey: 'off', displayName: 'OFF', color: '#f87171', trustScope: 'sources', noteCount: 500, repoCount: 5, recentCount: 0, x: 5000, y: 5000 },
    ];
    const getPos = (id: string) => (id === '__SECTOR_acme' ? { x: 400, y: 300 } : { x: 2500, y: 2500 });

    renderCollapsedSuperNodes(ctx, nodes, getPos, {
      hoveredNodeId: '__SECTOR_acme',
      viewportWidth: 1280,
      viewportHeight: 800,
    });

    expect(ctx.arc).toHaveBeenCalledWith(400, 300, expect.any(Number), 0, Math.PI * 2);
    expect(ctx.stroke).toHaveBeenCalled();
    expect(ctx.arc).not.toHaveBeenCalledWith(2500, 2500, expect.any(Number), 0, Math.PI * 2);
    expect(ctx.fillText).not.toHaveBeenCalled();
  });

  it('respects edgeMode when rendering aggregated super-edges and shows link badge on hover', () => {
    const bridges: ActiveSuperEdge[] = [
      { id: 'a--b', sourceNodeId: '__CAT_notes/arch', targetNodeId: '__SECTOR_acme', weight: 14, color: '#2dd4bf', involvesCurated: true },
    ];
    const getPos = (id: string) => (id.includes('arch') ? { x: 200, y: 200 } : { x: 600, y: 200 });

    // Focus mode without hover renders nothing
    const ctxFocusEmpty = createMockCtx();
    renderAggregatedSuperEdges(ctxFocusEmpty, bridges, getPos, { edgeMode: 'focus', hoveredNodeId: null });
    expect(ctxFocusEmpty.quadraticCurveTo).not.toHaveBeenCalled();

    // Focus mode with matching hover renders curved bridge and count pill
    const ctxFocusHovered = createMockCtx();
    renderAggregatedSuperEdges(ctxFocusHovered, bridges, getPos, { edgeMode: 'focus', hoveredNodeId: '__SECTOR_acme' });
    expect(ctxFocusHovered.quadraticCurveTo).toHaveBeenCalledTimes(1);
    expect(ctxFocusHovered.fillText).toHaveBeenCalledWith('14 links', expect.any(Number), expect.any(Number));
  });
});
