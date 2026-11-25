/**
 * @file sectorVisibility.ts
 * Batched Graphology visibility filter and shared sector/cluster matching helpers.
 */

import type Graph from 'graphology';
import { resolveCanonicalSectorKey, resolveProjectCategory } from '../nebulas';

export interface FocusTargets {
  clusterCat: string;
  cleanCluster: string;
  sectorCat: string;
  cleanSector: string;
}

export function normalizeFocusTargets(
  focusedSector: string | null | undefined,
  focusedCluster: string | null | undefined
): FocusTargets {
  const clusterCat = focusedCluster
    ? focusedCluster.replace('__CAT_', '').toLowerCase()
    : '';
  const cleanCluster = clusterCat.replace(/^(notes|sources)\//, '');

  const sectorCat = focusedSector
    ? focusedSector.replace('__SECTOR_', '').replace('__CAT_', '').toLowerCase()
    : '';
  const cleanSector = sectorCat.replace(/^(notes|sources)\//, '');

  return { clusterCat, cleanCluster, sectorCat, cleanSector };
}

export function matchesClusterFilter(
  rawCat: string,
  clusterKey: string | undefined,
  clusterCat: string,
  cleanCluster: string
): boolean {
  const cKey = clusterKey || rawCat;
  const cleanNoteCat = cKey.replace(/^(notes|sources)\//, '');
  return (
    cKey === clusterCat ||
    cKey.startsWith(clusterCat + '/') ||
    cleanNoteCat === cleanCluster ||
    cleanNoteCat.startsWith(cleanCluster + '/')
  );
}

export function matchesSectorFilter(
  rawCat: string,
  precomputedSectorKey: string | undefined,
  sectorCat: string,
  cleanSector: string
): boolean {
  const cleanNoteCat = rawCat.replace(/^(notes|sources)\//, '');
  const nodeSector = precomputedSectorKey || resolveProjectCategory(rawCat).sector.toLowerCase();
  const cleanNodeSector = nodeSector.replace(/^(notes|sources)\//, '');
  const canonicalSector = resolveCanonicalSectorKey(rawCat, precomputedSectorKey);
  const cleanCanonical = canonicalSector.replace(/^(notes|sources)\//, '');
  return (
    rawCat === sectorCat ||
    rawCat.startsWith(sectorCat + '/') ||
    cleanNoteCat === cleanSector ||
    cleanNoteCat.startsWith(cleanSector + '/') ||
    nodeSector === sectorCat ||
    cleanNodeSector === cleanSector ||
    canonicalSector === sectorCat ||
    cleanCanonical === cleanSector
  );
}

/**
 * Updates `hidden` attributes across nodes and edges in two single Graphology
 * batch calls (`updateEach*`) instead of emitting N+E individual attribute events.
 */
export function applySectorVisibilityFilter(
  graph: Graph,
  focusedSector: string | null,
  focusedCluster: string | null | undefined
): void {
  if (!graph || graph.order === 0) return;

  const { clusterCat, cleanCluster, sectorCat, cleanSector } = normalizeFocusTargets(
    focusedSector,
    focusedCluster
  );

  if (clusterCat) {
    graph.updateEachNodeAttributes((node, attrs) => {
      let nextHidden = false;
      if (node.startsWith('__SECTOR_')) {
        nextHidden = true;
      } else if (node.startsWith('__CAT_')) {
        const thisCat = node.replace('__CAT_', '').toLowerCase();
        const cleanThisCat = thisCat.replace(/^(notes|sources)\//, '');
        nextHidden = !(thisCat === clusterCat || cleanThisCat === cleanCluster);
      } else {
        const noteCat = String(attrs.originalCategory || attrs.category || 'root').toLowerCase();
        const clusterKey = typeof attrs.clusterKey === 'string' ? attrs.clusterKey.toLowerCase() : undefined;
        nextHidden = !matchesClusterFilter(noteCat, clusterKey, clusterCat, cleanCluster);
      }
      if (attrs.hidden === nextHidden) return attrs;
      return { ...attrs, hidden: nextHidden };
    }, { attributes: ['hidden', 'zIndex'] });
  } else if (sectorCat) {
    graph.updateEachNodeAttributes((node, attrs) => {
      let nextHidden = false;
      if (node.startsWith('__SECTOR_')) {
        const thisSector = node.replace('__SECTOR_', '').toLowerCase();
        const cleanThisSector = thisSector.replace(/^(notes|sources)\//, '');
        nextHidden = !(thisSector === sectorCat || cleanThisSector === cleanSector);
      } else if (node.startsWith('__CAT_')) {
        const thisCat = node.replace('__CAT_', '').toLowerCase();
        const cleanThisCat = thisCat.replace(/^(notes|sources)\//, '');
        const preSector = typeof attrs.sectorKey === 'string' ? attrs.sectorKey.toLowerCase() : undefined;
        const isCurrentSector = matchesSectorFilter(thisCat, preSector, sectorCat, cleanSector);
        const isSubsector = thisCat.startsWith(sectorCat + '/') || cleanThisCat.startsWith(cleanSector + '/');
        nextHidden = !(isCurrentSector || isSubsector);
      } else {
        const noteCat = String(attrs.originalCategory || attrs.category || 'root').toLowerCase();
        const preSector = typeof attrs.sectorKey === 'string' ? attrs.sectorKey.toLowerCase() : undefined;
        nextHidden = !matchesSectorFilter(noteCat, preSector, sectorCat, cleanSector);
      }
      if (attrs.hidden === nextHidden) return attrs;
      return { ...attrs, hidden: nextHidden };
    }, { attributes: ['hidden', 'zIndex'] });
  } else {
    graph.updateEachNodeAttributes((_node, attrs) => {
      if (attrs.hidden === false) return attrs;
      return { ...attrs, hidden: false };
    }, { attributes: ['hidden', 'zIndex'] });
  }

  const hasFocus = Boolean(clusterCat || sectorCat);
  graph.updateEachEdgeAttributes((_edge, attrs, _source, _target, sourceAttrs, targetAttrs) => {
    const nextHidden = hasFocus
      ? Boolean(sourceAttrs.hidden || targetAttrs.hidden)
      : Boolean(attrs.originalHidden);
    if (attrs.hidden === nextHidden) return attrs;
    return { ...attrs, hidden: nextHidden };
  }, { attributes: ['hidden', 'zIndex'] });
}
