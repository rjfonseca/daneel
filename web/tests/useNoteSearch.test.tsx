import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useNoteSearch } from '../src/hooks/useNoteSearch';

const { searchNotes } = vi.hoisted(() => ({ searchNotes: vi.fn() }));
vi.mock('../src/lib/apiClient', () => ({ client: { searchNotes } }));

describe('useNoteSearch', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  it.each(['Go', 'AI'])('searches the short term %s after debounce', async term => {
    searchNotes.mockResolvedValue({ results: [{ relPath: `notes/${term.toLowerCase()}.md`, title: term, snippet: `==${term}==` }] });
    const { result } = renderHook(() => useNoteSearch({ nodes: [], catCounts: {} }));
    let search: Promise<void> = Promise.resolve();

    act(() => { search = result.current.handleSearchChange(term); });
    expect(result.current.isLoading).toBe(true);
    expect(searchNotes).not.toHaveBeenCalled();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(180);
      await search;
    });

    expect(searchNotes).toHaveBeenCalledWith({ query: term, limit: 50 });
    expect(result.current.searchResults[0]?.title).toBe(term);
    expect(result.current.hasSearched).toBe(true);
    expect(result.current.isLoading).toBe(false);
  });

  it('ignores an older response after a newer query completes', async () => {
    let resolveOld!: (value: { results: Array<{ relPath: string; title: string; snippet: string }> }) => void;
    searchNotes
      .mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve; }))
      .mockResolvedValueOnce({ results: [{ relPath: 'notes/new.md', title: 'New result', snippet: 'new' }] });
    const { result } = renderHook(() => useNoteSearch({ nodes: [], catCounts: {} }));
    let oldSearch: Promise<void> = Promise.resolve();
    let newSearch: Promise<void> = Promise.resolve();

    act(() => { oldSearch = result.current.handleSearchChange('old'); });
    await act(async () => { await vi.advanceTimersByTimeAsync(180); });
    act(() => { newSearch = result.current.handleSearchChange('new'); });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(180);
      await newSearch;
    });
    await act(async () => {
      resolveOld({ results: [{ relPath: 'notes/old.md', title: 'Old result', snippet: 'old' }] });
      await oldSearch;
    });

    expect(result.current.searchResults.map(item => item.title)).toEqual(['New result']);
  });

  it('clears results for a short query without calling the search service', async () => {
    const { result } = renderHook(() => useNoteSearch({ nodes: [], catCounts: {} }));
    await act(async () => { await result.current.handleSearchChange('x'); });
    expect(searchNotes).not.toHaveBeenCalled();
    expect(result.current.searchResults).toEqual([]);
    expect(result.current.isLoading).toBe(false);
  });

  it('clears prior results while a new query is pending', async () => {
    searchNotes.mockResolvedValueOnce({ results: [{ relPath: 'notes/old.md', title: 'Old result', snippet: 'old' }] });
    const { result } = renderHook(() => useNoteSearch({ nodes: [], catCounts: {} }));
    let firstSearch: Promise<void> = Promise.resolve();
    act(() => { firstSearch = result.current.handleSearchChange('old'); });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(180);
      await firstSearch;
    });
    expect(result.current.searchResults).toHaveLength(1);

    act(() => { void result.current.handleSearchChange('new'); });
    expect(result.current.searchResults).toEqual([]);
    expect(result.current.isLoading).toBe(true);
  });

  it('keeps legacy Portuguese note folders included in English search filters', async () => {
    searchNotes.mockResolvedValue({ results: [
      { relPath: 'notes/pessoas/ada.md', title: 'Ada', snippet: 'Person note' },
      { relPath: 'notes/arquitetura/go.md', title: 'Architecture', snippet: 'System note' },
      { relPath: 'notes/people/alan.md', title: 'Alan', snippet: 'Person note' },
      { relPath: 'notes/architecture/graph.md', title: 'Graph', snippet: 'System note' },
    ] });
    const { result } = renderHook(() => useNoteSearch({ nodes: [], catCounts: {} }));
    let search: Promise<void> = Promise.resolve();
    act(() => { search = result.current.handleSearchChange('note'); });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(180);
      await search;
    });

    act(() => result.current.setActiveFilter('People'));
    expect(result.current.filteredResults.map(item => item.title)).toEqual(['Ada', 'Alan']);
    act(() => result.current.setActiveFilter('Architecture'));
    expect(result.current.filteredResults.map(item => item.title)).toEqual(['Architecture', 'Graph']);
  });

  it('reports service failures as an explicit error state', async () => {
    searchNotes.mockRejectedValue(new Error('offline'));
    const { result } = renderHook(() => useNoteSearch({ nodes: [], catCounts: {} }));
    let search: Promise<void> = Promise.resolve();
    act(() => { search = result.current.handleSearchChange('AI'); });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(180);
      await search;
    });
    expect(result.current.searchError).toBe('offline');
    expect(result.current.hasSearched).toBe(true);
  });
});
