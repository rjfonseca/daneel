export const CACHE_KEY = 'second_brain_galaxy_layout_v2';

const isSatelliteId = (id: string) =>
  id.startsWith('harvester:') || id.startsWith('routine:') || id.startsWith('skill:');

export function saveLayout(positions: Record<string, {x: number, y: number}>) {
  try {
    const sanitized: Record<string, {x: number, y: number}> = {};
    for (const [key, val] of Object.entries(positions)) {
      if (!isSatelliteId(key)) {
        sanitized[key] = val;
      }
    }
    localStorage.setItem(CACHE_KEY, JSON.stringify(sanitized));
  } catch (e) {
    console.error('Failed to save layout to localStorage', e);
  }
}

export function loadLayout(): Record<string, {x: number, y: number}> | null {
  try {
    const data = localStorage.getItem(CACHE_KEY);
    if (!data) return null;
    const parsed = JSON.parse(data);
    const sanitized: Record<string, {x: number, y: number}> = {};
    for (const [key, val] of Object.entries(parsed as Record<string, {x: number, y: number}>)) {
      if (!isSatelliteId(key)) {
        sanitized[key] = val;
      }
    }
    return sanitized;
  } catch (e) {
    console.error('Failed to parse cached layout', e);
    return null;
  }
}
