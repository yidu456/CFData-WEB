import { normalizeSection } from './navigation.js';
import { readThemePreference, applyThemePreference } from './platform/theme.js';
import { createSocketClient } from './api/socket.js';
import { mountTasks } from './features/tasks/view.js';

let storage;
try {
  storage = window.localStorage;
} catch {
  // The shell also works when browser policy denies access to storage.
}
const media = window.matchMedia('(prefers-color-scheme: dark)');
const themeSelect = document.querySelector('#theme-preference');
themeSelect.value = readThemePreference(storage);

function updateTheme() {
  applyThemePreference(themeSelect.value, {
    root: document.documentElement,
    storage,
    systemDark: media.matches,
    android: window.CFDataAndroid,
  });
}
updateTheme();
themeSelect.addEventListener('change', updateTheme);
media.addEventListener('change', updateTheme);
window.addEventListener('pageshow', event => {
  if (event.persisted) themeSelect.value = readThemePreference(storage, themeSelect.value);
  updateTheme();
});

const navigation = document.querySelectorAll('[data-section]');
const panels = document.querySelectorAll('[data-panel]');
for (const button of navigation) {
  button.addEventListener('click', () => {
    const section = normalizeSection(button.dataset.section);
    for (const item of navigation) {
      if (item.dataset.section === section) item.setAttribute('aria-current', 'page');
      else item.removeAttribute('aria-current');
    }
    for (const panel of panels) panel.hidden = panel.dataset.panel !== section;
    document.querySelector(`[data-panel="${section}"] h1`).focus();
  });
}

const modes = document.querySelectorAll('[data-mode]');
for (const button of modes) {
  button.addEventListener('click', () => {
    for (const item of modes) item.setAttribute('aria-pressed', String(item === button));
    for (const panel of document.querySelectorAll('[data-mode-panel]')) {
      panel.hidden = panel.dataset.modePanel !== button.dataset.mode;
    }
  });
}

const monitor = mountTasks({ root: document, client: { send: (...args) => client.send(...args) } });
const client = createSocketClient({
  url: `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/ws`,
  onState: value => {
    monitor.onState(value);
    if (value === 'auth-required') window.location.assign('/auth/login');
  },
  onMessage: monitor.onMessage,
  onError: monitor.onError,
});
client.start();
window.addEventListener('pagehide', () => { client.close(); monitor.destroy(); });
window.addEventListener('pageshow', event => { if (event.persisted) client.start(); });
