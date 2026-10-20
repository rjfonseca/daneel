# ADR 0023: Staff Telemetry Temporal Breakdown, Rolling Windows, and Leverage Matrix

* Status: accepted
* Date: 2026-09-29

## Context and Problem Statement

Daneel provides daily log time tracking and taxonomy extraction (`internal/ops/tracking/time.go`), exposing staff engineering allocation telemetry via CLI (`daneel log`) and ConnectRPC (`GetStaffTelemetry`).

However, the previous implementation suffered from several architectural and analytical limitations:
1. **Flat Aggregation**: `GetStaffTelemetry` only accepted `days: int` (defaulting to 7) and returned a flat accumulated sum, obscuring trends, trajectory shifts, and comparative evolution between weeks or months.
2. **Focus Drift Masking**: Aggregated quarterly numbers masked critical weekly variations where an engineer was stuck in firefighting mode (e.g. 54% local impact) versus high technical multiplication periods.
3. **Omission of [D] Low Impact / Bureaucracy**: While `DailyLog` parsed `tempo-D_LowImpact`, the Protobuf contract omitted the field, hiding bureaucratic friction from audit.
4. **Absence of Quantitative Technical Leverage Metric**: No automated calculation existed for the technical leadership leverage rate:
   $$\text{Staff Leverage Rate} = \frac{\text{Hours } [A] + [C]}{\text{Total Hours}}$$
5. **Ergonomic Deficiency**: The CLI only listed raw daily entries or individual logs, with no temporal comparison views (`--weeks`, `--monthly`).

## Decision Drivers

* **Trajectory & Leverage Visibility**: Immediate insight into whether an engineer's time is multiplying technical impact ($[A] + [C]$) or diverging into reactive local firefighting ($[B]$).
* **Zero-Breaking-Change Backward Compatibility**: Expand `api/v1/graph.proto` while keeping existing fields and semantics intact.
* **Deterministic CGO-Free Aggregation**: Pure Go calendar-based grouping (ISO 8601 weeks and calendar months) without external database or network dependencies.
* **Unified Dual Ergonomics (CLI + Web Cockpit)**: High-resolution tabular output with Lipgloss in terminal, plus interactive visual deck in Web Cockpit.
* **Open-Source English Invariant**: Maintain English-first presentation across all metrics, badges, and diagnostic labels.

## Considered Options

* **Option 1: Ad-hoc Client-Side Grouping**: Fetch raw daily logs on the frontend or CLI and compute weekly buckets client-side.
  * *Disadvantages*: Duplicates date math, ISO week bounds logic, and taxonomy classification across Go CLI and React TypeScript frontend.
* **Option 2: Time-Series Database (TSDB) Integration**: Introduce Prometheus or InfluxDB storage for daily telemetry metrics.
  * *Disadvantages*: Massive external dependency and overhead, violating Daneel's local-first single-binary invariant.
* **Option 3: Deterministic In-Core Temporal Aggregation with Protobuf Expansion**:
  * Implement `PeriodSummary`, `AggregateByWeek`, `AggregateByMonth`, and `ClassifyPeriodDiagnosis` in `internal/ops/tracking`.
  * Expand `api/v1/graph.proto` with `rolling_weeks`, `historical_months`, `low_impact_d_hours`, `leverage_rate`, and `StaffPeriodSummary`.
  * Enhance `daneel log` with `--weeks` and `--monthly` flags featuring styled Lipgloss tables.
  * Upgrade Web Cockpit (`CockpitModal.tsx`) with the Staff Leverage Index badge, 4-week comparative cards, and monthly history table.

## Decision Outcome

Chosen option: **Option 3: Deterministic In-Core Temporal Aggregation with Protobuf Expansion**.

### Architecture & Data Flow

```
   ┌─────────────────────────────────────────────────────────┐
   │ Daily Notes: daily/YYYY-MM-DD.md (Frontmatter Telemetry)│
   └────────────────────────────┬────────────────────────────┘
                                │
                                ▼
   ┌─────────────────────────────────────────────────────────┐
   │ internal/ops/tracking/time.go                           │
   │ • CalculateLeverageRate(tempoA, tempoC, total)          │
   │ • ClassifyPeriodDiagnosis(PeriodSummary)                │
   │ • AggregateByWeek(vaultRoot, numWeeks) -> ISO Weeks     │
   │ • AggregateByMonth(vaultRoot, numMonths) -> Cal Months  │
   └────────────────────────────┬────────────────────────────┘
                                │
               ┌────────────────┴────────────────┐
               ▼                                 ▼
   ┌───────────────────────────────┐ ┌───────────────────────┐
   │ ConnectRPC / cmd/serve.go     │ │ CLI: cmd/log.go       │
   │ • GetStaffTelemetry handler   │ │ • daneel log --weeks 4│
   │ • Rolling 4-week window       │ │ • daneel log --monthly│
   │ • 6-month historical trend    │ │ • Lipgloss ASCII table│
   └───────────────┬───────────────┘ └───────────────────────┘
                   ▼
   ┌───────────────────────────────┐
   │ Web Cockpit (CockpitModal.tsx)│
   │ • Leverage Rate Badge         │
   │ • 5-Archetype [A]-[E] Grid    │
   │ • Rolling 4-Weeks Matrix      │
   │ • Historical Monthly Table    │
   └───────────────────────────────┘
```

### Technical Specifications

1. **Staff Leverage Index**:
   $$\text{Leverage Rate} = \frac{\text{TempoA (Strategic)} + \text{TempoC (Glue Work)}}{\text{Total Hours}}$$
   - $\ge 50\%$: High Leverage (Strategic Multiplication)
   - $35 - 49\%$: Balanced Operational Focus
   - $< 35\%$: High Reactivity / Local Firefighting

2. **Automated Diagnostic Heuristics**:
   - `🚀 Strategic Multiplication`: Leverage Rate $\ge 50\%$ and Strategic A $\ge 35\%$.
   - `🔥 Local Reactivity & Firefighting`: Local Impact B $\ge 45\%$.
   - `🤝 High Glue & Alignment Load`: Glue C $\ge 35\%$ or Overhead E $\ge 25\%$.
   - `⚖️ Balanced Operational Focus`: Balanced baseline distribution.

3. **Protobuf Evolution (`api/v1/graph.proto`)**:
   - Added `rolling_weeks` and `historical_months` to `GetStaffTelemetryRequest`.
   - Added `StaffPeriodSummary` definition.
   - Added `low_impact_d_hours`, `leverage_rate`, `rolling_weeks`, and `monthly_history` to `GetStaffTelemetryResponse`.

4. **CLI Flags (`cmd/log.go`)**:
   - `daneel log --weeks 4`: Render rolling weekly comparative table.
   - `daneel log --monthly`: Render historical monthly trajectory table.
   - `--json`: Output structured JSON representations for agentic consumption.

## Consequences

### Positive
* Transparent, quantifiable staff engineering leverage tracking directly in local Markdown daily logs.
* Early detection of operational firefighting drift across rolling weekly windows.
* Seamless compatibility with existing ConnectRPC and CLI workflows.
* Zero external infrastructure required.

### Negative / Trade-offs
* Vault notes must follow standard OKF frontmatter (`tempo-A_Estrategico`, `tempo-B_ImpactoLocal`, etc.).
