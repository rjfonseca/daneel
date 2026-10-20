import { useEffect, useMemo, useState } from 'react';
import { X } from 'lucide-react';
import type { GalaxyNode } from '../types/galaxy';
import { HudCard } from './hud/HudCard';

interface KnowledgeExplorerProps {
  nodes: GalaxyNode[];
  scopeLabel: string;
  onSelectNode: (id: string) => void;
  onClose: () => void;
}

function noteType(node: GalaxyNode): string {
  const category = (node.category || '').toLocaleLowerCase();
  if (category.startsWith('daily')) return 'Daily';
  if (category.startsWith('sources')) return 'Source';
  return 'Note';
}

function updatedLabel(modTime: number | bigint | undefined): string {
  if (modTime === undefined) return 'Unknown date';
  const value = Number(modTime);
  if (!Number.isFinite(value) || value <= 0) return 'Unknown date';
  const date = new Date(value < 10_000_000_000 ? value * 1000 : value);
  return Number.isNaN(date.getTime()) ? 'Unknown date' : date.toLocaleDateString();
}

export function KnowledgeExplorer({ nodes, scopeLabel, onSelectNode, onClose }: KnowledgeExplorerProps) {
  const [query, setQuery] = useState('');
  const [visibleCount, setVisibleCount] = useState(50);
  useEffect(() => { setVisibleCount(50); }, [nodes, scopeLabel]);
  const filtered = useMemo(() => {
    const term = query.trim().toLocaleLowerCase();
    return term ? nodes.filter(node => [node.title, node.id, node.category].some(value => (value || '').toLocaleLowerCase().includes(term))) : nodes;
  }, [nodes, query]);
  const visible = filtered.slice(0, visibleCount);
  return (
    <HudCard role="complementary" aria-label="Note explorer" variant="default" elevation="elevated" style={{ top: 'var(--hud-content-top, 128px)' }} className="pointer-events-auto fixed left-3 bottom-20 z-30 flex w-[min(23rem,calc(100vw-1.5rem))] flex-col overflow-hidden font-mono">
      <header className="flex items-start justify-between gap-2 border-b border-white/10 px-4 py-3">
        <div><h2 className="text-xs font-semibold uppercase tracking-wider text-cyan-300">Notes</h2><p className="mt-1 text-[11px] text-slate-400">{scopeLabel} · {nodes.length} notes</p></div>
        <button type="button" className="flex min-h-9 min-w-9 items-center justify-center rounded-hud-md text-slate-400 hover:bg-white/5 hover:text-white cosmic-focus-visible" aria-label="Close note explorer" onClick={onClose}><X size={16} aria-hidden="true" /></button>
      </header>
      <label className="px-4 pt-3 text-xs text-slate-400" htmlFor="knowledge-explorer-search">Filter notes</label>
      <input id="knowledge-explorer-search" type="search" value={query} onChange={event => { setQuery(event.target.value); setVisibleCount(50); }} placeholder="Title, path or category" className="mx-4 mt-1 rounded-hud-md border border-white/15 bg-white/5 px-3 py-2 text-sm text-white outline-hidden focus:border-cyan-400" />
      <div role="list" aria-label="Notes" className="mt-2 min-h-0 flex-1 overflow-y-auto px-2 pb-3">
        {visible.length ? visible.map(node => <div key={node.id} role="listitem"><button type="button" aria-label={node.title || node.id} className="block w-full rounded-hud-md border border-transparent px-3 py-2.5 text-left hover:border-white/10 hover:bg-white/5 cosmic-focus-visible" onClick={() => onSelectNode(node.id)}>
          <span className="block truncate text-sm font-medium text-slate-100" title={node.title || node.id}>{node.title || node.id}</span>
          <span className="mt-1 flex flex-wrap gap-x-2 text-[11px] text-slate-400"><span>{noteType(node)}</span><span>Updated {updatedLabel(node.modTime)}</span><span>{node.connections || 0} connections</span></span>
        </button></div>) : <p className="px-3 py-8 text-center text-sm text-slate-400">No notes in this view.</p>}
      </div>
      <div className="flex items-center justify-between gap-2 border-t border-white/10 px-4 py-2 text-[11px] text-slate-400">
        <span>Showing {visible.length} of {filtered.length}</span>
        {visible.length < filtered.length && <button type="button" className="min-h-9 rounded-hud-md border border-cyan-500/30 px-2 text-cyan-200 hover:bg-cyan-500/10 cosmic-focus-visible" onClick={() => setVisibleCount(count => count + 50)}>Show more notes</button>}
      </div>
    </HudCard>
  );
}
