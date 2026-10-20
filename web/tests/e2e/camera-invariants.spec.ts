import type {} from './browser-types';
import { test, expect } from '@playwright/test';
import * as path from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

test.describe('Cosmic Camera Invariants & Systemic Verification', () => {
  const artifactDir = process.env.ARTIFACT_DIR || path.join(__dirname, '../../test-results');

  test('Enforces all 5 cosmic invariants across camera, shaders, tooltips, and photon defaults', async ({ page }) => {
    test.setTimeout(90000);
    page.on('console', msg => console.log('PAGE LOG:', msg.text()));

    // 1. Initial Load & Cosmic Photons Default Verification
    await page.goto('/');
    await page.waitForSelector('[data-testid="cosmic-loading-overlay"]', { state: 'attached', timeout: 5000 }).catch(() => {});
    await page.waitForSelector('[data-testid="cosmic-loading-overlay"]', { state: 'detached', timeout: 30000 });
    await page.waitForFunction(() => {
      const sigma = window.__sigmaInstance!;
      const ctx = window.__sigmaContext;
      return sigma && ctx?.frameCalloutBoxes && ctx.frameCalloutBoxes.length > 0;
    }, { timeout: 20000 });
    await page.waitForTimeout(1500);

    const initialSettings = await page.evaluate(() => {
      const viewButton = document.querySelector('[data-testid="toggle-controls-btn"]');
      const sigmaContext = window.__sigmaContext;

      return {
        isCosmicMode: Boolean(sigmaContext?.isCosmicMode),
        viewButtonFound: Boolean(viewButton),
      };
    });

    console.log('Invariant 1 - Initial State Check:', initialSettings);
    expect(initialSettings.isCosmicMode).toBe(true);
    expect(initialSettings.viewButtonFound).toBe(true);

    // 2. INVARIANT 2: Satellite Click & Label Click Centers Satellite in Open Viewport
    const satTarget = await page.evaluate(() => {
      const boxes = window.__sigmaContext?.frameCalloutBoxes || [];
      const satBox = boxes.find((b) => b.isStation && b.nodeId && b.minY > 90) ||
                     boxes.find((b) => b.isStation && b.nodeId);
      if (!satBox) return null;
      return {
        nodeId: satBox.nodeId,
        centerX: (satBox.minX + satBox.maxX) / 2,
        centerY: (satBox.minY + satBox.maxY) / 2,
      };
    });

    expect(satTarget).not.toBeNull();
    console.log('Testing Satellite Click on:', satTarget!.nodeId);

    // Click physically on satellite label callout
    await page.mouse.click(satTarget!.centerX, satTarget!.centerY);
    await page.waitForTimeout(1800);

    const satAfter = await page.evaluate((nodeId: string) => {
      const sigma = window.__sigmaInstance!;
      const graph = sigma.getGraph();
      const dims = sigma.getDimensions();
      const attrs = graph.getNodeAttributes(nodeId);
      const vp = sigma.graphToViewport(attrs);
      const expectedOpenCenterX = dims.width >= 900 ? (dims.width - 440) / 2 : dims.width / 2;

      return {
        nodeId,
        vp,
        camState: sigma.getCamera().getState(),
        expectedOpenCenterX,
        diffX: Math.abs(vp.x - expectedOpenCenterX),
        diffY: Math.abs(vp.y - dims.height / 2),
        ratio: sigma.getCamera().ratio,
        isInsideViewport: vp.x >= 0 && vp.x <= dims.width && vp.y >= 0 && vp.y <= dims.height,
      };
    }, satTarget!.nodeId!);

    console.log('Invariant 2 - Satellite Positioning Result:', satAfter);
    expect(satAfter.isInsideViewport).toBe(true);
    // Satellite must be centered within 25px of the open viewport center
    expect(satAfter.diffX).toBeLessThanOrEqual(25);
    expect(satAfter.diffY).toBeLessThanOrEqual(25);

    // Save screenshot of centered satellite
    await page.screenshot({
      path: path.join(artifactDir, 'satellite_centered_focus.png'),
    });

    // 3. INVARIANT 3: Category Nebula Cluster Fits Full Screen (65% - 90% of Viewport)
    // Close any open drawer and reset camera to return to full galaxy overview
    await page.keyboard.press('Escape');
    await page.waitForTimeout(400);
    const resetBtn = page.getByTestId('reset-camera-btn');
    if (await resetBtn.isVisible()) {
      await resetBtn.click();
      await page.waitForTimeout(1600);
    }
    await page.waitForFunction(() => {
      const boxes = window.__sigmaContext?.frameCalloutBoxes || [];
      return boxes.some((b) => b.isCategory);
    }, { timeout: 15000 });
    await page.waitForTimeout(600);

    const nebulaTarget = await page.evaluate(() => {
      const boxes = window.__sigmaContext?.frameCalloutBoxes || [];
      const arq = boxes.find((b) => b.isCategory && (b.category?.includes('architecture') || b.category?.includes('arquitetura')));
      const fallback = boxes.find((b) => b.isCategory);
      const chosen = arq || fallback;
      return chosen ? {
        category: chosen.category,
        nodeId: chosen.nodeId,
        isSector: chosen.isSector,
        centerX: (chosen.minX + chosen.maxX) / 2,
        centerY: (chosen.minY + chosen.maxY) / 2,
      } : null;
    });

    expect(nebulaTarget).not.toBeNull();
    console.log('Testing Nebula Click on:', nebulaTarget);

    await page.mouse.click(nebulaTarget!.centerX, nebulaTarget!.centerY);
    await page.waitForTimeout(1800);

    const clusterMetrics = await page.evaluate((catId: string) => {
      const sigma = window.__sigmaInstance!;
      const graph = sigma.getGraph();
      const dims = sigma.getDimensions();
      const cleanCat = catId.replace('__CAT_', '').toLowerCase();

      let minX = Infinity, maxX = -Infinity, minY = Infinity, maxY = -Infinity;
      let count = 0;
      graph.forEachNode((node: string, attrs) => {
        if (node.startsWith('__CAT_')) return;
        const noteCat = ((attrs.originalCategory || attrs.category || '') as string).toLowerCase();
        if (noteCat === cleanCat || noteCat.startsWith(cleanCat + '/')) {
          const v = sigma.graphToViewport(attrs);
          minX = Math.min(minX, v.x);
          maxX = Math.max(maxX, v.x);
          minY = Math.min(minY, v.y);
          maxY = Math.max(maxY, v.y);
          count++;
        }
      });

      const widthSpan = maxX - minX;
      const heightSpan = maxY - minY;

      return {
        cat: cleanCat,
        count,
        ratio: sigma.getCamera().ratio,
        widthSpan,
        heightSpan,
        widthPct: (widthSpan / dims.width) * 100,
        heightPct: (heightSpan / dims.height) * 100,
        maxSpanPct: Math.max((widthSpan / dims.width) * 100, (heightSpan / dims.height) * 100),
      };
    }, (nebulaTarget!.nodeId || nebulaTarget!.category)!);

    console.log('Invariant 3 - Cluster Framing Result:', clusterMetrics);
    // Aglomerado ocupa harmoniosamente entre 65% e 90% da tela ("full screen")
    expect(clusterMetrics.maxSpanPct).toBeGreaterThanOrEqual(65);
    expect(clusterMetrics.maxSpanPct).toBeLessThanOrEqual(90);

    // Save screenshot of full-screen nebula
    await page.screenshot({
      path: path.join(artifactDir, 'nebula_cluster_full_screen_harmonious.png'),
    });

    // 4. INVARIANT 4: Tooltips on Edge Filtering Controls
    // Open Cosmic Controls popover
    await page.getByTestId('toggle-controls-btn').click();
    await page.waitForSelector('[data-testid="controls-popover"]', { state: 'visible', timeout: 5000 });
    await page.waitForTimeout(300);

    const tooltips = await page.evaluate(() => {
      const modes = ['auto', 'bridges', 'focus', 'all'];
      const titles: Record<string, string | null> = {};
      modes.forEach(m => {
        const btn = document.querySelector(`[data-testid="edge-mode-${m}"]`);
        titles[m] = btn ? btn.getAttribute('title') : null;
      });
      return titles;
    });

    console.log('Invariant 4 - Edge Tooltips:', tooltips);
    expect(tooltips.auto).toBe('Automatically adapts edge density to zoom level');
    expect(tooltips.bridges).toBe('Shows connections between clusters and sectors');
    expect(tooltips.focus).toBe('Shows connections on hover or selection');
    expect(tooltips.all).toBe('Shows every visible graph connection');

    // Close controls popover
    await page.keyboard.press('Escape');
    await page.waitForTimeout(200);

    // 5. INVARIANT 5: Unified View controls
    await page.getByTestId('toggle-controls-btn').click();
    await page.waitForSelector('[data-testid="controls-popover"]', { state: 'visible', timeout: 5000 });

    await expect(page.getByRole('button', { name: 'Explore view' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Read view' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Present view' })).toBeVisible();
    await expect(page.getByTestId('toggle-cloak-sectors')).toBeVisible();
    await expect(page.getByTestId('toggle-cloak-nebulas')).toBeVisible();
    await expect(page.getByTestId('toggle-cloak-photons')).toBeVisible();
    await expect(page.getByTestId('toggle-cloak-perimeter')).toBeVisible();

    // Verify hotbar has no hardcoded weekly button
    const weekBtn = page.locator('[data-testid="hud-btn-week"]');
    await expect(weekBtn).toHaveCount(0);

    // Save screenshot of View controls
    await page.screenshot({
      path: path.join(artifactDir, 'view_controls_panel.png'),
    });
  });
});
