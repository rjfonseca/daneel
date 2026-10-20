import type { GalaxyNode } from '../types/galaxy';
import React, { useCallback, useLayoutEffect, useMemo, useRef, useState } from 'react';
import {
  X,
  Maximize2,
  Minimize2,
  Copy,
  Check,
  Orbit,
  Compass,
  ExternalLink,
  Sparkles,
  Pin,
  Minus,
  Plus,
} from 'lucide-react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { ScrollArea } from '@/components/ui/scroll-area';
import { cn } from '@/lib/utils';
import { getNoteHeadings, parseNoteDocument, removeDuplicateTitleHeading } from '@/lib/noteDocument';

export interface NoteRelationRef {
  id: string;
  title: string;
  color?: string;
  stellarBadge?: string;
}

export interface FloatingNoteCardProps {
  activeNote: GalaxyNode;
  noteContent: string;
  processedMarkdown: string;
  isMaximized: boolean;
  onToggleMaximize: () => void;
  onClose: () => void;
  onCopy: () => void;
  copied: boolean;
  onNavigateInternalLink: (href: string) => void;
  isConstellationActive: boolean;
  onResetConstellation: () => void;
  neighbors?: NoteRelationRef[];
  incomingLinks?: NoteRelationRef[];
  outgoingLinks?: NoteRelationRef[];
  isPinned?: boolean;
  onTogglePin?: () => void;
  drawerWidth?: number;
  onDrawerWidthChange?: (width: number) => void;
  initialScrollTop?: number;
  onScrollPositionChange?: (top: number) => void;
}

const MIN_DRAWER_WIDTH = 320;
const MAX_DRAWER_WIDTH = 800;
const DRAWER_WIDTH_STEP = 16;

function clampDrawerWidth(width: number): number {
  return Math.min(MAX_DRAWER_WIDTH, Math.max(MIN_DRAWER_WIDTH, width));
}

function formatMetadataValue(value: unknown, seen = new WeakSet<object>()): string {
  if (value && typeof value === 'object') {
    if (seen.has(value)) return '[Circular]';
    seen.add(value);
    if (Array.isArray(value)) return value.map((item) => formatMetadataValue(item, seen)).join(', ');
    return Object.entries(value).map(([key, item]) => `${key}: ${formatMetadataValue(item, seen)}`).join(', ');
  }
  return String(value);
}

export const FloatingNoteCard: React.FC<FloatingNoteCardProps> = ({
  activeNote,
  processedMarkdown,
  isMaximized,
  onToggleMaximize,
  onClose,
  onCopy,
  copied,
  onNavigateInternalLink,
  isConstellationActive,
  onResetConstellation,
  neighbors = [],
  incomingLinks,
  outgoingLinks,
  isPinned = false,
  onTogglePin,
  drawerWidth = 440,
  onDrawerWidthChange,
  initialScrollTop = 0,
  onScrollPositionChange,
}) => {
  const [fontSize, setFontSize] = useState(14);
  const dragStart = useRef<{ x: number; width: number } | null>(null);
  const scrollAreaRoot = useRef<HTMLDivElement>(null);

  useLayoutEffect(() => {
    const viewport = scrollAreaRoot.current?.querySelector<HTMLElement>('[data-slot="scroll-area-viewport"]');
    if (viewport) viewport.scrollTop = Math.max(0, initialScrollTop);
  }, [activeNote.id, initialScrollTop, processedMarkdown]);

  const parsedDocument = useMemo(() => parseNoteDocument(processedMarkdown), [processedMarkdown]);
  const title = activeNote.title || activeNote.relPath || activeNote.id;
  const markdown = useMemo(() => removeDuplicateTitleHeading(parsedDocument.body, title), [parsedDocument.body, title]);
  const headings = useMemo(() => getNoteHeadings(markdown), [markdown]);
  const showTableOfContents = headings.length >= 3 || (headings.length > 0 && markdown.length >= 1200);
  const hasDirectedRelations = incomingLinks !== undefined || outgoingLinks !== undefined;
  const incoming = incomingLinks ?? [];
  const outgoing = outgoingLinks ?? [];
  const headingIndex = useRef(0);
  headingIndex.current = 0;

  const navigateToAnchor = useCallback((event: React.MouseEvent<HTMLAnchorElement>, href?: string) => {
    if (!href?.startsWith('#')) return false;
    event.preventDefault();
    let id = href.slice(1);
    try { id = decodeURIComponent(id); } catch { /* Keep malformed fragments as literal IDs. */ }
    const viewport = event.currentTarget.closest('[data-slot="scroll-area"]')
      ?.querySelector<HTMLElement>('[data-slot="scroll-area-viewport"]');
    const target = viewport && Array.from(viewport.querySelectorAll<HTMLElement>('[id]'))
      .find((element) => element.id === id);
    if (viewport && target) {
      const top = viewport.scrollTop + target.getBoundingClientRect().top - viewport.getBoundingClientRect().top;
      viewport.scrollTo({ top, behavior: 'smooth' });
    }
    return true;
  }, []);

  // Keep ReactMarkdown's component types stable across unrelated parent renders.
  // Inline component maps otherwise remount the whole document on each render,
  // interrupting clicks and text selection in longer notes.
  const markdownComponents = useMemo(() => ({
    h1: ({ children, ...props }: React.ComponentProps<'h1'>) => {
      const heading = headings[headingIndex.current++];
      return <h1 id={heading?.id} {...props}>{children}</h1>;
    },
    h2: ({ children, ...props }: React.ComponentProps<'h2'>) => {
      const heading = headings[headingIndex.current++];
      return <h2 id={heading?.id} {...props}>{children}</h2>;
    },
    h3: ({ children, ...props }: React.ComponentProps<'h3'>) => {
      const heading = headings[headingIndex.current++];
      return <h3 id={heading?.id} {...props}>{children}</h3>;
    },
    h4: ({ children, ...props }: React.ComponentProps<'h4'>) => {
      const heading = headings[headingIndex.current++];
      return <h4 id={heading?.id} {...props}>{children}</h4>;
    },
    h5: ({ children, ...props }: React.ComponentProps<'h5'>) => {
      const heading = headings[headingIndex.current++];
      return <h5 id={heading?.id} {...props}>{children}</h5>;
    },
    h6: ({ children, ...props }: React.ComponentProps<'h6'>) => {
      const heading = headings[headingIndex.current++];
      return <h6 id={heading?.id} {...props}>{children}</h6>;
    },
    a: ({ href, children, ...props }: React.ComponentProps<'a'>) => {
      const isExternal = href?.startsWith('http://') || href?.startsWith('https://') || href?.startsWith('mailto:');
      if (href?.startsWith('#')) {
        return <a href={href} onClick={(event) => navigateToAnchor(event, href)} {...props}>{children}</a>;
      }
      if (!isExternal && href) {
        return <a href="#" onClick={(event) => {
          event.preventDefault();
          onNavigateInternalLink(href);
        }} className="text-indigo-400 hover:text-indigo-300 underline font-semibold cursor-pointer decoration-indigo-500/40 hover:decoration-indigo-400 transition-colors" title={`Open note: ${href}`} data-testid="internal-note-link" {...props}>{children}</a>;
      }
      return <a href={href} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1 text-cyan-400 hover:text-cyan-300 underline" {...props}>{children}<ExternalLink className="w-3 h-3 inline-block opacity-70" aria-hidden="true" /></a>;
    },
  }), [headings, navigateToAnchor, onNavigateInternalLink]);

  const changeFontSize = (delta: number) => setFontSize((current) => Math.min(20, Math.max(12, current + delta)));
  const changeDrawerWidth = (width: number) => onDrawerWidthChange?.(clampDrawerWidth(width));

  return (
    <aside
      data-testid="floating-note-hud"
      aria-label="Holographic Note Panel"
      style={!isMaximized ? { width: clampDrawerWidth(drawerWidth), top: 'var(--hud-content-top, 128px)' } : undefined}
      className={cn(
        'fixed z-40 flex flex-col bg-[#0b0c13]/90 backdrop-blur-2xl border border-indigo-500/30 rounded-2xl shadow-[0_0_50px_rgba(0,0,0,0.85)] transition-[width,background-color,border-color,box-shadow] duration-300 ease-out overflow-hidden select-text',
        isMaximized
          ? 'inset-4 sm:inset-8 lg:inset-12 z-50 border-indigo-400/50'
          : 'right-4 sm:right-6 bottom-6 w-[440px] max-w-[calc(100vw-2rem)] border-indigo-500/30',
      )}
    >
      {onDrawerWidthChange && !isMaximized && (
        <div
          role="separator"
          aria-label="Resize note panel"
          aria-orientation="vertical"
          aria-valuemin={MIN_DRAWER_WIDTH}
          aria-valuemax={MAX_DRAWER_WIDTH}
          aria-valuenow={clampDrawerWidth(drawerWidth)}
          tabIndex={0}
          onKeyDown={(event) => {
            if (event.key === 'ArrowLeft') {
              event.preventDefault();
              changeDrawerWidth(drawerWidth - DRAWER_WIDTH_STEP);
            } else if (event.key === 'ArrowRight') {
              event.preventDefault();
              changeDrawerWidth(drawerWidth + DRAWER_WIDTH_STEP);
            }
          }}
          onPointerDown={(event) => {
            dragStart.current = { x: event.clientX, width: drawerWidth };
            event.currentTarget.setPointerCapture?.(event.pointerId);
          }}
          onPointerMove={(event) => {
            if (dragStart.current) changeDrawerWidth(dragStart.current.width + dragStart.current.x - event.clientX);
          }}
          onPointerUp={() => { dragStart.current = null; }}
          onPointerCancel={() => { dragStart.current = null; }}
          className="absolute left-0 top-0 bottom-0 z-30 w-2 cursor-col-resize touch-none focus-visible:outline focus-visible:outline-2 focus-visible:outline-indigo-400"
          data-testid="resize-note-panel"
        />
      )}

      <div
        onDoubleClick={onToggleMaximize}
        className="p-5 sm:p-6 border-b border-gray-800/80 bg-linear-to-b from-indigo-950/30 to-transparent flex flex-col gap-3 select-none"
        title="Double-click to maximize or restore"
      >
        <div className="flex items-center justify-between gap-2 flex-wrap">
          <div className="flex items-center gap-1.5 flex-wrap">
            {isConstellationActive && (
              <button
                onClick={onResetConstellation}
                data-testid="constellation-hud-badge"
                aria-label="Restore full galaxy view from isolated constellation"
                className="group flex items-center gap-1.5 px-2.5 py-1 text-[10px] font-mono font-bold uppercase tracking-wider rounded-md bg-indigo-500/20 text-indigo-300 border border-indigo-500/40 hover:bg-indigo-500/30 transition-all shadow-[0_0_12px_rgba(99,102,241,0.25)] cursor-pointer cosmic-focus-visible"
                title="Isolated constellation (1-2 hops). Click to restore full galaxy view"
              >
                <Sparkles className="w-3 h-3 text-indigo-400 animate-pulse" aria-hidden="true" />
                <span>Isolated Constellation</span>
                <span className="text-gray-400 group-hover:text-white ml-0.5" aria-hidden="true">✕</span>
              </button>
            )}

            <span className="px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider rounded-md bg-gray-800/90 text-gray-300 border border-gray-700/80">
              {activeNote.category || 'root'}
            </span>
            {activeNote.stellarBadge && (
              <span
                className="px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider rounded-md font-mono"
                style={{
                  backgroundColor: `${activeNote.color || '#6366f1'}22`,
                  color: activeNote.color || '#818cf8',
                  border: `1px solid ${activeNote.color || '#6366f1'}55`,
                }}
              >
                {activeNote.stellarBadge}
              </span>
            )}
          </div>

          <div className="flex items-center gap-1" onClick={(event) => event.stopPropagation()}>
            {onTogglePin && (
              <button
                onClick={onTogglePin}
                aria-label={isPinned ? 'Unpin from Seldon Anchors' : 'Pin to Seldon Anchors'}
                className={cn(
                  'p-1.5 rounded-lg transition-colors cursor-pointer cosmic-focus-visible',
                  isPinned
                    ? 'bg-cyan-500/25 text-cyan-300 hover:bg-cyan-500/35 border border-cyan-400/50 shadow-[0_0_10px_rgba(56,189,248,0.3)]'
                    : 'bg-gray-800/60 hover:bg-gray-700 text-slate-300 hover:text-white',
                )}
                title={isPinned ? 'Unpin from Seldon Anchors' : 'Pin to Seldon Anchors'}
                data-testid="toggle-pin-note"
              >
                <Pin className={cn('w-4 h-4', isPinned && 'fill-cyan-300')} />
              </button>
            )}
            <button
              onClick={onCopy}
              aria-label={copied ? 'Copied note content to clipboard' : 'Copy note content to clipboard'}
              className="p-1.5 rounded-lg bg-gray-800/60 hover:bg-gray-700 text-slate-300 hover:text-white transition-colors cursor-pointer cosmic-focus-visible"
              title={copied ? 'Copied to clipboard!' : 'Copy Markdown content'}
              data-testid="copy-note-content"
            >
              {copied ? <Check className="w-4 h-4 text-emerald-400" /> : <Copy className="w-4 h-4" />}
            </button>
            <span className="sr-only" aria-live="polite">{copied ? 'Note content copied to clipboard' : ''}</span>
            <button
              onClick={onToggleMaximize}
              aria-label={isMaximized ? 'Restore compact view' : 'Maximize panel'}
              className="p-1.5 rounded-lg bg-gray-800/60 hover:bg-gray-700 text-slate-300 hover:text-white transition-colors cursor-pointer cosmic-focus-visible"
              title={isMaximized ? 'Restore compact view' : 'Maximize panel'}
              data-testid="toggle-maximize-note"
            >
              {isMaximized ? <Minimize2 className="w-4 h-4 text-indigo-400" /> : <Maximize2 className="w-4 h-4" />}
            </button>
            <button
              onClick={onClose}
              aria-label="Close note panel (Esc)"
              className="p-1.5 rounded-lg bg-gray-800/60 hover:bg-red-500/20 text-slate-300 hover:text-red-400 transition-colors cursor-pointer cosmic-focus-visible"
              title="Close panel (Esc)"
              data-testid="close-note-drawer"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>

        <h2 className="text-xl sm:text-2xl font-black text-white leading-tight tracking-tight wrap-break-word select-text cursor-text selection:bg-indigo-500/40 selection:text-white">
          {title}
        </h2>
        <div className="flex items-center gap-3 text-xs font-mono text-gray-400">
          <span className="flex items-center gap-1">
            <Orbit className="w-3.5 h-3.5 text-indigo-400" aria-hidden="true" />
            {hasDirectedRelations ? incoming.length + outgoing.length : activeNote.connections ?? 0} links
          </span>
          {activeNote.stellarDesc && <span className="text-gray-500 truncate">• {activeNote.stellarDesc}</span>}
          <div className="ml-auto flex items-center gap-1" aria-label="Text size controls">
            <button
              type="button"
              onClick={() => changeFontSize(-2)}
              aria-label="Decrease text size"
              title="Decrease text size"
              className="p-1 rounded-sm hover:bg-gray-800 text-gray-300 hover:text-white cosmic-focus-visible"
            ><Minus className="h-3.5 w-3.5" aria-hidden="true" /></button>
            <button
              type="button"
              onClick={() => changeFontSize(2)}
              aria-label="Increase text size"
              title="Increase text size"
              className="p-1 rounded-sm hover:bg-gray-800 text-gray-300 hover:text-white cosmic-focus-visible"
            ><Plus className="h-3.5 w-3.5" aria-hidden="true" /></button>
          </div>
        </div>
      </div>

      <div ref={scrollAreaRoot} className="flex-1 min-h-0 flex">
        <ScrollArea
          className="flex-1 min-h-0 select-text"
          onScrollCapture={(event) => {
            const target = event.target as HTMLElement;
            if (target.dataset.slot === 'scroll-area-viewport') onScrollPositionChange?.(target.scrollTop);
          }}
        >
          <div className={cn(
          'p-5 sm:p-6 pb-12 transition-all select-text cursor-text selection:bg-indigo-500/40 selection:text-white',
          isMaximized ? 'max-w-4xl mx-auto' : 'max-w-none',
          )}>
          {parsedDocument.metadata && Object.keys(parsedDocument.metadata).length > 0 && (
            <details className="mb-5 rounded-lg border border-gray-800 bg-gray-950/60 px-3 py-2 text-sm text-gray-300">
              <summary className="cursor-pointer font-medium text-indigo-300">Metadata</summary>
              <dl className="mt-3 grid grid-cols-[minmax(5rem,auto)_1fr] gap-x-3 gap-y-2">
                {Object.entries(parsedDocument.metadata).map(([key, value]) => (
                  <React.Fragment key={key}>
                    <dt className="font-mono text-xs text-gray-500">{key}</dt>
                    <dd className="wrap-break-word">{formatMetadataValue(value)}</dd>
                  </React.Fragment>
                ))}
              </dl>
            </details>
          )}

          {showTableOfContents && (
            <nav aria-label="Table of contents" className="mb-6 rounded-lg border border-indigo-500/20 bg-indigo-950/20 px-4 py-3">
              <div className="mb-2 text-xs font-semibold uppercase tracking-wide text-indigo-300">On this page</div>
              <ol className="space-y-1 text-sm">
                {headings.map((heading) => (
                  <li key={heading.id} style={{ paddingLeft: `${Math.max(0, heading.level - 2) * 12}px` }}>
                    <a
                      href={`#${heading.id}`}
                      onClick={(event) => navigateToAnchor(event, `#${heading.id}`)}
                      className="text-gray-300 hover:text-white"
                    >{heading.text}</a>
                  </li>
                ))}
              </ol>
            </nav>
          )}

          <div
            className={cn(
              'prose prose-invert prose-indigo prose-img:rounded-xl prose-headings:font-bold prose-a:text-indigo-400 hover:prose-a:text-indigo-300 max-w-none text-gray-200 select-text selection:bg-indigo-500/40 selection:text-white',
              isMaximized ? 'prose-base sm:prose-lg leading-relaxed' : 'leading-relaxed',
            )}
            style={{ fontSize }}
            data-testid="note-markdown"
            data-font-size={fontSize <= 12 ? 'small' : fontSize >= 16 ? 'large' : 'normal'}
          >
            <ReactMarkdown
              remarkPlugins={[remarkGfm]}
              components={markdownComponents}
            >
              {markdown}
            </ReactMarkdown>
          </div>

          {hasDirectedRelations ? (
            <div className="mt-8 grid gap-5 border-t border-gray-800/80 pt-4 sm:grid-cols-2">
              {([
                ['Incoming links', incoming],
                ['Outgoing links', outgoing],
              ] as const).map(([label, links]) => (
                <section key={label} aria-label={`${label} (${links.length})`}>
                  <h3 className="mb-2 flex items-center gap-1.5 text-[11px] font-mono font-semibold uppercase tracking-wider text-indigo-400">
                    <Compass className="h-3.5 w-3.5" aria-hidden="true" />
                    {label} ({links.length})
                  </h3>
                  <div className="flex flex-wrap gap-1.5">
                    {links.slice(0, 12).map((link) => (
                      <button
                        key={link.id}
                        onClick={() => onNavigateInternalLink(link.id)}
                        className="flex items-center gap-1.5 rounded-lg border border-gray-800 bg-gray-900/90 px-2.5 py-1 text-left text-xs font-mono text-gray-300 transition-all hover:border-indigo-500/50 hover:bg-indigo-950/60 hover:text-white"
                        title={`Navigate to ${link.title}`}
                      >
                        <span className="h-1.5 w-1.5 rounded-full" style={{ backgroundColor: link.color || '#6366f1' }} />
                        <span className="max-w-[140px] truncate">{link.title}</span>
                      </button>
                    ))}
                  </div>
                </section>
              ))}
            </div>
          ) : neighbors.length > 0 && (
            <section className="mt-8 border-t border-gray-800/80 pt-4" aria-label={`Immediate Stellar Neighbors (${neighbors.length})`}>
              <h3 className="mb-2 flex items-center gap-1.5 text-[11px] font-mono font-semibold uppercase tracking-wider text-indigo-400">
                <Compass className="h-3.5 w-3.5" aria-hidden="true" />
                Immediate Stellar Neighbors ({neighbors.length})
              </h3>
              <div className="flex flex-wrap gap-1.5">
                {neighbors.slice(0, 12).map((neighbor) => (
                  <button
                    key={neighbor.id}
                    onClick={() => onNavigateInternalLink(neighbor.id)}
                    className="flex items-center gap-1.5 rounded-lg border border-gray-800 bg-gray-900/90 px-2.5 py-1 text-left text-xs font-mono text-gray-300 transition-all hover:border-indigo-500/50 hover:bg-indigo-950/60 hover:text-white"
                    title={`Navigate to ${neighbor.title}`}
                  >
                    <span className="h-1.5 w-1.5 rounded-full" style={{ backgroundColor: neighbor.color || '#6366f1' }} />
                    <span className="max-w-[140px] truncate">{neighbor.title}</span>
                  </button>
                ))}
              </div>
            </section>
          )}
          </div>
        </ScrollArea>
      </div>
    </aside>
  );
};
