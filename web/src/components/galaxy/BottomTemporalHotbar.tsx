import React, { useMemo, useEffect, useCallback, useState, useRef } from 'react';
import { CalendarDays, ChevronDown, ChevronLeft, ChevronRight, X } from 'lucide-react';
import { GalaxyNode } from '../../types/galaxy';
import { HudCard } from '../hud/HudCard';
import { cn } from '@/lib/utils';
import { usePinnedAnchors, type PinnedAnchorItem } from '@/hooks/usePinnedAnchors';
import type { PinnedAnchor } from '@/gen/api/v1/graph_pb';
import { addCalendarDays, getISOWeekInfo, getMonday, parseLocalDateString, toLocalDateString } from '@/lib/calendarDate';

export interface BottomTemporalHotbarProps {
  nodes: GalaxyNode[];
  onSelectNode: (nodeId: string) => void;
  onFocusWeek?: (weekNodeIds: string[]) => void;
  onToast?: (title: string, message: string) => void;
  isMaximized?: boolean;
  hasActiveDrawer?: boolean;
  drawerWidth?: number;
  backendAnchors?: PinnedAnchor[];
  weeklyPattern?: string;
}

interface DayInfo {
  dateStr: string;
  dayNum: number;
  dayLabel: string;
  isToday: boolean;
  isFuture: boolean;
  matchingNode: GalaxyNode | null;
  title: string | null;
}

const isDailyMatch = (path: string, dateStr: string) => {
  return path === `daily/${dateStr}.md` || path === `daily/${dateStr}`;
};

export const BottomTemporalHotbar: React.FC<BottomTemporalHotbarProps> = ({
  nodes,
  onSelectNode,
  onFocusWeek: _onFocusWeek,
  onToast,
  isMaximized,
  hasActiveDrawer,
  drawerWidth = 440,
  backendAnchors = [],
  weeklyPattern: _weeklyPattern,
}) => {
  const [isOverflowOpen, setIsOverflowOpen] = useState(false);
  const overflowRef = useRef<HTMLDivElement>(null);

  const [today, setToday] = useState(() => new Date());
  const followsTodayRef = useRef(true);
  const todayStr = useMemo(() => toLocalDateString(today), [today]);
  const [weekStart, setWeekStart] = useState(() => getMonday(today));
  const [selectedDate, setSelectedDate] = useState(() => toLocalDateString(today));
  const { week, year } = useMemo(() => getISOWeekInfo(weekStart), [weekStart]);

  const refreshToday = useCallback(() => {
    const currentToday = new Date();
    const currentTodayStr = toLocalDateString(currentToday);
    setToday((previous) => toLocalDateString(previous) === currentTodayStr ? previous : currentToday);
    if (followsTodayRef.current) {
      setWeekStart(getMonday(currentToday));
      setSelectedDate(currentTodayStr);
    }
  }, []);

  useEffect(() => {
    const now = new Date();
    const nextMidnight = new Date(now.getFullYear(), now.getMonth(), now.getDate() + 1);
    const timeout = window.setTimeout(refreshToday, Math.max(0, nextMidnight.getTime() - now.getTime()) + 10);
    window.addEventListener('focus', refreshToday);
    document.addEventListener('visibilitychange', refreshToday);
    return () => {
      window.clearTimeout(timeout);
      window.removeEventListener('focus', refreshToday);
      document.removeEventListener('visibilitychange', refreshToday);
    };
  }, [todayStr, refreshToday]);

  // Compute 7 days for current week (Monday to Sunday)
  const days: DayInfo[] = useMemo(() => {
    const dayLabels = ['MON', 'TUE', 'WED', 'THU', 'FRI', 'SAT', 'SUN'];

    const result: DayInfo[] = [];
    for (let i = 0; i < 7; i++) {
      const d = addCalendarDays(weekStart, i);
      const dateStr = toLocalDateString(d);
      const isToday = dateStr === todayStr;
      const isFuture = dateStr > todayStr;

      // Find matching daily note in vault nodes
      const matchingNode = nodes.find(n => {
        const id = n.id || '';
        const rel = n.relPath || '';
        return isDailyMatch(id, dateStr) || isDailyMatch(rel, dateStr);
      }) || null;

      result.push({
        dateStr,
        dayNum: d.getDate(),
        dayLabel: dayLabels[i],
        isToday,
        isFuture,
        matchingNode,
        title: matchingNode?.title || null,
      });
    }
    return result;
  }, [weekStart, todayStr, nodes]);

  const selectedDay = useMemo(() => {
    const node = nodes.find(n => isDailyMatch(n.id || '', selectedDate) || isDailyMatch(n.relPath || '', selectedDate)) || null;
    return { node, title: node?.title || null };
  }, [nodes, selectedDate]);

  const openDailyNote = useCallback((dateStr: string, action: 'open' | 'focus' = 'open') => {
    const matchingNode = nodes.find(n => isDailyMatch(n.id || '', dateStr) || isDailyMatch(n.relPath || '', dateStr));
    if (matchingNode) {
      onSelectNode(matchingNode.id);
      onToast?.('Galactic Chronometer', `${action === 'focus' ? "Focusing today's daily log" : 'Opening daily log'}: ${matchingNode.id}`);
    } else {
      onToast?.('Galactic Chronometer', `No daily note found for ${dateStr}. Expected file: daily/${dateStr}.md.`);
    }
  }, [nodes, onSelectNode, onToast]);

  const changeWeek = useCallback((amount: number) => {
    followsTodayRef.current = false;
    setWeekStart(current => addCalendarDays(current, amount * 7));
    setSelectedDate(current => {
      const [year, month, day] = current.split('-').map(Number);
      return toLocalDateString(addCalendarDays(new Date(year, month - 1, day), amount * 7));
    });
  }, []);

  const returnToToday = useCallback(() => {
    const currentToday = new Date();
    const currentTodayStr = toLocalDateString(currentToday);
    followsTodayRef.current = true;
    setToday(currentToday);
    setWeekStart(getMonday(currentToday));
    setSelectedDate(currentTodayStr);
    openDailyNote(currentTodayStr, 'focus');
  }, [openDailyNote]);

  // Seldon Anchors Hook
  const { anchors, removePin } = usePinnedAnchors({ backendAnchors });

  // Max 3 custom anchors visible directly in the hotbar
  const visibleAnchors = useMemo(() => anchors.slice(0, 3), [anchors]);
  const overflowAnchors = useMemo(() => anchors.slice(3), [anchors]);

  const handleToday = useCallback(() => {
    const currentTodayStr = toLocalDateString(new Date());
    refreshToday();
    openDailyNote(currentTodayStr, 'focus');
  }, [openDailyNote, refreshToday]);

  const handleDayClick = useCallback((day: DayInfo) => {
    followsTodayRef.current = false;
    setSelectedDate(day.dateStr);
    openDailyNote(day.dateStr);
  }, [openDailyNote]);

  const handleAnchorClick = useCallback((anchor: PinnedAnchorItem) => {
    const matchingNode = nodes.find(n => n.id === anchor.path || n.relPath === anchor.path);
    if (matchingNode) {
      onSelectNode(matchingNode.id);
      onToast?.('Seldon Vault Anchor', `Focusing anchored note: ${anchor.label}`);
    } else {
      // Still try to open via path
      onSelectNode(anchor.path);
      onToast?.('Seldon Vault Anchor', `Opening: ${anchor.label}`);
    }
  }, [nodes, onSelectNode, onToast]);

  // Global keydown listeners for shortcuts '1' / 'Alt+1' and '2' / 'Alt+2'
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement | null;
      if (target && typeof target.tagName === 'string') {
        const tag = target.tagName.toLowerCase();
        if (tag === 'input' || tag === 'textarea' || tag === 'select' || target.isContentEditable) {
          return;
        }
      }

      // Supports both direct '1' / '2' and 'Alt+1' / 'Alt+2'
      if (e.key === '1' || (e.altKey && e.key === '1')) {
        e.preventDefault();
        handleToday();
      } else if (e.key === '2' || (e.altKey && e.key === '2')) {
        if (visibleAnchors.length > 0) {
          e.preventDefault();
          handleAnchorClick(visibleAnchors[0]);
        }
      } else if (e.key === 'Escape') {
        setIsOverflowOpen(false);
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [handleToday, handleAnchorClick, visibleAnchors]);

  // Close overflow on click outside
  useEffect(() => {
    if (!isOverflowOpen) return;
    const handleClickOutside = (e: MouseEvent) => {
      if (overflowRef.current && !overflowRef.current.contains(e.target as Node)) {
        setIsOverflowOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, [isOverflowOpen]);

  if (isMaximized) return null;

  return (
    <footer 
      className={cn(
        "fixed bottom-6 z-20 w-max max-w-[calc(100vw-1.5rem)] pointer-events-auto select-none transition-all duration-300 ease-out",
        hasActiveDrawer
          ? "left-1/2 -translate-x-1/2 lg:left-[calc((100vw-var(--drawer-width))/2)] lg:max-w-[calc(100vw-var(--drawer-width)-3rem)]"
          : "left-1/2 -translate-x-1/2"
      )}
      style={{ '--drawer-width': `${drawerWidth}px` } as React.CSSProperties}
      data-testid="bottom-temporal-hotbar"
    >
      <HudCard
        variant="default"
        elevation="surface"
        className="w-full min-w-0 px-3.5 py-1.5 flex flex-wrap justify-center items-center gap-3 text-xs"
      >
        {/* Temporal Relay Micro Header */}
        <div className="flex items-center gap-2.5 pr-3 border-r border-white/10">
          <div className="relative w-6 h-6 flex items-center justify-center">
            <div className="absolute inset-0 border border-cyan-400/30 rounded-full animate-spin animation-duration-[12s] motion-reduce:animate-none" />
            <span className="text-cyan-400 font-mono text-xs">◈</span>
          </div>
          <div>
            <div className="text-[9px] font-mono tracking-widest text-cyan-400/90 uppercase font-semibold leading-none">
              TEMPORAL RELAY
            </div>
            <div className="text-xs font-bold font-mono tracking-tight text-white whitespace-nowrap mt-0.5" data-testid="temporal-week-label">
              CYCLE W{week} · {year}
            </div>
          </div>
        </div>

        <div className="flex min-w-0 max-w-full flex-wrap items-center gap-1" role="group" aria-label="Week navigation and date selection">
          <button type="button" data-testid="temporal-previous-week" aria-label="Previous week" title="Previous week" onClick={() => changeWeek(-1)} className="min-h-9 min-w-9 flex items-center justify-center rounded-md border border-white/10 text-slate-300 hover:text-white hover:bg-white/5 cosmic-focus-visible">
            <ChevronLeft className="w-4 h-4" aria-hidden="true" />
          </button>
          <button type="button" data-testid="temporal-next-week" aria-label="Next week" title="Next week" onClick={() => changeWeek(1)} className="min-h-9 min-w-9 flex items-center justify-center rounded-md border border-white/10 text-slate-300 hover:text-white hover:bg-white/5 cosmic-focus-visible">
            <ChevronRight className="w-4 h-4" aria-hidden="true" />
          </button>
          <button type="button" data-testid="temporal-today" onClick={returnToToday} className="min-h-9 px-3 rounded-md border border-cyan-500/30 text-cyan-200 hover:bg-cyan-500/10 cosmic-focus-visible">Today</button>
          <div className="flex min-h-9 items-center gap-1.5 rounded-md border border-white/10 bg-surface px-2">
            <CalendarDays className="h-4 w-4 shrink-0 text-cyan-300" aria-hidden="true" />
            <label className="text-[9px] font-semibold tracking-wide text-slate-400" htmlFor="temporal-date-picker">OPEN DATE</label>
            <input
              id="temporal-date-picker"
              type="date"
              aria-label="Choose a date to open a daily note"
              value={selectedDate}
              onChange={(event) => {
                const date = parseLocalDateString(event.target.value);
                if (!date) return;
                const value = toLocalDateString(date);
                followsTodayRef.current = false;
                setSelectedDate(value);
                setWeekStart(getMonday(date));
                openDailyNote(value);
              }}
              className="min-h-9 w-40 min-w-40 rounded-md border-0 bg-transparent px-1 text-slate-200 cursor-pointer focus-visible:outline-hidden"
              style={{ colorScheme: 'dark' }}
              title="Choose a date to open its daily note"
            />
          </div>
        </div>

        {/* Weekly Timeline Pills (Mon -> Sun) */}
        <div className="flex items-center gap-1" role="group" aria-label="Weekly timeline">
          {days.map((day) => {
            const exists = Boolean(day.matchingNode);
            if (day.isToday) {
              return (
                <button
                  key={day.dateStr}
                  id="hud-btn-today"
                  onClick={() => handleDayClick(day)}
                  data-testid="hud-day-today"
                  className="relative flex flex-col items-center justify-center px-2.5 py-1 rounded-md bg-cyan-950/40 border border-cyan-400/80 text-cyan-300 shadow-[0_0_10px_rgba(56,189,248,0.25)] transition-all cursor-pointer cosmic-focus-visible min-w-[38px]"
                  title={`Today: ${day.dateStr}${exists ? ` — ${day.title}` : ' (No log)'} - Shortcut: 1 / Alt+1`}
                  aria-label={`Today, ${day.dayLabel} ${day.dayNum}, ${exists ? 'daily log recorded' : 'no log'}`}
                >
                  <span className="text-[8px] font-mono font-black tracking-wider text-cyan-300 leading-none">TODAY</span>
                  <div className="flex items-center gap-1 mt-0.5">
                    <span className="text-xs font-mono font-black text-white">{day.dayNum}</span>
                    <span className={cn("w-1.5 h-1.5 rounded-full", exists ? "bg-emerald-400 shadow-[0_0_8px_#34d399]" : "bg-cyan-400 shadow-[0_0_8px_#38bdf8]")} />
                  </div>
                </button>
              );
            }
            return (
              <button
                key={day.dateStr}
                onClick={() => handleDayClick(day)}
                data-testid={`hud-day-${day.dayLabel.toLowerCase()}`}
                className={cn(
                  "group flex flex-col items-center justify-center px-2 py-1 rounded-md transition-all cursor-pointer cosmic-focus-visible min-w-[34px]",
                  day.isFuture
                    ? "border border-transparent hover:border-white/15 hover:bg-white/5 opacity-60 text-slate-400"
                    : "border border-transparent hover:border-white/15 hover:bg-white/5 text-slate-300 hover:text-white"
                )}
                title={`${day.dayLabel}, ${day.dateStr}${exists ? ` — ${day.title}` : ' (No log)'}`}
                aria-label={`${day.dayLabel} ${day.dayNum}, ${exists ? 'daily log recorded' : 'no log'}`}
              >
                <span className="text-[9px] font-mono text-slate-400 group-hover:text-cyan-300 font-semibold leading-none">{day.dayLabel}</span>
                <div className="flex items-center gap-1 mt-0.5">
                  <span className="text-xs font-mono font-bold leading-tight">{day.dayNum}</span>
                  <span className={cn("w-1.5 h-1.5 rounded-full", exists ? "bg-emerald-400 shadow-[0_0_6px_#34d399]" : "bg-white/10")} />
                </div>
              </button>
            );
          })}
        </div>

        <div className="max-w-40 border-l border-white/10 pl-3 text-[10px] text-slate-300" data-testid="temporal-selected-day-summary" aria-live="polite">
          <span className="text-slate-500">{selectedDate}</span>
          <span className="block truncate" title={selectedDay.title || 'No daily note'}>{selectedDay.title || 'No daily note'}</span>
        </div>

        {/* Seldon Vault Anchors Section */}
        <div className="flex flex-wrap items-center gap-1.5 pl-3 border-l border-white/10 relative max-w-full" role="group" aria-label="Seldon Vault Anchors">
          {/* User / Config Pinned Anchors */}
          {visibleAnchors.map((anchor) => (
            <button
              key={anchor.path}
              onClick={() => handleAnchorClick(anchor)}
              data-testid={`hud-anchor-${anchor.path}`}
              className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-hud-md bg-surface/80 backdrop-blur-hud border border-white/10 hover:border-cyan-500/50 hover:bg-white/5 text-slate-300 hover:text-white transition-all text-xs font-mono cursor-pointer cosmic-focus-visible max-w-[130px]"
              title={`Seldon Vault Anchor: ${anchor.label} (${anchor.path})`}
              aria-label={`Open anchored note ${anchor.label}`}
            >
              <span className="text-cyan-400 text-xs">{anchor.icon || '📌'}</span>
              <span className="truncate">{anchor.label}</span>
            </button>
          ))}

          {/* Overflow Popover Button */}
          {overflowAnchors.length > 0 && (
            <div className="relative" ref={overflowRef}>
              <button
                onClick={() => setIsOverflowOpen(prev => !prev)}
                data-testid="hud-anchors-overflow-btn"
                aria-label="Show additional Seldon Vault Anchors"
                aria-expanded={isOverflowOpen}
                className={cn(
                  "flex items-center gap-1 px-2 py-1.5 rounded-hud-md backdrop-blur-hud border transition-all text-xs font-mono cursor-pointer cosmic-focus-visible",
                  isOverflowOpen
                    ? "bg-cyan-600/20 border-cyan-500/60 text-cyan-300 shadow-[0_0_10px_rgba(56,189,248,0.2)]"
                    : "bg-white/3 border-white/10 hover:border-white/20 text-slate-400 hover:text-white"
                )}
                title="Seldon Crypt Archive: View all pinned anchors"
              >
                <span>+{overflowAnchors.length}</span>
                <ChevronDown className={cn("w-3 h-3 transition-transform", isOverflowOpen && "rotate-180")} />
              </button>

              {/* Overflow Popover Card */}
              {isOverflowOpen && (
                <div className="absolute bottom-full mb-2 right-0 z-50 min-w-[220px]">
                  <HudCard
                    variant="cyan"
                    elevation="elevated"
                    className="p-3 flex flex-col gap-2 shadow-hud-deep"
                  >
                    <div className="flex items-center justify-between pb-1.5 border-b border-white/10">
                      <div className="flex items-center gap-1.5 text-[9px] font-mono font-bold tracking-wider text-cyan-400 uppercase">
                        <span>◈</span>
                        <span>SELDON VAULT ANCHORS</span>
                      </div>
                      <span className="text-[9px] font-mono text-slate-400">
                        {anchors.length} Total
                      </span>
                    </div>

                    <div className="flex flex-col gap-1 max-h-52 overflow-y-auto pr-1">
                      {anchors.map(anchor => (
                        <div
                          key={anchor.path}
                          className="flex items-center justify-between gap-2 p-1.5 rounded-md hover:bg-white/5 border border-transparent hover:border-white/10 group transition-all"
                        >
                          <button
                            onClick={() => {
                              handleAnchorClick(anchor);
                              setIsOverflowOpen(false);
                            }}
                            className="flex items-center gap-2 text-left truncate flex-1 text-slate-300 hover:text-cyan-300 text-xs font-mono cursor-pointer"
                            title={anchor.path}
                          >
                            <span className="text-cyan-400 text-xs">{anchor.icon || '📌'}</span>
                            <span className="truncate">{anchor.label}</span>
                          </button>

                          {anchor.isCustom && (
                            <button
                              onClick={(e) => {
                                e.stopPropagation();
                                removePin(anchor.path);
                                onToast?.('Seldon Anchors', `Unpinned: ${anchor.label}`);
                              }}
                              aria-label={`Unpin ${anchor.label}`}
                              className="p-1 rounded-sm text-slate-500 hover:text-red-400 hover:bg-red-500/10 transition-colors opacity-0 group-hover:opacity-100 cursor-pointer"
                              title="Unpin note from hotbar"
                            >
                              <X className="w-3 h-3" />
                            </button>
                          )}
                        </div>
                      ))}
                    </div>
                  </HudCard>
                </div>
              )}
            </div>
          )}
        </div>
      </HudCard>
    </footer>
  );
};
