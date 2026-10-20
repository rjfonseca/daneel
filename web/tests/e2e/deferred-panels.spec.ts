import { test, expect } from '@playwright/test';

// Exercise document workflows without decorative GPU animation; camera invariants cover the default mode.
test.use({ reducedMotion: 'reduce' });

test('loads tools and Markdown on demand and preserves cockpit state', async ({ page }) => {
  test.skip(process.env.DANEEL_E2E_PREVIEW !== '1' && process.env.DANEEL_E2E_EMBEDDED !== '1', 'Requires a production build to inspect chunk loading');
  test.setTimeout(90000);
  const scripts: string[] = [];
  page.on('request', request => {
    if (request.resourceType() === 'script') scripts.push(new URL(request.url()).pathname);
  });
  await page.goto('/');
  await expect(page.getByTestId('cosmic-loading-overlay')).toBeHidden({ timeout: 30000 });
  expect(scripts.some(path => /CockpitModal|CommandSearchDialog|vendor-markdown/.test(path))).toBe(false);

  const openCockpit = async () => {
    await page.getByRole('button', { name: 'More actions' }).click();
    await page.getByTestId('cockpit-toggle-btn').click();
  };
  await openCockpit();
  await expect(page.getByTestId('cockpit-modal')).toBeVisible();
  expect(scripts.some(path => path.includes('CockpitModal'))).toBe(true);
  const telemetry = page.getByTestId('cockpit-tab-telemetry');
  await telemetry.click();
  await expect(telemetry).toHaveClass(/bg-indigo-500\/20/);
  await page.getByTestId('cockpit-close-btn').click();
  await expect(page.getByTestId('cockpit-modal')).toBeHidden();
  await openCockpit();
  await expect(telemetry).toHaveClass(/bg-indigo-500\/20/);
  await page.getByTestId('cockpit-close-btn').click();

  await page.getByTestId('quick-search-btn').click();
  await page.locator('[data-slot="command-input"]').fill('cache');
  await page.locator('[data-slot="command-item"]').first().click();
  await expect(page.getByTestId('floating-note-hud')).toBeVisible({ timeout: 30000 });
  expect(scripts.some(path => path.includes('vendor-markdown'))).toBe(true);
  await expect(page.getByTestId('floating-note-hud')).toContainText('Cache and system design notes.');
});
