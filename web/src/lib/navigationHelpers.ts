import { resolveProjectCategory } from './nebulas';
import type { GalaxyLink, GalaxyNode } from '../types/galaxy';

export type NavigationTarget = { kind: 'note' | 'cluster' | 'sector'; id: string };
export type RecentNote = { id: string; title: string };
export type SavedSearch = { query: string };

const MAX_ITEMS = 20;
const cleanCategory = (value: string) => value.toLocaleLowerCase().replace(/^(notes|sources)\//, '');
const hasControlCharacter = (value: string) => Array.from(value).some(character => character.charCodeAt(0) < 32);

export function isSafeNoteId(value: unknown): value is string {
  return typeof value === 'string' && value.length > 0 && value.length <= 512 &&
    !value.startsWith('/') && !value.startsWith('\\') && !value.includes('\\') &&
    !value.split('/').some(part => part === '..' || part === '.') &&
    !hasControlCharacter(value) && !/[?#]/.test(value) &&
    (!/^[a-z][a-z\d+.-]*:/i.test(value) || /^(routine|skill|harvester):[^:]+$/i.test(value));
}

export function encodeNavigationHash(target: NavigationTarget | null): string {
  if (!target) return '';
  return `#${target.kind}=${encodeURIComponent(target.id)}`;
}

export function decodeNavigationHash(hash: string): NavigationTarget | null {
  const match = /^#(note|cluster|sector)=([^&]+)$/.exec(hash);
  if (!match) return null;
  try {
    const id = decodeURIComponent(match[2]);
    if (!isSafeNoteId(id) || (match[1] !== 'note' && id.includes(':'))) return null;
    return { kind: match[1] as NavigationTarget['kind'], id };
  } catch {
    return null;
  }
}

export function isMemoryNote(node: GalaxyNode): boolean {
  return !node.isCat && !node.id.startsWith('__CAT_') && !node.id.startsWith('__SECTOR_') &&
    (!node.ring || node.ring === 'memory');
}

export function getExplorerNodes(
  nodes: GalaxyNode[],
  scope: { visibleNodes?: ReadonlySet<string> | null; focusedCluster?: string | null; focusedSector?: string | null },
): GalaxyNode[] {
  const cluster = scope.focusedCluster?.replace(/^__CAT_/, '');
  const sector = scope.focusedSector?.replace(/^__SECTOR_/, '');
  return nodes.filter(node => {
    if (!isMemoryNote(node)) return false;
    if (scope.visibleNodes) return scope.visibleNodes.has(node.id);
    const category = cleanCategory(node.originalCategory || node.category || '');
    if (cluster) {
      const target = cleanCategory(cluster);
      return category === target || category.startsWith(`${target}/`);
    }
    if (sector) {
      const target = cleanCategory(sector);
      const nodeSector = cleanCategory(resolveProjectCategory(node.originalCategory || node.category || '').sector);
      return category === target || category.startsWith(`${target}/`) || nodeSector === target;
    }
    return true;
  }).sort((a, b) => (a.title || a.id).localeCompare(b.title || b.id));
}

export function getDirectedRelations(activeId: string, nodes: GalaxyNode[], links: GalaxyLink[]) {
  const byId = new Map(nodes.filter(isMemoryNote).map(node => [node.id, node]));
  const incoming = new Map<string, { id: string; title: string }>();
  const outgoing = new Map<string, { id: string; title: string }>();
  for (const link of links) {
    if (link.source === activeId && link.target !== activeId) {
      const node = byId.get(link.target);
      if (node) outgoing.set(node.id, { id: node.id, title: node.title || node.id });
    }
    if (link.target === activeId && link.source !== activeId) {
      const node = byId.get(link.source);
      if (node) incoming.set(node.id, { id: node.id, title: node.title || node.id });
    }
  }
  return { incoming: [...incoming.values()], outgoing: [...outgoing.values()] };
}

export function normalizeRecentNotes(value: unknown): RecentNote[] {
  if (!Array.isArray(value)) return [];
  const seen = new Set<string>();
  return value.flatMap(item => {
    if (!item || typeof item !== 'object') return [];
    const { id, title } = item as Record<string, unknown>;
    if (!isSafeNoteId(id) || id.includes(':') || typeof title !== 'string' || !title.trim() || title.length > 200 || seen.has(id)) return [];
    seen.add(id);
    return [{ id, title: title.trim() }];
  }).slice(0, MAX_ITEMS);
}

export function normalizeSavedSearches(value: unknown): SavedSearch[] {
  if (!Array.isArray(value)) return [];
  const seen = new Set<string>();
  return value.flatMap(item => {
    if (!item || typeof item !== 'object') return [];
    const query = (item as Record<string, unknown>).query;
    if (typeof query !== 'string') return [];
    const clean = query.trim();
    const key = clean.toLocaleLowerCase();
    if (clean.length < 2 || clean.length > 160 || hasControlCharacter(clean) || seen.has(key)) return [];
    seen.add(key);
    return [{ query: clean }];
  }).slice(0, MAX_ITEMS);
}

export function readLocalList<T>(key: string, normalize: (value: unknown) => T[]): T[] {
  try { return normalize(JSON.parse(localStorage.getItem(key) || '[]')); }
  catch { return []; }
}

export function writeLocalList<T>(key: string, value: T[]): void {
  try { localStorage.setItem(key, JSON.stringify(value)); }
  catch { /* Local storage may be unavailable. Navigation remains usable. */ }
}

export interface NavigationTransitionState { lastKey: string; suppressKey: string | null }

export function shouldPushNavigation(state: NavigationTransitionState, nextKey: string): boolean {
  if (state.suppressKey !== null && state.suppressKey === nextKey) {
    state.suppressKey = null;
    state.lastKey = nextKey;
    return false;
  }
  if (state.suppressKey !== null && state.lastKey === nextKey) return false;
  state.suppressKey = null;
  if (state.lastKey === nextKey) return false;
  state.lastKey = nextKey;
  return true;
}
