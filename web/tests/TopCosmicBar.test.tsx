import { GalaxyStats } from '../src/gen/api/v1/graph_pb';
import { describe, it, expect, vi } from 'vitest';
import { act, render, screen, fireEvent } from '@testing-library/react';
import React, { useState } from 'react';
import { TopCosmicBar, type TopCosmicBarProps } from '../src/components/TopCosmicBar';
import type { HeaderDisclosure } from '../src/lib/headerLayout';

const navigation: TopCosmicBarProps['navigation'] = {
  canBack: false, canForward: false, onBack: vi.fn(), onForward: vi.fn(), isExplorerOpen: false,
  onToggleExplorer: vi.fn(), recentNotes: [], savedSearches: [], searchQuery: '',
  onSelectRecent: vi.fn(), onSelectSavedSearch: vi.fn(), onSaveSearch: vi.fn(), onRemoveSavedSearch: vi.fn(),
  onOpenCapture: vi.fn(), onOpenReview: vi.fn(),
};
function ControlledHeader(props: Partial<TopCosmicBarProps> = {}) {
  const [disclosure, setDisclosure] = useState<HeaderDisclosure>(null);
  return <TopCosmicBar {...props} navigation={navigation} disclosure={disclosure} onDisclosureChange={setDisclosure} onResetCamera={props.onResetCamera || vi.fn()} onOpenSearch={props.onOpenSearch || vi.fn()} />;
}

describe('TopCosmicBar', () => {
  it('keeps desktop Capture beside Search and before View', () => {
    const originalWidth = window.innerWidth;
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 1440 });
    try {
      render(<ControlledHeader onToggleControls={vi.fn()} />);
      const search = screen.getByTestId('quick-search-btn');
      const capture = screen.getByTestId('capture-note-btn');
      const view = screen.getByTestId('toggle-controls-btn');
      expect(search.compareDocumentPosition(capture) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
      expect(capture.compareDocumentPosition(view) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
      const calls = vi.mocked(navigation.onOpenCapture!).mock.calls.length;
      fireEvent.click(capture);
      expect(navigation.onOpenCapture).toHaveBeenCalledTimes(calls + 1);
    } finally {
      Object.defineProperty(window, 'innerWidth', { configurable: true, value: originalWidth });
    }
  });
  it('keeps the breadcrumb ancestors operable inside the unified header', () => {
    const reset = vi.fn(); const select = vi.fn();
    render(<ControlledHeader onResetCamera={reset} onSelectSector={select} activeSectorName="PLATFORM" activeClusterName="GATEWAY" />);
    expect(screen.getByTestId('command-bridge')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Galaxy' }));
    fireEvent.click(screen.getByRole('button', { name: 'Sector: PLATFORM' }));
    expect(reset).toHaveBeenCalledOnce(); expect(select).toHaveBeenCalledOnce();
  });

  it('preserves real zero and unavailable statistics in the statistics disclosure', () => {
    render(<ControlledHeader stats={new GalaxyStats({ totalNotes: 0, totalLinks: 0, harvesterCount: 0 })} />);
    fireEvent.click(screen.getByRole('button', { name: 'More actions' }));
    fireEvent.click(screen.getByRole('button', { name: 'Galaxy statistics' }));
    const stats = screen.getByRole('region', { name: 'Galaxy statistics' });
    expect(stats).toHaveTextContent('Stars0');
    expect(stats).toHaveTextContent('Filaments0');
    expect(stats).toHaveTextContent('Harvesters0');
    expect(stats).toBeVisible();
  });

  it('keeps long neutral breadcrumbs and large or unavailable telemetry descriptive', () => {
    const originalWidth = window.innerWidth;
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 1440 });
    const first = render(<ControlledHeader
      activeSectorName="notes/engineering/systems-and-operations"
      activeClusterName="distributed-cache-and-storage-architecture"
      stats={new GalaxyStats({ totalNotes: 125000, totalLinks: 999999, harvesterCount: 2048 })}
    />);
    expect(screen.getByRole('button', { name: 'Sector: notes/engineering/systems-and-operations' })).toHaveAttribute('title', 'Sector: notes/engineering/systems-and-operations');
    expect(screen.getByText('distributed-cache-and-storage-architecture')).toHaveAttribute('title', 'Nebula / Cluster: distributed-cache-and-storage-architecture');
    expect(screen.getByTestId('cosmic-stat-notes')).toHaveTextContent('125000');
    expect(screen.getByTestId('cosmic-stat-links')).toHaveTextContent('999999');
    expect(screen.getByTestId('cosmic-stat-harvesters')).toHaveTextContent('2048');

    first.unmount();
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 1024 });
    try {
      render(<ControlledHeader stats={undefined} />);
      fireEvent.click(screen.getByRole('button', { name: 'More actions' }));
      fireEvent.click(screen.getByRole('button', { name: 'Galaxy statistics' }));
      expect(screen.getByRole('region', { name: 'Galaxy statistics' })).toHaveTextContent('Stars—');
      expect(screen.getByRole('region', { name: 'Galaxy statistics' })).toHaveTextContent('Filaments—');
      expect(screen.getByRole('region', { name: 'Galaxy statistics' })).toHaveTextContent('Harvesters—');
    } finally {
      Object.defineProperty(window, 'innerWidth', { configurable: true, value: originalWidth });
    }
  });

  it('uses Ctrl+K and Cmd+K labels on their respective platforms', () => {
    vi.stubGlobal('navigator', { platform: 'Win32' });
    const { rerender } = render(<ControlledHeader />);
    expect(screen.getByTestId('quick-search-btn')).toHaveAccessibleName('Global search and command palette (Ctrl+K)');
    vi.stubGlobal('navigator', { platform: 'MacIntel' });
    rerender(<ControlledHeader />);
    expect(screen.getByTestId('quick-search-btn')).toHaveAccessibleName('Global search and command palette (Cmd+K)');
    vi.unstubAllGlobals();
  });

  it('opens the actual View disclosure and invokes search, center, cockpit, and shortcut callbacks once', () => {
    const search = vi.fn(); const reset = vi.fn(); const cockpit = vi.fn(); const shortcuts = vi.fn();
    render(<ControlledHeader onOpenSearch={search} onResetCamera={reset} onToggleCockpit={cockpit} onOpenShortcuts={shortcuts} />);
    fireEvent.click(screen.getByTestId('quick-search-btn'));
    fireEvent.click(screen.getByTestId('reset-camera-btn'));
    fireEvent.click(screen.getByRole('button', { name: 'More actions' }));
    fireEvent.click(screen.getByRole('button', { name: /Cockpit/ }));
    fireEvent.click(screen.getByRole('button', { name: 'More actions' }));
    fireEvent.click(screen.getByRole('button', { name: /keyboard shortcuts/i }));
    expect(search).toHaveBeenCalledOnce(); expect(reset).toHaveBeenCalledOnce();
    expect(cockpit).toHaveBeenCalledOnce(); expect(shortcuts).toHaveBeenCalledOnce();
  });

  it('dismisses More to Recent on Escape and restores focus to More', () => {
    render(<ControlledHeader />);
    const more = screen.getByRole('button', { name: 'More actions' });
    fireEvent.click(more);
    fireEvent.click(screen.getByRole('button', { name: 'Recent' }));
    expect(screen.getByRole('region', { name: 'Recent notes' })).toBeVisible();
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(screen.queryByRole('region', { name: 'Recent notes' })).not.toBeInTheDocument();
    expect(more).toHaveFocus();
  });

  it('dismisses a disclosure outside the header without stealing the clicked target focus', () => {
    render(<><ControlledHeader /><button type="button">Outside action</button></>);
    fireEvent.click(screen.getByRole('button', { name: 'More actions' }));
    const outside = screen.getByRole('button', { name: 'Outside action' });
    fireEvent.pointerDown(outside);
    outside.focus();
    expect(screen.queryByRole('region', { name: 'More actions' })).not.toBeInTheDocument();
    expect(outside).toHaveFocus();
  });

  it('dismisses disclosures when crossing the shared responsive breakpoints', async () => {
    const originalWidth = window.innerWidth;
    try {
      Object.defineProperty(window, 'innerWidth', { configurable: true, value: 1024 });
      const { unmount } = render(<ControlledHeader />);
      fireEvent.click(screen.getByRole('button', { name: 'More actions' }));
      expect(screen.getByRole('region', { name: 'More actions' })).toBeVisible();
      Object.defineProperty(window, 'innerWidth', { configurable: true, value: 767 });
      act(() => window.dispatchEvent(new Event('resize')));
      await act(async () => { await new Promise(resolve => window.setTimeout(resolve, 25)); });
      expect(screen.queryByRole('region', { name: 'More actions' })).not.toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'More actions' })).toHaveFocus();
      unmount();
    } finally {
      Object.defineProperty(window, 'innerWidth', { configurable: true, value: originalWidth });
    }
  });

  it('publishes the measured header boundary and disconnects its resize observer', () => {
    let onResize: ResizeObserverCallback | undefined;
    let disconnected = false;
    class MockResizeObserver {
      constructor(callback: ResizeObserverCallback) { onResize = callback; }
      observe() {}
      unobserve() {}
      disconnect() { disconnected = true; }
    }
    const originalObserver = globalThis.ResizeObserver;
    Object.defineProperty(globalThis, 'ResizeObserver', { configurable: true, value: MockResizeObserver });
    let bottom = 116;
    const geometry = vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (this: HTMLElement) {
      if (this.dataset.testid === 'command-bridge') return { x: 0, y: 0, top: 0, left: 0, right: 1000, bottom, width: 1000, height: bottom, toJSON: () => ({}) } as DOMRect;
      return { x: 0, y: 0, top: 0, left: 0, right: 100, bottom: 100, width: 100, height: 100, toJSON: () => ({}) } as DOMRect;
    });
    const { unmount } = render(<div data-hud-root style={{ '--hud-content-top': '128px' } as React.CSSProperties}><ControlledHeader /></div>);
    const root = screen.getByTestId('command-bridge').parentElement;
    expect(root?.style.getPropertyValue('--hud-content-top')).toBe('124px');
    const setProperty = vi.spyOn(root!.style, 'setProperty');
    bottom = 132;
    act(() => onResize?.([], {} as ResizeObserver));
    expect(root?.style.getPropertyValue('--hud-content-top')).toBe('140px');
    act(() => onResize?.([], {} as ResizeObserver));
    expect(setProperty).toHaveBeenCalledOnce();
    unmount();
    expect(disconnected).toBe(true);
    geometry.mockRestore();
    Object.defineProperty(globalThis, 'ResizeObserver', { configurable: true, value: originalObserver });
  });
});
