import { describe, it, expect } from 'vitest';
import Graph from 'graphology';
import { TrafficSimulator } from '../src/lib/trafficSimulation';

describe('TrafficSimulator (Galactic Ships)', () => {
  it('deve inicializar naves ao longo de arestas reais do grafo', () => {
    const graph = new Graph();
    graph.addNode('starA');
    graph.addNode('starB');
    graph.addNode('starC');
    graph.addEdge('starA', 'starB');
    graph.addEdge('starB', 'starC');

    const sim = new TrafficSimulator(graph, 10);
    const ships = sim.getShips();

    expect(ships).toHaveLength(10);
    ships.forEach((s) => {
      expect(['starA', 'starB', 'starC']).toContain(s.source);
      expect(['starA', 'starB', 'starC']).toContain(s.target);
      expect(s.progress).toBeGreaterThanOrEqual(0);
      expect(s.progress).toBeLessThanOrEqual(1);
    });
  });

  it('deve avançar o progresso das naves ao atualizar deltaTime', () => {
    const graph = new Graph();
    graph.addNode('starA');
    graph.addNode('starB');
    graph.addEdge('starA', 'starB');

    const sim = new TrafficSimulator(graph, 1);
    const ship = sim.getShips()[0];
    const initialProgress = ship.progress;

    sim.update(0.5); // avança 0.5s
    expect(ship.progress).not.toBe(initialProgress);
  });

  it('deve utilizar posições de tela da viewport e velocidade constante em pixels', () => {
    const graph = new Graph();
    graph.addNode('starA');
    graph.addNode('starB');
    graph.addEdge('starA', 'starB');

    const sim = new TrafficSimulator(graph, 1);
    const viewportPositions: Record<string, { x: number; y: number }> = {
      starA: { x: 100, y: 100 },
      starB: { x: 500, y: 100 }, // distância 400px
    };

    const getPos = (id: string) => viewportPositions[id] || null;

    // Atualiza com viewport
    sim.update(0.1, getPos, 1000, 800);
    const ships = sim.getShips();
    expect(ships.length).toBeGreaterThan(0);
    expect(ships[0].screenDistPx).toBe(400);
  });

  it('filters out photons when either endpoint node is not visible', () => {
    const graph = new Graph();
    graph.addNode('starA');
    graph.addNode('starB');
    graph.addNode('starC');
    graph.addEdge('starA', 'starB');
    graph.addEdge('starB', 'starC');

    const sim = new TrafficSimulator(graph, 10);
    const getPos = () => ({ x: 100, y: 100 });

    // Only starA is visible; starB and starC are hidden by constellation filter
    const isVisible = (id: string) => id === 'starA';

    sim.update(0.1, getPos, 1000, 800, isVisible);
    // All ships should be filtered out because no edge has BOTH endpoints visible
    expect(sim.getShips()).toHaveLength(0);
  });
});

