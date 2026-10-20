/**
 * @file trafficSimulation.ts
 * Simulação de tráfego cósmico adaptativo ("fótons de dados / naves estelares")
 * trafegando ao longo das arestas visíveis no viewport da galáxia.
 *
 * Princípios Adaptativos:
 * 1. Velocidade constante em pixels de tela (~380 a 520 px/s): a velocidade aparente
 *    é sempre rápida e viva, não importando se a visualização está em zoom geral ou aproximado.
 * 2. Viewport-Aware: Fótons surgem dinamicamente nas arestas atualmente renderizadas
 *    na janela de visão do usuário, eliminando zonas mortas.
 * 3. Eye-candy efêmero: Ao alcançar o nó destino, o fóton se dissolve no núcleo estelar
 *    e novos fótons são gerados continuamente para manter a galáxia vibrante.
 */

import type Graph from 'graphology';

export interface SpaceShip {
  id: number;
  source: string;
  target: string;
  progress: number; // 0.0 a 1.0
  speed: number;    // velocidade normalizada por segundo (1 / durationSec)
  color: string;
  size: number;
  durationSec: number; // Tempo de trânsito estritamente controlado (1.4s a 2.6s independente do zoom)
  hops?: number;       // Saltos consecutivos em clusters densos
  screenDistPx?: number;
  currentDistPx?: number;
  speedPxPerSec?: number;
  reverse?: boolean;
}

export class TrafficSimulator {
  private ships: SpaceShip[] = [];
  private graph: Graph | null = null;
  private nextId = 1;
  private candidateEdges: Array<{ source: string; target: string }> = [];
  private targetPoolSize = 10;

  private photonColors = [
    'rgba(56, 189, 248, 0.95)',  // Ciano / Azul Estelar
    'rgba(168, 85, 247, 0.95)', // Violeta Plasma
    'rgba(52, 211, 153, 0.95)',  // Esmeralda Quântico
    'rgba(251, 191, 36, 0.95)',  // Dourado Solar
    'rgba(244, 114, 182, 0.95)', // Rosa Choque Estelar
  ];

  constructor(graph?: Graph, shipCount: number = 10) {
    this.targetPoolSize = shipCount;
    if (graph) {
      this.init(graph, shipCount);
    }
  }

  public init(graph: Graph, shipCount: number = 10): void {
    this.graph = graph;
    this.ships = [];
    this.targetPoolSize = shipCount;
    this.candidateEdges = [];

    graph.forEachEdge((_edge, _attrs, source, target) => {
      // Ignora nós fictícios de categorias de nebulosa
      if (!source.startsWith('__CAT_') && !target.startsWith('__CAT_')) {
        this.candidateEdges.push({ source, target });
      }
    });

    if (this.candidateEdges.length === 0) return;

    // Inicialização com tempos de trânsito suaves (velocidade reduzida pela metade)
    for (let i = 0; i < shipCount; i++) {
      const edge = this.candidateEdges[Math.floor(Math.random() * this.candidateEdges.length)];
      const reverse = Math.random() > 0.5;
      const durationSec = 3.2 + Math.random() * 2.4;
      this.ships.push({
        id: this.nextId++,
        source: reverse ? edge.target : edge.source,
        target: reverse ? edge.source : edge.target,
        progress: Math.random(),
        speed: 1 / durationSec,
        durationSec,
        hops: 0,
        color: this.photonColors[i % this.photonColors.length],
        size: 1.8 + Math.random() * 0.8,
        speedPxPerSec: 100,
        screenDistPx: 200,
        currentDistPx: Math.random() * 200,
      });
    }
  }

  /**
   * Atualização com tempo de viagem estritamente constante por aresta.
   * Independente do zoom da câmera, o fóton sempre leva entre 1.4s e 2.6s para atravessar.
   * Em clusters densos, pode saltar consecutivamente para o nó vizinho.
   */
  public update(
    deltaTimeSec: number,
    getNodeViewportPos?: (nodeId: string) => { x: number; y: number } | null,
    viewportWidth: number = typeof window !== 'undefined' ? window.innerWidth : 1280,
    viewportHeight: number = typeof window !== 'undefined' ? window.innerHeight : 800,
    isNodeVisible?: (nodeId: string) => boolean
  ): void {
    if (!this.graph || this.candidateEdges.length === 0) return;

    const margin = 80;
    const minX = -margin;
    const maxX = viewportWidth + margin;
    const minY = -margin;
    const maxY = viewportHeight + margin;

    // 1. Atualiza e filtra fótons ativos
    const survivingShips: SpaceShip[] = [];

    for (let i = 0; i < this.ships.length; i++) {
      const ship = this.ships[i];

      // Se qualquer um dos nós estiver oculto pelo filtro de constelação ou setor, descarta o fóton
      if (isNodeVisible && (!isNodeVisible(ship.source) || !isNodeVisible(ship.target))) {
        continue;
      }
      if (this.graph && this.graph.hasEdge(ship.source, ship.target)) {
        if (this.graph.getEdgeAttribute(ship.source, ship.target, 'hidden') === true) {
          continue;
        }
      }

      if (getNodeViewportPos) {
        const p1 = getNodeViewportPos(ship.source);
        const p2 = getNodeViewportPos(ship.target);

        // Se algum nó sumiu ou ambos estão totalmente fora do campo de visão
        if (!p1 || !p2) continue;
        const p1Off = p1.x < minX || p1.x > maxX || p1.y < minY || p1.y > maxY;
        const p2Off = p2.x < minX || p2.x > maxX || p2.y < minY || p2.y > maxY;
        if (p1Off && p2Off) continue;

        const screenDist = Math.hypot(p2.x - p1.x, p2.y - p1.y);
        ship.screenDistPx = Math.max(10, screenDist);

        // Função de velocidade baseada em tempo de trânsito constante suave (3.0s - 5.5s)
        const duration = ship.durationSec || 3.5;
        ship.progress += deltaTimeSec / duration;

        // Se chegou ao nó de destino
        if (ship.progress >= 1.0) {
          const currentHops = ship.hops || 0;
          // Salto contínuo sutil em clusters densos (apenas 1 salto adicional, 25% de chance)
          if (currentHops < 1 && Math.random() < 0.25 && this.graph.hasNode(ship.target)) {
            const neighbors = this.graph.neighbors(ship.target).filter(
              n => !n.startsWith('__CAT_') && !n.startsWith('__SECTOR_') && n !== ship.source && (!isNodeVisible || isNodeVisible(n))
            );
            if (neighbors.length > 0) {
              const nextTarget = neighbors[Math.floor(Math.random() * neighbors.length)];
              ship.source = ship.target;
              ship.target = nextTarget;
              ship.progress = 0.0;
              ship.hops = currentHops + 1;
              ship.durationSec = 3.0 + Math.random() * 2.0; // 3.0s a 5.0s
              survivingShips.push(ship);
              continue;
            }
          }
          // Dissolve no núcleo estelar
          continue;
        }

        survivingShips.push(ship);
      } else {
        // Fallback sem posições de tela (ex: testes unitários padrão)
        const duration = ship.durationSec || 3.5;
        ship.progress += deltaTimeSec / duration;
        if (ship.progress < 1.0) {
          survivingShips.push(ship);
        }
      }
    }

    this.ships = survivingShips;

    // 2. Repovoa o pool de fótons de forma orgânica (no máximo 1 novo fóton por frame)
    if (getNodeViewportPos && this.ships.length < this.targetPoolSize) {
      const needed = this.targetPoolSize - this.ships.length;
      let attempts = 0;
      let spawned = 0;

      while (spawned < needed && spawned < 1 && attempts < 16) {
        attempts++;
        const edge = this.candidateEdges[Math.floor(Math.random() * this.candidateEdges.length)];
        
        // Não gera fótons em arestas com nós ou arestas ocultas pela constelação ou setor
        if (isNodeVisible && (!isNodeVisible(edge.source) || !isNodeVisible(edge.target))) {
          continue;
        }
        if (this.graph && this.graph.hasEdge(edge.source, edge.target)) {
          if (this.graph.getEdgeAttribute(edge.source, edge.target, 'hidden') === true) {
            continue;
          }
        }

        const p1 = getNodeViewportPos(edge.source);
        const p2 = getNodeViewportPos(edge.target);

        if (!p1 || !p2) continue;

        const p1In = p1.x >= minX && p1.x <= maxX && p1.y >= minY && p1.y <= maxY;
        const p2In = p2.x >= minX && p2.x <= maxX && p2.y >= minY && p2.y <= maxY;

        if (!p1In && !p2In) continue;

        const screenDist = Math.hypot(p2.x - p1.x, p2.y - p1.y);
        // Evita arestas microscópicas irrelevantes (< 8px) ou gigantescas fora de escala (> 1200px)
        if (screenDist < 8 || screenDist > 1200) continue;

        const reverse = Math.random() > 0.5;
        const durationSec = 3.2 + Math.random() * 2.2; // Garante 3.2s a 5.4s por travessia (velocidade pela metade)
        this.ships.push({
          id: this.nextId++,
          source: reverse ? edge.target : edge.source,
          target: reverse ? edge.source : edge.target,
          progress: 0,
          speed: 1 / durationSec,
          durationSec,
          hops: 0,
          color: this.photonColors[Math.floor(Math.random() * this.photonColors.length)],
          size: 1.8 + Math.random() * 0.8,
          screenDistPx: screenDist,
          currentDistPx: 0,
        });

        spawned++;
      }
    }
  }

  public getShips(): SpaceShip[] {
    return this.ships;
  }

  public render(
    ctx: CanvasRenderingContext2D,
    getNodeViewportPos: (nodeId: string) => { x: number; y: number } | null,
    isNodeVisible?: (nodeId: string) => boolean
  ): void {
    if (this.ships.length === 0) return;

    ctx.save();

    for (let i = 0; i < this.ships.length; i++) {
      const ship = this.ships[i];

      // Oculta fótons de nós ou arestas fora da constelação ou setor isolado
      if (isNodeVisible && (!isNodeVisible(ship.source) || !isNodeVisible(ship.target))) {
        continue;
      }
      if (this.graph && this.graph.hasEdge(ship.source, ship.target)) {
        if (this.graph.getEdgeAttribute(ship.source, ship.target, 'hidden') === true) {
          continue;
        }
      }

      const p1 = getNodeViewportPos(ship.source);
      const p2 = getNodeViewportPos(ship.target);
      if (!p1 || !p2) continue;

      const currentX = p1.x + (p2.x - p1.x) * ship.progress;
      const currentY = p1.y + (p2.y - p1.y) * ship.progress;

      // Cauda luminosa de fóton de comprimento adaptativo em pixels (~22px)
      const dist = ship.screenDistPx || Math.hypot(p2.x - p1.x, p2.y - p1.y);
      const trailFraction = dist > 0 ? Math.min(0.25, 24 / dist) : 0.08;
      const tailProgress = Math.max(0, ship.progress - trailFraction);

      const tailX = p1.x + (p2.x - p1.x) * tailProgress;
      const tailY = p1.y + (p2.y - p1.y) * tailProgress;

      // Opacidade suave de fade in / fade out nas pontas
      const alpha = Math.sin(ship.progress * Math.PI);

      // Traço de cauda do feixe
      ctx.beginPath();
      ctx.moveTo(tailX, tailY);
      ctx.lineTo(currentX, currentY);
      ctx.strokeStyle = ship.color.replace('0.95', `${(alpha * 0.6).toFixed(2)}`);
      ctx.lineWidth = ship.size * 0.9;
      ctx.stroke();

      // Ponto de luz frontal (núcleo da nave / fóton)
      ctx.beginPath();
      ctx.arc(currentX, currentY, ship.size, 0, 2 * Math.PI);
      ctx.fillStyle = `rgba(255, 255, 255, ${(alpha * 0.95).toFixed(2)})`;
      ctx.shadowColor = ship.color;
      ctx.shadowBlur = 10;
      ctx.fill();
    }

    ctx.restore();
  }
}
