import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { EdgeVisibilityMode } from '../lib/edgeFiltering';
import { decodeNavigationHash, encodeNavigationHash, type NavigationTarget } from '../lib/navigationHelpers';

export interface NavigationSnapshot {
  target: NavigationTarget | null;
  camera: { x: number; y: number; ratio: number; angle: number } | null;
  activeFilter: string;
  edgeMode: EdgeVisibilityMode;
  showRings: boolean;
  showSectorLabels: boolean;
  showNebulaLabels: boolean;
  isCosmicMode: boolean;
  reducedMotion: boolean;
  drawerWidth: number;
  scrollTop: number;
}

interface BrowserNavigationState { __daneelNavigation: true; index: number; snapshot: NavigationSnapshot }

export function isNavigationSnapshot(value: unknown): value is NavigationSnapshot {
  if (!value || typeof value !== 'object') return false;
  const s = value as Record<string, unknown>;
  const target = s.target;
  if (target !== null) {
    if (typeof target !== 'object' || !target) return false;
    const candidate = target as Record<string, unknown>;
    if ((candidate.kind !== 'note' && candidate.kind !== 'cluster' && candidate.kind !== 'sector') || typeof candidate.id !== 'string' ||
      !decodeNavigationHash(encodeNavigationHash(candidate as NavigationTarget))) return false;
  }
  const camera = s.camera;
  if (camera !== null && (typeof camera !== 'object' || !camera ||
    !['x', 'y', 'ratio', 'angle'].every(key => Number.isFinite((camera as Record<string, unknown>)[key])) ||
    (camera as NavigationSnapshot['camera'])!.ratio <= 0 || (camera as NavigationSnapshot['camera'])!.ratio > 10)) return false;
  return typeof s.activeFilter === 'string' && s.activeFilter.length <= 80 &&
    ['auto', 'bridges', 'focus', 'all'].includes(String(s.edgeMode)) &&
    ['showRings', 'showSectorLabels', 'showNebulaLabels', 'isCosmicMode', 'reducedMotion'].every(key => typeof s[key] === 'boolean') &&
    typeof s.drawerWidth === 'number' && Number.isFinite(s.drawerWidth) && s.drawerWidth >= 320 && s.drawerWidth <= 800 &&
    typeof s.scrollTop === 'number' && Number.isFinite(s.scrollTop) && s.scrollTop >= 0 && s.scrollTop < 10_000_000;
}

function stateFor(index: number, snapshot: NavigationSnapshot): BrowserNavigationState {
  return { __daneelNavigation: true, index, snapshot };
}

function urlFor(snapshot: NavigationSnapshot): string {
  return `${window.location.pathname}${window.location.search}${encodeNavigationHash(snapshot.target)}`;
}

export function useNavigationHistory(initial: NavigationSnapshot, onRestore: (snapshot: NavigationSnapshot) => void) {
  const initialRef = useRef(initial);
  const restoreRef = useRef(onRestore);
  restoreRef.current = onRestore;
  const snapshots = useRef<NavigationSnapshot[]>([initialRef.current]);
  const position = useRef(0);
  const [index, setIndex] = useState(0);
  const [length, setLength] = useState(1);

  useEffect(() => {
    const existing = window.history.state as Partial<BrowserNavigationState> | null;
    if (existing?.__daneelNavigation && Number.isInteger(existing.index) && existing.index! >= 0 && existing.index! <= 1000 && isNavigationSnapshot(existing.snapshot)) {
      position.current = existing.index!;
      snapshots.current = Array.from({ length: existing.index! + 1 }, () => initialRef.current);
      snapshots.current[existing.index!] = existing.snapshot!;
      setIndex(existing.index!);
      setLength(existing.index! + 1);
    } else {
      window.history.replaceState(stateFor(0, initialRef.current), '', window.location.href);
    }
    const pop = (event: PopStateEvent) => {
      const state = event.state as Partial<BrowserNavigationState> | null;
      if (!state?.__daneelNavigation || !Number.isInteger(state.index) || !isNavigationSnapshot(state.snapshot)) return;
      const nextIndex = state.index!;
      if (nextIndex < 0 || nextIndex > 1000) return;
      position.current = nextIndex;
      snapshots.current[nextIndex] = state.snapshot!;
      setIndex(nextIndex);
      setLength(Math.max(snapshots.current.length, nextIndex + 1));
      restoreRef.current(state.snapshot!);
    };
    window.addEventListener('popstate', pop);
    return () => window.removeEventListener('popstate', pop);
  }, []);

  const replace = useCallback((snapshot: NavigationSnapshot) => {
    snapshots.current[position.current] = snapshot;
    window.history.replaceState(stateFor(position.current, snapshot), '', urlFor(snapshot));
  }, []);

  const push = useCallback((snapshot: NavigationSnapshot) => {
    if (!isNavigationSnapshot(snapshot)) return;
    const nextIndex = position.current + 1;
    snapshots.current = snapshots.current.slice(0, nextIndex);
    snapshots.current.push(snapshot);
    position.current = nextIndex;
    window.history.pushState(stateFor(nextIndex, snapshot), '', urlFor(snapshot));
    setIndex(nextIndex);
    setLength(nextIndex + 1);
  }, []);

  return useMemo(() => ({
    push,
    replace,
    back: () => window.history.back(),
    forward: () => window.history.forward(),
    canBack: index > 0 || window.history.length > 1,
    canForward: index < length - 1,
  }), [push, replace, index, length]);
}
