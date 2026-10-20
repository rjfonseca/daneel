import { useState, useEffect, useCallback, useMemo } from 'react';
import type { PinnedAnchor } from '../gen/api/v1/graph_pb';

export interface PinnedAnchorItem {
  path: string;
  label: string;
  icon?: string;
  isCustom?: boolean;
}

const STORAGE_KEY = 'daneel:pinned_anchors';
export const PINNED_ANCHORS_EVENT = 'daneel:pinned_anchors_changed';

export function getLocalPinnedAnchors(): PinnedAnchorItem[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    if (Array.isArray(parsed)) {
      return parsed.filter(item => item && typeof item.path === 'string');
    }
  } catch (e) {
    console.warn('Failed to parse pinned anchors from localStorage', e);
  }
  return [];
}

export function saveLocalPinnedAnchors(anchors: PinnedAnchorItem[]): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(anchors));
    window.dispatchEvent(new Event(PINNED_ANCHORS_EVENT));
  } catch (e) {
    console.warn('Failed to save pinned anchors to localStorage', e);
  }
}

export function addLocalPinnedAnchor(note: { path: string; label?: string; icon?: string }): boolean {
  if (!note.path) return false;
  const current = getLocalPinnedAnchors();
  const existingIdx = current.findIndex(a => a.path === note.path);
  const label = note.label || note.path.split('/').pop()?.replace(/\.md$/, '') || note.path;
  const icon = note.icon || '📌';
  let next: PinnedAnchorItem[];
  if (existingIdx >= 0) {
    next = current.map((a, i) => i === existingIdx ? { ...a, label: note.label || a.label, icon: note.icon || a.icon } : a);
  } else {
    next = [...current, { path: note.path, label, icon, isCustom: true }];
  }
  saveLocalPinnedAnchors(next);
  return true;
}

export function removeLocalPinnedAnchor(path: string): boolean {
  if (!path) return false;
  const current = getLocalPinnedAnchors();
  const next = current.filter(a => a.path !== path);
  saveLocalPinnedAnchors(next);
  return true;
}

export interface UsePinnedAnchorsOptions {
  backendAnchors?: PinnedAnchor[];
}

export interface UsePinnedAnchorsResult {
  anchors: PinnedAnchorItem[];
  isPinned: (path: string) => boolean;
  togglePin: (note: { path: string; label?: string; icon?: string }) => boolean;
  addPin: (note: { path: string; label?: string; icon?: string }) => boolean;
  removePin: (path: string) => void;
}

export function usePinnedAnchors({ backendAnchors = [] }: UsePinnedAnchorsOptions = {}): UsePinnedAnchorsResult {
  const [localAnchors, setLocalAnchors] = useState<PinnedAnchorItem[]>(() => getLocalPinnedAnchors());

  useEffect(() => {
    const handleSync = () => {
      setLocalAnchors(getLocalPinnedAnchors());
    };
    window.addEventListener(PINNED_ANCHORS_EVENT, handleSync);
    window.addEventListener('storage', handleSync);
    return () => {
      window.removeEventListener(PINNED_ANCHORS_EVENT, handleSync);
      window.removeEventListener('storage', handleSync);
    };
  }, []);

  // Merge backend anchors and local anchors (local can override or extend)
  // Deduping by path: backend first, then local additions.
  const anchors = useMemo(() => {
    const map = new Map<string, PinnedAnchorItem>();

    // 1. Backend declarative anchors
    backendAnchors.forEach(b => {
      if (!b.path) return;
      map.set(b.path, {
        path: b.path,
        label: b.label || b.path.split('/').pop()?.replace(/\.md$/, '') || b.path,
        icon: b.icon || '◈',
        isCustom: false,
      });
    });

    // 2. Client local storage anchors
    localAnchors.forEach(l => {
      if (!l.path) return;
      map.set(l.path, {
        ...l,
        isCustom: true,
      });
    });

    return Array.from(map.values());
  }, [backendAnchors, localAnchors]);

  const isPinned = useCallback(
    (path: string) => {
      if (!path) return false;
      return anchors.some(a => a.path === path);
    },
    [anchors]
  );

  const togglePin = useCallback(
    (note: { path: string; label?: string; icon?: string }): boolean => {
      if (!note.path) return false;
      const current = getLocalPinnedAnchors();
      const existingIdx = current.findIndex(a => a.path === note.path);

      let nowPinned: boolean;
      let next: PinnedAnchorItem[];

      if (existingIdx >= 0) {
        // Unpin
        next = current.filter(a => a.path !== note.path);
        nowPinned = false;
      } else {
        // Pin
        const label = note.label || note.path.split('/').pop()?.replace(/\.md$/, '') || note.path;
        next = [...current, { path: note.path, label, icon: note.icon || '📌', isCustom: true }];
        nowPinned = true;
      }

      saveLocalPinnedAnchors(next);
      setLocalAnchors(next);
      return nowPinned;
    },
    []
  );

  const addPin = useCallback((note: { path: string; label?: string; icon?: string }): boolean => {
    return addLocalPinnedAnchor(note);
  }, []);

  const removePin = useCallback((path: string) => {
    removeLocalPinnedAnchor(path);
  }, []);

  return {
    anchors,
    isPinned,
    togglePin,
    addPin,
    removePin,
  };
}
