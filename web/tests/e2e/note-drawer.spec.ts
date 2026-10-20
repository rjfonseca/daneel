import { test, expect } from '@playwright/test';
import path from 'path';

test.use({ reducedMotion: 'reduce' });

test.describe('Note Drawer Reading Experience & Scroll', () => {
  test('opens a 440px reader, scrolls a long note, maximizes and restores it', async ({ page }) => {
    test.setTimeout(60000);
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

    await page.goto('/');
    await expect(page.getByTestId('cosmic-loading-overlay')).toBeHidden({ timeout: 30000 });

    // Search and click note via Quick Search HUD
    await page.locator('[data-testid="quick-search-btn"]').click();
    await page.waitForTimeout(400);
    await page.fill('input[placeholder*="Search notes"]', 'architecture example 0');
    await page.waitForTimeout(1000);

    const noteOption = page.getByRole('option').filter({ hasText: 'notes/architecture/note-0.md' });
    await expect(noteOption).toBeVisible({ timeout: 5000 });
    await noteOption.click();
    await page.waitForTimeout(1500);

    const sheetContent = page.locator('[data-testid="floating-note-hud"]');
    await expect(sheetContent).toBeVisible();

    // Check default width is approximately 440px
    const initialBox = await sheetContent.boundingBox();
    expect(initialBox).not.toBeNull();
    if (initialBox) {
      expect(initialBox.width).toBeGreaterThan(400);
      expect(initialBox.width).toBeLessThan(500);
    }

    // Check ScrollArea and Viewport
    const scrollInfo = await page.evaluate(() => {
      const viewport = document.querySelector('[data-slot="scroll-area-viewport"]');
      if (!viewport) return null;
      return {
        clientHeight: viewport.clientHeight,
        scrollHeight: viewport.scrollHeight,
        canScroll: viewport.scrollHeight > viewport.clientHeight,
      };
    });

    expect(scrollInfo).not.toBeNull();
    expect(scrollInfo?.canScroll).toBe(true);

    // Test scrolling
    await page.evaluate(() => {
      const viewport = document.querySelector('[data-slot="scroll-area-viewport"]');
      if (viewport) {
        viewport.scrollTop = 250;
      }
    });
    await page.waitForTimeout(300);

    const scrolledTop = await page.evaluate(() => {
      const viewport = document.querySelector('[data-slot="scroll-area-viewport"]');
      return viewport ? viewport.scrollTop : 0;
    });
    expect(scrolledTop).toBeGreaterThan(0);

    // Take screenshot of default reading drawer with scrollbar
    const artifactDir = process.env.ARTIFACT_DIR || path.join(process.cwd(), 'test-results');
    await page.screenshot({ path: path.join(artifactDir, 'note_drawer_default_with_scroll.png') });

    // Click Maximize button
    const maximizeBtn = page.locator('[data-testid="toggle-maximize-note"]');
    await expect(maximizeBtn).toBeVisible();
    await maximizeBtn.click();
    await page.waitForTimeout(800);

    // Verify width is now maximized (~88vw, > 1000px on 1280px screen)
    const maximizedBox = await sheetContent.boundingBox();
    expect(maximizedBox).not.toBeNull();
    if (maximizedBox && initialBox) {
      expect(maximizedBox.width).toBeGreaterThan(1000);
      expect(maximizedBox.width).toBeGreaterThan(initialBox.width * 2);
    }

    // Take screenshot of maximized reading drawer
    await page.screenshot({ path: path.join(artifactDir, 'note_drawer_maximized.png') });

    // Click restore button to return to default width
    await maximizeBtn.click();
    await page.waitForTimeout(800);

    const restoredBox = await sheetContent.boundingBox();
    expect(restoredBox).not.toBeNull();
    if (restoredBox) {
      expect(restoredBox.width).toBeLessThan(500);
    }

    // Close note drawer
    const closeBtn = page.locator('[data-testid="close-note-drawer"]');
    await closeBtn.click();
    await page.waitForTimeout(500);

    await expect(sheetContent).not.toBeVisible();
    expect(errors).toEqual([]);
  });
});
