export function normalizeSection(value) {
  return ['workbench', 'tasks', 'targets', 'settings'].includes(value) ? value : 'workbench';
}
