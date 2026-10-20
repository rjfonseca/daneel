import { describe, expect, it } from 'vitest';
import { decodeNavigationHash, encodeNavigationHash, getDirectedRelations, getExplorerNodes, normalizeSavedSearches, normalizeRecentNotes, shouldPushNavigation } from '../src/lib/navigationHelpers';
import type { GalaxyNode } from '../src/types/galaxy';

const nodes: GalaxyNode[] = [
  { id: 'notes/a.md', title: 'Alpha', category: 'notes/topic', ring: 'memory', modTime: 1_700_000_000, connections: 2 },
  { id: 'notes/b.md', title: 'Beta', category: 'notes/topic/child', connections: 1 },
  { id: 'notes/c.md', title: 'Gamma', category: 'notes/other', connections: 0 },
  { id: 'skill:test', title: 'Skill', ring: 'skills', category: 'arms' },
];

describe('navigation helpers', () => {
  it('encodes and validates local note and cluster fragments', () => {
    expect(decodeNavigationHash(encodeNavigationHash({ kind: 'note', id: 'notes/a.md' }))).toEqual({ kind: 'note', id: 'notes/a.md' });
    expect(decodeNavigationHash('#cluster=notes%2Ftopic')).toEqual({ kind: 'cluster', id: 'notes/topic' });
    expect(decodeNavigationHash('#note=%2Fsecret')).toBeNull();
    expect(decodeNavigationHash('#note=%E0%A4%A')).toBeNull();
    expect(decodeNavigationHash('#note=routine%3Adoctor')).toEqual({ kind: 'note', id: 'routine:doctor' });
    expect(decodeNavigationHash(encodeNavigationHash({ kind: 'sector', id: 'notes/topic' }))).toEqual({ kind: 'sector', id: 'notes/topic' });
    expect(decodeNavigationHash('#note=https%3A%2F%2Fexample.com')).toBeNull();
  });

  it('scopes the explorer to cluster or constellation and excludes satellites', () => {
    expect(getExplorerNodes(nodes, { focusedCluster: '__CAT_notes/topic' }).map(node => node.id)).toEqual(['notes/a.md', 'notes/b.md']);
    expect(getExplorerNodes(nodes, { visibleNodes: new Set(['notes/c.md', 'skill:test']) }).map(node => node.id)).toEqual(['notes/c.md']);
  });

  it('preserves link direction and deduplicates relations', () => {
    expect(getDirectedRelations('notes/a.md', nodes, [
      { source: 'notes/a.md', target: 'notes/b.md' },
      { source: 'notes/a.md', target: 'notes/b.md' },
      { source: 'notes/c.md', target: 'notes/a.md' },
      { source: 'notes/a.md', target: 'skill:test' },
    ])).toEqual({ outgoing: [{ id: 'notes/b.md', title: 'Beta' }], incoming: [{ id: 'notes/c.md', title: 'Gamma' }] });
  });

  it('bounds and validates untrusted local lists', () => {
    expect(normalizeRecentNotes([{ id: '../bad', title: 'Bad' }, { id: 'notes/a.md', title: 'Alpha' }])).toEqual([{ id: 'notes/a.md', title: 'Alpha' }]);
    expect(normalizeSavedSearches([{ query: 'a' }, { query: 'architecture' }, { query: 'architecture' }])).toEqual([{ query: 'architecture' }]);
    expect(normalizeSavedSearches(Array.from({ length: 40 }, (_, i) => ({ query: `term ${i}` })))).toHaveLength(20);
  });

  it('records a new choice after Back without consuming stale restore suppression', () => {
    const state = { lastKey: 'note:notes/b.md', suppressKey: 'note:notes/a.md' };
    expect(shouldPushNavigation(state, 'note:notes/b.md')).toBe(false);
    expect(state.suppressKey).toBe('note:notes/a.md');
    expect(shouldPushNavigation(state, 'note:notes/a.md')).toBe(false);
    expect(shouldPushNavigation(state, 'note:notes/c.md')).toBe(true);
    state.suppressKey = 'note:notes/a.md';
    expect(shouldPushNavigation(state, 'note:notes/a.md')).toBe(false);
  });
});
