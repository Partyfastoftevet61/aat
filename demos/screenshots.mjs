// Screenshots of the web UI for the docs, and the repository's social preview.
// Run by demos/run.sh against an `aat web` serving the run and batch that the
// tapes recorded:
//
//   node demos/screenshots.mjs <base-url> <run-id> <batch-id> <out-dir>
//
// Writes ui-run-gantt.png, ui-step-request-curl.png, ui-batch-matrix.png, and
// social-preview.png to <out-dir>.

import { chromium } from 'playwright';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const [baseURL, runId, batchId, outDir] = process.argv.slice(2);
if (!baseURL || !runId || !batchId || !outDir) {
  console.error('usage: node demos/screenshots.mjs <base-url> <run-id> <batch-id> <out-dir>');
  process.exit(2);
}
const here = path.dirname(fileURLToPath(import.meta.url));

async function getJSON(route) {
  const res = await fetch(new URL(route, baseURL));
  if (!res.ok) throw new Error(`GET ${route}: ${res.status}`);
  return res.json();
}

// newPage opens a dark, high-density page whose clock is pinned shortly after
// `when`, so relative times ("just now") do not depend on how long the
// recording took.
async function newPage(browser, when, viewport) {
  const context = await browser.newContext({
    viewport,
    deviceScaleFactor: 2,
    colorScheme: 'dark',
    reducedMotion: 'reduce',
    locale: 'en-US',
    timezoneId: 'UTC',
  });
  // The batch page remembers its layout per browser; show the By Test matrix.
  await context.addInitScript(() => localStorage.setItem('aat:batchViewMode', 'tests'));
  const page = await context.newPage();
  await page.clock.setFixedTime(new Date(new Date(when).getTime() + 10_000));
  return page;
}

async function shoot(page, route, readySelectors, file) {
  await page.goto(new URL(route, baseURL).href);
  for (const selector of readySelectors) {
    await page.locator(selector).first().waitFor({ state: 'visible' });
  }
  await page.evaluate(() => document.fonts.ready);
  await page.screenshot({ path: path.join(outDir, file) });
  console.log(`  ${file}`);
}

const browser = await chromium.launch();
try {
  const run = await getJSON(`/api/runs/${runId}`);
  const batch = await getJSON(`/api/batches/${batchId}`);

  // The run timeline, tall enough to show both retried steps and the timed
  // payment and shipment steps.
  let page = await newPage(browser, run.timestamp, { width: 1440, height: 1320 });
  await shoot(page, `/runs/${runId}`, ['.step-entry', '.step-retry-badge', '.step-duration-bar'], 'ui-run-gantt.png');

  // The checkout step's request, with its Copy as cURL button.
  page = await newPage(browser, run.timestamp, { width: 1440, height: 900 });
  await shoot(page, `/runs/${runId}/steps/checkout`, ['.curl-copy-btn'], 'ui-step-request-curl.png');

  // The batch as a test-by-permutation matrix with its per-group filters.
  page = await newPage(browser, batch.timestamp, { width: 1440, height: 900 });
  await shoot(page, `/batches/${batchId}`, ['table.test-matrix', '.dimension-filters'], 'ui-batch-matrix.png');

  // The social preview GitHub shows for links to the repository: 1280x640.
  const context = await browser.newContext({ viewport: { width: 1280, height: 640 }, deviceScaleFactor: 1 });
  page = await context.newPage();
  await page.goto('file://' + path.join(here, 'social-preview.html'));
  await page.evaluate(() => document.fonts.ready);
  await page.screenshot({ path: path.join(outDir, 'social-preview.png') });
  console.log('  social-preview.png');
} finally {
  await browser.close();
}
