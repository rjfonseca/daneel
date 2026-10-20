import { describe, it, expect } from 'vitest';
import {
  REPO_INTERNAL_DIRS,
  resolveProjectCategory,
  getCategoryDisplayName,
  getSectorDisplayName,
} from '../src/lib/nebulas';

describe('Hierarchical Sectors & Repository Nebulas (nebulas.ts)', () => {
  describe('REPO_INTERNAL_DIRS sanitization', () => {
    it('does not contain application and code component directories', () => {
      const removedDirs = ['frontend', 'backend', 'web', 'ui', 'tools', 'tool', 'packages', 'package', 'pkg', 'src', 'app', 'apps', 'cmd', 'internal'];
      for (const dir of removedDirs) {
        expect(REPO_INTERNAL_DIRS.has(dir)).toBe(false);
      }
    });

    it('retains essential documentation and spec directories', () => {
      const docDirs = ['docs', 'doc', 'documentation', 'specs', 'spec', 'plans', 'conductor', 'runbooks', 'runbook', 'adrs', 'adr'];
      for (const dir of docDirs) {
        expect(REPO_INTERNAL_DIRS.has(dir)).toBe(true);
      }
    });
  });

  describe('resolveProjectCategory with hierarchical sectors', () => {
    it('resolves GitLab repositories under saas/ to the repository level', () => {
      const resDocs = resolveProjectCategory('sources/gitlab/saas/frontend/docs/readme.md');
      expect(resDocs).toEqual({
        category: 'sources/gitlab/saas/frontend',
        displayName: 'FRONTEND',
        sector: 'saas',
        sectorDisplayName: 'SAAS',
        isProject: true,
      });

      const resRoot = resolveProjectCategory('sources/gitlab/saas/frontend/README.md');
      expect(resRoot).toEqual({
        category: 'sources/gitlab/saas/frontend',
        displayName: 'FRONTEND',
        sector: 'saas',
        sectorDisplayName: 'SAAS',
        isProject: true,
      });

      const resNested = resolveProjectCategory('sources/gitlab/saas/backend/sub/api.md');
      expect(resNested).toEqual({
        category: 'sources/gitlab/saas/backend',
        displayName: 'BACKEND',
        sector: 'saas',
        sectorDisplayName: 'SAAS',
        isProject: true,
      });
    });

    it('resolves GitLab repositories under apis/ to the repository level', () => {
      const res = resolveProjectCategory('sources/gitlab/apis/customer-api/docs/endpoints.md');
      expect(res).toEqual({
        category: 'sources/gitlab/apis/customer-api',
        displayName: 'CUSTOMER-API',
        sector: 'apis',
        sectorDisplayName: 'APIS',
        isProject: true,
      });
    });

    it('resolves GitLab repositories nested under an organization with saas or apis', () => {
      const resSaas = resolveProjectCategory('sources/gitlab/myorg/saas/portal/docs/arch.md');
      expect(resSaas).toEqual({
        category: 'sources/gitlab/myorg/saas/portal',
        displayName: 'PORTAL',
        sector: 'saas',
        sectorDisplayName: 'SAAS',
        isProject: true,
      });

      const resApis = resolveProjectCategory('sources/gitlab/myorg/apis/gateway/specs/v1.md');
      expect(resApis).toEqual({
        category: 'sources/gitlab/myorg/apis/gateway',
        displayName: 'GATEWAY',
        sector: 'apis',
        sectorDisplayName: 'APIS',
        isProject: true,
      });

      const resStores = resolveProjectCategory('sources/gitlab/stores/checkout/docs/readme.md');
      expect(resStores).toEqual({
        category: 'sources/gitlab/stores/checkout',
        displayName: 'CHECKOUT',
        sector: 'stores',
        sectorDisplayName: 'STORES',
        isProject: true,
      });

      const resOrgStores = resolveProjectCategory('sources/gitlab/myorg/stores/catalog/specs/api.md');
      expect(resOrgStores).toEqual({
        category: 'sources/gitlab/myorg/stores/catalog',
        displayName: 'CATALOG',
        sector: 'stores',
        sectorDisplayName: 'STORES',
        isProject: true,
      });
    });

    it('does not collapse repositories named tools or frontend into their parent group', () => {
      const res = resolveProjectCategory('sources/gitlab/saas/tools/docs/cli.md');
      expect(res).toEqual({
        category: 'sources/gitlab/saas/tools',
        displayName: 'TOOLS',
        sector: 'saas',
        sectorDisplayName: 'SAAS',
        isProject: true,
      });
    });

    it('resolves GitHub repositories with owner sector', () => {
      const res = resolveProjectCategory('sources/github/acme/data-pipeline/docs/index.md');
      expect(res).toEqual({
        category: 'sources/github/acme/data-pipeline',
        displayName: 'DATA-PIPELINE',
        sector: 'acme',
        sectorDisplayName: 'ACME',
        isProject: true,
      });
    });

    it('resolves notes categories and subdirectories with correct sectors', () => {
      const resArch = resolveProjectCategory('notes/arquitetura/cache.md');
      expect(resArch).toEqual({
        category: 'notes/arquitetura',
        displayName: 'ARQUITETURA',
        sector: 'notes/arquitetura',
        sectorDisplayName: 'ARQUITETURA',
        isProject: false,
      });

      const resSub = resolveProjectCategory('notes/ia/agentes/langgraph.md');
      expect(resSub).toEqual({
        category: 'notes/ia/agentes',
        displayName: 'AGENTES',
        sector: 'notes/ia',
        sectorDisplayName: 'IA',
        isProject: false,
      });
    });

    it('resolves daily and root notes correctly', () => {
      const resDaily = resolveProjectCategory('daily/2026-09-09.md');
      expect(resDaily).toEqual({
        category: 'daily',
        displayName: 'DAILY',
        sector: 'daily',
        sectorDisplayName: 'DAILY',
        isProject: false,
      });

      const resRoot = resolveProjectCategory('README.md');
      expect(resRoot).toEqual({
        category: 'root',
        displayName: 'ROOT',
        sector: 'root',
        sectorDisplayName: 'ROOT',
        isProject: false,
      });
    });

    it('resolves explicit taxonomy passed from backend nodes without ambiguity', () => {
      const res = resolveProjectCategory(
        'sources/gitlab/acme/saas/pocs/my-repo/docs/guide.md',
        'sources/gitlab/acme/saas/pocs/my-repo',
        'sources/gitlab',
        'acme/saas/pocs',
        'my-repo'
      );
      expect(res).toEqual({
        category: 'sources/gitlab/acme/saas/pocs/my-repo',
        displayName: 'MY-REPO',
        domain: 'sources/gitlab',
        domainDisplayName: 'SOURCES/GITLAB',
        group: 'acme/saas/pocs',
        repository: 'my-repo',
        sector: 'acme/saas/pocs',
        sectorDisplayName: 'ACME/SAAS/POCS',
        isProject: true,
      });

      const resGroup = resolveProjectCategory(
        'sources/gitlab/acme/saas/pocs/index.md',
        'sources/gitlab/acme/saas/pocs',
        'sources/gitlab',
        'acme/saas/pocs',
        ''
      );
      expect(resGroup).toEqual({
        category: 'sources/gitlab/acme/saas/pocs',
        displayName: 'ACME/SAAS/POCS',
        domain: 'sources/gitlab',
        domainDisplayName: 'SOURCES/GITLAB',
        group: 'acme/saas/pocs',
        sector: 'acme/saas/pocs',
        sectorDisplayName: 'ACME/SAAS/POCS',
        isProject: true,
      });
    });
  });

  describe('getCategoryDisplayName and getSectorDisplayName', () => {
    it('returns appropriate display names for categories and sectors', () => {
      expect(getCategoryDisplayName('sources/gitlab/saas/frontend')).toBe('FRONTEND');
      expect(getSectorDisplayName('sources/gitlab/saas/frontend')).toBe('SAAS');

      expect(getCategoryDisplayName('sources/gitlab/apis/customer-api')).toBe('CUSTOMER-API');
      expect(getSectorDisplayName('sources/gitlab/apis/customer-api')).toBe('APIS');

      expect(getCategoryDisplayName('notes/ia/agentes')).toBe('AGENTES');
      expect(getSectorDisplayName('notes/ia/agentes')).toBe('IA');
    });
  });
});
