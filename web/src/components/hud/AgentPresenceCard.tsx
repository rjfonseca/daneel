import React, { useState } from 'react';
import { HudCard } from './HudCard';
import { NeonBadge } from './NeonBadge';
import { cn } from '@/lib/utils';

export interface ActiveAgentSession {
  sessionId: string;
  agent: string;
  event: string;
  cwd: string;
  timestamp: number;
}

interface AgentPresenceCardProps {
  sessions?: ActiveAgentSession[];
  onFocusProject?: (cwd: string) => void;
  className?: string;
}

export const AgentPresenceCard: React.FC<AgentPresenceCardProps> = ({
  sessions = [],
  onFocusProject,
  className = '',
}) => {
  const [collapsed, setCollapsed] = useState(false);

  return (
    <HudCard
      variant="cyan"
      className={cn('p-3 font-mono select-none w-72', className)}
    >
      <div
        className="flex items-center justify-between cursor-pointer border-b border-white/10 pb-2 mb-2"
        onClick={() => setCollapsed(!collapsed)}
      >
        <span className="font-bold text-xs uppercase tracking-wider text-cyan-300">
          Agent Ambient Telemetry
        </span>
        <NeonBadge accent={sessions.length > 0 ? 'emerald' : 'indigo'} size="sm">
          {sessions.length > 0 ? `${sessions.length} Active` : 'Idle'}
        </NeonBadge>
      </div>

      {!collapsed && (
        <div className="space-y-2 text-xs">
          {sessions.length === 0 ? (
            <div className="text-zinc-500 py-1 text-center italic">
              No active agents running in workspace.
            </div>
          ) : (
            sessions.map((sess) => (
              <div
                key={sess.sessionId}
                className="p-2 rounded-sm border border-zinc-800 bg-zinc-950/60 hover:border-cyan-500/40 transition-colors"
              >
                <div className="flex items-center justify-between gap-2 mb-1">
                  <span className="font-semibold text-cyan-400 capitalize">
                    {sess.agent || 'Agent'}
                  </span>
                  <NeonBadge accent="cyan" size="sm">
                    {sess.event || 'Active'}
                  </NeonBadge>
                </div>

                <div
                  className="text-zinc-400 truncate cursor-pointer hover:text-zinc-200"
                  onClick={() => onFocusProject?.(sess.cwd)}
                  title={`Focus project: ${sess.cwd}`}
                >
                  📁 {sess.cwd || 'Global Workspace'}
                </div>

                <div className="mt-1 flex items-center justify-between text-[10px] text-zinc-500">
                  <span className="truncate max-w-[120px]">{sess.sessionId}</span>
                  <span>{new Date(sess.timestamp).toLocaleTimeString()}</span>
                </div>
              </div>
            ))
          )}
        </div>
      )}
    </HudCard>
  );
};
