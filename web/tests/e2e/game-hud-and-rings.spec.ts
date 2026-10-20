import { test, expect } from '@playwright/test';
import * as path from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

test.describe('HUD Estilo Jogo, Anéis ARMS e Isolamento de Constelação', () => {
  const artifactDir = process.env.ARTIFACT_DIR || path.join(__dirname, '../../test-results');

  test('Deve renderizar HUD cósmico minimalista, alternar modo cósmico e anéis ARMS', async ({ page }) => {
    test.setTimeout(90000);
    const errors: string[] = [];
    page.on('console', (msg) => {
      const full = `${msg.text()} at ${msg.location().url}`;
      if (msg.type() === 'error' && !full.includes('favicon.ico')) {
        errors.push(`Console Error: ${full}`);
      }
    });

    await page.goto('/');
    await page.waitForSelector('[data-testid="cosmic-loading-overlay"]', { state: 'detached', timeout: 30000 }).catch(() => {});
    await page.waitForTimeout(1000);

    // 1. Verifica barra superior cósmica e Big Numbers
    await expect(page.getByText('PRIME RADIANT')).toBeVisible();
    await expect(page.getByTestId('cosmic-stat-notes')).toHaveText('58');
    await expect(page.getByTestId('cosmic-stat-links')).toHaveText('104');
    await expect(page.getByTestId('cosmic-stat-harvesters')).toHaveText('5');

    // Captura screenshot da visão inicial limpa (galaxy + HUD)
    await page.screenshot({ 
      path: path.join(artifactDir, 'hud_game_galaxy_overview.png') 
    });

    // Toggle photons and orbital rings in the unified View panel.
    const viewBtn = page.locator('[data-testid="toggle-controls-btn"]');
    await expect(viewBtn).toBeVisible();
    await viewBtn.click();
    await page.waitForTimeout(300);

    const cosmicBtn = page.locator('[data-testid="toggle-cloak-photons"]');
    await expect(cosmicBtn).toBeVisible();
    await expect(cosmicBtn).toHaveAttribute('aria-pressed', 'true');

    // Desativa Modo Cósmico
    await cosmicBtn.click();
    await page.waitForTimeout(600);
    await expect(cosmicBtn).toHaveAttribute('aria-pressed', 'false');

    // Reativa Modo Cósmico
    await cosmicBtn.click();
    await page.waitForTimeout(600);
    await expect(cosmicBtn).toHaveAttribute('aria-pressed', 'true');

    // Captura screenshot com Modo Cósmico ativo
    await page.screenshot({ 
      path: path.join(artifactDir, 'hud_cosmic_mode_active.png') 
    });

    // 3. Verifica Anéis ARMS toggle (desativa e reativa)
    const ringsBtn = page.locator('[data-testid="toggle-cloak-perimeter"]');
    await expect(ringsBtn).toBeVisible();
    await expect(ringsBtn).toHaveAttribute('aria-pressed', 'true');

    // Desativa anéis ARMS (oculta anéis e estações)
    await ringsBtn.click();
    await page.waitForTimeout(600);
    await expect(ringsBtn).toHaveAttribute('aria-pressed', 'false');
    await page.screenshot({ 
      path: path.join(artifactDir, 'hud_arms_toggled_off.png') 
    });

    // Reativa anéis ARMS
    await ringsBtn.click();
    await page.waitForTimeout(600);
    await expect(ringsBtn).toHaveAttribute('aria-pressed', 'true');

    // Close the View panel.
    await viewBtn.click();
    await page.waitForTimeout(300);

    // 4. Abre menu de controles sob demanda
    const controlsBtn = page.locator('[data-testid="toggle-controls-btn"]');
    await controlsBtn.click();
    await page.waitForTimeout(500);

    const controlsPopover = page.locator('[data-testid="controls-popover"]');
    await expect(controlsPopover).toBeVisible();
    await expect(page.locator('[data-testid="max-nebulas-value"]')).toHaveText('6');
    await expect(page.locator('[data-testid="edge-mode-auto"]')).toBeVisible();

    // Fecha menu de controles
    await controlsBtn.click();
    await expect(controlsPopover).not.toBeVisible();

    expect(errors).toEqual([]);
  });

  test('Deve buscar nota, isolar constelação e renderizar FloatingNoteCard HUD', async ({ page }) => {
    test.setTimeout(60000);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await page.goto('/');
    await page.waitForSelector('[data-testid="cosmic-loading-overlay"]', { state: 'detached', timeout: 30000 }).catch(() => {});
    await page.waitForTimeout(1000);

    // 1. Abre busca via HUD Quick Search (Cmd+K)
    const searchBtn = page.locator('[data-testid="quick-search-btn"]');
    await searchBtn.click();
    await page.waitForTimeout(500);

    // Digita query de busca
    const searchInput = page.locator('input[placeholder*="Search notes"]');
    await expect(searchInput).toBeVisible();
    await searchInput.fill('architecture example 0');
    await page.waitForTimeout(1000);

    // Seleciona um resultado de nota (ignora categoria NEBULOSA / NEBULA)
    const noteOption = page.getByRole('option').filter({ hasText: 'notes/architecture/note-0.md' });
    await expect(noteOption).toBeVisible({ timeout: 5000 });
    await noteOption.click();
    await page.waitForTimeout(1500);

    // 2. Verifica presença do FloatingNoteCard
    const floatingHud = page.locator('[data-testid="floating-note-hud"]');
    await expect(floatingHud).toBeVisible();

    // 3. Verifica Badge de Isolamento de Constelação ativo
    const constellationBadge = page.locator('[data-testid="constellation-hud-badge"]');
    await expect(constellationBadge).toBeVisible();
    await expect(constellationBadge).toHaveText(/Isolated Constellation/);

    // Captura screenshot da constelação isolada com floating HUD
    await page.screenshot({ 
      path: path.join(artifactDir, 'hud_constellation_isolation_active.png') 
    });

    // 4. Maximiza e restaura o HUD
    const maxBtn = page.locator('[data-testid="toggle-maximize-note"]');
    await maxBtn.click();
    await page.waitForTimeout(500);
    await maxBtn.click();
    await page.waitForTimeout(500);

    // 5. Restaura constelação via badge
    await constellationBadge.click();
    await page.waitForTimeout(800);
    await expect(constellationBadge).not.toBeVisible();

    // Fecha HUD
    const closeBtn = page.locator('[data-testid="close-note-drawer"]');
    await closeBtn.click();
    await expect(floatingHud).not.toBeVisible();
  });

  test('Deve inspecionar satélite orbital ARMS com telemetria holográfica', async ({ page }) => {
    test.setTimeout(60000);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await page.goto('/');
    await page.waitForSelector('[data-testid="cosmic-loading-overlay"]', { state: 'detached', timeout: 30000 }).catch(() => {});
    await page.waitForTimeout(1000);

    // Abre busca para localizar estação orbital Harvester
    const searchBtn = page.locator('[data-testid="quick-search-btn"]');
    await searchBtn.click();
    await page.waitForTimeout(500);

    const searchInput = page.locator('input[placeholder*="Search notes"]');
    await searchInput.fill('harvester');
    await page.waitForTimeout(1000);

    const harvesterOption = page.locator('[role="option"]').filter({ hasText: 'Harvester' }).first();
    await expect(harvesterOption).toBeVisible({ timeout: 5000 });
    await harvesterOption.click();
    await page.waitForTimeout(1500);

    // Verifica se abriu card de inspeção holográfico ARMS
    const inspectorCard = page.locator('[data-testid="arms-inspector-card"]');
    await expect(inspectorCard).toBeVisible();
    await expect(inspectorCard.getByText('online', { exact: true })).toBeVisible();
    const headerBounds = await page.getByTestId('command-bridge').boundingBox();
    const inspectorBounds = await inspectorCard.boundingBox();
    expect(headerBounds).not.toBeNull();
    expect(inspectorBounds).not.toBeNull();
    expect(Math.abs(inspectorBounds!.y - headerBounds!.y - headerBounds!.height - 8)).toBeLessThanOrEqual(1);
    await page.screenshot({ 
      path: path.join(artifactDir, 'hud_arms_station_inspector.png') 
    });
    await page.locator('[data-testid="close-arms-inspector"]').click();
    await expect(inspectorCard).not.toBeVisible();
  });
});
