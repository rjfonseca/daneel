import React, { useEffect, useRef } from 'react';
import { 
  Eye, 
  EyeOff, 
  X, 
  Layers, 
  Compass, 
  Sparkles, 
  Orbit, 
  ShieldAlert,
  Check
} from 'lucide-react';
import { HudCard } from './HudCard';
import { cn } from '@/lib/utils';

export interface VisualCloakPopoverProps {
  isOpen: boolean;
  onClose: () => void;
  showSectorLabels: boolean;
  onToggleSectorLabels: () => void;
  showNebulaLabels: boolean;
  onToggleNebulaLabels: () => void;
  isCosmicMode: boolean;
  onToggleCosmicMode: () => void;
  showRings: boolean;
  onToggleRings: () => void;
}

interface CloakLayerItem {
  id: string;
  testId: string;
  label: string;
  desc: string;
  icon: React.ReactNode;
  isActive: boolean;
  onToggle: () => void;
}

export const VisualCloakPopover: React.FC<VisualCloakPopoverProps> = ({
  isOpen,
  onClose,
  showSectorLabels,
  onToggleSectorLabels,
  showNebulaLabels,
  onToggleNebulaLabels,
  isCosmicMode,
  onToggleCosmicMode,
  showRings,
  onToggleRings,
}) => {
  const popoverRef = useRef<HTMLDivElement>(null);

  // Close on click outside or Escape
  useEffect(() => {
    if (!isOpen) return;
    const handleClickOutside = (e: MouseEvent) => {
      const target = e.target as Element | null;
      if (target?.closest('[data-testid="visual-cloak-toggle"]')) {
        return;
      }
      if (popoverRef.current && !popoverRef.current.contains(e.target as Node)) {
        onClose();
      }
    };
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose();
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [isOpen, onClose]);

  if (!isOpen) return null;

  const isZenSkyActive = !showSectorLabels && !showNebulaLabels && !isCosmicMode && !showRings;

  const handleToggleZenSky = () => {
    if (isZenSkyActive) {
      // Restore all sensor overlays
      if (!showSectorLabels) onToggleSectorLabels();
      if (!showNebulaLabels) onToggleNebulaLabels();
      if (!isCosmicMode) onToggleCosmicMode();
      if (!showRings) onToggleRings();
    } else {
      // Cloak all overlays for pure starscape observation
      if (showSectorLabels) onToggleSectorLabels();
      if (showNebulaLabels) onToggleNebulaLabels();
      if (isCosmicMode) onToggleCosmicMode();
      if (showRings) onToggleRings();
    }
  };

  const layers: CloakLayerItem[] = [
    {
      id: 'sectors',
      testId: 'toggle-cloak-sectors',
      label: 'Sector Beacons',
      desc: 'Galactic sector callouts & navigational beacons',
      icon: <Compass className="w-4 h-4 text-cyan-400" />,
      isActive: showSectorLabels,
      onToggle: onToggleSectorLabels,
    },
    {
      id: 'nebulas',
      testId: 'toggle-cloak-nebulas',
      label: 'Nebula Labels',
      desc: 'Thematic category tags & cluster clouds',
      icon: <Layers className="w-4 h-4 text-indigo-400" />,
      isActive: showNebulaLabels,
      onToggle: onToggleNebulaLabels,
    },
    {
      id: 'photons',
      testId: 'toggle-cloak-photons',
      label: 'Cosmic Photons',
      desc: 'Quantum telemetry packets in transit',
      icon: <Sparkles className="w-4 h-4 text-amber-400" />,
      isActive: isCosmicMode,
      onToggle: onToggleCosmicMode,
    },
    {
      id: 'perimeter',
      testId: 'toggle-cloak-perimeter',
      label: 'Seldon Perimeter',
      desc: 'Orbital foundation rings & satellite stations',
      icon: <Orbit className="w-4 h-4 text-purple-400" />,
      isActive: showRings,
      onToggle: onToggleRings,
    },
  ];

  return (
    <div 
      ref={popoverRef}
      data-testid="visual-cloak-popover"
      className="fixed top-16 right-4 sm:right-28 z-40 w-84 select-none animate-in fade-in slide-in-from-top-2 duration-150"
    >
      <HudCard
        variant="cyan"
        elevation="elevated"
        techCorners
        className="p-4 flex flex-col gap-3.5 shadow-hud-deep border-cyan-500/40"
      >
        {/* Header */}
        <div className="flex items-center justify-between border-b border-white/10 pb-2.5">
          <div className="flex items-center gap-2">
            <div className="p-1.5 rounded-md bg-cyan-950/60 border border-cyan-400/40 text-cyan-400 shadow-[0_0_8px_rgba(56,189,248,0.2)]">
              {isZenSkyActive ? (
                <EyeOff className="w-4 h-4 text-amber-400" />
              ) : (
                <Eye className="w-4 h-4 text-cyan-400" />
              )}
            </div>
            <div>
              <div className="flex items-center gap-1.5 text-xs font-mono font-bold uppercase tracking-wider text-cyan-300">
                VISUAL CLOAK
              </div>
              <div className="text-[9px] font-mono tracking-wide text-slate-400">
                Sensor Array & Holographic Filters
              </div>
            </div>
          </div>
          <button
            onClick={onClose}
            data-testid="close-visual-cloak"
            aria-label="Close Visual Cloak panel"
            className="p-1 rounded-md hover:bg-white/10 text-slate-400 hover:text-white transition-colors cursor-pointer cosmic-focus-visible"
            title="Close Visual Cloak"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Master Zen Sky Mode Button */}
        <button
          onClick={handleToggleZenSky}
          data-testid="toggle-zen-sky"
          aria-label={isZenSkyActive ? 'Deactivate Zen Sky mode' : 'Activate Zen Sky mode'}
          aria-pressed={isZenSkyActive}
          className={cn(
            "w-full flex items-center justify-between px-3 py-2 rounded-hud-md border transition-all text-xs font-mono cursor-pointer cosmic-focus-visible",
            isZenSkyActive
              ? "bg-amber-600/25 border-amber-500/80 text-amber-200 shadow-[0_0_12px_rgba(245,158,11,0.25)]"
              : "bg-white/4 border-white/10 hover:border-cyan-500/40 hover:bg-white/10 text-slate-300 hover:text-white"
          )}
          title="Zen Sky: Hide all holographic overlays and beacons for an unencumbered starry expanse"
        >
          <div className="flex items-center gap-2 text-left">
            <ShieldAlert className={cn("w-4 h-4", isZenSkyActive ? "text-amber-400" : "text-slate-400")} />
            <div>
              <div className="font-bold tracking-tight">ZEN SKY OBSERVATION</div>
              <div className="text-[9px] text-slate-400 font-sans">
                {isZenSkyActive ? 'All overlays cloaked (Clean starscape)' : 'Cloak all beacons and overlays'}
              </div>
            </div>
          </div>
          <span className={cn(
            "px-1.5 py-0.5 rounded-sm text-[9px] font-bold uppercase tracking-wider font-mono",
            isZenSkyActive ? "bg-amber-500/30 text-amber-300 border border-amber-400/50" : "bg-white/10 text-slate-400"
          )}>
            {isZenSkyActive ? 'ACTIVE' : 'OFF'}
          </span>
        </button>

        {/* Granular Sensor Toggles */}
        <div className="flex flex-col gap-1.5 pt-1" role="group" aria-label="Visual overlay toggles">
          {layers.map(layer => (
            <button
              key={layer.id}
              onClick={layer.onToggle}
              data-testid={layer.testId}
              aria-label={`Toggle ${layer.label}`}
              aria-pressed={layer.isActive}
              className={cn(
                "flex items-center justify-between p-2 rounded-hud-md border transition-all text-xs text-left cursor-pointer cosmic-focus-visible group",
                layer.isActive
                  ? "bg-cyan-950/20 border-cyan-500/40 hover:border-cyan-400 hover:bg-cyan-950/30"
                  : "bg-[#060810]/60 border-white/5 opacity-60 hover:opacity-100 hover:border-white/20"
              )}
            >
              <div className="flex items-center gap-2.5 truncate">
                <div className="p-1 rounded-sm bg-black/40 border border-white/10 shrink-0">
                  {layer.icon}
                </div>
                <div className="truncate">
                  <div className={cn(
                    "font-medium text-xs truncate transition-colors",
                    layer.isActive ? "text-white font-semibold" : "text-slate-400 group-hover:text-slate-200"
                  )}>
                    {layer.label}
                  </div>
                  <div className="text-[10px] text-slate-500 truncate font-mono">
                    {layer.desc}
                  </div>
                </div>
              </div>

              {/* State pill */}
              <div className={cn(
                "flex items-center justify-center w-6 h-6 rounded-md border shrink-0 ml-2 transition-all",
                layer.isActive
                  ? "bg-cyan-500/20 border-cyan-400 text-cyan-300 shadow-[0_0_8px_rgba(56,189,248,0.3)]"
                  : "bg-white/5 border-white/10 text-transparent"
              )}>
                <Check className={cn("w-3.5 h-3.5", layer.isActive ? "opacity-100" : "opacity-0")} />
              </div>
            </button>
          ))}
        </div>
      </HudCard>
    </div>
  );
};
