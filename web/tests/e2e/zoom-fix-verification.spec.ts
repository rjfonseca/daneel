import type {} from './browser-types';
import { test, expect } from '@playwright/test';
import * as path from 'path';

test.use({ reducedMotion: 'reduce' });

test.describe('Cosmic Zoom Fix Verification', () => {
  const artifactDir = process.env.ARTIFACT_DIR || path.join(process.cwd(), 'test-results');

  test('Applies full proportional zoom when clicking nebula label and constellation note', async ({ page }) => {
    test.setTimeout(90000);

    // 1. Carrega aplicação e aguarda estabilização
    await page.goto('/');
    await page.waitForSelector('[data-testid="cosmic-loading-overlay"]', { state: 'detached', timeout: 30000 }).catch(() => {});
    await page.waitForFunction(() => {
      const sigma = window.__sigmaInstance!;
      const ctx = window.__sigmaContext;
      return sigma && ctx?.frameCalloutBoxes && ctx.frameCalloutBoxes.length > 0;
    }, { timeout: 20000 });
    await page.waitForTimeout(1000);

    // 2. PARTE 1: Clique Físico na Label de uma Nebulosa
    const targetBox = await page.evaluate(() => {
      const ctx = window.__sigmaContext;
      const boxes = ctx?.frameCalloutBoxes || [];
      const arq = boxes.find((b) => b.isCategory && b.category?.includes('architecture'));
      const fallback = boxes.find((b) => b.isCategory);
      const chosen = arq || fallback;
      return chosen ? {
        category: chosen.category,
        nodeId: chosen.nodeId,
        centerX: (chosen.minX + chosen.maxX) / 2,
        centerY: (chosen.minY + chosen.maxY) / 2,
      } : null;
    });

    expect(targetBox).not.toBeNull();
    console.log('Target Nebula Callout Box:', targetBox);

    const initialRatio = await page.evaluate(() => window.__sigmaInstance!.getCamera().ratio);
    expect(initialRatio).toBe(1);

    // Clica fisicamente no centro da caixa do rótulo da nebulosa
    await page.mouse.click(targetBox!.centerX, targetBox!.centerY);
    await page.waitForTimeout(1800);

    // Mede métricas da nebulosa focada
    const nebulaMetrics = await page.evaluate((catId: string) => {
      const sigma = window.__sigmaInstance!;
      const graph = sigma.getGraph();
      const camera = sigma.getCamera();
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
        ratio: camera.ratio,
        focusedSector: window.__sigmaContext?.focusedSector,
        count,
        widthSpan,
        heightSpan,
        widthPct: (widthSpan / dims.width) * 100,
        heightPct: (heightSpan / dims.height) * 100,
      };
    }, (targetBox!.nodeId || targetBox!.category)!);

    console.log('Nebula Metrics after physical click:', nebulaMetrics);
    expect(nebulaMetrics.ratio).toBeLessThanOrEqual(0.25);
    expect(nebulaMetrics.ratio).toBeGreaterThanOrEqual(0.020);
    expect(nebulaMetrics.focusedSector).toBeTruthy();
    // Aglomerado ocupa espaço nobre da tela (mais de 60% de largura ou altura)
    expect(Math.max(nebulaMetrics.widthPct, nebulaMetrics.heightPct)).toBeGreaterThanOrEqual(60);

    // Screenshot da nebulosa com enquadramento de tela inteira
    await page.screenshot({
      path: path.join(artifactDir, 'nebula_label_click_zoomed_full.png'),
    });

    // 3. PARTE 2: Clique em Nota da Constelação
    // Clica no botão de retornar para a galáxia
    const returnBtn = page.getByRole('button', { name: /Return to Full Galaxy/i });
    await expect(returnBtn).toBeVisible();
    await returnBtn.click();
    await page.waitForTimeout(1500);

    // Use a connected fixture note so a synthetic sector cannot satisfy this check.
    const testNoteId = 'notes/architecture/note-0.md';
    console.log('Testing Note Constellation Zoom on:', testNoteId);

    // Clica na nota para abrir constelação e drawer
    await page.evaluate((nodeId: string) => {
      window.__sigmaInstance!.emit('clickNode', { node: nodeId });
    }, testNoteId);
    await expect(page.getByTestId('floating-note-hud')).toContainText('architecture example 0');
    await expect(page).toHaveURL(/#note=notes%2Farchitecture%2Fnote-0\.md/);
    await page.waitForTimeout(2000);

    // Mede métricas da constelação focada na tela
    const noteMetrics = await page.evaluate((nodeId: string) => {
      const sigma = window.__sigmaInstance!;
      const graph = sigma.getGraph();
      const camera = sigma.getCamera();
      const dims = sigma.getDimensions();
      const constel = window.__sigmaContext?.constellation;

      const visibleList: string[] = constel && constel.visibleNodes
        ? Array.from(constel.visibleNodes)
        : [nodeId];

      let minX = Infinity, maxX = -Infinity, minY = Infinity, maxY = -Infinity;
      visibleList.forEach((id: string) => {
        if (graph.hasNode(id)) {
          const v = sigma.graphToViewport(graph.getNodeAttributes(id));
          minX = Math.min(minX, v.x);
          maxX = Math.max(maxX, v.x);
          minY = Math.min(minY, v.y);
          maxY = Math.max(maxY, v.y);
        }
      });

      const widthSpan = maxX - minX;
      const heightSpan = maxY - minY;
      const availableWidth = dims.width >= 900 ? (dims.width - 440) : dims.width;
      const selectedPosition = sigma.graphToViewport(graph.getNodeAttributes(nodeId));

      return {
        ratio: camera.ratio,
        constelSize: visibleList.length,
        widthSpan,
        heightSpan,
        availableWidth,
        spanPctOfAvailableWidth: (widthSpan / availableWidth) * 100,
        selectedX: selectedPosition.x,
        selectedY: selectedPosition.y,
        expectedCenterX: availableWidth / 2,
        expectedCenterY: dims.height / 2,
      };
    }, testNoteId);

    console.log('Note Constellation Metrics after click:', noteMetrics);
    // Node framing adapts to the local neighborhood within [0.04, 0.14].
    // Distant connected nodes may extend outside the viewport; the selected
    // note must remain anchored at the visible center beside the drawer.
    expect(noteMetrics.ratio).toBeGreaterThanOrEqual(0.04);
    expect(noteMetrics.ratio).toBeLessThanOrEqual(0.14);
    expect(noteMetrics.constelSize).toBeGreaterThan(1);
    expect(Math.abs(noteMetrics.selectedX - noteMetrics.expectedCenterX)).toBeLessThanOrEqual(25);
    expect(Math.abs(noteMetrics.selectedY - noteMetrics.expectedCenterY)).toBeLessThanOrEqual(25);

    // Screenshot da constelação íntima com drawer aberto
    await page.screenshot({
      path: path.join(artifactDir, 'note_constellation_intimate_zoom.png'),
    });
  });
});
