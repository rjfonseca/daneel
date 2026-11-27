import type {} from './browser-types';
import { test, expect } from '@playwright/test';

test.describe('Semantic Zoom Tiers, Progressive Disclosure & Knowledge Scope', () => {
  test('collapses external sources into Super-Nodes at overview, unfolds on zoom, re-collapses on zoom-out, and switches scope modes', async ({ page }) => {
    test.setTimeout(90000);

    await page.goto('/');
    await page.waitForSelector('[data-testid="cosmic-loading-overlay"]', { state: 'attached', timeout: 5000 }).catch(() => {});
    await page.waitForSelector('[data-testid="cosmic-loading-overlay"]', { state: 'detached', timeout: 30000 });
    await page.waitForFunction(() => Boolean(window.__sigmaInstance && window.__sigmaContext?.lodState), { timeout: 20000 });

    // 1. Initial Hybrid Overview: sources/github/acme/* collapsed into __SECTOR_acme while curated notes stay visible
    const overviewState = await page.evaluate(() => {
      const ctx = window.__sigmaContext!;
      const sigma = window.__sigmaInstance!;
      const lod = ctx.lodState!;
      const curatedDisp = sigma.getNodeDisplayData('notes/architecture/note-0.md');
      const sourceDisp = sigma.getNodeDisplayData('sources/github/acme/orion/note-0.md');
      const sectorDisp = sigma.getNodeDisplayData('__SECTOR_acme');
      return {
        scopeMode: ctx.scopeMode,
        tier: lod.tier,
        collapsedSectors: Array.from(lod.collapsedSectorNodes),
        visibleStarCount: lod.visibleStarCount,
        totalStarCount: lod.totalStarCount,
        curatedHidden: curatedDisp?.hidden ?? false,
        sourceHidden: sourceDisp?.hidden ?? false,
        sectorHidden: sectorDisp?.hidden ?? false,
      };
    });

    expect(overviewState.scopeMode).toBe('hybrid');
    expect(overviewState.tier).toBe('overview');
    expect(overviewState.collapsedSectors).toContain('__SECTOR_acme');
    expect(overviewState.visibleStarCount).toBeLessThan(overviewState.totalStarCount);
    expect(overviewState.curatedHidden).toBe(false);
    expect(overviewState.sourceHidden).toBe(true);
    expect(overviewState.sectorHidden).toBe(false);

    // 2. Zoom into __SECTOR_acme (Tier 1: ratio = 0.35) -> unfolds sector into Cluster Super-Nodes
    await page.evaluate(() => {
      const sigma = window.__sigmaInstance!;
      const graph = sigma.getGraph();
      const attrs = graph.getNodeAttributes('__SECTOR_acme');
      const framed = sigma.normalizationFunction!(attrs);
      sigma.getCamera().animate({ x: framed.x, y: framed.y, ratio: 0.35, angle: 0 }, { duration: 1 });
    });

    await page.waitForFunction(() => window.__sigmaContext?.lodState?.tier === 'sector', { timeout: 10000 });
    const tier1State = await page.evaluate(() => {
      const lod = window.__sigmaContext!.lodState!;
      return {
        tier: lod.tier,
        expandedSectors: Array.from(lod.expandedSectors),
        collapsedClusters: Array.from(lod.collapsedClusterNodes),
      };
    });
    expect(tier1State.tier).toBe('sector');
    expect(tier1State.expandedSectors).toContain('acme');
    expect(tier1State.collapsedClusters).toContain('__CAT_sources/github/acme/orion');

    // 3. Zoom deeply into sources/github/acme/orion (Tier 2: ratio = 0.14) -> unfolds individual stars
    await page.evaluate(() => {
      const sigma = window.__sigmaInstance!;
      const graph = sigma.getGraph();
      const attrs = graph.getNodeAttributes('__CAT_sources/github/acme/orion');
      const framed = sigma.normalizationFunction!(attrs);
      sigma.getCamera().animate({ x: framed.x, y: framed.y, ratio: 0.14, angle: 0 }, { duration: 1 });
    });

    await page.waitForFunction(() => window.__sigmaContext?.lodState?.tier === 'cluster', { timeout: 10000 });
    const tier2State = await page.evaluate(() => {
      const sigma = window.__sigmaInstance!;
      const lod = window.__sigmaContext!.lodState!;
      const sourceDisp = sigma.getNodeDisplayData('sources/github/acme/orion/note-0.md');
      return {
        tier: lod.tier,
        expandedClusters: Array.from(lod.expandedClusters),
        sourceHidden: sourceDisp?.hidden ?? false,
      };
    });
    expect(tier2State.tier).toBe('cluster');
    expect(tier2State.expandedClusters).toContain('sources/github/acme/orion');
    expect(tier2State.sourceHidden).toBe(false);

    // 4. Zoom back out to ratio = 1.0 -> re-collapses acme into Sector Super-Node
    await page.evaluate(() => {
      const sigma = window.__sigmaInstance!;
      sigma.getCamera().animate({ x: 0.5, y: 0.5, ratio: 1.0, angle: 0 }, { duration: 1 });
    });

    await page.waitForFunction(() => window.__sigmaContext?.lodState?.tier === 'overview', { timeout: 10000 });
    const recollapsedState = await page.evaluate(() => {
      const sigma = window.__sigmaInstance!;
      const lod = window.__sigmaContext!.lodState!;
      const sourceDisp = sigma.getNodeDisplayData('sources/github/acme/orion/note-0.md');
      return {
        collapsedSectors: Array.from(lod.collapsedSectorNodes),
        sourceHidden: sourceDisp?.hidden ?? false,
      };
    });
    expect(recollapsedState.collapsedSectors).toContain('__SECTOR_acme');
    expect(recollapsedState.sourceHidden).toBe(true);

    // 5. Switch Knowledge scope via ViewControlsPanel (Curated -> All -> Hybrid)
    await page.getByTestId('toggle-controls-btn').click();
    await page.waitForSelector('[data-testid="controls-popover"]', { state: 'visible', timeout: 5000 });
    await expect(page.getByTestId('lod-telemetry')).toBeVisible();

    await page.getByTestId('scope-mode-curated').click();
    await page.waitForFunction(() => window.__sigmaContext?.scopeMode === 'curated', { timeout: 5000 });
    const curatedOnlyHidden = await page.evaluate(() => window.__sigmaInstance!.getNodeDisplayData('__SECTOR_acme')?.hidden ?? false);
    expect(curatedOnlyHidden).toBe(true);

    await page.getByTestId('scope-mode-all').click();
    await page.waitForFunction(() => window.__sigmaContext?.scopeMode === 'all', { timeout: 5000 });
    const allVisibleSource = await page.evaluate(() => window.__sigmaInstance!.getNodeDisplayData('sources/github/acme/orion/note-0.md')?.hidden ?? false);
    expect(allVisibleSource).toBe(false);

    await page.getByTestId('scope-mode-hybrid').click();
    await page.waitForFunction(() => window.__sigmaContext?.scopeMode === 'hybrid', { timeout: 5000 });
  });
});
