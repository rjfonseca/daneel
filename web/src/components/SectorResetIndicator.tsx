import React from 'react';
import { Orbit } from 'lucide-react';
import { cn } from '@/lib/utils';

interface SectorResetIndicatorProps {
  focusedSector: string | null;
  onReset: () => void;
  isConstellationActive?: boolean;
  onResetConstellation?: () => void;
  hasActiveDrawer?: boolean;
  drawerWidth?: number;
  isObstructedBySidePanel?: boolean;
}

export const SectorResetIndicator: React.FC<SectorResetIndicatorProps> = ({
  focusedSector,
  onReset,
  isConstellationActive = false,
  onResetConstellation,
  hasActiveDrawer = false,
  drawerWidth = 440,
  isObstructedBySidePanel = false,
}) => {
  if (!focusedSector && !isConstellationActive) return null;
  if (isObstructedBySidePanel && !focusedSector) return null;

  const handleClick = focusedSector ? onReset : (onResetConstellation || onReset);

  return (
    <div
      className={cn(
        'fixed z-20 -translate-x-1/2 transition-all duration-300 ease-out',
        hasActiveDrawer ? 'left-1/2 lg:left-[calc((100vw-var(--drawer-width))/2)]' : 'left-1/2'
      )}
      style={{
        top: 'calc(var(--hud-content-top, 128px) + 12px)',
        '--drawer-width': `${drawerWidth}px`,
      } as React.CSSProperties}
    >
      <button
        onClick={handleClick}
        aria-label="Return to full galaxy view"
        className="px-5 py-2 bg-indigo-600/90 hover:bg-indigo-500 text-white font-bold rounded-full shadow-[0_0_25px_rgba(79,70,229,0.5)] backdrop-blur-md transition-all flex items-center gap-2 uppercase tracking-widest text-xs border border-indigo-400/40 cursor-pointer cosmic-focus-visible whitespace-nowrap"
        data-testid="return-to-galaxy-button"
      >
        <Orbit className="w-4 h-4" aria-hidden="true" />
        Return to Full Galaxy
      </button>
    </div>
  );
};
