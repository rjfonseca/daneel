/**
 * Cosmic HUD Design System Tokens (Single Source of Truth)
 *
 * Shared between:
 * 1. WebGL shaders and HTML5 Canvas 2D renderers (canvasCustomLabels, canvasMath, orbitalRings)
 * 2. Tailwind CSS theme extensions (tailwind.config.js)
 * 3. React DOM Cosmic HUD components (web/src/components/hud/*)
 */

export const cosmicTokens = {
  colors: {
    // Spatial Void & Glass Surfaces
    void: '#050508',               // Level 0: Pure space canvas background
    surface: '#0a0d18',            // Level 1: Base HUD glass surface (85% alpha in DOM)
    surfaceElevated: '#0f1424',   // Level 2: Popovers, dialogs, sidebars (95% alpha)
    surfaceMuted: '#141a2e',      // Level 3: Inset areas, badges, kbd backgrounds

    // Standardized HUD Borders
    border: {
      subtle: 'rgba(255, 255, 255, 0.08)',
      medium: 'rgba(255, 255, 255, 0.15)',
      cyan: 'rgba(56, 189, 248, 0.35)',
      indigo: 'rgba(99, 102, 241, 0.35)',
      purple: 'rgba(168, 85, 247, 0.35)',
      amber: 'rgba(245, 158, 11, 0.35)',
      emerald: 'rgba(16, 185, 129, 0.35)',
      rose: 'rgba(251, 113, 133, 0.35)',
    },

    // Spectral & Sector Accents (Semantic Roles)
    starlight: {
      DEFAULT: '#38bdf8',          // Cyan: Quasars, Temporal HUD, Primary Telemetry
      bright: '#67e8f9',
      glow: 'rgba(56, 189, 248, 0.40)',
      subtle: 'rgba(56, 189, 248, 0.12)',
    },
    quantum: {
      DEFAULT: '#6366f1',          // Indigo: Gravity links, Constellation focus, Core knowledge
      bright: '#818cf8',
      glow: 'rgba(99, 102, 241, 0.40)',
      subtle: 'rgba(99, 102, 241, 0.12)',
    },
    foundation: {
      DEFAULT: '#a855f7',          // Purple: First Foundation (Skills), Orbital perimeter
      bright: '#c084fc',
      glow: 'rgba(168, 85, 247, 0.40)',
      subtle: 'rgba(168, 85, 247, 0.12)',
    },
    cadence: {
      DEFAULT: '#f59e0b',          // Amber: Second Foundation (Routines, Automation)
      bright: '#fbbf24',
      glow: 'rgba(245, 158, 11, 0.40)',
      subtle: 'rgba(245, 158, 11, 0.12)',
    },
    biolum: {
      DEFAULT: '#10b981',          // Emerald: Harvesters, Status online, Health
      bright: '#34d399',
      glow: 'rgba(16, 185, 129, 0.40)',
      subtle: 'rgba(16, 185, 129, 0.12)',
    },
    supernova: {
      DEFAULT: '#fb7185',          // Rose: Red dwarfs (WCAG AA/AAA), Alerts, Destructive actions
      bright: '#fda4af',
      glow: 'rgba(251, 113, 133, 0.40)',
      subtle: 'rgba(251, 113, 133, 0.12)',
    },
    bronze: {
      DEFAULT: '#d97706',          // Warm Bronze: Brown dwarfs (WCAG AA), Background dust
      bright: '#f59e0b',
      glow: 'rgba(217, 119, 6, 0.40)',
      subtle: 'rgba(217, 119, 6, 0.15)',
    },
  },

  shadows: {
    hudPanel: '0 8px 32px rgba(0, 0, 0, 0.60), 0 0 15px rgba(0, 0, 0, 0.40)',
    hudDeep: '0 0 60px rgba(0, 0, 0, 0.90)',
    glowCyan: '0 0 25px rgba(56, 189, 248, 0.25)',
    glowIndigo: '0 0 30px rgba(99, 102, 241, 0.25)',
    glowPurple: '0 0 25px rgba(168, 85, 247, 0.25)',
    glowAmber: '0 0 25px rgba(245, 158, 11, 0.25)',
    glowEmerald: '0 0 25px rgba(16, 185, 129, 0.25)',
    glowRose: '0 0 25px rgba(251, 113, 133, 0.25)',
  },

  radii: {
    xs: '4px',
    sm: '8px',
    md: '12px',
    lg: '16px',
    xl: '20px',
    pill: '9999px',
  },

  blur: {
    hud: '16px',
    deep: '24px',
  },

  typography: {
    fontSans: "'Geist Variable', sans-serif",
    fontMono: "'JetBrains Mono', monospace",
  },
} as const;

export type CosmicTokens = typeof cosmicTokens;
