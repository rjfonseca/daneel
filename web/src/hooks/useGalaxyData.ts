import { useState, useEffect } from 'react';
import { client } from '../lib/apiClient';
import { resolveProjectCategory } from '../lib/nebulas';
import type { GalaxyStats, PinnedAnchor } from '../gen/api/v1/graph_pb';
import type { GalaxyDataState, GalaxyNode, GalaxyLink } from '../types/galaxy';

export interface UseGalaxyDataResult {
  data: GalaxyDataState;
  unindexedLinks: GalaxyLink[];
  stats: GalaxyStats | undefined;
  catCounts: Record<string, number>;
  pinnedAnchors: PinnedAnchor[];
  weeklyPattern: string;
  isLoading: boolean;
  error: Error | null;
  reloadGalaxy: () => void;
}

export const useGalaxyData = (): UseGalaxyDataResult => {
  const [data, setData] = useState<GalaxyDataState>({ nodes: [], links: [] });
  const [unindexedLinks, setUnindexedLinks] = useState<GalaxyLink[]>([]);
  const [stats, setStats] = useState<GalaxyStats | undefined>(undefined);
  const [catCounts, setCatCounts] = useState<Record<string, number>>({});
  const [pinnedAnchors, setPinnedAnchors] = useState<PinnedAnchor[]>([]);
  const [weeklyPattern, setWeeklyPattern] = useState<string>('');
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [error, setError] = useState<Error | null>(null);
  const [trigger, setTrigger] = useState(0);

  const reloadGalaxy = () => setTrigger(prev => prev + 1);

  useEffect(() => {
    let isMounted = true;
    setIsLoading(true);

    client.getGalaxy({})
      .then(res => {
        if (!isMounted) return;
        const safeNodes: GalaxyNode[] = (res.nodes || []).filter(n => Boolean(n.id)).map(n => {
          const resolved = resolveProjectCategory(n.id, n.category, n.domain || ('sector' in n && typeof n.sector === 'string' ? n.sector : ''), n.group, n.repository);
          return {
            ...n,
            category: resolved.category,
            domain: n.domain || resolved.domain || resolved.sector,
            sector: n.domain || resolved.domain || resolved.sector,
            group: n.group || '',
            repository: n.repository || '',
          };
        });
        const nodeIds = new Set(safeNodes.map(n => n.id));
        const rawLinks = (res.links || []) as GalaxyLink[];
        const safeLinks: GalaxyLink[] = rawLinks.filter(
          l => nodeIds.has(l.source) && nodeIds.has(l.target)
        );
        setUnindexedLinks(rawLinks.filter(l => !nodeIds.has(l.source) || !nodeIds.has(l.target)));

        const counts: Record<string, number> = {};
        safeNodes.forEach(n => {
          const isMemory = !n.ring || n.ring === 'memory';
          if (isMemory) {
            const c = n.category || 'root';
            counts[c] = (counts[c] || 0) + 1;
          }
        });

        setCatCounts(counts);
        setStats(res.stats);
        setPinnedAnchors(res.pinnedAnchors || []);
        setWeeklyPattern(res.weeklyPattern || '');
        setData({ nodes: safeNodes, links: safeLinks });
        setIsLoading(false);
      })
      .catch(err => {
        if (!isMounted) return;
        console.error("Failed to load galaxy from backend:", err);
        setError(err instanceof Error ? err : new Error(String(err)));
        setData({ nodes: [], links: [] });
        setUnindexedLinks([]);
        setIsLoading(false);
      });

    return () => {
      isMounted = false;
    };
  }, [trigger]);

  return { data, unindexedLinks, stats, catCounts, pinnedAnchors, weeklyPattern, isLoading, error, reloadGalaxy };
};
