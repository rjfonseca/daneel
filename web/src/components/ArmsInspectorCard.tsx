import type { GalaxyNode } from '../types/galaxy';
import React, { useState, useEffect } from 'react';
import { 
  X, 
  Sparkles,
  FileText,
  Terminal,
  Play,
  Maximize2,
  Loader2
} from 'lucide-react';
import ReactMarkdown from 'react-markdown';
import { ScrollArea } from "@/components/ui/scroll-area";
import { CyberButton } from "@/components/hud/CyberButton";
import { CyberTerminal } from "@/components/hud/CyberTerminal";
import { SkillParameterDeck } from "@/components/hud/SkillParameterDeck";
import { resolveStationRoutine } from "@/lib/stationResolver";
import { useRoutineExecution, type UseRoutineExecutionResult } from "@/hooks/useRoutineExecution";
import { cn } from "@/lib/utils";

interface ArmsInspectorCardProps {
  station: GalaxyNode;
  contentMarkdown?: string;
  onClose: () => void;
  isConstellationActive: boolean;
  onResetConstellation: () => void;
  onNavigateInternalLink?: (href: string) => void;
  execution?: UseRoutineExecutionResult;
  onOpenCockpit?: (routineName?: string) => void;
}

const renderIcon = (icon?: string) => {
  const map: Record<string, string> = {
    github: '🛸',
    gitlab: '🦊',
    jira: '🔷',
    gdrive: '📁',
    mcp: '🔌',
    moon: '🌙',
    clock: '⏱️',
    radar: '📡',
    sparkles: '✨',
    users: '👥',
    list: '📋',
    'git-branch': '🌿',
    award: '🏆',
  };
  return (icon && map[icon.toLowerCase()]) || icon || '🛰️';
};

export const ArmsInspectorCard: React.FC<ArmsInspectorCardProps> = ({
  station,
  contentMarkdown,
  onClose,
  isConstellationActive,
  onResetConstellation,
  onNavigateInternalLink,
  execution,
  onOpenCockpit,
}) => {
  const [activeTab, setActiveTab] = useState<'spec' | 'terminal'>('spec');
  const internalExecution = useRoutineExecution();
  const exec = execution ?? internalExecution;

  const executable = React.useMemo(() => resolveStationRoutine(station), [station]);

  useEffect(() => {
    setActiveTab('spec');
  }, [station?.id]);

  const ringInfo = React.useMemo(() => {
    const ring = station?.ring;
    if (ring === 'skills' || ring === '1') {
      return {
        label: 'FIRST FOUNDATION // TERMINUS OPERATIONAL (SKILL)',
        desc: 'Autonomous agent capability & execution protocol (Isaac Asimov First Foundation).',
        color: '#a855f7',
        badgeBg: 'bg-purple-900/30 text-purple-300 border-purple-700/50',
        glow: 'shadow-[0_0_25px_rgba(168,85,247,0.3)]',
      };
    }
    if (ring === 'routines' || ring === '2') {
      return {
        label: 'SECOND FOUNDATION // STAR\'S END MENTALICS (ROUTINE)',
        desc: 'Continuous telemetry, automated health cadences & background synthesis.',
        color: '#f59e0b',
        badgeBg: 'bg-amber-900/30 text-amber-300 border-amber-700/50',
        glow: 'shadow-[0_0_25px_rgba(245,158,11,0.3)]',
      };
    }
    return {
      label: 'FIRST FOUNDATION // TERMINUS EXPEDITIONARY (HARVESTER)',
      desc: 'External data ingestion satellite & service synchronization pipeline.',
      color: '#10b981',
      badgeBg: 'bg-emerald-900/30 text-emerald-300 border-emerald-700/50',
      glow: 'shadow-[0_0_25px_rgba(16,185,129,0.3)]',
    };
  }, [station?.ring]);

  if (!station) return null;

  return (
    <aside
      data-testid="arms-inspector-card"
      aria-label="Seldon Station Holographic Inspector"
      style={{ top: 'var(--hud-content-top, 128px)' }}
      className={cn(
        "fixed right-4 sm:right-6 bottom-6 w-[460px] max-w-[calc(100vw-2rem)] z-40 flex flex-col bg-[#08090f]/95 backdrop-blur-2xl border border-indigo-500/40 rounded-2xl shadow-hud-deep transition-all duration-300 ease-out overflow-hidden select-text",
        ringInfo.glow
      )}
    >
      {/* Top Holographic Header */}
      <div className="p-5 sm:p-6 border-b border-gray-800/80 bg-linear-to-b from-indigo-950/40 via-purple-950/10 to-transparent flex flex-col gap-3">
        {/* Ring & Status Row */}
        <div className="flex items-center justify-between gap-2 flex-wrap">
          <div className="flex items-center gap-2">
            <span 
              className={cn("px-2.5 py-1 text-[10px] font-mono font-bold uppercase tracking-wider rounded-md border", ringInfo.badgeBg)}
            >
              {ringInfo.label}
            </span>

            <span className="inline-flex items-center gap-1.5 px-2 py-0.5 text-[10px] font-mono font-bold uppercase rounded-md bg-emerald-950/50 text-emerald-300 border border-emerald-700/40">
              <span className="relative flex h-2 w-2">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
              </span>
              {station.status || 'ONLINE'}
            </span>
          </div>

          <div className="flex items-center gap-1.5">
            {executable && onOpenCockpit && (
              <button
                onClick={() => onOpenCockpit(executable.routineName)}
                aria-label="Open in Operational Cockpit"
                className="p-1.5 rounded-lg bg-gray-800/60 hover:bg-cyan-500/20 text-slate-300 hover:text-cyan-400 transition-colors cursor-pointer cosmic-focus-visible"
                title="Open in Cockpit (Alt+C)"
                data-testid="arms-open-cockpit-btn"
              >
                <Maximize2 className="w-4 h-4" />
              </button>
            )}
            <button
              onClick={onClose}
              aria-label="Close Inspector (Esc)"
              className="p-1.5 rounded-lg bg-gray-800/60 hover:bg-red-500/20 text-slate-300 hover:text-red-400 transition-colors cursor-pointer cosmic-focus-visible"
              title="Close Inspector (Esc)"
              data-testid="close-arms-inspector"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>

        {/* Station Title & Icon */}
        <div className="flex items-center gap-3">
          <div className="w-11 h-11 rounded-xl bg-gray-900/90 border border-gray-700/80 flex items-center justify-center text-2xl shadow-inner select-none">
            {renderIcon(station.icon)}
          </div>
          <div className="min-w-0 flex-1">
            <h2 className="text-xl sm:text-2xl font-black text-white leading-tight select-text cursor-text selection:bg-indigo-500/40 selection:text-white truncate">
              {station.title || station.id}
            </h2>
            <p className="text-xs text-gray-400 mt-0.5">{ringInfo.desc}</p>
          </div>
        </div>

        {/* Constellation Isolation Alert & Reset Button */}
        {isConstellationActive && (
          <button
            onClick={onResetConstellation}
            data-testid="constellation-reset-btn"
            aria-label="Restore full galaxy view from isolated constellation"
            className="group flex items-center justify-between px-3 py-1.5 text-xs font-mono rounded-lg bg-indigo-950/50 border border-indigo-500/40 text-indigo-300 hover:bg-indigo-900/40 transition-all cursor-pointer cosmic-focus-visible"
            title="Click to restore full galaxy view"
          >
            <span className="flex items-center gap-1.5">
              <Sparkles className="w-3.5 h-3.5 text-indigo-400 animate-pulse" />
              <span>Isolated Constellation (1-2 Hops)</span>
            </span>
            <span className="text-[10px] uppercase text-indigo-400 underline group-hover:text-white">Restore</span>
          </button>
        )}

        {/* Action Tabs & Quick Run */}
        {executable && (
          <div className="flex items-center justify-between gap-2 pt-2 border-t border-gray-800/60">
            <div className="flex rounded-hud-md bg-surface/80 border border-white/10 p-0.5">
              <button
                onClick={() => setActiveTab('spec')}
                data-testid="arms-tab-spec"
                className={cn(
                  "px-2.5 py-1 text-xs font-mono rounded-sm transition-all cursor-pointer cosmic-focus-visible flex items-center gap-1.5",
                  activeTab === 'spec'
                    ? "bg-indigo-500/20 text-indigo-300 border border-indigo-500/40 shadow-glow-indigo"
                    : "text-slate-400 hover:text-white hover:bg-white/5"
                )}
              >
                <FileText className="w-3.5 h-3.5" />
                <span>Hologram Spec</span>
              </button>
              <button
                onClick={() => setActiveTab('terminal')}
                data-testid="arms-tab-terminal"
                className={cn(
                  "px-2.5 py-1 text-xs font-mono rounded-sm transition-all cursor-pointer cosmic-focus-visible flex items-center gap-1.5",
                  activeTab === 'terminal'
                    ? "bg-cyan-500/20 text-cyan-300 border border-cyan-500/40 shadow-glow-cyan"
                    : "text-slate-400 hover:text-white hover:bg-white/5"
                )}
              >
                <Terminal className="w-3.5 h-3.5" />
                <span>Live Terminal</span>
                {exec.isRunning && (
                  <span className="relative flex h-2 w-2 ml-0.5">
                    <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-amber-400 opacity-75"></span>
                    <span className="relative inline-flex rounded-full h-2 w-2 bg-amber-500"></span>
                  </span>
                )}
              </button>
            </div>

            {activeTab === 'spec' && (
              <CyberButton
                onClick={() => {
                  setActiveTab('terminal');
                  exec.executeRoutine(executable.routineName, exec.verbose);
                }}
                disabled={exec.isRunning}
                variant="hud"
                accent={
                  executable.armsLayer === 'skill'
                    ? 'purple'
                    : executable.armsLayer === 'application'
                    ? 'emerald'
                    : 'cyan'
                }
                size="sm"
                data-testid="arms-execute-routine-btn"
                className="gap-1.5 px-3 py-1 text-xs shrink-0"
              >
                {exec.isRunning && exec.runningRoutineName === executable.routineName ? (
                  <Loader2 className="w-3 h-3 animate-spin text-cyan-300" />
                ) : executable.armsLayer === 'skill' ? (
                  <Sparkles className="w-3 h-3 text-purple-300" />
                ) : (
                  <Play className="w-3 h-3 fill-current" />
                )}
                <span>
                  {executable.armsLayer === 'skill'
                    ? 'Dispatch Agent'
                    : executable.armsLayer === 'application'
                    ? 'Sync Platform'
                    : 'Run Routine'}
                </span>
              </CyberButton>
            )}
          </div>
        )}
      </div>

      {/* Body: Spec or Terminal */}
      {activeTab === 'terminal' && executable ? (
        <div className="flex-1 min-h-0">
          <CyberTerminal
            logs={exec.logs}
            isRunning={exec.isRunning}
            runningRoutineName={exec.runningRoutineName}
            targetRoutineTitle={`${executable.title} (${executable.routineName})`}
            verbose={exec.verbose}
            onToggleVerbose={() => exec.setVerbose(!exec.verbose)}
            onAbort={exec.abortRoutine}
            onClear={exec.clearLogs}
            onExecute={() => exec.executeRoutine(executable.routineName, exec.verbose)}
            onOpenCockpit={onOpenCockpit ? () => onOpenCockpit(executable.routineName) : undefined}
            compact
            testIdPrefix="arms"
          />
        </div>
      ) : (
        <ScrollArea className="flex-1 min-h-0 select-text">
          <div className="p-5 sm:p-6 pb-12 select-text cursor-text selection:bg-indigo-500/40 selection:text-white space-y-5">
            {/* Specialized ARMS Layer Panels */}
            {executable?.armsLayer === 'skill' && (
              <SkillParameterDeck
                skillName={executable.routineName}
                skillTitle={executable.title}
                isRunning={exec.isRunning}
                onDispatch={(params) => {
                  setActiveTab('terminal');
                  exec.executeRoutine(executable.routineName, exec.verbose, params);
                }}
                testIdPrefix="arms-skill"
              />
            )}

            {executable?.armsLayer === 'application' && (
              <div 
                data-testid="arms-app-deck"
                className="rounded-xl border border-emerald-500/30 bg-[#070a14]/80 backdrop-blur-md p-3 sm:p-4 shadow-[0_0_20px_rgba(16,185,129,0.12)] space-y-3"
              >
                <div className="flex items-center justify-between gap-2 border-b border-emerald-500/20 pb-2.5">
                  <div className="flex items-center gap-2 min-w-0">
                    <span className="px-2 py-0.5 text-[10px] font-mono font-bold uppercase rounded-sm bg-emerald-950/60 text-emerald-300 border border-emerald-500/30">
                      {executable.platform ? `${executable.platform} Harvester` : 'Harvester Service'}
                    </span>
                    {executable.cliCommand && (
                      <code className="text-[10px] font-mono text-slate-400 bg-black/40 px-1.5 py-0.5 rounded-sm border border-white/5">
                        {executable.cliCommand}
                      </code>
                    )}
                  </div>
                </div>
                <div className="flex items-center justify-between gap-2 pt-1 flex-wrap sm:flex-nowrap">
                  <span className="text-[11px] font-mono text-slate-400">
                    Ingest technical documentation and architecture records
                  </span>
                  <div className="flex items-center gap-2 shrink-0">
                    {executable.routineName !== 'harvest' && (
                      <CyberButton
                        onClick={() => {
                          setActiveTab('terminal');
                          exec.executeRoutine('harvest', exec.verbose);
                        }}
                        disabled={exec.isRunning}
                        variant="ghost"
                        size="sm"
                        data-testid="arms-sync-all-btn"
                        className="text-slate-400 hover:text-emerald-300 border border-emerald-500/20 text-xs px-2.5 py-1"
                        title="Run all configured harvesters"
                      >
                        <span>Sync All</span>
                      </CyberButton>
                    )}
                    <CyberButton
                      onClick={() => {
                        setActiveTab('terminal');
                        exec.executeRoutine(executable.routineName, exec.verbose);
                      }}
                      disabled={exec.isRunning}
                      variant="hud"
                      accent="emerald"
                      size="sm"
                      data-testid="arms-sync-platform-btn"
                      className="gap-1.5 px-3 py-1 text-xs font-bold"
                    >
                      <Play className="w-3 h-3 fill-emerald-300" />
                      <span>Sync {executable.platform || 'Platform'}</span>
                    </CyberButton>
                  </div>
                </div>
              </div>
            )}

            {executable?.armsLayer === 'routine' && (
              <div 
                data-testid="arms-routine-deck"
                className="rounded-xl border border-amber-500/30 bg-[#070a14]/80 backdrop-blur-md p-3 sm:p-4 shadow-[0_0_20px_rgba(245,158,11,0.12)] space-y-3"
              >
                <div className="flex items-center justify-between gap-2 border-b border-amber-500/20 pb-2.5">
                  <div className="flex items-center gap-2 min-w-0">
                    <span className="px-2 py-0.5 text-[10px] font-mono font-bold uppercase rounded-sm bg-amber-950/60 text-amber-300 border border-amber-500/30">
                      Cadence: {executable.schedule || 'Automated Routine'}
                    </span>
                    {executable.cliCommand && (
                      <code className="text-[10px] font-mono text-slate-400 bg-black/40 px-1.5 py-0.5 rounded-sm border border-white/5">
                        {executable.cliCommand}
                      </code>
                    )}
                  </div>
                </div>
                <div className="flex items-center justify-between gap-2 pt-1 flex-wrap sm:flex-nowrap">
                  <span className="text-[11px] font-mono text-slate-400">
                    Deterministic background cadence & telemetry maintenance
                  </span>
                  <CyberButton
                    onClick={() => {
                      setActiveTab('terminal');
                      exec.executeRoutine(executable.routineName, exec.verbose);
                    }}
                    disabled={exec.isRunning}
                    variant="hud"
                    accent="amber"
                    size="sm"
                    data-testid="arms-run-routine-deck-btn"
                    className="gap-1.5 px-3 py-1 text-xs font-bold shrink-0"
                  >
                    <Play className="w-3 h-3 fill-amber-300" />
                    <span>Run Routine</span>
                  </CyberButton>
                </div>
              </div>
            )}

            {contentMarkdown ? (
              <div className="prose prose-invert prose-indigo text-xs sm:text-sm text-gray-200 leading-relaxed max-w-none select-text selection:bg-indigo-500/40 selection:text-white">
                <ReactMarkdown
                  components={{
                    a: ({ href, children, ...props }) => {
                      const isExternal = href?.startsWith('http://') || href?.startsWith('https://') || href?.startsWith('mailto:');
                      if (!isExternal && href && onNavigateInternalLink) {
                        return (
                          <a
                            href="#"
                            onClick={(e) => {
                              e.preventDefault();
                              onNavigateInternalLink(href);
                            }}
                            className="text-indigo-400 hover:text-indigo-300 underline font-semibold"
                            {...props}
                          >
                            {children}
                          </a>
                        );
                      }
                      return (
                        <a href={href} target="_blank" rel="noopener noreferrer" className="text-cyan-400 underline" {...props}>
                          {children}
                        </a>
                      );
                    }
                  }}
                >
                  {contentMarkdown}
                </ReactMarkdown>
              </div>
            ) : (
              <div className="text-gray-400 text-xs py-8 text-center font-mono">
                Loading station telemetry...
              </div>
            )}
          </div>
        </ScrollArea>
      )}
    </aside>
  );
};
