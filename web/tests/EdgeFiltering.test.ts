import { describe, it, expect } from 'vitest';
import {
  computeEdgeDisplay,
  createEdgeReducer,
  EdgeFilterContext,
  EdgeData,
} from '../src/lib/edgeFiltering';

describe('EdgeFiltering', () => {
  const defaultContext: EdgeFilterContext = {
    cameraRatio: 1.0,
    mode: 'auto',
    hoveredNodeId: null,
    selectedNodeId: null,
    focusedCategory: null,
  };

  const intraEdge: EdgeData = {
    isCrossCluster: false,
    sourceCategory: 'arquitetura',
    targetCategory: 'arquitetura',
  };

  const crossEdge: EdgeData = {
    isCrossCluster: true,
    sourceCategory: 'arquitetura',
    targetCategory: 'linguagens',
  };

  describe('computeEdgeDisplay', () => {
    it('always hides anchor edges connected to __CAT_ centroids', () => {
      const anchorEdge: EdgeData = { originalHidden: true };
      const res1 = computeEdgeDisplay(anchorEdge, '__CAT_arquitetura', 'notes/arch.md', defaultContext);
      expect(res1.hidden).toBe(true);

      const res2 = computeEdgeDisplay(intraEdge, 'notes/arch.md', '__CAT_arquitetura', defaultContext);
      expect(res2.hidden).toBe(true);
    });

    it('flares up connected edges when a node is hovered or selected', () => {
      const hoverCtx: EdgeFilterContext = {
        ...defaultContext,
        hoveredNodeId: 'node-a',
      };

      const result = computeEdgeDisplay(intraEdge, 'node-a', 'node-b', hoverCtx);
      expect(result.hidden).toBe(false);
      expect(result.color).toBe('rgba(186, 230, 253, 0.40)');
      expect(result.size).toBe(0.25);
      expect(result.zIndex).toBe(10);
    });

    it('handles "bridges" mode by hiding intra-cluster edges and showing cross-cluster edges', () => {
      const bridgesCtx: EdgeFilterContext = {
        ...defaultContext,
        mode: 'bridges',
      };

      const intraRes = computeEdgeDisplay(intraEdge, 'node-a', 'node-b', bridgesCtx);
      expect(intraRes.hidden).toBe(true);

      const crossRes = computeEdgeDisplay(crossEdge, 'node-a', 'node-c', bridgesCtx);
      expect(crossRes.hidden).toBe(false);
      expect(crossRes.size).toBe(0.12);
    });

    it('handles "focus" mode by highlighting the focused category edges and hiding others', () => {
      const focusCtx: EdgeFilterContext = {
        ...defaultContext,
        mode: 'focus',
        focusedCategory: 'arquitetura',
      };

      const inSectorRes = computeEdgeDisplay(intraEdge, 'node-a', 'node-b', focusCtx);
      expect(inSectorRes.hidden).toBe(false);
      expect(inSectorRes.color).toContain('rgba(147, 197, 253');

      const otherEdge: EdgeData = {
        isCrossCluster: false,
        sourceCategory: 'database',
        targetCategory: 'database',
      };
      const outSectorRes = computeEdgeDisplay(otherEdge, 'node-x', 'node-y', focusCtx);
      expect(outSectorRes.hidden).toBe(true);
    });

    it('handles "all" mode by rendering all valid edges', () => {
      const allCtx: EdgeFilterContext = {
        ...defaultContext,
        mode: 'all',
      };

      const res = computeEdgeDisplay(intraEdge, 'node-a', 'node-b', allCtx);
      expect(res.hidden).toBe(false);
      expect(res.size).toBe(0.10);
    });

    describe('Progressive Disclosure (mode = "auto")', () => {
      it('at Overview zoom (cameraRatio >= 0.65): hides intra-cluster edges to eliminate the white blob', () => {
        const overviewCtx: EdgeFilterContext = {
          ...defaultContext,
          cameraRatio: 0.95,
          mode: 'auto',
        };

        const intraRes = computeEdgeDisplay(intraEdge, 'node-a', 'node-b', overviewCtx);
        expect(intraRes.hidden).toBe(true);

        const majorCrossEdge: EdgeData = {
          ...crossEdge,
          sourceDegree: 15,
        };
        const crossRes = computeEdgeDisplay(majorCrossEdge, 'node-a', 'node-c', overviewCtx);
        expect(crossRes.hidden).toBe(false);
        expect(crossRes.size).toBe(0.10);
      });

      it('at Intermediate zoom (0.35 <= cameraRatio < 0.65): shows sector details or faint outlines', () => {
        const midCtx: EdgeFilterContext = {
          ...defaultContext,
          cameraRatio: 0.50,
          mode: 'auto',
          focusedCategory: 'arquitetura',
        };

        const inSectorRes = computeEdgeDisplay(intraEdge, 'node-a', 'node-b', midCtx);
        expect(inSectorRes.hidden).toBe(false);
        expect(inSectorRes.size).toBe(0.18);
      });

      it('at Deep zoom (cameraRatio < 0.35): renders all edges with high contrast', () => {
        const deepCtx: EdgeFilterContext = {
          ...defaultContext,
          cameraRatio: 0.20,
          mode: 'auto',
        };

        const intraRes = computeEdgeDisplay(intraEdge, 'node-a', 'node-b', deepCtx);
        expect(intraRes.hidden).toBe(false);
        expect(intraRes.size).toBe(0.12);

        const crossRes = computeEdgeDisplay(crossEdge, 'node-a', 'node-c', deepCtx);
        expect(crossRes.hidden).toBe(false);
        expect(crossRes.size).toBe(0.16);
      });
    });
  });

  describe('createEdgeReducer', () => {
    it('creates a reducer function compatible with Sigma', () => {
      const endpointsMap = new Map([
        ['edge-1', { source: 'node-a', target: 'node-b' }],
      ]);

      const reducer = createEdgeReducer(
        (id) => endpointsMap.get(id),
        () => ({ ...defaultContext, cameraRatio: 0.8 })
      );

      const reduced = reducer('edge-1', {
        ...intraEdge,
        label: 'test',
      });

      expect(reduced.hidden).toBe(true);
      expect(reduced.label).toBe('test');
    });
  });
});
