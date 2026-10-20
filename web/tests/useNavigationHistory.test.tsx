import { act, renderHook } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useNavigationHistory, type NavigationSnapshot } from '../src/hooks/useNavigationHistory';

const blank: NavigationSnapshot = {
  target: null, camera: null, activeFilter: 'All', edgeMode: 'focus',
  showRings: true, showSectorLabels: true, showNebulaLabels: true,
  isCosmicMode: true, reducedMotion: false, drawerWidth: 440, scrollTop: 0,
};

describe('useNavigationHistory', () => {
  beforeEach(() => window.history.replaceState(null, '', '/'));

  it('tracks snapshots and restores camera, filters and scroll on browser Back', () => {
    const onRestore = vi.fn();
    const { result } = renderHook(() => useNavigationHistory(blank, onRestore));
    const note: NavigationSnapshot = { ...blank, target: { kind: 'note', id: 'notes/a.md' }, camera: { x: .3, y: .4, ratio: .2, angle: 0 }, edgeMode: 'all', scrollTop: 80 };
    act(() => result.current.push(note));
    expect(window.location.hash).toBe('#note=notes%2Fa.md');
    expect(result.current.canBack).toBe(true);
    act(() => result.current.replace({ ...note, scrollTop: 120 }));
    act(() => window.dispatchEvent(new PopStateEvent('popstate', { state: { __daneelNavigation: true, index: 0, snapshot: blank } })));
    expect(onRestore).toHaveBeenCalledWith(blank);
    expect(result.current.canForward).toBe(true);
    act(() => window.dispatchEvent(new PopStateEvent('popstate', { state: { __daneelNavigation: true, index: 1, snapshot: { ...note, scrollTop: 120 } } })));
    expect(onRestore).toHaveBeenLastCalledWith({ ...note, scrollTop: 120 });
  });

  it('ignores malformed popstate values', () => {
    const onRestore = vi.fn();
    renderHook(() => useNavigationHistory(blank, onRestore));
    act(() => window.dispatchEvent(new PopStateEvent('popstate', { state: { __daneelNavigation: true, index: 1, snapshot: { ...blank, camera: { x: Infinity, y: 0, ratio: 1, angle: 0 } } } })));
    expect(onRestore).not.toHaveBeenCalled();
  });
});
