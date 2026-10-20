import React, { useEffect, useState } from 'react';
import { Dialog } from '@base-ui/react/dialog';
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import type { SearchResultItem } from '../types/galaxy';
import { isAutomationSatellite, stripSearchMarkup } from '../lib/searchHelpers';

interface CommandSearchDialogProps {
  isOpen: boolean;
  onClose: () => void;
  searchQuery: string;
  onSearchChange: (val: string) => void;
  activeFilter: string;
  onSelectFilter: (filter: string) => void;
  filteredResults: SearchResultItem[];
  onSelectResult: (result: SearchResultItem) => void;
  isLoading?: boolean;
  searchError?: string | null;
  hasSearched?: boolean;
  onSaveSearch?: () => void;
}

const SEARCH_FILTERS = ['All', 'People', 'Architecture', 'Daily', 'Sources'];

export const CommandSearchDialog: React.FC<CommandSearchDialogProps> = ({
  isOpen,
  onClose,
  searchQuery,
  onSearchChange,
  activeFilter,
  onSelectFilter,
  filteredResults,
  onSelectResult,
  isLoading = false,
  searchError = null,
  hasSearched = filteredResults.length > 0,
  onSaveSearch,
}) => {
  const [visibleCount, setVisibleCount] = useState(8);

  useEffect(() => setVisibleCount(8), [searchQuery, activeFilter]);

  const visibleResults = filteredResults.slice(0, visibleCount);
  const groups = [
    { title: 'Notes', results: visibleResults.filter(result => !result.isCat && !isAutomationSatellite(result)) },
    { title: 'Clusters', results: visibleResults.filter(result => result.isCat) },
    { title: 'Automations', results: visibleResults.filter(result => isAutomationSatellite(result)) },
  ].filter(group => group.results.length > 0);

  const renderSnippet = (snippet: string) => stripSearchMarkup(snippet).split(/(==.*?==)/g).map((part, index) => {
    if (part.startsWith('==') && part.endsWith('==')) {
      return <mark key={index} className="rounded-sm bg-amber-300/20 px-0.5 text-amber-200">{part.slice(2, -2)}</mark>;
    }
    return <React.Fragment key={index}>{part}</React.Fragment>;
  });

  return (
    <Dialog.Root open={isOpen} onOpenChange={(open) => { if (!open) onClose(); }}>
      <Dialog.Portal>
        <Dialog.Backdrop className="fixed inset-0 z-50 bg-black/60 backdrop-blur-xs" />
        <Dialog.Popup aria-modal="true" className="fixed left-1/2 top-20 z-50 w-[calc(100vw-2rem)] max-w-2xl -translate-x-1/2 rounded-2xl border border-indigo-500/40 bg-[#0c0d15]/95 shadow-hud-deep backdrop-blur-2xl overflow-hidden outline-hidden">
          <Dialog.Title className="sr-only">Search notes</Dialog.Title>
          <Dialog.Description className="sr-only">Find notes, clusters, and automations.</Dialog.Description>
        <Command className="bg-transparent" shouldFilter={false}>
          <CommandInput 
            placeholder="Search notes, Seldon satellites, or clusters..." 
            value={searchQuery} 
            onValueChange={onSearchChange} 
            className="border-none focus:ring-0 text-white placeholder-gray-500 py-4 text-sm"
          />
          
          <div
            role="group"
            aria-label="Filter search results by category"
            className="flex flex-wrap gap-2 px-4 py-2.5 border-b border-gray-800/50"
          >
            {SEARCH_FILTERS.map(f => (
              <button 
                key={f}
                type="button"
                onClick={() => onSelectFilter(f)}
                aria-pressed={activeFilter === f}
                className={`shrink-0 px-3 py-1 text-xs font-bold rounded-full transition-colors border cursor-pointer cosmic-focus-visible ${
                  activeFilter === f 
                    ? 'bg-indigo-500/20 text-indigo-300 border-indigo-500/50' 
                    : 'bg-transparent text-gray-500 border-gray-800 hover:text-gray-300 hover:bg-gray-800'
                }`}
              >
                {f}
              </button>
            ))}
          </div>

          <CommandList className="max-h-[50vh] p-2 custom-scrollbar">
            {searchError ? (
              <div role="alert" className="py-8 text-center text-rose-300 font-mono text-xs">{searchError}</div>
            ) : isLoading ? (
              <div role="status" className="py-8 text-center text-gray-400 font-mono text-xs">Searching notes and automations…</div>
            ) : filteredResults.length > 0 ? (
              <>
                {groups.map(group => (
                  <CommandGroup key={group.title} heading={group.title} className="text-gray-400">
                    {group.results.map((res, idx) => (
                  <CommandItem 
                    key={res.relPath ? `${res.relPath}-${idx}` : (res.id ? `${res.id}-${idx}` : `item-${idx}`)}
                    onSelect={() => onSelectResult(res)}
                    className="cursor-pointer mb-1 p-3 rounded-xl data-[selected=true]:bg-gray-800/80 data-[selected=true]:text-white text-white transition-colors"
                  >
                    <div className="flex flex-col gap-1 w-full">
                      <div className={`font-bold text-sm truncate ${isAutomationSatellite(res) ? 'text-emerald-400' : res.isCat ? 'text-purple-400' : 'text-indigo-400'}`}>
                        {res.title || res.relPath || res.id}
                      </div>
                      <div className="text-gray-400 text-xs line-clamp-2 leading-relaxed">{renderSnippet(res.snippet)}</div>
                      {res.relPath && !res.isCat && <div className="text-gray-500 text-[10px] truncate">{res.relPath}</div>}
                    </div>
                  </CommandItem>
                    ))}
                  </CommandGroup>
                ))}
                {filteredResults.length > visibleCount && (
                  <button type="button" className="w-full rounded-lg px-3 py-2 text-xs text-indigo-300 hover:bg-gray-800/70" onClick={() => setVisibleCount(count => count + 8)}>
                    Show more results
                  </button>
                )}
              </>
            ) : (
              <CommandEmpty className="py-8 text-center text-gray-500 font-mono text-xs">
                {searchQuery.trim().length < 2 ? 'Type at least 2 characters to search.' : hasSearched ? 'No results found.' : 'Search notes, clusters, and automations.'}
              </CommandEmpty>
            )}
          </CommandList>
          {onSaveSearch && <div className="flex justify-end border-t border-white/10 px-4 py-2">
            <button type="button" onClick={onSaveSearch} disabled={searchQuery.trim().length < 2} className="rounded-md border border-indigo-500/30 px-3 py-1.5 text-xs text-indigo-200 hover:bg-indigo-500/15 disabled:opacity-40 cosmic-focus-visible">Save search</button>
          </div>}
        </Command>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
};
