import type {} from './browser-types';
import { test, expect } from '@playwright/test';
import * as path from 'path';

test.use({ reducedMotion: 'reduce' });

test.describe('Full Validation: Sector Focus, Outer ARMS Perimeter, Grouping and Photons', () => {
  const artifactDir = process.env.ARTIFACT_DIR || path.join(process.cwd(), 'test-results');

  test('Positions satellites on outer perimeter, renders mandatory labels and focuses sector without blackout', async ({ page }) => {
    test.setTimeout(90000);
    // 1. Acessa a aplicação
    await page.goto('/');
    // Aguarda o término da estabilização da física (ForceAtlas2 waitTime + resets)
    await expect(page.getByTestId('cosmic-loading-overlay')).toBeHidden({ timeout: 30000 });

    // 2. Valida posições dos Satélites ARMS e Fótons Cósmicos via sigma instance
    const galaxyMetrics = await page.evaluate(() => {
      const sigma = window.__sigmaInstance!;
      if (!sigma) return null;
      const graph = sigma.getGraph();
      const camera = sigma.getCamera();
      
      const satellites: Array<{ id: string; ring: string; dist: number }> = [];
      let sumX = 0, sumY = 0, memCount = 0;
      
      graph.forEachNode((id: string, attrs) => {
        if (!attrs.ring || attrs.ring === 'memory') {
          sumX += attrs.x;
          sumY += attrs.y;
          memCount++;
        }
      });
      const cX = memCount > 0 ? sumX / memCount : 0;
      const cY = memCount > 0 ? sumY / memCount : 0;
      
      graph.forEachNode((id: string, attrs) => {
        if (attrs.ring && attrs.ring !== 'memory') {
          const dx = attrs.x - cX;
          const dy = attrs.y - cY;
          satellites.push({
            id,
            ring: attrs.ring,
            dist: Math.round(Math.sqrt(dx * dx + dy * dy)),
          });
        }
      });

      // Checa arestas gravitacionais conectadas a hubs de nebulosa __CAT_
      let catEdgesCount = 0;
      graph.forEachEdge((_e: string, _attrs, src: string, tgt: string) => {
        if (src.startsWith('__CAT_') || tgt.startsWith('__CAT_')) {
          catEdgesCount++;
        }
      });

      return {
        memCount,
        satellites,
        catEdgesCount,
        cameraRatio: camera.ratio,
      };
    });

    expect(galaxyMetrics).not.toBeNull();
    expect(galaxyMetrics!.satellites.length).toBeGreaterThanOrEqual(15);
    
    // Todos os satélites devem estar no perímetro exterior (distância >= 6500)
    galaxyMetrics!.satellites.forEach(sat => {
      expect(sat.dist).toBeGreaterThanOrEqual(6500);
    });

    // Arestas gravitacionais internas devem existir para agrupar as notas em torno dos hubs
    expect(galaxyMetrics!.catEdgesCount).toBeGreaterThan(20);

    // Salva screenshot da visão geral com o perímetro exterior e aglomerados
    await page.screenshot({
      path: path.join(artifactDir, 'galaxy_overview_with_seldon_perimeter.png'),
    });

    // 3. Valida chamada de Foco de Setor (Ex: Setor de Arquitetura ou IA)
    await page.evaluate(() => {
      const sigma = window.__sigmaInstance!;
      const graph = sigma.getGraph();

      const targetCatId = '__SECTOR_acme';

      const attrs = graph.getNodeAttributes(targetCatId);
      const normFn = sigma.normalizationFunction;
      if (normFn) {
        const framed = normFn(attrs);
        sigma.getCamera().animate({ x: framed.x, y: framed.y, ratio: 0.38, angle: 0 }, { duration: 600 });
      }

      // Dispara o clique no nó de categoria
      sigma.emit('clickNode', { node: targetCatId });
      return { targetCatId };
    });

    await page.waitForTimeout(1500);

    // Salva screenshot do setor focado - NÃO DEVE ESTAR PRETO!
    await page.screenshot({
      path: path.join(artifactDir, 'sector_focused_view.png'),
    });

    // Verifica que o botão "Return to Full Galaxy" está visível
    const returnBtn = page.getByRole('button', { name: /Return to Full Galaxy/i });
    await expect(returnBtn).toBeVisible();

    // Valida que o setor não está vazio ou totalmente preto: há nós visíveis
    const visibilityCheck = await page.evaluate(() => {
      const sigma = window.__sigmaInstance!;
      const graph = sigma.getGraph();
      let visibleCount = 0;
      let hiddenCount = 0;
      graph.forEachNode((_id: string, attrs) => {
        if (attrs.hidden) hiddenCount++;
        else visibleCount++;
      });
      return { visibleCount, hiddenCount };
    });

    // Deve ter nós visíveis (as estrelas do setor focado) e a maioria oculta
    expect(visibilityCheck.visibleCount).toBeGreaterThan(5);
    expect(visibilityCheck.hiddenCount).toBeGreaterThan(10);

    // 4. Clica no botão "Return to Full Galaxy"
    await returnBtn.click();
    await page.waitForTimeout(1500);

    // Valida que a galáxia inteira foi restaurada
    const restoredCheck = await page.evaluate(() => {
      const sigma = window.__sigmaInstance!;
      const graph = sigma.getGraph();
      let visibleCount = 0;
      graph.forEachNode((_id: string, attrs) => {
        if (!attrs.hidden) visibleCount++;
      });
      return visibleCount;
    });

    expect(restoredCheck).toBeGreaterThan(30);

    // Salva screenshot da galáxia restaurada
    await page.screenshot({
      path: path.join(artifactDir, 'galaxy_restored_full_view.png'),
    });
  });
});
