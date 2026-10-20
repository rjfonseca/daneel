import { expect, test } from '@playwright/test';

test.use({ reducedMotion: 'reduce', timezoneId: 'UTC' });

test('keeps input focus visible in forced colors', async ({ page }) => {
  test.setTimeout(90000);
  await page.emulateMedia({ forcedColors: 'active' });
  await page.goto('/');
  await expect(page.getByTestId('cosmic-loading-overlay')).toBeHidden({ timeout: 30000 });
  await page.getByRole('button', { name: 'Capture note', exact: true }).click();
  const title = page.getByRole('dialog', { name: 'Capture a note' }).getByRole('textbox', { name: 'Title', exact: true });
  await title.focus();
  await expect(title).toBeFocused();
  await expect(title).toHaveCSS('outline-style', 'solid');
  await expect(title).toHaveCSS('outline-width', '2px');
});

test('capture fields and actions remain reachable in a short viewport', async ({ page }, testInfo) => {
  test.setTimeout(90000);
  await page.setViewportSize({ width: 844, height: 390 });
  await page.goto('/');
  await expect(page.getByTestId('cosmic-loading-overlay')).toBeHidden({ timeout: 30000 });
  await page.getByRole('button', { name: 'Capture note', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: 'Capture a note' });
  await expect(dialog).toBeVisible();
  const bounds = await dialog.boundingBox();
  expect(bounds!.y).toBeGreaterThanOrEqual(16);
  expect(bounds!.y + bounds!.height).toBeLessThanOrEqual(374);
  await dialog.getByRole('textbox', { name: 'Title', exact: true }).fill('A short-screen draft');
  await dialog.getByRole('textbox', { name: 'Markdown content' }).fill('Draft content.');
  await dialog.getByRole('textbox', { name: 'Source URL (optional)' }).fill('https://example.com');
  const save = dialog.getByRole('button', { name: 'Save to Inbox' });
  await save.scrollIntoViewIfNeeded();
  await expect(save).toBeInViewport();
  await expect(save).toBeEnabled();
  await page.screenshot({ path: testInfo.outputPath('capture-landscape.png') });
  await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
  await expect(dialog).toBeHidden();
});

test('date selection opens its daily note and remains usable beside the reader', async ({ page }, testInfo) => {
  test.setTimeout(90000);
  await page.clock.setFixedTime(new Date('2026-10-16T12:00:00Z'));
  await page.goto('/');
  await expect(page.getByTestId('cosmic-loading-overlay')).toBeHidden({ timeout: 30000 });
  const picker = page.getByLabel('Choose a date to open a daily note', { exact: true });
  await expect(picker).toBeVisible();
  expect((await picker.boundingBox())!.width).toBeGreaterThanOrEqual(150);
  await picker.fill('2026-10-07');
  await expect(page).toHaveURL(/#note=daily%2F2026-10-07\.md/);
  await expect(page.getByTestId('floating-note-hud')).toContainText('Daily log for 2026-10-07', { timeout: 30000 });
  const hotbar = await page.getByTestId('bottom-temporal-hotbar').boundingBox();
  const reader = await page.getByTestId('floating-note-hud').boundingBox();
  expect(hotbar!.x + hotbar!.width).toBeLessThanOrEqual(reader!.x + 1);
  await page.getByTestId('temporal-today').click();
  await expect(page).toHaveURL(/#note=daily%2F2026-10-16\.md/);
  await picker.fill('2026-10-06');
  await expect(page.getByText(/No daily note found.*2026-10-06/)).toBeVisible();
  await expect(page).toHaveURL(/#note=daily%2F2026-10-16\.md/);
  await page.getByTestId('close-note-drawer').click();
  await page.screenshot({ path: testInfo.outputPath('hud-desktop.png') });
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(picker).toBeVisible();
  await expect.poll(async () => {
    const box = await picker.boundingBox();
    return !!box && box.x >= 0 && box.x + box.width <= 390;
  }).toBe(true);
  await page.screenshot({ path: testInfo.outputPath('hud-mobile.png') });
});

test('knowledge menus and search close before the underlying note', async ({ page }, testInfo) => {
  test.setTimeout(90000);
  await page.goto('/#note=notes%2Farchitecture%2Fnote-0.md');
  await expect(page.getByTestId('cosmic-loading-overlay')).toBeHidden({ timeout: 30000 });
  await expect(page.getByTestId('floating-note-hud')).toBeVisible({ timeout: 30000 });
  const navigation = page.getByRole('navigation', { name: 'Knowledge navigation' });
  await navigation.getByRole('button', { name: 'Recent', exact: true }).click();
  await expect(page.getByRole('region', { name: 'Recent notes' })).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('region', { name: 'Recent notes' })).toBeHidden();
  await expect(page.getByTestId('floating-note-hud')).toBeVisible();
  await navigation.getByRole('button', { name: 'Notes', exact: true }).click();
  await expect(page.getByRole('complementary', { name: 'Note explorer' })).toBeVisible();
  await navigation.getByRole('button', { name: 'Review', exact: true }).click();
  await expect(page.getByRole('complementary', { name: 'Note explorer' })).toBeHidden();
  await expect(page.getByRole('complementary', { name: 'Knowledge review' })).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('complementary', { name: 'Knowledge review' })).toBeHidden();
  await expect(page.getByTestId('floating-note-hud')).toBeVisible();
  await page.getByTestId('quick-search-btn').click();
  const search = page.getByRole('dialog', { name: 'Search notes', exact: true });
  await expect(search).toBeVisible({ timeout: 30000 });
  const input = search.getByRole('combobox');
  await expect(input).toBeFocused();
  const focusCycle = [
    ...['All', 'People', 'Architecture', 'Daily', 'Sources']
      .map(name => search.getByRole('button', { name, exact: true })),
    input,
  ];
  for (let index = 0; index < 12; index += 1) {
    await page.keyboard.press('Tab');
    // Focus guards wrap on an animation frame; assert the final focus target.
    await expect(focusCycle[index % focusCycle.length]!).toBeFocused();
  }
  await page.keyboard.press('Escape');
  await expect(search).toBeHidden();
  await expect(page.getByTestId('floating-note-hud')).toBeVisible();
  await expect(page.getByTestId('quick-search-btn')).toBeFocused();
  await navigation.getByRole('button', { name: 'Recent', exact: true }).click();
  await page.screenshot({ path: testInfo.outputPath('recent-hud.png') });
});
