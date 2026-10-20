import React from 'react';
import { cn } from '@/lib/utils';

export interface HotkeyBadgeProps extends React.HTMLAttributes<HTMLElement> {
  keys: string[];
}

export const HotkeyBadge: React.FC<HotkeyBadgeProps> = ({ keys, className, ...props }) => (
  <kbd
    className={cn(
      'inline-flex items-center gap-0.5 px-1.5 py-0.5 text-[9px] font-mono font-bold text-slate-300 bg-surface-muted/90 rounded-sm border border-white/10 shadow-xs select-none',
      className
    )}
    {...props}
  >
    {keys.map((k, i) => (
      <span key={i}>{k}</span>
    ))}
  </kbd>
);
