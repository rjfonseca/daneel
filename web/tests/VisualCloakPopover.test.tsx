import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { VisualCloakPopover } from '../src/components/hud/VisualCloakPopover';

describe('VisualCloakPopover Component', () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  const defaultProps = {
    isOpen: true,
    onClose: vi.fn(),
    showSectorLabels: true,
    onToggleSectorLabels: vi.fn(),
    showNebulaLabels: true,
    onToggleNebulaLabels: vi.fn(),
    isCosmicMode: true,
    onToggleCosmicMode: vi.fn(),
    showRings: true,
    onToggleRings: vi.fn(),
  };

  it('renders correctly when isOpen is true', () => {
    render(<VisualCloakPopover {...defaultProps} />);

    expect(screen.getByText('VISUAL CLOAK')).toBeDefined();
    expect(screen.getByText('ZEN SKY OBSERVATION')).toBeDefined();
    expect(screen.getByTestId('toggle-cloak-sectors')).toBeDefined();
    expect(screen.getByTestId('toggle-cloak-nebulas')).toBeDefined();
    expect(screen.getByTestId('toggle-cloak-photons')).toBeDefined();
    expect(screen.getByTestId('toggle-cloak-perimeter')).toBeDefined();
  });

  it('returns null when isOpen is false', () => {
    const { container } = render(<VisualCloakPopover {...defaultProps} isOpen={false} />);
    expect(container.firstChild).toBeNull();
  });

  it('calls onClose when close button is clicked', () => {
    const onClose = vi.fn();
    render(<VisualCloakPopover {...defaultProps} onClose={onClose} />);

    const closeBtn = screen.getByTestId('close-visual-cloak');
    fireEvent.click(closeBtn);

    expect(onClose).toHaveBeenCalled();
  });

  it('toggles individual layers when clicked', () => {
    const onToggleSectorLabels = vi.fn();
    const onToggleNebulaLabels = vi.fn();
    const onToggleCosmicMode = vi.fn();
    const onToggleRings = vi.fn();

    render(
      <VisualCloakPopover
        {...defaultProps}
        onToggleSectorLabels={onToggleSectorLabels}
        onToggleNebulaLabels={onToggleNebulaLabels}
        onToggleCosmicMode={onToggleCosmicMode}
        onToggleRings={onToggleRings}
      />
    );

    fireEvent.click(screen.getByTestId('toggle-cloak-sectors'));
    expect(onToggleSectorLabels).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByTestId('toggle-cloak-nebulas'));
    expect(onToggleNebulaLabels).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByTestId('toggle-cloak-photons'));
    expect(onToggleCosmicMode).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByTestId('toggle-cloak-perimeter'));
    expect(onToggleRings).toHaveBeenCalledTimes(1);
  });

  it('engages Zen Sky by toggling off all active layers', () => {
    const onToggleSectorLabels = vi.fn();
    const onToggleNebulaLabels = vi.fn();
    const onToggleCosmicMode = vi.fn();
    const onToggleRings = vi.fn();

    render(
      <VisualCloakPopover
        {...defaultProps}
        showSectorLabels={true}
        showNebulaLabels={true}
        isCosmicMode={true}
        showRings={true}
        onToggleSectorLabels={onToggleSectorLabels}
        onToggleNebulaLabels={onToggleNebulaLabels}
        onToggleCosmicMode={onToggleCosmicMode}
        onToggleRings={onToggleRings}
      />
    );

    const zenBtn = screen.getByTestId('toggle-zen-sky');
    fireEvent.click(zenBtn);

    expect(onToggleSectorLabels).toHaveBeenCalledTimes(1);
    expect(onToggleNebulaLabels).toHaveBeenCalledTimes(1);
    expect(onToggleCosmicMode).toHaveBeenCalledTimes(1);
    expect(onToggleRings).toHaveBeenCalledTimes(1);
  });

  it('restores all layers when Zen Sky is already active', () => {
    const onToggleSectorLabels = vi.fn();
    const onToggleNebulaLabels = vi.fn();
    const onToggleCosmicMode = vi.fn();
    const onToggleRings = vi.fn();

    render(
      <VisualCloakPopover
        {...defaultProps}
        showSectorLabels={false}
        showNebulaLabels={false}
        isCosmicMode={false}
        showRings={false}
        onToggleSectorLabels={onToggleSectorLabels}
        onToggleNebulaLabels={onToggleNebulaLabels}
        onToggleCosmicMode={onToggleCosmicMode}
        onToggleRings={onToggleRings}
      />
    );

    expect(screen.getByText('ACTIVE')).toBeDefined();

    const zenBtn = screen.getByTestId('toggle-zen-sky');
    fireEvent.click(zenBtn);

    expect(onToggleSectorLabels).toHaveBeenCalledTimes(1);
    expect(onToggleNebulaLabels).toHaveBeenCalledTimes(1);
    expect(onToggleCosmicMode).toHaveBeenCalledTimes(1);
    expect(onToggleRings).toHaveBeenCalledTimes(1);
  });

  it('closes on click outside', () => {
    const onClose = vi.fn();
    render(
      <div>
        <div data-testid="outside-element">Outside</div>
        <VisualCloakPopover {...defaultProps} onClose={onClose} />
      </div>
    );

    fireEvent.mouseDown(screen.getByTestId('outside-element'));
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
