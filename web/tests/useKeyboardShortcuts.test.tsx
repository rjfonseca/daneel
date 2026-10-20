import { fireEvent, renderHook } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { useKeyboardShortcuts } from '../src/hooks/useKeyboardShortcuts';

function callbacks() {
  return {
    onToggleSearch: vi.fn(),
    onCloseSearch: vi.fn(),
    onCloseControls: vi.fn(),
    onCloseCapture: vi.fn(),
    onCloseNote: vi.fn(),
    onCloseExplorer: vi.fn(),
    onCloseReview: vi.fn(),
    onToggleCockpit: vi.fn(),
    onToggleShortcuts: vi.fn(),
  };
}

describe('useKeyboardShortcuts', () => {
  it('closes review, then explorer, then the underlying note on successive Escape presses', () => {
    const actions = callbacks();
    const { rerender } = renderHook(({ review, explorer }) => useKeyboardShortcuts({
      ...actions,
      isSearchOpen: false,
      isControlsOpen: false,
      isReviewOpen: review,
      isExplorerOpen: explorer,
      hasActiveNote: true,
    }), { initialProps: { review: true, explorer: true } });

    fireEvent.keyDown(window, { key: 'Escape' });
    expect(actions.onCloseReview).toHaveBeenCalledOnce();
    expect(actions.onCloseExplorer).not.toHaveBeenCalled();
    expect(actions.onCloseNote).not.toHaveBeenCalled();

    rerender({ review: false, explorer: true });
    fireEvent.keyDown(window, { key: 'Escape' });
    expect(actions.onCloseExplorer).toHaveBeenCalledOnce();
    expect(actions.onCloseNote).not.toHaveBeenCalled();

    rerender({ review: false, explorer: false });
    fireEvent.keyDown(window, { key: 'Escape' });
    expect(actions.onCloseNote).toHaveBeenCalledOnce();
  });

  it('keeps capture ahead of knowledge panels in the Escape order', () => {
    const actions = callbacks();
    renderHook(() => useKeyboardShortcuts({
      ...actions,
      isSearchOpen: false,
      isControlsOpen: false,
      isCaptureOpen: true,
      isReviewOpen: true,
      hasActiveNote: true,
    }));

    fireEvent.keyDown(window, { key: 'Escape' });
    expect(actions.onCloseCapture).toHaveBeenCalledOnce();
    expect(actions.onCloseReview).not.toHaveBeenCalled();
    expect(actions.onCloseNote).not.toHaveBeenCalled();
  });

  it('does not handle an Escape already consumed by a local popover', () => {
    const actions = callbacks();
    renderHook(() => useKeyboardShortcuts({
      ...actions,
      isSearchOpen: false,
      isControlsOpen: false,
      hasActiveNote: true,
    }));

    const escape = new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true });
    escape.preventDefault();
    window.dispatchEvent(escape);
    expect(actions.onCloseNote).not.toHaveBeenCalled();
  });

  it('ignores typing shortcuts in a select but keeps Ctrl+K available', () => {
    const actions = callbacks();
    const select = document.createElement('select');
    document.body.append(select);
    try {
      renderHook(() => useKeyboardShortcuts({
        ...actions,
        isSearchOpen: false,
        isControlsOpen: false,
        hasActiveNote: false,
      }));

      fireEvent.keyDown(select, { key: '?', shiftKey: true });
      fireEvent.keyDown(select, { key: 'c', altKey: true });
      expect(actions.onToggleShortcuts).not.toHaveBeenCalled();
      expect(actions.onToggleCockpit).not.toHaveBeenCalled();

      fireEvent.keyDown(select, { key: 'k', ctrlKey: true });
      expect(actions.onToggleSearch).toHaveBeenCalledOnce();
    } finally {
      select.remove();
    }
  });
});
