import React, { useEffect, useRef } from 'react';
import { HudCard } from './HudCard';
import { HotkeyBadge } from './HotkeyBadge';
import { X, Keyboard } from 'lucide-react';

export interface ShortcutsModalProps {
  isOpen: boolean;
  onClose: () => void;
}

const SHORTCUT_GROUPS = [
  {
    category: 'Navigation & Search',
    items: [
      { keys: ['⌘', 'K'], label: 'Global Search & Command Palette (Ctrl+K on Linux/Win)' },
      { keys: ['⌥', 'C'], label: 'Action Deck & Cockpit (Alt+C on Linux/Win)' },
      { keys: ['⌥', '1'], label: 'Jump to Today\'s Daily Log (Alt+1 on Linux/Win)' },
      { keys: ['⌥', '2'], label: 'Frame Current Week Cluster (Alt+2 on Linux/Win)' },
      { keys: ['Esc'], label: 'Close Active Panel / Deselect Focus / Dismiss Dialog' },
      { keys: ['?'], label: 'Toggle Keyboard Shortcuts Cheatsheet' },
    ],
  },
  {
    category: 'Galactic Viewport',
    items: [
      { keys: ['Click Callout'], label: 'Focus Sector, Repository Nebula, or Satellite' },
      { keys: ['Click Star'], label: 'Isolate Constellation & Open Inspector Drawer' },
      { keys: ['Wheel / Pinch'], label: 'Dynamic Zoom (Overview → Nebulas → Notes)' },
      { keys: ['Drag'], label: 'Pan Across Deep Space Coordinates' },
    ],
  },
];

export const ShortcutsModal: React.FC<ShortcutsModalProps> = ({ isOpen, onClose }) => {
  const modalRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!isOpen) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.stopPropagation();
        onClose();
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onClose]);

  if (!isOpen) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 animate-in fade-in duration-150"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      aria-labelledby="shortcuts-modal-title"
    >
      <div onClick={(e) => e.stopPropagation()} className="w-full max-w-lg">
        <HudCard
          ref={modalRef}
          variant="indigo"
          elevation="elevated"
          techCorners
          className="p-6 relative shadow-hud-deep"
        >
          {/* Header */}
          <div className="flex items-center justify-between pb-4 mb-4 border-b border-white/10">
            <div className="flex items-center gap-3">
              <div className="p-2 rounded-lg bg-indigo-500/20 text-indigo-300 border border-indigo-500/30">
                <Keyboard className="w-5 h-5" aria-hidden="true" />
              </div>
              <div>
                <h2 id="shortcuts-modal-title" className="text-base font-bold text-white font-mono tracking-wide">
                  COSMIC COMMAND INTERFACE
                </h2>
                <p className="text-xs text-slate-400">Keyboard Shortcuts & Navigation Affordances</p>
              </div>
            </div>
            <button
              onClick={onClose}
              aria-label="Close shortcuts dialog"
              className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-white/10 transition-colors cosmic-focus-visible"
            >
              <X className="w-5 h-5" aria-hidden="true" />
            </button>
          </div>

          {/* Body */}
          <div className="space-y-5">
            {SHORTCUT_GROUPS.map((group, gIdx) => (
              <div key={gIdx} className="space-y-2.5">
                <h3 className="text-[11px] font-mono uppercase tracking-wider text-indigo-400/90 font-semibold">
                  {group.category}
                </h3>
                <div className="space-y-1.5">
                  {group.items.map((item, iIdx) => (
                    <div
                      key={iIdx}
                      className="flex items-center justify-between p-2 rounded-lg bg-surface-muted/50 border border-white/5 hover:border-white/10 transition-colors"
                    >
                      <span className="text-xs text-slate-200">{item.label}</span>
                      <HotkeyBadge keys={item.keys} />
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>

          {/* Footer */}
          <div className="mt-6 pt-3 border-t border-white/10 flex items-center justify-between text-[11px] text-slate-400">
            <span>Daneel Prime Radiant HUD v1.0</span>
            <span>Press <HotkeyBadge keys={['Esc']} className="mx-1 inline-flex" /> to dismiss</span>
          </div>
        </HudCard>
      </div>
    </div>
  );
};
