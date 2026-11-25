import React from 'react';
import type { HoverTooltipState } from '../types/galaxy';
import { getCategoryDisplayName } from '../lib/nebulas';

interface NodeHoverTooltipProps {
  hoverTooltip: HoverTooltipState | null;
}

export const NodeHoverTooltip: React.FC<NodeHoverTooltipProps> = ({ hoverTooltip }) => {
  if (!hoverTooltip) return null;

  const posX = typeof window !== 'undefined' 
    ? Math.min(Math.max(12, hoverTooltip.pos.x + 18), window.innerWidth - 310)
    : hoverTooltip.pos.x + 18;
  const posY = typeof window !== 'undefined'
    ? (hoverTooltip.pos.y + 26 > window.innerHeight - 135
        ? Math.max(12, hoverTooltip.pos.y - 128)
        : Math.max(12, hoverTooltip.pos.y + 26))
    : hoverTooltip.pos.y + 26;

  const displayCategory = hoverTooltip.node.originalCategory 
    ? getCategoryDisplayName(hoverTooltip.node.originalCategory)
    : 'NOTES';

  return (
    <div 
      className="absolute z-50 pointer-events-none transition-all duration-150 ease-out"
      style={{ 
        left: posX, 
        top: posY 
      }}
      data-testid="node-hover-tooltip"
    >
      <div className="bg-[#13131a]/95 backdrop-blur-xl border border-indigo-500/50 p-3 rounded-xl shadow-[0_0_30px_rgba(79,70,229,0.3)] w-72 max-w-[320px] flex flex-col gap-1.5 overflow-hidden">
        <div className="flex items-center justify-between gap-2 overflow-hidden">
          <span 
            className="text-[10px] uppercase tracking-widest text-indigo-400 font-bold truncate flex-1 min-w-0"
            title={hoverTooltip.node.originalCategory || 'Notes'}
          >
            {displayCategory}
          </span>
          {hoverTooltip.node.stellarBadge && (
            <span 
              className="text-[10px] px-2 py-0.5 rounded-sm font-mono font-bold tracking-wide shrink-0 whitespace-nowrap"
              style={{ 
                backgroundColor: `${hoverTooltip.node.color || '#6366f1'}22`,
                color: hoverTooltip.node.color || '#818cf8',
                border: `1px solid ${hoverTooltip.node.color || '#6366f1'}55`
              }}
            >
              {hoverTooltip.node.stellarBadge}
            </span>
          )}
        </div>
        <div className="text-white font-semibold text-sm truncate flex items-center gap-1.5" title={hoverTooltip.node.title || hoverTooltip.node.id}>
          {hoverTooltip.node.icon && <span>{hoverTooltip.node.icon}</span>}
          <span className="truncate">{hoverTooltip.node.title || hoverTooltip.node.id}</span>
        </div>
        <div className="flex flex-col gap-1 text-[11px] text-gray-400 font-mono mt-0.5 border-t border-gray-800/80 pt-1.5">
          {hoverTooltip.node.isSuperNode ? (
            <>
              <span className="text-slate-200 truncate">
                {(hoverTooltip.node.repoCount ?? 0) > 0 ? `${hoverTooltip.node.repoCount} repositories · ` : ''}
                {hoverTooltip.node.noteCount ?? 0} stars
                {(hoverTooltip.node.recentCount ?? 0) > 0 ? ` · ${hoverTooltip.node.recentCount} active (7d)` : ''}
              </span>
              <span className="text-[10px] text-cyan-300 truncate">
                Click or zoom in to expand
                {(hoverTooltip.node.bridgeCount ?? 0) > 0 ? ` · ${hoverTooltip.node.bridgeCount} cross-bridges` : ''}
              </span>
            </>
          ) : (
            <div className="flex items-center gap-3">
              <span className="shrink-0">🔗 {hoverTooltip.node.connections ?? 0} {hoverTooltip.node.connections === 1 ? 'connection' : 'connections'}</span>
              {hoverTooltip.node.stellarDesc && (
                <span className="text-gray-500 truncate min-w-0">• {hoverTooltip.node.stellarDesc}</span>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
