import React, { useRef, useEffect, useState } from 'react';
import { 
  Play, 
  Square, 
  Trash2, 
  Terminal, 
  Maximize2,
  Copy,
  Check,
  Search,
  Download,
  X
} from 'lucide-react';
import { CyberButton } from './CyberButton';
import { NeonBadge } from './NeonBadge';
import { cn } from '@/lib/utils';

export interface CyberTerminalProps {
  logs: string[];
  isRunning: boolean;
  runningRoutineName?: string | null;
  targetRoutineTitle?: string;
  verbose: boolean;
  onToggleVerbose: () => void;
  onAbort: () => void;
  onClear: () => void;
  onExecute?: () => void;
  executeDisabled?: boolean;
  onOpenCockpit?: () => void;
  compact?: boolean;
  testIdPrefix?: string;
  className?: string;
}

export const CyberTerminal: React.FC<CyberTerminalProps> = ({
  logs,
  isRunning,
  runningRoutineName,
  targetRoutineTitle,
  verbose,
  onToggleVerbose,
  onAbort,
  onClear,
  onExecute,
  executeDisabled = false,
  onOpenCockpit,
  compact = false,
  testIdPrefix = 'terminal',
  className,
}) => {
  const terminalEndRef = useRef<HTMLDivElement>(null);
  const [copiedLogs, setCopiedLogs] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [showSearch, setShowSearch] = useState(false);

  // Auto-scroll to bottom on log line update
  useEffect(() => {
    if (!searchQuery && terminalEndRef.current && typeof terminalEndRef.current.scrollIntoView === 'function') {
      terminalEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [logs, searchQuery]);

  const handleCopyLogs = async () => {
    if (logs.length === 0) return;
    try {
      if (navigator?.clipboard?.writeText) {
        await navigator.clipboard.writeText(logs.join('\n'));
      }
      setCopiedLogs(true);
      setTimeout(() => setCopiedLogs(false), 2000);
    } catch (err) {
      console.warn('Failed to copy logs to clipboard:', err);
    }
  };

  const handleDownloadLogs = () => {
    if (logs.length === 0) return;
    try {
      const blob = new Blob([logs.join('\n')], { type: 'text/plain;charset=utf-8' });
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      const timestamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
      link.download = `daneel-terminal-${timestamp}.txt`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);
    } catch (err) {
      console.warn('Failed to download logs:', err);
    }
  };

  const filteredLogs = searchQuery.trim()
    ? logs.filter((line) => line.toLowerCase().includes(searchQuery.toLowerCase()))
    : logs;

  return (
    <div className={cn('flex flex-col h-full bg-void/90 overflow-hidden', className)}>
      {/* Terminal Action Header */}
      <div 
        className={cn(
          'border-b border-white/10 flex flex-wrap items-center justify-between gap-2 bg-surface/80 select-none',
          compact ? 'p-2 sm:px-3' : 'p-3 sm:px-4'
        )}
      >
        <div className="flex items-center gap-2 min-w-0">
          <Terminal className="w-3.5 h-3.5 text-cyan-400 shrink-0" />
          <span className="text-xs font-mono font-bold text-white uppercase truncate">
            {targetRoutineTitle || (runningRoutineName ? `Routine: ${runningRoutineName}` : 'Terminal')}
          </span>
          <NeonBadge 
            accent={isRunning ? 'amber' : 'emerald'} 
            ping={isRunning} 
            size="sm"
          >
            {isRunning ? 'EXEC' : 'IDLE'}
          </NeonBadge>
        </div>

        <div className="flex items-center gap-1.5 shrink-0">
          {/* Toggle Log Search */}
          <CyberButton
            onClick={() => {
              setShowSearch(!showSearch);
              if (showSearch) setSearchQuery('');
            }}
            variant="ghost"
            size="sm"
            title="Search terminal logs"
            data-testid={`${testIdPrefix}-search-toggle-btn`}
            className={`p-1.5 ${showSearch ? 'text-cyan-300 bg-cyan-500/10' : 'text-slate-400 hover:text-white'}`}
          >
            <Search className="w-3.5 h-3.5" />
          </CyberButton>

          {/* Verbose Mode Toggle */}
          <button
            onClick={onToggleVerbose}
            disabled={isRunning}
            data-testid={`${testIdPrefix}-verbose-toggle`}
            aria-pressed={verbose}
            className={`px-2 py-1 text-[11px] font-mono rounded-sm border transition-all cursor-pointer cosmic-focus-visible flex items-center gap-1 ${
              verbose
                ? 'bg-amber-500/20 text-amber-300 border-amber-500/40 shadow-glow-amber'
                : 'bg-surface text-slate-400 border-white/10 hover:text-white'
            }`}
            title="Toggle --verbose output"
          >
            <span className="font-bold">V</span>
            {!compact && <span>Verbose</span>}
          </button>

          {/* Clear Logs */}
          <CyberButton
            onClick={onClear}
            variant="ghost"
            size="sm"
            title="Clear terminal logs"
            data-testid={`${testIdPrefix}-clear-btn`}
            className="text-slate-400 hover:text-white p-1.5"
          >
            <Trash2 className="w-3.5 h-3.5" />
          </CyberButton>

          {/* Download Terminal Output */}
          <CyberButton
            onClick={handleDownloadLogs}
            disabled={logs.length === 0}
            variant="ghost"
            size="sm"
            title="Download terminal logs"
            data-testid={`${testIdPrefix}-download-logs-btn`}
            className="text-slate-400 hover:text-cyan-300 p-1.5"
          >
            <Download className="w-3.5 h-3.5" />
          </CyberButton>

          {/* Copy Terminal Output */}
          <CyberButton
            onClick={handleCopyLogs}
            disabled={logs.length === 0}
            variant="ghost"
            size="sm"
            title="Copy terminal output to clipboard"
            data-testid={`${testIdPrefix}-copy-logs-btn`}
            className="text-slate-400 hover:text-cyan-300 p-1.5"
          >
            {copiedLogs ? (
              <Check className="w-3.5 h-3.5 text-emerald-400" />
            ) : (
              <Copy className="w-3.5 h-3.5" />
            )}
          </CyberButton>

          {/* Pop out to Cockpit Modal if provided */}
          {onOpenCockpit && (
            <CyberButton
              onClick={onOpenCockpit}
              variant="ghost"
              size="sm"
              title="Open in full Cockpit (Alt+C)"
              data-testid={`${testIdPrefix}-open-cockpit-btn`}
              className="text-slate-400 hover:text-cyan-300 p-1.5"
            >
              <Maximize2 className="w-3.5 h-3.5" />
            </CyberButton>
          )}

          {/* Abort or Execute Action */}
          {isRunning ? (
            <CyberButton
              onClick={onAbort}
              variant="hud"
              size="sm"
              data-testid={`${testIdPrefix}-abort-btn`}
              className="border-rose-500/50 text-rose-300 bg-rose-950/40 px-2.5 py-1 text-xs"
            >
              <Square className="w-3 h-3 fill-rose-300" />
              <span>Abort</span>
            </CyberButton>
          ) : (
            onExecute && (
              <CyberButton
                onClick={onExecute}
                disabled={executeDisabled}
                variant="hud"
                accent="cyan"
                size="sm"
                data-testid={`${testIdPrefix}-primary-run-btn`}
                className="px-2.5 py-1 text-xs"
              >
                <Play className="w-3 h-3 fill-cyan-300" />
                <span>Execute</span>
              </CyberButton>
            )
          )}
        </div>
      </div>

      {/* Search Filter Bar */}
      {showSearch && (
        <div 
          className="px-3 py-1.5 bg-[#070a14] border-b border-white/10 flex items-center justify-between gap-2 text-xs select-none"
          data-testid={`${testIdPrefix}-search-bar`}
        >
          <div className="relative flex-1">
            <Search className="w-3 h-3 absolute left-2.5 top-1/2 -translate-y-1/2 text-slate-500" />
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Filter terminal logs..."
              data-testid={`${testIdPrefix}-search-input`}
              className="w-full pl-8 pr-7 py-1 text-xs bg-[#030305] border border-white/10 rounded-sm text-white placeholder-slate-500 focus:outline-hidden focus:border-cyan-500/60 font-mono"
            />
            {searchQuery && (
              <button
                onClick={() => setSearchQuery('')}
                data-testid={`${testIdPrefix}-search-clear-btn`}
                aria-label="Clear search query"
                className="absolute right-2 top-1/2 -translate-y-1/2 text-slate-500 hover:text-slate-300 cursor-pointer"
              >
                <X className="w-3 h-3" />
              </button>
            )}
          </div>
          {searchQuery && (
            <span 
              className="text-[10px] font-mono text-cyan-400 shrink-0"
              data-testid={`${testIdPrefix}-match-count`}
            >
              {filteredLogs.length} / {logs.length} matched
            </span>
          )}
        </div>
      )}

      {/* Monospace Output Screen */}
      <div 
        className="flex-1 p-3 sm:p-4 font-mono text-xs overflow-y-auto space-y-1 select-text bg-[#030305]"
        data-testid={`${testIdPrefix}-terminal`}
      >
        <div className="text-slate-600 mb-2 select-none">
          # Daneel ConnectRPC Operational Stream v1.0
          <br />
          # Real-time stdout/stderr stream from headless routines.
        </div>

        {logs.length === 0 && (
          <div className="text-slate-600 italic py-6 text-center select-none">
            Terminal ready. Click &quot;Execute&quot; to begin headless streaming.
          </div>
        )}

        {logs.length > 0 && filteredLogs.length === 0 && searchQuery && (
          <div className="text-slate-500 italic py-4 text-center select-none">
            No lines matched &quot;{searchQuery}&quot;.
          </div>
        )}

        {filteredLogs.map((line, idx) => {
          let style = 'text-slate-300';
          if (line.includes('❌') || line.includes('[Error]')) {
            style = 'text-rose-400 font-semibold';
          } else if (line.includes('✅') || line.includes('✨')) {
            style = 'text-emerald-400 font-semibold';
          } else if (line.includes('🚀') || line.includes('⚡') || line.includes('🧬')) {
            style = 'text-cyan-300';
          } else if (line.includes('⚠️') || line.includes('💡')) {
            style = 'text-amber-300';
          } else if (line.startsWith('─')) {
            style = 'text-slate-600';
          }

          return (
            <div key={idx} className={`leading-relaxed break-all ${style}`}>
              {line}
            </div>
          );
        })}

        {isRunning && (
          <div className="flex items-center gap-2 text-cyan-400 pt-1 animate-pulse select-none">
            <span className="inline-block w-2 h-3.5 bg-cyan-400" />
            <span>Streaming routine stdout/stderr...</span>
          </div>
        )}

        <div ref={terminalEndRef} />
      </div>
    </div>
  );
};
