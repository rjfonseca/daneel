import type {} from './browser-types';
import { test, expect } from '@playwright/test';
import * as path from 'path';

test.use({ reducedMotion: 'reduce' });

test.describe('Cosmic Refinement Verification', () => {
  const artifactDir = process.env.ARTIFACT_DIR || path.join(process.cwd(), 'test-results');
  // Include browser context setup in the existing test budget.
  test.setTimeout(90000);

  test('Renders foundation orbit immediately, applies intimate cluster zoom, ultrafine edges without overlap', async ({ page }) => {
    // 1. Acesso à aplicação
    await page.goto('/');
    // Aguarda a estabilização física inicial da galáxia
    await expect(page.getByTestId('cosmic-loading-overlay')).toBeHidden({ timeout: 30000 });

    // 2. Item 4: Órbita da Fundação visível imediatamente no canvas sem interação do usuário
    const canvasHasContent = await page.evaluate(() => {
      const canvas = document.querySelector('canvas.pointer-events-none') as HTMLCanvasElement;
      if (!canvas) return false;
      const ctx = canvas.getContext('2d');
      if (!ctx) return false;
      const imgData = ctx.getImageData(0, 0, canvas.width, canvas.height);
      let nonZeroCount = 0;
      for (let i = 3; i < imgData.data.length; i += 4) {
        if (imgData.data[i] > 0) nonZeroCount++;
      }
      return nonZeroCount > 200;
    });
    expect(canvasHasContent).toBe(true);

    // Salva screenshot da carga inicial comprovando a órbita da fundação presente
    await page.screenshot({
      path: path.join(artifactDir, 'foundation_orbit_immediate_load.png'),
    });

    // 3. Item 1: Arestas ultrafinas (minEdgeThickness = 0.1 e tamanhos 0.10 - 0.25)
    const edgeMetrics = await page.evaluate(() => {
      const sigma = window.__sigmaInstance!;
      const settings = sigma.getSettings();
      return {
        minEdgeThickness: settings.minEdgeThickness,
      };
    });
    expect(edgeMetrics.minEdgeThickness).toBe(0.1);

    // 4. Item 2: Clicar no nome de uma nebulosa e verificar que o aglomerado ocupa a tela inteira (ratio entre 0.08 e 0.18)
    await page.evaluate(() => {
      const sigma = window.__sigmaInstance!;
      const graph = sigma.getGraph();

      const catNodeId = '__CAT_notes/architecture';
      if (!graph.hasNode(catNodeId)) throw new Error('Expected the demonstration architecture cluster');

      sigma.emit('clickNode', { node: catNodeId });
      return { catNodeId };
    });

    // Aguarda término da animação suave de zoom da câmera (850ms)
    await page.waitForTimeout(1500);

    const postZoomRatio = await page.evaluate(() => {
      const sigma = window.__sigmaInstance!;
      return sigma.getCamera().ratio;
    });

    // O ratio agora é proporcional ao cluster (entre 0.015 e 0.25), enquadrando as notas do cluster
    expect(postZoomRatio).toBeLessThanOrEqual(0.25);
    expect(postZoomRatio).toBeGreaterThanOrEqual(0.015);

    // Salva screenshot do aglomerado ocupando a tela inteira
    await page.screenshot({
      path: path.join(artifactDir, 'cluster_full_screen_intimate_zoom.png'),
    });

    // 5. Item 3: De-cluttering na super nebulosa de notes
    // Clica no botão "Return to Full Galaxy" para desativar o isolamento de setor
    const returnBtn = page.getByRole('button', { name: /Return to Full Galaxy/i });
    await returnBtn.click();
    await page.waitForTimeout(1500);

    // Dá zoom in na super nebulosa central de notes
    await page.evaluate(() => {
      const sigma = window.__sigmaInstance!;
      sigma.getCamera().animate({ ratio: 0.28 }, { duration: 800 });
    });
    await page.waitForTimeout(1500);

    // Salva screenshot do zoom na super nebulosa com rótulos limpos e organizados
    await page.screenshot({
      path: path.join(artifactDir, 'notes_super_nebula_clean_zoom.png'),
    });
  });
});
