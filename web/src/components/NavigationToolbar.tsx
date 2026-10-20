import { createPortal } from 'react-dom';
import { ArrowLeft, ArrowRight, BookmarkPlus, Clock3, List, ListChecks, Search, X } from 'lucide-react';
import type { RecentNote, SavedSearch } from '../lib/navigationHelpers';
import type { HeaderDisclosure, HeaderLayout } from '../lib/headerLayout';

export interface NavigationToolbarProps {
  canBack: boolean;
  canForward: boolean;
  onBack: () => void;
  onForward: () => void;
  isExplorerOpen: boolean;
  onToggleExplorer: () => void;
  recentNotes: RecentNote[];
  savedSearches: SavedSearch[];
  searchQuery: string;
  onSelectRecent: (id: string) => void;
  onSelectSavedSearch: (query: string) => void;
  onSaveSearch: () => void;
  onRemoveSavedSearch: (query: string) => void;
  onOpenCapture?: () => void;
  onOpenReview?: () => void;
  disclosure: HeaderDisclosure;
  onDisclosureChange: (next: HeaderDisclosure) => void;
  layout: HeaderLayout;
}

export function NavigationToolbar(props: NavigationToolbarProps) {
  const { canBack, canForward, onBack, onForward, isExplorerOpen, onToggleExplorer,
    recentNotes, savedSearches, searchQuery, onSelectRecent, onSelectSavedSearch,
    onSaveSearch, onRemoveSavedSearch, onOpenReview,
    disclosure, onDisclosureChange, layout } = props;
  const compact = layout === 'compact';
  const full = layout === 'full';
  const button = 'pointer-events-auto group inline-flex min-h-9 min-w-9 shrink-0 items-center justify-center gap-1.5 rounded-md border border-transparent px-2 text-[11px] font-mono font-medium tracking-wide text-slate-300 transition-colors hover:border-indigo-500/40 hover:bg-white/5 hover:text-white cosmic-focus-visible disabled:cursor-not-allowed disabled:opacity-40 max-[767px]:min-h-11 max-[767px]:min-w-11';
  const icon = 'h-4 w-4 shrink-0 text-cyan-400';
  const trigger = (kind: 'recent' | 'saved', label: string) => <button type="button" className={`${button} ${disclosure === kind ? 'border-cyan-500/50 bg-cyan-500/10 text-cyan-200' : ''}`} onClick={() => onDisclosureChange(disclosure === kind ? null : kind)} aria-label={label} aria-expanded={disclosure === kind} aria-controls="knowledge-navigation-popover" title={label}>{kind === 'recent' ? <Clock3 className={icon} aria-hidden="true" /> : <Search className={icon} aria-hidden="true" />}<span className="hidden min-[768px]:inline">{label}</span></button>;
  const disclosureRoot = typeof document === 'undefined' ? null : document.querySelector('[data-hud-root]');
  const renderDisclosure = (content: React.ReactNode) => disclosureRoot ? createPortal(content, disclosureRoot) : content;
  return <>
    <nav aria-label="Knowledge navigation" className="flex min-w-0 items-center gap-0.5 whitespace-nowrap font-mono">
      {!compact && <>
        <button type="button" className={button} onClick={onBack} disabled={!canBack} aria-label="Back" title="Back"><ArrowLeft className="h-4 w-4" aria-hidden="true" /><span className="hidden min-[1280px]:inline">Back</span></button>
        <button type="button" className={button} onClick={onForward} disabled={!canForward} aria-label="Forward" title="Forward"><ArrowRight className="h-4 w-4" aria-hidden="true" /><span className="hidden min-[1280px]:inline">Forward</span></button>
        <span className="mx-1 h-5 w-px shrink-0 bg-white/15" aria-hidden="true" />
      </>}
      <button type="button" className={`${button} ${isExplorerOpen ? 'border-cyan-500/50 bg-cyan-500/10 text-cyan-200' : ''}`} onClick={() => { onDisclosureChange(null); onToggleExplorer(); }} aria-label="Notes" aria-expanded={isExplorerOpen} title="Notes"><List className={icon} aria-hidden="true" /><span className="hidden min-[1280px]:inline">Notes</span></button>
      {full && <>{trigger('recent', 'Recent')}{trigger('saved', 'Saved')}</>}
      {onOpenReview && full && <button type="button" className={button} onClick={() => { onDisclosureChange(null); onOpenReview(); }} aria-label="Review" title="Knowledge review"><ListChecks className={icon} aria-hidden="true" /><span>Review</span></button>}
      {!compact && !full && <span className="mx-1 h-5 w-px shrink-0 bg-white/15" aria-hidden="true" />}
    </nav>
    {disclosure === 'recent' || disclosure === 'saved' ? renderDisclosure(<section id="knowledge-navigation-popover" role="region" tabIndex={-1} aria-label={disclosure === 'recent' ? 'Recent notes' : 'Saved searches'} style={{ top: 'var(--hud-content-top, 128px)', maxHeight: 'calc(100dvh - var(--hud-content-top, 128px) - 8px)' }} className="pointer-events-auto fixed left-3 z-40 flex w-80 max-w-[calc(100vw-1.5rem)] flex-col overflow-hidden rounded-hud-lg border border-white/10 bg-surface-elevated/95 p-2 shadow-hud-deep backdrop-blur-deep">
      <div className="shrink-0 border-b border-white/10 px-2 pb-2 text-[10px] font-semibold uppercase tracking-widest text-cyan-400">{disclosure === 'recent' ? 'Recent notes' : 'Saved searches'}</div>
      <div className="min-h-0 max-h-64 overflow-y-auto pt-1 text-xs font-mono">
        {disclosure === 'recent' ? recentNotes.length ? recentNotes.map(note => <button key={note.id} type="button" className="block min-h-9 w-full truncate rounded-md px-2 py-2 text-left text-slate-200 hover:bg-white/5 hover:text-cyan-200 cosmic-focus-visible" title={note.id} onClick={() => { onSelectRecent(note.id); onDisclosureChange(null); }}>{note.title}</button>) : <p className="px-2 py-3 text-slate-400">Notes you open will appear here.</p> : <>
          <button type="button" className="inline-flex min-h-9 w-full items-center justify-center gap-2 rounded-md border border-cyan-500/30 px-2 text-cyan-200 hover:bg-cyan-500/10 disabled:opacity-40 cosmic-focus-visible" onClick={onSaveSearch} disabled={searchQuery.trim().length < 2}><BookmarkPlus className="h-4 w-4" aria-hidden="true" /> Save current search</button>
          {savedSearches.length ? savedSearches.map(search => <div key={search.query} className="mt-1 flex items-center gap-1"><button type="button" className="min-h-9 min-w-0 flex-1 truncate px-2 py-2 text-left text-slate-200 hover:text-cyan-200 cosmic-focus-visible" onClick={() => { onSelectSavedSearch(search.query); onDisclosureChange(null); }}>{search.query}</button><button type="button" className="min-h-9 shrink-0 px-2 text-[11px] text-slate-400 hover:text-white cosmic-focus-visible" aria-label={`Remove saved search ${search.query}`} onClick={() => onRemoveSavedSearch(search.query)}><X className="h-4 w-4" aria-hidden="true" /></button></div>) : <p className="px-2 py-3 text-slate-400">No saved searches yet.</p>}
        </>}
      </div>
    </section>) : null}
  </>;
}
