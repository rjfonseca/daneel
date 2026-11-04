import { test, expect } from '@playwright/test';

test.use({ reducedMotion: 'reduce' });

test('Enter clears the search query without opening the selected result', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByTestId('cosmic-loading-overlay')).toBeHidden({ timeout: 30000 });
  await page.getByTestId('quick-search-btn').click();
  const dialog = page.getByRole('dialog', { name: 'Search notes', exact: true });
  const input = dialog.getByRole('combobox');
  await input.fill('architecture');
  await expect(dialog.getByRole('option').first()).toBeVisible();
  await dialog.getByRole('button', { name: 'Clear search query' }).focus();
  await page.keyboard.press('Enter');
  await expect(input).toHaveValue('');
  await expect(dialog).toBeVisible();
  await expect(page.getByTestId('floating-note-hud')).toBeHidden();
});
