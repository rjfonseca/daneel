import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QuickCaptureDialog } from '../src/components/QuickCaptureDialog';
import { client } from '../src/lib/apiClient';

vi.mock('../src/lib/apiClient', () => ({
  client: {
    captureNote: vi.fn(),
    ingestPath: vi.fn(),
  },
}));

describe('QuickCaptureDialog', () => {
  beforeEach(() => vi.clearAllMocks());
  afterEach(cleanup);

  it('saves only after an explicit submit and reports the saved path', async () => {
    vi.mocked(client.captureNote).mockResolvedValue({ relPath: 'inbox/new-note.md', warning: '' } as never);
    const onCaptured = vi.fn();
    render(<QuickCaptureDialog isOpen onClose={vi.fn()} onCaptured={onCaptured} />);
    expect(client.captureNote).not.toHaveBeenCalled();
    fireEvent.change(screen.getByRole('textbox', { name: 'Title' }), { target: { value: 'New idea' } });
    fireEvent.change(screen.getByRole('textbox', { name: 'Markdown content' }), { target: { value: 'Body text' } });
    fireEvent.change(screen.getByRole('textbox', { name: 'Source URL (optional)' }), { target: { value: 'https://example.com' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save to Inbox' }));
    await waitFor(() => expect(onCaptured).toHaveBeenCalledWith('inbox/new-note.md', undefined));
    expect(client.captureNote).toHaveBeenCalledWith({ title: 'New idea', contentMarkdown: 'Body text', sourceUrl: 'https://example.com' });
    expect(screen.getByRole('status').textContent).toContain('Saved');
  });

  it('preserves the draft after failed save and close', async () => {
    vi.mocked(client.captureNote).mockRejectedValue(new Error('Unable to save'));
    const onClose = vi.fn();
    const view = render(<QuickCaptureDialog isOpen onClose={onClose} onCaptured={vi.fn()} />);
    fireEvent.change(screen.getByRole('textbox', { name: 'Title' }), { target: { value: 'Draft' } });
    fireEvent.change(screen.getByRole('textbox', { name: 'Markdown content' }), { target: { value: 'Keep me' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save to Inbox' }));
    await waitFor(() => expect(screen.getByRole('alert').textContent).toContain('Unable to save'));
    fireEvent.click(screen.getByRole('button', { name: 'Close capture' }));
    expect(onClose).toHaveBeenCalledOnce();
    view.rerender(<QuickCaptureDialog isOpen={false} onClose={onClose} onCaptured={vi.fn()} />);
    view.rerender(<QuickCaptureDialog isOpen onClose={onClose} onCaptured={vi.fn()} />);
    expect((screen.getByRole('textbox', { name: 'Title' }) as HTMLInputElement).value).toBe('Draft');
    expect((screen.getByRole('textbox', { name: 'Markdown content' }) as HTMLTextAreaElement).value).toBe('Keep me');
  });

  it('blocks duplicate saves while a request is pending and announces indexing warnings', async () => {
    let finish!: (value: { relPath: string; warning: string }) => void;
    vi.mocked(client.captureNote).mockImplementation(() => new Promise(resolve => { finish = resolve as typeof finish; }) as never);
    const onCaptured = vi.fn();
    const onClose = vi.fn();
    render(<QuickCaptureDialog isOpen onClose={onClose} onCaptured={onCaptured} />);
    fireEvent.change(screen.getByRole('textbox', { name: 'Title' }), { target: { value: 'Idea' } });
    fireEvent.change(screen.getByRole('textbox', { name: 'Markdown content' }), { target: { value: 'Body' } });
    const save = screen.getByRole('button', { name: 'Save to Inbox' });
    fireEvent.click(save);
    expect((save as HTMLButtonElement).disabled).toBe(true);
    fireEvent.click(save);
    expect(client.captureNote).toHaveBeenCalledTimes(1);
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(onClose).not.toHaveBeenCalled();
    finish({ relPath: 'inbox/idea.md', warning: 'Search indexing failed.' });
    await waitFor(() => expect(screen.getByRole('status').textContent).toContain('Search indexing failed.'));
    expect(onCaptured).toHaveBeenCalledWith('inbox/idea.md', 'Search indexing failed.');
  });

  it('closes on Escape and keeps keyboard focus inside the dialog', () => {
    const onClose = vi.fn();
    render(<QuickCaptureDialog isOpen onClose={onClose} onCaptured={vi.fn()} />);
    fireEvent.change(screen.getByRole('textbox', { name: 'Title' }), { target: { value: 'Idea' } });
    fireEvent.change(screen.getByRole('textbox', { name: 'Markdown content' }), { target: { value: 'Body' } });
    screen.getByRole('button', { name: 'Save to Inbox' }).focus();
    fireEvent.keyDown(document, { key: 'Tab' });
    expect(screen.getByRole('button', { name: 'Close capture' })).toHaveFocus();
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(onClose).toHaveBeenCalledOnce();
  });

  it('switches between Write Note and Import Files / Host Path tabs with correct WAI-ARIA semantics', () => {
    render(<QuickCaptureDialog isOpen onClose={vi.fn()} onCaptured={vi.fn()} />);

    // Tab buttons and initial state
    const writeTab = screen.getByRole('tab', { name: 'Write Note' });
    const importTab = screen.getByRole('tab', { name: 'Import Files / Host Path' });

    expect(writeTab).toHaveAttribute('id', 'tab-write');
    expect(writeTab).toHaveAttribute('aria-controls', 'tabpanel-write');
    expect(writeTab).toHaveAttribute('aria-selected', 'true');

    expect(importTab).toHaveAttribute('id', 'tab-import');
    expect(importTab).toHaveAttribute('aria-controls', 'tabpanel-import');
    expect(importTab).toHaveAttribute('aria-selected', 'false');

    // Tab panels
    const writePanel = screen.getByRole('tabpanel', { name: 'Write Note' });
    expect(writePanel).toHaveAttribute('id', 'tabpanel-write');
    expect(writePanel).toHaveAttribute('aria-labelledby', 'tab-write');

    expect(screen.getByRole('textbox', { name: 'Title' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save to Inbox' })).toBeInTheDocument();

    // Switch to Import Files / Host Path tab
    fireEvent.click(importTab);
    expect(importTab).toHaveAttribute('aria-selected', 'true');
    expect(writeTab).toHaveAttribute('aria-selected', 'false');

    const importPanel = screen.getByRole('tabpanel', { name: 'Import Files / Host Path' });
    expect(importPanel).toHaveAttribute('id', 'tabpanel-import');
    expect(importPanel).toHaveAttribute('aria-labelledby', 'tab-import');

    expect(screen.queryByRole('textbox', { name: 'Title' })).not.toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Vault Destination (optional)' })).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Host Path' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Ingest Path' })).toBeInTheDocument();
    expect(screen.getByText('Drop markdown files here, or click to browse')).toBeInTheDocument();

    // Switch back to Write Note tab
    fireEvent.click(writeTab);
    expect(screen.getByRole('textbox', { name: 'Title' })).toBeInTheDocument();
  });

  it('imports files via file picker, keeps dialog open, and triggers onCaptured on item click', async () => {
    vi.mocked(client.captureNote).mockResolvedValue({ relPath: 'notes/rfcs/my-proposal.md', warning: '' } as never);
    const onCaptured = vi.fn();
    render(<QuickCaptureDialog isOpen onClose={vi.fn()} onCaptured={onCaptured} />);

    fireEvent.click(screen.getByRole('tab', { name: 'Import Files / Host Path' }));

    const destInput = screen.getByRole('textbox', { name: 'Vault Destination (optional)' });
    expect((destInput as HTMLInputElement).value).toBe('inbox');
    fireEvent.change(destInput, { target: { value: 'notes/rfcs' } });

    const file = new File(['# Architecture\nDetails here'], 'my-proposal.md', { type: 'text/markdown' });
    const fileInput = screen.getByTestId('file-import-input');
    fireEvent.change(fileInput, { target: { files: [file] } });

    await waitFor(() => {
      expect(client.captureNote).toHaveBeenCalledWith({
        title: 'my-proposal',
        contentMarkdown: '# Architecture\nDetails here',
        destination: 'notes/rfcs',
      });
    });

    // Dialog stays open: onCaptured must NOT be called automatically in the loop
    expect(onCaptured).not.toHaveBeenCalled();
    expect(screen.getByRole('status').textContent).toContain('Successfully imported 1 file to notes/rfcs.');

    // Interactive button in feedback ledger
    const noteButton = screen.getByRole('button', { name: 'notes/rfcs/my-proposal.md' });
    expect(noteButton).toBeInTheDocument();

    // User clicks the ledger item to focus/view the note
    fireEvent.click(noteButton);
    expect(onCaptured).toHaveBeenCalledWith('notes/rfcs/my-proposal.md', undefined);
  });

  it('imports files via drag-and-drop zone and triggers onCaptured on item click', async () => {
    vi.mocked(client.captureNote).mockResolvedValue({ relPath: 'inbox/dropped-note.md', warning: '' } as never);
    const onCaptured = vi.fn();
    render(<QuickCaptureDialog isOpen onClose={vi.fn()} onCaptured={onCaptured} />);

    fireEvent.click(screen.getByRole('tab', { name: 'Import Files / Host Path' }));

    const dropZone = screen.getByText('Drop markdown files here, or click to browse').closest('div')!;
    const file = new File(['Drop content'], 'dropped-note.md', { type: 'text/markdown' });
    fireEvent.drop(dropZone, {
      dataTransfer: {
        files: [file],
      },
    });

    await waitFor(() => {
      expect(client.captureNote).toHaveBeenCalledWith({
        title: 'dropped-note',
        contentMarkdown: 'Drop content',
        destination: 'inbox',
      });
    });

    // Kept open for feedback
    expect(onCaptured).not.toHaveBeenCalled();

    const noteButton = screen.getByRole('button', { name: 'inbox/dropped-note.md' });
    fireEvent.click(noteButton);
    expect(onCaptured).toHaveBeenCalledWith('inbox/dropped-note.md', undefined);
  });

  it('rejects unsupported file types on drag-and-drop', async () => {
    render(<QuickCaptureDialog isOpen onClose={vi.fn()} onCaptured={vi.fn()} />);

    fireEvent.click(screen.getByRole('tab', { name: 'Import Files / Host Path' }));

    const dropZone = screen.getByText('Drop markdown files here, or click to browse').closest('div')!;
    const invalidFile = new File(['binary'], 'image.png', { type: 'image/png' });
    fireEvent.drop(dropZone, {
      dataTransfer: {
        files: [invalidFile],
      },
    });

    expect(client.captureNote).not.toHaveBeenCalled();
    expect(screen.getByRole('alert').textContent).toContain('Only markdown (.md, .markdown) and text (.txt) files are supported.');
  });

  it('ingests a host path via client.ingestPath and provides interactive note buttons in ledger', async () => {
    vi.mocked(client.ingestPath).mockResolvedValue({
      items: [
        {
          sourcePath: '/tmp/notes/test.md',
          relPath: 'inbox/test.md',
          title: 'test',
          sizeBytes: 120n,
          warning: '',
        },
      ],
      totalProcessed: 1,
      warning: '',
    } as never);
    const onCaptured = vi.fn();
    render(<QuickCaptureDialog isOpen onClose={vi.fn()} onCaptured={onCaptured} />);

    fireEvent.click(screen.getByRole('tab', { name: 'Import Files / Host Path' }));

    const hostInput = screen.getByRole('textbox', { name: 'Host Path' });
    fireEvent.change(hostInput, { target: { value: '~/Downloads/notes' } });
    fireEvent.click(screen.getByRole('button', { name: 'Ingest Path' }));

    await waitFor(() => {
      expect(client.ingestPath).toHaveBeenCalledWith({
        path: '~/Downloads/notes',
        destination: 'inbox',
        recursive: true,
        move: false,
      });
    });

    // Kept open for user inspection
    expect(onCaptured).not.toHaveBeenCalled();
    expect(screen.getByRole('status').textContent).toContain('Successfully ingested 1 item(s)');

    const noteButton = screen.getByRole('button', { name: 'inbox/test.md' });
    fireEvent.click(noteButton);
    expect(onCaptured).toHaveBeenCalledWith('inbox/test.md', undefined);
  });

  it('displays error when file upload fails', async () => {
    vi.mocked(client.captureNote).mockRejectedValue(new Error('Vault storage disk full'));
    render(<QuickCaptureDialog isOpen onClose={vi.fn()} onCaptured={vi.fn()} />);

    fireEvent.click(screen.getByRole('tab', { name: 'Import Files / Host Path' }));

    const file = new File(['content'], 'failed.md', { type: 'text/markdown' });
    const fileInput = screen.getByTestId('file-import-input');
    fireEvent.change(fileInput, { target: { files: [file] } });

    await waitFor(() => {
      expect(screen.getByRole('alert').textContent).toContain('Vault storage disk full');
    });
  });

  it('displays error when host path ingestion fails', async () => {
    vi.mocked(client.ingestPath).mockRejectedValue(new Error('Path not found on host'));
    render(<QuickCaptureDialog isOpen onClose={vi.fn()} onCaptured={vi.fn()} />);

    fireEvent.click(screen.getByRole('tab', { name: 'Import Files / Host Path' }));

    const hostInput = screen.getByRole('textbox', { name: 'Host Path' });
    fireEvent.change(hostInput, { target: { value: '/invalid/path' } });
    fireEvent.click(screen.getByRole('button', { name: 'Ingest Path' }));

    await waitFor(() => {
      expect(screen.getByRole('alert').textContent).toContain('Path not found on host');
    });
  });
});
