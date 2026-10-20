import React, { useEffect, useRef } from 'react';
import { X, Minus, Plus } from 'lucide-react';
import { STELLAR_SPECTRUM_LEGEND } from '../lib/stellarClassification';
import type { EdgeVisibilityMode } from '../lib/edgeFiltering';
import { activeViewPreset, applyViewPreset, type GalaxyViewPreset, type GalaxyViewState } from '../lib/galaxyView';

export interface ViewControlsPanelProps {
  isOpen: boolean;
  onClose: () => void;
  view: GalaxyViewState;
  onChange: (next: GalaxyViewState) => void;
  maxNebulas: number;
  onUpdateMaxNebulas: (value: number) => void;
}

const EDGE_MODES: Array<{ id: EdgeVisibilityMode; label: string; description: string }> = [
  { id: 'auto', label: 'Auto', description: 'Automatically adapts edge density to zoom level' },
  { id: 'bridges', label: 'Bridges', description: 'Shows connections between clusters and sectors' },
  { id: 'focus', label: 'Focus', description: 'Shows connections on hover or selection' },
  { id: 'all', label: 'All', description: 'Shows every visible graph connection' },
];

const OVERLAYS: Array<{ key: 'showSectorLabels' | 'showNebulaLabels' | 'isCosmicMode' | 'showRings'; label: string; description: string; testId: string }> = [
  { key: 'showSectorLabels', label: 'Sector beacons', description: 'Sector callouts and navigation', testId: 'toggle-cloak-sectors' },
  { key: 'showNebulaLabels', label: 'Nebula labels', description: 'Category and cluster labels', testId: 'toggle-cloak-nebulas' },
  { key: 'isCosmicMode', label: 'Cosmic photons', description: 'Animated traffic along visible links', testId: 'toggle-cloak-photons' },
  { key: 'showRings', label: 'Orbital rings', description: 'Perimeter and satellite rings', testId: 'toggle-cloak-perimeter' },
];

export const ViewControlsPanel: React.FC<ViewControlsPanelProps> = ({
  isOpen, onClose, view, onChange, maxNebulas, onUpdateMaxNebulas,
}) => {
  const panelRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!isOpen) return;
    const onKeyDown = (event: KeyboardEvent) => { if (event.key === 'Escape') onClose(); };
    const onMouseDown = (event: MouseEvent) => {
      const target = event.target as Element;
      if (target.closest('[data-testid="toggle-controls-btn"], [data-testid="visual-cloak-toggle"]')) return;
      if (!panelRef.current?.contains(target)) onClose();
    };
    document.addEventListener('keydown', onKeyDown);
    document.addEventListener('mousedown', onMouseDown);
    return () => {
      document.removeEventListener('keydown', onKeyDown);
      document.removeEventListener('mousedown', onMouseDown);
    };
  }, [isOpen, onClose]);

  if (!isOpen) return null;
  const activePreset = activeViewPreset(view);
  const update = <K extends keyof GalaxyViewState>(key: K, value: GalaxyViewState[K]) => onChange({ ...view, [key]: value });

  return (
    <div ref={panelRef} id="controls-popover" data-testid="controls-popover" aria-label="View settings" style={{ top: 'var(--hud-content-top, 128px)', maxHeight: 'calc(100dvh - var(--hud-content-top, 128px) - 1rem)' }} className="fixed right-4 sm:right-6 z-40 w-[min(22rem,calc(100vw-2rem))] overflow-y-auto rounded-2xl border border-indigo-500/40 bg-[#0c0d15]/95 p-4 text-xs text-slate-200 shadow-[0_0_40px_rgba(0,0,0,0.8)] backdrop-blur-2xl">
      <div className="mb-4 flex items-center justify-between border-b border-white/10 pb-2">
        <div>
          <div className="font-mono font-bold uppercase tracking-wider text-indigo-300">View</div>
          <p className="mt-1 text-[11px] text-slate-400">Choose a layout, then adjust individual layers.</p>
        </div>
        <button type="button" onClick={onClose} aria-label="Close view settings" className="rounded-sm p-1 text-slate-400 hover:bg-white/10 hover:text-white cosmic-focus-visible"><X className="h-4 w-4" /></button>
      </div>

      <div role="group" aria-label="View presets" className="mb-4 grid grid-cols-3 gap-1.5">
        {(['explore', 'read', 'present'] as GalaxyViewPreset[]).map((preset) => (
          <button key={preset} type="button" aria-label={`${preset[0].toUpperCase()}${preset.slice(1)} view`} aria-pressed={activePreset === preset} onClick={() => onChange(applyViewPreset(view, preset))} className={`rounded-lg border px-2 py-2 font-semibold capitalize cosmic-focus-visible ${activePreset === preset ? 'border-indigo-400 bg-indigo-600 text-white' : 'border-white/10 bg-white/5 text-slate-300 hover:bg-white/10'}`}>{preset}</button>
        ))}
      </div>

      <div className="mb-4 rounded-lg border border-white/10 bg-white/3 p-3">
        <div className="mb-2 font-semibold text-slate-200">Connections</div>
        <div className="grid grid-cols-4 gap-1" role="radiogroup" aria-label="Edge filtering mode">
          {EDGE_MODES.map((mode) => (
            <button key={mode.id} type="button" role="radio" aria-checked={view.edgeMode === mode.id} title={mode.description} data-testid={`edge-mode-${mode.id}`} onClick={() => update('edgeMode', mode.id)} className={`rounded-sm px-1 py-1.5 cosmic-focus-visible ${view.edgeMode === mode.id ? 'bg-indigo-600 font-semibold text-white' : 'bg-gray-800 text-slate-300 hover:bg-gray-700'}`}>{mode.label}</button>
          ))}
        </div>
        <p className="mt-2 text-[10px] text-slate-400">{EDGE_MODES.find((mode) => mode.id === view.edgeMode)?.description}</p>
      </div>

      <div className="mb-4 rounded-lg border border-white/10 bg-white/3 p-3">
        <div className="mb-2 font-semibold text-slate-200">Overlays</div>
        {OVERLAYS.map((overlay) => (
          <button key={overlay.key} type="button" data-testid={overlay.testId} aria-label={`Toggle ${overlay.label}`} aria-pressed={view[overlay.key]} title={overlay.description} onClick={() => update(overlay.key, !view[overlay.key])} className="mb-1 flex w-full items-center justify-between rounded-sm px-2 py-1.5 text-left hover:bg-white/10 cosmic-focus-visible">
            <span><span className="block font-medium">{overlay.label}</span><span className="text-[10px] text-slate-400">{overlay.description}</span></span>
            <span aria-hidden="true" className={`ml-2 rounded-sm border px-1.5 py-0.5 font-mono ${view[overlay.key] ? 'border-cyan-400/50 bg-cyan-500/20 text-cyan-200' : 'border-white/10 text-slate-500'}`}>{view[overlay.key] ? 'On' : 'Off'}</span>
          </button>
        ))}
      </div>

      <div className="mb-4 flex items-center justify-between rounded-lg border border-white/10 bg-white/3 px-3 py-2">
        <span><span className="block font-semibold">Nebula label limit</span><span className="text-[10px] text-slate-400">Maximum visible category labels</span></span>
        <span className="flex items-center gap-1.5">
          <button type="button" data-testid="decrease-max-nebulas" aria-label="Decrease maximum visible nebulas" onClick={() => onUpdateMaxNebulas(maxNebulas - 1)} className="rounded-sm bg-gray-800 p-1 cosmic-focus-visible"><Minus className="h-3 w-3" /></button>
          <span data-testid="max-nebulas-value" className="w-5 text-center font-mono text-indigo-300">{maxNebulas}</span>
          <button type="button" data-testid="increase-max-nebulas" aria-label="Increase maximum visible nebulas" onClick={() => onUpdateMaxNebulas(maxNebulas + 1)} className="rounded-sm bg-gray-800 p-1 cosmic-focus-visible"><Plus className="h-3 w-3" /></button>
        </span>
      </div>

      <button type="button" aria-label="Reduce motion" aria-pressed={view.reducedMotion} onClick={() => update('reducedMotion', !view.reducedMotion)} className="mb-4 flex w-full items-center justify-between rounded-lg border border-white/10 bg-white/3 px-3 py-2 text-left hover:bg-white/10 cosmic-focus-visible"><span><span className="block font-semibold">Reduce motion</span><span className="text-[10px] text-slate-400">Pause photons and shorten camera movement</span></span><span aria-hidden="true" className="text-indigo-300">{view.reducedMotion ? 'On' : 'Off'}</span></button>

      <details className="rounded-lg border border-white/10 bg-white/3 px-3 py-2">
        <summary className="cursor-pointer font-semibold">Stellar spectrum help</summary>
        <p className="mt-2 text-[10px] text-slate-400">Star colors describe recency and connection mass.</p>
        <div className="mt-2 grid grid-cols-2 gap-1">
          {STELLAR_SPECTRUM_LEGEND.map((item) => <span key={item.classification} title={`${item.label}: ${item.description}`} className="flex items-center gap-1"><span className="h-2 w-2 rounded-full" style={{ backgroundColor: item.color }} />{item.label}</span>)}
        </div>
      </details>
    </div>
  );
};
