import { describe, it, expect } from 'vitest';
import { findWeeklySummaryNote } from '../src/lib/weeklyResolver';
import type { GalaxyNode } from '../src/types/galaxy';

describe('findWeeklySummaryNote', () => {
  const nodes: GalaxyNode[] = [
    { id: 'notes/architecture.md', title: 'Architecture' },
    { id: 'daily/2026-09-15.md', title: '2026-09-15' },
    { id: 'weekly/2026-W38.md', title: 'Weekly 38 - 2026' },
    { id: 'periodic/weekly/2026-W39.md', title: 'Weekly 39' },
    { id: 'notes/weekly/custom-pattern-2026-week-40.md', title: 'Week 40 Priorities' },
  ];

  it('resolves standard weekly note pattern weekly/YYYY-WW.md', () => {
    const match = findWeeklySummaryNote(nodes, 38, 2026);
    expect(match).toBeDefined();
    expect(match?.id).toBe('weekly/2026-W38.md');
  });

  it('resolves periodic weekly note periodic/weekly/YYYY-WW.md', () => {
    const match = findWeeklySummaryNote(nodes, 39, 2026);
    expect(match).toBeDefined();
    expect(match?.id).toBe('periodic/weekly/2026-W39.md');
  });

  it('resolves custom pattern with {{year}} and {{week}} variables', () => {
    const customPattern = 'notes/weekly/custom-pattern-{{year}}-week-{{week}}.md';
    const match = findWeeklySummaryNote(nodes, 40, 2026, customPattern);
    expect(match).toBeDefined();
    expect(match?.id).toBe('notes/weekly/custom-pattern-2026-week-40.md');
  });

  it('resolves fuzzy matches when path includes year and W{week}', () => {
    const fuzzyNodes: GalaxyNode[] = [
      { id: 'journals/2026/Week 38 Summary.md', title: 'Week 38 Priorities' }
    ];
    const match = findWeeklySummaryNote(fuzzyNodes, 38, 2026);
    expect(match).toBeDefined();
    expect(match?.id).toBe('journals/2026/Week 38 Summary.md');
  });

  it('returns null when no matching weekly note exists in nodes', () => {
    const match = findWeeklySummaryNote(nodes, 12, 2026);
    expect(match).toBeNull();
  });
});
