import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import {
  HudCard,
  NeonBadge,
  CyberButton,
  HotkeyBadge,
  ShortcutsModal,
} from '../src/components/hud';

describe('Cosmic HUD Primitives', () => {
  afterEach(() => {
    cleanup();
  });

  describe('HudCard', () => {
    it('renders children with default glass styles', () => {
      render(<HudCard data-testid="test-card">Card Content</HudCard>);
      const el = screen.getByTestId('test-card');
      expect(el).toBeDefined();
      expect(el.textContent).toBe('Card Content');
      expect(el.className).toContain('backdrop-blur-hud');
      expect(el.className).toContain('rounded-hud-lg');
    });

    it('applies variant and elevation styles', () => {
      render(
        <HudCard
          data-testid="cyan-elevated-card"
          variant="cyan"
          elevation="elevated"
          techCorners
        >
          Tech Card
        </HudCard>
      );
      const el = screen.getByTestId('cyan-elevated-card');
      expect(el.className).toContain('border-cyan-500/30');
      expect(el.className).toContain('tech-corners');
      expect(el.className).toContain('backdrop-blur-deep');
    });
  });

  describe('NeonBadge', () => {
    it('renders with semantic accent styling', () => {
      render(<NeonBadge accent="cyan" data-testid="neon-badge">QUASAR</NeonBadge>);
      const el = screen.getByTestId('neon-badge');
      expect(el.textContent).toBe('QUASAR');
      expect(el.className).toContain('bg-cyan-500/15');
      expect(el.className).toContain('text-cyan-300');
    });

    it('renders radar ping indicator when ping is true', () => {
      const { container } = render(
        <NeonBadge accent="emerald" ping>ONLINE</NeonBadge>
      );
      const pingEl = container.querySelector('.animate-ping');
      expect(pingEl).toBeDefined();
    });
  });

  describe('CyberButton', () => {
    it('renders idle and active states with proper glow', () => {
      const { rerender } = render(
        <CyberButton data-testid="cyber-btn" accent="indigo" active={false}>
          Button
        </CyberButton>
      );
      let btn = screen.getByTestId('cyber-btn');
      expect(btn.className).toContain('cosmic-focus-visible');

      rerender(
        <CyberButton data-testid="cyber-btn" accent="indigo" active={true}>
          Button
        </CyberButton>
      );
      btn = screen.getByTestId('cyber-btn');
      expect(btn.className).toContain('bg-indigo-600/30');
      expect(btn.className).toContain('text-indigo-200');
    });

    it('handles click events', () => {
      const onClick = vi.fn();
      render(<CyberButton onClick={onClick}>Click Me</CyberButton>);
      fireEvent.click(screen.getByText('Click Me'));
      expect(onClick).toHaveBeenCalledTimes(1);
    });
  });

  describe('HotkeyBadge', () => {
    it('renders individual key tokens inside monospace kbd element', () => {
      render(<HotkeyBadge keys={['⌘', 'K']} data-testid="hotkey" />);
      const kbd = screen.getByTestId('hotkey');
      expect(kbd.tagName).toBe('KBD');
      expect(kbd.textContent).toContain('⌘K');
      expect(kbd.className).toContain('font-mono');
    });
  });

  describe('ShortcutsModal', () => {
    it('does not render when isOpen is false', () => {
      const { container } = render(
        <ShortcutsModal isOpen={false} onClose={() => {}} />
      );
      expect(container.firstChild).toBeNull();
    });

    it('renders cheatsheet when isOpen is true', () => {
      render(
        <ShortcutsModal isOpen={true} onClose={() => {}} />
      );
      expect(screen.getByText(/COSMIC COMMAND INTERFACE/i)).toBeDefined();
      expect(screen.getByText(/Navigation & Search/i)).toBeDefined();
    });

    it('calls onClose when close button is clicked or Escape is pressed', () => {
      const onClose = vi.fn();
      render(<ShortcutsModal isOpen={true} onClose={onClose} />);
      
      const closeBtn = screen.getByLabelText(/close shortcuts dialog/i);
      fireEvent.click(closeBtn);
      expect(onClose).toHaveBeenCalledTimes(1);

      fireEvent.keyDown(window, { key: 'Escape' });
      expect(onClose).toHaveBeenCalledTimes(2);
    });
  });
});
