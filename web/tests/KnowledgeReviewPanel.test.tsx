import { fireEvent, render, screen, within } from '@testing-library/react';
import { beforeEach, describe, expect, it } from 'vitest';
import { KnowledgeReviewPanel } from '../src/components/KnowledgeReviewPanel';
import type { GalaxyLink, GalaxyNode } from '../src/types/galaxy';

const nodes: GalaxyNode[] = [
  { id: 'notes/focus.md', title: 'Focus' },
  { id: 'notes/candidate.md', title: 'Candidate' },
  { id: 'notes/shared.md', title: 'Shared note' },
  { id: 'notes/orphan.md', title: 'Orphan' },
  { id: 'skill:build', title: 'Build', ring: 'skills' },
];
const links: GalaxyLink[] = [
  { source: 'notes/focus.md', target: 'notes/shared.md' },
  { source: 'notes/candidate.md', target: 'notes/shared.md' },
];
const props = {
  isOpen: true,
  onClose: () => {},
  nodes,
  links,
  unindexedLinks: [{ source: 'notes/focus.md', target: 'notes/not-indexed.md' }],
  onSelectNode: () => {},
};

describe('KnowledgeReviewPanel', () => {
  beforeEach(() => localStorage.clear());

  it('shows structural suggestions with the reason and review-only actions', () => {
    render(<KnowledgeReviewPanel {...props} />);
    fireEvent.click(screen.getByRole('tab', { name: /suggested connections/i }));
    fireEvent.change(screen.getByRole('combobox', { name: 'Focus note' }), { target: { value: 'notes/focus.md' } });
    expect(screen.getByText('Both notes share neighbors: Shared note.')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /save suggestion candidate/i }));
    expect(screen.getByText(/Saved for review/)).toBeInTheDocument();
    expect(screen.getByText(/No notes are edited/)).toBeInTheDocument();
    expect(JSON.parse(localStorage.getItem('daneel:knowledge_review_decisions') || '[]')).toEqual([
      { focusId: 'notes/focus.md', candidateId: 'notes/candidate.md', decision: 'accepted' },
    ]);
  });

  it('keeps ignored candidates out after reopen and offers reset', () => {
    const first = render(<KnowledgeReviewPanel {...props} />);
    fireEvent.click(screen.getByRole('tab', { name: /suggested connections/i }));
    fireEvent.change(screen.getByRole('combobox', { name: 'Focus note' }), { target: { value: 'notes/focus.md' } });
    fireEvent.click(screen.getByRole('button', { name: /ignore candidate/i }));
    first.unmount();
    render(<KnowledgeReviewPanel {...props} />);
    fireEvent.click(screen.getByRole('tab', { name: /suggested connections/i }));
    fireEvent.change(screen.getByRole('combobox', { name: 'Focus note' }), { target: { value: 'notes/focus.md' } });
    expect(screen.queryByRole('button', { name: /save suggestion candidate/i })).not.toBeInTheDocument();
    const review = screen.getByLabelText('Reviewed suggestions');
    fireEvent.click(within(review).getByRole('button', { name: /reset decision for candidate/i }));
    expect(screen.getByRole('button', { name: /save suggestion candidate/i })).toBeInTheDocument();
  });

  it('describes unindexed targets accurately and opens only the indexed source', () => {
    const onSelectNode = vi.fn();
    render(<KnowledgeReviewPanel {...props} onSelectNode={onSelectNode} />);
    fireEvent.click(screen.getByRole('tab', { name: /unindexed targets/i }));
    expect(screen.getByText(/not present in the current index/i)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /open source focus/i }));
    expect(onSelectNode).toHaveBeenCalledWith('notes/focus.md');
  });

  it('retains a selected note beyond the first 50 options when search is cleared', () => {
    const manyNodes = Array.from({ length: 60 }, (_, index) => ({ id: `notes/note-${index}.md`, title: `Note ${String(index).padStart(2, '0')}` }));
    render(<KnowledgeReviewPanel {...props} nodes={manyNodes} links={[]} />);
    fireEvent.click(screen.getByRole('tab', { name: /suggested connections/i }));
    fireEvent.change(screen.getByRole('searchbox', { name: 'Search focus note' }), { target: { value: 'Note 59' } });
    const select = screen.getByRole('combobox', { name: 'Focus note' }) as HTMLSelectElement;
    fireEvent.change(select, { target: { value: 'notes/note-59.md' } });
    fireEvent.change(screen.getByRole('searchbox', { name: 'Search focus note' }), { target: { value: '' } });
    expect(select.value).toBe('notes/note-59.md');
    expect(within(select).getByRole('option', { name: 'Note 59' })).toBeInTheDocument();
  });

  it('keeps orphan actions as buttons inside list items', () => {
    render(<KnowledgeReviewPanel {...props} />);
    const orphan = screen.getByText('Orphan').closest('[role="listitem"]');
    expect(orphan).not.toBeNull();
    expect(within(orphan as HTMLElement).getByRole('button', { name: /Orphan/ })).toBeInTheDocument();
  });
});
