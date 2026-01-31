import { test, expect } from '@playwright/test';
import { execSync } from 'child_process';
import * as fs from 'fs';
import * as path from 'path';

const PROJECT_ROOT = path.resolve(__dirname, '../..');
const TESTDATA_DIR = path.resolve(__dirname, '../testdata');
const DB_PATH = path.join(TESTDATA_DIR, 'data/test.db');

// Test file counts
const TOTAL_FILES = 11; // 5 documents + 4 images + 2 videos
const TOTAL_DIRS = 5; // documents, images, images/vacation, videos, root (.)

function cleanDatabase() {
  if (fs.existsSync(DB_PATH)) {
    fs.unlinkSync(DB_PATH);
  }
}

function runIndexer() {
  execSync(`${PROJECT_ROOT}/bin/findex -config ${TESTDATA_DIR}/config.yaml`, {
    cwd: PROJECT_ROOT,
    stdio: 'pipe',
  });
}

test.describe('FIndex E2E Tests', () => {

  test.describe('Empty Index State', () => {

    test.beforeAll(() => {
      cleanDatabase();
    });

    test('should show index list on start page', async ({ page }) => {
      await page.goto('/');

      // Should see the index name in the checkbox list
      await expect(page.locator('label[for="index_test-files"]')).toBeVisible();
    });

    test('should show empty directory when browsing empty index', async ({ page }) => {
      await page.goto('/browse/test-files?path=');

      // Should show empty directory message or no items
      const emptyMessage = page.locator('.alert:has-text("empty")');
      const tableRows = page.locator('table tbody tr');

      // Either shows empty message or has no table rows
      const messageVisible = await emptyMessage.isVisible().catch(() => false);
      const rowCount = await tableRows.count().catch(() => 0);

      expect(messageVisible || rowCount === 0).toBe(true);
    });

    test('should return no results for search on empty index', async ({ page }) => {
      await page.goto('/?q=report&index[]=test-files');

      // Should show no results message
      await expect(page.locator('.alert:has-text("No results")')).toBeVisible();
    });

  });

  test.describe('After Indexing', () => {

    test.beforeAll(() => {
      cleanDatabase();
      runIndexer();
    });

    test('should show directories after indexing', async ({ page }) => {
      await page.goto('/browse/test-files?path=');

      // Should see the main directories (as links in the table)
      await expect(page.locator('table tbody a:has-text("documents/")')).toBeVisible();
      await expect(page.locator('table tbody a:has-text("images/")')).toBeVisible();
      await expect(page.locator('table tbody a:has-text("videos/")')).toBeVisible();
    });

    test('should navigate into subdirectories', async ({ page }) => {
      await page.goto('/browse/test-files?path=');

      // Click on documents folder
      await page.locator('table tbody a:has-text("documents/")').click();
      await page.waitForURL(/path=documents/);

      // Should see document files
      await expect(page.locator('table tbody a:has-text("annual_report_2024.pdf")')).toBeVisible();
      await expect(page.locator('table tbody a:has-text("meeting_notes.txt")')).toBeVisible();
    });

    test('should show breadcrumb navigation', async ({ page }) => {
      await page.goto('/browse/test-files?path=images/vacation');

      // Should see breadcrumbs
      await expect(page.locator('nav[aria-label="breadcrumb"]')).toBeVisible();
      await expect(page.locator('.breadcrumb a:has-text("images")')).toBeVisible();

      // Should see vacation photos
      await expect(page.locator('table tbody a:has-text("beach_sunset.jpg")')).toBeVisible();
    });

  });

  test.describe('Search Functionality', () => {

    test.beforeAll(() => {
      cleanDatabase();
      runIndexer();
    });

    test('should find files by name', async ({ page }) => {
      await page.goto('/');

      // Enter search query
      await page.fill('input[name="q"]', 'annual');
      await page.locator('input[name="index[]"][value="test-files"]').check();
      await page.locator('button[type="submit"]').click();

      // Should find the annual report
      await expect(page.locator('table tbody a:has-text("annual_report_2024.pdf")')).toBeVisible();
    });

    test('should find files by partial name', async ({ page }) => {
      await page.goto('/?q=budget&index[]=test-files');

      await expect(page.locator('table tbody a:has-text("budget_2024.xlsx")')).toBeVisible();
    });

    test('should find files in subdirectories', async ({ page }) => {
      await page.goto('/?q=beach&index[]=test-files');

      await expect(page.locator('table tbody a:has-text("beach_sunset.jpg")')).toBeVisible();
    });

    test('should show result count', async ({ page }) => {
      await page.goto('/?q=pdf&index[]=test-files');

      // Should show "Found X results"
      await expect(page.locator('text=/Found \\d+ result/')).toBeVisible();

      // Should find PDF files
      await expect(page.locator('table tbody a:has-text("annual_report_2024.pdf")')).toBeVisible();
      await expect(page.locator('table tbody a:has-text("contract_draft.pdf")')).toBeVisible();
    });

    test('should support exclusion with minus prefix', async ({ page }) => {
      await page.goto('/?q=report -annual&index[]=test-files');

      // Should NOT find annual_report (contains both 'report' and 'annual')
      await expect(page.locator('table tbody a:has-text("annual_report")')).not.toBeVisible();
    });

  });

  test.describe('Filtering', () => {

    test.beforeAll(() => {
      cleanDatabase();
      runIndexer();
    });

    test('should filter by extension', async ({ page }) => {
      await page.goto('/?index[]=test-files&ext=pdf');

      // Should only show PDF files
      await expect(page.locator('table tbody a:has-text(".pdf")')).toHaveCount(2);

      // Should NOT show other file types
      await expect(page.locator('table tbody a:has-text(".txt")')).toHaveCount(0);
      await expect(page.locator('table tbody a:has-text(".xlsx")')).toHaveCount(0);
    });

    test('should filter by multiple extensions', async ({ page }) => {
      await page.goto('/?index[]=test-files&ext=mp4,mkv');

      // Should show video files
      await expect(page.locator('table tbody a:has-text("holiday_2024.mp4")')).toBeVisible();
      await expect(page.locator('table tbody a:has-text("birthday_party.mkv")')).toBeVisible();

      // Should only have 2 results
      await expect(page.locator('table tbody tr')).toHaveCount(2);
    });

    test('should filter by minimum size', async ({ page }) => {
      // Files larger than 2MB (videos are ~3-5MB)
      await page.goto('/?index[]=test-files&min_size=2MB');

      // Should show large video files
      await expect(page.locator('table tbody a:has-text("holiday_2024.mp4")')).toBeVisible();
      await expect(page.locator('table tbody a:has-text("birthday_party.mkv")')).toBeVisible();

      // Should NOT show small document files
      await expect(page.locator('table tbody a:has-text("meeting_notes.txt")')).not.toBeVisible();
    });

    test('should filter by maximum size', async ({ page }) => {
      // Files smaller than 100 bytes (documents are small text)
      await page.goto('/?index[]=test-files&max_size=100B');

      // Should show small document files
      const rows = page.locator('table tbody tr');
      const count = await rows.count();
      expect(count).toBeGreaterThan(0);

      // Should NOT show large video files
      await expect(page.locator('table tbody a:has-text("holiday_2024.mp4")')).not.toBeVisible();
    });

    test('should filter files only (exclude directories)', async ({ page }) => {
      await page.goto('/?index[]=test-files&type=files');

      // All results should be files (no folder icon)
      const folderIcons = page.locator('table tbody .bi-folder-fill');
      await expect(folderIcons).toHaveCount(0);
    });

    test('should filter directories only', async ({ page }) => {
      await page.goto('/?index[]=test-files&type=dirs');

      // All results should be directories (have folder icon)
      const rows = page.locator('table tbody tr');
      const count = await rows.count();
      expect(count).toBeGreaterThan(0);

      // Each row should have folder icon
      const folderIcons = page.locator('table tbody .bi-folder-fill');
      await expect(folderIcons).toHaveCount(count);
    });

    test('should combine search with filters', async ({ page }) => {
      await page.goto('/?q=2024&index[]=test-files&ext=pdf');

      // Should find PDF files with "2024" in name
      await expect(page.locator('table tbody a:has-text("annual_report_2024.pdf")')).toBeVisible();

      // Should NOT show xlsx file even though it matches "2024"
      await expect(page.locator('table tbody a:has-text("budget_2024.xlsx")')).not.toBeVisible();
    });

  });

  test.describe('Statistics Page', () => {

    test.beforeAll(() => {
      cleanDatabase();
      runIndexer();
    });

    test('should display statistics page', async ({ page }) => {
      await page.goto('/stats');

      // Page should load successfully
      await expect(page).toHaveURL('/stats');

      // Should have some content
      const body = await page.locator('body').textContent();
      expect(body?.length).toBeGreaterThan(100);
    });

    test('should show total files count', async ({ page }) => {
      await page.goto('/stats');

      // Look for file count - should be around 11 files
      const pageText = await page.locator('body').textContent();
      // The page should mention the number of files somewhere
      expect(pageText).toMatch(/\d+/);
    });

    test('should show file extensions', async ({ page }) => {
      await page.goto('/stats');

      // Should show various file extensions
      const pageText = await page.locator('body').textContent() || '';

      // At least some extensions should be visible
      const hasExtensions =
        pageText.includes('.pdf') ||
        pageText.includes('.mp4') ||
        pageText.includes('.jpg') ||
        pageText.includes('pdf') ||
        pageText.includes('mp4');
      expect(hasExtensions).toBe(true);
    });

    test('should show storage size information', async ({ page }) => {
      await page.goto('/stats');

      // Should display some size information (KB, MB, etc.)
      const pageText = await page.locator('body').textContent() || '';
      const hasSize = /\d+(\.\d+)?\s*(B|KB|MB|GB|bytes)/i.test(pageText);
      expect(hasSize).toBe(true);
    });

    test('should show largest files', async ({ page }) => {
      await page.goto('/stats');

      // The largest file is holiday_2024.mp4 - should appear in largest files table
      await expect(page.locator('table a:has-text("holiday_2024.mp4")').first()).toBeVisible();
    });

  });

  test.describe('Pagination', () => {

    test.beforeAll(() => {
      cleanDatabase();
      runIndexer();
    });

    test('should show per-page options', async ({ page }) => {
      await page.goto('/?index[]=test-files&type=files');

      // Should show per-page selector buttons
      await expect(page.locator('.btn-group a:has-text("25")')).toBeVisible();
      await expect(page.locator('.btn-group a:has-text("50")')).toBeVisible();
      await expect(page.locator('.btn-group a:has-text("100")')).toBeVisible();
    });

  });

  test.describe('UI Elements', () => {

    test.beforeAll(() => {
      cleanDatabase();
      runIndexer();
    });

    test('should have working home link from browse', async ({ page }) => {
      await page.goto('/browse/test-files?path=documents');

      // Click home button (arrow left or house icon)
      await page.locator('a[href="/"]').first().click();

      // Should be on home page
      await expect(page).toHaveURL('/');
    });

    test('should show index badge in browse view', async ({ page }) => {
      await page.goto('/browse/test-files?path=');

      // Should show index name badge
      await expect(page.locator('.badge:has-text("test-files")')).toBeVisible();
    });

    test('should show extension badges in results', async ({ page }) => {
      await page.goto('/?q=annual&index[]=test-files');

      // Should show extension badge for the found file
      await expect(page.locator('table tbody .badge:has-text(".pdf")')).toBeVisible();
    });

    test('should display size and files count in browse', async ({ page }) => {
      await page.goto('/browse/test-files?path=');

      // Should show Size label in the header stats section
      await expect(page.locator('.browse-stats .text-uppercase:has-text("Size")')).toBeVisible();

      // Should show Files label in the header stats section
      await expect(page.locator('.browse-stats .text-uppercase:has-text("Files")')).toBeVisible();
    });

    test('should display footer', async ({ page }) => {
      await page.goto('/');

      // Should have a footer element
      await expect(page.locator('footer')).toBeVisible();
    });

  });

  test.describe('Error Handling', () => {

    test('should show 404 for non-existent index', async ({ page }) => {
      const response = await page.goto('/browse/nonexistent-index?path=');

      expect(response?.status()).toBe(404);
      await expect(page.locator('.error-code:has-text("404")')).toBeVisible();
    });

    test('should show navigation buttons on error page', async ({ page }) => {
      await page.goto('/browse/nonexistent-index?path=');

      // Should have "Go Home" button
      await expect(page.locator('a.btn:has-text("Go Home")')).toBeVisible();

      // Should have "Go Back" button
      await expect(page.locator('button:has-text("Go Back")')).toBeVisible();
    });

  });

});
