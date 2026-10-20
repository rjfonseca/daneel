import { describe, it, expect } from 'vitest';
import {
  isRootNoteCategory,
  getCategoryDisplayName,
  resolveProjectCategory,
  calculateNebulaAllocations,
  DEFAULT_MAX_NEBULAS
} from '../src/lib/nebulas';

describe('Nebula Labels & Category Allocation', () => {
  describe('isRootNoteCategory', () => {
    it('identifies root directories inside notes/ correctly', () => {
      expect(isRootNoteCategory('notes/arquitetura')).toBe(true);
      expect(isRootNoteCategory('notes/carreira')).toBe(true);
      expect(isRootNoteCategory('notes/dados')).toBe(true);
      expect(isRootNoteCategory('notes/ia')).toBe(true);
      expect(isRootNoteCategory('notes/pessoas')).toBe(true);
      expect(isRootNoteCategory('/notes/engenharia/')).toBe(true);
    });

    it('rejects subdirectories deeper than 1 level in notes/', () => {
      expect(isRootNoteCategory('notes/ia/agentes')).toBe(false);
      expect(isRootNoteCategory('notes/carreira/avaliacoes')).toBe(false);
      expect(isRootNoteCategory('notes/pessoas/candidatos/entrevistas')).toBe(false);
    });

    it('rejects directories outside notes/', () => {
      expect(isRootNoteCategory('sources/plans')).toBe(false);
      expect(isRootNoteCategory('sources/specs')).toBe(false);
      expect(isRootNoteCategory('daily')).toBe(false);
      expect(isRootNoteCategory('notes')).toBe(false);
      expect(isRootNoteCategory('')).toBe(false);
    });
  });

  describe('getCategoryDisplayName', () => {
    it('returns the directory name in uppercase', () => {
      expect(getCategoryDisplayName('notes/arquitetura')).toBe('ARQUITETURA');
      expect(getCategoryDisplayName('notes/ia/agentes')).toBe('AGENTES');
      expect(getCategoryDisplayName('sources/plans')).toBe('PLANS');
      expect(getCategoryDisplayName('daily')).toBe('DAILY');
    });
  });

  describe('resolveProjectCategory', () => {
    it('collapses deeply nested GitLab repository files into repository root', () => {
      const res = resolveProjectCategory('sources/gitlab/acme/edge/gateway/docs/superpowers/specs/2026-03-09.md');
      expect(res.category).toBe('sources/gitlab/acme/edge/gateway');
      expect(res.displayName).toBe('GATEWAY');
      expect(res.isProject).toBe(true);
    });

    it('collapses conductor tracks inside GitLab repository into repository root', () => {
      const res = resolveProjectCategory('sources/gitlab/acme/edge/gateway/conductor/tracks/recup/spec.md');
      expect(res.category).toBe('sources/gitlab/acme/edge/gateway');
      expect(res.displayName).toBe('GATEWAY');
      expect(res.isProject).toBe(true);
    });

    it('collapses GitHub repository documentation into repository root', () => {
      const res = resolveProjectCategory('sources/github/acme/data-pipeline/docs/index.md');
      expect(res.category).toBe('sources/github/acme/data-pipeline');
      expect(res.displayName).toBe('DATA-PIPELINE');
      expect(res.isProject).toBe(true);
    });

    it('preserves non-repository directories and subdirectories', () => {
      const res1 = resolveProjectCategory('notes/ia/agentes/langgraph.md');
      expect(res1.category).toBe('notes/ia/agentes');
      expect(res1.displayName).toBe('AGENTES');
      expect(res1.isProject).toBe(false);

      const res2 = resolveProjectCategory('daily/2026-09-09.md');
      expect(res2.category).toBe('daily');
      expect(res2.displayName).toBe('DAILY');
      expect(res2.isProject).toBe(false);
    });
  });

  describe('calculateNebulaAllocations', () => {
    const categories = [
      'notes/pessoas',
      'notes/ferramentas',
      'notes/carreira',
      'notes/processos',
      'notes/organizacao',
      'notes/ia',
      'notes/arquitetura',
      'notes/go',
      'notes/dados',
      'notes/engenharia',
      'notes/sistemas',
      'notes/infra',
      'notes/linguagens',
      'notes/observabilidade',
      'notes/seguranca',
      'notes/produto',
      // Subdirectories
      'notes/ia/agentes',
      'notes/carreira/avaliacoes',
      'notes/pessoas/candidatos',
      // External sources
      'sources/plans',
      'sources/specs',
    ];

    const catCounts: Record<string, number> = {
      'notes/pessoas': 267,
      'notes/ferramentas': 54,
      'notes/carreira': 52,
      'notes/processos': 38,
      'notes/organizacao': 35,
      'notes/ia': 31,
      'notes/arquitetura': 30,
      'notes/go': 28,
      'notes/dados': 23,
      'notes/engenharia': 22,
      'notes/sistemas': 19,
      'notes/infra': 16,
      'notes/linguagens': 13,
      'notes/observabilidade': 10,
      'notes/seguranca': 7,
      'notes/produto': 1,
      // Subdirectories
      'notes/ia/agentes': 13,
      'notes/carreira/avaliacoes': 10,
      'notes/pessoas/candidatos': 3,
      // External
      'sources/plans': 25,
      'sources/specs': 15,
    };

    it('allocates all mandatory root categories (>=2 notes) and project nebulas up to maxNebulas', () => {
      const result = calculateNebulaAllocations(categories, catCounts, DEFAULT_MAX_NEBULAS);

      expect(DEFAULT_MAX_NEBULAS).toBe(6);
      expect(result.rootCount).toBe(16);

      // Mandatory root note categories (count >= 2) must all be visible
      const mandatoryRoots = [
        'notes/pessoas',
        'notes/ferramentas',
        'notes/carreira',
        'notes/processos',
        'notes/organizacao',
        'notes/ia',
        'notes/arquitetura',
        'notes/go',
        'notes/dados',
        'notes/engenharia',
        'notes/sistemas',
        'notes/infra',
        'notes/linguagens',
        'notes/observabilidade',
        'notes/seguranca',
      ];

      mandatoryRoots.forEach(cat => {
        expect(result.overviewVisibleCategories.has(cat)).toBe(true);
        expect(result.categoryMap.get(cat)?.isOverviewVisible).toBe(true);
        expect(result.categoryMap.get(cat)?.isMandatoryRoot).toBe(true);
        expect(result.categoryMap.get(cat)?.dynSize).toBe(18);
      });

      // notes/produto has 1 note, but as a level-1 notes/ category it is strategic and mandatory
      expect(result.overviewVisibleCategories.has('notes/produto')).toBe(true);
      expect(result.categoryMap.get('notes/produto')?.isMandatoryRoot).toBe(true);

      // Top project / other categories up to maxNebulas (6) are also visible
      expect(result.overviewVisibleCategories.has('sources/plans')).toBe(true); // 25 notes
      expect(result.overviewVisibleCategories.has('sources/specs')).toBe(true); // 15 notes
      expect(result.overviewVisibleCategories.has('notes/ia/agentes')).toBe(true); // 13 notes
    });

    it('allocates all mandatory root categories plus custom number of project nebulas', () => {
      const result = calculateNebulaAllocations(categories, catCounts, 2);

      // Mandatory root categories are always preserved
      expect(result.overviewVisibleCategories.has('notes/arquitetura')).toBe(true);
      expect(result.overviewVisibleCategories.has('notes/pessoas')).toBe(true);
      expect(result.overviewVisibleCategories.has('notes/carreira')).toBe(true);
      expect(result.overviewVisibleCategories.has('notes/produto')).toBe(true);

      // Only top 2 non-root categories are visible
      expect(result.overviewVisibleCategories.has('sources/plans')).toBe(true); // top 1
      expect(result.overviewVisibleCategories.has('sources/specs')).toBe(true); // top 2
      expect(result.overviewVisibleCategories.has('notes/ia/agentes')).toBe(false); // top 3 rejected
    });

    it('groups Git repositories by sector and caps overviewVisibleSectors to maxNebulas', () => {
      const projectCats = [
        'sources/gitlab/saas/frontend',
        'sources/gitlab/saas/backend',
        'sources/gitlab/apis/customer-api',
        'sources/gitlab/apis/order-api',
        'sources/gitlab/tools/cli',
        'sources/gitlab/infra/terraform',
        'sources/gitlab/mobile/app',
        'sources/gitlab/ai/inference',
      ];
      const projectCounts: Record<string, number> = {
        'sources/gitlab/saas/frontend': 50,
        'sources/gitlab/saas/backend': 60, // saas total: 110
        'sources/gitlab/apis/customer-api': 40,
        'sources/gitlab/apis/order-api': 35, // apis total: 75
        'sources/gitlab/infra/terraform': 30, // infra total: 30
        'sources/gitlab/mobile/app': 20, // mobile total: 20
        'sources/gitlab/ai/inference': 15, // ai total: 15
        'sources/gitlab/tools/cli': 5, // tools total: 5
      };

      // With maxNebulas = 3, only top 3 sectors (saas, apis, infra) are overview visible
      const result = calculateNebulaAllocations(projectCats, projectCounts, 3);
      expect(result.sectorMap.size).toBe(6);
      expect(result.overviewVisibleSectors.size).toBe(3);

      expect(result.sectorMap.get('saas')?.isOverviewVisible).toBe(true);
      expect(result.sectorMap.get('saas')?.priorityRank).toBe(1);
      expect(result.sectorMap.get('saas')?.totalNotes).toBe(110);

      expect(result.sectorMap.get('apis')?.isOverviewVisible).toBe(true);
      expect(result.sectorMap.get('apis')?.priorityRank).toBe(2);
      expect(result.sectorMap.get('apis')?.totalNotes).toBe(75);

      expect(result.sectorMap.get('infra')?.isOverviewVisible).toBe(true);
      expect(result.sectorMap.get('infra')?.priorityRank).toBe(3);
      expect(result.sectorMap.get('infra')?.totalNotes).toBe(30);

      // Remaining sectors exceed maxNebulas and are not overview visible
      expect(result.sectorMap.get('mobile')?.isOverviewVisible).toBe(false);
      expect(result.sectorMap.get('mobile')?.priorityRank).toBe(4);

      expect(result.sectorMap.get('ai')?.isOverviewVisible).toBe(false);
      expect(result.sectorMap.get('ai')?.priorityRank).toBe(5);

      expect(result.sectorMap.get('tools')?.isOverviewVisible).toBe(false);
      expect(result.sectorMap.get('tools')?.priorityRank).toBe(6);
    });
  });
});
