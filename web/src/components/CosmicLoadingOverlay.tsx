import React from 'react';
import { Orbit } from 'lucide-react';

interface CosmicLoadingOverlayProps {
  isReady: boolean;
}

export const CosmicLoadingOverlay: React.FC<CosmicLoadingOverlayProps> = ({ isReady }) => {
  if (isReady) return null;

  return (
    <div data-testid="cosmic-loading-overlay" className="absolute inset-0 z-50 flex flex-col items-center justify-center bg-void">
      <Orbit className="w-16 h-16 text-indigo-500 animate-spin mb-6 opacity-80" />
      <h2 className="text-xl font-bold text-transparent bg-clip-text bg-linear-to-r from-blue-400 to-purple-600 animate-pulse">
        Synthesizing Artificial Gravity...
      </h2>
      <p className="text-gray-500 text-sm mt-2">Stabilizing Seldon perimeter and stellar constellations...</p>
    </div>
  );
};
