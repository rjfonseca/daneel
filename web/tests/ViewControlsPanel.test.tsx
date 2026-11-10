import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { ViewControlsPanel } from '../src/components/ViewControlsPanel';
import { DEFAULT_GALAXY_VIEW } from '../src/lib/galaxyView';

describe('ViewControlsPanel', () => {
  it('applies a preset and allows granular edge and overlay changes', () => {
    const onChange = vi.fn();
    const { rerender } = render(<ViewControlsPanel isOpen onClose={vi.fn()} view={DEFAULT_GALAXY_VIEW} onChange={onChange} maxNebulas={6} onUpdateMaxNebulas={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', { name: 'Read view' }));
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ showRings: false, isCosmicMode: false, edgeMode: 'focus' }));

    rerender(<ViewControlsPanel isOpen onClose={vi.fn()} view={DEFAULT_GALAXY_VIEW} onChange={onChange} maxNebulas={6} onUpdateMaxNebulas={vi.fn()} />);
    fireEvent.click(screen.getByTestId('edge-mode-bridges'));
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ edgeMode: 'bridges' }));
    fireEvent.click(screen.getByTestId('toggle-cloak-nebulas'));
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ showNebulaLabels: false }));
  });

  it('keeps the existing edge mode titles and reduces decorative motion', () => {
    const onChange = vi.fn();
    render(<ViewControlsPanel isOpen onClose={vi.fn()} view={DEFAULT_GALAXY_VIEW} onChange={onChange} maxNebulas={6} onUpdateMaxNebulas={vi.fn()} />);
    expect(screen.getByTestId('edge-mode-auto').getAttribute('title')).toMatch(/zoom/i);
    fireEvent.click(screen.getByRole('button', { name: 'Reduce motion' }));
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ reducedMotion: true }));
  });

  it('disables stepper buttons and sets informative titles at lower and upper bounds', () => {
    const onUpdateMaxNebulas = vi.fn();
    const { rerender } = render(
      <ViewControlsPanel
        isOpen
        onClose={vi.fn()}
        view={DEFAULT_GALAXY_VIEW}
        onChange={vi.fn()}
        maxNebulas={1}
        onUpdateMaxNebulas={onUpdateMaxNebulas}
      />
    );

    const decreaseBtn = screen.getByTestId('decrease-max-nebulas');
    const increaseBtn = screen.getByTestId('increase-max-nebulas');

    expect(decreaseBtn).toBeDisabled();
    expect(decreaseBtn).toHaveAttribute('title', 'Decrease maximum visible nebulas (minimum reached)');
    expect(increaseBtn).not.toBeDisabled();
    expect(increaseBtn).toHaveAttribute('title', 'Increase maximum visible nebulas');

    rerender(
      <ViewControlsPanel
        isOpen
        onClose={vi.fn()}
        view={DEFAULT_GALAXY_VIEW}
        onChange={vi.fn()}
        maxNebulas={32}
        onUpdateMaxNebulas={onUpdateMaxNebulas}
      />
    );

    expect(decreaseBtn).not.toBeDisabled();
    expect(decreaseBtn).toHaveAttribute('title', 'Decrease maximum visible nebulas');
    expect(increaseBtn).toBeDisabled();
    expect(increaseBtn).toHaveAttribute('title', 'Increase maximum visible nebulas (maximum reached)');

    rerender(
      <ViewControlsPanel
        isOpen
        onClose={vi.fn()}
        view={DEFAULT_GALAXY_VIEW}
        onChange={vi.fn()}
        maxNebulas={6}
        onUpdateMaxNebulas={onUpdateMaxNebulas}
      />
    );

    expect(decreaseBtn).not.toBeDisabled();
    expect(increaseBtn).not.toBeDisabled();
    fireEvent.click(decreaseBtn);
    expect(onUpdateMaxNebulas).toHaveBeenCalledWith(5);
    fireEvent.click(increaseBtn);
    expect(onUpdateMaxNebulas).toHaveBeenCalledWith(7);
  });
});
