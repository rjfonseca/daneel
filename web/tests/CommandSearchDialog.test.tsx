import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { CommandSearchDialog } from '../src/components/CommandSearchDialog';
import type { SearchResultItem } from '../src/types/galaxy';

describe('CommandSearchDialog Component', () => {
  const mockResults: SearchResultItem[] = [
    {
      id: 'note-1',
      relPath: 'notes/foundation.md',
      title: 'Foundation Protocol',
      snippet: 'Overview of <b>Foundation</b> protocol.',
      isArms: false,
      isCat: false,
    },
    {
      id: 'station-1',
      title: 'Terminus Station',
      snippet: 'Primary <b>Terminus</b> station node.',
      isArms: true,
      isCat: false,
    },
  ];

  it('does not render when isOpen is false', () => {
    const { container } = render(
      <CommandSearchDialog
        isOpen={false}
        onClose={vi.fn()}
        searchQuery=""
        onSearchChange={vi.fn()}
        activeFilter="All"
        onSelectFilter={vi.fn()}
        filteredResults={[]}
        onSelectResult={vi.fn()}
      />
    );
    expect(container.firstChild).toBeNull();
  });

  it('provides an accessible modal boundary for keyboard search', () => {
    render(<CommandSearchDialog isOpen onClose={vi.fn()} searchQuery="" onSearchChange={vi.fn()} activeFilter="All" onSelectFilter={vi.fn()} filteredResults={[]} onSelectResult={vi.fn()} />);
    expect(screen.getByRole('dialog', { name: 'Search notes' })).toHaveAttribute('aria-modal', 'true');
  });

  it('exposes the search filter group and active category', () => {
    render(
      <CommandSearchDialog
        isOpen={true}
        onClose={vi.fn()}
        searchQuery="foundation"
        onSearchChange={vi.fn()}
        activeFilter="Architecture"
        onSelectFilter={vi.fn()}
        filteredResults={mockResults}
        onSelectResult={vi.fn()}
      />
    );

    const filterGroup = screen.getByRole('group', { name: 'Filter search results by category' });
    expect(filterGroup).toBeDefined();

    const activeFilterBtn = screen.getByRole('button', { name: 'Architecture' });
    expect(activeFilterBtn.getAttribute('aria-pressed')).toBe('true');

    const inactiveFilterBtn = screen.getByRole('button', { name: 'People' });
    expect(inactiveFilterBtn.getAttribute('aria-pressed')).toBe('false');
  });

  it('calls onSelectFilter when a filter button is clicked', () => {
    const onSelectFilter = vi.fn();
    render(
      <CommandSearchDialog
        isOpen={true}
        onClose={vi.fn()}
        searchQuery=""
        onSearchChange={vi.fn()}
        activeFilter="All"
        onSelectFilter={onSelectFilter}
        filteredResults={[]}
        onSelectResult={vi.fn()}
      />
    );

    fireEvent.click(screen.getByRole('button', { name: 'People' }));
    expect(onSelectFilter).toHaveBeenCalledWith('People');
  });

  it('calls onSelectResult when a search item is clicked', () => {
    const onSelectResult = vi.fn();
    render(
      <CommandSearchDialog
        isOpen={true}
        onClose={vi.fn()}
        searchQuery="foundation"
        onSearchChange={vi.fn()}
        activeFilter="All"
        onSelectFilter={vi.fn()}
        filteredResults={mockResults}
        onSelectResult={onSelectResult}
      />
    );

    const firstItem = screen.getByText('Foundation Protocol');
    fireEvent.click(firstItem);
    expect(onSelectResult).toHaveBeenCalledWith(mockResults[0]);
  });

  it('renders safe highlighted snippets and result paths under type groups', () => {
    render(
      <CommandSearchDialog
        isOpen={true}
        onClose={vi.fn()}
        searchQuery="AI"
        onSearchChange={vi.fn()}
        activeFilter="All"
        onSelectFilter={vi.fn()}
        filteredResults={[
          { relPath: 'notes/ai.md', title: 'AI notes', snippet: '## ==AI== <img src=x onerror=alert(1)>' },
          { id: 'routine:ai', title: 'AI routine', snippet: 'Run ==AI== automation', category: 'arms.routines', isArms: true },
          { relPath: '__CAT_ai', title: 'AI cluster', snippet: 'Cluster summary', isCat: true },
        ]}
        onSelectResult={vi.fn()}
      />
    );

    expect(screen.getAllByText('AI', { selector: 'mark' })).toHaveLength(2);
    expect(screen.queryByRole('img')).toBeNull();
    expect(screen.getByText('notes/ai.md')).toBeDefined();
    expect(screen.getByText('Notes')).toBeDefined();
    expect(screen.getByText('Clusters')).toBeDefined();
    expect(screen.getByText('Automations')).toBeDefined();
  });

  it('shows loading, error and empty states explicitly', () => {
    const { rerender } = render(
      <CommandSearchDialog isOpen onClose={vi.fn()} searchQuery="AI" onSearchChange={vi.fn()} activeFilter="All" onSelectFilter={vi.fn()} filteredResults={[]} onSelectResult={vi.fn()} isLoading />
    );
    expect(screen.getByRole('status').textContent).toContain('Searching');

    rerender(
      <CommandSearchDialog isOpen onClose={vi.fn()} searchQuery="AI" onSearchChange={vi.fn()} activeFilter="All" onSelectFilter={vi.fn()} filteredResults={[]} onSelectResult={vi.fn()} searchError="Search is unavailable." />
    );
    expect(screen.getByRole('alert').textContent).toContain('Search is unavailable.');

    rerender(
      <CommandSearchDialog isOpen onClose={vi.fn()} searchQuery="AI" onSearchChange={vi.fn()} activeFilter="All" onSelectFilter={vi.fn()} filteredResults={[]} onSelectResult={vi.fn()} hasSearched />
    );
    expect(screen.getByText('No results found.')).toBeDefined();
  });

  it('reveals more than eight results on demand', () => {
    const results = Array.from({ length: 10 }, (_, idx) => ({ relPath: `notes/${idx}.md`, title: `Note ${idx}`, snippet: 'Result' }));
    render(
      <CommandSearchDialog isOpen onClose={vi.fn()} searchQuery="note" onSearchChange={vi.fn()} activeFilter="All" onSelectFilter={vi.fn()} filteredResults={results} onSelectResult={vi.fn()} />
    );

    expect(screen.getByText('Note 7')).toBeDefined();
    expect(screen.queryByText('Note 8')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Show more results' }));
    expect(screen.getByText('Note 8')).toBeDefined();
    expect(screen.getByText('Note 9')).toBeDefined();
  });
});
