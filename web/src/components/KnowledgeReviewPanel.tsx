import { useMemo, useState } from 'react';
import { X } from 'lucide-react';
import { findOrphanNotes, getSuggestedConnections } from '../lib/graphReview';
import { isMemoryNote } from '../lib/navigationHelpers';
import { useReviewDecisions } from '../hooks/useReviewDecisions';
import type { GalaxyLink, GalaxyNode } from '../types/galaxy';

export interface KnowledgeReviewPanelProps {
  isOpen: boolean;
  onClose: () => void;
  nodes: GalaxyNode[];
  links: GalaxyLink[];
  unindexedLinks?: GalaxyLink[];
  onSelectNode: (id: string) => void;
}

type ReviewTab = 'orphans' | 'suggestions' | 'unindexed';
const PAGE_SIZE = 30;

function nodeLabel(node: GalaxyNode | undefined, fallback: string): string {
  return node?.title || node?.relPath || fallback;
}

export function KnowledgeReviewPanel({ isOpen, onClose, nodes, links, unindexedLinks = [], onSelectNode }: KnowledgeReviewPanelProps) {
  const [tab, setTab] = useState<ReviewTab>('orphans');
  const [shownOrphans, setShownOrphans] = useState(PAGE_SIZE);
  const [shownSuggestions, setShownSuggestions] = useState(PAGE_SIZE);
  const [focusId, setFocusId] = useState('');
  const [focusQuery, setFocusQuery] = useState('');
  const { decisions, save, ignore, reset } = useReviewDecisions();

  const notes = useMemo(() => nodes.filter(isMemoryNote).sort((a, b) => nodeLabel(a, a.id).localeCompare(nodeLabel(b, b.id)) || a.id.localeCompare(b.id)), [nodes]);
  const effectiveFocusId = notes.some(node => node.id === focusId) ? focusId : notes[0]?.id || '';
  const byId = useMemo(() => new Map(nodes.map(node => [node.id, node])), [nodes]);
  const orphans = useMemo(() => findOrphanNotes(nodes, links), [nodes, links]);
  const suggestions = useMemo(() => effectiveFocusId ? getSuggestedConnections(effectiveFocusId, nodes, links, 2_000) : [], [effectiveFocusId, nodes, links]);
  const focusDecisions = decisions.filter(item => item.focusId === effectiveFocusId);
  const decisionByCandidate = new Map(focusDecisions.map(item => [item.candidateId, item]));
  const visibleSuggestions = suggestions.filter(item => !decisionByCandidate.has(item.node.id));
  const reviewed = focusDecisions;
  const searchMatches = notes.filter(node => [node.title, node.id, node.relPath].some(value => (value || '').toLocaleLowerCase().includes(focusQuery.trim().toLocaleLowerCase()))).slice(0, 50);
  const selectOptions = searchMatches.some(node => node.id === effectiveFocusId)
    ? searchMatches
    : [notes.find(node => node.id === effectiveFocusId), ...searchMatches].filter((node): node is GalaxyNode => Boolean(node));

  const indexedSources = new Map(nodes.filter(isMemoryNote).map(node => [node.id, node]));
  const uniqueUnindexed = new Map<string, { source: GalaxyNode; target: string }>();
  for (const link of unindexedLinks) {
    const source = indexedSources.get(link.source);
    if (!source || byId.has(link.target)) continue;
    uniqueUnindexed.set(`${link.source}\u0000${link.target}`, { source, target: link.target });
  }

  if (!isOpen) return null;

  return (
    <aside aria-label="Knowledge review" style={{ top: 'var(--hud-content-top, 128px)' }} className="pointer-events-auto fixed right-3 bottom-20 z-40 flex w-[min(32rem,calc(100vw-1.5rem))] flex-col overflow-hidden rounded-xl border border-white/15 bg-[#0b0d19]/95 shadow-2xl backdrop-blur-md">
      <header className="flex items-start justify-between gap-2 border-b border-white/10 px-4 py-3">
        <div>
          <h2 className="text-sm font-semibold text-white">Knowledge review</h2>
          <p className="mt-1 text-xs text-slate-400">Structural review only. No notes are edited.</p>
        </div>
        <button type="button" className="rounded-sm p-1.5 text-slate-400 hover:bg-white/10 hover:text-white cosmic-focus-visible" aria-label="Close knowledge review" onClick={onClose}><X size={16} /></button>
      </header>

      <div role="tablist" aria-label="Knowledge review sections" className="flex border-b border-white/10 px-2">
        <button type="button" role="tab" aria-selected={tab === 'orphans'} className="px-3 py-2.5 text-xs text-slate-300 aria-selected:border-b-2 aria-selected:border-cyan-300 aria-selected:text-white" onClick={() => setTab('orphans')}>Orphans ({orphans.length})</button>
        <button type="button" role="tab" aria-selected={tab === 'suggestions'} className="px-3 py-2.5 text-xs text-slate-300 aria-selected:border-b-2 aria-selected:border-cyan-300 aria-selected:text-white" onClick={() => setTab('suggestions')}>Suggested connections</button>
        <button type="button" role="tab" aria-selected={tab === 'unindexed'} className="px-3 py-2.5 text-xs text-slate-300 aria-selected:border-b-2 aria-selected:border-cyan-300 aria-selected:text-white" onClick={() => setTab('unindexed')}>Unindexed targets ({uniqueUnindexed.size})</button>
      </div>

      {tab === 'orphans' && <section role="tabpanel" aria-label="Orphan notes" className="min-h-0 flex-1 overflow-y-auto p-3">
        <p className="px-2 pb-2 text-xs text-slate-400">Notes with no incoming or outgoing links in the current graph.</p>
        {orphans.length === 0 ? <p className="px-2 py-8 text-center text-sm text-slate-400">No orphan notes in the current graph.</p> : <div role="list">
          {orphans.slice(0, shownOrphans).map(node => <div key={node.id} role="listitem"><button type="button" className="block w-full rounded-lg px-3 py-2 text-left hover:bg-white/10 cosmic-focus-visible" onClick={() => onSelectNode(node.id)}>
            <span className="block truncate text-sm text-slate-100">{nodeLabel(node, node.id)}</span><span className="mt-0.5 block truncate text-[11px] text-slate-500">{node.relPath || node.id}</span>
          </button></div>)}
          {shownOrphans < orphans.length && <button type="button" className="mx-auto mt-2 block rounded-sm px-3 py-2 text-xs text-cyan-200 hover:bg-white/10" onClick={() => setShownOrphans(count => count + PAGE_SIZE)}>Load more orphan notes</button>}
        </div>}
      </section>}

      {tab === 'suggestions' && <section role="tabpanel" aria-label="Suggested connections" className="min-h-0 flex-1 overflow-y-auto p-3">
        <p className="px-2 pb-2 text-xs text-slate-400">Suggestions use shared graph neighbors as evidence. They do not imply semantic similarity.</p>
        <label htmlFor="knowledge-review-focus-search" className="px-2 text-xs text-slate-300">Search focus note</label>
        <input id="knowledge-review-focus-search" type="search" value={focusQuery} onChange={event => setFocusQuery(event.target.value)} placeholder="Title or path" className="mx-2 mt-1 block w-[calc(100%-1rem)] rounded-md border border-white/15 bg-white/5 px-3 py-2 text-sm text-white outline-hidden focus:border-cyan-400" />
        <label htmlFor="knowledge-review-focus" className="sr-only">Focus note</label>
        <select id="knowledge-review-focus" value={effectiveFocusId} onChange={event => setFocusId(event.target.value)} className="mx-2 mt-2 block w-[calc(100%-1rem)] rounded-md border border-white/15 bg-[#121522] px-3 py-2 text-sm text-white outline-hidden focus:border-cyan-400">
          {selectOptions.map(node => <option key={node.id} value={node.id}>{nodeLabel(node, node.id)}</option>)}
        </select>
        {visibleSuggestions.length === 0 ? <p className="px-2 py-7 text-center text-sm text-slate-400">No unreviewed structural suggestions for this note.</p> : <div className="mt-2" role="list" aria-label="Suggested notes">
          {visibleSuggestions.slice(0, shownSuggestions).map(item => <article key={item.node.id} role="listitem" className="m-1 rounded-lg border border-white/10 px-3 py-3">
            <h3 className="truncate text-sm font-medium text-white">{nodeLabel(item.node, item.node.id)}</h3>
            <p className="mt-1 text-xs text-slate-300">{item.reason}</p>
            <p className="mt-1 truncate text-[11px] text-slate-500">{item.node.relPath || item.node.id}</p>
            <div className="mt-2 flex gap-2">
              <button type="button" aria-label={`Save suggestion ${nodeLabel(item.node, item.node.id)}`} className="rounded-sm bg-cyan-900/70 px-2.5 py-1.5 text-xs text-cyan-100 hover:bg-cyan-800" onClick={() => save(effectiveFocusId, item.node.id)}>Save suggestion</button>
              <button type="button" aria-label={`Ignore ${nodeLabel(item.node, item.node.id)}`} className="rounded-sm px-2.5 py-1.5 text-xs text-slate-300 hover:bg-white/10" onClick={() => ignore(effectiveFocusId, item.node.id)}>Ignore</button>
            </div>
          </article>)}
          {shownSuggestions < visibleSuggestions.length && <button type="button" className="mx-auto mt-2 block rounded-sm px-3 py-2 text-xs text-cyan-200 hover:bg-white/10" onClick={() => setShownSuggestions(count => count + PAGE_SIZE)}>Load more suggestions</button>}
        </div>}
        {reviewed.length > 0 && <div aria-label="Reviewed suggestions" className="mt-3 border-t border-white/10 pt-3">
          <h3 className="px-2 text-xs font-semibold uppercase tracking-wide text-slate-400">Reviewed suggestions</h3>
          {reviewed.map(item => <div key={item.candidateId} className="flex items-center justify-between gap-2 rounded-sm px-2 py-2 text-xs">
            <span className="min-w-0 truncate text-slate-200">{nodeLabel(byId.get(item.candidateId), item.candidateId)} <span className="text-slate-500">· {item.decision === 'accepted' ? 'Saved for review' : 'Ignored'}</span></span>
            <button type="button" aria-label={`Reset decision for ${nodeLabel(byId.get(item.candidateId), item.candidateId)}`} className="shrink-0 rounded-sm px-2 py-1 text-cyan-200 hover:bg-white/10" onClick={() => reset(item.focusId, item.candidateId)}>Reset decision</button>
          </div>)}
        </div>}
        <p className="px-2 pb-2 pt-3 text-[11px] text-slate-500">Saved decisions stay in this browser. They do not create links or edit notes.</p>
      </section>}

      {tab === 'unindexed' && <section role="tabpanel" aria-label="Unindexed targets" className="min-h-0 flex-1 overflow-y-auto p-3">
        <p className="px-2 pb-2 text-xs text-slate-400">These link targets are not present in the current index. This does not confirm that a file is missing.</p>
        {uniqueUnindexed.size === 0 ? <p className="px-2 py-8 text-center text-sm text-slate-400">No unindexed targets in the current graph.</p> : [...uniqueUnindexed.entries()].map(([key, entry]) => <article key={key} className="m-1 rounded-lg border border-white/10 px-3 py-3">
          <h3 className="text-xs text-slate-400">Source</h3><p className="mt-0.5 truncate text-sm text-white">{nodeLabel(entry.source, entry.source.id)}</p><p className="truncate text-[11px] text-slate-500">{entry.source.relPath || entry.source.id}</p>
          <h3 className="mt-2 text-xs text-slate-400">Unindexed target</h3><p className="mt-0.5 break-all text-sm text-slate-200">{entry.target}</p>
          <button type="button" aria-label={`Open source ${nodeLabel(entry.source, entry.source.id)}`} className="mt-2 rounded-sm bg-white/10 px-2.5 py-1.5 text-xs text-slate-100 hover:bg-white/15" onClick={() => onSelectNode(entry.source.id)}>Open source</button>
        </article>)}
      </section>}
    </aside>
  );
}
