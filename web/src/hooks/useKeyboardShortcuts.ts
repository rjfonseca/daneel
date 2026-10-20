import { useEffect } from 'react';

interface UseKeyboardShortcutsProps {
  onToggleSearch: () => void;
  isSearchOpen: boolean;
  onCloseSearch: () => void;
  isControlsOpen: boolean;
  onCloseControls: () => void;
  isExplorerOpen?: boolean;
  onCloseExplorer?: () => void;
  isReviewOpen?: boolean;
  onCloseReview?: () => void;
  isCaptureOpen?: boolean;
  onCloseCapture?: () => void;
  hasActiveNote: boolean;
  onCloseNote: () => void;
  isShortcutsOpen?: boolean;
  onToggleShortcuts?: () => void;
  onCloseShortcuts?: () => void;
  isCloakOpen?: boolean;
  onCloseCloak?: () => void;
  isCockpitOpen?: boolean;
  onToggleCockpit?: () => void;
  onCloseCockpit?: () => void;
}

export const useKeyboardShortcuts = ({
  onToggleSearch,
  isSearchOpen,
  onCloseSearch,
  isControlsOpen,
  onCloseControls,
  isExplorerOpen = false,
  onCloseExplorer,
  isReviewOpen = false,
  onCloseReview,
  isCaptureOpen = false,
  onCloseCapture,
  hasActiveNote,
  onCloseNote,
  isShortcutsOpen = false,
  onToggleShortcuts,
  onCloseShortcuts,
  isCloakOpen = false,
  onCloseCloak,
  isCockpitOpen = false,
  onToggleCockpit,
  onCloseCockpit,
}: UseKeyboardShortcutsProps): void => {
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.defaultPrevented) return;
      const target = e.target as HTMLElement | null;
      const isInputFocused =
        target &&
        (target.tagName === 'INPUT' ||
          target.tagName === 'TEXTAREA' ||
          target.tagName === 'SELECT' ||
          target.isContentEditable);

      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        onToggleSearch();
      } else if (e.altKey && e.key.toLowerCase() === 'c' && !isInputFocused) {
        e.preventDefault();
        onToggleCockpit?.();
      } else if (e.key === '?' && !e.ctrlKey && !e.metaKey && !e.altKey && !isInputFocused) {
        e.preventDefault();
        onToggleShortcuts?.();
      } else if (e.key === 'Escape') {
        if (isCaptureOpen) {
          onCloseCapture?.();
        } else if (isCockpitOpen) {
          onCloseCockpit?.();
        } else if (isShortcutsOpen) {
          onCloseShortcuts?.();
        } else if (isCloakOpen) {
          onCloseCloak?.();
        } else if (isSearchOpen) {
          onCloseSearch();
        } else if (isControlsOpen) {
          onCloseControls();
        } else if (isReviewOpen) {
          onCloseReview?.();
        } else if (isExplorerOpen) {
          onCloseExplorer?.();
        } else if (hasActiveNote) {
          onCloseNote();
        }
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [
    onToggleSearch,
    isSearchOpen,
    onCloseSearch,
    isControlsOpen,
    onCloseControls,
    isExplorerOpen,
    onCloseExplorer,
    isReviewOpen,
    onCloseReview,
    isCaptureOpen,
    onCloseCapture,
    hasActiveNote,
    onCloseNote,
    isShortcutsOpen,
    onToggleShortcuts,
    onCloseShortcuts,
    isCloakOpen,
    onCloseCloak,
    isCockpitOpen,
    onToggleCockpit,
    onCloseCockpit,
  ]);
};
