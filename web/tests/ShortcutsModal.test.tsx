import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import React from 'react';
import { ShortcutsModal } from '../src/components/hud/ShortcutsModal';

describe('ShortcutsModal Component', () => {
  it('scopes role="dialog" to the inner modal container rather than the backdrop overlay', () => {
    const onClose = vi.fn();
    const { container } = render(<ShortcutsModal isOpen onClose={onClose} />);

    const dialog = screen.getByRole('dialog', { name: /COSMIC COMMAND INTERFACE/i });
    expect(dialog).toBeInTheDocument();
    expect(dialog).toHaveAttribute('aria-modal', 'true');
    expect(dialog).toHaveAttribute('aria-labelledby', 'shortcuts-modal-title');
    expect(dialog).not.toBe(container.firstElementChild);

    // Clicking inside the dialog should not close it
    fireEvent.click(dialog);
    expect(onClose).not.toHaveBeenCalled();

    // Clicking the backdrop overlay closes the dialog
    fireEvent.click(container.firstElementChild as HTMLElement);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('manages initial focus, traps focus on Tab, and restores focus on unmount', () => {
    const onClose = vi.fn();
    const triggerBtn = document.createElement('button');
    document.body.appendChild(triggerBtn);
    triggerBtn.focus();
    expect(document.activeElement).toBe(triggerBtn);

    const { unmount } = render(<ShortcutsModal isOpen onClose={onClose} />);

    const closeBtn = screen.getByRole('button', { name: /Close shortcuts dialog/i });
    expect(document.activeElement).toBe(closeBtn);

    // Pressing Tab when only one focusable element exists loops back to the same element
    fireEvent.keyDown(window, { key: 'Tab' });
    expect(document.activeElement).toBe(closeBtn);

    // Pressing Shift+Tab also loops back
    fireEvent.keyDown(window, { key: 'Tab', shiftKey: true });
    expect(document.activeElement).toBe(closeBtn);

    // Pressing Escape closes the modal
    fireEvent.keyDown(window, { key: 'Escape' });
    expect(onClose).toHaveBeenCalledTimes(1);

    unmount();
    expect(document.activeElement).toBe(triggerBtn);
    document.body.removeChild(triggerBtn);
  });
});
