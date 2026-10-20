import React from 'react';
import { 
  SlidersHorizontal, 
  X, 
  Layers, 
  Minus, 
  Plus, 
  Sparkles, 
  Radio 
} from 'lucide-react';
import { STELLAR_SPECTRUM_LEGEND } from '../lib/stellarClassification';
import type { EdgeVisibilityMode } from '../lib/edgeFiltering';

interface CosmicControlsPopoverProps {
  isOpen: boolean;
  onClose: () => void;
  maxNebulas: number;
  onUpdateMaxNebulas: (val: number) => void;
  edgeMode: EdgeVisibilityMode;
  onChangeEdgeMode: (mode: EdgeVisibilityMode) => void;
}

const EDGE_MODES = [
  { 
    id: 'auto' as const, 
    label: 'Auto', 
    desc: 'Dynamic LOD · Automatically adapts edge density based on zoom altitude' 
  },
  { 
    id: 'bridges' as const, 
    label: 'Bridges', 
    desc: 'Inter-cluster Bridges · Shows only gravity bridges and cross-sector connections' 
  },
  { 
    id: 'focus' as const, 
    label: 'Focus', 
    desc: 'Interactive Focus · Keeps galaxy clean, highlighting connections only on hover or focus' 
  },
  { 
    id: 'all' as const, 
    label: 'All', 
    desc: 'Full Mesh · Permanently displays all neural connections across the knowledge graph' 
  },
];

export const CosmicControlsPopover: React.FC<CosmicControlsPopoverProps> = ({
  isOpen,
  onClose,
  maxNebulas,
  onUpdateMaxNebulas,
  edgeMode,
  onChangeEdgeMode,
}) => {
  if (!isOpen) return null;

  return (
    <div 
      data-testid="controls-popover"
      className="fixed top-16 right-4 sm:right-6 z-40 w-80 bg-[#0c0d15]/95 backdrop-blur-2xl border border-indigo-500/40 rounded-2xl shadow-[0_0_40px_rgba(0,0,0,0.8)] p-4 flex flex-col gap-3.5 animate-in fade-in slide-in-from-top-2 duration-150"
    >
      <div className="flex items-center justify-between border-b border-gray-800/80 pb-2">
        <span className="text-xs font-mono font-bold uppercase tracking-wider text-indigo-300 flex items-center gap-1.5">
          <SlidersHorizontal className="w-3.5 h-3.5 text-indigo-400" />
          Cosmic Controls
        </span>
        <button 
          onClick={onClose}
          aria-label="Close Controls"
          className="p-1 rounded-lg hover:bg-gray-800 text-slate-400 hover:text-white cursor-pointer cosmic-focus-visible"
          title="Close Controls"
        >
          <X className="w-3.5 h-3.5" />
        </button>
      </div>

      {/* Stepper control for Nebula Labels */}
      <div className="flex items-center justify-between px-3 py-2 bg-gray-900/80 rounded-lg border border-gray-800/80 text-xs">
        <span className="flex items-center gap-1.5 font-medium text-slate-300" title="Adjust maximum rendered nebula label density">
          <Layers className="w-3.5 h-3.5 text-indigo-400" />
          Nebula Limit:
        </span>
        <div className="flex items-center gap-1.5">
          <button 
            onClick={() => onUpdateMaxNebulas(maxNebulas - 1)}
            aria-label="Decrease maximum visible nebulas"
            className="w-6 h-6 flex items-center justify-center rounded-sm bg-gray-800 hover:bg-gray-700 text-slate-300 hover:text-white transition-colors cursor-pointer cosmic-focus-visible"
            title="Decrease nebula limit"
            data-testid="decrease-max-nebulas"
          >
            <Minus className="w-3 h-3" />
          </button>
          <span className="font-mono font-bold text-indigo-300 w-6 text-center text-sm" data-testid="max-nebulas-value">
            {maxNebulas}
          </span>
          <button 
            onClick={() => onUpdateMaxNebulas(maxNebulas + 1)}
            aria-label="Increase maximum visible nebulas"
            className="w-6 h-6 flex items-center justify-center rounded-sm bg-gray-800 hover:bg-gray-700 text-slate-300 hover:text-white transition-colors cursor-pointer cosmic-focus-visible"
            title="Increase nebula limit"
            data-testid="increase-max-nebulas"
          >
            <Plus className="w-3 h-3" />
          </button>
        </div>
      </div>

      {/* Edge Mode Filter */}
      <div className="flex flex-col gap-1.5 px-3 py-2 bg-gray-900/80 rounded-lg border border-gray-800/80 text-xs">
        <div className="flex items-center justify-between">
          <span className="flex items-center gap-1.5 font-medium text-slate-300" title="Filter gravitational knowledge bridges and inter-cluster connections">
            <Sparkles className="w-3.5 h-3.5 text-indigo-400" />
            Edge Filtering:
          </span>
          <span className="font-mono text-[10px] text-indigo-300 font-semibold uppercase">
            {edgeMode === 'auto' ? 'Dynamic LOD' : edgeMode}
          </span>
        </div>
        <div className="grid grid-cols-4 gap-1 mt-1" role="radiogroup" aria-label="Edge filtering mode">
          {EDGE_MODES.map(m => (
            <button
              key={m.id}
              role="radio"
              aria-checked={edgeMode === m.id}
              onClick={() => onChangeEdgeMode(m.id)}
              title={m.desc}
              className={`px-2 py-1 rounded-sm text-[11px] font-medium transition-colors cursor-pointer cosmic-focus-visible ${
                edgeMode === m.id
                  ? 'bg-indigo-600 text-white shadow-xs font-semibold'
                  : 'bg-gray-800/80 text-slate-400 hover:text-slate-200 hover:bg-gray-700'
              }`}
              data-testid={`edge-mode-${m.id}`}
            >
              {m.label}
            </button>
          ))}
        </div>
      </div>

      {/* Cosmic Spectral Legend */}
      <div className="px-3 py-2 bg-gray-900/80 rounded-lg border border-gray-800/80 text-xs flex flex-col gap-1.5">
        <div className="flex items-center justify-between text-gray-400 font-medium">
          <span className="flex items-center gap-1.5">
            <Radio className="w-3.5 h-3.5 text-cyan-400" />
            Stellar Spectrum:
          </span>
          <span className="text-[10px] text-gray-500 font-mono">Recency & Mass</span>
        </div>
        <div className="grid grid-cols-4 gap-1 pt-1 border-t border-gray-800/60 text-[9px] font-mono">
          {STELLAR_SPECTRUM_LEGEND.map(s => (
            <div key={s.classification} className="flex items-center gap-1 text-gray-300 truncate" title={`${s.label}: ${s.description}`}>
              <span className="w-2 h-2 rounded-full shrink-0" style={{ backgroundColor: s.color }} />
              <span className="truncate">{s.badge.replace(/^[^\s]+\s/, '')}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
};
