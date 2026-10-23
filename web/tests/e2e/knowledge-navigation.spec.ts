import type {} from './browser-types';
import { expect, test } from '@playwright/test';

// Exercise document workflows without decorative GPU animation; camera invariants cover the default mode.
test.use({ reducedMotion: 'reduce' });

test('search, linked-note history, deep link reload and drawer framing', async ({ page }, testInfo) => {
  test.setTimeout(120000);
  await page.goto('/');
  await expect(page.getByTestId('cosmic-loading-overlay')).toBeHidden({ timeout: 30000 });
  await page.screenshot({ path: testInfo.outputPath('overview.png') });

  await page.getByTestId('quick-search-btn').click();
  await page.locator('[data-slot="command-input"]').fill('architecture example 0');
  const result = page.locator('[data-slot="command-item"]').filter({ hasText: 'architecture example 0' }).first();
  await expect(result).toBeVisible({ timeout: 10000 });
  await result.click();
  await expect(page.getByTestId('floating-note-hud')).toBeVisible({ timeout: 30000 });
  await expect(page.getByTestId('note-markdown')).toContainText('Cache and system design notes.');
  await expect(page).toHaveURL(/#note=notes%2Farchitecture%2Fnote-0\.md/);
  const historyAfterA = await page.evaluate(() => ({ index: history.state?.index, camera: history.state?.snapshot?.camera }));
  expect(historyAfterA.index).toBe(1);
  expect(Object.values(historyAfterA.camera as Record<string, number>).every(Number.isFinite)).toBe(true);
  await page.waitForTimeout(1000);
  const firstCamera = await page.evaluate(() => window.__sigmaInstance!.getCamera().getState());
  await page.screenshot({ path: testInfo.outputPath('reader.png') });

  await page.getByTestId('internal-note-link').filter({ hasText: 'Next example' }).click();
  await expect(page).toHaveURL(/#note=notes%2Farchitecture%2Fnote-1\.md/);
  const historyAfterB = await page.evaluate(() => ({ index: history.state?.index, camera: history.state?.snapshot?.camera }));
  expect(historyAfterB.index).toBe(2);
  expect(Object.values(historyAfterB.camera as Record<string, number>).every(Number.isFinite)).toBe(true);
  await expect(page.getByTestId('floating-note-hud')).toContainText('architecture example 1');
  await page.getByRole('button', { name: 'Back', exact: true }).click();
  await expect(page).toHaveURL(/#note=notes%2Farchitecture%2Fnote-0\.md/);
  await expect(page.getByTestId('floating-note-hud')).toContainText('architecture example 0');
  await page.waitForTimeout(1050);
  const restoredCamera = await page.evaluate(() => window.__sigmaInstance!.getCamera().getState());
  expect(Math.abs(restoredCamera.x - firstCamera.x)).toBeLessThan(0.03);
  expect(Math.abs(restoredCamera.y - firstCamera.y)).toBeLessThan(0.03);
  expect(Math.abs(restoredCamera.ratio - firstCamera.ratio)).toBeLessThan(0.03);

  await page.reload();
  await expect(page.getByTestId('cosmic-loading-overlay')).toBeHidden({ timeout: 30000 });
  await expect(page.getByTestId('floating-note-hud')).toContainText('architecture example 0');
  const resize = page.getByRole('separator', { name: 'Resize note panel' });
  await resize.focus();
  await resize.press('ArrowRight');
  await resize.press('ArrowRight');
  await expect(resize).toHaveAttribute('aria-valuenow', '472');
  await page.waitForTimeout(450);
  const center = await page.evaluate(() => {
    const sigma = window.__sigmaInstance!;
    const attrs = sigma.getGraph().getNodeAttributes('notes/architecture/note-0.md');
    const point = sigma.graphToViewport(attrs);
    const dims = sigma.getDimensions();
    return { actual: point.x, expected: (dims.width - 472) / 2 };
  });
  expect(Math.abs(center.actual - center.expected)).toBeLessThanOrEqual(25);

  await page.setViewportSize({ width: 390, height: 844 });
  await page.waitForTimeout(500);
  await expect(page.getByTestId('close-note-drawer')).toBeVisible();
  const mobileReader = await page.getByTestId('floating-note-hud').boundingBox();
  expect(mobileReader).not.toBeNull();
  expect(mobileReader!.x).toBeGreaterThanOrEqual(0);
  expect(mobileReader!.x + mobileReader!.width).toBeLessThanOrEqual(391);
  await page.screenshot({ path: testInfo.outputPath('reader-mobile.png') });
  await page.getByTestId('close-note-drawer').click();
  const toolbar = page.getByRole('navigation', { name: 'Knowledge navigation' });
  await expect(toolbar.getByRole('button', { name: 'Notes' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Capture note' })).toBeVisible();
  const mobileNavigation = await toolbar.boundingBox();
  expect(mobileNavigation).not.toBeNull();
  expect(mobileNavigation!.x + mobileNavigation!.width).toBeLessThanOrEqual(391);
  await page.screenshot({ path: testInfo.outputPath('overview-mobile.png') });
});

test('review panel shows structural evidence and keeps local decisions', async ({ page }) => {
  test.setTimeout(90000);
  await page.goto('/');
  await expect(page.getByTestId('cosmic-loading-overlay')).toBeHidden({ timeout: 30000 });
  await page.getByRole('navigation', { name: 'Knowledge navigation' }).getByRole('button', { name: 'Review' }).click();
  const review = page.getByRole('complementary', { name: 'Knowledge review' });
  await expect(review).toBeVisible({ timeout: 20000 });
  await expect(review.getByRole('tabpanel', { name: 'Orphan notes' })).toBeVisible();
  await review.getByRole('tab', { name: 'Unindexed targets', exact: false }).click();
  await expect(review.getByRole('tabpanel', { name: 'Unindexed targets' })).toContainText('not present in the current index');
  await review.getByRole('tab', { name: 'Suggested connections' }).click();
  await expect(review.getByRole('tabpanel', { name: 'Suggested connections' })).toContainText('shared graph neighbors as evidence');
  await review.getByLabel('Focus note', { exact: true }).selectOption('notes/architecture/note-0.md');
  const save = review.getByRole('button', { name: /^Save suggestion / }).first();
  await expect(save).toBeVisible();
  await save.click();
  await expect(review.getByLabel('Reviewed suggestions')).toContainText('Saved for review');

  await page.reload();
  await expect(page.getByTestId('cosmic-loading-overlay')).toBeHidden({ timeout: 30000 });
  await page.getByRole('navigation', { name: 'Knowledge navigation' }).getByRole('button', { name: 'Review' }).click();
  const restoredReview = page.getByRole('complementary', { name: 'Knowledge review' });
  await restoredReview.getByRole('tab', { name: 'Suggested connections' }).click();
  await restoredReview.getByLabel('Focus note', { exact: true }).selectOption('notes/architecture/note-0.md');
  await expect(restoredReview.getByLabel('Reviewed suggestions')).toContainText('Saved for review');
});

test('a fresh note fragment replaces its browser entry without duplicating history', async ({ page }) => {
  test.setTimeout(90000);
  await page.goto('about:blank');
  await page.goto('/#note=notes%2Farchitecture%2Fnote-0.md');
  const entries = await page.evaluate(() => window.history.length);
  await expect(page.getByTestId('cosmic-loading-overlay')).toBeHidden({ timeout: 30000 });
  await expect(page.getByTestId('floating-note-hud')).toContainText('architecture example 0', { timeout: 30000 });
  expect(await page.evaluate(() => window.history.length)).toBe(entries);
  await page.goBack();
  await expect(page).toHaveURL('about:blank');
});

test('resizing a reloaded reader takes precedence over deferred camera restoration', async ({ page }) => {
  test.setTimeout(90000);
  // Make deferred history restoration overlap an explicit reader resize.
  await page.addInitScript(() => {
    const schedule = window.setTimeout.bind(window);
    window.setTimeout = ((callback: TimerHandler, delay?: number, ...args: unknown[]) =>
      schedule(callback, delay === 900 ? 3000 : delay, ...args)) as typeof window.setTimeout;
  });
  await page.goto('/#note=notes%2Farchitecture%2Fnote-0.md');
  await expect(page.getByTestId('cosmic-loading-overlay')).toBeHidden({ timeout: 30000 });
  await expect(page.getByTestId('floating-note-hud')).toContainText('architecture example 0');
  await expect.poll(() => page.evaluate(() => window.__sigmaInstance!.getCamera().isAnimated())).toBe(false);
  const savedX = await page.evaluate(() => {
    const camera = window.__sigmaInstance!.getCamera();
    const x = camera.x + 0.05;
    camera.setState({ x });
    return x;
  });
  await expect.poll(() => page.evaluate(() => history.state?.snapshot?.camera?.x)).toBe(savedX);

  await page.reload();
  await expect(page.getByTestId('cosmic-loading-overlay')).toBeHidden({ timeout: 30000 });
  const resize = page.getByRole('separator', { name: 'Resize note panel' });
  await resize.press('ArrowRight');
  await resize.press('ArrowRight');
  await expect(resize).toHaveAttribute('aria-valuenow', '472');
  // Observe the frame after the deferred callback would have run, not just the immediate resize.
  await page.waitForTimeout(3200);
  const offset = await page.evaluate(() => {
    const sigma = window.__sigmaInstance!;
    const attrs = sigma.getGraph().getNodeAttributes('notes/architecture/note-0.md');
    return Math.abs(sigma.graphToViewport(attrs).x - (sigma.getDimensions().width - 472) / 2);
  });
  expect(offset).toBeLessThanOrEqual(25);
});

test('event stream failures keep retries bounded while navigating notes', async ({ page }) => {
  test.setTimeout(90000);
  let subscriptions = 0;
  await page.route('**/graph.v1.GraphService/SubscribeUIEvents', async route => {
    subscriptions += 1;
    await route.fulfill({
      status: 503, contentType: 'application/json',
      body: JSON.stringify({ code: 'unavailable', message: 'Event stream temporarily unavailable' }),
    });
  });
  await page.goto('/');
  await expect(page.getByTestId('cosmic-loading-overlay')).toBeHidden({ timeout: 30000 });
  await page.getByTestId('quick-search-btn').click();
  await page.locator('[data-slot="command-input"]').fill('architecture example 0');
  await page.locator('[data-slot="command-item"]').filter({ hasText: 'architecture example 0' }).first().click();
  await expect(page.getByTestId('note-markdown')).toContainText('Cache and system design notes.');
  expect(subscriptions).toBeGreaterThanOrEqual(1);
  // Exponential backoff allows at most eight attempts within this test's lifetime.
  expect(subscriptions).toBeLessThanOrEqual(8);
});
