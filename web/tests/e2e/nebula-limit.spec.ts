import type {} from './browser-types';
import { test, expect } from '@playwright/test';
import path from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

test.use({ reducedMotion: 'reduce' });

test.describe('Nebula Limits & Stepper Configuration', () => {
  test('initializes with default limit 6, allows adjustment via stepper, and persists in localStorage', async ({ page }) => {
    const errors: string[] = [];
    page.on('console', (msg) => {
      const fullText = `${msg.text()} at ${msg.location().url}`;
      if (msg.type() === 'error' && !fullText.includes('favicon.ico')) {
        errors.push(`Console Error: ${fullText}`);
      }
    });
    page.on('pageerror', (err) => {
      errors.push(`Page Error: ${err.message}`);
    });

    // Clear any previous localStorage key before loading
    await page.addInitScript(() => {
      localStorage.removeItem('daneel_max_nebulas');
    });

    test.setTimeout(60000);
    await page.goto('/');
    await page.waitForSelector('[data-testid="cosmic-loading-overlay"]', { state: 'detached', timeout: 30000 }).catch(() => {});
    await page.waitForTimeout(1000);

    // Verify canvas is rendered
    await expect(page.locator('canvas').first()).toBeVisible();

    // Open HUD controls popover
    await page.locator('[data-testid="toggle-controls-btn"]').click();
    await page.waitForTimeout(400);

    // Verify default limit is 6
    const nebulaValue = page.locator('[data-testid="max-nebulas-value"]');
    await expect(nebulaValue).toHaveText('6');

    // Screenshot initial state with 6 nebulas
    const artifactDir = process.env.ARTIFACT_DIR || path.join(__dirname, '../../test-results');
    await page.screenshot({ path: path.join(artifactDir, 'nebula_limit_default_6.png') });

    // Click "+" button twice
    const increaseBtn = page.locator('[data-testid="increase-max-nebulas"]');
    await increaseBtn.click();
    await expect(nebulaValue).toHaveText('7');

    await increaseBtn.click();
    await expect(nebulaValue).toHaveText('8');

    // Verify localStorage was updated
    const storedValueAfterIncrease = await page.evaluate(() => localStorage.getItem('daneel_max_nebulas'));
    expect(storedValueAfterIncrease).toBe('8');

    // Screenshot state with 8 nebulas
    await page.screenshot({ path: path.join(artifactDir, 'nebula_limit_increased_8.png') });

    // Click "-" button three times
    const decreaseBtn = page.locator('[data-testid="decrease-max-nebulas"]');
    await decreaseBtn.click();
    await decreaseBtn.click();
    await decreaseBtn.click();
    await expect(nebulaValue).toHaveText('5');

    const storedValueAfterDecrease = await page.evaluate(() => localStorage.getItem('daneel_max_nebulas'));
    expect(storedValueAfterDecrease).toBe('5');

    // Screenshot state with 5 nebulas
    await page.screenshot({ path: path.join(artifactDir, 'nebula_limit_decreased_5.png') });

    // Reset back to 6 for standard state
    await increaseBtn.click();
    await expect(nebulaValue).toHaveText('6');

    // Verify reset camera button exists and functions without error
    const resetCameraBtn = page.locator('[data-testid="reset-camera-btn"]');
    await expect(resetCameraBtn).toBeVisible();
    await resetCameraBtn.click();
    await page.waitForTimeout(1000);

    // Verify no console errors occurred
    expect(errors).toEqual([]);
  });

  test('renders distinct cybernetic HUD callout for sectors with beacon reticle and tethers', async ({ page }) => {
    test.setTimeout(60000);
    await page.goto('/');
    await page.waitForSelector('[data-testid="cosmic-loading-overlay"]', { state: 'detached', timeout: 30000 }).catch(() => {});
    await page.waitForTimeout(1000);

    const sectorBox = await page.evaluate(() => {
      const boxes = window.__sigmaContext?.frameCalloutBoxes || [];
      const sec = boxes.find((b) => b.isSector);
      return sec ? {
        nodeId: sec.nodeId,
        category: sec.category,
        isSector: sec.isSector,
        minX: sec.minX,
        maxX: sec.maxX,
        minY: sec.minY,
        maxY: sec.maxY,
        width: sec.maxX - sec.minX,
        height: sec.maxY - sec.minY,
      } : null;
    });

    expect(sectorBox).not.toBeNull();
    expect(sectorBox!.isSector).toBe(true);
    expect(sectorBox!.height).toBeGreaterThanOrEqual(30);

    const artifactDir = process.env.ARTIFACT_DIR || path.join(__dirname, '../../test-results');
    await page.screenshot({ path: path.join(artifactDir, 'sector_visual_identity_overview.png') });
  });
});
