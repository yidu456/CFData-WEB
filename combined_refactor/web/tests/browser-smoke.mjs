import assert from 'node:assert/strict';
import { mkdir } from 'node:fs/promises';
import { createRequire } from 'node:module';
import path from 'node:path';

// Use an already installed Playwright; no production dependency or build step is needed.
const { chromium } = createRequire(import.meta.url)(process.env.PLAYWRIGHT_MODULE || 'playwright');
const baseURL = process.argv[2] || 'http://127.0.0.1:13336';
const output = path.resolve(process.argv[3] || 'build/workbench-smoke');
await mkdir(output, { recursive: true });
const browser = await chromium.launch({ headless: true, channel: process.env.BROWSER_CHANNEL || 'msedge' });

try {
  const context = await browser.newContext({ viewport: { width: 1366, height: 950 }, colorScheme: 'light' });
  const page = await context.newPage();
  const errors = [];
  const requests = [];
  page.on('pageerror', error => errors.push(error.message));
  page.on('request', request => requests.push(request.url()));
  await page.goto(baseURL + '/workbench');
  await page.waitForFunction(() => document.documentElement.dataset.theme === 'light');
  await page.waitForFunction(() => document.querySelector('[data-connection]').textContent === '已连接服务器' && !document.querySelector('[data-task-action="refresh"]').disabled);
  assert.match(await page.locator('[data-connection]').innerText(), /已连接服务器/);
  assert.equal(await page.locator('[data-panel]:visible').count(), 1);
  await page.locator('[data-mode="nonstandard"]').click();
  assert.equal(await page.locator('[data-mode-panel="nonstandard"]').isVisible(), true);
  assert.equal(await page.locator('[data-mode="nonstandard"]').getAttribute('aria-pressed'), 'true');
  await page.locator('[data-mode="official"]').click();
  for (const section of ['tasks', 'targets', 'settings', 'workbench']) {
    await page.locator(`[data-section="${section}"]`).click();
    assert.equal(await page.locator(`[data-panel="${section}"]`).isVisible(), true);
    assert.equal(await page.locator('[data-panel]:visible').count(), 1);
    assert.equal(await page.locator(`[data-section="${section}"]`).getAttribute('aria-current'), 'page');
    assert.equal(await page.locator(`[data-panel="${section}"] h1`).evaluate(node => node === document.activeElement), true);
  }
  await page.locator('#theme-preference').selectOption('dark');
  assert.equal(await page.evaluate(() => localStorage.getItem('cfdata-theme')), 'dark');
  assert.equal(await page.locator('html').getAttribute('data-theme'), 'dark');
  await page.screenshot({ path: path.join(output, 'dark.png'), fullPage: true });
  await page.reload();
  assert.equal(await page.locator('#theme-preference').inputValue(), 'dark');
  await page.locator('#theme-preference').selectOption('light');
  await page.waitForFunction(() => getComputedStyle(document.querySelector('.content')).opacity === '1');
  await page.screenshot({ path: path.join(output, 'desktop.png'), fullPage: true });

  for (const width of [320, 375, 768, 1366]) {
    await page.setViewportSize({ width, height: 950 });
    for (const section of ['workbench', 'tasks', 'targets', 'settings']) {
      await page.locator(`[data-section="${section}"]`).click();
      const metrics = await page.evaluate(() => ({ width: document.documentElement.clientWidth, scroll: document.documentElement.scrollWidth }));
      assert(metrics.scroll <= metrics.width, `${width}px ${section} overflows: ${JSON.stringify(metrics)}`);
      for (const control of await page.locator('button:visible, select:visible, a.button:visible').all()) {
        const box = await control.boundingBox();
        assert(box.height >= 44 && box.x >= 0 && box.x + box.width <= width + 1, `unreachable control at ${width}px`);
      }
    }
    await page.locator('[data-section="workbench"]').click();
    if (width === 375) await page.screenshot({ path: path.join(output, 'mobile.png'), fullPage: true });
  }
  await page.locator('#theme-preference').selectOption('auto');
  await page.emulateMedia({ colorScheme: 'dark', reducedMotion: 'reduce' });
  await page.waitForFunction(() => document.documentElement.dataset.theme === 'dark');
  await page.emulateMedia({ colorScheme: 'light' });
  await page.waitForFunction(() => document.documentElement.dataset.theme === 'light');

  // The legacy page shares this key; BFCache must not restore a stale selector over it.
  await page.evaluate(() => {
    localStorage.setItem('cfdata-theme', 'dark');
    window.dispatchEvent(new PageTransitionEvent('pageshow', { persisted: true }));
  });
  assert.equal(await page.locator('#theme-preference').inputValue(), 'dark');
  assert.equal(await page.evaluate(() => localStorage.getItem('cfdata-theme')), 'dark');

  const legacy = await page.request.get(baseURL + '/');
  assert.equal(legacy.status(), 200);
  const legacyHTML = await legacy.text();
  assert.match(legacyHTML, /case 'scan_result'/);
  assert.match(legacyHTML, /function applyTheme\(/);
  assert.doesNotMatch(legacyHTML, /\/assets\/app.js/);
  assert.deepEqual(errors, []);
  assert(requests.every(url => new URL(url).origin === new URL(baseURL).origin), 'external runtime resource requested');
  await context.close();

  const blocked = await browser.newContext({ viewport: { width: 320, height: 950 } });
  await blocked.addInitScript(() => {
    Object.defineProperty(window, 'localStorage', { get() { throw new Error('blocked storage'); } });
    window.CFDataAndroid = {
      isDarkMode() { return true; },
      setDarkModeStatusBar(value) { window.nativeDark = value; },
    };
  });
  const blockedPage = await blocked.newPage();
  blockedPage.on('pageerror', error => errors.push(error.message));
  await blockedPage.goto(baseURL + '/workbench');
  await blockedPage.waitForFunction(() => document.documentElement.dataset.theme === 'dark');
  assert.equal(await blockedPage.evaluate(() => window.nativeDark), true);
  await blockedPage.locator('#theme-preference').selectOption('light');
  assert.equal(await blockedPage.evaluate(() => window.nativeDark), false);
  await blockedPage.evaluate(() => window.dispatchEvent(new PageTransitionEvent('pageshow', { persisted: true })));
  assert.equal(await blockedPage.locator('#theme-preference').inputValue(), 'light');
  await blockedPage.locator('[data-section="targets"]').focus();
  await blockedPage.keyboard.press('Enter');
  assert.equal(await blockedPage.locator('[data-panel="targets"]').isVisible(), true);
  assert.deepEqual(errors, []);
  await blocked.close();
  console.log('PASS: real embedded shell, modes/navigation, keyboard/focus, themes/BFCache/Android/blocked storage, 320/375/768/1366px, legacy root, local-only resources.');
} finally {
  await browser.close();
}
