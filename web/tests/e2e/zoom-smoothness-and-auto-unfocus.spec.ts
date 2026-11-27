import type {} from './browser-types';
import { test, expect } from '@playwright/test';
import * as path from 'path';

test.describe('Crisp Stellar Zoom, Smooth Wheel & 2-Step Hierarchical Auto-Unfocus', () => {
  const artifactDir = process.env.ARTIFACT_DIR || path.join(process.cwd(), 'test-results');

  test('renders crisp stars on deep zoom without WebGL errors and keeps move flicker disabled', async ({ page }) => {
    test.setTimeout(90000);
    const consoleErrors: string[] = [];
    page.on('console', msg => {
      if (msg.type() === 'error') consoleErrors.push(msg.text());
    });
    page.on('pageerror', err => consoleErrors.push(err.message));

    await page.goto('/');
    await page.waitForSelector('[data-testid="cosmic-loading-overlay"]', { state: 'detached', timeout: 30000 }).catch(() => {});
    await page.waitForFunction(() => Boolean(window.__sigmaInstance && window.__sigmaContext?.lodState), { timeout: 20000 });

    const settingsCheck = await page.evaluate(() => {
      const sigma = window.__sigmaInstance!;
      const settings = sigma.getSettings();
      const targetNode = 'notes/architecture/note-0.md';
      const attrs = sigma.getGraph().getNodeAttributes(targetNode);
      const framed = sigma.normalizationFunction!(attrs);
      sigma.getCamera().setState({ x: framed.x, y: framed.y, ratio: 0.03, angle: 0 });
      return {
        hideEdgesOnMove: settings.hideEdgesOnMove,
        hideLabelsOnMove: settings.hideLabelsOnMove,
        ratio: sigma.getCamera().ratio,
      };
    });

    await page.waitForTimeout(300);
    expect(settingsCheck.hideEdgesOnMove).toBe(false);
    expect(settingsCheck.hideLabelsOnMove).toBe(false);
    expect(settingsCheck.ratio).toBeLessThanOrEqual(0.04);
    expect(consoleErrors).toEqual([]);

    await page.screenshot({
      path: path.join(artifactDir, 'stellar_deep_zoom_crisp.png'),
    });
  });

  test('accumulates smooth cursor-anchored wheel zoom without synchronous history.replaceState flood', async ({ page }) => {
    test.setTimeout(90000);

    await page.goto('/');
    await page.waitForSelector('[data-testid="cosmic-loading-overlay"]', { state: 'detached', timeout: 30000 }).catch(() => {});
    await page.waitForFunction(() => Boolean(window.__sigmaInstance && window.__sigmaContext?.lodState), { timeout: 20000 });
    await page.waitForTimeout(1600);

    await page.evaluate(() => {
      const win = window as unknown as { __replaceStateCount?: number; __origReplaceState?: typeof history.replaceState };
      win.__replaceStateCount = 0;
      const orig = history.replaceState.bind(history);
      win.__origReplaceState = orig;
      history.replaceState = function (...args: Parameters<typeof history.replaceState>) {
        win.__replaceStateCount = (win.__replaceStateCount || 0) + 1;
        return orig(...args);
      };
    });

    const box = await page.locator('canvas').first().boundingBox();
    expect(box).not.toBeNull();
    const cx = box!.x + box!.width * 0.55;
    const cy = box!.y + box!.height * 0.5;
    await page.mouse.move(cx, cy);

    const startRatio = await page.evaluate(() => window.__sigmaInstance!.getCamera().ratio);
    for (let i = 0; i < 6; i++) {
      await page.mouse.wheel(0, -140);
      await page.waitForTimeout(25);
    }

    await page.waitForTimeout(450);
    const midRatio = await page.evaluate(() => window.__sigmaInstance!.getCamera().ratio);
    expect(midRatio).toBeLessThan(startRatio * 0.75);

    const replaceCountDuringMotion = await page.evaluate(() => {
      const win = window as unknown as { __replaceStateCount?: number };
      return win.__replaceStateCount || 0;
    });
    // Debounced to <= 4 replaceState calls rather than ~40 synchronous 60Hz calls
    expect(replaceCountDuringMotion).toBeLessThanOrEqual(4);
  });

  test('auto-closes focused sector in place on zoom-out and performs 2-step cluster -> sector -> galaxy exit', async ({ page }) => {
    test.setTimeout(90000);

    await page.goto('/');
    await page.waitForSelector('[data-testid="cosmic-loading-overlay"]', { state: 'detached', timeout: 30000 }).catch(() => {});
    await page.waitForFunction(() => Boolean(window.__sigmaInstance && window.__sigmaContext?.lodState), { timeout: 20000 });
    await page.waitForTimeout(1600);

    // 1. Focus sector __SECTOR_acme via clickNode
    await page.evaluate(() => {
      window.__sigmaInstance!.emit('clickNode', { node: '__SECTOR_acme' });
    });

    const returnBtn = page.getByTestId('return-to-galaxy-button');
    await expect(returnBtn).toBeVisible({ timeout: 5000 });
    await page.waitForTimeout(1100);

    const focusedState = await page.evaluate(() => ({
      focusedSector: window.__sigmaContext?.focusedSector,
      ratio: window.__sigmaInstance!.getCamera().ratio,
      x: window.__sigmaInstance!.getCamera().x,
      y: window.__sigmaInstance!.getCamera().y,
    }));
    expect(focusedState.focusedSector).toBe('__SECTOR_acme');
    expect(focusedState.ratio).toBeLessThan(0.56);

    // 2. Scroll wheel out past exit threshold -> sector auto-closes in place
    const box = await page.locator('canvas').first().boundingBox();
    expect(box).not.toBeNull();
    await page.mouse.move(box!.x + box!.width / 2, box!.y + box!.height / 2);

    for (let i = 0; i < 12; i++) {
      const stillFocused = await page.evaluate(() => Boolean(window.__sigmaContext?.focusedSector));
      if (!stillFocused) break;
      await page.mouse.wheel(0, 180);
      await page.waitForTimeout(120);
    }

    await expect(returnBtn).toBeHidden({ timeout: 5000 });
    const afterSectorExit = await page.evaluate(() => {
      const sigma = window.__sigmaInstance!;
      let visibleCount = 0;
      sigma.getGraph().forEachNode((_id, attrs) => {
        if (!attrs.hidden) visibleCount++;
      });
      return {
        focusedSector: window.__sigmaContext?.focusedSector,
        focusedCluster: window.__sigmaContext?.focusedCluster,
        ratio: sigma.getCamera().ratio,
        visibleCount,
      };
    });
    expect(afterSectorExit.focusedSector).toBeNull();
    expect(afterSectorExit.focusedCluster).toBeNull();
    expect(afterSectorExit.visibleCount).toBeGreaterThan(30);
    // Must stay near the zoom-out altitude rather than yanking back to 1.0 unless user scrolled all the way
    expect(afterSectorExit.ratio).toBeGreaterThan(0.18);

    // 3. 2-step hierarchical exit: Cluster -> Sector -> Full Galaxy
    await page.evaluate(() => {
      window.__sigmaInstance!.emit('clickNode', { node: '__CAT_sources/github/acme/orion' });
    });
    await expect(returnBtn).toBeVisible({ timeout: 5000 });
    await page.waitForTimeout(1100);

    const clusterFocused = await page.evaluate(() => ({
      focusedCluster: window.__sigmaContext?.focusedCluster,
      focusedSector: window.__sigmaContext?.focusedSector,
      ratio: window.__sigmaInstance!.getCamera().ratio,
    }));
    expect(clusterFocused.focusedCluster).toBe('__CAT_sources/github/acme/orion');
    expect(clusterFocused.focusedSector).toBe('__SECTOR_acme');

    // Step 1: zoom out gently until cluster exits while sector remains active
    for (let i = 0; i < 24; i++) {
      const state = await page.evaluate(() => ({
        cluster: window.__sigmaContext?.focusedCluster,
        sector: window.__sigmaContext?.focusedSector,
      }));
      if (!state.cluster) break;
      await page.mouse.wheel(0, 140);
      await page.waitForTimeout(160);
    }

    const midStep = await page.evaluate(() => ({
      focusedCluster: window.__sigmaContext?.focusedCluster,
      focusedSector: window.__sigmaContext?.focusedSector,
    }));
    expect(midStep.focusedCluster).toBeNull();
    expect(midStep.focusedSector).toBe('__SECTOR_acme');
    await expect(returnBtn).toBeVisible();

    // Step 2: continue zooming out until sector also closes
    for (let i = 0; i < 18; i++) {
      const sector = await page.evaluate(() => window.__sigmaContext?.focusedSector);
      if (!sector) break;
      await page.mouse.wheel(0, 180);
      await page.waitForTimeout(140);
    }

    await expect(returnBtn).toBeHidden({ timeout: 5000 });
    const finalState = await page.evaluate(() => ({
      focusedCluster: window.__sigmaContext?.focusedCluster,
      focusedSector: window.__sigmaContext?.focusedSector,
    }));
    expect(finalState.focusedCluster).toBeNull();
    expect(finalState.focusedSector).toBeNull();
  });

  test('softly dims non-constellation stars, exits constellation on zoom-out, clears hover tooltips, and keeps HUD unobstructed', async ({ page }) => {
    test.setTimeout(90000);

    await page.goto('/');
    await page.waitForSelector('[data-testid="cosmic-loading-overlay"]', { state: 'detached', timeout: 30000 }).catch(() => {});
    await page.waitForFunction(() => Boolean(window.__sigmaInstance && window.__sigmaContext?.lodState), { timeout: 20000 });
    await page.waitForTimeout(1600);

    // 1. Overview ARMS station callouts stay below TopCosmicBar and sanitize redundant prefixes
    const headerBox = await page.getByTestId('command-bridge').boundingBox();
    expect(headerBox).not.toBeNull();
    const headerBottom = headerBox!.y + headerBox!.height;

    const overviewCheck = await page.evaluate((minAllowedY: number) => {
      const boxes = window.__sigmaContext?.frameCalloutBoxes || [];
      const stationBoxes = boxes.filter(b => b.isStation);
      const clippedTop = stationBoxes.filter(b => b.minY < minAllowedY - 1).length;
      return { totalStations: stationBoxes.length, clippedTop };
    }, headerBottom);
    expect(overviewCheck.totalStations).toBeGreaterThan(0);
    expect(overviewCheck.clippedTop).toBe(0);

    // 2. Hovering a node then clicking a sector clears tooltip & places Return button strictly below TopCosmicBar
    await page.evaluate(() => {
      window.__sigmaInstance!.emit('enterNode', { node: '__SECTOR_acme' });
    });
    await expect(page.getByTestId('node-hover-tooltip')).toBeVisible({ timeout: 3000 });

    await page.evaluate(() => {
      window.__sigmaInstance!.emit('clickNode', { node: '__SECTOR_acme' });
    });
    await expect(page.getByTestId('node-hover-tooltip')).toBeHidden({ timeout: 3000 });

    const returnBtn = page.getByTestId('return-to-galaxy-button');
    await expect(returnBtn).toBeVisible({ timeout: 5000 });
    const returnBtnBox = await returnBtn.boundingBox();
    expect(returnBtnBox).not.toBeNull();
    expect(returnBtnBox!.y).toBeGreaterThanOrEqual(headerBottom + 12);
    await returnBtn.click();
    await expect(returnBtn).toBeHidden({ timeout: 5000 });
    await page.waitForTimeout(1100);

    // 3. Click a note -> soft cosmic dimming keeps background stars visible & zoom-out releases constellation while reader stays open
    await page.evaluate(() => {
      window.__sigmaInstance!.emit('clickNode', { node: 'notes/architecture/note-0.md' });
    });
    await expect(page.getByTestId('floating-note-hud')).toBeVisible({ timeout: 5000 });
    await expect(returnBtn).toBeVisible({ timeout: 5000 });
    await page.waitForTimeout(1100);

    const dimmingState = await page.evaluate(() => {
      const sigma = window.__sigmaInstance!;
      const otherDisp = sigma.getNodeDisplayData('daily/2026-10-16.md');
      return {
        hasConstellation: Boolean(window.__sigmaContext?.constellation),
        otherHidden: otherDisp?.hidden,
        otherColor: otherDisp?.color,
      };
    });
    expect(dimmingState.hasConstellation).toBe(true);
    expect(dimmingState.otherHidden).toBe(false);
    expect(String(dimmingState.otherColor)).toContain('0.18');

    const canvasBox = await page.locator('canvas').first().boundingBox();
    expect(canvasBox).not.toBeNull();
    await page.mouse.move(canvasBox!.x + canvasBox!.width * 0.35, canvasBox!.y + canvasBox!.height * 0.5);

    for (let i = 0; i < 16; i++) {
      const active = await page.evaluate(() => Boolean(window.__sigmaContext?.constellation));
      if (!active) break;
      await page.mouse.wheel(0, 180);
      await page.waitForTimeout(120);
    }

    const afterZoomOut = await page.evaluate(() => ({
      hasConstellation: Boolean(window.__sigmaContext?.constellation),
    }));
    expect(afterZoomOut.hasConstellation).toBe(false);
    // Reading drawer remains open while galaxy returns in background
    await expect(page.getByTestId('floating-note-hud')).toBeVisible();
  });

  test('keeps unfolded sector expanded when zooming off-center toward its outer edge (no whiplash snap-back)', async ({ page }) => {
    test.setTimeout(90000);

    await page.goto('/');
    await page.waitForSelector('[data-testid="cosmic-loading-overlay"]', { state: 'detached', timeout: 30000 }).catch(() => {});
    await page.waitForFunction(() => Boolean(window.__sigmaInstance && window.__sigmaContext?.lodState), { timeout: 20000 });
    await page.waitForTimeout(1600);

    // Position pointer over a peripheral cluster inside __SECTOR_acme rather than exact sector centroid
    const starViewport = await page.evaluate(() => {
      const sigma = window.__sigmaInstance!;
      const attrs = sigma.getGraph().getNodeAttributes('sources/github/acme/orion/note-0.md');
      return sigma.graphToViewport(attrs);
    });
    await page.mouse.move(starViewport.x, starViewport.y);

    // Zoom deeply past sector & cluster unfold thresholds
    for (let i = 0; i < 18; i++) {
      await page.mouse.wheel(0, -160);
      await page.waitForTimeout(80);
    }
    await page.waitForTimeout(400);

    const lodAfterDeepZoom = await page.evaluate(() => {
      const lod = window.__sigmaContext!.lodState!;
      const ratio = window.__sigmaInstance!.getCamera().ratio;
      return {
        ratio,
        tier: lod.tier,
        hasAcmeExpanded: lod.expandedSectors.has('acme'),
        hasAcmeCollapsed: lod.collapsedSectorNodes.has('__SECTOR_acme'),
      };
    });

    expect(lodAfterDeepZoom.ratio).toBeLessThan(0.25);
    expect(lodAfterDeepZoom.hasAcmeExpanded).toBe(true);
    expect(lodAfterDeepZoom.hasAcmeCollapsed).toBe(false);
  });

  test('zooms monotonically without Sigma camera.animate tug-of-war or slingshot reversals', async ({ page }) => {
    test.setTimeout(90000);

    await page.goto('/');
    await page.waitForSelector('[data-testid="cosmic-loading-overlay"]', { state: 'detached', timeout: 30000 }).catch(() => {});
    await page.waitForFunction(() => Boolean(window.__sigmaInstance && window.__sigmaContext?.lodState), { timeout: 20000 });
    await page.waitForTimeout(1600);

    await page.evaluate(() => {
      const sigma = window.__sigmaInstance!;
      const camera = sigma.getCamera();
      const win = window as unknown as {
        __slingshotTrace?: {
          ratios: number[];
          animateCalls: number;
          cleanup: () => void;
        };
      };
      const ratios: number[] = [camera.ratio];
      let animateCalls = 0;

      const origAnimate = camera.animate.bind(camera);
      camera.animate = function (...args: Parameters<typeof camera.animate>) {
        animateCalls += 1;
        return origAnimate(...args);
      };

      const onUpdate = () => {
        ratios.push(camera.ratio);
      };
      camera.on('updated', onUpdate);

      win.__slingshotTrace = {
        get ratios() {
          return ratios;
        },
        get animateCalls() {
          return animateCalls;
        },
        cleanup: () => {
          camera.off('updated', onUpdate);
          camera.animate = origAnimate;
        },
      };
    });

    const box = await page.locator('canvas').first().boundingBox();
    expect(box).not.toBeNull();
    await page.mouse.move(box!.x + box!.width * 0.55, box!.y + box!.height * 0.5);

    for (let i = 0; i < 8; i++) {
      await page.mouse.wheel(0, -120);
      await page.waitForTimeout(40);
    }
    await page.waitForTimeout(700);

    const trace = await page.evaluate(() => {
      const win = window as unknown as {
        __slingshotTrace?: {
          ratios: number[];
          animateCalls: number;
          cleanup: () => void;
        };
      };
      const t = win.__slingshotTrace!;
      const result = {
        ratios: [...t.ratios],
        animateCalls: t.animateCalls,
      };
      t.cleanup();
      delete win.__slingshotTrace;
      return result;
    });

    let reversals = 0;
    for (let i = 1; i < trace.ratios.length; i++) {
      if (trace.ratios[i] > trace.ratios[i - 1] + 1e-6) {
        reversals += 1;
      }
    }

    expect(trace.animateCalls).toBe(0);
    expect(reversals).toBe(0);
    expect(trace.ratios.length).toBeGreaterThan(3);
    expect(trace.ratios[trace.ratios.length - 1]).toBeLessThan(trace.ratios[0] * 0.6);
  });
});
