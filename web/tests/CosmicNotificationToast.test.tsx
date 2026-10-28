import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';
import React from 'react';
import { CosmicNotificationToast } from '../src/components/CosmicNotificationToast';

describe('CosmicNotificationToast', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('renders null when notification is null', () => {
    const { container } = render(<CosmicNotificationToast notification={null} onClose={vi.fn()} />);
    expect(container.firstChild).toBeNull();
  });

  it('renders notification with accessible role="status" and aria-live="polite"', () => {
    const notification = { title: 'Galactic Chronometer', message: 'Opening daily log: daily/2025-05-18.md', id: 1 };
    render(<CosmicNotificationToast notification={notification} onClose={vi.fn()} />);

    const statusEl = screen.getByRole('status');
    expect(statusEl).toBeInTheDocument();
    expect(statusEl).toHaveAttribute('aria-live', 'polite');
    expect(statusEl).toHaveAttribute('aria-atomic', 'true');
    expect(screen.getByText('Galactic Chronometer')).toBeInTheDocument();
    expect(screen.getByText('Opening daily log: daily/2025-05-18.md')).toBeInTheDocument();
  });

  it('invokes onClose when close button is clicked', () => {
    const onClose = vi.fn();
    const notification = { title: 'Seldon Vault Anchor', message: 'Focusing anchored note', id: 2 };
    render(<CosmicNotificationToast notification={notification} onClose={onClose} />);

    const closeBtn = screen.getByRole('button', { name: 'Close notification' });
    expect(closeBtn).toBeInTheDocument();
    expect(closeBtn).toHaveClass('cosmic-focus-visible');

    fireEvent.click(closeBtn);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('auto-dismisses after 4500ms', () => {
    const onClose = vi.fn();
    const notification = { title: 'Test Toast', message: 'Auto dismiss test', id: 3 };
    render(<CosmicNotificationToast notification={notification} onClose={onClose} />);

    expect(onClose).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(4500);
    });

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('invokes onClose when Escape key is pressed', () => {
    const onClose = vi.fn();
    const notification = { title: 'Keyboard Test', message: 'Press Escape to dismiss', id: 4 };
    render(<CosmicNotificationToast notification={notification} onClose={onClose} />);

    fireEvent.keyDown(window, { key: 'Escape' });
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
