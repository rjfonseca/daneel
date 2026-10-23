# Palette Journal

## 2025-05-18 - Resizable panel separator keyboard shortcuts (Home/End)
**Learning:** Resizable handles with `role="separator"` need `Home` and `End` keys bound to `aria-valuemin` and `aria-valuemax` to satisfy standard WAI-ARIA slider/separator keyboard accessibility patterns.
**Action:** When adding or enhancing resize handles, include `Home` (set to min) and `End` (set to max) key handlers alongside `ArrowLeft`/`ArrowRight` or `ArrowUp`/`ArrowDown`.
