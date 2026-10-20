import { expect, test } from '@playwright/test';

// Exercise document workflows without decorative GPU animation; camera invariants cover the default mode.
test.use({ reducedMotion: 'reduce' });

test('captures a note in the temporary vault and finds it after refresh', async ({ page, request, baseURL }) => {
  test.setTimeout(90000);
  const title = 'A captured research idea';
  const body = 'A distinct note about durable local knowledge capture.';
  const sourceUrl = 'https://example.com/reference';

  await page.goto('/');
  await page.getByTestId('cosmic-loading-overlay').waitFor({ state: 'detached', timeout: 30000 });
  await page.getByTestId('capture-note-btn').click();
  const dialog = page.getByRole('dialog', { name: 'Capture a note' });
  await expect(dialog).toBeVisible();
  await dialog.getByRole('textbox', { name: 'Title' }).fill(title);
  await dialog.getByRole('textbox', { name: 'Markdown content' }).fill(body);
  await dialog.getByRole('textbox', { name: 'Source URL (optional)' }).fill(sourceUrl);

  const responsePromise = page.waitForResponse(response => response.url().endsWith('/graph.v1.GraphService/CaptureNote'));
  await dialog.getByRole('button', { name: 'Save to Inbox' }).click();
  const captureResponse = await responsePromise;
  expect(captureResponse.ok()).toBe(true);
  const captured = await captureResponse.json() as { relPath: string; warning?: string };
  expect(captured.relPath).toMatch(/^inbox\/[^/]+\.md$/);
  expect(captured.warning ?? '').toBe('');

  const noteResponse = await request.post('http://127.0.0.1:8088/graph.v1.GraphService/GetNote', {
    headers: { Origin: baseURL! },
    data: { relPath: captured.relPath },
  });
  expect(noteResponse.ok()).toBe(true);
  const note = await noteResponse.json() as { contentMarkdown: string };
  expect(note.contentMarkdown).toContain(`title: ${title}`);
  expect(note.contentMarkdown).toContain(`source_url: ${sourceUrl}`);
  expect(note.contentMarkdown).toContain(`# ${title}\n\n${body}`);

  await expect(page.getByTestId('floating-note-hud')).toContainText(title, { timeout: 30000 });
  await page.waitForFunction(path => {
    const sigma = window.__sigmaInstance;
    return sigma?.getGraph().hasNode(path) && window.__sigmaContext?.selectedNodeId === path;
  }, captured.relPath, { timeout: 30000 });
  await expect(page).toHaveURL(new RegExp(`#note=${encodeURIComponent(captured.relPath)}`));
  let centeredSince: number | null = null;
  await expect.poll(async () => {
    const offset = await page.evaluate(path => {
      const sigma = window.__sigmaInstance!;
      const point = sigma.graphToViewport(sigma.getGraph().getNodeAttributes(path));
      const dims = sigma.getDimensions();
      return Math.max(Math.abs(point.x - (dims.width - 440) / 2), Math.abs(point.y - dims.height / 2));
    }, captured.relPath);
    if (offset > 25) {
      centeredSince = null;
      return offset;
    }
    centeredSince ??= Date.now();
    return Date.now() - centeredSince >= 1800 ? offset : 26;
  }, { timeout: 15000 }).toBeLessThanOrEqual(25);

  await page.reload();
  await page.getByTestId('cosmic-loading-overlay').waitFor({ state: 'detached', timeout: 30000 });
  await page.getByTestId('quick-search-btn').click();
  await page.locator('[data-slot="command-input"]').fill('captured');
  await expect(page.getByRole('option').filter({ hasText: title })).toBeVisible();
});
