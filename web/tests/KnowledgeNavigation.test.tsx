import { fireEvent, render, screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { KnowledgeExplorer } from '../src/components/KnowledgeExplorer';
import { NavigationToolbar, type NavigationToolbarProps } from '../src/components/NavigationToolbar';
import { useState } from 'react';
import type { HeaderDisclosure } from '../src/lib/headerLayout';

function renderToolbar(props: Omit<NavigationToolbarProps, 'disclosure' | 'onDisclosureChange' | 'layout'>) {
  function Harness() {
    const [disclosure, setDisclosure] = useState<HeaderDisclosure>(null);
    return <NavigationToolbar {...props} disclosure={disclosure} onDisclosureChange={setDisclosure} layout="full" />;
  }
  return render(<Harness />);
}

describe('knowledge navigation', () => {
  it('filters the scoped list and opens a note through the shared selection action', () => {
    const select = vi.fn();
    render(<KnowledgeExplorer
      scopeLabel="Constellation"
      nodes={[
        { id: 'notes/a.md', title: 'Alpha', category: 'notes/topic', modTime: 1_700_000_000, connections: 3 },
        { id: 'notes/b.md', title: 'Beta', category: 'notes/topic', connections: 1 },
      ]}
      onSelectNode={select}
      onClose={() => {}}
    />);
    expect(screen.getByText('Constellation · 2 notes')).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Filter notes'), { target: { value: 'beta' } });
    expect(screen.queryByText('Alpha')).not.toBeInTheDocument();
    fireEvent.click(screen.getByText('Beta'));
    expect(select).toHaveBeenCalledWith('notes/b.md');
  });

  it('renders notes in bounded batches while searching the entire scope', () => {
    const many = Array.from({ length: 125 }, (_, index) => ({ id: `notes/note-${index}.md`, title: `Note ${index}`, category: 'notes/topic' }));
    const select = vi.fn();
    const { rerender } = render(<KnowledgeExplorer scopeLabel="Galaxy" nodes={many} onSelectNode={select} onClose={() => {}} />);
    const list = screen.getByRole('list');
    expect(within(list).getAllByRole('listitem')).toHaveLength(50);
    expect(screen.getByText('Showing 50 of 125')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Show more notes' }));
    expect(within(list).getAllByRole('listitem')).toHaveLength(100);
    fireEvent.change(screen.getByLabelText('Filter notes'), { target: { value: 'Note 124' } });
    expect(within(list).getAllByRole('listitem')).toHaveLength(1);
    fireEvent.click(screen.getByRole('button', { name: 'Note 124' }));
    expect(select).toHaveBeenCalledWith('notes/note-124.md');
    fireEvent.change(screen.getByLabelText('Filter notes'), { target: { value: '' } });
    expect(within(list).getAllByRole('listitem')).toHaveLength(50);
    rerender(<KnowledgeExplorer scopeLabel="Sector" nodes={many.slice(0, 70)} onSelectNode={select} onClose={() => {}} />);
    expect(screen.getByText('Sector · 70 notes')).toBeInTheDocument();
    expect(within(list).getAllByRole('listitem')).toHaveLength(50);
  });

  it('keeps navigation actions reachable and runs callbacks once', () => {
    const back = vi.fn(); const recent = vi.fn(); const saved = vi.fn();
    renderToolbar({ canBack: true, canForward: false, onBack: back, onForward: vi.fn(), isExplorerOpen: false,
      onToggleExplorer: vi.fn(), recentNotes: [{ id: 'notes/a.md', title: 'Alpha' }], savedSearches: [{ query: 'architecture' }],
      searchQuery: 'architecture', onSelectRecent: recent, onSelectSavedSearch: saved, onSaveSearch: vi.fn(),
      onRemoveSavedSearch: vi.fn() });
    fireEvent.click(screen.getByRole('button', { name: 'Back' }));
    expect(back).toHaveBeenCalledOnce();
    expect(screen.getByRole('button', { name: 'Forward' })).toBeDisabled();
    fireEvent.click(screen.getByRole('button', { name: 'Recent' }));
    fireEvent.click(screen.getByRole('button', { name: 'Alpha' }));
    expect(recent).toHaveBeenCalledExactlyOnceWith('notes/a.md');
    fireEvent.click(screen.getByRole('button', { name: 'Saved' }));
    fireEvent.click(screen.getByRole('button', { name: 'architecture' }));
    expect(saved).toHaveBeenCalledExactlyOnceWith('architecture');
  });

  it('renders recent content from the shared controlled disclosure state', () => {
    render(<NavigationToolbar canBack={false} canForward={false} onBack={vi.fn()} onForward={vi.fn()} isExplorerOpen={false}
      onToggleExplorer={vi.fn()} recentNotes={[{ id: 'notes/a.md', title: 'Alpha' }]} savedSearches={[]} searchQuery=""
      onSelectRecent={vi.fn()} onSelectSavedSearch={vi.fn()} onSaveSearch={vi.fn()} onRemoveSavedSearch={vi.fn()}
      disclosure="recent" onDisclosureChange={vi.fn()} layout="full" />);
    expect(screen.getByRole('region', { name: 'Recent notes' })).toBeVisible();
  });

  it('shows the recent disclosure from the controlled parent state', () => {
    render(<NavigationToolbar canBack={false} canForward={false} onBack={vi.fn()} onForward={vi.fn()} isExplorerOpen={false}
      onToggleExplorer={vi.fn()} recentNotes={[]} savedSearches={[]} searchQuery="" onSelectRecent={vi.fn()}
      onSelectSavedSearch={vi.fn()} onSaveSearch={vi.fn()} onRemoveSavedSearch={vi.fn()} disclosure="recent"
      onDisclosureChange={vi.fn()} layout="full" />);
    expect(screen.getByRole('region', { name: 'Recent notes' })).toBeVisible();
    expect(screen.getByText('Notes you open will appear here.')).toBeInTheDocument();
  });
});
