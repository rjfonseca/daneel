import { test, expect } from '@playwright/test';

test.describe('App Initialization Guardrails', () => {
  test('loads the bundled interface font', async ({ page }) => {
    await page.goto('/');
    const loadedFonts = await page.evaluate(async () => {
      const faces = await document.fonts.load('400 16px "Geist Variable"', 'Daneel');
      return faces.map(face => face.status);
    });
    expect(loadedFonts).toContain('loaded');
  });

  test('Deve carregar a página inicial sem NENHUM erro no console ou falha do React', async ({ page }) => {
    const errors: string[] = [];

    // Capture console errors
    page.on('console', (msg) => {
      // Ignore routine Vite or HMR logs or favicon 404
      const full = `${msg.text()} at ${msg.location().url}`;
      if (msg.type() === 'error' && !full.includes('favicon.ico')) {
        errors.push(`Console Error: ${full}`);
      }
    });

    // Capture uncaught exceptions / React Error Boundary crashes
    page.on('pageerror', (err) => {
      errors.push(`Page Error: ${err.message}`);
    });

    // Navigate to the app (now using the real backend via webServer proxy)
    await page.goto('/');
    
    // Wait for the app to settle
    await page.waitForTimeout(3000);
    
    // Check if there are any caught errors (including 404s!)
    expect(errors).toEqual([]);
    
    // Verify that the main UI elements are present
    await expect(page.locator('canvas').first()).toBeVisible();
  });
});
