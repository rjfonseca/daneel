import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QuickCaptureDialog } from '../src/components/QuickCaptureDialog';
import { client } from '../src/lib/apiClient';

vi.mock('../src/lib/apiClient', () => ({ client: { captureNote: vi.fn() } }));

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
});
