import { describe, it, expect } from 'vitest';
import { resolveProjectCategory, getSectorDisplayName, getCategoryDisplayName } from '../src/lib/nebulas';

describe('BreadcrumbResolution and GitLab Taxonomy', () => {
  it('resolves GitLab repositories under an organization without leaking org as sector', () => {
    const resDocs = resolveProjectCategory('sources/gitlab/acme/saas/billing/docs/readme.md');
    expect(resDocs.sector).toBe('saas');
    expect(resDocs.sectorDisplayName).toBe('SAAS');
    expect(resDocs.displayName).toBe('BILLING');
    expect(getSectorDisplayName('sources/gitlab/acme/saas/billing/docs/readme.md')).toBe('SAAS');
    expect(getCategoryDisplayName('sources/gitlab/acme/saas/billing/docs/readme.md')).toBe('BILLING');

    const resRepo = resolveProjectCategory('sources/gitlab/acme/saas/billing');
    expect(resRepo.sector).toBe('saas');
    expect(resRepo.sectorDisplayName).toBe('SAAS');
    expect(resRepo.displayName).toBe('BILLING');
    expect(getSectorDisplayName('sources/gitlab/acme/saas/billing')).toBe('SAAS');
    expect(getCategoryDisplayName('sources/gitlab/acme/saas/billing')).toBe('BILLING');

    const resApis = resolveProjectCategory('sources/gitlab/acme/apis/auth/specs/v1.md');
    expect(resApis.sector).toBe('apis');
    expect(resApis.sectorDisplayName).toBe('APIS');
    expect(resApis.displayName).toBe('AUTH');
    expect(getSectorDisplayName('sources/gitlab/acme/apis/auth')).toBe('APIS');
    expect(getCategoryDisplayName('sources/gitlab/acme/apis/auth')).toBe('AUTH');

    const resInfra = resolveProjectCategory('sources/gitlab/acme/infra/k8s');
    expect(resInfra.sector).toBe('infra');
    expect(resInfra.sectorDisplayName).toBe('INFRA');
    expect(resInfra.displayName).toBe('K8S');
  });

  it('resolves organizational group MOCs to their own functional sector', () => {
    const resSaas = resolveProjectCategory('sources/gitlab/acme/saas');
    expect(resSaas.sector).toBe('saas');
    expect(resSaas.sectorDisplayName).toBe('SAAS');
    expect(resSaas.displayName).toBe('SAAS');
    expect(getSectorDisplayName('sources/gitlab/acme/saas')).toBe('SAAS');
    expect(getCategoryDisplayName('sources/gitlab/acme/saas')).toBe('SAAS');

    const resApis = resolveProjectCategory('sources/gitlab/acme/apis');
    expect(resApis.sector).toBe('apis');
    expect(resApis.sectorDisplayName).toBe('APIS');
    expect(resApis.displayName).toBe('APIS');
    expect(getSectorDisplayName('sources/gitlab/acme/apis')).toBe('APIS');
    expect(getCategoryDisplayName('sources/gitlab/acme/apis')).toBe('APIS');
  });

  it('resolves arbitrary non-standard engineering groups without defaulting to the organization', () => {
    const resFiscal = resolveProjectCategory('sources/gitlab/acme/fiscal/dfe');
    expect(resFiscal.sector).toBe('fiscal');
    expect(resFiscal.sectorDisplayName).toBe('FISCAL');
    expect(resFiscal.displayName).toBe('DFE');
    expect(getSectorDisplayName('sources/gitlab/acme/fiscal/dfe')).toBe('FISCAL');
    expect(getCategoryDisplayName('sources/gitlab/acme/fiscal/dfe')).toBe('DFE');

    const resFiscalGroup = resolveProjectCategory('sources/gitlab/acme/fiscal');
    expect(resFiscalGroup.sector).toBe('fiscal');
    expect(resFiscalGroup.sectorDisplayName).toBe('FISCAL');
  });

  it('resolves org-prefixed groups passed from backend', () => {
    expect(getSectorDisplayName('acme/saas')).toBe('SAAS');
    expect(getCategoryDisplayName('acme/saas')).toBe('SAAS');

    expect(getSectorDisplayName('acme/apis')).toBe('APIS');
    expect(getCategoryDisplayName('acme/apis')).toBe('APIS');

    expect(getSectorDisplayName('acme/fiscal')).toBe('FISCAL');
    expect(getCategoryDisplayName('acme/fiscal')).toBe('FISCAL');

    expect(getSectorDisplayName('acme/saas/pocs')).toBe('SAAS');
    expect(getCategoryDisplayName('acme/saas/pocs')).toBe('POCS');
  });

  it('resolves sector and category anchor keys correctly', () => {
    expect(getSectorDisplayName('__SECTOR_saas')).toBe('SAAS');
    expect(getSectorDisplayName('__SECTOR_apis')).toBe('APIS');
    expect(getSectorDisplayName('__SECTOR_notes/arquitetura')).toBe('ARQUITETURA');

    expect(getCategoryDisplayName('__CAT_sources/gitlab/acme/saas/billing')).toBe('BILLING');
    expect(getCategoryDisplayName('__CAT_sources/gitlab/acme/apis/auth')).toBe('AUTH');
  });
});
