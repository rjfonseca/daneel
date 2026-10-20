import { PinnedAnchor } from '../src/gen/api/v1/graph_pb';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, act } from '@testing-library/react';
import { BottomTemporalHotbar } from '../src/components/galaxy/BottomTemporalHotbar';
import type { GalaxyNode } from '../src/types/galaxy';

describe('BottomTemporalHotbar Component', () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
    vi.useRealTimers();
    localStorage.clear();
  });

  const mockToday = new Date();
  const year = mockToday.getFullYear();
  const month = String(mockToday.getMonth() + 1).padStart(2, '0');
  const day = String(mockToday.getDate()).padStart(2, '0');
  const todayStr = `${year}-${month}-${day}`;

  const sampleNodes: GalaxyNode[] = [
    {
      id: `daily/${todayStr}.md`,
      title: todayStr,
      category: 'daily',
    },
    {
      id: 'daily/2026-01-01.md',
      title: 'Old Note',
      category: 'daily',
    }
  ];

  it('renders the Temporal Relay micro-header and 7 day pills, with no hardcoded weekly button', () => {
    render(
      <BottomTemporalHotbar
        nodes={sampleNodes}
        onSelectNode={() => {}}
      />
    );

    expect(screen.getByText(/TEMPORAL/i)).toBeDefined();
    expect(screen.getByText(/RELAY/i)).toBeDefined();
    expect(screen.getByTestId('hud-day-today')).toBeDefined();
    expect(screen.queryByTestId('hud-btn-week')).toBeNull();
  });

  it('navigates weeks and returns to today without losing pinned anchors', () => {
    render(
      <BottomTemporalHotbar
        nodes={sampleNodes}
        onSelectNode={vi.fn()}
        backendAnchors={[new PinnedAnchor({ path: 'notes/architecture.md', label: 'Architecture' })]}
      />
    );
    const initialWeek = screen.getByTestId('temporal-week-label').textContent;
    fireEvent.click(screen.getByTestId('temporal-next-week'));
    expect(screen.getByTestId('temporal-week-label').textContent).not.toBe(initialWeek);
    expect(screen.getByTestId('hud-anchor-notes/architecture.md')).toBeInTheDocument();

    fireEvent.click(screen.getByTestId('temporal-today'));
    expect(screen.getByTestId('temporal-week-label').textContent).toBe(initialWeek);
    expect(screen.getByTestId('hud-anchor-notes/architecture.md')).toBeInTheDocument();
  });

  it('opens today daily note when returning to today', () => {
    const onSelectNode = vi.fn();
    const onToast = vi.fn();
    render(<BottomTemporalHotbar nodes={sampleNodes} onSelectNode={onSelectNode} onToast={onToast} />);

    fireEvent.click(screen.getByTestId('temporal-today'));

    expect(onSelectNode).toHaveBeenCalledWith(`daily/${todayStr}.md`);
    expect(onToast).toHaveBeenCalledWith('Galactic Chronometer', expect.stringContaining(`daily/${todayStr}.md`));
  });

  it('opens the selected daily note and previews its title from the date picker', () => {
    const titledNodes: GalaxyNode[] = [
      { id: 'daily/2026-10-12.md', relPath: 'daily/2026-10-12.md', title: 'Planning the week', category: 'daily' },
    ];
    const onSelectNode = vi.fn();
    const onToast = vi.fn();
    render(<BottomTemporalHotbar nodes={titledNodes} onSelectNode={onSelectNode} onToast={onToast} />);

    const datePicker = screen.getByLabelText('Choose a date to open a daily note');
    fireEvent.change(datePicker, { target: { value: '2026-10-12' } });
    expect(onSelectNode).toHaveBeenCalledWith('daily/2026-10-12.md');
    expect(onToast).toHaveBeenCalledWith('Galactic Chronometer', expect.stringContaining('daily/2026-10-12.md'));
    expect(screen.getByTestId('temporal-selected-day-summary')).toHaveTextContent('Planning the week');
    expect(screen.getByTestId('hud-day-mon')).toHaveAttribute('title', expect.stringContaining('Planning the week'));
  });

  it('reports a missing selected daily note without opening or creating one', () => {
    const onSelectNode = vi.fn();
    const onToast = vi.fn();
    render(<BottomTemporalHotbar nodes={[]} onSelectNode={onSelectNode} onToast={onToast} />);

    fireEvent.change(screen.getByLabelText('Choose a date to open a daily note'), { target: { value: '2026-02-28' } });

    expect(onSelectNode).not.toHaveBeenCalled();
    expect(onToast).toHaveBeenCalledWith('Galactic Chronometer', expect.stringContaining('daily/2026-02-28.md'));
    expect(screen.getByTestId('temporal-selected-day-summary')).toHaveTextContent('2026-02-28');
    expect(screen.getByTestId('temporal-selected-day-summary')).toHaveTextContent('No daily note');
  });

  it('shows a readable date control with a visible English label', () => {
    render(<BottomTemporalHotbar nodes={sampleNodes} onSelectNode={vi.fn()} />);

    expect(screen.getByText('OPEN DATE')).toBeVisible();
    expect(screen.getByLabelText('Choose a date to open a daily note')).toHaveClass('w-40', 'min-h-9');
  });

  it('selects today node when clicking today pill', () => {
    const onSelectNode = vi.fn();
    const onToast = vi.fn();

    render(
      <BottomTemporalHotbar
        nodes={sampleNodes}
        onSelectNode={onSelectNode}
        onToast={onToast}
      />
    );

    const todayBtn = screen.getByTestId('hud-day-today');
    fireEvent.click(todayBtn);

    expect(onSelectNode).toHaveBeenCalledWith(`daily/${todayStr}.md`);
  });

  it('renders pinned anchors and displays overflow menu when exceeding slot limit of 3', () => {
    const onSelectNode = vi.fn();
    const backendAnchors = [
      { path: 'notes/architecture.md', label: 'Architecture', icon: '🏛️' },
      { path: 'notes/roadmap.md', label: 'Roadmap', icon: '🗺️' },
      { path: 'notes/rfc-01.md', label: 'RFC 01', icon: '📜' },
      { path: 'notes/rfc-02.md', label: 'RFC 02', icon: '📄' },
    ];

    render(
      <BottomTemporalHotbar
        nodes={sampleNodes}
        onSelectNode={onSelectNode}
        backendAnchors={backendAnchors.map(anchor => new PinnedAnchor(anchor))}
      />
    );

    // First three anchors visible
    expect(screen.getByTestId('hud-anchor-notes/architecture.md')).toBeDefined();
    expect(screen.getByTestId('hud-anchor-notes/roadmap.md')).toBeDefined();
    expect(screen.getByTestId('hud-anchor-notes/rfc-01.md')).toBeDefined();

    // Overflow button exists for 4th anchor
    const overflowBtn = screen.getByTestId('hud-anchors-overflow-btn');
    expect(overflowBtn).toBeDefined();
    expect(overflowBtn.textContent).toContain('+1');

    // Click overflow button to view popover
    fireEvent.click(overflowBtn);
    expect(screen.getByText(/SELDON VAULT ANCHORS/i)).toBeDefined();
  });

  it('handles global keyboard shortcuts 1 (today) and 2 (first pinned anchor)', () => {
    const onSelectNode = vi.fn();
    const backendAnchors = [
      { path: 'notes/architecture.md', label: 'Architecture', icon: '🏛️' },
    ];

    render(
      <BottomTemporalHotbar
        nodes={sampleNodes}
        onSelectNode={onSelectNode}
        backendAnchors={backendAnchors.map(anchor => new PinnedAnchor(anchor))}
      />
    );

    // Press '1' -> selects today
    fireEvent.keyDown(window, { key: '1' });
    expect(onSelectNode).toHaveBeenCalledWith(`daily/${todayStr}.md`);

    // Press '2' -> selects first anchor
    fireEvent.keyDown(window, { key: '2' });
    expect(onSelectNode).toHaveBeenCalledWith('notes/architecture.md');
  });

  it('does not trigger shortcuts when typing inside an input element', () => {
    const onSelectNode = vi.fn();
    const onFocusWeek = vi.fn();

    render(
      <div>
        <input data-testid="search-input" />
        <BottomTemporalHotbar
          nodes={sampleNodes}
          onSelectNode={onSelectNode}
          onFocusWeek={onFocusWeek}
        />
      </div>
    );

    const input = screen.getByTestId('search-input');
    input.focus();

    fireEvent.keyDown(input, { key: '1' });
    fireEvent.keyDown(input, { key: '2' });

    expect(onSelectNode).not.toHaveBeenCalled();
    expect(onFocusWeek).not.toHaveBeenCalled();
  });

  it('does not trigger hotbar shortcuts while a select control has focus', () => {
    const onSelectNode = vi.fn();
    render(
      <div>
        <select data-testid="review-focus-select" aria-label="Review focus">
          <option value="notes/ada.md">Ada</option>
        </select>
        <BottomTemporalHotbar nodes={sampleNodes} onSelectNode={onSelectNode} />
      </div>,
    );

    const select = screen.getByTestId('review-focus-select');
    select.focus();
    fireEvent.keyDown(select, { key: '1' });
    fireEvent.keyDown(select, { key: '2' });

    expect(onSelectNode).not.toHaveBeenCalled();
  });

  it('hides when isMaximized is true', () => {
    const { container } = render(
      <BottomTemporalHotbar
        nodes={sampleNodes}
        onSelectNode={vi.fn()}
        isMaximized={true}
      />
    );

    expect(container.firstChild).toBeNull();
  });

  it('centers beside a controlled drawer width on desktop', () => {
    render(<BottomTemporalHotbar nodes={sampleNodes} onSelectNode={vi.fn()} hasActiveDrawer drawerWidth={600} />);
    const hotbar = screen.getByTestId('bottom-temporal-hotbar');
    expect(hotbar.style.getPropertyValue('--drawer-width')).toBe('600px');
    expect(hotbar.className).toContain('lg:left-[calc((100vw-var(--drawer-width))/2)]');
  });

  it('does not match non-daily notes whose id or relPath contains the date string', () => {
    const onSelectNode = vi.fn();
    const onToast = vi.fn();

    // Node that contains todayStr in path but is not daily/todayStr(.md)
    const looseNodes: GalaxyNode[] = [
      {
        id: `notes/meeting-${todayStr}-summary.md`,
        relPath: `notes/meeting-${todayStr}-summary.md`,
        title: 'Meeting Summary',
        category: 'meeting',
      },
    ];

    render(
      <BottomTemporalHotbar
        nodes={looseNodes}
        onSelectNode={onSelectNode}
        onToast={onToast}
      />
    );

    const todayBtn = screen.getByTestId('hud-day-today');
    fireEvent.click(todayBtn);

    // Should NOT select the loose note
    expect(onSelectNode).not.toHaveBeenCalled();
    expect(onToast).toHaveBeenCalledWith(
      'Galactic Chronometer',
      expect.stringContaining('No daily note found for')
    );
  });

  it('updates Today and the highlighted day when local midnight passes', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 9, 18, 23, 59, 30));
    const nextDayNode: GalaxyNode = { id: 'daily/2026-10-19.md', title: 'Monday log', category: 'daily' };
    const onSelectNode = vi.fn();
    render(<BottomTemporalHotbar nodes={[nextDayNode]} onSelectNode={onSelectNode} />);

    expect(screen.getByTestId('hud-day-today')).toHaveAttribute('title', expect.stringContaining('2026-10-18'));
    act(() => vi.advanceTimersByTime(31_000));

    expect(screen.getByTestId('hud-day-today')).toHaveAttribute('title', expect.stringContaining('2026-10-19'));
    fireEvent.click(screen.getByTestId('temporal-today'));
    expect(screen.getByTestId('temporal-selected-day-summary')).toHaveTextContent('2026-10-19');
  });

  it('refreshes Today on tab return and shortcut while retaining a manually selected historical week', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 9, 18, 23, 59, 30));
    const nextDayNode: GalaxyNode = { id: 'daily/2026-10-19.md', title: 'Monday log', category: 'daily' };
    const onSelectNode = vi.fn();
    render(<BottomTemporalHotbar nodes={[nextDayNode]} onSelectNode={onSelectNode} />);

    fireEvent.click(screen.getByTestId('temporal-previous-week'));
    const historicalWeek = screen.getByTestId('temporal-week-label').textContent;
    vi.setSystemTime(new Date(2026, 9, 19, 0, 1));
    fireEvent(window, new Event('focus'));

    expect(screen.getByTestId('temporal-week-label')).toHaveTextContent(historicalWeek || '');
    expect(screen.queryByTestId('hud-day-today')).toBeNull();
    fireEvent.keyDown(window, { key: '1' });
    expect(onSelectNode).toHaveBeenCalledWith('daily/2026-10-19.md');
  });
});
