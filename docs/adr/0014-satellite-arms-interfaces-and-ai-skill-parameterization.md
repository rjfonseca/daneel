# ADR 0014: Satellite ARMS Specialization, Dynamic Skill Parameterization, and Agent Prompt Dispatch

* Status: accepted
* Date: 2026-09-16

## Context and Problem Statement

Daneel renders infrastructure satellites along the Seldon Perimeter ($r \approx 7000$ in graph space). These satellites represent diverse components governed by the **ARMS framework**:
1. **Applications**: Data ingestion harvesters (Jira, GitHub, GitLab, Google Drive).
2. **Routines**: Scheduled deterministic health cadences (tidy, doctor, index, guard, time logging).
3. **Models & Skills**: Autonomous AI agent capability protocols residing in `.agents/skills/` (e.g., `meeting-preflight`, `vault-dream-cycle`, `staff-avd-compiler`).
4. **Systems**: Core second brain daemon and indexing services.

Previously, the Action Deck and drawer inspector attempted to invoke all entities as raw CLI subcommands (`daneel <name>`). For AI skills such as `meeting-preflight`, this resulted in failure (`Error: unknown command "meeting-preflight" for "daneel"`). AI skills are prompt protocols, not binary CLI subcommands; they require dynamic contextual parameters (e.g., meeting attendees, topic, date, reference notes) and orchestration via an AI agent session (Claude Code, Antigravity CLI, or Cursor). Furthermore, harvesters required targeted platform synchronization rather than indiscriminate global harvesting.

## Decision Drivers

* Strict separation of concerns between deterministic CLI subcommands and parameter-driven AI skills.
* Provide dynamic parameter input forms for AI skills in both the holographic inspector (`ArmsInspectorCard`) and full modal cockpit (`CockpitModal`).
* Support dual action paths:
  1. **Direct Agent Execution**: Headless execution via configured agent CLI (`agy` / `claude` with `-p "<prompt>"`) streaming real-time stdout/stderr via ConnectRPC.
  2. **1-Click Prompt Copy**: Instant hydration of the prompt protocol for external agent sessions.
* Open-source generalization: parameter schemas and skill discovery dynamically resolve from `.agents/skills/` and `.daneel.yaml` without hardcoded company or user strings.
* Strict English-only UI invariant across all form fields, labels, tooltips, and badges.

## Considered Options

* **Option 1: Static Parameterless Dispatch**: Run a generic prompt without user parameters.
* **Option 2: Terminal Interactive Prompting**: Require the user to open a terminal CLI to answer questions interactively.
* **Option 3: Dynamic ARMS Specialization & Hydrated Prompt Dispatch**:
  - Enrich satellite resolution with `armsLayer` (`'application' | 'routine' | 'skill' | 'system'`).
  - Introduce `getSkillParameters(skillName)` and `generateSkillPrompt(skillName, params, title)`.
  - Provide a reusable, cyber-styled `SkillParameterDeck` component with input fields, prompt preview toggle, 1-click clipboard copying, and agent dispatch.
  - Implement `executeAISkill` in `cmd/serve.go` with ConnectRPC streaming.

## Decision Outcome

Chosen option: **Option 3: Dynamic ARMS Specialization & Hydrated Prompt Dispatch**.

### Architectural Layers

1. **Domain & Resolution Layer (`stationResolver.ts`)**:
   - Classifies stations into `ArmsLayer` (`application`, `routine`, `skill`, `system`).
   - Defines `SkillParameterDef` schemas for known skills (`meeting-preflight`, `vault-dream-cycle`, `staff-avd-compiler`) and provides generic fallbacks for user-defined skills in `.agents/skills/`.
   - Implements `generateSkillPrompt` to hydrate Markdown prompts ready for agent consumption.

2. **Presentation Layer (`SkillParameterDeck.tsx`, `ArmsInspectorCard.tsx`, `CockpitModal.tsx`)**:
   - `SkillParameterDeck`: Renders dynamic inputs (text, date, textarea, select), prompt protocol preview, copy-to-clipboard button, and dispatch trigger.
   - `ArmsInspectorCard`: In the Hologram Spec tab, displays specialized control decks based on `armsLayer`:
     - Applications: Platform badge, CLI subcommand, "Sync Platform", and "Sync All".
     - Routines: Recurrence cadence, CLI subcommand, and "Run Routine".
     - Skills: Embedded `SkillParameterDeck` alongside specification docs.
   - `CockpitModal`: Embeds `SkillParameterDeck` in the Action Deck pane whenever an AI skill is selected.
   - `CyberTerminal`: Adds a "Copy Output" button in the toolbar with visual confirmation.

3. **Backend Service Layer (`cmd/serve.go`)**:
   - ConnectRPC `ExecuteRoutine` identifies AI skills via `isAISkill(routine)`.
   - `executeAISkill` streams the hydrated prompt protocol, inspects `.daneel.yaml` chat profile (`agy` / `claude`), executes the agent in non-interactive print mode, and pipes real-time streaming output to the web terminal.
   - If the agent binary is not in PATH or running headless, outputs the complete formatted prompt and instructions with exit code 0.

### Positive Consequences

* AI skills no longer trigger "unknown command" errors.
* Users can customize meeting preflight briefings directly from the visualizer with attendees, agenda topics, and context.
* Zero external tool lock-in: users can dispatch via daemon or copy the prompt into any external agent with a single click.
* Full test coverage across unit test suites (`stationResolver.test.ts`, `SkillParameterDeck.test.tsx`, `ArmsInspectorCard.test.tsx`, `CockpitModal.test.tsx`, and Go backend tests).
