/**
 * @file constellationFilter.ts
 * Módulo para cálculo de isolamento de constelação (k-hop neighborhood).
 * Ao focar em uma nota central, isola a vizinhança estelar a 1 ou 2 saltos,
 * atenuando todo o ruído de fundo da galáxia.
 */

import type Graph from 'graphology';

export interface ConstellationResult {
  centerNodeId: string;
  visibleNodes: Set<string>;
  visibleEdges: Set<string>;
  hopDistances: Map<string, number>;
  isNodeInConstellation: (nodeId: string) => boolean;
  isEdgeInConstellation: (edgeId: string) => boolean;
  getNodeDimmingAlpha: (nodeId: string) => number;
}

/**
 * Calcula a constelação ao redor de uma estrela alvo até `maxHops` saltos.
 * Nós fora da constelação recebem opacidade quase nula (0.04) para fade-out cósmico.
 */
export function computeConstellation(
  graph: Graph,
  targetNodeId: string | null | undefined,
  maxHops: number = 2
): ConstellationResult | null {
  if (!targetNodeId || !graph.hasNode(targetNodeId)) {
    return null;
  }

  const visibleNodes = new Set<string>();
  const visibleEdges = new Set<string>();
  const hopDistances = new Map<string, number>();

  // BFS para extrair nós até maxHops
  const queue: Array<{ id: string; hop: number }> = [{ id: targetNodeId, hop: 0 }];
  visibleNodes.add(targetNodeId);
  hopDistances.set(targetNodeId, 0);

  while (queue.length > 0) {
    const current = queue.shift()!;
    if (current.hop >= maxHops) {
      continue;
    }

    graph.forEachNeighbor(current.id, (neighbor: string) => {
      // Ignora nós fictícios de nebulosa (__CAT_) para a constelação conceitual
      if (neighbor.startsWith('__CAT_')) return;

      if (!visibleNodes.has(neighbor)) {
        visibleNodes.add(neighbor);
        hopDistances.set(neighbor, current.hop + 1);
        queue.push({ id: neighbor, hop: current.hop + 1 });
      }
    });
  }

  // Identifica arestas que conectam nós da constelação
  visibleNodes.forEach((nodeId) => {
    graph.forEachEdge(nodeId, (edge: string, _attributes: Record<string, unknown>, source: string, target: string) => {
      if (visibleNodes.has(source) && visibleNodes.has(target)) {
        visibleEdges.add(edge);
      }
    });
  });

  return {
    centerNodeId: targetNodeId,
    visibleNodes,
    visibleEdges,
    hopDistances,
    isNodeInConstellation: (nodeId: string) => visibleNodes.has(nodeId),
    isEdgeInConstellation: (edgeId: string) => visibleEdges.has(edgeId),
    getNodeDimmingAlpha: (nodeId: string) => {
      if (nodeId === targetNodeId) return 1.0;
      const hop = hopDistances.get(nodeId);
      if (hop === 1) return 0.95;
      if (hop === 2) return 0.70;
      return 0.03; // Fade out estelar profundo para nós não pertencentes à constelação
    },
  };
}
