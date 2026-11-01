const themeKey = 'cfdata-theme';

function normalizePreference(value) {
  return ['auto', 'light', 'dark'].includes(value) ? value : 'auto';
}

export function readThemePreference(storage, fallback = 'auto') {
  try {
    return normalizePreference(storage.getItem(themeKey));
  } catch {
    return normalizePreference(fallback);
  }
}

export function isDarkTheme(preference, systemDark, androidDark) {
  return preference === 'dark' || (preference === 'auto' && (systemDark || androidDark));
}

export function applyThemePreference(preference, { root, storage, systemDark, android }) {
  preference = normalizePreference(preference);
  let androidDark = false;
  try {
    androidDark = android?.isDarkMode() === true;
  } catch {
    // Native appearance may be unavailable while the WebView is resuming.
  }
  const dark = isDarkTheme(preference, systemDark, androidDark);
  root.dataset.theme = dark ? 'dark' : 'light';
  root.style.colorScheme = dark ? 'dark' : 'light';
  try {
    storage?.setItem(themeKey, preference);
  } catch {
    // Private browsing can block persistence, but not the appearance change.
  }
  try {
    android?.setDarkModeStatusBar(dark);
  } catch {
    // Keep the page usable when the optional native bridge is unavailable.
  }
  return dark;
}
