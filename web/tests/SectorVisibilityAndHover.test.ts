import { describe, it, expect, vi } from 'vitest';
import Graph from 'graphology';
import { applySectorVisibilityFilter } from '../src/lib/galaxy/sectorVisibility';
import { HoverController } from '../src/lib/galaxy/hoverController';
import type { SigmaContextState } from '../src/types/galaxy';

describe('sectorVisibility & hoverController', () => {
  describe('applySectorVisibilityFilter', () => {
    it('batches node and edge hidden updates using updateEach* instead of N individual setAttribute calls', () => {
      const graph = new Graph();
      graph.addNode('__SECTOR_acme', { hidden: false });
      graph.addNode('__SECTOR_far', { hidden: false });
      graph.addNode('__CAT_sources/github/acme/orion', { hidden: false, sectorKey: 'acme', clusterKey: 'sources/github/acme/orion' });
      graph.addNode('sources/github/acme/orion/a.md', { hidden: false, originalCategory: 'sources/github/acme/orion', sectorKey: 'acme', clusterKey: 'sources/github/acme/orion' });
      graph.addNode('notes/architecture/x.md', { hidden: false, originalCategory: 'notes/architecture', sectorKey: 'notes/architecture', clusterKey: 'notes/architecture' });
      graph.addEdge('sources/github/acme/orion/a.md', 'notes/architecture/x.md', { hidden: false, originalHidden: false });

      const setNodeSpy = vi.spyOn(graph, 'setNodeAttribute');
      const setEdgeSpy = vi.spyOn(graph, 'setEdgeAttribute');
      const updateEachNodeSpy = vi.spyOn(graph, 'updateEachNodeAttributes');
      const updateEachEdgeSpy = vi.spyOn(graph, 'updateEachEdgeAttributes');

      applySectorVisibilityFilter(graph, '__SECTOR_acme', null);

      expect(updateEachNodeSpy).toHaveBeenCalledTimes(1);
      expect(updateEachEdgeSpy).toHaveBeenCalledTimes(1);
      expect(setNodeSpy).not.toHaveBeenCalled();
      expect(setEdgeSpy).not.toHaveBeenCalled();

      expect(graph.getNodeAttribute('__SECTOR_acme', 'hidden')).toBe(false);
      expect(graph.getNodeAttribute('__SECTOR_far', 'hidden')).toBe(true);
      expect(graph.getNodeAttribute('sources/github/acme/orion/a.md', 'hidden')).toBe(false);
      expect(graph.getNodeAttribute('notes/architecture/x.md', 'hidden')).toBe(true);

      // Restores all nodes cleanly when focus clears
      applySectorVisibilityFilter(graph, null, null);
      expect(graph.getNodeAttribute('__SECTOR_far', 'hidden')).toBe(false);
      expect(graph.getNodeAttribute('notes/architecture/x.md', 'hidden')).toBe(false);
    });

    it('keeps nested GitLab sector stars visible even when star sectorKey holds raw backend group', () => {
      const graph = new Graph();
      graph.addNode('__SECTOR_ai', { hidden: false });
      graph.addNode('__CAT_sources/gitlab/acme/ai/wiki/hugin', {
        hidden: false,
        sectorKey: 'acme/ai/wiki',
        clusterKey: 'sources/gitlab/acme/ai/wiki/hugin',
      });
      graph.addNode('sources/gitlab/acme/ai/wiki/hugin/AGENTS.md', {
        hidden: false,
        originalCategory: 'sources/gitlab/acme/ai/wiki/hugin',
        sectorKey: 'acme/ai/wiki',
        clusterKey: 'sources/gitlab/acme/ai/wiki/hugin',
      });

      applySectorVisibilityFilter(graph, '__SECTOR_ai', null);

      expect(graph.getNodeAttribute('__SECTOR_ai', 'hidden')).toBe(false);
      expect(graph.getNodeAttribute('__CAT_sources/gitlab/acme/ai/wiki/hugin', 'hidden')).toBe(false);
      expect(graph.getNodeAttribute('sources/gitlab/acme/ai/wiki/hugin/AGENTS.md', 'hidden')).toBe(false);
    });
  });

  describe('HoverController', () => {
    it('highlights neighbors via sigmaContext and refreshes partialGraph without mutating node sizes in graphology', () => {
      const graph = new Graph();
      graph.addNode('n1', { x: 0, y: 0, size: 2, label: 'Star 1', color: '#38bdf8' });
      graph.addNode('n2', { x: 10, y: 10, size: 2, label: 'Star 2', color: '#38bdf8' });
      graph.addEdge('n1', 'n2', { hidden: false });

      const setNodeSpy = vi.spyOn(graph, 'setNodeAttribute');
      const refresh = vi.fn();
      const sigma = {
        getGraph: () => graph,
        graphToViewport: () => ({ x: 100, y: 200 }),
        refresh,
      };

      const ctx = {
        hoveredNodeId: null,
        hoveredNeighborIds: new Set<string>(),
        lodState: null,
        superNodeRegistry: new Map(),
        superEdgeIndex: null,
      } as unknown as SigmaContextState;

      const onHover = vi.fn();
      const controller = new HoverController({
        sigma: sigma as never,
        context: ctx,
        onHover,
        getColor: () => '#38bdf8',
      });

      controller.handleEnterNode('n1');

      expect(ctx.hoveredNodeId).toBe('n1');
      expect(ctx.hoveredNeighborIds!.has('n2')).toBe(true);
      expect(setNodeSpy).not.toHaveBeenCalled();
      expect(refresh).toHaveBeenCalledWith(expect.objectContaining({
        partialGraph: expect.objectContaining({
          nodes: expect.arrayContaining(['n1', 'n2']),
        }),
        schedule: true,
      }));
      expect(onHover).toHaveBeenCalledWith(expect.objectContaining({ id: 'n1' }), { x: 100, y: 200 });

      controller.handleLeaveNode();
      expect(ctx.hoveredNodeId).toBeNull();
      expect(ctx.hoveredNeighborIds!.size).toBe(0);
      expect(onHover).toHaveBeenLastCalledWith(null, null);
    });

    it('clears hovered tooltip, neighbors, and pointer cursor explicitly via clear()', () => {
      const graph = new Graph();
      graph.addNode('n1', { x: 0, y: 0, size: 2, label: 'Star 1', color: '#38bdf8' });
      const container = { style: { cursor: '' } };
      const refresh = vi.fn();
      const sigma = {
        getGraph: () => graph,
        getContainer: () => container,
        graphToViewport: () => ({ x: 50, y: 60 }),
        refresh,
      };
      const ctx = {
        hoveredNodeId: null,
        hoveredNeighborIds: new Set<string>(),
        lodState: null,
        superNodeRegistry: new Map(),
        superEdgeIndex: null,
      } as unknown as SigmaContextState;
      const onHover = vi.fn();
      const controller = new HoverController({
        sigma: sigma as never,
        context: ctx,
        onHover,
        getColor: () => '#38bdf8',
      });

      controller.handleEnterNode('n1');
      expect(ctx.hoveredNodeId).toBe('n1');
      expect(container.style.cursor).toBe('pointer');

      controller.clear();
      expect(ctx.hoveredNodeId).toBeNull();
      expect(ctx.hoveredNeighborIds!.size).toBe(0);
      expect(onHover).toHaveBeenLastCalledWith(null, null);
      expect(container.style.cursor).toBe('');
    });
  });
});
