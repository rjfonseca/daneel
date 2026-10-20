import { isMemoryNote } from './navigationHelpers';
import type { GalaxyLink, GalaxyNode } from '../types/galaxy';

const MAX_FOCUS_NEIGHBORS = 500;
const MAX_SECOND_HOP_NEIGHBORS = 500;
const MAX_CANDIDATES = 2_000;
const DEFAULT_RESULT_LIMIT = 50;

export interface GraphReviewSuggestion {
  node: GalaxyNode;
  sharedNeighbors: GalaxyNode[];
  reason: string;
}

function compareNodes(a: GalaxyNode, b: GalaxyNode): number {
  const left = a.title || a.id;
  const right = b.title || b.id;
  if (left !== right) return left < right ? -1 : 1;
  return a.id < b.id ? -1 : a.id === b.id ? 0 : 1;
}

export function findOrphanNotes(nodes: GalaxyNode[], links: GalaxyLink[]): GalaxyNode[] {
  const notes = nodes.filter(isMemoryNote);
  const noteIds = new Set(notes.map(node => node.id));
  const connected = new Set<string>();
  for (const link of links) {
    if (noteIds.has(link.source)) connected.add(link.source);
    if (noteIds.has(link.target)) connected.add(link.target);
  }
  return notes.filter(node => !connected.has(node.id)).sort(compareNodes);
}

/**
 * Suggests candidates only when both notes share an outgoing or incoming
 * neighbor. Enumeration uses deterministic slices of the selected note's
 * adjacency and each shared neighbor's matching adjacency.
 */
export function getSuggestedConnections(
  focusId: string,
  nodes: GalaxyNode[],
  links: GalaxyLink[],
  resultLimit = DEFAULT_RESULT_LIMIT,
): GraphReviewSuggestion[] {
  const byId = new Map(nodes.filter(isMemoryNote).map(node => [node.id, node]));
  const focus = byId.get(focusId);
  if (!focus) return [];

  const outgoing = new Map<string, Set<string>>();
  const incoming = new Map<string, Set<string>>();
  const existing = new Set<string>();
  for (const link of links) {
    if (!byId.has(link.source) || !byId.has(link.target)) continue;
    let targets = outgoing.get(link.source);
    if (!targets) {
      targets = new Set();
      outgoing.set(link.source, targets);
    }
    targets.add(link.target);
    let sources = incoming.get(link.target);
    if (!sources) {
      sources = new Set();
      incoming.set(link.target, sources);
    }
    sources.add(link.source);
    if (link.source === focusId) existing.add(link.target);
    if (link.target === focusId) existing.add(link.source);
  }

  const focusNeighbors = [...new Set([...(outgoing.get(focusId) || []), ...(incoming.get(focusId) || [])])]
    .filter(id => id !== focusId)
    .sort((a, b) => compareNodes(byId.get(a)!, byId.get(b)!))
    .slice(0, MAX_FOCUS_NEIGHBORS);
  const evidence = new Map<string, Set<string>>();
  let candidateCount = 0;

  for (const sharedId of focusNeighbors) {
    const focusPointsToShared = (outgoing.get(focusId) || new Set()).has(sharedId);
    const candidates = focusPointsToShared ? incoming.get(sharedId) || new Set<string>() : outgoing.get(sharedId) || new Set<string>();
    const boundedCandidates = [...candidates]
      .filter(id => id !== focusId && id !== sharedId && byId.has(id) && !existing.has(id))
      .sort((a, b) => compareNodes(byId.get(a)!, byId.get(b)!))
      .slice(0, MAX_SECOND_HOP_NEIGHBORS);
    for (const candidateId of boundedCandidates) {
      if (!evidence.has(candidateId)) {
        if (candidateCount >= MAX_CANDIDATES) break;
        evidence.set(candidateId, new Set());
        candidateCount += 1;
      }
      evidence.get(candidateId)!.add(sharedId);
    }
    if (candidateCount >= MAX_CANDIDATES) break;
  }

  const limit = Number.isFinite(resultLimit) ? Math.max(0, Math.min(Math.floor(resultLimit), MAX_CANDIDATES)) : DEFAULT_RESULT_LIMIT;
  return [...evidence.entries()]
    .map(([id, sharedEvidence]) => {
      const sharedNeighbors = [...sharedEvidence.keys()].map(sharedId => byId.get(sharedId)!).sort(compareNodes);
      const names = sharedNeighbors.map(node => node.title || node.id);
      const neighborList = names.length === 1
        ? names[0]
        : `${names.slice(0, -1).join(', ')} and ${names[names.length - 1]}`;
      const reason = `Both notes share neighbors: ${neighborList}.`;
      return { node: byId.get(id)!, sharedNeighbors, reason };
    })
    .sort((a, b) => b.sharedNeighbors.length - a.sharedNeighbors.length || compareNodes(a.node, b.node))
    .slice(0, limit);
}
