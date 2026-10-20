import React from 'react';
import { cn } from '@/lib/utils';

export interface HudCardProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: 'default' | 'cyan' | 'indigo' | 'purple' | 'amber' | 'emerald' | 'rose';
  elevation?: 'surface' | 'elevated' | 'muted';
  techCorners?: boolean;
}

export const HudCard = React.forwardRef<HTMLDivElement, HudCardProps>(
  (
    {
      className,
      variant = 'default',
      elevation = 'surface',
      techCorners = false,
      children,
      ...props
    },
    ref
  ) => {
    const variantStyles: Record<NonNullable<HudCardProps['variant']>, string> = {
      default: 'border-white/10 hover:border-white/20',
      cyan: 'border-cyan-500/30 hover:border-cyan-500/50 shadow-glow-cyan',
      indigo: 'border-indigo-500/30 hover:border-indigo-500/50 shadow-glow-indigo',
      purple: 'border-purple-500/30 hover:border-purple-500/50 shadow-glow-purple',
      amber: 'border-amber-500/30 hover:border-amber-500/50 shadow-glow-amber',
      emerald: 'border-emerald-500/30 hover:border-emerald-500/50 shadow-glow-emerald',
      rose: 'border-rose-500/30 hover:border-rose-500/50 shadow-glow-rose',
    };

    const elevationStyles: Record<NonNullable<HudCardProps['elevation']>, string> = {
      surface: 'bg-surface/85 backdrop-blur-hud',
      elevated: 'bg-surface-elevated/95 backdrop-blur-deep',
      muted: 'bg-surface-muted/90 backdrop-blur-hud',
    };

    return (
      <div
        ref={ref}
        className={cn(
          'rounded-hud-lg border transition-all duration-200 shadow-hud-panel',
          elevationStyles[elevation],
          variantStyles[variant],
          techCorners && 'tech-corners',
          className
        )}
        {...props}
      >
        {children}
      </div>
    );
  }
);

HudCard.displayName = 'HudCard';
