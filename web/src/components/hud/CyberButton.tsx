import React from 'react';
import { cn } from '@/lib/utils';

export interface CyberButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  active?: boolean;
  accent?: 'indigo' | 'cyan' | 'purple' | 'amber' | 'emerald' | 'rose';
  size?: 'sm' | 'md' | 'icon';
  variant?: 'hud' | 'ghost' | 'glow';
}

export const CyberButton = React.forwardRef<HTMLButtonElement, CyberButtonProps>(
  (
    {
      className,
      active = false,
      accent = 'indigo',
      size = 'md',
      variant = 'hud',
      children,
      ...props
    },
    ref
  ) => {
    const activeStyles: Record<NonNullable<CyberButtonProps['accent']>, string> = {
      indigo: 'bg-indigo-600/30 border-indigo-500/80 text-indigo-200 shadow-glow-indigo',
      cyan: 'bg-cyan-600/30 border-cyan-500/80 text-cyan-200 shadow-glow-cyan',
      purple: 'bg-purple-600/30 border-purple-500/80 text-purple-200 shadow-glow-purple',
      amber: 'bg-amber-600/30 border-amber-500/80 text-amber-200 shadow-glow-amber',
      emerald: 'bg-emerald-600/30 border-emerald-500/80 text-emerald-200 shadow-glow-emerald',
      rose: 'bg-rose-600/30 border-rose-500/80 text-rose-200 shadow-glow-rose',
    };

    const idleStyles =
      variant === 'ghost'
        ? 'bg-transparent border-transparent text-slate-400 hover:text-white hover:bg-white/5'
        : 'bg-surface/80 border-white/10 text-slate-300 hover:text-white hover:border-white/25 hover:bg-white/5';

    const sizeStyles: Record<NonNullable<CyberButtonProps['size']>, string> = {
      sm: 'px-2.5 py-1.5 text-xs',
      md: 'px-3.5 py-2 text-xs',
      icon: 'p-2',
    };

    return (
      <button
        ref={ref}
        className={cn(
          'flex items-center gap-2 rounded-hud-md border backdrop-blur-hud transition-all font-medium cursor-pointer select-none cosmic-focus-visible',
          sizeStyles[size],
          active ? activeStyles[accent] : idleStyles,
          className
        )}
        {...props}
      >
        {children}
      </button>
    );
  }
);

CyberButton.displayName = 'CyberButton';
