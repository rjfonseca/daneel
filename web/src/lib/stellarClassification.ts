/**
 * Classificação Astrofísica de Notas (A Galáxia do Conhecimento)
 *
 * Mapeia notas para o espectro astrofísico real com base em:
 * 1. Energia Térmica / Cinética: Recência de edição (mtime) espelhando a Lei de Wien
 *    (Radiação de alta frequência/azul para recém-nascidas até infravermelho/marrom para frias).
 * 2. Massa Gravitacional: Grau de conexões reais da rede.
 * 3. Fenômenos Cósmicos Especiais:
 *    - Quasares: Super-hubs ativos com massa e energia extremas.
 *    - Pulsares: Emissão periódica rítmica (notas daily recentes).
 *    - Anãs Marrons: Corpos subestelares de baixa massa/energia que formam a poeira estelar de fundo.
 */

export type StellarClass =
  | 'QUASAR'
  | 'PULSAR'
  | 'BLUE_GIANT'
  | 'WHITE_CYAN'
  | 'YELLOW_SUN'
  | 'ORANGE_STAR'
  | 'RED_DWARF'
  | 'BROWN_DWARF';

export interface StellarAttributes {
  classification: StellarClass;
  badge: string;
  label: string;
  color: string;
  size: number;
  alpha: number;
  description: string;
  spectralClass: string;
}

export interface NodeLike {
  id: string;
  category?: string;
  modTime?: number | string | bigint;
  connections?: number;
  title?: string;
  ring?: string;
  noteType?: string;
  repository?: string;
}

export const STELLAR_CLASSES: Record<StellarClass, StellarAttributes> = {
  QUASAR: {
    classification: 'QUASAR',
    badge: '⚡ QUASAR',
    label: 'Supermassive Quasar',
    color: '#38bdf8',
    size: 2.5,
    alpha: 1.0,
    description: 'Active Central Super-Hub',
    spectralClass: 'AGN',
  },
  PULSAR: {
    classification: 'PULSAR',
    badge: '📡 PULSAR',
    label: 'Rhythmic Pulsar',
    color: '#67e8f9',
    size: 1.8,
    alpha: 0.95,
    description: 'Periodic Daily Log Beacon',
    spectralClass: 'PSR',
  },
  BLUE_GIANT: {
    classification: 'BLUE_GIANT',
    badge: '💎 BLUE GIANT',
    label: 'Blue Giant (Class O/B)',
    color: '#60a5fa',
    size: 1.5,
    alpha: 0.90,
    description: 'High-Energy Young Star',
    spectralClass: 'O/B',
  },
  WHITE_CYAN: {
    classification: 'WHITE_CYAN',
    badge: '✨ WHITE STAR',
    label: 'White Star (Class A/F)',
    color: '#a5f3fc',
    size: 1.1,
    alpha: 0.85,
    description: 'Expanding Star of the Week',
    spectralClass: 'A/F',
  },
  YELLOW_SUN: {
    classification: 'YELLOW_SUN',
    badge: '☀️ YELLOW DWARF',
    label: 'Yellow Dwarf (Class G)',
    color: '#facc15',
    size: 0.85,
    alpha: 0.80,
    description: 'Stellar Maturity (Solar Type)',
    spectralClass: 'G',
  },
  ORANGE_STAR: {
    classification: 'ORANGE_STAR',
    badge: '🟠 ORANGE STAR',
    label: 'Orange Star (Class K)',
    color: '#fb923c',
    size: 0.75,
    alpha: 0.75,
    description: 'Consolidated Knowledge with Links',
    spectralClass: 'K',
  },
  RED_DWARF: {
    classification: 'RED_DWARF',
    badge: '🔴 RED DWARF',
    label: 'Red Dwarf (Class M)',
    color: '#fb7185',
    size: 0.60,
    alpha: 0.85,
    description: 'Stable Reference Knowledge',
    spectralClass: 'M',
  },
  BROWN_DWARF: {
    classification: 'BROWN_DWARF',
    badge: '🤎 BROWN DWARF',
    label: 'Substellar Brown Dwarf',
    color: '#d97706',
    size: 0.40,
    alpha: 0.80,
    description: 'Substellar Leaf Node (Cosmic Dust)',
    spectralClass: 'L/T/Y',
  },
};

export const STELLAR_SPECTRUM_LEGEND = [
  STELLAR_CLASSES.QUASAR,
  STELLAR_CLASSES.PULSAR,
  STELLAR_CLASSES.BLUE_GIANT,
  STELLAR_CLASSES.WHITE_CYAN,
  STELLAR_CLASSES.YELLOW_SUN,
  STELLAR_CLASSES.ORANGE_STAR,
  STELLAR_CLASSES.RED_DWARF,
  STELLAR_CLASSES.BROWN_DWARF,
];

/**
 * Classifica um nó com base em idade (modTime em segundos) e conexões.
 *
 * @param node Objeto de nó contendo id, modTime, connections e category
 * @param nowTimestampSec Timestamp Unix atual opcional em segundos (para determinismo de testes)
 */
export function classifyStar(node: NodeLike, nowTimestampSec?: number): StellarAttributes {
  const now = nowTimestampSec !== undefined ? nowTimestampSec : Date.now() / 1000;
  
  let modSec = 0;
  if (node.modTime !== undefined && node.modTime !== null) {
    const rawVal = typeof node.modTime === 'bigint' ? Number(node.modTime) : Number(node.modTime);
    if (!Number.isNaN(rawVal) && rawVal > 0) {
      // Se maior que 100 bilhões, assume milissegundos
      modSec = rawVal > 1e11 ? rawVal / 1000 : rawVal;
    }
  }

  const daysOld = modSec > 0 ? Math.max(0, (now - modSec) / 86400) : 9999;
  const connections = Math.max(0, Number(node.connections) || 0);
  const category = (node.category || '').toLowerCase();
  const id = (node.id || '').toLowerCase();
  const ring = (node.ring || '').toLowerCase();

  // ARMS Orbital Stations
  if (ring === 'harvesters' || category.startsWith('arms/harvester')) {
    return {
      classification: 'QUASAR',
      badge: '🛸 HARVESTER',
      label: 'Harvester Satellite (ARMS III)',
      color: '#10b981',
      size: 6.0,
      alpha: 1.0,
      description: 'External Ingestion & Synchronization',
      spectralClass: 'ARMS-III',
    };
  }
  if (ring === 'routines' || category.startsWith('arms/routine')) {
    return {
      classification: 'QUASAR',
      badge: '⏱️ ROUTINE',
      label: 'Routine Station (ARMS II)',
      color: '#f59e0b',
      size: 6.0,
      alpha: 1.0,
      description: 'Autonomous Cadence & Telemetry',
      spectralClass: 'ARMS-II',
    };
  }
  if (ring === 'skills' || category.startsWith('arms/skill')) {
    return {
      classification: 'QUASAR',
      badge: '🧠 SKILL',
      label: 'Skill Capsule (ARMS I)',
      color: '#a855f7',
      size: 6.0,
      alpha: 1.0,
      description: 'Executable Agent Skill',
      spectralClass: 'ARMS-I',
    };
  }

  // Repository (Star Cluster) Core: Alpha Star
  const isRepoCore =
    node.noteType === 'repository' ||
    (Boolean(node.repository) && (id.endsWith('/index.md') || id === 'index.md'));
  if (isRepoCore) {
    return {
      classification: 'BLUE_GIANT',
      badge: '☀️ CLUSTER CORE',
      label: 'Cluster Core (α Star)',
      color: '#38bdf8',
      size: 2.2,
      alpha: 1.0,
      description: 'Primary Gravitational Anchor of Star Cluster',
      spectralClass: 'α-CORE',
    };
  }

  // Group (Cosmic Nebula) Core
  const isGroupCore =
    node.noteType === 'group' ||
    (id.endsWith('/index.md') && !node.repository && (category.startsWith('sources/gitlab') || category.startsWith('sources/github')));
  if (isGroupCore) {
    return {
      classification: 'WHITE_CYAN',
      badge: '🌌 NEBULA CORE',
      label: 'Nebula Core (MOC)',
      color: '#c084fc',
      size: 1.8,
      alpha: 0.95,
      description: 'Organizational Group Nexus',
      spectralClass: 'NEB-CORE',
    };
  }

  // 1. Quasar: Super-Hub Ativo (massa de conexões altíssima + recência recente)
  if (connections >= 15 && daysOld <= 7) {
    return STELLAR_CLASSES.QUASAR;
  }

  // 2. Pulsar: Registro diário contínuo emitindo pulsos (daily notes recentes)
  const isDaily = category === 'daily' || id.startsWith('daily/') || id.includes('/daily/');
  if (isDaily && daysOld <= 3) {
    return STELLAR_CLASSES.PULSAR;
  }

  // 3. Gigante Azul (O/B): Notas hiperativas/jovens
  if (daysOld <= 3) {
    return STELLAR_CLASSES.BLUE_GIANT;
  }

  // 4. Estrela Branca/Ciano (A/F): Notas da última semana
  if (daysOld <= 7) {
    return STELLAR_CLASSES.WHITE_CYAN;
  }

  // 5. Anã Amarela (G): Maturidade estelar (1 a 3 semanas)
  if (daysOld <= 20) {
    return STELLAR_CLASSES.YELLOW_SUN;
  }

  // 6. Conhecimento Antigo / Consolidado (> 20 dias):
  // Dividido pela gravidade / densidade de conexões
  if (connections >= 3) {
    return STELLAR_CLASSES.ORANGE_STAR;
  }

  if (connections === 2) {
    return STELLAR_CLASSES.RED_DWARF;
  }

  // Folhas ou nós quase isolados: Anãs Marrons (subestelar)
  return STELLAR_CLASSES.BROWN_DWARF;
}
