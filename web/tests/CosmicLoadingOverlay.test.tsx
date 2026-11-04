import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import React from 'react';
import { CosmicLoadingOverlay } from '../src/components/CosmicLoadingOverlay';

describe('CosmicLoadingOverlay', () => {
  it('renders null when isReady is true', () => {
    const { container } = render(<CosmicLoadingOverlay isReady={true} />);
    expect(container.firstChild).toBeNull();
  });

  it('renders accessible loading status when isReady is false', () => {
    render(<CosmicLoadingOverlay isReady={false} />);

    const overlay = screen.getByTestId('cosmic-loading-overlay');
    expect(overlay).toBeInTheDocument();
    expect(overlay).toHaveAttribute('role', 'status');
    expect(overlay).toHaveAttribute('aria-live', 'polite');

    expect(screen.getByText(/Synthesizing Artificial Gravity/i)).toBeInTheDocument();
    expect(screen.getByText(/Stabilizing Seldon perimeter/i)).toBeInTheDocument();
  });

  it('respects prefers-reduced-motion with motion-reduce classes', () => {
    const { container } = render(<CosmicLoadingOverlay isReady={false} />);

    const spinner = container.querySelector('.animate-spin');
    expect(spinner).toHaveClass('motion-reduce:animate-none');

    const heading = screen.getByText(/Synthesizing Artificial Gravity/i);
    expect(heading).toHaveClass('motion-reduce:animate-none');
  });
});
