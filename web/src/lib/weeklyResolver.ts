import type { GalaxyNode } from '../types/galaxy';

/**
 * Normalizes a path for comparison (lowercased, forward slashes, trims leading/trailing slashes).
 */
export function normalizePath(path: string): string {
  return path.toLowerCase().replace(/\\/g, '/').replace(/^\/+|\/+$/g, '');
}

/**
 * Resolves the weekly summary note for a given week and year.
 *
 * Evaluation hierarchy:
 * 1. Explicit custom pattern (e.g. "weekly/{{year}}-W{{week}}.md")
 * 2. Standard second-brain conventions:
 *    - weekly/${year}-W${weekPadded}.md
 *    - weekly/${year}-W${week}.md
 *    - weekly/${year}-${weekPadded}.md
 *    - periodic/weekly/${year}-W${weekPadded}.md
 *    - periodic/${year}/weekly/W${weekPadded}.md
 *    - journals/weekly/${year}-W${weekPadded}.md
 *    - notes/weekly/${year}-W${weekPadded}.md
 * 3. Heuristic / fuzzy search: any note whose normalized id/path or title
 *    matches both year and week token (e.g. "W38", "w38", "week-38", "week 38").
 */
export function findWeeklySummaryNote(
  nodes: GalaxyNode[],
  week: number,
  year: number,
  customPattern?: string
): GalaxyNode | null {
  if (!nodes || nodes.length === 0) return null;

  const weekPadded = String(week).padStart(2, '0');
  const weekRaw = String(week);
  const yearStr = String(year);

  // 1. Explicit Custom Pattern
  if (customPattern && customPattern.trim().length > 0) {
    const substituted = customPattern
      .replace(/\{\{year\}\}/gi, yearStr)
      .replace(/\{\{week\}\}/gi, weekPadded)
      .replace(/\{\{week_raw\}\}/gi, weekRaw)
      .replace(/\{\{weekNum\}\}/gi, weekRaw);

    const normSubstituted = normalizePath(substituted);
    const exactMatch = nodes.find(n => {
      const normId = normalizePath(n.id || '');
      const normRel = normalizePath(n.relPath || '');
      return normId === normSubstituted || normRel === normSubstituted || normId.endsWith(normSubstituted);
    });
    if (exactMatch) return exactMatch;
  }

  // 2. Standard Convention Candidates
  const candidates = [
    `weekly/${yearStr}-w${weekPadded}.md`,
    `weekly/${yearStr}-w${weekRaw}.md`,
    `weekly/${yearStr}-${weekPadded}.md`,
    `periodic/weekly/${yearStr}-w${weekPadded}.md`,
    `periodic/${yearStr}/weekly/w${weekPadded}.md`,
    `periodic/${yearStr}/w${weekPadded}.md`,
    `journals/weekly/${yearStr}-w${weekPadded}.md`,
    `notes/weekly/${yearStr}-w${weekPadded}.md`,
    `weekly/w${weekPadded}.md`,
  ];

  for (const candidate of candidates) {
    const normCand = normalizePath(candidate);
    const match = nodes.find(n => {
      const normId = normalizePath(n.id || '');
      const normRel = normalizePath(n.relPath || '');
      return normId === normCand || normRel === normCand || normId.endsWith(normCand);
    });
    if (match) return match;
  }

  // 3. Heuristic Search: matches both year and week
  // e.g. "weekly", "w38" or "week-38", and "2026"
  const weekRegex = new RegExp(`(^|[\\b\\-_/\\s])(w${weekPadded}|w${weekRaw}|week[\\-_\\s]?${weekPadded}|week[\\-_\\s]?${weekRaw})([\\b\\-_/\\s.]|$)`, 'i');
  const yearRegex = new RegExp(`(^|[\\b\\-_/\\s])${yearStr}([\\b\\-_/\\s.]|$)`);

  // First check nodes in weekly / periodic directories
  const periodicMatch = nodes.find(n => {
    const norm = normalizePath(n.id || n.relPath || '');
    const isWeeklyCategory = norm.includes('weekly') || norm.includes('periodic');
    if (!isWeeklyCategory) return false;
    return weekRegex.test(norm) && yearRegex.test(norm);
  });
  if (periodicMatch) return periodicMatch;

  // General fuzzy check across title and path
  const generalMatch = nodes.find(n => {
    const text = `${n.id} ${n.relPath || ''} ${n.title || ''}`.toLowerCase();
    return weekRegex.test(text) && yearRegex.test(text);
  });
  if (generalMatch) return generalMatch;

  return null;
}
