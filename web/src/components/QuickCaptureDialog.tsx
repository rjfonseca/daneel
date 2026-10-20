import { useEffect, useRef, useState, type FormEvent } from 'react';
import { client } from '../lib/apiClient';

export interface QuickCaptureDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onCaptured: (path: string, warning?: string) => void;
}

export function QuickCaptureDialog({ isOpen, onClose, onCaptured }: QuickCaptureDialogProps) {
  const [title, setTitle] = useState('');
  const [content, setContent] = useState('');
  const [sourceUrl, setSourceUrl] = useState('');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const submitting = useRef(false);
  const dialog = useRef<HTMLElement>(null);

  useEffect(() => {
    if (!isOpen) return;
    const firstInput = dialog.current?.querySelector<HTMLInputElement>('input');
    firstInput?.focus();
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault();
        event.stopPropagation();
        if (!submitting.current) onClose();
        return;
      }
      event.stopPropagation();
      if (event.key !== 'Tab' || !dialog.current) return;
      const focusable = Array.from(dialog.current.querySelectorAll<HTMLElement>('button:not(:disabled), input:not(:disabled), textarea:not(:disabled)'));
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && (document.activeElement === first || !dialog.current.contains(document.activeElement))) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && (document.activeElement === last || !dialog.current.contains(document.activeElement))) {
        event.preventDefault();
        first.focus();
      }
    };
    document.addEventListener('keydown', handleKeyDown, true);
    return () => document.removeEventListener('keydown', handleKeyDown, true);
  }, [isOpen, onClose]);

  if (!isOpen) return null;

  const save = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (submitting.current) return;
    submitting.current = true;
    setSaving(true);
    setError('');
    setNotice('');
    try {
      const result = await client.captureNote({
        title: title.trim(),
        contentMarkdown: content,
        sourceUrl: sourceUrl.trim(),
      });
      setNotice(result.warning ? `Saved to Inbox. ${result.warning}` : 'Saved to Inbox.');
      onCaptured(result.relPath, result.warning || undefined);
      setTitle('');
      setContent('');
      setSourceUrl('');
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Unable to save this note. Please try again.');
    } finally {
      submitting.current = false;
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 z-60 flex items-center justify-center bg-black/70 p-4 backdrop-blur-xs" onMouseDown={(event) => { if (event.target === event.currentTarget && !submitting.current) onClose(); }}>
      <section ref={dialog} role="dialog" aria-modal="true" aria-labelledby="capture-title" aria-describedby="capture-description" className="max-h-[calc(100dvh-2rem)] w-full max-w-2xl overflow-y-auto overscroll-contain rounded-2xl border border-indigo-500/40 bg-[#0c0d15] p-5 text-white shadow-hud-deep sm:p-7">
        <div className="flex items-start justify-between gap-4">
          <div>
            <h2 id="capture-title" className="text-lg font-semibold">Capture a note</h2>
            <p id="capture-description" className="mt-1 text-sm text-gray-400">Save a thought directly to your local Inbox.</p>
          </div>
          <button type="button" aria-label="Close capture" onClick={onClose} disabled={saving} className="rounded-lg px-2 py-1 text-gray-400 hover:bg-gray-800 hover:text-white disabled:opacity-50 cosmic-focus-visible">✕</button>
        </div>
        <form onSubmit={save} className="mt-5 space-y-4">
          <div>
            <label htmlFor="capture-note-title" className="mb-1 block text-sm font-medium">Title</label>
            <input id="capture-note-title" autoFocus required maxLength={200} value={title} onChange={(event) => setTitle(event.target.value)} className="w-full rounded-lg border border-gray-700 bg-gray-900 px-3 py-2 text-sm outline-hidden focus:border-indigo-400" placeholder="What is this about?" />
          </div>
          <div>
            <label htmlFor="capture-note-content" className="mb-1 block text-sm font-medium">Markdown content</label>
            <textarea id="capture-note-content" required maxLength={1048576} value={content} onChange={(event) => setContent(event.target.value)} rows={8} className="w-full resize-y rounded-lg border border-gray-700 bg-gray-900 px-3 py-2 font-mono text-sm outline-hidden focus:border-indigo-400" placeholder="Write your note…" />
          </div>
          <div>
            <label htmlFor="capture-note-source" className="mb-1 block text-sm font-medium">Source URL (optional)</label>
            <input id="capture-note-source" type="url" maxLength={2048} value={sourceUrl} onChange={(event) => setSourceUrl(event.target.value)} className="w-full rounded-lg border border-gray-700 bg-gray-900 px-3 py-2 text-sm outline-hidden focus:border-indigo-400" placeholder="https://example.com" />
          </div>
          {error && <p role="alert" className="text-sm text-rose-300">{error}</p>}
          {notice && <p role="status" aria-live="polite" className="text-sm text-emerald-300">{notice}</p>}
          <div className="flex justify-end gap-3">
            <button type="button" onClick={onClose} disabled={saving} className="rounded-lg border border-gray-700 px-4 py-2 text-sm text-gray-300 hover:bg-gray-800 disabled:opacity-50 cosmic-focus-visible">Cancel</button>
            <button type="submit" disabled={saving || !title.trim() || !content.trim()} className="rounded-lg bg-indigo-500 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-400 disabled:cursor-not-allowed disabled:opacity-50 cosmic-focus-visible">{saving ? 'Saving…' : 'Save to Inbox'}</button>
          </div>
        </form>
      </section>
    </div>
  );
}
