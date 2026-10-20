import { describe, expect, it } from 'vitest';
import { deduplicateSearchResults, isAutomationSatellite, normalizeSearchPath, stripSearchMarkup } from '../src/lib/searchHelpers';
import type { SearchResultItem } from '../src/types/galaxy';

describe('search helpers', () => {
  it('normalizes paths independent of slash style and casing', () => {
    expect(normalizeSearchPath('\\Notes\\AI\\Go.md')).toBe('notes/ai/go.md');
  });

  it('keeps a note when a satellite shares its normalized path', () => {
    const satellite: SearchResultItem = { id: 'automation', relPath: 'notes/ai/go.md', title: 'Automation', snippet: '', isArms: true };
    const note: SearchResultItem = { relPath: 'Notes/AI/Go.md', title: 'Go and AI', snippet: '' };
    expect(deduplicateSearchResults([satellite, note])).toEqual([note]);
  });

  it('does not classify ordinary memory notes as automation satellites', () => {
    expect(isAutomationSatellite({ id: 'notes/ai.md', relPath: 'notes/ai.md', title: 'AI', snippet: '', ring: 'outer' })).toBe(false);
    expect(isAutomationSatellite({ id: 'routine:weekly', title: 'Weekly review', snippet: '', category: 'arms.routines' })).toBe(true);
  });

  it('strips markdown formatting while preserving HTML-looking text for React escaping', () => {
    expect(stripSearchMarkup('## **Safe** [snippet](https://example.com) ==AI== <img src=x onerror=alert(1)> `code`'))
      .toBe('Safe snippet ==AI== <img src=x onerror=alert(1)> code');
  });

  it('preserves technical angle-bracket text for safe React text rendering', () => {
    expect(stripSearchMarkup('std::vector<int> and x < y > z')).toBe('std::vector<int> and x < y > z');
  });
});
