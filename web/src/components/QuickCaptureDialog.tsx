import { useEffect, useRef, useState, type FormEvent } from 'react';
import { client } from '../lib/apiClient';

export interface QuickCaptureDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onCaptured: (path: string, warning?: string) => void;
}

interface ImportedFileItem {
  relPath: string;
  title: string;
  warning?: string;
}

const ALLOWED_EXTENSIONS = ['.md', '.markdown', '.txt'];

function isAllowedFile(file: File): boolean {
  const lower = file.name.toLowerCase();
  return ALLOWED_EXTENSIONS.some((ext) => lower.endsWith(ext));
}

async function readFileText(file: File): Promise<string> {
  if (typeof file.text === 'function') {
    return file.text();
  }
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve((reader.result as string) || '');
    reader.onerror = () => reject(reader.error || new Error('Failed to read file'));
    reader.readAsText(file);
  });
}

export function QuickCaptureDialog({ isOpen, onClose, onCaptured }: QuickCaptureDialogProps) {
  const [activeTab, setActiveTab] = useState<'write' | 'import'>('write');
  const [title, setTitle] = useState('');
  const [content, setContent] = useState('');
  const [sourceUrl, setSourceUrl] = useState('');
  const [destination, setDestination] = useState('inbox');
  const [hostPath, setHostPath] = useState('');
  const [isDragging, setIsDragging] = useState(false);
  const [importedFiles, setImportedFiles] = useState<ImportedFileItem[]>([]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const submitting = useRef(false);
  const isMounted = useRef(true);
  const dialog = useRef<HTMLElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    isMounted.current = true;
    return () => {
      isMounted.current = false;
    };
  }, []);

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
      const focusable = Array.from(
        dialog.current.querySelectorAll<HTMLElement>(
          'button:not(:disabled), input:not(:disabled), textarea:not(:disabled)'
        )
      );
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
      if (!isMounted.current) return;
      setNotice(result.warning ? `Saved to Inbox. ${result.warning}` : 'Saved to Inbox.');
      onCaptured(result.relPath, result.warning || undefined);
      setTitle('');
      setContent('');
      setSourceUrl('');
    } catch (cause) {
      if (!isMounted.current) return;
      setError(cause instanceof Error ? cause.message : 'Unable to save this note. Please try again.');
    } finally {
      submitting.current = false;
      if (isMounted.current) {
        setSaving(false);
      }
    }
  };

  const handleFiles = async (files: FileList | File[]) => {
    if (submitting.current) return;
    submitting.current = true;
    setSaving(true);
    setError('');
    setNotice('');
    const targetDest = destination.trim() || 'inbox';
    const fileList = Array.from(files);
    if (fileList.length === 0) {
      submitting.current = false;
      if (isMounted.current) setSaving(false);
      return;
    }

    const imported: ImportedFileItem[] = [];
    try {
      for (const file of fileList) {
        const contentMarkdown = await readFileText(file);
        const fileTitle = file.name.replace(/\.[^/.]+$/, '') || file.name;
        const result = await client.captureNote({
          title: fileTitle,
          contentMarkdown,
          destination: targetDest,
        });
        imported.push({ relPath: result.relPath, title: fileTitle, warning: result.warning || undefined });
      }
      if (!isMounted.current) return;
      setImportedFiles((prev) => [...imported, ...prev]);
      const count = imported.length;
      setNotice(`Successfully imported ${count} ${count === 1 ? 'file' : 'files'} to ${targetDest}. Click any note below to view.`);
    } catch (cause) {
      if (!isMounted.current) return;
      setError(cause instanceof Error ? cause.message : 'Unable to import file(s). Please try again.');
    } finally {
      submitting.current = false;
      if (isMounted.current) {
        setSaving(false);
      }
      if (fileInputRef.current) {
        fileInputRef.current.value = '';
      }
    }
  };

  const handleHostPath = async () => {
    const trimmedPath = hostPath.trim();
    if (submitting.current || !trimmedPath) return;
    submitting.current = true;
    setSaving(true);
    setError('');
    setNotice('');
    const targetDest = destination.trim() || 'inbox';
    try {
      const result = await client.ingestPath({
        path: trimmedPath,
        destination: targetDest,
        recursive: true,
        move: false,
      });
      if (!isMounted.current) return;
      const items = result.items || [];
      if (items.length > 0) {
        setImportedFiles((prev) => [
          ...items.map((item) => ({
            relPath: item.relPath,
            title: item.title || item.relPath,
            warning: item.warning || undefined,
          })),
          ...prev,
        ]);
        setNotice(
          result.warning
            ? `Ingested ${items.length} item(s). ${result.warning} Click any note below to view.`
            : `Successfully ingested ${items.length} item(s) from host path. Click any note below to view.`
        );
      } else {
        setNotice(result.warning || 'Host path processed (no files were ingested).');
      }
      setHostPath('');
    } catch (cause) {
      if (!isMounted.current) return;
      setError(cause instanceof Error ? cause.message : 'Unable to ingest host path. Please try again.');
    } finally {
      submitting.current = false;
      if (isMounted.current) {
        setSaving(false);
      }
    }
  };

  return (
    <div
      className="fixed inset-0 z-60 flex items-center justify-center bg-black/70 p-4 backdrop-blur-xs"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget && !submitting.current) onClose();
      }}
    >
      <section
        ref={dialog}
        role="dialog"
        aria-modal="true"
        aria-labelledby="capture-title"
        aria-describedby="capture-description"
        className="max-h-[calc(100dvh-2rem)] w-full max-w-2xl overflow-y-auto overscroll-contain rounded-2xl border border-indigo-500/40 bg-[#0c0d15] p-5 text-white shadow-hud-deep sm:p-7"
      >
        <div className="flex items-start justify-between gap-4">
          <div>
            <h2 id="capture-title" className="text-lg font-semibold">
              Capture a note
            </h2>
            <p id="capture-description" className="mt-1 text-sm text-gray-400">
              Save a thought or import documents directly to your Second Brain.
            </p>
          </div>
          <button
            type="button"
            aria-label="Close capture"
            onClick={onClose}
            disabled={saving}
            className="rounded-lg px-2 py-1 text-gray-400 hover:bg-gray-800 hover:text-white disabled:opacity-50 cosmic-focus-visible"
          >
            ✕
          </button>
        </div>

        <div role="tablist" aria-label="Capture modes" className="mt-4 flex gap-2 border-b border-gray-800 pb-2">
          <button
            id="tab-write"
            type="button"
            role="tab"
            aria-selected={activeTab === 'write'}
            aria-controls="tabpanel-write"
            onClick={() => {
              setActiveTab('write');
              setError('');
              setNotice('');
            }}
            className={`rounded-lg px-3 py-1.5 text-sm font-medium transition-colors cursor-pointer cosmic-focus-visible ${
              activeTab === 'write'
                ? 'bg-indigo-600/30 text-indigo-300 border border-indigo-500/50'
                : 'text-gray-400 hover:bg-gray-800 hover:text-white'
            }`}
          >
            Write Note
          </button>
          <button
            id="tab-import"
            type="button"
            role="tab"
            aria-selected={activeTab === 'import'}
            aria-controls="tabpanel-import"
            onClick={() => {
              setActiveTab('import');
              setError('');
              setNotice('');
            }}
            className={`rounded-lg px-3 py-1.5 text-sm font-medium transition-colors cursor-pointer cosmic-focus-visible ${
              activeTab === 'import'
                ? 'bg-indigo-600/30 text-indigo-300 border border-indigo-500/50'
                : 'text-gray-400 hover:bg-gray-800 hover:text-white'
            }`}
          >
            Import Files / Host Path
          </button>
        </div>

        {activeTab === 'write' && (
          <div role="tabpanel" id="tabpanel-write" aria-labelledby="tab-write">
            <form onSubmit={save} className="mt-5 space-y-4">
              <div>
                <label htmlFor="capture-note-title" className="mb-1 block text-sm font-medium">
                  Title
                </label>
                <input
                  id="capture-note-title"
                  autoFocus
                  required
                  maxLength={200}
                  value={title}
                  onChange={(event) => setTitle(event.target.value)}
                  className="w-full rounded-lg border border-gray-700 bg-gray-900 px-3 py-2 text-sm outline-hidden focus:border-indigo-400"
                  placeholder="What is this about?"
                />
              </div>
              <div>
                <label htmlFor="capture-note-content" className="mb-1 block text-sm font-medium">
                  Markdown content
                </label>
                <textarea
                  id="capture-note-content"
                  required
                  maxLength={1048576}
                  value={content}
                  onChange={(event) => setContent(event.target.value)}
                  rows={8}
                  className="w-full resize-y rounded-lg border border-gray-700 bg-gray-900 px-3 py-2 font-mono text-sm outline-hidden focus:border-indigo-400"
                  placeholder="Write your note…"
                />
              </div>
              <div>
                <label htmlFor="capture-note-source" className="mb-1 block text-sm font-medium">
                  Source URL (optional)
                </label>
                <input
                  id="capture-note-source"
                  type="url"
                  maxLength={2048}
                  value={sourceUrl}
                  onChange={(event) => setSourceUrl(event.target.value)}
                  className="w-full rounded-lg border border-gray-700 bg-gray-900 px-3 py-2 text-sm outline-hidden focus:border-indigo-400"
                  placeholder="https://example.com"
                />
              </div>
              {error && <p role="alert" className="text-sm text-rose-300">{error}</p>}
              {notice && <p role="status" aria-live="polite" className="text-sm text-emerald-300">{notice}</p>}
              <div className="flex justify-end gap-3">
                <button
                  type="button"
                  onClick={onClose}
                  disabled={saving}
                  className="rounded-lg border border-gray-700 px-4 py-2 text-sm text-gray-300 hover:bg-gray-800 disabled:opacity-50 cosmic-focus-visible"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={saving || !title.trim() || !content.trim()}
                  className="rounded-lg bg-indigo-500 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-400 disabled:cursor-not-allowed disabled:opacity-50 cosmic-focus-visible"
                >
                  {saving ? 'Saving…' : 'Save to Inbox'}
                </button>
              </div>
            </form>
          </div>
        )}

        {activeTab === 'import' && (
          <div role="tabpanel" id="tabpanel-import" aria-labelledby="tab-import" className="mt-5 space-y-4">
            <div>
              <label htmlFor="import-destination" className="mb-1 block text-sm font-medium">
                Vault Destination (optional)
              </label>
              <input
                id="import-destination"
                value={destination}
                onChange={(event) => setDestination(event.target.value)}
                className="w-full rounded-lg border border-gray-700 bg-gray-900 px-3 py-2 text-sm outline-hidden focus:border-indigo-400"
                placeholder="inbox"
              />
            </div>

            <div>
              <span className="mb-1 block text-sm font-medium">
                Import Markdown Files
              </span>
              <div
                onDragOver={(event) => {
                  event.preventDefault();
                  event.stopPropagation();
                  setIsDragging(true);
                }}
                onDragLeave={(event) => {
                  event.preventDefault();
                  event.stopPropagation();
                  setIsDragging(false);
                }}
                onDrop={(event) => {
                  event.preventDefault();
                  event.stopPropagation();
                  setIsDragging(false);
                  if (!event.dataTransfer.files || event.dataTransfer.files.length === 0) return;

                  const droppedFiles = Array.from(event.dataTransfer.files);
                  const validFiles = droppedFiles.filter(isAllowedFile);
                  const invalidFiles = droppedFiles.filter((file) => !isAllowedFile(file));

                  if (invalidFiles.length > 0 && validFiles.length === 0) {
                    setError('Only markdown (.md, .markdown) and text (.txt) files are supported.');
                    return;
                  }
                  if (invalidFiles.length > 0) {
                    setError(`Skipped ${invalidFiles.length} unsupported file(s). Only markdown (.md, .markdown) and text (.txt) files are supported.`);
                  }
                  if (validFiles.length > 0) {
                    void handleFiles(validFiles);
                  }
                }}
                onClick={() => fileInputRef.current?.click()}
                className={`flex flex-col items-center justify-center rounded-xl border-2 border-dashed p-6 text-center cursor-pointer transition-colors ${
                  isDragging
                    ? 'border-indigo-400 bg-indigo-500/10'
                    : 'border-gray-700 bg-gray-900/50 hover:border-indigo-400/60 hover:bg-gray-900/80'
                }`}
              >
                <input
                  ref={fileInputRef}
                  type="file"
                  multiple
                  accept=".md,.markdown,.txt"
                  data-testid="file-import-input"
                  aria-label="Upload markdown files"
                  onChange={(event) => {
                    if (event.target.files && event.target.files.length > 0) {
                      const files = Array.from(event.target.files);
                      const validFiles = files.filter(isAllowedFile);
                      if (validFiles.length < files.length) {
                        setError(`Skipped ${files.length - validFiles.length} unsupported file(s).`);
                      }
                      if (validFiles.length > 0) {
                        void handleFiles(validFiles);
                      }
                    }
                  }}
                  className="sr-only"
                />
                <p className="pointer-events-none text-sm font-medium text-gray-300">
                  Drop markdown files here, or click to browse
                </p>
                <p className="pointer-events-none mt-1 text-xs text-gray-500">
                  Accepts .md, .markdown, .txt
                </p>
              </div>
            </div>

            <div>
              <label htmlFor="import-host-path" className="mb-1 block text-sm font-medium">
                Host Path
              </label>
              <div className="flex gap-2">
                <input
                  id="import-host-path"
                  value={hostPath}
                  onChange={(event) => setHostPath(event.target.value)}
                  placeholder="/path/to/file.md or ~/Downloads/notes"
                  className="w-full rounded-lg border border-gray-700 bg-gray-900 px-3 py-2 text-sm outline-hidden focus:border-indigo-400"
                />
                <button
                  type="button"
                  onClick={() => void handleHostPath()}
                  disabled={saving || !hostPath.trim()}
                  className="shrink-0 rounded-lg bg-indigo-500 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-400 disabled:cursor-not-allowed disabled:opacity-50 cosmic-focus-visible"
                >
                  {saving ? 'Ingesting…' : 'Ingest Path'}
                </button>
              </div>
            </div>

            {error && <p role="alert" className="text-sm text-rose-300">{error}</p>}
            {notice && <p role="status" aria-live="polite" className="text-sm text-emerald-300">{notice}</p>}

            {importedFiles.length > 0 && (
              <div className="rounded-lg border border-gray-800 bg-gray-900/60 p-3">
                <div className="flex items-center justify-between">
                  <h4 className="text-xs font-semibold uppercase tracking-wider text-gray-400">
                    Imported Notes ({importedFiles.length})
                  </h4>
                  <span className="text-[11px] text-gray-500">Click to view note</span>
                </div>
                <ul className="mt-2 max-h-36 divide-y divide-gray-800 overflow-y-auto text-xs">
                  {importedFiles.map((file, idx) => (
                    <li key={`${file.relPath}-${idx}`} className="flex items-center justify-between py-1.5">
                      <button
                        type="button"
                        onClick={() => onCaptured(file.relPath, file.warning)}
                        title={`View note: ${file.relPath}`}
                        className="font-mono text-indigo-300 hover:text-indigo-200 hover:underline truncate max-w-sm text-left cursor-pointer cosmic-focus-visible"
                      >
                        {file.relPath}
                      </button>
                      {file.warning ? (
                        <span className="ml-2 text-amber-400 text-[10px] truncate max-w-[150px]" title={file.warning}>
                          {file.warning}
                        </span>
                      ) : (
                        <span className="ml-2 text-emerald-400 text-[10px]">Indexed</span>
                      )}
                    </li>
                  ))}
                </ul>
              </div>
            )}

            <div className="flex justify-end gap-3 pt-2">
              <button
                type="button"
                onClick={onClose}
                disabled={saving}
                className="rounded-lg border border-gray-700 px-4 py-2 text-sm text-gray-300 hover:bg-gray-800 disabled:opacity-50 cosmic-focus-visible"
              >
                Close
              </button>
            </div>
          </div>
        )}
      </section>
    </div>
  );
}
