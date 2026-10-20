import { renderAsync } from './helpers/renderAsync';
import { fireEvent, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { FloatingNoteCard } from '../src/components/FloatingNoteCard';

const baseProps = {
  activeNote: { id: 'notes/ada.md', title: 'Ada Lovelace', category: 'notes' },
  noteContent: '',
  processedMarkdown: '',
  isMaximized: false,
  onToggleMaximize: vi.fn(),
  onClose: vi.fn(),
  onCopy: vi.fn(),
  copied: false,
  onNavigateInternalLink: vi.fn(),
  isConstellationActive: false,
  onResetConstellation: vi.fn(),
};

describe('FloatingNoteCard reader', () => {
  it('shows valid frontmatter in collapsed metadata and keeps malformed frontmatter readable', async () => {
    const { rerender } = await renderAsync(
      <FloatingNoteCard
        {...baseProps}
        noteContent={'---\ntags:\n  - mathematics\n---\n# Ada Lovelace'}
        processedMarkdown={'---\ntags:\n  - mathematics\n---\n# Ada Lovelace'}
      />,
    );

    const metadata = screen.getByText('Metadata').closest('details');
    expect(metadata?.open).toBe(false);
    fireEvent.click(screen.getByText('Metadata'));
    expect(screen.getByText('mathematics')).toBeTruthy();

    const malformed = '---\ntitle: [broken\n---\n# Visible body';
    rerender(<FloatingNoteCard {...baseProps} noteContent={malformed} processedMarkdown={malformed} />);
    expect(screen.getByText('title: [broken')).toBeTruthy();
    expect(screen.getByRole('heading', { name: 'Visible body' })).toBeTruthy();
  });

  it('renders GFM tables and suppresses the first heading when it repeats the panel title', async () => {
    await renderAsync(
      <FloatingNoteCard
        {...baseProps}
        noteContent={'# Ada Lovelace\n\n| Year | Work |\n| --- | --- |\n| 1843 | Notes |'}
        processedMarkdown={'# Ada Lovelace\n\n| Year | Work |\n| --- | --- |\n| 1843 | Notes |'}
      />,
    );

    expect(screen.queryAllByRole('heading', { name: 'Ada Lovelace' })).toHaveLength(1);
    expect(screen.getByRole('table')).toBeTruthy();
    expect(screen.getByRole('cell', { name: '1843' })).toBeTruthy();
  });

  it('offers font size controls and a local table of contents for long notes', async () => {
    const scrollTo = vi.fn();
    const note = [
      '## Overview', 'Overview text',
      '## Early work', 'Early work text',
      '## Legacy', 'Legacy text',
    ].join('\n\n');
    const { container } = await renderAsync(
      <FloatingNoteCard {...baseProps} noteContent={note} processedMarkdown={note} />,
    );
    const viewport = container.querySelector('[data-slot="scroll-area-viewport"]') as HTMLElement;
    viewport.scrollTo = scrollTo;

    fireEvent.click(screen.getByRole('button', { name: 'Increase text size' }));
    expect(screen.getByTestId('note-markdown')).toHaveAttribute('data-font-size', 'large');
    fireEvent.click(screen.getByRole('link', { name: 'Early work' }));
    expect(baseProps.onNavigateInternalLink).not.toHaveBeenCalled();
    expect(scrollTo).toHaveBeenCalled();
  });

  it('shows a table of contents for a long note with only two sections', async () => {
    const note = `## Overview\n\n${'A long note paragraph. '.repeat(60)}\n\n## Details\n\nDetails.`;
    await renderAsync(<FloatingNoteCard {...baseProps} noteContent={note} processedMarkdown={note} />);

    expect(screen.getByRole('navigation', { name: 'Table of contents' })).toBeTruthy();
  });

  it('shows incoming and outgoing relationships with their labels and link counts', async () => {
    await renderAsync(
      <FloatingNoteCard
        {...baseProps}
        incomingLinks={[{ id: 'notes/grace.md', title: 'Grace Hopper' }]}
        outgoingLinks={[{ id: 'notes/turing.md', title: 'Alan Turing' }]}
      />,
    );

    expect(screen.getByText('Incoming links (1)')).toBeTruthy();
    expect(screen.getByText('Outgoing links (1)')).toBeTruthy();
  });

  it('uses controlled drawer width and exposes an accessible resize handle', async () => {
    const onDrawerWidthChange = vi.fn();
    await renderAsync(
      <FloatingNoteCard
        {...baseProps}
        drawerWidth={520}
        onDrawerWidthChange={onDrawerWidthChange}
      />,
    );

    const handle = screen.getByRole('separator', { name: 'Resize note panel' });
    expect(handle.getAttribute('aria-valuenow')).toBe('520');
    fireEvent.keyDown(handle, { key: 'ArrowLeft' });
    expect(onDrawerWidthChange).toHaveBeenCalledWith(504);
  });

  it('keeps the 440-pixel compact default and neighbor fallback', async () => {
    const { rerender } = await renderAsync(
      <FloatingNoteCard
        {...baseProps}
        neighbors={[{ id: 'notes/grace.md', title: 'Grace Hopper' }]}
      />,
    );

    expect(screen.getByTestId('floating-note-hud').getAttribute('style')).toContain('width: 440px');
    expect(screen.getByText('Immediate Stellar Neighbors (1)')).toBeTruthy();
    rerender(<FloatingNoteCard {...baseProps} isMaximized />);
    expect(screen.getByTestId('floating-note-hud').getAttribute('style')).toBe('');
  });

  it('restores and reports scroll position on the note reader viewport', async () => {
    const onScrollPositionChange = vi.fn();
    const { container, rerender } = await renderAsync(
      <FloatingNoteCard
        {...baseProps}
        initialScrollTop={86}
        onScrollPositionChange={onScrollPositionChange}
      />,
    );
    const viewport = container.querySelector('[data-slot="scroll-area-viewport"]') as HTMLElement;
    expect(viewport.scrollTop).toBe(86);

    viewport.scrollTop = 120;
    fireEvent.scroll(viewport);
    expect(onScrollPositionChange).toHaveBeenCalledWith(120);

    rerender(
      <FloatingNoteCard
        {...baseProps}
        activeNote={{ id: 'notes/grace.md', title: 'Grace Hopper' }}
        initialScrollTop={12}
        onScrollPositionChange={onScrollPositionChange}
      />,
    );
    expect(viewport.scrollTop).toBe(12);
  });

  it('renders cyclic YAML aliases without recursive metadata formatting', async () => {
    const note = '---\nloop: &loop\n  self: *loop\n---\nBody';
    await renderAsync(<FloatingNoteCard {...baseProps} noteContent={note} processedMarkdown={note} />);
    fireEvent.click(screen.getByText('Metadata'));
    expect(screen.getByText(/self: \[Circular\]/)).toBeTruthy();
  });

  it('links setext headings correctly and tolerates malformed percent escapes', async () => {
    const note = 'First\n=====\n\nSecond\n------\n\n## Third\n\n[Bad](#bad%zz)';
    const { container } = await renderAsync(<FloatingNoteCard {...baseProps} noteContent={note} processedMarkdown={note} />);
    expect(container.querySelector('h2#second')).toBeTruthy();
    expect(screen.getByRole('navigation', { name: 'Table of contents' })).toBeTruthy();
    expect(() => fireEvent.click(screen.getByRole('link', { name: 'Bad' }))).not.toThrow();
  });

  it('keeps table of contents targets aligned with headings inside blockquotes and lists', async () => {
    const note = '> ## Quoted\n\n- ## Listed\n\n## Ordinary';
    const { container } = await renderAsync(<FloatingNoteCard {...baseProps} noteContent={note} processedMarkdown={note} />);
    const headings = Array.from(container.querySelectorAll('[data-testid="note-markdown"] h2'));
    const links = Array.from(container.querySelectorAll('nav[aria-label="Table of contents"] a'));

    expect(headings.map((heading) => heading.id)).toEqual(['quoted', 'listed', 'ordinary']);
    expect(links.map((link) => link.getAttribute('href'))).toEqual(['#quoted', '#listed', '#ordinary']);
  });

  it('reapplies saved scroll offset when note content arrives after the shell', async () => {
    const { container, rerender } = await renderAsync(<FloatingNoteCard {...baseProps} initialScrollTop={86} />);
    const viewport = container.querySelector('[data-slot="scroll-area-viewport"]') as HTMLElement;
    viewport.scrollTop = 0;
    rerender(<FloatingNoteCard {...baseProps} processedMarkdown={'A'.repeat(2000)} initialScrollTop={86} />);
    expect(viewport.scrollTop).toBe(86);
  });
});
