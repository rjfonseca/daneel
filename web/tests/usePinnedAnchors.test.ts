import { PinnedAnchor } from '../src/gen/api/v1/graph_pb';
import { describe, it, expect, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { usePinnedAnchors } from '../src/hooks/usePinnedAnchors';

describe('usePinnedAnchors Hook', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('initializes with backend anchors when provided', () => {
    const backendAnchors = [
      { path: 'notes/architecture.md', label: 'Architecture', icon: '🏛️' },
    ];

    const { result } = renderHook(() => usePinnedAnchors({ backendAnchors: backendAnchors.map(anchor => new PinnedAnchor(anchor)) }));

    expect(result.current.anchors.length).toBe(1);
    expect(result.current.anchors[0].path).toBe('notes/architecture.md');
    expect(result.current.isPinned('notes/architecture.md')).toBe(true);
    expect(result.current.isPinned('notes/other.md')).toBe(false);
  });

  it('allows pinning and unpinning notes dynamically via localStorage', () => {
    const { result } = renderHook(() => usePinnedAnchors());

    expect(result.current.anchors.length).toBe(0);

    // Pin note
    act(() => {
      const pinned = result.current.togglePin({ path: 'notes/weekly-plan.md', label: 'Weekly Plan' });
      expect(pinned).toBe(true);
    });

    expect(result.current.isPinned('notes/weekly-plan.md')).toBe(true);
    expect(result.current.anchors.length).toBe(1);

    // Toggle again to unpin
    act(() => {
      const pinned = result.current.togglePin({ path: 'notes/weekly-plan.md' });
      expect(pinned).toBe(false);
    });

    expect(result.current.isPinned('notes/weekly-plan.md')).toBe(false);
    expect(result.current.anchors.length).toBe(0);
  });

  it('removes pin by path', () => {
    const { result } = renderHook(() => usePinnedAnchors());

    act(() => {
      result.current.togglePin({ path: 'notes/alpha.md', label: 'Alpha' });
      result.current.togglePin({ path: 'notes/beta.md', label: 'Beta' });
    });

    expect(result.current.anchors.length).toBe(2);

    act(() => {
      result.current.removePin('notes/alpha.md');
    });

    expect(result.current.anchors.length).toBe(1);
    expect(result.current.isPinned('notes/alpha.md')).toBe(false);
    expect(result.current.isPinned('notes/beta.md')).toBe(true);
  });

  it('idempotently adds pins using addPin', () => {
    const { result } = renderHook(() => usePinnedAnchors());

    act(() => {
      result.current.addPin({ path: 'notes/gamma.md', label: 'Gamma', icon: '⚡' });
    });

    expect(result.current.isPinned('notes/gamma.md')).toBe(true);
    expect(result.current.anchors.length).toBe(1);
    expect(result.current.anchors[0].label).toBe('Gamma');

    // Calling addPin again does not unpin (unlike togglePin)
    act(() => {
      result.current.addPin({ path: 'notes/gamma.md', label: 'Gamma Updated' });
    });

    expect(result.current.isPinned('notes/gamma.md')).toBe(true);
    expect(result.current.anchors.length).toBe(1);
    expect(result.current.anchors[0].label).toBe('Gamma Updated');
  });
});
