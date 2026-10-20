import { test, expect } from '@playwright/test';
import * as path from 'path';

test.use({ reducedMotion: 'reduce' });

test.describe('Search Bar Harmony & Note Text Selection', () => {
  const artifactDir = process.env.ARTIFACT_DIR || path.join(process.cwd(), 'test-results');

  test('Search bar is dark and borderless, and note text is selectable and copyable', async ({ page }) => {
    test.setTimeout(60000);

    await page.goto('/');
    await page.waitForSelector('[data-testid="cosmic-loading-overlay"]', { state: 'detached', timeout: 30000 }).catch(() => {});
    await page.waitForTimeout(1000);

    // 1. VERIFY SEARCH BAR HARMONY (NO WHITE BOX)
    const searchBtn = page.getByTestId('quick-search-btn');
    await searchBtn.click();
    await page.waitForSelector('[data-slot="command-input"]', { state: 'visible', timeout: 5000 });
    await page.waitForTimeout(400);

    const inputStyles = await page.evaluate(() => {
      const input = document.querySelector('[data-slot="command-input"]') as HTMLElement | null;
      const wrapper = document.querySelector('[data-slot="command-input-wrapper"]') as HTMLElement | null;
      if (!input || !wrapper) return null;

      const sInput = window.getComputedStyle(input);
      const sWrapper = window.getComputedStyle(wrapper);

      return {
        inputBorder: sInput.border,
        inputBorderWidth: sInput.borderWidth,
        inputBg: sInput.backgroundColor,
        wrapperBorderTop: sWrapper.borderTopWidth,
        wrapperBorderLeft: sWrapper.borderLeftWidth,
        wrapperBorderRight: sWrapper.borderRightWidth,
        wrapperBorderBottom: `${sWrapper.borderBottomWidth} ${sWrapper.borderBottomStyle} ${sWrapper.borderBottomColor}`,
        // Must NOT have a full-box white border (e.g. 1px solid rgb(229, 231, 235))
        hasFullBoxBorder: sWrapper.borderLeftWidth !== '0px' && sWrapper.borderRightWidth !== '0px',
      };
    });

    console.log('Search Bar Styles:', inputStyles);
    expect(inputStyles).not.toBeNull();
    // Input itself has zero border
    expect(inputStyles!.inputBorderWidth).toBe('0px');
    // Wrapper does NOT have a full bounding rectangle (no left/right borders)
    expect(inputStyles!.hasFullBoxBorder).toBe(false);

    // Take screenshot of search modal
    await page.screenshot({
      path: path.join(artifactDir, 'search_modal_harmonious.png'),
    });

    // 2. SEARCH & OPEN NOTE TO VERIFY TEXT SELECTION
    await page.fill('[data-slot="command-input"]', 'cache');
    await page.waitForTimeout(800);

    const firstResult = page.locator('[role="option"]').first();
    await expect(firstResult).toBeVisible({ timeout: 5000 });
    await firstResult.click();
    await page.waitForTimeout(1200);

    // Verify note drawer is visible
    const noteDrawer = page.locator('[data-testid="floating-note-hud"]');
    await expect(noteDrawer).toBeVisible({ timeout: 5000 });

    // 3. VERIFY USER-SELECT IS 'text' (NOT 'none')
    const selectionCheck = await page.evaluate(() => {
      const hud = document.querySelector('[data-testid="floating-note-hud"]');
      const prose = hud?.querySelector('.prose');
      const paragraph = prose?.querySelector('p');
      const title = hud?.querySelector('h2');

      if (!hud || !prose || !paragraph) return null;

      const sHud = window.getComputedStyle(hud);
      const sProse = window.getComputedStyle(prose);
      const sParagraph = window.getComputedStyle(paragraph);
      const sTitle = title ? window.getComputedStyle(title) : null;

      // Programmatically select paragraph text
      const range = document.createRange();
      range.selectNodeContents(paragraph);
      const sel = window.getSelection();
      sel?.removeAllRanges();
      sel?.addRange(range);

      return {
        hudUserSelect: sHud.userSelect,
        proseUserSelect: sProse.userSelect,
        paragraphUserSelect: sParagraph.userSelect,
        titleUserSelect: sTitle?.userSelect,
        selectedString: sel?.toString() || '',
      };
    });

    console.log('Selection Check Result:', selectionCheck);
    expect(selectionCheck).not.toBeNull();
    // In Chromium, userSelect returns 'text' when enabled
    expect(selectionCheck!.paragraphUserSelect).toBe('text');
    expect(selectionCheck!.proseUserSelect).toBe('text');
    expect(selectionCheck!.hudUserSelect).toBe('text');
    // Selected string must NOT be empty
    expect(selectionCheck!.selectedString.length).toBeGreaterThan(5);

    // Save screenshot of note with selected text highlight
    await page.screenshot({
      path: path.join(artifactDir, 'note_text_selection.png'),
    });
  });
});
