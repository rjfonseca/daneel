import { expect, test, type Page } from '@playwright/test';

test.use({ reducedMotion: 'reduce', timezoneId: 'UTC' });

const sizes = [
  { width: 1440, height: 900 },
  { width: 1024, height: 768 },
  { width: 768, height: 1024 },
  { width: 390, height: 844 },
  { width: 320, height: 740 },
  { width: 844, height: 390 },
];

async function ready(page: Page) {
  await page.goto('/');
  await expect(page.getByTestId('cosmic-loading-overlay')).toBeHidden({ timeout: 30000 });
}

test('keeps populated Recent and Saved disclosures scrollable in short landscape viewports', async ({ page }) => {
  test.setTimeout(90000);
  await page.setViewportSize({ width: 844, height: 390 });
  await page.addInitScript(() => {
    localStorage.setItem('daneel:recent-notes:v1', JSON.stringify(Array.from({ length: 20 }, (_, i) => ({
      id: `notes/example-${i}.md`, title: `Example note ${i}`,
    }))));
    localStorage.setItem('daneel:saved-searches:v1', JSON.stringify(Array.from({ length: 20 }, (_, i) => ({
      query: `Example search ${i}`,
    }))));
  });
  await ready(page);
  for (const [trigger, region, last] of [
    ['Recent', 'Recent notes', 'Example note 19'],
    ['Saved', 'Saved searches', 'Example search 19'],
  ]) {
    await page.getByRole('button', { name: 'More actions' }).click();
    await page.getByRole('button', { name: trigger, exact: true }).click();
    const panel = page.getByRole('region', { name: region, exact: true });
    await expect(panel).toBeVisible();
    const bounds = (await panel.boundingBox())!;
    expect(bounds.y + bounds.height).toBeLessThanOrEqual(382);
    const lastItem = panel.getByRole('button', { name: last, exact: true });
    await lastItem.scrollIntoViewIfNeeded();
    await expect(lastItem).toBeInViewport();
    await page.keyboard.press('Escape');
    await expect(panel).toBeHidden();
  }
});

async function assertHeaderBounds(page: Page, width: number, height: number) {
  const header = page.getByTestId('command-bridge');
  await expect(header).toBeVisible();
  const box = await header.boundingBox();
  expect(box).not.toBeNull();
  expect(box!.x).toBeGreaterThanOrEqual(12);
  expect(box!.x + box!.width).toBeLessThanOrEqual(width - 12);
  const controls = await header.locator('button:visible').evaluateAll(elements => elements.map(element => {
    const rect = element.getBoundingClientRect();
    return { x: rect.x, y: rect.y, right: rect.right, bottom: rect.bottom, width: rect.width, height: rect.height };
  }));
  for (const control of controls) {
    expect(control.x).toBeGreaterThanOrEqual(0);
    expect(control.right).toBeLessThanOrEqual(width);
    expect(control.y).toBeGreaterThanOrEqual(0);
    expect(control.bottom).toBeLessThanOrEqual(height);
  }
  for (let index = 1; index < controls.length; index += 1) {
    const previous = controls[index - 1];
    const current = controls[index];
    const intersects = previous.x < current.right && previous.right > current.x && previous.y < current.bottom && previous.bottom > current.y;
    expect(intersects, `visible controls ${index - 1} and ${index} must not overlap`).toBe(false);
  }
}

test('keeps one two-row Command Bridge within desktop, tablet, mobile, zoom-equivalent, and landscape widths', async ({ page }, testInfo) => {
  test.setTimeout(90000);
  await ready(page);
  for (const { width, height } of sizes) {
    await page.setViewportSize({ width, height });
    await expect.poll(async () => page.getByTestId('command-bridge').locator('button[aria-label="More actions"]').isVisible()).toBe(true);
    await assertHeaderBounds(page, width, height);
    const header = page.getByTestId('command-bridge');
    await expect(header.locator(':scope > div > div').nth(0)).toBeVisible();
    await expect(header.locator(':scope > div > div').nth(1)).toBeVisible();
    if (width === 1440) await page.screenshot({ path: testInfo.outputPath('command-bridge-desktop.png') });
    if (width === 390) await page.screenshot({ path: testInfo.outputPath('command-bridge-mobile.png') });
    if (width === 320) {
      await expect(header.getByRole('button', { name: 'Notes', exact: true })).toBeVisible();
      await expect(header.getByTestId('quick-search-btn')).toBeVisible();
      await expect(header.getByTestId('capture-note-btn')).toBeVisible();
      await expect(header.getByRole('button', { name: 'View settings' })).toBeVisible();
    }
  }
});

test('keeps More disclosures, note reading, dialogs, and visible focus coherent', async ({ page }, testInfo) => {
  test.setTimeout(90000);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/#note=daily%2F2026-10-16.md');
  await expect(page.getByTestId('cosmic-loading-overlay')).toBeHidden({ timeout: 30000 });
  const reader = page.getByTestId('floating-note-hud');
  await expect(reader).toBeVisible({ timeout: 30000 });
  const originalUrl = page.url();
  const more = page.getByRole('button', { name: 'More actions' });
  await more.click();
  await page.getByRole('button', { name: 'Recent', exact: true }).click();
  await expect(page.getByRole('region', { name: 'Recent notes' })).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('region', { name: 'Recent notes' })).toBeHidden();
  await expect(reader).toBeVisible();
  expect(page.url()).toBe(originalUrl);
  await expect(more).toBeFocused();

  await more.click();
  await page.getByRole('button', { name: /keyboard shortcuts/i }).click();
  const shortcuts = page.getByRole('dialog', { name: 'COSMIC COMMAND INTERFACE' });
  await expect(shortcuts).toBeVisible({ timeout: 30000 });
  await page.setViewportSize({ width: 844, height: 390 });
  await page.keyboard.press('Escape');
  await expect(shortcuts).toBeHidden();
  await expect(more).toBeFocused();

  await page.getByTestId('quick-search-btn').click();
  const search = page.getByRole('dialog', { name: 'Search notes', exact: true });
  await expect(search).toBeVisible({ timeout: 30000 });
  await page.keyboard.press('Escape');
  await expect(search).toBeHidden();
  await page.keyboard.press('Control+k');
  await expect(search).toBeVisible();
  for (let index = 0; index < 8; index += 1) {
    await page.keyboard.press('Tab');
    expect(await search.evaluate(element => element.contains(document.activeElement))).toBe(true);
  }
  await page.keyboard.press('Escape');
  await expect(search).toBeHidden();
  await expect(page.getByTestId('quick-search-btn')).toBeFocused();
  await page.screenshot({ path: testInfo.outputPath('command-bridge-reader.png') });
});

test('positions Notes, Review, View, and the normal reader below the measured header', async ({ page }, testInfo) => {
  test.setTimeout(90000);
  await page.setViewportSize({ width: 844, height: 390 });
  await page.goto('/#note=daily%2F2026-10-16.md');
  await expect(page.getByTestId('cosmic-loading-overlay')).toBeHidden({ timeout: 30000 });
  await expect(page.getByTestId('floating-note-hud')).toBeVisible({ timeout: 30000 });
  await page.screenshot({ path: testInfo.outputPath('command-bridge-reader-placement.png') });
  const header = page.getByTestId('command-bridge');
  const assertBelowHeader = async (selector: string) => {
    const headerBottom = (await header.boundingBox())!.y + (await header.boundingBox())!.height;
    const panel = page.locator(selector);
    const box = await panel.boundingBox();
    expect(box).not.toBeNull();
    expect(Math.abs(box!.y - headerBottom - 8)).toBeLessThanOrEqual(1);
    expect(box!.y + box!.height).toBeLessThanOrEqual(390);
  };
  await assertBelowHeader('[data-testid="floating-note-hud"]');

  await page.getByRole('button', { name: 'Notes', exact: true }).click();
  const explorer = page.getByRole('complementary', { name: 'Note explorer' });
  await expect(explorer).toBeVisible();
  await assertBelowHeader('[aria-label="Note explorer"]');
  await page.getByRole('button', { name: 'Close note explorer' }).click();

  await page.getByRole('button', { name: 'More actions' }).click();
  await page.getByRole('button', { name: 'Review', exact: true }).click();
  const review = page.getByRole('complementary', { name: 'Knowledge review' });
  await expect(review).toBeVisible();
  await assertBelowHeader('[aria-label="Knowledge review"]');
  await page.keyboard.press('Escape');

  await page.getByRole('button', { name: 'View settings' }).click();
  const view = page.getByTestId('controls-popover');
  await expect(view).toBeVisible();
  await assertBelowHeader('[data-testid="controls-popover"]');
  await page.screenshot({ path: testInfo.outputPath('command-bridge-view.png') });
  await expect(view.getByRole('radio', { name: 'Auto' })).toBeVisible();
});
