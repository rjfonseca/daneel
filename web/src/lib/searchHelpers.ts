import type { SearchResultItem } from '../types/galaxy';

export const normalizeSearchPath = (path: string): string => path
  .replace(/\\/g, '/')
  .replace(/\/+/g, '/')
  .replace(/^\.\//, '')
  .replace(/^\/+|\/+$/g, '')
  .toLocaleLowerCase();

export const isAutomationSatellite = (node: SearchResultItem): boolean => {
  if (node.isArms) return true;
  const category = (node.category || '').toLocaleLowerCase();
  const id = (node.id || '').toLocaleLowerCase();
  const ring = (node.ring || '').toLocaleLowerCase();
  return category.startsWith('arms.') || category === 'arms' ||
    ['skills', 'routines', 'harvesters'].includes(ring) ||
    id.startsWith('arms:') || id.startsWith('harvester:') || id.startsWith('routine:') || id.startsWith('skill:');
};

const resultPriority = (result: SearchResultItem): number => {
  if (!result.isCat && !isAutomationSatellite(result)) return 3;
  if (isAutomationSatellite(result)) return 2;
  return 1;
};

export const deduplicateSearchResults = (results: SearchResultItem[]): SearchResultItem[] => {
  const retained = new Map<string, SearchResultItem>();
  for (const result of results) {
    const key = normalizeSearchPath(result.relPath || result.id || result.title);
    const previous = retained.get(key);
    if (!previous || resultPriority(result) > resultPriority(previous)) retained.set(key, result);
  }
  return [...retained.values()];
};

export const stripSearchMarkup = (value: string): string => value
  .replace(/!\[([^\]]*)\]\([^)]*\)/g, '$1')
  .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
  .replace(/^\s{0,3}#{1,6}\s+/gm, '')
  .replace(/^\s*[-*+]\s+/gm, '')
  .replace(/^\s{0,3}>+\s?/gm, '')
  .replace(/(`{1,3}|\*{1,2}|_{1,2}|~~)/g, '')
  .replace(/[ \t]{2,}/g, ' ')
  .trim();
