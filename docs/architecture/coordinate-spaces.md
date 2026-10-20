# Coordinate Spaces & Camera Math

The Daneel Galaxy visualizer operates across three strictly separated coordinate systems. Understanding these transformations is vital to avoid visual drift and incorrect camera framing.

## 1. The Three Coordinate Spaces

```mermaid
graph LR
    Graphology[1. Graphology Space: Raw node coords x, y in [-8000, 8000]] -->|Sigma Camera + Graph Extents| Framed[2. Framed Graph Space: Normalized u, v in [0, 1]]
    Framed -->|Viewport Dimensions w, h| Screen[3. Screen Space: DOM Pixels in [0, w] x [0, h]]
```

1. **Graphology Space**: Raw graph coordinates produced by ForceAtlas2. Core notes cluster in $[-2000, 2000]$; outer orbital satellites sit at fixed radius $r \approx 7000$.
2. **Sigma Framed Graph Space**: Internal normalized coordinates in `[0, 1]` calculated by Sigma (`sigma.viewportToFramedGraph` and `sigma.framedGraphToViewport`).
3. **Viewport Screen Space**: Physical CSS pixels in the browser DOM (`[0, width] x [0, height]`).

## 2. Camera Zoom Semantics (`camera.ratio`)

In Sigma.js, `camera.ratio` is **inversely proportional** to zoom magnification:
- `ratio = 1.0`: Full panoramic galaxy overview.
- `ratio = 0.20 - 0.40`: Medium cluster and nebula framing.
- `ratio = 0.02 - 0.08`: Deep inspection of single stars and note reading.

> [!CAUTION]
> **Never hard-clamp `camera.ratio` for clusters**: Clamping with `Math.min(0.08, ...)` forces 80% of cluster stars out of view. Always use `calculateClusterFraming(bbox, viewport)` which guarantees the cluster occupies between 65% and 90% of the visible viewport.

## 3. Lateral Drawer Compensation

When the note inspection drawer (width 440px) is opened on the right side:
- The user's visual center shifts from `width / 2` to `openCenterX = (width - drawerWidth) / 2`.
- Any focused entity (satellite or note) must apply an X-axis compensation in framed space:

$$dx = \left(\frac{\text{width}}{2} - \text{openCenterX}\right) \times \frac{\text{camera.ratio}}{\min(\text{width}, \text{height})}$$

In screen space, the target node must project to `(width - drawerWidth) / 2` within a tolerance of $\pm 25$ pixels.

## 4. Intact Anchor Principle

- **Perimeter Satellites ($r \approx 7000$)**: Satellites reside in the outer ring. A satellite must NEVER be centered by averaging with its connected neighbors in the galactic core; doing so places the camera in empty black space. Satellites are framed with 100% focus on their own coordinates (`ratio: 0.08`).
- **Core Notes**: When calculating framing for a note and its neighbors, filter out neighbors with normalized distance $> 0.18$ to prevent long cross-galaxy edges from corrupting local framing.
