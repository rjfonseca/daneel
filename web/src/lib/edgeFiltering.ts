/**
 * Mecânica de Progressive Disclosure (LOD) e Filtragem de Arestas
 *
 * Em grafos densos de conhecimento, dezenas de milhares de arestas sobrepostas criam
 * um "blob branco" impenetrável. Esta biblioteca aplica Nível de Detalhe (LOD) baseado
 * no zoom da câmera e relevância contextual:
 *
 * 1. Overview (Câmera distante):
 *    - Oculta arestas internas de clusters (intra-cluster).
 *    - Exibe apenas filamentos tênues entre super-hubs ou pontes maiores.
 * 2. Intermediate (Zoom médio):
 *    - Revela arestas do setor focado ou com opacidade reduzida.
 * 3. Deep Zoom (Mergulho profundo):
 *    - Revela topologia completa com alta nitidez.
 * 4. Foco / Hover:
 *    - Ao interagir com qualquer nó, sua constelação acende em neon violeta vibrante.
 */

export type EdgeVisibilityMode = 'auto' | 'bridges' | 'focus' | 'all';

export interface EdgeFilterContext {
  cameraRatio: number;
  mode: EdgeVisibilityMode;
  hoveredNodeId?: string | null;
  selectedNodeId?: string | null;
  focusedCategory?: string | null;
}

export interface EdgeData {
  source?: string;
  target?: string;
  isCrossCluster?: boolean;
  sourceCategory?: string;
  targetCategory?: string;
  sourceDegree?: number;
  targetDegree?: number;
  originalHidden?: boolean;
  size?: number;
  color?: string;
  hidden?: boolean;
  [key: string]: unknown;
}

export interface ReducedEdgeAttributes {
  hidden: boolean;
  color: string;
  size: number;
  zIndex: number;
}

/**
 * Calcula os atributos visuais de uma aresta com base no contexto atual de visualização.
 */
export function computeEdgeDisplay(
  data: EdgeData,
  source: string,
  target: string,
  ctx: EdgeFilterContext
): ReducedEdgeAttributes {
  // Arestas de força centrípeta (âncoras invisíveis de ForceAtlas2) nunca são renderizadas
  if (data.originalHidden === true || source.startsWith('__CAT_') || target.startsWith('__CAT_')) {
    return {
      hidden: true,
      color: 'rgba(0,0,0,0)',
      size: 0.1,
      zIndex: 0,
    };
  }

  const isConnectedToHover = Boolean(
    ctx.hoveredNodeId && (source === ctx.hoveredNodeId || target === ctx.hoveredNodeId)
  );
  const isConnectedToSelect = Boolean(
    ctx.selectedNodeId && (source === ctx.selectedNodeId || target === ctx.selectedNodeId)
  );

  // 1. Destaque por Hover / Seleção (Filamentos estelares ultrafinos e sutis)
  if (isConnectedToHover || isConnectedToSelect) {
    return {
      hidden: false,
      color: 'rgba(186, 230, 253, 0.40)', // Filamentos estelares ciano/branco ultrafinos
      size: 0.25,
      zIndex: 10,
    };
  }

  const isCross = Boolean(data.isCrossCluster);
  const ratio = typeof ctx.cameraRatio === 'number' && !Number.isNaN(ctx.cameraRatio) ? ctx.cameraRatio : 1.0;
  const sDeg = Number(data.sourceDegree) || 0;
  const tDeg = Number(data.targetDegree) || 0;
  const isMajorBridge = isCross && (sDeg >= 10 || tDeg >= 10);

  // 2. Modo: Apenas Pontes Inter-Cluster
  if (ctx.mode === 'bridges') {
    if (isCross) {
      return {
        hidden: false,
        color: 'rgba(99, 102, 241, 0.04)',
        size: 0.12,
        zIndex: 2,
      };
    }
    return {
      hidden: true,
      color: 'rgba(0,0,0,0)',
      size: 0.1,
      zIndex: 0,
    };
  }

  // 3. Modo: Foco Estrito no Setor Ativo
  if (ctx.mode === 'focus') {
    if (ctx.focusedCategory) {
      const cleanFocused = ctx.focusedCategory.replace('__CAT_', '').toLowerCase();
      const srcCat = (data.sourceCategory || '').toLowerCase();
      const tgtCat = (data.targetCategory || '').toLowerCase();
      const inSector = (srcCat === cleanFocused || srcCat.startsWith(cleanFocused + '/')) ||
                       (tgtCat === cleanFocused || tgtCat.startsWith(cleanFocused + '/'));

      if (inSector) {
        const isInternal = (srcCat === cleanFocused || srcCat.startsWith(cleanFocused + '/')) &&
                           (tgtCat === cleanFocused || tgtCat.startsWith(cleanFocused + '/'));
        return {
          hidden: false,
          color: isInternal ? 'rgba(147, 197, 253, 0.16)' : 'rgba(99, 102, 241, 0.06)',
          size: isInternal ? 0.18 : 0.10,
          zIndex: isInternal ? 3 : 1,
        };
      }
    }
    return {
      hidden: true,
      color: 'rgba(0,0,0,0)',
      size: 0.1,
      zIndex: 0,
    };
  }

  // 4. Modo: Todas as Arestas
  if (ctx.mode === 'all') {
    return {
      hidden: false,
      color: isCross ? 'rgba(99, 102, 241, 0.03)' : 'rgba(148, 163, 184, 0.025)',
      size: 0.10,
      zIndex: 1,
    };
  }

  // 5. Modo 'auto' (Progressive Disclosure por Zoom / LOD)
  // Overview / Vista Cósmica Panorâmica (ratio >= 0.65)
  if (ratio >= 0.65) {
    // No overview, exibe apenas os filamentos cósmicos entre super-hubs ativos
    if (isMajorBridge) {
      return {
        hidden: false,
        color: 'rgba(99, 102, 241, 0.025)',
        size: 0.10,
        zIndex: 1,
      };
    }
    // Todas as outras arestas ocultas no overview para eliminar 100% o white blob
    return {
      hidden: true,
      color: 'rgba(0,0,0,0)',
      size: 0.1,
      zIndex: 0,
    };
  }

  // Zoom Intermediário (0.35 <= ratio < 0.65)
  if (ratio >= 0.35) {
    if (ctx.focusedCategory) {
      const inFocused =
        data.sourceCategory === ctx.focusedCategory && data.targetCategory === ctx.focusedCategory;
      if (inFocused) {
        return {
          hidden: false,
          color: 'rgba(147, 197, 253, 0.12)',
          size: 0.18,
          zIndex: 2,
        };
      }
    }

    if (isCross) {
      return {
        hidden: false,
        color: 'rgba(99, 102, 241, 0.04)',
        size: 0.12,
        zIndex: 1,
      };
    }

    return {
      hidden: false,
      color: 'rgba(148, 163, 184, 0.02)',
      size: 0.10,
      zIndex: 0,
    };
  }

  // Deep Zoom (ratio < 0.35)
  return {
    hidden: false,
    color: isCross ? 'rgba(99, 102, 241, 0.10)' : 'rgba(148, 163, 184, 0.06)',
    size: isCross ? 0.16 : 0.12,
    zIndex: 1,
  };
}

/**
 * Cria o redutor de arestas para passar às opções do Sigma
 */
export function createEdgeReducer(
  arg1: ((edge: string) => { source: string; target: string } | undefined) | (() => EdgeFilterContext),
  arg2?: () => EdgeFilterContext
) {
  let getEndpoints: ((edge: string) => { source: string; target: string } | undefined) | undefined;
  let getContext: () => EdgeFilterContext;

  if (typeof arg2 === 'function') {
    getEndpoints = arg1 as (edge: string) => { source: string; target: string } | undefined;
    getContext = arg2;
  } else {
    getContext = arg1 as () => EdgeFilterContext;
  }

  return (edge: string, data: Record<string, unknown>) => {
    let source = (data.source as string) || '';
    let target = (data.target as string) || '';

    if ((!source || !target) && getEndpoints) {
      const ep = getEndpoints(edge);
      if (ep) {
        source = ep.source;
        target = ep.target;
      }
    }

    if (!source || !target) return data;

    const ctx = getContext();
    const result = computeEdgeDisplay(data as EdgeData, source, target, ctx);

    return {
      ...data,
      ...result,
    };
  };
}
