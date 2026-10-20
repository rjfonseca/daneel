import { NavigationToolbar } from './NavigationToolbar';
import React, { useEffect, useRef, useState } from 'react';
import { Activity, ChevronDown, Compass, Eye, EyeOff, Keyboard, Search, SlidersHorizontal, SquarePen, Terminal } from 'lucide-react';
import type { GalaxyStats } from '../gen/api/v1/graph_pb';
import { HudCard } from './hud/HudCard';
import { HotkeyBadge } from './hud/HotkeyBadge';
import { getSearchShortcut } from '../lib/searchShortcut';
import { resolveHeaderLayout, type HeaderDisclosure, type HeaderLayout } from '../lib/headerLayout';
import type { NavigationToolbarProps } from './NavigationToolbar';

export interface TopCosmicBarProps {
  stats?: GalaxyStats;
  onResetCamera: () => void;
  onOpenSearch: () => void;
  onToggleControls?: () => void;
  isControlsOpen?: boolean;
  activeSectorName?: string | null;
  activeClusterName?: string | null;
  onSelectSector?: () => void;
  onOpenShortcuts?: () => void;
  isCloakOpen?: boolean;
  onToggleCloak?: () => void;
  isZenSky?: boolean;
  onToggleCockpit?: () => void;
  isCockpitOpen?: boolean;
  navigation: Omit<NavigationToolbarProps, 'disclosure' | 'onDisclosureChange' | 'layout'>;
  disclosure: HeaderDisclosure;
  onDisclosureChange: (next: HeaderDisclosure) => void;
}

export const TopCosmicBar: React.FC<TopCosmicBarProps> = ({
  stats, onResetCamera, onOpenSearch, onToggleControls, isControlsOpen = false,
  activeSectorName, activeClusterName, onSelectSector, onOpenShortcuts,
  isCloakOpen = false, onToggleCloak, isZenSky = false, onToggleCockpit, isCockpitOpen = false,
  navigation, disclosure, onDisclosureChange,
}) => {
  const headerRef = useRef<HTMLElement>(null);
  const moreRef = useRef<HTMLButtonElement>(null);
  const [layout, setLayout] = useState<HeaderLayout>(() => resolveHeaderLayout(typeof window === 'undefined' ? 1440 : window.innerWidth));
  const shortcut = getSearchShortcut(typeof navigator === 'undefined' ? '' : navigator.platform);
  const [previousLayout, setPreviousLayout] = useState(layout);
  useEffect(() => {
    const onResize = () => {
      const next = resolveHeaderLayout(window.innerWidth);
      setLayout(current => current === next ? current : next);
    };
    window.addEventListener('resize', onResize);
    return () => window.removeEventListener('resize', onResize);
  }, []);
  useEffect(() => {
    if (layout === previousLayout) return;
    setPreviousLayout(layout);
    if (disclosure) {
      onDisclosureChange(null);
      requestAnimationFrame(() => moreRef.current?.focus());
    }
  }, [layout, previousLayout, disclosure, onDisclosureChange]);
  useEffect(() => {
    if (!disclosure) return;
    const frame = requestAnimationFrame(() => {
      const header = headerRef.current;
      if (!header) return;
      const target = disclosure === 'more'
        ? header.querySelector<HTMLElement>('[role="region"][aria-label="More actions"] button')
        : disclosure === 'view'
          ? document.querySelector<HTMLElement>('#controls-popover button')
        : disclosure === 'stats'
            ? header.querySelector<HTMLElement>('[role="region"][aria-label="Galaxy statistics"]')
            : document.querySelector<HTMLElement>('#knowledge-navigation-popover button') || document.querySelector<HTMLElement>('#knowledge-navigation-popover');
      (target || moreRef.current)?.focus();
    });
    return () => cancelAnimationFrame(frame);
  }, [disclosure]);
  useEffect(() => {
    const root = headerRef.current?.closest('[data-hud-root]') as HTMLElement | null;
    const header = headerRef.current;
    if (!root || !header) return;
    const publishBoundary = () => {
      const next = `${Math.ceil(header.getBoundingClientRect().bottom + 8)}px`;
      if (root.style.getPropertyValue('--hud-content-top') !== next) {
        root.style.setProperty('--hud-content-top', next);
      }
    };
    publishBoundary();
    if (typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver(publishBoundary);
    observer.observe(header);
    return () => observer.disconnect();
  }, []);
  useEffect(() => {
    if (!disclosure) return;
    const onPointer = (event: PointerEvent) => {
      const target = event.target as Element;
      if (!headerRef.current?.contains(target) && !target.closest('#controls-popover, #knowledge-navigation-popover')) onDisclosureChange(null);
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      event.preventDefault();
      event.stopPropagation();
      onDisclosureChange(null);
      if (disclosure === 'view') headerRef.current?.querySelector<HTMLElement>('[data-testid="toggle-controls-btn"]')?.focus();
      else if ((disclosure === 'recent' || disclosure === 'saved') && layout === 'full') {
        headerRef.current?.querySelector<HTMLElement>(`[aria-label="${disclosure === 'recent' ? 'Recent' : 'Saved'}"]`)?.focus();
      } else moreRef.current?.focus();
    };
    document.addEventListener('pointerdown', onPointer);
    document.addEventListener('keydown', onKey, true);
    return () => { document.removeEventListener('pointerdown', onPointer); document.removeEventListener('keydown', onKey, true); };
  }, [disclosure, layout, onDisclosureChange]);

  const button = 'pointer-events-auto inline-flex min-h-9 min-w-9 items-center justify-center gap-1.5 rounded-md border border-transparent px-2 text-[11px] font-medium text-slate-300 hover:border-white/15 hover:bg-white/5 hover:text-white cosmic-focus-visible max-[767px]:min-h-11 max-[767px]:min-w-11';
  const more = disclosure === 'more';
  const statsValues = [stats ? stats.totalNotes : '—', stats ? stats.totalLinks : '—', stats ? stats.harvesterCount : '—'];
  const labels = ['Stars', 'Filaments', 'Harvesters'];
  return <header ref={headerRef} className="pointer-events-none fixed left-3 right-3 top-3 z-50 select-none" data-testid="command-bridge">
    <HudCard variant="default" elevation="surface" className="pointer-events-none relative px-2 py-1.5 sm:px-3">
      <div className="flex min-h-10 items-center gap-2 border-b border-white/10 px-1 pb-1.5">
        <div className="flex min-w-0 flex-1 items-center gap-2">
          <span role="img" aria-label="Prime Radiant"><Activity className="h-4 w-4 shrink-0 text-cyan-400 min-[768px]:hidden" aria-hidden="true" /></span>
          <div className="hidden min-w-0 min-[768px]:block">
            <div className="font-black text-xs tracking-wider text-indigo-200">PRIME RADIANT</div>
          </div>
          <span className="hidden h-5 w-px bg-white/10 min-[768px]:block" aria-hidden="true" />
          <nav aria-label="Cosmic Breadcrumbs" className="flex min-w-0 items-center gap-1 truncate text-[10px] font-mono font-semibold uppercase tracking-wider">
            <button type="button" onClick={onResetCamera} className={`pointer-events-auto shrink-0 hover:text-white ${!activeSectorName ? 'text-cyan-400' : 'text-slate-400'}`} title="Return to full galaxy overview">Galaxy</button>
            {activeSectorName && <><span className="text-slate-600">/</span><button type="button" onClick={onSelectSector || onResetCamera} className={`pointer-events-auto min-w-0 truncate hover:text-white ${!activeClusterName ? 'text-cyan-400' : 'text-slate-400'}`} title={`Sector: ${activeSectorName}`} aria-label={`Sector: ${activeSectorName}`}>{activeSectorName}</button></>}
            {activeClusterName && <><span className="text-slate-600">/</span><span className="min-w-0 truncate text-cyan-300" title={`Nebula / Cluster: ${activeClusterName}`} aria-label={`Cluster: ${activeClusterName}`}>{activeClusterName}</span></>}
          </nav>
        </div>
        {layout === 'full' && <div className="flex shrink-0 items-center gap-3 text-[10px] font-mono text-slate-300" aria-label="Galaxy statistics">
          {labels.map((label, i) => <span key={label} title={`${label} in the local vault context`}>{label} <b className="text-white" data-testid={['cosmic-stat-notes','cosmic-stat-links','cosmic-stat-harvesters'][i]}>{statsValues[i]}</b></span>)}
        </div>}
        {layout === 'condensed' && <span className="shrink-0 text-[10px] font-mono text-slate-300">Stars <b data-testid="cosmic-stat-notes">{statsValues[0]}</b></span>}
        <button type="button" onClick={onResetCamera} data-testid="reset-camera-btn" aria-label="Center galaxy panoramic overview" title="Center galaxy" className={`${button} ml-auto shrink-0 text-cyan-200`}><Compass className="h-4 w-4" aria-hidden="true" /><span className="hidden min-[768px]:inline">Center</span></button>
      </div>
      <div className="flex min-h-10 items-center justify-between gap-1 px-1 pt-1.5">
        <div className="min-w-0"><NavigationInline navigation={navigation} disclosure={disclosure} onDisclosureChange={onDisclosureChange} layout={layout} /></div>
        <div className="flex shrink-0 items-center gap-0.5">
          <button type="button" onClick={() => { onDisclosureChange(null); onOpenSearch(); }} data-testid="quick-search-btn" aria-label={`Global search and command palette (${shortcut.label})`} title={`Search notes, satellites, or clusters (${shortcut.label})`} className={`${button} ${layout === 'compact' ? '' : 'px-2.5'}`}><Search className="h-4 w-4 text-indigo-400" aria-hidden="true" /><span className="hidden min-[768px]:inline">Search</span><HotkeyBadge keys={[shortcut.badge, 'K']} className="hidden min-[1280px]:inline-flex" /></button>
          <span className="mx-1 h-5 w-px shrink-0 bg-white/15" aria-hidden="true" />
          {navigation.onOpenCapture && <button type="button" data-testid="capture-note-btn" aria-label="Capture note" title="Capture note" className={`${button} text-cyan-200`} onClick={() => { onDisclosureChange(null); navigation.onOpenCapture?.(); }}><SquarePen className="h-4 w-4 text-cyan-300" aria-hidden="true" /><span className="hidden min-[768px]:inline">Capture</span></button>}
          {onToggleControls && <button type="button" onClick={() => onDisclosureChange(disclosure === 'view' ? null : 'view')} data-testid="toggle-controls-btn" aria-label="View settings" aria-expanded={isControlsOpen} aria-controls="controls-popover" title="View settings" className={button}><SlidersHorizontal className="h-4 w-4" aria-hidden="true" /><span className="hidden min-[768px]:inline">View</span></button>}
          <button ref={moreRef} type="button" onClick={() => onDisclosureChange(more ? null : 'more')} aria-label="More actions" aria-expanded={more} className={button}><ChevronDown className={`h-4 w-4 transition-transform ${more ? 'rotate-180' : ''}`} aria-hidden="true" /><span className="hidden min-[768px]:inline">More</span></button>
        </div>
      </div>
      {more && <div role="region" aria-label="More actions" className="pointer-events-auto absolute right-2 top-full z-40 mt-2 flex max-h-[min(60vh,32rem)] w-64 flex-col gap-1 overflow-y-auto rounded-hud-lg border border-white/10 bg-surface-elevated/95 p-2 shadow-hud-deep backdrop-blur-deep">
        {layout !== 'full' && <>
          {layout === 'compact' && <><button className={button} disabled={!navigation.canBack} onClick={navigation.onBack} aria-label="Back">Back</button><button className={button} disabled={!navigation.canForward} onClick={navigation.onForward} aria-label="Forward">Forward</button></>}
          <button className={button} onClick={() => onDisclosureChange('recent')} aria-label="Recent">Recent</button><button className={button} onClick={() => onDisclosureChange('saved')} aria-label="Saved">Saved</button>
          {navigation.onOpenReview && <button className={button} onClick={() => { onDisclosureChange(null); navigation.onOpenReview?.(); }} aria-label="Review">Review</button>}
        </>}
        {onToggleCockpit && <button type="button" data-testid="cockpit-toggle-btn" aria-label="Open Action Deck & Cockpit (Alt+C)" aria-expanded={isCockpitOpen} className={button} onClick={() => { moreRef.current?.focus(); onDisclosureChange(null); onToggleCockpit(); }}><Terminal className="h-4 w-4 text-cyan-400" />Cockpit</button>}
        {onOpenShortcuts && <button type="button" data-testid="shortcuts-help-btn" aria-label="Show keyboard shortcuts cheatsheet (?)" className={button} onClick={() => { moreRef.current?.focus(); onDisclosureChange(null); onOpenShortcuts(); }}><Keyboard className="h-4 w-4" />Keyboard shortcuts</button>}
        {onToggleCloak && <button type="button" data-testid="visual-cloak-toggle" aria-label="Visual cloak and sensor filters" aria-expanded={isCloakOpen} className={button} onClick={() => { onDisclosureChange(null); onToggleCloak(); }}>{isZenSky ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}Visual filters</button>}
        {layout !== 'full' && <button type="button" className={button} onClick={() => onDisclosureChange('stats')} aria-label="Galaxy statistics">Galaxy statistics</button>}
      </div>}
      {disclosure === 'stats' && <section role="region" aria-label="Galaxy statistics" tabIndex={-1} className="pointer-events-auto absolute right-3 top-full z-40 mt-2 rounded-hud-lg border border-white/10 bg-surface-elevated/95 p-3 text-xs shadow-hud-deep">{labels.map((label, i) => <p key={label} className="flex justify-between gap-6 py-1 text-slate-300"><span>{label}</span><b>{statsValues[i]}</b></p>)}</section>}
    </HudCard>
  </header>;
};

function NavigationInline({ navigation, disclosure, onDisclosureChange, layout }: {
  navigation: TopCosmicBarProps['navigation']; disclosure: HeaderDisclosure;
  onDisclosureChange: (next: HeaderDisclosure) => void; layout: HeaderLayout;
}) {
  return <div className="relative"><NavigationToolbar {...navigation} disclosure={disclosure} onDisclosureChange={onDisclosureChange} layout={layout} /></div>;
}
