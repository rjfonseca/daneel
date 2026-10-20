import { test, expect } from '@playwright/test';
import * as path from 'path';

test.use({ reducedMotion: 'reduce' });

test.describe('Radial Outward Satellites and Project Repository Nebulas', () => {
  const artifactDir = process.env.ARTIFACT_DIR || path.join(process.cwd(), 'test-results');

  test('Renders outward satellite labels and groups repository notes into project nebulas', async ({ page }) => {
    test.setTimeout(60000);
    await page.goto('/');
    await expect(page.getByTestId('cosmic-loading-overlay')).toBeHidden({ timeout: 30000 });

    // 1. Full screenshot showing satellites pointing outwards
    await page.screenshot({ 
      path: path.join(artifactDir, 'outward_arms_satellite_callouts.png') 
    });

    // 2. Open search via HUD Quick Search
    const searchBtn = page.locator('[data-testid="quick-search-btn"]');
    await expect(searchBtn).toBeVisible();
    await searchBtn.click();
    await page.waitForTimeout(500);

    const searchInput = page.locator('input[placeholder*="Search notes"]');
    await expect(searchInput).toBeVisible();
    await searchInput.fill('orion');
    await page.waitForTimeout(1000);

    // Save screenshot of search results
    await page.screenshot({ 
      path: path.join(artifactDir, 'project_nebula_search.png') 
    });

    // Focus on first nebula item if present
    const nebulaItem = page.getByRole('option').filter({ hasText: 'Cluster with 4 note(s).' });
    await expect(nebulaItem).toBeVisible();
    await nebulaItem.click();
    await expect(page).toHaveURL(/#cluster=sources%2Fgithub%2Facme%2Forion/);
    await page.screenshot({ path: path.join(artifactDir, 'project_nebula_focused.png') });
  });
});
