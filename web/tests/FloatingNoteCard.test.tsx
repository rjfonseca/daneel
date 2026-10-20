import { renderAsync } from './helpers/renderAsync';
import { describe, it, expect, vi } from 'vitest';
import { screen, fireEvent } from '@testing-library/react';
import { FloatingNoteCard } from '../src/components/FloatingNoteCard';

describe('FloatingNoteCard Component', () => {
  const mockNote = {
    id: 'notes/architecture.md',
    title: 'System Architecture',
    category: 'architecture',
    connections: 5,
  };

  it('renders note header and calls onTogglePin when pin button is clicked', async () => {
    const onTogglePin = vi.fn();

    await renderAsync(
      <FloatingNoteCard
        activeNote={mockNote}
        noteContent="# Content"
        processedMarkdown="# Content"
        isMaximized={false}
        onToggleMaximize={() => {}}
        onClose={() => {}}
        onCopy={() => {}}
        copied={false}
        onNavigateInternalLink={() => {}}
        isConstellationActive={false}
        onResetConstellation={() => {}}
        isPinned={false}
        onTogglePin={onTogglePin}
      />
    );

    const pinBtn = screen.getByTestId('toggle-pin-note');
    expect(pinBtn).toBeDefined();
    expect(pinBtn.getAttribute('aria-label')).toBe('Pin to Seldon Anchors');

    fireEvent.click(pinBtn);
    expect(onTogglePin).toHaveBeenCalled();
  });

  it('displays active state when note is pinned', async () => {
    await renderAsync(
      <FloatingNoteCard
        activeNote={mockNote}
        noteContent="# Content"
        processedMarkdown="# Content"
        isMaximized={false}
        onToggleMaximize={() => {}}
        onClose={() => {}}
        onCopy={() => {}}
        copied={false}
        onNavigateInternalLink={() => {}}
        isConstellationActive={false}
        onResetConstellation={() => {}}
        isPinned={true}
        onTogglePin={() => {}}
      />
    );

    const pinBtn = screen.getByTestId('toggle-pin-note');
    expect(pinBtn.getAttribute('aria-label')).toBe('Unpin from Seldon Anchors');
  });

  it('updates copy button aria-label and live region announcement when copied is true', async () => {
    const { rerender } = await renderAsync(
      <FloatingNoteCard
        activeNote={mockNote}
        noteContent="# Content"
        processedMarkdown="# Content"
        isMaximized={false}
        onToggleMaximize={() => {}}
        onClose={() => {}}
        onCopy={() => {}}
        copied={false}
        onNavigateInternalLink={() => {}}
        isConstellationActive={false}
        onResetConstellation={() => {}}
      />
    );

    const copyBtn = screen.getByTestId('copy-note-content');
    expect(copyBtn.getAttribute('aria-label')).toBe('Copy note content to clipboard');

    rerender(
      <FloatingNoteCard
        activeNote={mockNote}
        noteContent="# Content"
        processedMarkdown="# Content"
        isMaximized={false}
        onToggleMaximize={() => {}}
        onClose={() => {}}
        onCopy={() => {}}
        copied={true}
        onNavigateInternalLink={() => {}}
        isConstellationActive={false}
        onResetConstellation={() => {}}
      />
    );

    expect(copyBtn.getAttribute('aria-label')).toBe('Copied note content to clipboard');
    expect(screen.getByText('Note content copied to clipboard')).toBeDefined();
  });

  it('renders isolated constellation badge with correct accessibility attributes and handles reset click', async () => {
    const onResetConstellation = vi.fn();

    await renderAsync(
      <FloatingNoteCard
        activeNote={mockNote}
        noteContent="# Content"
        processedMarkdown="# Content"
        isMaximized={false}
        onToggleMaximize={() => {}}
        onClose={() => {}}
        onCopy={() => {}}
        copied={false}
        onNavigateInternalLink={() => {}}
        isConstellationActive={true}
        onResetConstellation={onResetConstellation}
        isPinned={false}
        onTogglePin={() => {}}
      />
    );

    const constellationBadge = screen.getByTestId('constellation-hud-badge');
    expect(constellationBadge).toBeDefined();
    expect(constellationBadge.getAttribute('aria-label')).toBe('Restore full galaxy view from isolated constellation');

    fireEvent.click(constellationBadge);
    expect(onResetConstellation).toHaveBeenCalledTimes(1);
  });
});
