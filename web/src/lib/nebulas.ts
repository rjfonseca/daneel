export const DEFAULT_MAX_NEBULAS = 6;
export const STORAGE_KEY_MAX_NEBULAS = 'daneel_max_nebulas';

/**
 * Checks if a category corresponds to a direct child directory of `notes/`.
 * E.g.:
 * - "notes/arquitetura" -> true
 * - "notes/carreira" -> true
 * - "notes/ia" -> true
 * - "notes/ia/agentes" -> false (subdirectory)
 * - "sources/plans" -> false (outside notes)
 * - "daily" -> false
 */
export function isRootNoteCategory(category: string): boolean {
  if (!category) return false;
  const normalized = category.replace(/^\/+/, '').replace(/\/+$/, '');
  const parts = normalized.split('/');
  return parts.length === 2 && parts[0] === 'notes' && parts[1].length > 0;
}

export const KNOWN_SECTOR_SUBGROUPS = new Set([
  'saas', 'apis', 'stores',
  'platform', 'infra', 'core',
  'services', 'tools', 'apps',
  'mobile', 'web', 'shared',
  'data', 'analytics', 'edge',
  'fiscal', 'financeiro', 'produtos',
  'growth', 'sre', 'devops', 'security',
  'integracoes', 'billing', 'portal',
]);

export const REPO_INTERNAL_DIRS = new Set([
  'docs', 'doc', 'documentation',
  'specs', 'spec', 'plans', 'conductor',
  'runbooks', 'runbook', 'adrs', 'adr',
  'tracks', 'archive', 'superpowers',
  'vendor', 'node_modules', 'dist', 'build',
]);

export interface ProjectCategoryResult {
  category: string;
  displayName: string;
  domain?: string;
  domainDisplayName?: string;
  sector: string;
  sectorDisplayName: string;
  group?: string;
  repository?: string;
  isProject: boolean;
}

/**
 * Resolves the canonical project or repository category for a note path.
 * Groups subfolders of a repository (docs, specs, conductor, etc.)
 * under the root of that project or repository (e.g. `sources/gitlab/acme/edge/gateway` -> `GATEWAY`).
 */
export function resolveProjectCategory(
  relPath: string,
  rawCategory?: string,
  explicitSector?: string,
  explicitGroup?: string,
  explicitRepository?: string
): ProjectCategoryResult {
  const target = (rawCategory || relPath || '').replace(/^\/+/, '').replace(/\/+$/, '');
  if (!target) {
    return { category: 'root', displayName: 'ROOT', domain: 'root', domainDisplayName: 'ROOT', sector: 'root', sectorDisplayName: 'ROOT', isProject: false };
  }

  // 0. Explicit taxonomy from backend (OKF deterministic resolution)
  if (explicitRepository) {
    const groupName = explicitGroup || explicitSector || 'sources';
    return {
      category: target,
      displayName: explicitRepository.toUpperCase(),
      domain: (explicitSector || 'sources').toLowerCase(),
      domainDisplayName: (explicitSector || 'sources').toUpperCase(),
      sector: groupName.toLowerCase(),
      sectorDisplayName: groupName.toUpperCase(),
      group: explicitGroup,
      repository: explicitRepository,
      isProject: true,
    };
  }
  if (explicitGroup && !explicitRepository) {
    return {
      category: target,
      displayName: explicitGroup.toUpperCase(),
      domain: (explicitSector || 'sources').toLowerCase(),
      domainDisplayName: (explicitSector || 'sources').toUpperCase(),
      sector: explicitGroup.toLowerCase(),
      sectorDisplayName: explicitGroup.toUpperCase(),
      group: explicitGroup,
      isProject: true,
    };
  }

  const parts = target.split('/');

  // 1. sources/... -> Collapse repositories into their project root
  if (parts[0] === 'sources') {
    if (parts.length === 1) {
      return { category: 'sources', displayName: 'SOURCES', sector: 'sources', sectorDisplayName: 'SOURCES', isProject: true };
    }

    const provider = parts[1]; // e.g. github, gitlab, jira, reunioes, relatorios

    // GitHub: sources/github/<owner>/<repo>/... -> sources/github/<owner>/<repo>
    if (provider === 'github' || provider === 'github.com') {
      if (parts.length >= 4) {
        const owner = parts[2];
        const repo = parts[3];
        return {
          category: `sources/${provider}/${owner}/${repo}`,
          displayName: repo.toUpperCase(),
          sector: owner.toLowerCase(),
          sectorDisplayName: owner.toUpperCase(),
          isProject: true,
        };
      }
      if (parts.length === 3) {
        return {
          category: `sources/${provider}/${parts[2]}`,
          displayName: parts[2].toUpperCase(),
          sector: parts[2].toLowerCase(),
          sectorDisplayName: parts[2].toUpperCase(),
          isProject: true,
        };
      }
      return {
        category: `sources/${provider}`,
        displayName: provider.toUpperCase(),
        sector: provider.toLowerCase(),
        sectorDisplayName: provider.toUpperCase(),
        isProject: true,
      };
    }

    // GitLab: sources/gitlab/<group>/.../<repo>/[internal_dir]...
    if (provider === 'gitlab' || provider === 'gitlab.com') {
      // 1. General GitLab repository parsing with REPO_INTERNAL_DIRS (highest priority)
      let cutIndex = -1;
      for (let i = 3; i < parts.length; i++) {
        if (REPO_INTERNAL_DIRS.has(parts[i].toLowerCase())) {
          cutIndex = i;
          break;
        }
      }

      if (cutIndex !== -1) {
        const repoPath = parts.slice(0, cutIndex).join('/');
        const repoName = parts[cutIndex - 1];
        const parentSector = parts.slice(2, cutIndex - 1).join('/') || parts[2];
        const sectorName = parts[cutIndex - 2] || parts[2];
        const isOrgPath = parts.length > 4 && !KNOWN_SECTOR_SUBGROUPS.has(parts[2].toLowerCase());
        const sectorKey = isOrgPath
          ? (parts[3] || sectorName).toLowerCase()
          : (KNOWN_SECTOR_SUBGROUPS.has(sectorName.toLowerCase())
            ? sectorName.toLowerCase()
            : parentSector.toLowerCase());
        const displaySector = isOrgPath
          ? (parts[3] || sectorName).toUpperCase()
          : (KNOWN_SECTOR_SUBGROUPS.has(sectorName.toLowerCase())
            ? sectorName.toUpperCase()
            : (parts.length > 4 ? sectorName.toUpperCase() : parentSector.toUpperCase()));
        return {
          category: repoPath,
          displayName: repoName.toUpperCase(),
          sector: sectorKey,
          sectorDisplayName: displaySector,
          isProject: true,
        };
      }

      // 2. Check for known sector subgroups like saas/, apis/, stores/, etc.
      // Pattern A: sources/gitlab/<sector>/<repo>/...
      if (parts.length >= 4 && KNOWN_SECTOR_SUBGROUPS.has(parts[2].toLowerCase())) {
        const sector = parts[2].toLowerCase();
        const repo = parts[3];
        return {
          category: `sources/${provider}/${sector}/${repo}`,
          displayName: repo.toUpperCase(),
          sector,
          sectorDisplayName: sector.toUpperCase(),
          isProject: true,
        };
      }
      // Pattern B: sources/gitlab/<org>/<sector>/<repo>/...
      if (parts.length >= 5 && KNOWN_SECTOR_SUBGROUPS.has(parts[3].toLowerCase())) {
        const sector = parts[3].toLowerCase();
        const repo = parts[4];
        return {
          category: `sources/${provider}/${parts[2]}/${sector}/${repo}`,
          displayName: repo.toUpperCase(),
          sector,
          sectorDisplayName: sector.toUpperCase(),
          isProject: true,
        };
      }

      // Pattern C: sources/gitlab/<org>/<sector> (e.g. sources/gitlab/acme/saas)
      if (parts.length === 4 && KNOWN_SECTOR_SUBGROUPS.has(parts[3].toLowerCase())) {
        const sector = parts[3].toLowerCase();
        return {
          category: `sources/${provider}/${parts[2]}/${sector}`,
          displayName: sector.toUpperCase(),
          sector,
          sectorDisplayName: sector.toUpperCase(),
          isProject: true,
        };
      }

      // Pattern D: sources/gitlab/<org>/<group>/<repo>/... for any 5+ part path
      if (parts.length >= 5) {
        const sector = parts[3].toLowerCase();
        const last = parts[parts.length - 1];
        const isFile = last.includes('.');
        const repoParts = isFile ? parts.slice(0, parts.length - 1) : parts;
        const repoName = repoParts[repoParts.length - 1] || sector;
        return {
          category: repoParts.join('/'),
          displayName: repoName.toUpperCase(),
          sector,
          sectorDisplayName: sector.toUpperCase(),
          isProject: true,
        };
      }

      // If no internal dir found, if it ends with a file (e.g. .md), use parent dir as repo
      const last = parts[parts.length - 1];
      const isFile = last.includes('.');
      const repoParts = isFile ? parts.slice(0, parts.length - 1) : parts;
      const repoName = repoParts[repoParts.length - 1] || provider;
      const parentSector = repoParts.length > 3 ? repoParts.slice(2, repoParts.length - 1).join('/') : repoParts[2] || provider;
      const sectorName = repoParts.length > 3 ? repoParts[repoParts.length - 2] : parentSector;
      return {
        category: repoParts.join('/'),
        displayName: repoName.toUpperCase(),
        sector: parentSector.toLowerCase(),
        sectorDisplayName: sectorName.toUpperCase(),
        isProject: true,
      };
    }

    // Other sources: e.g. sources/jira/post-mortems, sources/reunioes, sources/relatorios
    if (parts.length >= 3) {
      const srcName = parts[1];
      const sub = parts[2];
      return {
        category: `sources/${srcName}/${sub}`,
        displayName: sub.toUpperCase(),
        sector: srcName.toLowerCase(),
        sectorDisplayName: srcName.toUpperCase(),
        isProject: true,
      };
    }
    return {
      category: `sources/${parts[1]}`,
      displayName: parts[1].toUpperCase(),
      sector: parts[1].toLowerCase(),
      sectorDisplayName: parts[1].toUpperCase(),
      isProject: true,
    };
  }

  // Non-sources (notes/..., daily/..., etc.):
  // If target ends with a file (e.g. something.md), use directory path
  const last = parts[parts.length - 1];
  const isFile = last.includes('.');
  const dirParts = isFile ? parts.slice(0, parts.length - 1) : parts;
  const dirPath = dirParts.join('/') || 'root';
  const lastDirPart = dirParts[dirParts.length - 1] || 'ROOT';

  if (dirParts[0] === 'notes') {
    const rootSection = dirParts.length >= 2 ? dirParts[1] : 'notes';
    return {
      category: dirPath,
      displayName: lastDirPart.toUpperCase(),
      sector: `notes/${rootSection}`,
      sectorDisplayName: rootSection.toUpperCase(),
      isProject: false,
    };
  }

  // Handle org-prefixed groups like "<org>/saas" or "<org>/saas/pocs"
  if (dirParts.length >= 2 && dirParts[0] !== 'notes' && dirParts[0] !== 'sources' && dirParts[0] !== 'daily') {
    const sectorName = dirParts[1];
    return {
      category: dirPath,
      displayName: lastDirPart.toUpperCase(),
      sector: sectorName.toLowerCase(),
      sectorDisplayName: sectorName.toUpperCase(),
      isProject: false,
    };
  }

  return {
    category: dirPath,
    displayName: lastDirPart.toUpperCase(),
    sector: dirParts[0] || 'root',
    sectorDisplayName: (dirParts[0] || 'ROOT').toUpperCase(),
    isProject: false,
  };
}

/**
 * Returns the display name for a nebula based on its directory name or project root.
 */
export function getCategoryDisplayName(category: string): string {
  if (!category) return 'ROOT';
  const clean = category.replace(/^__SECTOR_/, '').replace(/^__CAT_/, '').trim();

  // If path like "<org>/saas/pocs" -> last part is POCS
  const parts = clean.split('/');
  if (parts.length >= 3 && parts[0] !== 'notes' && parts[0] !== 'sources' && parts[0] !== 'daily') {
    return parts[parts.length - 1].toUpperCase();
  }

  return resolveProjectCategory(clean).displayName;
}

/**
 * Returns the sector display name for a category.
 */
export function getSectorDisplayName(category: string): string {
  if (!category) return 'ROOT';
  const clean = category.replace(/^__SECTOR_/, '').replace(/^__CAT_/, '').trim();

  // Strip org prefix like "<org>/saas" or "<org>/saas/pocs" -> SAAS
  const parts = clean.split('/');
  if (parts.length >= 2 && parts[0] !== 'notes' && parts[0] !== 'sources' && parts[0] !== 'daily') {
    return parts[1].toUpperCase();
  }

  return resolveProjectCategory(clean).sectorDisplayName;
}

export interface NebulaCategoryInfo {
  category: string;
  displayName: string;
  sector: string;
  sectorDisplayName: string;
  isRoot: boolean;
  isMandatoryRoot?: boolean;
  count: number;
  isOverviewVisible: boolean;
  dynSize: number;
  priorityRank: number;
}

export interface SectorAllocationInfo {
  sector: string;
  displayName: string;
  sectorDisplayName: string;
  categories: string[];
  totalNotes: number;
  isOverviewVisible: boolean;
  priorityRank: number;
}

export interface NebulaAllocations {
  categoryMap: Map<string, NebulaCategoryInfo>;
  overviewVisibleCategories: Set<string>;
  sectorMap: Map<string, SectorAllocationInfo>;
  overviewVisibleSectors: Set<string>;
  rootCount: number;
  otherCount: number;
}

/**
 * Computes nebula label allocations, sector rankings, and priority sizes.
 * All strategic root note categories (direct subdirectories under `notes/`) are
 * mandatory core sectors and are always visible at overview zoom.
 * `maxNebulas` specifies the maximum number of sectors and secondary project nebulas
 * simultaneously visible in overview zoom.
 */
export function calculateNebulaAllocations(
  categories: string[],
  catCounts: Record<string, number>,
  maxNebulas: number = DEFAULT_MAX_NEBULAS
): NebulaAllocations {
  const rootCats: string[] = [];
  const otherCats: string[] = [];

  const uniqueCats = Array.from(new Set(categories));
  for (const cat of uniqueCats) {
    if (isRootNoteCategory(cat)) {
      rootCats.push(cat);
    } else {
      otherCats.push(cat);
    }
  }

  // Sort descending by note count
  rootCats.sort((a, b) => (catCounts[b] || 0) - (catCounts[a] || 0));
  otherCats.sort((a, b) => (catCounts[b] || 0) - (catCounts[a] || 0));

  const overviewVisibleCategories = new Set<string>();
  const categoryMap = new Map<string, NebulaCategoryInfo>();

  let rank = 1;

  // Process root categories: direct subdirectories under notes/ are
  // strategic core sectors and ALWAYS visible in overview.
  rootCats.forEach((cat) => {
    const count = catCounts[cat] || 1;
    const isMandatoryRoot = true;
    const isOverviewVisible = true;
    overviewVisibleCategories.add(cat);
    const resolved = resolveProjectCategory(cat);
    categoryMap.set(cat, {
      category: cat,
      displayName: resolved.displayName,
      sector: resolved.sector,
      sectorDisplayName: resolved.sectorDisplayName,
      isRoot: true,
      isMandatoryRoot,
      count,
      isOverviewVisible,
      dynSize: 18,
      priorityRank: rank++,
    });
  });

  // Group project categories into sectors and calculate aggregate note counts
  const sectorGroups = new Map<string, { sectorDisplayName: string; categories: string[]; totalNotes: number }>();
  for (const cat of uniqueCats) {
    const resolved = resolveProjectCategory(cat);
    if (resolved.isProject && resolved.sector) {
      const sKey = resolved.sector.toLowerCase();
      const group = sectorGroups.get(sKey) || {
        sectorDisplayName: resolved.sectorDisplayName,
        categories: [],
        totalNotes: 0,
      };
      group.categories.push(cat);
      group.totalNotes += catCounts[cat] || 1;
      sectorGroups.set(sKey, group);
    }
  }

  // Sort sectors descending by aggregate note count
  const sortedSectors = Array.from(sectorGroups.entries()).sort(
    ([, a], [, b]) => b.totalNotes - a.totalNotes
  );

  const sectorMap = new Map<string, SectorAllocationInfo>();
  const overviewVisibleSectors = new Set<string>();

  sortedSectors.forEach(([sKey, group], index) => {
    const isOverviewVisible = index < maxNebulas;
    if (isOverviewVisible) {
      overviewVisibleSectors.add(sKey);
    }
    sectorMap.set(sKey, {
      sector: sKey,
      displayName: group.sectorDisplayName,
      sectorDisplayName: group.sectorDisplayName,
      categories: group.categories,
      totalNotes: group.totalNotes,
      isOverviewVisible,
      priorityRank: index + 1,
    });
  });

  // Process other categories (project nebulas from sources/ and subdirectories):
  // maxNebulas controls how many additional project nebulas are visible in overview.
  otherCats.forEach((cat, index) => {
    const isOverviewVisible = index < maxNebulas;
    if (isOverviewVisible) {
      overviewVisibleCategories.add(cat);
    }
    const count = catCounts[cat] || 1;
    const resolved = resolveProjectCategory(cat);
    categoryMap.set(cat, {
      category: cat,
      displayName: resolved.displayName,
      sector: resolved.sector,
      sectorDisplayName: resolved.sectorDisplayName,
      isRoot: false,
      isMandatoryRoot: false,
      count,
      isOverviewVisible,
      dynSize: isOverviewVisible ? 14 : Math.max(0.1, Math.min(6, count * 0.2)),
      priorityRank: rank++,
    });
  });

  return {
    categoryMap,
    overviewVisibleCategories,
    sectorMap,
    overviewVisibleSectors,
    rootCount: rootCats.length,
    otherCount: otherCats.length,
  };
}

export function loadMaxNebulas(): number {
  try {
    const raw = localStorage.getItem(STORAGE_KEY_MAX_NEBULAS);
    if (!raw) return DEFAULT_MAX_NEBULAS;
    const parsed = parseInt(raw, 10);
    if (isNaN(parsed) || parsed < 1) return DEFAULT_MAX_NEBULAS;
    return Math.min(32, Math.max(1, parsed));
  } catch {
    return DEFAULT_MAX_NEBULAS;
  }
}

export function saveMaxNebulas(value: number): void {
  try {
    const clamped = Math.min(32, Math.max(1, value));
    localStorage.setItem(STORAGE_KEY_MAX_NEBULAS, String(clamped));
  } catch (e) {
    console.error('Failed to save max nebulas to localStorage', e);
  }
}
