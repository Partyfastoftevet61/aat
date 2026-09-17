// The launch cards: the announcement card, the "three files" explainer, and one
// card per project repository. Rendered from demos/cards/*.html with the
// Playwright pinned in demos/package.json.
//
//   node demos/cards.mjs [out-dir] [--only <name>]
//
// Unlike demos/run.sh this needs no sandbox, no VHS, and no free ports: it only
// opens local file:// pages. It does need the JetBrains Mono font installed.
// Default out-dir is demos/out/cards, which is gitignored; three-files.png is
// the one card the docs commit, so run.sh copies it into docs/user/assets.
//
// Every number on a card comes from a recorded run: the shop batch numbers are
// the ones demos/run.sh asserts, and each project's line is that project's own
// full batch (result.totalRuns, result.totalDurationMs in its batch.json).

import { chromium } from 'playwright';
import { fileURLToPath } from 'node:url';
import fs from 'node:fs';
import path from 'node:path';

const here = path.dirname(fileURLToPath(import.meta.url));

const args = process.argv.slice(2);
const onlyAt = args.indexOf('--only');
const only = onlyAt === -1 ? null : args[onlyAt + 1];
const outDir = (onlyAt === 0 ? null : args[0]) || path.join(here, 'out', 'cards');

// The shop batch, as demos/run.sh asserts it and social-preview.html shows it.
const batchTerminal = [
  [{ t: 'aat: batch run — 7 plans x 9 permutations = 63 total runs (parallel=4)', c: 'muted' }],
  [{ t: 'aat: dedup — 36 duplicate permutations detected', c: 'muted' }],
  [{ t: 'Batch: ' }, { t: '27/63 PASSED', c: 'passed' }, { t: ', 36 SKIPPED (11.4s)' }],
];

// A project's own batch summary, printed the way `aat run batch` prints it.
const projectBatch = (passed, total, seconds) => [
  [{ t: `$ aat run batch`, c: 'muted' }],
  [{ t: 'Batch: ' }, { t: `${passed}/${total} PASSED`, c: 'passed' }, { t: ` (${seconds}s)` }],
];

const launch = {
  template: 'card.html',
  data: {
    wordmark: 'aat',
    kicker: 'Adaptive API Toolkit',
    badge: 'v0.2.0',
    tagline:
      'Model your API as a graph once. Get long-chain integration tests, layer\u00a0\u00d7\u00a0environment matrices, CI-ready runs, and an MCP server for AI\u00a0coding\u00a0tools \u2014 all from the same YAML.',
    terminal: batchTerminal,
    url: 'github.com/gburgyan/aat',
  },
};

const project = (name, tagline, terminal) => ({
  template: 'card.html',
  data: {
    variant: 'project',
    wordmark: `aat-${name}`,
    kicker: 'an AAT project',
    tagline,
    terminal,
    url: `github.com/gburgyan/aat-${name}`,
  },
});

const cards = [
  { name: 'launch-card', size: { width: 1200, height: 630 }, ...launch },
  { name: 'launch-card-16x9', size: { width: 1600, height: 900 }, ...launch },
  {
    name: 'three-files',
    template: 'three-files.html',
    size: { width: 1200, height: 675 },
    scale: 2,
    data: {},
  },
  {
    name: 'card-duffel',
    size: { width: 1280, height: 640 },
    ...project(
      'duffel',
      "The Duffel flights API in test mode: 66 operations run by 47 plans against the live test API, with 14 layers crossed into matrices — and no official OpenAPI spec to start from.",
      projectBatch(47, 47, '194.9'),
    ),
  },
  {
    name: 'card-stripe',
    size: { width: 1280, height: 640 },
    ...project(
      'stripe',
      "Stripe's API in test mode: 82 operations run by 53 plans against the live test API, with every request and response checked against Stripe's own 205,445-line spec as it goes.",
      projectBatch(53, 53, '317.3'),
    ),
  },
  {
    name: 'card-shippo',
    size: { width: 1280, height: 640 },
    ...project(
      'shippo',
      "Shippo's shipping API in test mode: 46 of 70 operations run by 28 plans against the live test API, with a lane × parcel matrix, six tracking fixtures, and real labels in the web UI.",
      projectBatch(28, 28, '148.6'),
    ),
  },
];

const selected = only ? cards.filter((c) => c.name === only) : cards;
if (selected.length === 0) {
  console.error(`cards: no card named ${only} (have: ${cards.map((c) => c.name).join(', ')})`);
  process.exit(2);
}

fs.mkdirSync(outDir, { recursive: true });

const browser = await chromium.launch();
try {
  for (const card of selected) {
    const context = await browser.newContext({
      viewport: card.size,
      deviceScaleFactor: card.scale || 1,
      colorScheme: 'dark',
      reducedMotion: 'reduce',
    });
    const page = await context.newPage();
    await page.addInitScript((data) => {
      window.__card = data;
    }, card.data);
    await page.goto('file://' + path.join(here, 'cards', card.template));
    await page.evaluate(() => document.fonts.ready);
    const file = path.join(outDir, `${card.name}.png`);
    await page.screenshot({ path: file });
    const kb = Math.round(fs.statSync(file).size / 1024);
    console.log(`  ${card.name}.png  ${card.size.width}x${card.size.height}${card.scale ? ` @${card.scale}x` : ''}  ${kb} KB`);
    await context.close();
  }
} finally {
  await browser.close();
}
