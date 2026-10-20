import React, { useState, useEffect, useRef } from 'react';
import { 
  X, 
  Terminal, 
  Play, 
  Loader2, 
  Activity, 
  Search,
  Cog,
  BarChart3,
  Flame,
  ShieldCheck,
  Plus,
  Pencil,
  Trash2,
  Clock,
  Brain,
  RefreshCw,
  Zap,
  Compass,
  FileText,
  AlertTriangle,
  Sparkles,
  CheckCircle2,
  TrendingUp,
  Calendar
} from 'lucide-react';
import { client } from '@/lib/apiClient';
import { HudCard } from './HudCard';
import { CyberButton } from './CyberButton';
import { NeonBadge } from './NeonBadge';
import { HotkeyBadge } from './HotkeyBadge';
import { CyberTerminal } from './CyberTerminal';
import { SkillParameterDeck } from './SkillParameterDeck';
import { RoutineEditorModal } from './RoutineEditorModal';
import { RoutineHistoryDeck, formatRelativeTime } from './RoutineHistoryDeck';
import { useRoutineExecution, type UseRoutineExecutionResult } from '@/hooks/useRoutineExecution';
import type { 
  RoutineStatusItem, 
  RoutineRunItem,
  GetStaffTelemetryResponse, 
  GetCognitiveTelemetryResponse 
} from '@/gen/api/v1/graph_pb';
import { describeLastResult, describeSchedule, formatRunTimestamp } from '@/lib/cockpitPresentation';

export interface CockpitModalProps {
  isOpen: boolean;
  onClose: () => void;
  preselectedRoutineName?: string | null;
  execution?: UseRoutineExecutionResult;
}

type TabMode = 'routines' | 'telemetry' | 'metacognition';
type CategoryFilter = 'all' | 'deterministic' | 'ai_skill';

export const CockpitModal: React.FC<CockpitModalProps> = ({ 
  isOpen, 
  onClose,
  preselectedRoutineName,
  execution,
}) => {
  const [activeTab, setActiveTab] = useState<TabMode>('routines');
  const [routines, setRoutines] = useState<RoutineStatusItem[]>([]);
  const [selectedRoutine, setSelectedRoutine] = useState<RoutineStatusItem | null>(null);
  const [categoryFilter, setCategoryFilter] = useState<CategoryFilter>('all');
  const [searchQuery, setSearchQuery] = useState<string>('');
  const [telemetry, setTelemetry] = useState<GetStaffTelemetryResponse | null>(null);
  const [cognitiveTelemetry, setCognitiveTelemetry] = useState<GetCognitiveTelemetryResponse | null>(null);
  const [routineRuns, setRoutineRuns] = useState<RoutineRunItem[]>([]);
  const [isLoadingRoutines, setIsLoadingRoutines] = useState<boolean>(false);
  const [isLoadingTelemetry, setIsLoadingTelemetry] = useState<boolean>(false);
  const [isLoadingCognitive, setIsLoadingCognitive] = useState<boolean>(false);
  const [isLoadingHistory, setIsLoadingHistory] = useState<boolean>(false);
  const [actionsViewMode, setActionsViewMode] = useState<'terminal' | 'history'>('terminal');
  const [isEditorOpen, setIsEditorOpen] = useState<boolean>(false);
  const [editingRoutine, setEditingRoutine] = useState<RoutineStatusItem | null>(null);

  const internalExecution = useRoutineExecution();
  const exec = execution ?? internalExecution;

  const loadRoutines = async (selectName?: string) => {
    setIsLoadingRoutines(true);
    try {
      const res = await client.getRoutinesOverview({});
      const newRoutines = res.routines || [];
      setRoutines(newRoutines);
      if (selectName) {
        const match = newRoutines.find((r) => r.name.toLowerCase() === selectName.toLowerCase());
        if (match) setSelectedRoutine(match);
      } else {
        setSelectedRoutine((prev) => {
          if (!prev) return newRoutines[0] || null;
          return newRoutines.find((r) => r.name === prev.name) || newRoutines[0] || null;
        });
      }
    } catch (err) {
      console.error('Failed to load routines overview:', err);
    } finally {
      setIsLoadingRoutines(false);
    }
  };

  const loadStaffTelemetry = async () => {
    setIsLoadingTelemetry(true);
    try {
      const res = await client.getStaffTelemetry({
        days: 7,
        rollingWeeks: 4,
        historicalMonths: 6,
      });
      setTelemetry(res);
    } catch (err) {
      console.error('Failed to load staff telemetry:', err);
    } finally {
      setIsLoadingTelemetry(false);
    }
  };

  const loadCognitiveTelemetry = async () => {
    setIsLoadingCognitive(true);
    try {
      const res = await client.getCognitiveTelemetry({});
      setCognitiveTelemetry(res);
    } catch (err) {
      console.error('Failed to load cognitive telemetry:', err);
    } finally {
      setIsLoadingCognitive(false);
    }
  };

  const loadRoutineHistory = async (routineName?: string) => {
    setIsLoadingHistory(true);
    try {
      const res = await client.getRoutineHistory({
        routineName: routineName || '',
        limit: 30,
      });
      setRoutineRuns(res.runs || []);
    } catch (err) {
      console.error('Failed to load routine execution history:', err);
    } finally {
      setIsLoadingHistory(false);
    }
  };

  const handleRefreshAll = () => {
    loadRoutines();
    loadStaffTelemetry();
    loadCognitiveTelemetry();
    loadRoutineHistory();
  };

  const handleOpenCreateRoutine = () => {
    setEditingRoutine(null);
    setIsEditorOpen(true);
  };

  const handleOpenEditRoutine = (routine: RoutineStatusItem) => {
    setEditingRoutine(routine);
    setIsEditorOpen(true);
  };

  const handleDeleteRoutine = async (routine: RoutineStatusItem) => {
    const confirmed = window.confirm(
      `Are you sure you want to delete routine "${routine.name}"? This will remove its configuration and systemd timer.`
    );
    if (!confirmed) return;

    try {
      const res = await client.deleteRoutine({ name: routine.name });
      if (!res.success) {
        alert(res.error || 'Failed to delete routine');
        return;
      }
      if (selectedRoutine?.name === routine.name) {
        setSelectedRoutine(null);
      }
      await loadRoutines();
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Error deleting routine');
    }
  };

  // Select preselected routine if provided
  useEffect(() => {
    if (preselectedRoutineName && routines.length > 0) {
      const match = routines.find(
        (r) => r.name.toLowerCase() === preselectedRoutineName.toLowerCase()
      );
      if (match) setSelectedRoutine(match);
    }
  }, [preselectedRoutineName, routines]);

  // Load routines, staff telemetry, cognitive telemetry, and history when modal opens
  useEffect(() => {
    if (!isOpen) return;

    loadRoutines();
    loadStaffTelemetry();
    loadCognitiveTelemetry();
    loadRoutineHistory();
  }, [isOpen]);

  // Track routine name during execution
  const lastRunningRoutineRef = useRef<string | null>(null);
  const prevRunningRef = useRef<boolean>(false);

  useEffect(() => {
    if (exec.isRunning && exec.runningRoutineName) {
      lastRunningRoutineRef.current = exec.runningRoutineName;
      setActionsViewMode('terminal');
    }
  }, [exec.isRunning, exec.runningRoutineName]);

  // Reactive reload of telemetry and routine history when a routine finishes execution
  useEffect(() => {
    if (prevRunningRef.current && !exec.isRunning) {
      const finishedRoutine = (lastRunningRoutineRef.current || '').toLowerCase();
      // Reload routines and routine history so cards and timeline reflect new run immediately
      loadRoutines();
      loadRoutineHistory();

      if (exec.activeExitCode === 0) {
        if (
          finishedRoutine === 'dream' ||
          finishedRoutine === 'audit_agents' ||
          finishedRoutine === 'audit:agents' ||
          finishedRoutine === 'evals' ||
          finishedRoutine.startsWith('audit')
        ) {
          loadCognitiveTelemetry();
        }
      }
    }
    prevRunningRef.current = exec.isRunning;
  }, [exec.isRunning, exec.activeExitCode]);

  // Escape key and Ctrl+Enter keyboard listener
  useEffect(() => {
    if (!isOpen) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.stopPropagation();
        onClose();
        return;
      }

      const target = e.target as HTMLElement;
      if (target?.tagName === 'INPUT' || target?.tagName === 'TEXTAREA') {
        return;
      }

      if (e.ctrlKey && e.key === 'Enter') {
        e.preventDefault();
        if (selectedRoutine && !exec.isRunning) {
          exec.executeRoutine(selectedRoutine.name, exec.verbose);
        }
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onClose, selectedRoutine, exec]);

  if (!isOpen) return null;

  // Filter routines based on category and search query
  const filteredRoutines = routines.filter((r) => {
    const matchesCategory =
      categoryFilter === 'all' ||
      (categoryFilter === 'deterministic' && r.category === 'deterministic') ||
      (categoryFilter === 'ai_skill' && r.category === 'ai_skill');

    const matchesSearch =
      searchQuery.trim() === '' ||
      r.title.toLowerCase().includes(searchQuery.toLowerCase()) ||
      r.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      r.description.toLowerCase().includes(searchQuery.toLowerCase());

    return matchesCategory && matchesSearch;
  });

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-md p-3 sm:p-6 animate-in fade-in duration-150"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      aria-labelledby="cockpit-title"
      data-testid="cockpit-modal"
    >
      <div 
        onClick={(e) => e.stopPropagation()} 
        className="w-full max-w-6xl h-[88vh] max-h-[850px] flex flex-col"
      >
        <HudCard
          variant="cyan"
          elevation="elevated"
          techCorners
          className="w-full h-full flex flex-col overflow-hidden relative shadow-hud-deep border-cyan-500/30"
        >
          {/* Cockpit Top Header */}
          <div className="flex flex-wrap items-center justify-between gap-3 p-4 sm:px-6 border-b border-white/10 bg-[#070a14]/60">
            <div className="flex items-center gap-3">
              <div className="p-2 rounded-lg bg-cyan-500/20 text-cyan-300 border border-cyan-500/40 shadow-glow-cyan">
                <Terminal className="w-5 h-5" aria-hidden="true" />
              </div>
              <div>
                <div className="flex items-center gap-2">
                  <h2 id="cockpit-title" className="text-sm sm:text-base font-bold text-white font-mono tracking-wider">
                    Routines and activity
                  </h2>
                  <NeonBadge 
                    accent={exec.isRunning ? 'amber' : 'emerald'} 
                    ping={exec.isRunning} 
                    size="sm"
                  >
                    {exec.isRunning ? 'EXEC' : 'READY'}
                  </NeonBadge>
                </div>
                <p className="text-xs text-slate-400">
                  Review what each routine does and when it runs.
                </p>
              </div>
            </div>

            {/* Navigation Tabs & Actions */}
            <div className="flex items-center gap-2">
              <div className="flex rounded-hud-md bg-surface/80 border border-white/10 p-1">
                <button
                  onClick={() => setActiveTab('routines')}
                  data-testid="cockpit-tab-routines"
                  className={`px-3 py-1 text-xs font-mono rounded-sm transition-all cursor-pointer cosmic-focus-visible ${
                    activeTab === 'routines'
                      ? 'bg-cyan-500/20 text-cyan-300 border border-cyan-500/40 shadow-glow-cyan'
                      : 'text-slate-400 hover:text-white hover:bg-white/5'
                  }`}
                >
                  <span className="flex items-center gap-1.5">
                    <Cog className="w-3.5 h-3.5" />
                    Actions Deck
                  </span>
                </button>
                <button
                  onClick={() => setActiveTab('telemetry')}
                  data-testid="cockpit-tab-telemetry"
                  className={`px-3 py-1 text-xs font-mono rounded-sm transition-all cursor-pointer cosmic-focus-visible ${
                    activeTab === 'telemetry'
                      ? 'bg-indigo-500/20 text-indigo-300 border border-indigo-500/40 shadow-glow-indigo'
                      : 'text-slate-400 hover:text-white hover:bg-white/5'
                  }`}
                >
                  <span className="flex items-center gap-1.5">
                    <BarChart3 className="w-3.5 h-3.5" />
                    Work summary
                  </span>
                </button>
                <button
                  onClick={() => setActiveTab('metacognition')}
                  data-testid="cockpit-tab-metacognition"
                  className={`px-3 py-1 text-xs font-mono rounded-sm transition-all cursor-pointer cosmic-focus-visible ${
                    activeTab === 'metacognition'
                      ? 'bg-purple-500/20 text-purple-300 border border-purple-500/40 shadow-glow-purple'
                      : 'text-slate-400 hover:text-white hover:bg-white/5'
                  }`}
                >
                  <span className="flex items-center gap-1.5">
                    <Brain className="w-3.5 h-3.5" />
                    Agent insights
                  </span>
                </button>
              </div>

              <CyberButton
                onClick={handleRefreshAll}
                variant="ghost"
                size="icon"
                aria-label="Refresh Data"
                data-testid="cockpit-refresh-btn"
                className="text-slate-400 hover:text-cyan-300 hover:bg-white/10"
                title="Refresh all telemetry & routines"
              >
                <RefreshCw className={`w-4 h-4 ${isLoadingRoutines || isLoadingTelemetry || isLoadingCognitive ? 'animate-spin text-cyan-400' : ''}`} />
              </CyberButton>

              <CyberButton
                onClick={onClose}
                variant="ghost"
                size="icon"
                aria-label="Close Cockpit"
                data-testid="cockpit-close-btn"
                className="text-slate-400 hover:text-white hover:bg-white/10"
              >
                <X className="w-4 h-4" />
              </CyberButton>
            </div>
          </div>

          {/* Tab 1: Actions & Routines Deck */}
          {activeTab === 'routines' && (
            <div className="flex-1 min-h-0 grid grid-cols-1 lg:grid-cols-12 overflow-hidden">
              {/* Left Column: Routines List */}
              <div className="lg:col-span-5 flex flex-col border-b lg:border-b-0 lg:border-r border-white/10 bg-[#070a14]/40 overflow-hidden">
                {/* Quick-Action Presets */}
                <div className="p-2.5 bg-[#0a0e1c]/70 border-b border-white/10 shrink-0">
                  <div className="text-[10px] font-mono text-slate-400 uppercase tracking-wider mb-1.5 flex items-center justify-between">
                    <span className="flex items-center gap-1.5 text-cyan-300 font-bold">
                      <Zap className="w-3 h-3 text-cyan-400" />
                      Quick Presets
                    </span>
                    <span className="text-[9px] text-slate-500 font-normal">1-Click Dispatch</span>
                  </div>
                  <div className="flex items-center gap-1.5 overflow-x-auto pb-0.5 scrollbar-thin">
                    <CyberButton
                      onClick={() => {
                        const r = routines.find((item) => item.name === 'dream');
                        if (r) setSelectedRoutine(r);
                        exec.executeRoutine('dream', exec.verbose, { stats: 'true' });
                      }}
                      disabled={exec.isRunning}
                      size="sm"
                      variant="hud"
                      accent="purple"
                      className="text-[10px] py-1 px-2 whitespace-nowrap flex items-center gap-1 shrink-0"
                      data-testid="cockpit-quick-dream"
                      title="Run cognitive dream cycle with introspective profiling"
                    >
                      <Sparkles className="w-3 h-3 text-purple-300" />
                      <span>Dream</span>
                    </CyberButton>

                    <CyberButton
                      onClick={() => {
                        const r = routines.find((item) => item.name === 'audit_agents');
                        if (r) setSelectedRoutine(r);
                        exec.executeRoutine('audit_agents', exec.verbose);
                      }}
                      disabled={exec.isRunning}
                      size="sm"
                      variant="hud"
                      accent="rose"
                      className="text-[10px] py-1 px-2 whitespace-nowrap flex items-center gap-1 shrink-0"
                      data-testid="cockpit-quick-audit_agents"
                      title="Audit agent transcripts for missed opportunities"
                    >
                      <Brain className="w-3 h-3 text-rose-300" />
                      <span>Audit Agents</span>
                    </CyberButton>

                    <CyberButton
                      onClick={() => {
                        const r = routines.find((item) => item.name === 'doctor');
                        if (r) setSelectedRoutine(r);
                        exec.executeRoutine('doctor', exec.verbose);
                      }}
                      disabled={exec.isRunning}
                      size="sm"
                      variant="hud"
                      accent="emerald"
                      className="text-[10px] py-1 px-2 whitespace-nowrap flex items-center gap-1 shrink-0"
                      data-testid="cockpit-quick-doctor"
                      title="Run vault diagnostics and daemon health checks"
                    >
                      <ShieldCheck className="w-3 h-3 text-emerald-300" />
                      <span>Doctor</span>
                    </CyberButton>

                    <CyberButton
                      onClick={() => {
                        const r = routines.find((item) => item.name === 'tidy');
                        if (r) setSelectedRoutine(r);
                        exec.executeRoutine('tidy', exec.verbose);
                      }}
                      disabled={exec.isRunning}
                      size="sm"
                      variant="hud"
                      accent="cyan"
                      className="text-[10px] py-1 px-2 whitespace-nowrap flex items-center gap-1 shrink-0"
                      data-testid="cockpit-quick-tidy"
                      title="Tidy notes and reindex SQLite FTS5 database"
                    >
                      <FileText className="w-3 h-3 text-cyan-300" />
                      <span>Tidy</span>
                    </CyberButton>

                    <CyberButton
                      onClick={() => {
                        const r = routines.find((item) => item.name === 'workspace');
                        if (r) setSelectedRoutine(r);
                        exec.executeRoutine('workspace', exec.verbose);
                      }}
                      disabled={exec.isRunning}
                      size="sm"
                      variant="hud"
                      accent="amber"
                      className="text-[10px] py-1 px-2 whitespace-nowrap flex items-center gap-1 shrink-0"
                      data-testid="cockpit-quick-workspace"
                      title="Scan workspace Git repositories and sync metadata"
                    >
                      <Compass className="w-3 h-3 text-amber-300" />
                      <span>Workspace</span>
                    </CyberButton>
                  </div>
                </div>

                {/* Search and Filters Bar */}
                <div className="p-3 border-b border-white/10 space-y-2">
                  <div className="relative">
                    <Search className="w-3.5 h-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-slate-500" />
                    <input
                      type="text"
                      value={searchQuery}
                      onChange={(e) => setSearchQuery(e.target.value)}
                      placeholder="Filter routines or skills..."
                      className="w-full pl-9 pr-3 py-1.5 text-xs bg-void/80 border border-white/10 rounded-hud-md text-white placeholder-slate-500 focus:outline-hidden focus:border-cyan-500/60 transition-colors font-mono"
                    />
                  </div>

                  <div className="flex items-center justify-between gap-1.5 pt-1 text-[11px] font-mono">
                    <div className="flex items-center gap-1.5">
                      <button
                        onClick={() => setCategoryFilter('all')}
                        className={`px-2 py-1 rounded-sm transition-colors cursor-pointer ${
                          categoryFilter === 'all'
                            ? 'bg-white/10 text-white font-bold'
                            : 'text-slate-400 hover:text-slate-200'
                        }`}
                      >
                        All ({routines.length})
                      </button>
                      <button
                        onClick={() => setCategoryFilter('deterministic')}
                        className={`px-2 py-1 rounded-sm transition-colors cursor-pointer ${
                          categoryFilter === 'deterministic'
                            ? 'bg-cyan-500/20 text-cyan-300 font-bold'
                            : 'text-slate-400 hover:text-cyan-400'
                        }`}
                      >
                        Routines
                      </button>
                      <button
                        onClick={() => setCategoryFilter('ai_skill')}
                        className={`px-2 py-1 rounded-sm transition-colors cursor-pointer ${
                          categoryFilter === 'ai_skill'
                            ? 'bg-purple-500/20 text-purple-300 font-bold'
                            : 'text-slate-400 hover:text-purple-400'
                        }`}
                      >
                        AI Skills
                      </button>
                    </div>

                    <CyberButton
                      onClick={handleOpenCreateRoutine}
                      variant="hud"
                      accent="cyan"
                      size="sm"
                      className="text-[10px] py-1 px-2.5 flex items-center gap-1 shrink-0"
                      data-testid="cockpit-add-routine-btn"
                      title="Add new scheduled routine"
                    >
                      <Plus className="w-3 h-3" />
                      <span>New Routine</span>
                    </CyberButton>
                  </div>
                </div>

                {/* Scrollable Routine Cards */}
                <div className="flex-1 overflow-y-auto p-2 space-y-1.5">
                  {isLoadingRoutines ? (
                    <div className="flex flex-col items-center justify-center p-8 text-slate-500 text-xs gap-2">
                      <Loader2 className="w-5 h-5 animate-spin text-cyan-400" />
                      <span>Loading routines catalog...</span>
                    </div>
                  ) : filteredRoutines.length === 0 ? (
                    <div className="p-6 text-center text-xs text-slate-500">
                      No routines match your filter.
                    </div>
                  ) : (
                    filteredRoutines.map((routine) => {
                      const isSelected = selectedRoutine?.name === routine.name;
                      const isItemRunning = exec.isRunning && exec.runningRoutineName === routine.name;

                      return (
                        <div
                          key={routine.name}
                          onClick={() => setSelectedRoutine(routine)}
                          data-testid={`cockpit-routine-${routine.name}`}
                          className={`p-3 rounded-hud-md border transition-all cursor-pointer ${
                            isSelected
                              ? 'bg-cyan-950/40 border-cyan-500/50 shadow-glow-cyan'
                              : 'bg-surface/60 border-white/5 hover:border-white/20 hover:bg-surface-elevated/70'
                          }`}
                        >
                          <div className="flex items-start justify-between gap-2">
                            <div className="space-y-1 min-w-0 flex-1">
                              <div className="flex items-center gap-2 flex-wrap">
                                <span className="font-mono text-xs font-semibold text-white truncate">
                                  {routine.title}
                                </span>
                                <NeonBadge
                                  accent={routine.category === 'deterministic' ? 'cyan' : 'purple'}
                                  size="sm"
                                >
                                  {routine.category === 'deterministic' ? 'BUILT-IN' : 'AI SKILL'}
                                </NeonBadge>
                                {routine.schedule && (
                                  <span
                                    className="flex items-center gap-1 text-[10px] text-cyan-400 font-mono bg-cyan-950/40 px-1.5 py-0.5 rounded-sm border border-cyan-500/20"
                                    title={describeSchedule(routine.schedule) || undefined}
                                    data-testid={`cockpit-routine-schedule-${routine.name}`}
                                  >
                                    <Clock className="w-2.5 h-2.5" />
                                    {describeSchedule(routine.schedule)}
                                  </span>
                                )}
                                {Number(routine.lastRunTimestamp) > 0 && (
                                  <span
                                    className="flex items-center gap-1 text-[10px] text-slate-300 font-mono bg-white/5 px-1.5 py-0.5 rounded-sm border border-white/10"
                                    title={`Last run: ${new Date(Number(routine.lastRunTimestamp) * 1000).toLocaleString()}`}
                                    data-testid={`cockpit-routine-last-run-${routine.name}`}
                                  >
                                    <Clock className="w-2.5 h-2.5 text-slate-500" />
                                    Last run: {formatRelativeTime(routine.lastRunTimestamp)}
                                  </span>
                                )}
                                {routine.lastResult && routine.lastResult !== 'unknown' && routine.lastResult !== 'ready' && routine.lastResult !== 'not installed' && (
                                  <NeonBadge
                                    accent={
                                      routine.lastResult === 'success'
                                        ? 'emerald'
                                        : routine.lastResult === 'running'
                                        ? 'amber'
                                        : 'rose'
                                    }
                                    size="sm"
                                    data-testid={`cockpit-routine-last-result-${routine.name}`}
                                  >
                                    {describeLastResult(routine.lastResult)}
                                  </NeonBadge>
                                )}
                                {Number(routine.nextTriggerTimestamp) > 0 && (
                                  <span
                                    className="flex items-center gap-1 text-[10px] text-indigo-300 font-mono bg-indigo-950/40 px-1.5 py-0.5 rounded-sm border border-indigo-500/20"
                                    title={`Next run: ${formatRunTimestamp(routine.nextTriggerTimestamp)}`}
                                    data-testid={`cockpit-routine-next-trigger-${routine.name}`}
                                  >
                                    Next run: {formatRunTimestamp(routine.nextTriggerTimestamp)}
                                  </span>
                                )}
                              </div>
                              <p className="text-[11px] text-slate-400 line-clamp-2 leading-relaxed">
                                {routine.description}
                              </p>
                            </div>

                            <div className="flex items-center gap-1 shrink-0">
                              {routine.category === 'deterministic' && (
                                <>
                                  <CyberButton
                                    onClick={(e) => {
                                      e.stopPropagation();
                                      handleOpenEditRoutine(routine);
                                    }}
                                    size="sm"
                                    variant="ghost"
                                    data-testid={`cockpit-edit-btn-${routine.name}`}
                                    className="p-1.5 text-slate-400 hover:text-cyan-300 hover:bg-cyan-500/10"
                                    title={`Edit routine ${routine.name}`}
                                  >
                                    <Pencil className="w-3.5 h-3.5" />
                                  </CyberButton>
                                  <CyberButton
                                    onClick={(e) => {
                                      e.stopPropagation();
                                      handleDeleteRoutine(routine);
                                    }}
                                    size="sm"
                                    variant="ghost"
                                    data-testid={`cockpit-delete-btn-${routine.name}`}
                                    className="p-1.5 text-slate-400 hover:text-rose-400 hover:bg-rose-500/10"
                                    title={`Delete routine ${routine.name}`}
                                  >
                                    <Trash2 className="w-3.5 h-3.5" />
                                  </CyberButton>
                                </>
                              )}

                              <CyberButton
                                onClick={(e) => {
                                  e.stopPropagation();
                                  setSelectedRoutine(routine);
                                  exec.executeRoutine(routine.name, exec.verbose);
                                }}
                                disabled={exec.isRunning}
                                size="sm"
                                variant="hud"
                                accent={routine.category === 'deterministic' ? 'cyan' : 'purple'}
                                data-testid={`cockpit-run-btn-${routine.name}`}
                                className="p-1.5"
                                title={`Execute ${routine.name}`}
                              >
                                {isItemRunning ? (
                                  <Loader2 className="w-3.5 h-3.5 animate-spin text-cyan-300" />
                                ) : (
                                  <Play className="w-3.5 h-3.5 text-cyan-300 fill-cyan-300/30" />
                                )}
                              </CyberButton>
                            </div>
                          </div>
                        </div>
                      );
                    })
                  )}
                </div>
              </div>

              {/* Right Column: Live Streaming Terminal View, Execution History & AI Parameter Deck */}
              <div className="lg:col-span-7 flex flex-col bg-void/90 overflow-hidden divide-y divide-white/10">
                {/* Right Column Sub-Header / Deck Switcher */}
                <div className="flex items-center justify-between px-3 py-1.5 bg-surface/90 border-b border-white/10 shrink-0">
                  <div className="flex items-center gap-1 bg-black/40 p-0.5 rounded-sm border border-white/5 font-mono text-[11px]">
                    <button
                      onClick={() => setActionsViewMode('terminal')}
                      data-testid="cockpit-view-terminal-btn"
                      className={`px-2.5 py-1 rounded-sm transition-all cursor-pointer flex items-center gap-1.5 ${
                        actionsViewMode === 'terminal'
                          ? 'bg-cyan-500/20 text-cyan-300 font-bold border border-cyan-500/30 shadow-glow-cyan'
                          : 'text-slate-400 hover:text-white hover:bg-white/5'
                      }`}
                    >
                      <Terminal className="w-3 h-3" />
                      <span>Live Output</span>
                      {exec.isRunning && (
                        <span className="w-1.5 h-1.5 rounded-full bg-cyan-400 animate-ping" />
                      )}
                    </button>

                    <button
                      onClick={() => {
                        setActionsViewMode('history');
                        loadRoutineHistory();
                      }}
                      data-testid="cockpit-view-history-btn"
                      className={`px-2.5 py-1 rounded-sm transition-all cursor-pointer flex items-center gap-1.5 ${
                        actionsViewMode === 'history'
                          ? 'bg-indigo-500/20 text-indigo-300 font-bold border border-indigo-500/30 shadow-glow-indigo'
                          : 'text-slate-400 hover:text-white hover:bg-white/5'
                      }`}
                    >
                      <Clock className="w-3 h-3" />
                      <span>Recent Runs</span>
                      {routineRuns.length > 0 && (
                        <span className="text-[9px] bg-white/10 px-1.5 py-0.2 rounded-sm text-slate-300 font-normal">
                          {routineRuns.length}
                        </span>
                      )}
                    </button>
                  </div>

                  {actionsViewMode === 'terminal' && selectedRoutine && (
                    <span className="text-[11px] text-slate-300 truncate max-w-[240px]">
                      Routine details
                    </span>
                  )}
                </div>

                {actionsViewMode === 'history' ? (
                  <div className="flex-1 min-h-0">
                    <RoutineHistoryDeck
                      runs={routineRuns}
                      isLoading={isLoadingHistory}
                      onRefresh={() => loadRoutineHistory()}
                      onSelectRoutine={(name) => {
                        const match = routines.find((r) => r.name.toLowerCase() === name.toLowerCase());
                        if (match) setSelectedRoutine(match);
                        setActionsViewMode('terminal');
                      }}
                      testIdPrefix="cockpit-history"
                    />
                  </div>
                ) : (
                  <>
                    {selectedRoutine && (
                      <section data-testid="cockpit-routine-preview" aria-label="Routine details" className="p-3 bg-[#080b16]/90 border-b border-white/10 shrink-0 space-y-2">
                        <div className="font-semibold text-sm text-white">{selectedRoutine.title}</div>
                        <p className="text-xs leading-relaxed text-slate-300">{selectedRoutine.description || 'No description is available for this routine.'}</p>
                        <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-slate-400">
                          {describeSchedule(selectedRoutine.schedule) && <span>Schedule: {describeSchedule(selectedRoutine.schedule)}</span>}
                          {formatRunTimestamp(selectedRoutine.nextTriggerTimestamp) && <span>Next run: {formatRunTimestamp(selectedRoutine.nextTriggerTimestamp)}</span>}
                          {Number(selectedRoutine.lastRunTimestamp) > 0 && <span>Last run: {formatRelativeTime(selectedRoutine.lastRunTimestamp)}</span>}
                          {describeLastResult(selectedRoutine.lastResult) && <span>Last result: {describeLastResult(selectedRoutine.lastResult)}</span>}
                        </div>
                      </section>
                    )}
                    {selectedRoutine?.category === 'ai_skill' && (
                      <div className="p-3 bg-[#080b16]/90 border-b border-purple-500/20 max-h-[320px] overflow-y-auto shrink-0">
                        <SkillParameterDeck
                          skillName={selectedRoutine.name}
                          skillTitle={selectedRoutine.title}
                          isRunning={exec.isRunning}
                          onDispatch={(params) => {
                            exec.executeRoutine(selectedRoutine.name, exec.verbose, params);
                          }}
                          testIdPrefix="cockpit-skill"
                        />
                      </div>
                    )}
                    <div className="flex-1 min-h-0">
                      <CyberTerminal
                        logs={exec.logs}
                        isRunning={exec.isRunning}
                        runningRoutineName={exec.runningRoutineName}
                        targetRoutineTitle={selectedRoutine ? `${selectedRoutine.title} (${selectedRoutine.name})` : undefined}
                        verbose={exec.verbose}
                        onToggleVerbose={() => exec.setVerbose(!exec.verbose)}
                        onAbort={exec.abortRoutine}
                        onClear={exec.clearLogs}
                        onExecute={() => {
                          if (selectedRoutine) {
                            exec.executeRoutine(selectedRoutine.name, exec.verbose);
                          }
                        }}
                        executeDisabled={!selectedRoutine}
                        testIdPrefix="cockpit"
                      />
                    </div>
                  </>
                )}
              </div>
            </div>
          )}

          {/* Tab 2: Staff Telemetry Dashboard */}
          {activeTab === 'telemetry' && (
            <div className="flex-1 min-h-0 overflow-y-auto p-4 sm:p-6 space-y-6 bg-[#070a14]/50">
              {isLoadingTelemetry ? (
                <div className="flex flex-col items-center justify-center p-12 text-slate-500 text-xs gap-2">
                  <Loader2 className="w-6 h-6 animate-spin text-indigo-400" />
                  <span>Loading work summary...</span>
                </div>
              ) : telemetry ? (
                <>
                  {/* Executive Header Card with Staff Leverage Rate */}
                  <HudCard variant="default" className="p-4 sm:p-5 relative overflow-hidden border-indigo-500/30">
                    <div className="flex flex-wrap items-center justify-between gap-4">
                      <div className="space-y-1">
                        <div className="flex items-center gap-2">
                          <span className="text-[10px] font-mono uppercase tracking-wider text-indigo-300 font-bold flex items-center gap-1.5">
                            <TrendingUp className="w-3.5 h-3.5 text-indigo-400" />
                            Technical Leadership Leverage Index
                          </span>
                          <NeonBadge
                            accent={
                              (telemetry.leverageRate ?? 0) >= 0.50
                                ? 'emerald'
                                : (telemetry.leverageRate ?? 0) >= 0.35
                                ? 'amber'
                                : 'rose'
                            }
                            size="sm"
                          >
                            {(telemetry.leverageRate ?? 0) >= 0.50
                              ? 'HIGH LEVERAGE (STRATEGIC)'
                              : (telemetry.leverageRate ?? 0) >= 0.35
                              ? 'BALANCED FOCUS'
                              : 'HIGH REACTIVITY / DRIFT'}
                          </NeonBadge>
                        </div>
                        <div className="flex items-baseline gap-2">
                          <span className="text-3xl font-extrabold font-mono text-white tracking-tight">
                            {((telemetry.leverageRate ?? 0) * 100).toFixed(1)}%
                          </span>
                          <span className="text-xs text-slate-400 font-mono">
                            Leverage Rate (Target: &ge; 50.0%)
                          </span>
                        </div>
                        <p className="text-[11px] text-slate-400 leading-relaxed font-mono">
                          Computed as <span className="text-indigo-300 font-semibold">([A] Strategic + [C] Glue) / Total</span>. Measures engineering force multiplication vs localized execution.
                        </p>
                      </div>

                      <div className="flex items-center gap-3 bg-void/80 px-3 py-2 rounded-hud-md border border-white/10 text-xs font-mono">
                        <div>
                          <div className="text-[10px] text-slate-500 uppercase">7-Day Total</div>
                          <div className="text-sm font-bold text-white">{telemetry.totalHours.toFixed(1)}h</div>
                        </div>
                        <div className="w-px h-6 bg-white/10" />
                        <div>
                          <div className="text-[10px] text-slate-500 uppercase">Leverage Hours</div>
                          <div className="text-sm font-bold text-cyan-300">
                            {(telemetry.strategicAHours + telemetry.glueWorkCHours).toFixed(1)}h
                          </div>
                        </div>
                      </div>
                    </div>
                  </HudCard>

                  {/* Archetype Big Numbers (5 Columns) */}
                  <div className="grid grid-cols-2 sm:grid-cols-5 gap-3 sm:gap-4">
                    <HudCard variant="indigo" className="p-4 relative">
                      <div className="text-[10px] font-mono text-indigo-300 uppercase">
                        [A] Strategic Impact
                      </div>
                      <div className="text-2xl font-bold font-mono text-white mt-1">
                        {telemetry.strategicAHours.toFixed(1)}h
                      </div>
                      <div className="text-[11px] text-slate-400 mt-1">
                        Target: 35-45%
                      </div>
                    </HudCard>

                    <HudCard variant="cyan" className="p-4 relative">
                      <div className="text-[10px] font-mono text-cyan-300 uppercase">
                        [B] Local Impact
                      </div>
                      <div className="text-2xl font-bold font-mono text-white mt-1">
                        {telemetry.localImpactBHours.toFixed(1)}h
                      </div>
                      <div className="text-[11px] text-slate-400 mt-1">
                        Target: 25-35%
                      </div>
                    </HudCard>

                    <HudCard variant="purple" className="p-4 relative">
                      <div className="text-[10px] font-mono text-purple-300 uppercase">
                        [C] Glue Work
                      </div>
                      <div className="text-2xl font-bold font-mono text-white mt-1">
                        {telemetry.glueWorkCHours.toFixed(1)}h
                      </div>
                      <div className="text-[11px] text-slate-400 mt-1">
                        Target: 15-25%
                      </div>
                    </HudCard>

                    <HudCard variant="default" className="p-4 relative border-slate-700/50">
                      <div className="text-[10px] font-mono text-slate-300 uppercase">
                        [D] Low Impact
                      </div>
                      <div className="text-2xl font-bold font-mono text-white mt-1">
                        {(telemetry.lowImpactDHours ?? 0).toFixed(1)}h
                      </div>
                      <div className="text-[11px] text-slate-400 mt-1">
                        Target: &lt; 5%
                      </div>
                    </HudCard>

                    <HudCard variant="amber" className="p-4 relative">
                      <div className="text-[10px] font-mono text-amber-300 uppercase">
                        [E] Overhead
                      </div>
                      <div className="text-2xl font-bold font-mono text-white mt-1">
                        {telemetry.overheadEHours.toFixed(1)}h
                      </div>
                      <div className="text-[11px] text-slate-400 mt-1">
                        Target: &lt; 15%
                      </div>
                    </HudCard>
                  </div>

                  {/* Summary & Top Initiatives */}
                  <div className="grid grid-cols-1 lg:grid-cols-12 gap-4">
                    <div className="lg:col-span-7 space-y-4">
                      <HudCard variant="default" className="p-5">
                        <h3 className="text-xs font-mono font-bold text-white uppercase tracking-wider mb-3 flex items-center gap-2">
                          <Activity className="w-4 h-4 text-cyan-400" />
                          7-Day Time Distribution Balance
                        </h3>
                        <div className="space-y-3">
                          <div>
                            <div className="flex justify-between text-xs font-mono text-slate-300 mb-1">
                              <span>Total Tracked Hours:</span>
                              <span className="font-bold text-white">
                                {telemetry.totalHours.toFixed(1)}h
                              </span>
                            </div>
                            <div className="w-full bg-void rounded-full h-3 flex overflow-hidden border border-white/10">
                              {telemetry.totalHours > 0 && (
                                <>
                                  <div 
                                    style={{ width: `${(telemetry.strategicAHours / telemetry.totalHours) * 100}%` }}
                                    className="bg-indigo-500 transition-all" 
                                    title={`Strategic [A]: ${telemetry.strategicAHours.toFixed(1)}h (${((telemetry.strategicAHours / telemetry.totalHours) * 100).toFixed(1)}%)`}
                                  />
                                  <div 
                                    style={{ width: `${(telemetry.localImpactBHours / telemetry.totalHours) * 100}%` }}
                                    className="bg-cyan-500 transition-all" 
                                    title={`Local Impact [B]: ${telemetry.localImpactBHours.toFixed(1)}h (${((telemetry.localImpactBHours / telemetry.totalHours) * 100).toFixed(1)}%)`}
                                  />
                                  <div 
                                    style={{ width: `${(telemetry.glueWorkCHours / telemetry.totalHours) * 100}%` }}
                                    className="bg-purple-500 transition-all" 
                                    title={`Glue Work [C]: ${telemetry.glueWorkCHours.toFixed(1)}h (${((telemetry.glueWorkCHours / telemetry.totalHours) * 100).toFixed(1)}%)`}
                                  />
                                  <div 
                                    style={{ width: `${((telemetry.lowImpactDHours ?? 0) / telemetry.totalHours) * 100}%` }}
                                    className="bg-slate-500 transition-all" 
                                    title={`Low Impact [D]: ${(telemetry.lowImpactDHours ?? 0).toFixed(1)}h (${(((telemetry.lowImpactDHours ?? 0) / telemetry.totalHours) * 100).toFixed(1)}%)`}
                                  />
                                  <div 
                                    style={{ width: `${(telemetry.overheadEHours / telemetry.totalHours) * 100}%` }}
                                    className="bg-amber-500 transition-all" 
                                    title={`Overhead [E]: ${telemetry.overheadEHours.toFixed(1)}h (${((telemetry.overheadEHours / telemetry.totalHours) * 100).toFixed(1)}%)`}
                                  />
                                </>
                              )}
                            </div>
                            <div className="flex items-center justify-between text-[10px] font-mono text-slate-400 mt-2 flex-wrap gap-2">
                              <span className="flex items-center gap-1">
                                <span className="w-2 h-2 rounded-full bg-indigo-500" /> [A] Strat: {telemetry.totalHours > 0 ? ((telemetry.strategicAHours / telemetry.totalHours) * 100).toFixed(0) : 0}%
                              </span>
                              <span className="flex items-center gap-1">
                                <span className="w-2 h-2 rounded-full bg-cyan-500" /> [B] Local: {telemetry.totalHours > 0 ? ((telemetry.localImpactBHours / telemetry.totalHours) * 100).toFixed(0) : 0}%
                              </span>
                              <span className="flex items-center gap-1">
                                <span className="w-2 h-2 rounded-full bg-purple-500" /> [C] Glue: {telemetry.totalHours > 0 ? ((telemetry.glueWorkCHours / telemetry.totalHours) * 100).toFixed(0) : 0}%
                              </span>
                              <span className="flex items-center gap-1">
                                <span className="w-2 h-2 rounded-full bg-slate-500" /> [D] Low: {telemetry.totalHours > 0 ? (((telemetry.lowImpactDHours ?? 0) / telemetry.totalHours) * 100).toFixed(0) : 0}%
                              </span>
                              <span className="flex items-center gap-1">
                                <span className="w-2 h-2 rounded-full bg-amber-500" /> [E] Ovh: {telemetry.totalHours > 0 ? ((telemetry.overheadEHours / telemetry.totalHours) * 100).toFixed(0) : 0}%
                              </span>
                            </div>
                          </div>
                        </div>
                      </HudCard>
                    </div>

                    <div className="lg:col-span-5 space-y-4">
                      <HudCard variant="default" className="p-5">
                        <h3 className="text-xs font-mono font-bold text-white uppercase tracking-wider mb-3 flex items-center gap-2">
                          <Flame className="w-4 h-4 text-amber-400" />
                          Top Strategic Initiatives
                        </h3>
                        <ul className="space-y-2">
                          {telemetry.topInitiatives?.length ? (
                            telemetry.topInitiatives.map((init, idx) => (
                              <li 
                                key={idx} 
                                className="flex items-center gap-2 text-xs font-mono text-slate-300 bg-surface/60 p-2 rounded-sm border border-white/5"
                              >
                                <span className="text-cyan-400 font-bold">#{idx + 1}</span>
                                <span className="truncate">{init}</span>
                              </li>
                            ))
                          ) : (
                            <li className="text-xs text-slate-500 italic">
                              No strategic initiatives configured in .daneel.yaml.
                            </li>
                          )}
                        </ul>
                      </HudCard>
                    </div>
                  </div>

                  {/* Rolling 4-Weeks Comparative Window Matrix */}
                  {telemetry.rollingWeeks && telemetry.rollingWeeks.length > 0 && (
                    <HudCard variant="default" className="p-5 space-y-3">
                      <div className="flex items-center justify-between flex-wrap gap-2">
                        <h3 className="text-xs font-mono font-bold text-white uppercase tracking-wider flex items-center gap-2">
                          <Calendar className="w-4 h-4 text-cyan-400" />
                          Rolling Weekly Comparative Matrix
                        </h3>
                        <span className="text-[11px] font-mono text-slate-400">
                          {telemetry.rollingWeeks.length} Rolling ISO Weeks
                        </span>
                      </div>

                      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-3">
                        {telemetry.rollingWeeks.map((week) => {
                          const wTotal = week.totalHours || 1;
                          const pctA = Math.round((week.strategicAHours / wTotal) * 100);
                          const pctB = Math.round((week.localImpactBHours / wTotal) * 100);
                          const pctC = Math.round((week.glueWorkCHours / wTotal) * 100);
                          const pctD = Math.round((week.lowImpactDHours / wTotal) * 100);
                          const pctE = Math.round((week.overheadEHours / wTotal) * 100);
                          const levRate = Math.round((week.leverageRate || 0) * 100);

                          return (
                            <div
                              key={week.periodKey}
                              data-testid={`cockpit-weekly-${week.periodKey}`}
                              className="p-3 bg-surface/70 rounded-hud-md border border-white/10 space-y-2.5 flex flex-col justify-between"
                            >
                              <div>
                                <div className="flex items-start justify-between gap-1">
                                  <div className="min-w-0">
                                    <span className="font-mono text-xs font-bold text-white truncate block">
                                      {week.label}
                                    </span>
                                    <span className="text-[10px] font-mono text-slate-400">
                                      {week.periodKey} &bull; {week.daysCount} active days
                                    </span>
                                  </div>
                                  <span className="text-xs font-mono font-bold text-white shrink-0">
                                    {week.totalHours.toFixed(1)}h
                                  </span>
                                </div>

                                {/* Stacked Progress Bar */}
                                <div className="w-full bg-void rounded-full h-2 flex overflow-hidden border border-white/10 mt-2">
                                  {week.totalHours > 0 && (
                                    <>
                                      <div style={{ width: `${pctA}%` }} className="bg-indigo-500" title={`A: ${pctA}%`} />
                                      <div style={{ width: `${pctB}%` }} className="bg-cyan-500" title={`B: ${pctB}%`} />
                                      <div style={{ width: `${pctC}%` }} className="bg-purple-500" title={`C: ${pctC}%`} />
                                      <div style={{ width: `${pctD}%` }} className="bg-slate-500" title={`D: ${pctD}%`} />
                                      <div style={{ width: `${pctE}%` }} className="bg-amber-500" title={`E: ${pctE}%`} />
                                    </>
                                  )}
                                </div>

                                <div className="flex items-center justify-between text-[9px] font-mono text-slate-400 mt-1">
                                  <span>[A] {pctA}%</span>
                                  <span>[B] {pctB}%</span>
                                  <span>[C] {pctC}%</span>
                                  <span>[E] {pctE}%</span>
                                </div>
                              </div>

                              <div className="pt-2 border-t border-white/5 space-y-1">
                                <div className="flex items-center justify-between text-[11px] font-mono">
                                  <span className="text-slate-400">Leverage:</span>
                                  <span
                                    className={`font-bold ${
                                      levRate >= 50
                                        ? 'text-emerald-400'
                                        : levRate >= 35
                                        ? 'text-amber-400'
                                        : 'text-rose-400'
                                    }`}
                                  >
                                    {levRate}%
                                  </span>
                                </div>
                                <div
                                  className="text-[10px] font-mono text-slate-300 truncate"
                                  title={week.diagnosis}
                                >
                                  {week.diagnosis}
                                </div>
                              </div>
                            </div>
                          );
                        })}
                      </div>
                    </HudCard>
                  )}

                  {/* Historical Monthly Trend */}
                  {telemetry.monthlyHistory && telemetry.monthlyHistory.length > 0 && (
                    <HudCard variant="default" className="p-5 space-y-3">
                      <div className="flex items-center justify-between flex-wrap gap-2">
                        <h3 className="text-xs font-mono font-bold text-white uppercase tracking-wider flex items-center gap-2">
                          <BarChart3 className="w-4 h-4 text-purple-400" />
                          Historical Monthly Telemetry
                        </h3>
                        <span className="text-[11px] font-mono text-slate-400">
                          {telemetry.monthlyHistory.length} Recorded Months
                        </span>
                      </div>

                      <div className="overflow-x-auto">
                        <table className="w-full text-left border-collapse font-mono text-xs">
                          <thead>
                            <tr className="border-b border-white/10 text-[10px] text-slate-400 uppercase">
                              <th className="py-2 px-3">Month</th>
                              <th className="py-2 px-3">Days</th>
                              <th className="py-2 px-3 text-indigo-300">[A] Strat</th>
                              <th className="py-2 px-3 text-cyan-300">[B] Local</th>
                              <th className="py-2 px-3 text-purple-300">[C] Glue</th>
                              <th className="py-2 px-3 text-slate-400">[D] Low</th>
                              <th className="py-2 px-3 text-amber-300">[E] Ovh</th>
                              <th className="py-2 px-3 text-white">Total</th>
                              <th className="py-2 px-3">Leverage</th>
                              <th className="py-2 px-3">Diagnosis</th>
                            </tr>
                          </thead>
                          <tbody className="divide-y divide-white/5">
                            {telemetry.monthlyHistory.map((m) => (
                              <tr key={m.periodKey} className="hover:bg-white/5 transition-colors">
                                <td className="py-2 px-3 font-semibold text-white">{m.label}</td>
                                <td className="py-2 px-3 text-slate-400">{m.daysCount}d</td>
                                <td className="py-2 px-3 text-indigo-300">{m.strategicAHours.toFixed(1)}h</td>
                                <td className="py-2 px-3 text-cyan-300">{m.localImpactBHours.toFixed(1)}h</td>
                                <td className="py-2 px-3 text-purple-300">{m.glueWorkCHours.toFixed(1)}h</td>
                                <td className="py-2 px-3 text-slate-400">{m.lowImpactDHours.toFixed(1)}h</td>
                                <td className="py-2 px-3 text-amber-300">{m.overheadEHours.toFixed(1)}h</td>
                                <td className="py-2 px-3 font-bold text-white">{m.totalHours.toFixed(1)}h</td>
                                <td className="py-2 px-3">
                                  <span
                                    className={`font-bold ${
                                      (m.leverageRate || 0) >= 0.50
                                        ? 'text-emerald-400'
                                        : (m.leverageRate || 0) >= 0.35
                                        ? 'text-amber-400'
                                        : 'text-rose-400'
                                    }`}
                                  >
                                    {((m.leverageRate || 0) * 100).toFixed(1)}%
                                  </span>
                                </td>
                                <td className="py-2 px-3 text-slate-300 truncate max-w-[200px]" title={m.diagnosis}>
                                  {m.diagnosis}
                                </td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    </HudCard>
                  )}
                </>
              ) : (
                <div className="p-8 text-center text-xs text-slate-500">
                  Telemetry data unavailable.
                </div>
              )}
            </div>
          )}

          {/* Tab 3: Agent Evals & Metacognitive Telemetry */}
          {activeTab === 'metacognition' && (
            <div className="flex-1 min-h-0 overflow-y-auto p-4 sm:p-6 space-y-6 bg-[#070a14]/50">
              {isLoadingCognitive ? (
                <div className="flex flex-col items-center justify-center p-12 text-slate-500 text-xs gap-2">
                  <Loader2 className="w-6 h-6 animate-spin text-purple-400" />
                  <span>Loading cognitive dream metrics & agent evals...</span>
                </div>
              ) : cognitiveTelemetry ? (
                <>
                  {/* Top Metric Cards */}
                  <div className="grid grid-cols-2 sm:grid-cols-5 gap-3 sm:gap-4">
                    <HudCard variant="purple" className="p-4 relative">
                      <div className="text-[10px] font-mono text-purple-300 uppercase">
                        Dream Duration
                      </div>
                      <div className="text-2xl font-bold font-mono text-white mt-1">
                        {(Number(cognitiveTelemetry.lastCycleDurationMs) / 1000).toFixed(2)}s
                      </div>
                      <div className="text-[11px] text-slate-400 mt-1">
                        Introspective Cycle
                      </div>
                    </HudCard>

                    <HudCard 
                      variant={cognitiveTelemetry.bottlenecks.length > 0 ? "amber" : "emerald"} 
                      className="p-4 relative"
                    >
                      <div className={`text-[10px] font-mono uppercase ${
                        cognitiveTelemetry.bottlenecks.length > 0 ? 'text-amber-300' : 'text-emerald-300'
                      }`}>
                        Bottlenecks
                      </div>
                      <div className="text-2xl font-bold font-mono text-white mt-1">
                        {cognitiveTelemetry.bottlenecks.length}
                      </div>
                      <div className="text-[11px] text-slate-400 mt-1">
                        Phase Latencies &gt; 35%
                      </div>
                    </HudCard>

                    <HudCard 
                      variant={cognitiveTelemetry.missedOpportunitiesCount > 0 ? "rose" : "default"} 
                      className="p-4 relative"
                    >
                      <div className="text-[10px] font-mono text-rose-300 uppercase">
                        Missed Opportunities
                      </div>
                      <div className="text-2xl font-bold font-mono text-white mt-1">
                        {cognitiveTelemetry.missedOpportunitiesCount}
                      </div>
                      <div className="text-[11px] text-slate-400 mt-1">
                        Suboptimal Tool Evals
                      </div>
                    </HudCard>

                    <HudCard variant="cyan" className="p-4 relative">
                      <div className="text-[10px] font-mono text-cyan-300 uppercase">
                        Knowledge Gaps
                      </div>
                      <div className="text-2xl font-bold font-mono text-white mt-1">
                        {cognitiveTelemetry.knowledgeGapsCount}
                      </div>
                      <div className="text-[11px] text-slate-400 mt-1">
                        Missing Vault Entities
                      </div>
                    </HudCard>

                    <HudCard variant="indigo" className="p-4 relative">
                      <div className="text-[10px] font-mono text-indigo-300 uppercase">
                        Scanned Turns
                      </div>
                      <div className="text-2xl font-bold font-mono text-white mt-1">
                        {cognitiveTelemetry.scannedTurns}
                      </div>
                      <div className="text-[11px] text-slate-400 mt-1">
                        Agent Transcripts
                      </div>
                    </HudCard>
                  </div>

                  {/* Main Grid: Dream Engine Profiling vs Agent Telemetry */}
                  <div className="grid grid-cols-1 lg:grid-cols-12 gap-4">
                    {/* Left Column: Dream Engine Introspective Profiling */}
                    <div className="lg:col-span-6 space-y-4">
                      {/* Phase Timings */}
                      <HudCard variant="default" className="p-5">
                        <div className="flex items-center justify-between mb-3">
                          <h3 className="text-xs font-mono font-bold text-white uppercase tracking-wider flex items-center gap-2">
                            <Sparkles className="w-4 h-4 text-purple-400" />
                            Dream Engine Phase Breakdown
                          </h3>
                          <NeonBadge accent="purple" size="sm">
                            Introspective Profiling
                          </NeonBadge>
                        </div>

                        {cognitiveTelemetry.stageTimings.length > 0 ? (
                          <div className="space-y-3">
                            {cognitiveTelemetry.stageTimings.map((stage, idx) => {
                              const totalMs = Number(cognitiveTelemetry.lastCycleDurationMs) || 1;
                              const pct = Math.min(100, Math.max(2, (Number(stage.durationMs) / totalMs) * 100));

                              return (
                                <div key={idx} className="space-y-1">
                                  <div className="flex items-center justify-between text-xs font-mono">
                                    <div className="flex items-center gap-2">
                                      <span className="text-white font-semibold">{stage.name}</span>
                                      {stage.itemsCount > 0 && (
                                        <span className="text-[10px] text-slate-400 bg-white/5 px-1.5 py-0.5 rounded-sm border border-white/10">
                                          {stage.itemsCount} items
                                        </span>
                                      )}
                                    </div>
                                    <span className="text-purple-300 font-bold">{Number(stage.durationMs)}ms</span>
                                  </div>
                                  <div className="w-full bg-void rounded-full h-2 overflow-hidden border border-white/10">
                                    <div 
                                      style={{ width: `${pct}%` }} 
                                      className="h-full bg-linear-to-r from-purple-500 to-cyan-500 transition-all"
                                    />
                                  </div>
                                  {stage.description && (
                                    <div className="text-[10px] text-slate-400">
                                      {stage.description}
                                    </div>
                                  )}
                                </div>
                              );
                            })}
                          </div>
                        ) : (
                          <div className="p-4 text-center text-xs text-slate-500 font-mono">
                            No phase metrics recorded yet. Trigger a dream cycle with stats.
                          </div>
                        )}
                      </HudCard>

                      {/* Detected Latency Bottlenecks */}
                      <HudCard variant="default" className="p-5">
                        <h3 className="text-xs font-mono font-bold text-white uppercase tracking-wider mb-3 flex items-center gap-2">
                          <AlertTriangle className="w-4 h-4 text-amber-400" />
                          Detected Bottlenecks
                        </h3>
                        {cognitiveTelemetry.bottlenecks.length > 0 ? (
                          <ul className="space-y-2">
                            {cognitiveTelemetry.bottlenecks.map((b, idx) => (
                              <li 
                                key={idx} 
                                className="flex items-start gap-2 text-xs font-mono text-amber-200 bg-amber-950/20 border border-amber-500/30 p-2.5 rounded-sm"
                              >
                                <AlertTriangle className="w-3.5 h-3.5 text-amber-400 shrink-0 mt-0.5" />
                                <span>{b}</span>
                              </li>
                            ))}
                          </ul>
                        ) : (
                          <div className="flex items-center gap-2 text-xs font-mono text-emerald-400 bg-emerald-950/20 border border-emerald-500/30 p-3 rounded-sm">
                            <CheckCircle2 className="w-4 h-4 text-emerald-400 shrink-0" />
                            <span>Optimal throughput. No phase latency exceeded the 35% threshold.</span>
                          </div>
                        )}
                      </HudCard>

                      {/* Induced Cross-Domain Syntheses */}
                      <HudCard variant="default" className="p-5">
                        <h3 className="text-xs font-mono font-bold text-white uppercase tracking-wider mb-3 flex items-center gap-2">
                          <Zap className="w-4 h-4 text-cyan-400" />
                          Cross-Domain Syntheses
                        </h3>
                        {cognitiveTelemetry.inducedSyntheses.length > 0 ? (
                          <ul className="space-y-2">
                            {cognitiveTelemetry.inducedSyntheses.map((syn, idx) => (
                              <li 
                                key={idx} 
                                className="flex items-start gap-2 text-xs font-mono text-slate-200 bg-surface/60 border border-cyan-500/20 p-2.5 rounded-sm"
                              >
                                <Sparkles className="w-3.5 h-3.5 text-cyan-400 shrink-0 mt-0.5" />
                                <span className="leading-relaxed">{syn}</span>
                              </li>
                            ))}
                          </ul>
                        ) : (
                          <div className="p-3 text-center text-xs text-slate-500 font-mono">
                            No cross-domain syntheses generated in the latest cycle.
                          </div>
                        )}
                      </HudCard>
                    </div>

                    {/* Right Column: Agent Telemetry & Learning Ledger */}
                    <div className="lg:col-span-6 space-y-4">
                      {/* Agent Audit Action Card */}
                      <HudCard variant="default" className="p-5">
                        <div className="flex items-center justify-between gap-3 flex-wrap">
                          <div>
                            <h3 className="text-xs font-mono font-bold text-white uppercase tracking-wider flex items-center gap-2">
                              <Brain className="w-4 h-4 text-rose-400" />
                              Agent Transcript Audits & Evals
                            </h3>
                            <p className="text-[11px] text-slate-400 mt-0.5">
                              Continuous evaluation of Antigravity and agent interactions
                            </p>
                          </div>
                          <CyberButton
                            onClick={() => {
                              const audit = routines.find(r => r.name === 'audit_agents');
                              if (audit) setSelectedRoutine(audit);
                              setActiveTab('routines');
                              exec.executeRoutine('audit_agents', exec.verbose);
                            }}
                            disabled={exec.isRunning}
                            size="sm"
                            variant="hud"
                            accent="rose"
                            className="text-xs py-1.5 px-3 flex items-center gap-1.5 shrink-0"
                            data-testid="cockpit-audit-now-btn"
                          >
                            <Play className="w-3 h-3 text-rose-300 fill-rose-300/30" />
                            <span>Run Transcript Audit</span>
                          </CyberButton>
                        </div>
                      </HudCard>

                      {/* Recent Evaluations List */}
                      <HudCard variant="default" className="p-5">
                        <h3 className="text-xs font-mono font-bold text-white uppercase tracking-wider mb-3">
                          Recent Agent Calibrations
                        </h3>
                        {cognitiveTelemetry.recentEvals.length > 0 ? (
                          <div className="space-y-3">
                            {cognitiveTelemetry.recentEvals.map((item, idx) => (
                              <div 
                                key={idx}
                                className="p-3 bg-surface/70 rounded-hud-md border border-white/10 space-y-2"
                              >
                                <div className="flex items-center justify-between gap-2 flex-wrap">
                                  <div className="flex items-center gap-2">
                                    <NeonBadge 
                                      accent={
                                        item.classification === 'missed_opportunity' 
                                          ? 'rose' 
                                          : item.classification === 'knowledge_gap' 
                                            ? 'amber' 
                                            : 'purple'
                                      }
                                      size="sm"
                                    >
                                      {item.classification === 'missed_opportunity' 
                                        ? 'MISSED OPPORTUNITY' 
                                        : item.classification === 'knowledge_gap' 
                                          ? 'KNOWLEDGE GAP' 
                                          : item.classification.toUpperCase()}
                                    </NeonBadge>
                                    <span className="font-mono text-xs font-bold text-white">
                                      {item.agent}
                                    </span>
                                  </div>
                                  <span className="text-[10px] font-mono text-slate-500">
                                    {item.topic}
                                  </span>
                                </div>
                                <p className="text-xs text-slate-300 leading-relaxed font-mono">
                                  {item.description}
                                </p>
                                {item.suggestedAction && (
                                  <div className="text-[11px] font-mono text-cyan-300 bg-cyan-950/30 border border-cyan-500/20 p-2 rounded-sm">
                                    💡 <span className="text-slate-400">Action:</span> {item.suggestedAction}
                                  </div>
                                )}
                              </div>
                            ))}
                          </div>
                        ) : (
                          <div className="p-6 text-center text-xs text-slate-500 font-mono">
                            No agent missed opportunities or calibrations recorded yet.
                          </div>
                        )}
                      </HudCard>

                      {/* Vault Health & Evolution Ledger Summary */}
                      <HudCard variant="default" className="p-5 space-y-3">
                        <h3 className="text-xs font-mono font-bold text-white uppercase tracking-wider flex items-center gap-2">
                          <ShieldCheck className="w-4 h-4 text-emerald-400" />
                          Vault Knowledge Mesh Status
                        </h3>
                        <div className="grid grid-cols-2 gap-2 text-xs font-mono">
                          <div className="p-2.5 bg-void/60 border border-white/5 rounded-sm">
                            <span className="text-slate-400 block text-[10px] uppercase">Notes Scanned</span>
                            <span className="text-base font-bold text-white">
                              {cognitiveTelemetry.totalFilesScanned}
                            </span>
                          </div>
                          <div className="p-2.5 bg-void/60 border border-white/5 rounded-sm">
                            <span className="text-slate-400 block text-[10px] uppercase">Broken Links Triaged</span>
                            <span className="text-base font-bold text-cyan-300">
                              {cognitiveTelemetry.brokenLinksCount}
                            </span>
                          </div>
                        </div>
                        {cognitiveTelemetry.evolutionLogPath && (
                          <div className="text-[11px] font-mono text-slate-400 pt-1">
                            Evolution Ledger: <span className="text-slate-200">{cognitiveTelemetry.evolutionLogPath}</span>
                          </div>
                        )}
                      </HudCard>
                    </div>
                  </div>
                </>
              ) : (
                <div className="p-8 text-center text-xs text-slate-500 font-mono">
                  Cognitive telemetry data unavailable. Run `daneel dream --stats` to generate introspective data.
                </div>
              )}
            </div>
          )}

          {/* Footer Quick Hints */}
          <div className="px-6 py-2.5 border-t border-white/10 bg-[#070a14]/70 flex flex-wrap items-center justify-between text-[11px] text-slate-400 font-mono">
            <div className="flex items-center gap-4">
              <span>Press <HotkeyBadge keys={['Esc']} /> to dismiss</span>
              <span>Execute <HotkeyBadge keys={['Ctrl', 'Enter']} /></span>
              <span>Toggle verbose <HotkeyBadge keys={['V']} /></span>
            </div>
            <div className="flex items-center gap-2 text-slate-500">
              <ShieldCheck className="w-3.5 h-3.5 text-emerald-400" />
              <span>CGO-Free Native Execution</span>
            </div>
          </div>
        </HudCard>
      </div>

      <RoutineEditorModal
        isOpen={isEditorOpen}
        onClose={() => setIsEditorOpen(false)}
        onSaved={(savedName) => loadRoutines(savedName)}
        initialRoutine={editingRoutine}
      />
    </div>
  );
};
