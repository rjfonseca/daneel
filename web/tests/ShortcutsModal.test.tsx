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
});
