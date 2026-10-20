import React from 'react';
import { Orbit } from 'lucide-react';

interface SectorResetIndicatorProps {
  focusedSector: string | null;
  onReset: () => void;
}

export const SectorResetIndicator: React.FC<SectorResetIndicatorProps> = ({
  focusedSector,
  onReset,
}) => {
  if (!focusedSector) return null;

  return (
    <div className="absolute top-20 left-1/2 -translate-x-1/2 z-20">
      <button 
        onClick={onReset}
        aria-label="Return to full galaxy view"
        className="px-5 py-2 bg-indigo-600/90 hover:bg-indigo-500 text-white font-bold rounded-full shadow-[0_0_25px_rgba(79,70,229,0.5)] backdrop-blur-md transition-all flex items-center gap-2 uppercase tracking-widest text-xs border border-indigo-400/40 cursor-pointer cosmic-focus-visible"
        data-testid="return-to-galaxy-button"
      >
        <Orbit className="w-4 h-4" aria-hidden="true" />
        Return to Full Galaxy
      </button>
    </div>
  );
};
