import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const load = async file => import('data:text/javascript;base64,' + Buffer.from(await readFile(new URL(file, import.meta.url))).toString('base64'));

test('navigation only accepts known sections', async () => {
  const { normalizeSection } = await load('../navigation.js');
  for (const value of ['workbench', 'tasks', 'targets', 'settings']) {
    assert.equal(normalizeSection(value), value);
  }
  for (const value of [undefined, null, '', '../secrets', 'constructor', 'toString', 'TASKS']) {
    assert.equal(normalizeSection(value), 'workbench');
  }
});

test('theme reads compatible preferences and tolerates invalid or blocked storage', async () => {
  const { readThemePreference } = await load('../platform/theme.js');
  for (const value of ['auto', 'light', 'dark']) {
    assert.equal(readThemePreference({ getItem: key => {
      assert.equal(key, 'cfdata-theme');
      return value;
    } }), value);
  }
  for (const value of [null, '', 'invalid', 'constructor']) {
    assert.equal(readThemePreference({ getItem: () => value }), 'auto');
  }
  assert.equal(readThemePreference(undefined), 'auto');
  assert.equal(readThemePreference({ getItem: () => { throw new Error('blocked'); } }), 'auto');
});

test('blocked storage preserves the in-memory preference on page restoration', async () => {
  const { readThemePreference } = await load('../platform/theme.js');
  assert.equal(readThemePreference(undefined, 'light'), 'light');
  assert.equal(readThemePreference({ getItem: () => { throw new Error('blocked'); } }, 'dark'), 'dark');
});

test('explicit appearance overrides system and Android automatic appearance', async () => {
  const { isDarkTheme } = await load('../platform/theme.js');
  for (const systemDark of [true, false]) {
    for (const androidDark of [true, false]) {
      assert.equal(isDarkTheme('auto', systemDark, androidDark), systemDark || androidDark);
      assert.equal(isDarkTheme('dark', systemDark, androidDark), true);
      assert.equal(isDarkTheme('light', systemDark, androidDark), false);
    }
  }
});

test('theme updates the document and native status bar despite blocked storage', async () => {
  const { applyThemePreference } = await load('../platform/theme.js');
  const root = { dataset: {}, style: {} };
  const native = [];
  const android = { isDarkMode: () => true, setDarkModeStatusBar: value => native.push(value) };
  const storage = { setItem: () => { throw new Error('blocked'); } };
  assert.equal(applyThemePreference('auto', { root, storage, android, systemDark: false }), true);
  assert.equal(root.dataset.theme, 'dark');
  assert.equal(root.style.colorScheme, 'dark');
  assert.deepEqual(native, [true]);
  android.isDarkMode = () => { throw new Error('unavailable'); };
  android.setDarkModeStatusBar = () => { throw new Error('unavailable'); };
  assert.equal(applyThemePreference('light', { root, storage, android, systemDark: true }), false);
  assert.equal(root.dataset.theme, 'light');
});

test('theme writes only the compatible key with normalized values', async () => {
  const { applyThemePreference } = await load('../platform/theme.js');
  const writes = [];
  const root = { dataset: {}, style: {} };
  applyThemePreference('invalid', { root, storage: { setItem: (...args) => writes.push(args) }, systemDark: false });
  assert.deepEqual(writes, [['cfdata-theme', 'auto']]);
  assert.equal(root.dataset.theme, 'light');
});

test('entry keeps styles and scripts modular and has no external runtime resources', async () => {
  const html = await readFile(new URL('../index.html', import.meta.url), 'utf8');
  assert.match(html, /<html lang="zh-CN"/);
  assert.match(html, /<script type="module" src="\/assets\/app.js"><\/script>/);
  assert.equal((html.match(/rel="stylesheet"/g) || []).length, 3);
  assert.doesNotMatch(html, /<style\b|\sstyle=|\son\w+=|https?:\/\//i);
  assert.match(html, /href="\/"/);
  assert.match(html, /role="status"/);
  assert.match(html, /尚未接入/);
  for (const section of ['workbench', 'tasks', 'targets', 'settings']) {
    assert.match(html, new RegExp(`data-section="${section}"`));
    assert.match(html, new RegExp(`data-panel="${section}"`));
  }
  const app = await readFile(new URL('../app.js', import.meta.url), 'utf8');
  assert.doesNotMatch(app, /new WebSocket|\bfetch\s*\(|innerHTML\s*=/);
});
