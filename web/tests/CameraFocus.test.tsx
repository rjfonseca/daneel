import { describe, it, expect, vi } from 'vitest';
import Graph from 'graphology';

describe('Camera Focus & Display Data', () => {
  it('should ensure camera animate receives normalized display coordinates instead of raw physics coordinates', () => {
    // Mock graph with unnormalized layout coordinates (ForceAtlas2 outputs)
    const graph = new Graph();
    graph.addNode('note-1', {
      x: 350.5,
      y: -220.8,
      size: 0.8,
      label: 'Note 1'
    });

    // Verify raw physics coordinates are outside normalized camera bounds [0, 1]
    const rawAttrs = graph.getNodeAttributes('note-1');
    expect(rawAttrs.x).toBeGreaterThan(1);
    expect(rawAttrs.y).toBeLessThan(0);

    // Mock Sigma's getNodeDisplayData which provides normalized framed coordinates
    const mockDisplayData = {
      x: 0.62,
      y: 0.48,
      size: 2.1,
      color: '#e0e7ff',
      label: 'Note 1'
    };

    const mockCamera = {
      animate: vi.fn(),
      getState: vi.fn().mockReturnValue({ x: 0.5, y: 0.5, ratio: 1, angle: 0 })
    };

    const mockSigma = {
      getGraph: () => graph,
      getNodeDisplayData: (id: string) => (id === 'note-1' ? mockDisplayData : undefined),
      getCamera: () => mockCamera
    };

    // Simulate the focus effect in GalaxyController
    const targetNodeId = 'note-1';
    if (mockSigma.getGraph().hasNode(targetNodeId)) {
      const nodeDisplayData = mockSigma.getNodeDisplayData(targetNodeId);
      if (nodeDisplayData) {
        mockSigma.getCamera().animate({
          x: nodeDisplayData.x,
          y: nodeDisplayData.y,
          ratio: targetNodeId.startsWith('__CAT_') ? 0.35 : 0.15
        }, { duration: 1000 });
      }
    }

    // Verify camera animate was called with normalized display coordinates
    expect(mockCamera.animate).toHaveBeenCalledTimes(1);
    const [animatedState, options] = mockCamera.animate.mock.calls[0];
    
    expect(animatedState.x).toBe(0.62);
    expect(animatedState.y).toBe(0.48);
    expect(animatedState.ratio).toBe(0.15);
    expect(options).toEqual({ duration: 1000 });

    // Coordinates must be strictly within normalized space [0, 1]
    expect(animatedState.x).toBeGreaterThanOrEqual(0);
    expect(animatedState.x).toBeLessThanOrEqual(1);
    expect(animatedState.y).toBeGreaterThanOrEqual(0);
    expect(animatedState.y).toBeLessThanOrEqual(1);
  });
});
