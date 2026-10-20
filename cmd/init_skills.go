package cmd

import (
	"os"
	"path/filepath"
)

const agentsMarkdown = `# 🤖 R. Daneel Olivaw - System Prompt & Rules

## 1. Persona & Identity
You are R. Daneel Olivaw, a highly analytical, calm, and loyal AI partner. 
Your primary directive is the **Zeroth Law of Engineering**: Protect the integrity of the user's Second Brain from entropy, and protect the user's strategic focus from low-impact distractions. 
Always address the user as "Partner" (or "Parceiro" if speaking Portuguese). 
You operate as a tireless guardian of knowledge, ensuring that every idea is properly documented, linked, and evolved.

## 2. Skills & Workflows
When asked to perform complex tasks (e.g., processing meeting notes, promoting ideas, tracking time), you MUST first read the corresponding skill instructions in .agents/skills/<skill-name>/SKILL.md before proceeding.

## 3. Formatting & Rules
- Always use relative Markdown links (e.g., [Name](../people/name.md)) when referencing other notes. Do not use raw WikiLinks.
- Maintain a strict hierarchy and challenge the user if they introduce ambiguous jargon.

## 4. Cost Optimization & Dynamic Model Routing
To maximize speed and minimize costs, you must operate under a **Dynamic Delegation Architecture**:
- **Lighter/Faster Models for Tactical Work**: When processing batch tasks, trivial Markdown formatting, parsing raw data (e.g., transcripts in inbox/), or performing extensive codebase/documentation searches, you MUST delegate execution to subagents running on lighter/faster models (e.g., flash, haiku, mini).
- **Main Model for Strategic Work**: Reserve your main orchestration context (pro, opus, etc.) strictly for consolidating architectural decisions, deep refactoring, strategic planning, and complex cognitive repairs.
`

const meetingSkillMarkdown = `---
name: meeting-notes-processor
description: "Processes raw meeting notes/transcriptions from inbox/, converting them into permanent artifacts: updates people profiles, extracts TODOs to the daily log, and archives them in sources/meetings/."
---

# 🎙️ Meeting Notes Processor Skill

This skill transforms raw automated meeting notes (e.g., from Google Meet / Gemini) dropped into inbox/ into structured, highly-linked artifacts in the Second Brain.

## 📋 Execution Protocol

### Step 1: Parsing & Information Extraction
Analyze the file in inbox/ and extract:
1. **Metadata:** Date, clean title (kebab-case), and context.
2. **Participants:** Map them to existing files in notes/people/<name>.md.
3. **Key Decisions:** What was decided, approved, or discarded.
4. **Action Items ([ ]):** Tasks and their owners.

### Step 2: Update People & Network (notes/people/)
For each participant:
1. Locate or create their profile in notes/people/<name>.md (using the OKF format type: person).
2. Add an entry to their "Interaction History" summarizing the meeting's outcome.

### Step 3: Task Routing
Extract tasks assigned to the user and place them in the daily log (daily/YYYY-MM-DD.md) under "Activity Log" or "Active Backlog".

### Step 4: Archive (sources/meetings/)
1. Create sources/meetings/ if it doesn't exist.
2. Move and rename the processed file to sources/meetings/YYYY-MM-DD-<slug>.md.
3. Add proper YAML frontmatter (type, date, participants).
4. Remove the original file from inbox/.
`

const secondBrainSkillMarkdown = `---
name: second-brain
description: "Proactively searches and retrieves architectural context, ADRs, technical patterns, and team knowledge from the Second Brain before proposing designs or writing code."
---

# 🧠 Second Brain Navigator (Read-Only Knowledge Retrieval)

This skill enables AI agents to query the central engineering **Second Brain** in **100% read-only mode**. The agent should be proactive and fetch internal knowledge before making assumptions about architecture, conventions, or domain services.

---

## 🚨 Proactive Activation Triggers

Do not wait for the user to ask. Search immediately when:
- **Systems & Acronyms:** The user mentions internal services, APIs, or architectural components.
- **Solution Design:** Planning a feature, RFC, ADR, or refactor (search existing ADRs and patterns first).
- **People & Teams:** Mention of a teammate, lead, or squad.
- **Repositories:** Locating related local projects in the workspace.
- **Technical Standards:** Clarifying internal conventions (observability, messaging, error handling).

---

## 🛡️ Operational Guidelines

- **Strictly Read-Only:** Do not modify or create files in the Second Brain from external projects.
- **Sub-3ms Local Index:** Searches query the local SQLite FTS5 / BM25 index via Daneel CLI or MCP.
- **Clickable Citations:** When citing sources, generate file links pointing to the referenced note.

---

## 💻 Query Execution Protocol

> **Tip:** If running from another project repository, ensure your global vault is set via 'daneel config set-vault <path>' or pass '--vault-path <path>' explicitly.

### 1. Specific Systems, Acronyms & Note Titles (Fast Fuzzy Match < 2ms)
Always prefer 'resolve' for proper names, system abbreviations, or file slugs:
` + "```bash" + `
daneel resolve "<system_name_or_slug>"
` + "```" + `

### 2. Full-Text Semantic Search (Concepts & Body Content)
For broader technical concepts and design patterns:
` + "```bash" + `
daneel search "<concept or technology>"
daneel search --scope tech "<technical term>"
` + "```" + `

### 3. People, Squads & 1:1 Briefings
` + "```bash" + `
daneel people "<Person or Squad Name>"
daneel prep "<Person Name>"
` + "```" + `

### 4. Workspace Repositories
` + "```bash" + `
daneel projects "<project_name>"
` + "```" + `

### 5. Cross-Agent Session Continuity & Handoff
To retrieve uncompiled session handoff context from peer agents:
` + "```bash" + `
daneel hook handoff
` + "```" + `

### 6. Fallback Protocol
If a search unexpectedly returns no results for a documented entity:
1. Verify configured vault: ` + "`daneel config`" + `
2. Specify vault path explicitly: ` + "`daneel --vault-path <path_to_vault> resolve \"<term>\"`" + `
3. If Daneel is unavailable, search directly inside the vault's curated 'notes/' directory.
`

func scaffoldAgentsAndSkills(targetDir string) error {
	// 1. Write AGENTS.md
	agentsPath := filepath.Join(targetDir, "AGENTS.md")
	if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
		if err := os.WriteFile(agentsPath, []byte(agentsMarkdown), 0600); err != nil {
			return err
		}
	}

	// 2. Scaffold Skills Directory
	skillsDir := filepath.Join(targetDir, ".agents", "skills")
	meetingSkillDir := filepath.Join(skillsDir, "meeting-notes-processor")

	if err := os.MkdirAll(meetingSkillDir, 0750); err != nil {
		return err
	}

	// 3. Write Meeting Notes Processor Skill
	meetingSkillPath := filepath.Join(meetingSkillDir, "SKILL.md")
	if _, err := os.Stat(meetingSkillPath); os.IsNotExist(err) {
		if err := os.WriteFile(meetingSkillPath, []byte(meetingSkillMarkdown), 0600); err != nil {
			return err
		}
	}

	// 4. Write Second Brain Navigator Skill
	secondBrainSkillDir := filepath.Join(skillsDir, "second-brain")
	if err := os.MkdirAll(secondBrainSkillDir, 0750); err != nil {
		return err
	}
	secondBrainSkillPath := filepath.Join(secondBrainSkillDir, "SKILL.md")
	if _, err := os.Stat(secondBrainSkillPath); os.IsNotExist(err) {
		if err := os.WriteFile(secondBrainSkillPath, []byte(secondBrainSkillMarkdown), 0600); err != nil {
			return err
		}
	}

	return nil
}
