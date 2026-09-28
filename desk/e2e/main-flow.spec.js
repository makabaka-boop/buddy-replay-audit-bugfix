import { test, expect } from '@playwright/test';

// Main flow: allocate → free → reuse, driven through the demo batch that
// auto-runs on page load. Requires desk + allocator running (see README).
test('allocate → free → reuse main flow', async ({ page }) => {
  await page.goto('/');

  // Demo batch auto-replays: 10 events land in the log.
  await expect(page.getByTestId('event-log').locator('.erow:not(.ehead)')).toHaveCount(10);

  // Step through the whole batch from the initial state.
  await expect(page.getByTestId('position')).toHaveText('0 / 10');
  await expect(page.getByTestId('m-total-free')).toHaveText('256 B');

  // Allocate phase: four 64 B blocks fill the space.
  await page.getByTestId('step-last').click();
  await expect(page.getByTestId('position')).toHaveText('10 / 10');

  // The fragmented allocate (e7) is a valid OOM event, not an error.
  await page.getByTestId('ev-e7').click();
  await expect(page.getByTestId('ev-e7').locator('.badge')).toHaveText('oom');
  await expect(page.getByTestId('m-total-free')).toHaveText('128 B');
  await expect(page.getByTestId('m-max-free')).toHaveText('64 B');
  await expect(page.getByTestId('m-ext-frag')).toHaveText('64 B');
  await expect(page.getByTestId('describe')).toContainText('external fragmentation');

  // After the frees cascade-merge, the final allocate reuses address 0.
  await page.getByTestId('step-last').click();
  await expect(page.getByTestId('ev-e10').locator('.badge')).toHaveText('ok');
  await expect(page.getByTestId('bar-alloc-e2')).toBeVisible();
  await expect(page.getByTestId('tree-alloc-e2')).toBeVisible();
  await expect(page.getByTestId('m-total-free')).toHaveText('128 B');
  await expect(page.getByTestId('m-max-free')).toHaveText('128 B');
  await expect(page.getByTestId('m-ext-frag')).toHaveText('0 B');
  await expect(page.getByTestId('describe')).toContainText('[0, 128)');

  // Structural errors reject the whole batch: free an unknown id.
  await page.getByTestId('add-free').click();
  await page.locator('[data-testid="op-row"]').last().locator('input').fill('ghost');
  await page.getByTestId('run').click();
  await expect(page.getByTestId('error-banner')).toContainText('unknown allocation id');
});
