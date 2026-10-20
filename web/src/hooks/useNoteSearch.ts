import { useState, useMemo, useCallback, useEffect, useRef } from 'react';
import { client } from '../lib/apiClient';
import { getCategoryDisplayName } from '../lib/nebulas';
import { deduplicateSearchResults, isAutomationSatellite } from '../lib/searchHelpers';
import type { GalaxyNode, SearchResultItem } from '../types/galaxy';

interface UseNoteSearchParams {
  nodes: GalaxyNode[];
  catCounts: Record<string, number>;
}

const SEARCH_DEBOUNCE_MS = 180;

export interface UseNoteSearchResult {
  searchQuery: string;
  setSearchQuery: (query: string) => void;
  activeFilter: string;
  setActiveFilter: (filter: string) => void;
  searchResults: SearchResultItem[];
  filteredResults: SearchResultItem[];
  isLoading: boolean;
  searchError: string | null;
  hasSearched: boolean;
  handleSearchChange: (val: string) => Promise<void>;
  resetSearch: () => void;
}

export const useNoteSearch = ({ nodes, catCounts }: UseNoteSearchParams): UseNoteSearchResult => {
  const [searchQuery, setSearchQuery] = useState('');
  const [searchResults, setSearchResults] = useState<SearchResultItem[]>([]);
  const [activeFilter, setActiveFilter] = useState('All');
  const [isLoading, setIsLoading] = useState(false);
  const [searchError, setSearchError] = useState<string | null>(null);
  const [hasSearched, setHasSearched] = useState(false);
  const requestId = useRef(0);
  const debounceTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const resolvePendingSearch = useRef<(() => void) | null>(null);

  const cancelPendingSearch = useCallback(() => {
    if (debounceTimer.current) clearTimeout(debounceTimer.current);
    debounceTimer.current = null;
    resolvePendingSearch.current?.();
    resolvePendingSearch.current = null;
  }, []);

  useEffect(() => () => {
    requestId.current += 1;
    cancelPendingSearch();
  }, [cancelPendingSearch]);

  const handleSearchChange = useCallback((val: string): Promise<void> => {
    setSearchQuery(val);
    const query = val.trim();
    const currentRequest = ++requestId.current;
    cancelPendingSearch();
    setSearchError(null);
    setSearchResults([]);

    if (query.length < 2) {
      setSearchResults([]);
      setIsLoading(false);
      setHasSearched(false);
      return Promise.resolve();
    }

    setIsLoading(true);
    setHasSearched(false);

    return new Promise(resolve => {
      resolvePendingSearch.current = resolve;
      debounceTimer.current = setTimeout(async () => {
        debounceTimer.current = null;
        resolvePendingSearch.current = null;
        try {
          const res = await client.searchNotes({ query, limit: 50 });
          if (currentRequest !== requestId.current) return;

          const normalizedQuery = query.toLocaleLowerCase();
          const clusterMatches: SearchResultItem[] = Object.keys(catCounts)
            .filter(category => category.toLocaleLowerCase().includes(normalizedQuery))
            .slice(0, 3)
            .map(category => ({
              relPath: `__CAT_${category}`,
              title: getCategoryDisplayName(category),
              snippet: `Cluster with ${catCounts[category]} note(s).`,
              isCat: true,
            }));

          const automationMatches: SearchResultItem[] = nodes
            .filter(node => {
              const result = node as SearchResultItem;
              return isAutomationSatellite(result) && [node.title, node.id, node.description]
                .some(value => (value || '').toLocaleLowerCase().includes(normalizedQuery));
            })
            .slice(0, 4)
            .map(node => ({
              id: node.id,
              relPath: node.relPath || node.id,
              title: node.title || node.id,
              snippet: node.description || 'Automation satellite',
              ring: node.ring,
              status: node.status,
              icon: node.icon,
              category: node.category,
              isArms: true,
            }));

          const noteMatches: SearchResultItem[] = (res.results || []).map(result => ({
            relPath: result.relPath,
            title: result.title,
            snippet: result.snippet,
          }));

          setSearchResults(deduplicateSearchResults([...noteMatches, ...clusterMatches, ...automationMatches]));
          setHasSearched(true);
        } catch (error) {
          if (currentRequest !== requestId.current) return;
          setSearchResults([]);
          setSearchError(error instanceof Error ? error.message : 'Search is unavailable.');
          setHasSearched(true);
        } finally {
          if (currentRequest === requestId.current) setIsLoading(false);
          resolve();
        }
      }, SEARCH_DEBOUNCE_MS);
    });
  }, [cancelPendingSearch, catCounts, nodes]);

  const resetSearch = useCallback(() => {
    requestId.current += 1;
    cancelPendingSearch();
    setSearchQuery('');
    setSearchResults([]);
    setSearchError(null);
    setIsLoading(false);
    setHasSearched(false);
  }, [cancelPendingSearch]);

  const filteredResults = useMemo(() => {
    return searchResults.filter(result => {
      if (activeFilter === 'All') return true;
      const path = (result.relPath || result.id || '').toLocaleLowerCase();
      if (activeFilter === 'People') return path.includes('/people/') || path.startsWith('people/') || path.includes('/pessoas/') || path.startsWith('pessoas/');
      if (activeFilter === 'Architecture') return path.includes('/architecture/') || path.startsWith('architecture/') || path.includes('/arquitetura/') || path.startsWith('arquitetura/');
      if (activeFilter === 'Daily') return path.startsWith('daily/');
      if (activeFilter === 'Sources') return path.startsWith('sources/');
      return true;
    });
  }, [searchResults, activeFilter]);

  return {
    searchQuery,
    setSearchQuery,
    activeFilter,
    setActiveFilter,
    searchResults,
    filteredResults,
    isLoading,
    searchError,
    hasSearched,
    handleSearchChange,
    resetSearch,
  };
};
