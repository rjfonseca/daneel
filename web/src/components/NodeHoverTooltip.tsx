import React from 'react';
import type { HoverTooltipState } from '../types/galaxy';
import { getCategoryDisplayName } from '../lib/nebulas';

interface NodeHoverTooltipProps {
  hoverTooltip: HoverTooltipState | null;
}

export const NodeHoverTooltip: React.FC<NodeHoverTooltipProps> = ({ hoverTooltip }) => {
  if (!hoverTooltip) return null;

  const posX = typeof window !== 'undefined' 
    ? Math.min(Math.max(10, hoverTooltip.pos.x + 15), window.innerWidth - 320)
    : hoverTooltip.pos.x + 15;
  const posY = typeof window !== 'undefined'
    ? Math.min(Math.max(10, hoverTooltip.pos.y - 15), window.innerHeight - 130)
    : hoverTooltip.pos.y - 15;

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
        <div className="flex items-center gap-3 text-[11px] text-gray-400 font-mono mt-0.5 border-t border-gray-800/80 pt-1.5">
          <span className="shrink-0">🔗 {hoverTooltip.node.connections ?? 0} {hoverTooltip.node.connections === 1 ? 'connection' : 'connections'}</span>
          {hoverTooltip.node.stellarDesc && (
            <span className="text-gray-500 truncate min-w-0">• {hoverTooltip.node.stellarDesc}</span>
          )}
        </div>
      </div>
    </div>
  );
};
