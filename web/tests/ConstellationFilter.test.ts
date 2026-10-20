import { describe, it, expect } from 'vitest';
import Graph from 'graphology';
import { computeConstellation } from '../src/lib/constellationFilter';

describe('ConstellationFilter (k-Hop Isolation)', () => {
  it('deve retornar null para nó inexistente ou nulo', () => {
    const graph = new Graph();
    expect(computeConstellation(graph, null)).toBeNull();
    expect(computeConstellation(graph, 'inexistente')).toBeNull();
  });

  it('deve isolar a constelação de 1 e 2 saltos corretamente', () => {
    const graph = new Graph();
    // Monta grafo: Target -> N1 -> N2 -> N3 (3 saltos)
    graph.addNode('target', { title: 'Target' });
    graph.addNode('n1', { title: 'Vizinho 1' });
    graph.addNode('n2', { title: 'Vizinho 2' });
    graph.addNode('n3', { title: 'Distante 3' });
    graph.addNode('solto', { title: 'Desconectado' });

    graph.addEdge('target', 'n1');
    graph.addEdge('n1', 'n2');
    graph.addEdge('n2', 'n3');

    // Com maxHops = 2: Target (hop 0), n1 (hop 1), n2 (hop 2)
    const result = computeConstellation(graph, 'target', 2);
    expect(result).not.toBeNull();
    if (!result) return;

    expect(result.visibleNodes.has('target')).toBe(true);
    expect(result.visibleNodes.has('n1')).toBe(true);
    expect(result.visibleNodes.has('n2')).toBe(true);
    expect(result.visibleNodes.has('n3')).toBe(false); // 3 saltos está fora
    expect(result.visibleNodes.has('solto')).toBe(false);

    expect(result.hopDistances.get('target')).toBe(0);
    expect(result.hopDistances.get('n1')).toBe(1);
    expect(result.hopDistances.get('n2')).toBe(2);

    expect(result.getNodeDimmingAlpha('target')).toBe(1.0);
    expect(result.getNodeDimmingAlpha('n1')).toBe(0.95);
    expect(result.getNodeDimmingAlpha('n2')).toBe(0.70);
    expect(result.getNodeDimmingAlpha('n3')).toBe(0.03);
    expect(result.getNodeDimmingAlpha('solto')).toBe(0.03);
  });

  it('deve ignorar nós de agrupamento __CAT_ na contagem de saltos', () => {
    const graph = new Graph();
    graph.addNode('target', { title: 'Target' });
    graph.addNode('__CAT_ia', { title: 'Nebulosa IA' });
    graph.addNode('peer', { title: 'Peer' });

    graph.addEdge('target', '__CAT_ia');
    graph.addEdge('target', 'peer');

    const result = computeConstellation(graph, 'target', 2);
    expect(result).not.toBeNull();
    if (!result) return;

    expect(result.visibleNodes.has('peer')).toBe(true);
    expect(result.visibleNodes.has('__CAT_ia')).toBe(false);
  });
});
