import { renderHook, waitFor, act } from '@testing-library/react';
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { useUIEventSync } from '../src/hooks/useUIEventSync';
import { client } from '../src/lib/apiClient';
import { UIActionType, UIEvent, ReportActiveContextResponse } from '../src/gen/api/v1/graph_pb';

vi.mock('../src/lib/apiClient', () => ({
  client: {
    subscribeUIEvents: vi.fn(),
    reportActiveContext: vi.fn()
  }
}));

describe('useUIEventSync', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('handles ACTION_OPEN_NOTE correctly', async () => {
    let pushEvent!: (result: IteratorResult<UIEvent>) => void;
    const stream = {
      [Symbol.asyncIterator]() {
        return {
          next() {
            return new Promise<IteratorResult<UIEvent>>((resolve) => {
              pushEvent = resolve;
            });
          }
        };
      }
    };
    vi.mocked(client.subscribeUIEvents).mockReturnValue(stream);

    const setSelectedNodeId = vi.fn();
    const setIsDrawerOpen = vi.fn();
    const setIsConstellationIsolated = vi.fn();
    const setFocusedSector = vi.fn();
    const reloadGalaxy = vi.fn();

    renderHook(() => useUIEventSync({
      setSelectedNodeId,
      setIsDrawerOpen,
      setIsConstellationIsolated,
      setFocusedSector,
      reloadGalaxy,
      selectedNodeId: null,
      focusedSector: null
    }));

    act(() => {
      pushEvent({
        value: new UIEvent({
          action: UIActionType.ACTION_OPEN_NOTE,
          targetPath: 'notes/test.md'
        }),
        done: false
      });
    });

    await waitFor(() => {
      expect(setSelectedNodeId).toHaveBeenCalledWith('notes/test.md');
    });
  });

  it('handles ACTION_FLASH_NOTIFICATION', async () => {
    let pushEvent!: (result: IteratorResult<UIEvent>) => void;
    const stream = {
      [Symbol.asyncIterator]() {
        return {
          next() {
            return new Promise<IteratorResult<UIEvent>>((resolve) => {
              pushEvent = resolve;
            });
          }
        };
      }
    };
    vi.mocked(client.subscribeUIEvents).mockReturnValue(stream);

    const { result } = renderHook(() => useUIEventSync({
      setSelectedNodeId: vi.fn(),
      setIsDrawerOpen: vi.fn(),
      setIsConstellationIsolated: vi.fn(),
      setFocusedSector: vi.fn(),
      reloadGalaxy: vi.fn(),
      selectedNodeId: null,
      focusedSector: null
    }));

    act(() => {
      pushEvent({
        value: new UIEvent({
          action: UIActionType.ACTION_FLASH_NOTIFICATION,
          targetPath: 'Alert!',
          payloadJson: JSON.stringify({ title: 'Danger', message: 'Core breach' })
        }),
        done: false
      });
    });

    await waitFor(() => {
      expect(result.current.toastNotification).not.toBeNull();
    });
    
    expect(result.current.toastNotification?.title).toBe('Danger');
  });
  
  it('reports active context on debounced change', async () => {
    const stream = {
      [Symbol.asyncIterator]() {
        return { next: async () => ({ done: true as const, value: undefined }) };
      }
    };
    vi.mocked(client.subscribeUIEvents).mockReturnValue(stream);
    vi.mocked(client.reportActiveContext).mockResolvedValue(new ReportActiveContextResponse());

    renderHook(() => useUIEventSync({
      setSelectedNodeId: vi.fn(),
      setIsDrawerOpen: vi.fn(),
      setIsConstellationIsolated: vi.fn(),
      setFocusedSector: vi.fn(),
      reloadGalaxy: vi.fn(),
      selectedNodeId: 'notes/active.md',
      focusedSector: 'sector1'
    }));

    await waitFor(() => {
      expect(client.reportActiveContext).toHaveBeenCalled();
    });
  });

  it('handles ACTION_PIN_NOTE and ACTION_UNPIN_NOTE correctly', async () => {
    localStorage.clear();
    let pushEvent!: (result: IteratorResult<UIEvent>) => void;
    const stream = {
      [Symbol.asyncIterator]() {
        return {
          next() {
            return new Promise<IteratorResult<UIEvent>>((resolve) => {
              pushEvent = resolve;
            });
          }
        };
      }
    };
    vi.mocked(client.subscribeUIEvents).mockReturnValue(stream);

    const { result } = renderHook(() => useUIEventSync({
      setSelectedNodeId: vi.fn(),
      setIsDrawerOpen: vi.fn(),
      setIsConstellationIsolated: vi.fn(),
      setFocusedSector: vi.fn(),
      reloadGalaxy: vi.fn(),
      selectedNodeId: null,
      focusedSector: null
    }));

    // Test Pin
    act(() => {
      pushEvent({
        value: new UIEvent({
          action: UIActionType.ACTION_PIN_NOTE,
          targetPath: 'notes/architecture.md',
          payloadJson: JSON.stringify({ label: 'Architecture', icon: '🏛️' })
        }),
        done: false
      });
    });

    await waitFor(() => {
      expect(result.current.toastNotification?.title).toBe('Seldon Vault Anchors');
      expect(result.current.toastNotification?.message).toBe('Pinned to hotbar: Architecture');
    });

    const stored = JSON.parse(localStorage.getItem('daneel:pinned_anchors') || '[]');
    expect(stored).toEqual([
      { path: 'notes/architecture.md', label: 'Architecture', icon: '🏛️', isCustom: true }
    ]);

    // Test Unpin
    act(() => {
      pushEvent({
        value: new UIEvent({
          action: UIActionType.ACTION_UNPIN_NOTE,
          targetPath: 'notes/architecture.md'
        }),
        done: false
      });
    });

    await waitFor(() => {
      expect(result.current.toastNotification?.message).toBe('Unpinned from hotbar: architecture');
    });

    const storedAfter = JSON.parse(localStorage.getItem('daneel:pinned_anchors') || '[]');
    expect(storedAfter).toEqual([]);
  });
});
