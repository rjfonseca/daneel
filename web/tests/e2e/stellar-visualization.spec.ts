import type {} from './browser-types';
import { test, expect } from '@playwright/test';
import * as path from 'path';

test.use({ reducedMotion: 'reduce' });

test.describe('Visualização Astrofísica e Filtragem de Arestas', () => {
  test('Deve renderizar os controles de aresta, legenda estelar e aplicar os modos', async ({ page }) => {
    test.setTimeout(60000);
    await page.goto('/');
    await page.waitForSelector('[data-testid="cosmic-loading-overlay"]', { state: 'detached', timeout: 30000 }).catch(() => {});
    await page.waitForTimeout(1000);

    // 1. Abre menu de controles do HUD
    await page.locator('[data-testid="toggle-controls-btn"]').click();
    await page.waitForTimeout(400);

    const autoBtn = page.locator('[data-testid="edge-mode-auto"]');
    const bridgesBtn = page.locator('[data-testid="edge-mode-bridges"]');
    const focusBtn = page.locator('[data-testid="edge-mode-focus"]');
    const allBtn = page.locator('[data-testid="edge-mode-all"]');

    await expect(autoBtn).toBeVisible();
    await expect(bridgesBtn).toBeVisible();
    await expect(focusBtn).toBeVisible();
    await expect(allBtn).toBeVisible();

    // Focus deve vir ativo por padrão (estética limpa estelar)
    await expect(focusBtn).toHaveClass(/bg-indigo-600/);

    const diagnostics = await page.evaluate(() => {
      const sigma = window.__sigmaInstance;
      return { hasInstance: !!sigma };
    });
    console.log('DIAGNOSTICS:', diagnostics);

    // Captura screenshot da Galáxia com Focus ativado (visão limpa sem linhas confusas)
    const artifactDir = process.env.ARTIFACT_DIR || path.join(process.cwd(), 'test-results');
    await page.screenshot({ 
      path: path.join(artifactDir, 'stellar_galaxy_overview_lod.png') 
    });

    // 2. Alterna para modo "Pontes"
    await bridgesBtn.click();
    await expect(bridgesBtn).toHaveClass(/bg-indigo-600/);
    await page.waitForTimeout(1000);

    await page.screenshot({ 
      path: path.join(artifactDir, 'stellar_galaxy_bridges_mode.png') 
    });

    // 3. Verifica a Legenda do Espectro Astrofísico
    await page.getByText('Stellar spectrum help', { exact: true }).click();
    await expect(page.getByText('Star colors describe recency and connection mass.')).toBeVisible();
    await expect(page.getByText('Supermassive Quasar', { exact: true })).toBeVisible();
    await expect(page.getByText('Blue Giant (Class O/B)', { exact: true })).toBeVisible();

    // 4. Retorna para o modo Auto
    await autoBtn.click();
    await expect(autoBtn).toHaveClass(/bg-indigo-600/);
  });

  test('Deve abrir uma nota e exibir o badge astrofísico correspondente no cabeçalho do drawer', async ({ page }) => {
    test.setTimeout(60000);
    await page.goto('/');
    await page.waitForSelector('[data-testid="cosmic-loading-overlay"]', { state: 'detached', timeout: 30000 }).catch(() => {});
    await page.waitForTimeout(1000);

    const artifactDir = process.env.ARTIFACT_DIR || path.join(process.cwd(), 'test-results');

    // Busca uma nota via Quick Search
    await page.locator('[data-testid="quick-search-btn"]').click();
    await page.waitForTimeout(400);

    const searchInput = page.locator('input[placeholder*="Search notes"]');
    await searchInput.fill('cache');
    await page.waitForTimeout(1000);

    // Clica no primeiro resultado de nota
    const firstOption = page.locator('[role="option"]').filter({ hasNotText: /NEBULA|NEBULOSA/ }).first();
    await expect(firstOption).toBeVisible({ timeout: 5000 });
    await firstOption.click();
    await page.waitForTimeout(1500);

    // Floating HUD card deve abrir com badge visível
    await expect(page.locator('[data-testid="floating-note-hud"]')).toBeVisible();

    await page.screenshot({ 
      path: path.join(artifactDir, 'stellar_note_drawer_badge.png') 
    });
  });

  test('Deve focar em um setor estelar e exibir as estrelas e arestas detalhadas do cluster', async ({ page }) => {
    test.setTimeout(60000);
    await page.goto('/');
    await page.waitForSelector('[data-testid="cosmic-loading-overlay"]', { state: 'detached', timeout: 30000 }).catch(() => {});
    await page.waitForTimeout(1000);

    const artifactDir = process.env.ARTIFACT_DIR || path.join(process.cwd(), 'test-results');

    // Foca um setor estelar via busca por nebulosa
    await page.locator('[data-testid="quick-search-btn"]').click();
    await page.waitForTimeout(400);

    const searchInput = page.locator('input[placeholder*="Search notes"]');
    await searchInput.fill('architecture');
    await page.waitForTimeout(1000);

    await expect(page.getByRole('option').first()).toBeVisible();
    const nebulaItem = page.getByRole('option').filter({ hasText: 'Cluster with 16 note(s).' });
    for (let batch = 0; batch < 6 && !await nebulaItem.isVisible(); batch++) {
      await page.getByRole('button', { name: 'Show more results' }).click();
    }
    await expect(nebulaItem).toBeVisible({ timeout: 5000 });
    const itemText = await nebulaItem.textContent();
    console.log('CLICKED ITEM TEXT:', itemText);
    await nebulaItem.click();
    await page.waitForTimeout(2500);

    const camState = await page.evaluate(() => {
      const sigma = window.__sigmaInstance;
      if (!sigma) return 'No sigma instance';
      return {
        camera: sigma.getCamera().getState(),
        order: sigma.getGraph().order,
        size: sigma.getGraph().size,
      };
    });
    console.log('CAM STATE:', camState);

    await page.screenshot({ 
      path: path.join(artifactDir, 'stellar_sector_deep_zoom.png') 
    });
  });
});
