import React, { useState } from 'react';
import { 
  Clock, 
  RefreshCw, 
  AlertTriangle, 
  CheckCircle2, 
  ChevronDown, 
  ChevronUp, 
  Loader2 
} from 'lucide-react';
import { NeonBadge } from './NeonBadge';
import { CyberButton } from './CyberButton';
import type { RoutineRunItem } from '@/gen/api/v1/graph_pb';

export function formatRelativeTime(unixSec: bigint | number): string {
  const sec = typeof unixSec === 'bigint' ? Number(unixSec) : unixSec;
  if (!sec || sec <= 0) return 'Never';
  const now = Math.floor(Date.now() / 1000);
  const diff = now - sec;
  if (diff < 5) return 'Just now';
  if (diff < 60) return `${diff}s ago`;
  const mins = Math.floor(diff / 60);
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

export function formatDuration(durationMs: bigint | number): string {
  const ms = typeof durationMs === 'bigint' ? Number(durationMs) : durationMs;
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

export interface RoutineHistoryDeckProps {
  runs: RoutineRunItem[];
  isLoading: boolean;
  onRefresh: () => void;
  onSelectRoutine?: (name: string) => void;
  testIdPrefix?: string;
}

export const RoutineHistoryDeck: React.FC<RoutineHistoryDeckProps> = ({
  runs,
  isLoading,
  onRefresh,
  onSelectRoutine,
  testIdPrefix = 'history-deck',
}) => {
  const [expandedRunId, setExpandedRunId] = useState<string | null>(null);

  const toggleExpand = (id: string) => {
    setExpandedRunId((prev) => (prev === id ? null : id));
  };

  const getTriggerBadge = (trigger: string) => {
    const norm = trigger.toLowerCase();
    switch (norm) {
      case 'systemd':
        return <NeonBadge accent="indigo" size="sm">SYSTEMD</NeonBadge>;
      case 'cron':
        return <NeonBadge accent="amber" size="sm">CRON</NeonBadge>;
      case 'web_cockpit':
      case 'cockpit':
        return <NeonBadge accent="cyan" size="sm">COCKPIT</NeonBadge>;
      case 'cli':
        return <NeonBadge accent="bronze" size="sm">CLI</NeonBadge>;
      case 'mcp':
        return <NeonBadge accent="purple" size="sm">MCP</NeonBadge>;
      default:
        return <NeonBadge accent="bronze" size="sm">{trigger.toUpperCase()}</NeonBadge>;
    }
  };

  return (
    <div className="flex flex-col h-full bg-void/95 overflow-hidden" data-testid={`${testIdPrefix}-container`}>
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-2.5 bg-surface border-b border-white/10 shrink-0">
        <div className="flex items-center gap-2">
          <Clock className="w-4 h-4 text-cyan-400" />
          <span className="font-mono text-xs font-bold text-white tracking-wider">
            RECENT RUNS / EXECUTION TIMELINE
          </span>
          <span className="text-[10px] font-mono text-slate-400 bg-white/5 px-2 py-0.5 rounded-sm border border-white/10">
            {runs.length} recorded
          </span>
        </div>
        <CyberButton
          onClick={onRefresh}
          variant="ghost"
          size="sm"
          data-testid={`${testIdPrefix}-refresh-btn`}
          className="text-xs text-slate-400 hover:text-cyan-300 flex items-center gap-1.5 py-1 px-2"
          title="Refresh execution history"
        >
          <RefreshCw className={`w-3.5 h-3.5 ${isLoading ? 'animate-spin text-cyan-400' : ''}`} />
          <span>Refresh</span>
        </CyberButton>
      </div>

      {/* Content Table / Feed */}
      <div className="flex-1 overflow-y-auto p-3 space-y-2">
        {isLoading && runs.length === 0 ? (
          <div className="flex flex-col items-center justify-center p-12 text-slate-500 text-xs gap-2">
            <Loader2 className="w-5 h-5 animate-spin text-cyan-400" />
            <span>Loading routine execution history...</span>
          </div>
        ) : runs.length === 0 ? (
          <div className="p-8 text-center text-xs text-slate-500 font-mono">
            No routine executions recorded in ledger yet.
          </div>
        ) : (
          <div className="divide-y divide-white/5 rounded-hud-md border border-white/10 bg-[#070a14]/60 overflow-hidden font-mono text-xs">
            {/* Table Header */}
            <div className="grid grid-cols-12 gap-2 px-3 py-2 bg-white/5 text-[10px] text-slate-400 uppercase font-semibold">
              <div className="col-span-2">Status</div>
              <div className="col-span-3">Routine</div>
              <div className="col-span-2">Trigger</div>
              <div className="col-span-2">Started</div>
              <div className="col-span-1">Duration</div>
              <div className="col-span-2 text-right">Details</div>
            </div>

            {/* Rows */}
            {runs.map((run) => {
              const isExpanded = expandedRunId === run.id;
              const isSuccess = run.status === 'success' || run.exitCode === 0;

              return (
                <div key={run.id} className="transition-colors hover:bg-white/2" data-testid={`${testIdPrefix}-run-item`}>
                  <div 
                    onClick={() => toggleExpand(run.id)}
                    className="grid grid-cols-12 gap-2 px-3 py-2.5 items-center cursor-pointer select-none"
                  >
                    {/* Status */}
                    <div className="col-span-2 flex items-center gap-1.5">
                      {isSuccess ? (
                        <CheckCircle2 className="w-3.5 h-3.5 text-emerald-400 shrink-0" />
                      ) : (
                        <AlertTriangle className="w-3.5 h-3.5 text-rose-400 shrink-0" />
                      )}
                      <NeonBadge accent={isSuccess ? 'emerald' : 'rose'} size="sm">
                        {isSuccess ? 'SUCCESS' : `EXIT ${run.exitCode}`}
                      </NeonBadge>
                    </div>

                    {/* Routine Name */}
                    <div className="col-span-3 flex items-center gap-1.5 truncate">
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          if (onSelectRoutine) onSelectRoutine(run.routineName);
                        }}
                        className="text-white hover:text-cyan-300 font-bold truncate text-left cursor-pointer"
                        title={`Select routine: ${run.routineName}`}
                      >
                        {run.routineName}
                      </button>
                    </div>

                    {/* Trigger */}
                    <div className="col-span-2 flex items-center">
                      {getTriggerBadge(run.triggerSource)}
                    </div>

                    {/* Started */}
                    <div 
                      className="col-span-2 text-slate-300 text-[11px]" 
                      title={new Date(Number(run.startedAt) * 1000).toLocaleString()}
                    >
                      {formatRelativeTime(run.startedAt)}
                    </div>

                    {/* Duration */}
                    <div className="col-span-1 text-slate-400 text-[11px]">
                      {formatDuration(run.durationMs)}
                    </div>

                    {/* Details Action / Chevron */}
                    <div className="col-span-2 flex items-center justify-end gap-1.5 text-slate-400">
                      <span className="text-[10px] truncate max-w-[120px] text-slate-500">
                        {run.summary || (isSuccess ? 'Completed' : 'Failed')}
                      </span>
                      {isExpanded ? (
                        <ChevronUp className="w-3.5 h-3.5 text-slate-400" />
                      ) : (
                        <ChevronDown className="w-3.5 h-3.5 text-slate-400" />
                      )}
                    </div>
                  </div>

                  {/* Expanded Summary & Error Log */}
                  {isExpanded && (
                    <div className="px-4 py-3 bg-[#05070f] border-t border-white/5 space-y-2 text-[11px]">
                      {run.summary && (
                        <div>
                          <span className="text-slate-400 font-semibold">Summary: </span>
                          <span className="text-slate-200">{run.summary}</span>
                        </div>
                      )}
                      <div>
                        <span className="text-slate-400 font-semibold">Started At: </span>
                        <span className="text-slate-200">{new Date(Number(run.startedAt) * 1000).toISOString()}</span>
                        {run.completedAt > 0 && (
                          <>
                            <span className="text-slate-500 mx-2">|</span>
                            <span className="text-slate-400 font-semibold">Completed: </span>
                            <span className="text-slate-200">{new Date(Number(run.completedAt) * 1000).toISOString()}</span>
                          </>
                        )}
                      </div>
                      {run.errorMessage && (
                        <div className="p-2.5 rounded-sm bg-rose-950/30 border border-rose-500/30 text-rose-300 font-mono text-[10px] whitespace-pre-wrap">
                          <div className="flex items-center gap-1.5 font-bold mb-1 text-rose-400">
                            <AlertTriangle className="w-3.5 h-3.5" />
                            Error Diagnostic Output:
                          </div>
                          {run.errorMessage}
                        </div>
                      )}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
};
