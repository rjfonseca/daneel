import React from 'react';
import { cn } from '@/lib/utils';

export interface NeonBadgeProps extends React.HTMLAttributes<HTMLSpanElement> {
  accent?: 'cyan' | 'indigo' | 'purple' | 'amber' | 'emerald' | 'rose' | 'bronze';
  ping?: boolean;
  size?: 'sm' | 'md';
}

export const NeonBadge: React.FC<NeonBadgeProps> = ({
  accent = 'indigo',
  ping = false,
  size = 'md',
  className,
  children,
  ...props
}) => {
  const accentStyles: Record<NonNullable<NeonBadgeProps['accent']>, string> = {
    cyan: 'bg-cyan-500/15 text-cyan-300 border-cyan-500/30',
    indigo: 'bg-indigo-500/15 text-indigo-300 border-indigo-500/30',
    purple: 'bg-purple-500/15 text-purple-300 border-purple-500/30',
    amber: 'bg-amber-500/15 text-amber-300 border-amber-500/30',
    emerald: 'bg-emerald-500/15 text-emerald-300 border-emerald-500/30',
    rose: 'bg-rose-500/15 text-rose-300 border-rose-500/30',
    bronze: 'bg-amber-600/15 text-amber-400 border-amber-600/30',
  };

  const pingColors: Record<NonNullable<NeonBadgeProps['accent']>, string> = {
    cyan: 'bg-cyan-400',
    indigo: 'bg-indigo-400',
    purple: 'bg-purple-400',
    amber: 'bg-amber-400',
    emerald: 'bg-emerald-400',
    rose: 'bg-rose-400',
    bronze: 'bg-amber-500',
  };

  const sizeStyles = size === 'sm' ? 'px-2 py-0.5 text-[9px]' : 'px-2.5 py-1 text-[10px]';

  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 font-mono font-bold uppercase tracking-wider rounded-md border',
        sizeStyles,
        accentStyles[accent],
        className
      )}
      {...props}
    >
      {ping && (
        <span className="relative flex h-2 w-2">
          <span
            className={cn(
              'animate-ping absolute inline-flex h-full w-full rounded-full opacity-75 motion-reduce:hidden',
              pingColors[accent]
            )}
          />
          <span className={cn('relative inline-flex rounded-full h-2 w-2', pingColors[accent])} />
        </span>
      )}
      {children}
    </span>
  );
};
