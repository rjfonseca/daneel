import { describe, expect, it } from 'vitest';
import { findOrphanNotes, getSuggestedConnections } from '../src/lib/graphReview';
import type { GalaxyLink, GalaxyNode } from '../src/types/galaxy';

const nodes: GalaxyNode[] = [
  { id: 'notes/focus.md', title: 'Focus' },
  { id: 'notes/candidate.md', title: 'Candidate' },
  { id: 'notes/neighbor.md', title: 'Neighbor' },
  { id: 'notes/isolated.md', title: 'Isolated' },
  { id: 'skill:build', title: 'Build skill', ring: 'skills' },
];

describe('graph review', () => {
  it('counts a note with an incoming directed edge as connected and excludes satellites', () => {
    const orphans = findOrphanNotes(nodes, [
      { source: 'notes/neighbor.md', target: 'notes/focus.md' },
      { source: 'skill:build', target: 'notes/candidate.md' },
    ]);
    expect(orphans.map(node => node.id)).toEqual(['notes/isolated.md']);
  });

  it('suggests only bounded two-hop notes with explicit shared-neighbor reasons', () => {
    const links: GalaxyLink[] = [
      { source: 'notes/focus.md', target: 'notes/neighbor.md' },
      { source: 'notes/candidate.md', target: 'notes/neighbor.md' },
      { source: 'notes/focus.md', target: 'notes/focus.md' },
      { source: 'notes/focus.md', target: 'notes/candidate.md' },
      { source: 'notes/focus.md', target: 'skill:build' },
    ];
    const result = getSuggestedConnections('notes/focus.md', nodes, links);
    expect(result).toHaveLength(0);
  });

  it('orders candidates deterministically by evidence count then title', () => {
    const extendedNodes = [...nodes, { id: 'notes/other.md', title: 'Another' }, { id: 'notes/second.md', title: 'Second' }];
    const links: GalaxyLink[] = [
      { source: 'notes/focus.md', target: 'notes/neighbor.md' },
      { source: 'notes/focus.md', target: 'notes/other.md' },
      { source: 'notes/candidate.md', target: 'notes/neighbor.md' },
      { source: 'notes/candidate.md', target: 'notes/other.md' },
      { source: 'notes/second.md', target: 'notes/neighbor.md' },
    ];
    expect(getSuggestedConnections('notes/focus.md', extendedNodes, links).map(item => [item.node.id, item.sharedNeighbors.map(n => n.id), item.reason])).toEqual([
      ['notes/candidate.md', ['notes/other.md', 'notes/neighbor.md'], 'Both notes share neighbors: Another and Neighbor.'],
      ['notes/second.md', ['notes/neighbor.md'], 'Both notes share neighbors: Neighbor.'],
    ]);
  });

  it('describes mixed incoming and outgoing evidence without assigning one direction to all neighbors', () => {
    const mixedNodes = [...nodes, { id: 'notes/incoming.md', title: 'Incoming' }];
    const links: GalaxyLink[] = [
      { source: 'notes/focus.md', target: 'notes/neighbor.md' },
      { source: 'notes/candidate.md', target: 'notes/neighbor.md' },
      { source: 'notes/incoming.md', target: 'notes/focus.md' },
      { source: 'notes/incoming.md', target: 'notes/candidate.md' },
    ];

    expect(getSuggestedConnections('notes/focus.md', mixedNodes, links)[0]).toMatchObject({
      node: { id: 'notes/candidate.md' },
      sharedNeighbors: [{ title: 'Incoming' }, { title: 'Neighbor' }],
      reason: 'Both notes share neighbors: Incoming and Neighbor.',
    });
  });

  it('caps candidate enumeration for a high-degree focus', () => {
    const broadNodes: GalaxyNode[] = [{ id: 'notes/focus.md', title: 'Focus' }];
    const broadLinks: GalaxyLink[] = [];
    for (let index = 0; index < 3000; index += 1) {
      const neighbor = `notes/n-${String(index).padStart(4, '0')}.md`;
      const candidate = `notes/c-${String(index).padStart(4, '0')}.md`;
      broadNodes.push({ id: neighbor, title: neighbor }, { id: candidate, title: candidate });
      broadLinks.push({ source: 'notes/focus.md', target: neighbor }, { source: candidate, target: neighbor });
    }
    expect(getSuggestedConnections('notes/focus.md', broadNodes, broadLinks, 100)).toHaveLength(100);
  });
});
