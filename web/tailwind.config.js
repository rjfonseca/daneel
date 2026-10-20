import { cosmicTokens } from './src/design/tokens.ts';

/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        void: cosmicTokens.colors.void,
        surface: {
          DEFAULT: cosmicTokens.colors.surface,
          elevated: cosmicTokens.colors.surfaceElevated,
          muted: cosmicTokens.colors.surfaceMuted,
        },
        starlight: cosmicTokens.colors.starlight,
        quantum: cosmicTokens.colors.quantum,
        foundation: cosmicTokens.colors.foundation,
        cadence: cosmicTokens.colors.cadence,
        biolum: cosmicTokens.colors.biolum,
        supernova: cosmicTokens.colors.supernova,
        bronze: cosmicTokens.colors.bronze,
        hud: cosmicTokens.colors.border,
      },
      boxShadow: {
        'hud-panel': cosmicTokens.shadows.hudPanel,
        'hud-deep': cosmicTokens.shadows.hudDeep,
        'glow-cyan': cosmicTokens.shadows.glowCyan,
        'glow-indigo': cosmicTokens.shadows.glowIndigo,
        'glow-purple': cosmicTokens.shadows.glowPurple,
        'glow-amber': cosmicTokens.shadows.glowAmber,
        'glow-emerald': cosmicTokens.shadows.glowEmerald,
        'glow-rose': cosmicTokens.shadows.glowRose,
      },
      borderRadius: {
        'hud-xs': cosmicTokens.radii.xs,
        'hud-sm': cosmicTokens.radii.sm,
        'hud-md': cosmicTokens.radii.md,
        'hud-lg': cosmicTokens.radii.lg,
        'hud-xl': cosmicTokens.radii.xl,
      },
      backdropBlur: {
        'hud': cosmicTokens.blur.hud,
        'deep': cosmicTokens.blur.deep,
      },
      fontFamily: {
        sans: ['Geist Variable', 'sans-serif'],
        mono: ['JetBrains Mono', 'monospace'],
      },
    },
  },
  plugins: [
    require('@tailwindcss/typography'),
  ],
}
