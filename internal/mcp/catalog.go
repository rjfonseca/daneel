//nolint:revive // package comment not required
package mcp

import (
	"sort"
	"strings"

	"github.com/rjfonseca/daneel/internal/toon"
)

// ToolDescriptor provides metadata, category classification, and keywords for an MCP tool.
type ToolDescriptor struct {
	Name        string      `json:"name"`
	Category    string      `json:"category"`
	Description string      `json:"description"`
	Keywords    []string    `json:"keywords"`
	InputSchema interface{} `json:"inputSchema"`
}

// ToolCatalog indexes and retrieves tools dynamically (Tool-as-RAG).
type ToolCatalog struct {
	tools []ToolDescriptor
}

// NewToolCatalog constructs the default Daneel MCP tool catalog with metadata taxonomy.
func NewToolCatalog() *ToolCatalog {
	catalog := &ToolCatalog{}
	catalog.registerDefaultTools()
	return catalog
}

// Register registers a tool descriptor into the catalog.
func (c *ToolCatalog) Register(tool ToolDescriptor) {
	c.tools = append(c.tools, tool)
}

// All returns all registered tools.
func (c *ToolCatalog) All() []ToolDescriptor {
	return c.tools
}

// Search performs BM25-weighted token retrieval across tool names, keywords, and descriptions.
func (c *ToolCatalog) Search(query string, category string, limit int) []ToolDescriptor {
	if limit <= 0 {
		limit = 5
	}

	queryTokens := tokenize(query)
	catFilter := strings.ToLower(strings.TrimSpace(category))

	type scoredTool struct {
		tool  ToolDescriptor
		score float64
	}

	var scored []scoredTool

	for _, tool := range c.tools {
		if catFilter != "" && strings.ToLower(tool.Category) != catFilter {
			continue
		}

		score := 0.0
		toolNameTokens := tokenize(tool.Name)
		toolDescTokens := tokenize(tool.Description)

		for _, q := range queryTokens {
			// Name match (highest weight)
			if strings.EqualFold(tool.Name, q) {
				score += 10.0
			} else {
				for _, nt := range toolNameTokens {
					if strings.EqualFold(nt, q) {
						score += 4.0
					}
				}
			}

			// Keyword match (high weight)
			for _, kw := range tool.Keywords {
				if strings.EqualFold(kw, q) {
					score += 3.0
				} else if strings.Contains(strings.ToLower(kw), q) {
					score += 1.5
				}
			}

			// Description match
			for _, dt := range toolDescTokens {
				if strings.EqualFold(dt, q) {
					score += 1.0
				}
			}
		}

		if score > 0 || len(queryTokens) == 0 {
			scored = append(scored, scoredTool{tool: tool, score: score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	if len(scored) > limit {
		scored = scored[:limit]
	}

	results := make([]ToolDescriptor, len(scored))
	for i, st := range scored {
		results[i] = st.tool
	}
	return results
}

// FormatTOON formats tool descriptors into compact TOON tabular notation.
func (c *ToolCatalog) FormatTOON(tools []ToolDescriptor) string {
	headers := []string{"name", "category", "description"}
	rows := make([][]string, len(tools))
	for i, t := range tools {
		rows[i] = []string{t.Name, t.Category, t.Description}
	}
	return toon.EncodeTabular("tools", headers, rows)
}

func tokenize(s string) []string {
	var tokens []string
	clean := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return ' '
	}, s)

	for _, field := range strings.Fields(clean) {
		t := strings.ToLower(strings.TrimSpace(field))
		if len(t) > 1 {
			tokens = append(tokens, t)
		}
	}
	return tokens
}

func (c *ToolCatalog) registerDefaultTools() {
	formatProp := map[string]interface{}{
		"type":        "string",
		"enum":        []string{"toon", "markdown"},
		"description": "Output serialization format: 'toon' (compact Token-Oriented Object Notation, default) or 'markdown'.",
	}

	c.Register(ToolDescriptor{
		Name:        "resolve_tools",
		Category:    "meta",
		Description: "Dynamically discovers and retrieves matching MCP tool definitions based on intent or query (Tool-as-RAG).",
		Keywords:    []string{"tools", "discover", "search", "schema", "meta", "rag"},
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query":    map[string]interface{}{"type": "string", "description": "Search query or task intent."},
				"category": map[string]interface{}{"type": "string", "description": "Filter by tool domain (vault, people, workspace, routines, ingest, guard, ui, tui, codeact)."},
				"limit":    map[string]interface{}{"type": "integer", "description": "Maximum number of tools to return (default 5)."},
				"format":   formatProp,
			},
			"required": []string{"query"},
		},
	})

	c.Register(ToolDescriptor{
		Name:        "search_vault",
		Category:    "vault",
		Description: "Instant (< 3ms) full-text search across the Second Brain using SQLite FTS5, BM25 ranking, and faceted metadata filters.",
		Keywords:    []string{"search", "vault", "notes", "fts5", "bm25", "find"},
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query":     map[string]interface{}{"type": "string", "description": "Search query or concept."},
				"scope":     map[string]interface{}{"type": "string", "enum": []string{"all", "tech", "meta", "daily", "inbox", "sources"}},
				"note_type": map[string]interface{}{"type": "string"},
				"tag":       map[string]interface{}{"type": "string"},
				"format":    formatProp,
			},
			"required": []string{"query"},
		},
	})

	c.Register(ToolDescriptor{
		Name:        "resolve_note",
		Category:    "vault",
		Description: "Deterministically resolves a note by title, alias, or wikilink without scanning the filesystem.",
		Keywords:    []string{"resolve", "wikilink", "alias", "slug", "note"},
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{"type": "string", "description": "Title, slug, or wikilink of the note."},
			},
			"required": []string{"name"},
		},
	})

	c.Register(ToolDescriptor{
		Name:        "get_note_content",
		Category:    "vault",
		Description: "Reads and returns the complete Markdown text and metadata frontmatter of a vault note.",
		Keywords:    []string{"read", "content", "markdown", "open", "text"},
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"rel_path": map[string]interface{}{"type": "string", "description": "Relative path of the note."},
			},
			"required": []string{"rel_path"},
		},
	})

	c.Register(ToolDescriptor{
		Name:        "get_backlinks",
		Category:    "vault",
		Description: "Discovers all incoming wikilinks referencing a specific target note.",
		Keywords:    []string{"backlinks", "incoming", "references", "graph", "links"},
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"rel_path": map[string]interface{}{"type": "string", "description": "Relative path of the target note."},
				"format":   formatProp,
			},
			"required": []string{"rel_path"},
		},
	})

	c.Register(ToolDescriptor{
		Name:        "vault_health",
		Category:    "vault",
		Description: "Runs vault health checks and diagnostics, reporting dangling wikilinks, orphan notes, and indexing status.",
		Keywords:    []string{"health", "doctor", "diagnostics", "dangling", "orphans", "status"},
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	})

	c.Register(ToolDescriptor{
		Name:        "search_people",
		Category:    "people",
		Description: "Faceted search across colleague and collaborator profiles in the Second Brain.",
		Keywords:    []string{"people", "colleague", "collaborator", "team", "person", "squad", "role"},
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query":  map[string]interface{}{"type": "string"},
				"squad":  map[string]interface{}{"type": "string"},
				"role":   map[string]interface{}{"type": "string"},
				"format": formatProp,
			},
		},
	})

	c.Register(ToolDescriptor{
		Name:        "person_briefing",
		Category:    "people",
		Description: "Generates an executive strategic briefing for 1:1s and meetings, combining interaction history, active backlog, and recent mentions.",
		Keywords:    []string{"briefing", "1:1", "one-on-one", "meeting", "colleague", "prep", "preflight"},
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":   map[string]interface{}{"type": "string", "description": "Name or slug of the person."},
				"format": formatProp,
			},
			"required": []string{"name"},
		},
	})

	c.Register(ToolDescriptor{
		Name:        "search_projects",
		Category:    "workspace",
		Description: "Discovers and filters local git repositories in ~/Work.",
		Keywords:    []string{"projects", "repositories", "repos", "git", "dirty", "clean", "stack"},
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query":      map[string]interface{}{"type": "string"},
				"dirty_only": map[string]interface{}{"type": "boolean"},
				"clean_only": map[string]interface{}{"type": "boolean"},
				"stack":      map[string]interface{}{"type": "string"},
				"format":     formatProp,
			},
		},
	})

	c.Register(ToolDescriptor{
		Name:        "scan_workspace_repos",
		Category:    "workspace",
		Description: "Inspects git repositories across the workspace, detecting dirty states, active branches, and unpushed commits.",
		Keywords:    []string{"workspace", "scan", "dirty", "unpushed", "branches", "repos"},
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"format": formatProp,
			},
		},
	})

	c.Register(ToolDescriptor{
		Name:        "clone_repository",
		Category:    "workspace",
		Description: "Clones a remote repository into the structured local workspace.",
		Keywords:    []string{"clone", "git", "repo", "remote", "workspace"},
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url": map[string]interface{}{"type": "string", "description": "Remote git URL."},
			},
			"required": []string{"url"},
		},
	})

	c.Register(ToolDescriptor{
		Name:        "get_active_backlog",
		Category:    "routines",
		Description: "Returns active priority tasks (P0, P1, P2) extracted from weekly engineering summaries.",
		Keywords:    []string{"backlog", "tasks", "priority", "p0", "p1", "todos"},
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"priority": map[string]interface{}{"type": "string"},
				"format":   formatProp,
			},
		},
	})

	c.Register(ToolDescriptor{
		Name:        "run_dream_cycle",
		Category:    "routines",
		Description: "Runs the cognitive dream cycle to triage inbox notes, audit references, and synthesize daily learnings.",
		Keywords:    []string{"dream", "cognitive", "metacognition", "triage", "routine"},
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	})

	c.Register(ToolDescriptor{
		Name:        "compile_staff_dossier",
		Category:    "routines",
		Description: "Compiles staff engineering performance review executive dossier.",
		Keywords:    []string{"dossier", "avd", "staff", "performance", "telemetry", "review"},
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"year":     map[string]interface{}{"type": "integer"},
				"semester": map[string]interface{}{"type": "integer"},
			},
		},
	})

	c.Register(ToolDescriptor{
		Name:        "ingest_note",
		Category:    "ingest",
		Description: "Ingests external text or markdown into the vault with automatic OKF metadata formatting.",
		Keywords:    []string{"ingest", "create", "write", "note", "new"},
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"title":   map[string]interface{}{"type": "string"},
				"content": map[string]interface{}{"type": "string"},
				"type":    map[string]interface{}{"type": "string"},
			},
			"required": []string{"title", "content"},
		},
	})

	c.Register(ToolDescriptor{
		Name:        "validate_public_export",
		Category:    "guard",
		Description: "Audits notes for private, proprietary, or confidential data leaks before public publishing.",
		Keywords:    []string{"guard", "audit", "security", "export", "redact", "leak"},
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{"type": "string"},
			},
			"required": []string{"path"},
		},
	})

	c.Register(ToolDescriptor{
		Name:        "ui_get_active_context",
		Category:    "ui",
		Description: "Returns user's current spatial presence, active surface, focused note, and viewport context.",
		Keywords:    []string{"ui", "context", "spatial", "presence", "active", "screen", "viewport"},
		InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
	})

	c.Register(ToolDescriptor{
		Name:        "ui_open_note",
		Category:    "ui",
		Description: "Opens and focuses a note in the active UI (Web Galaxy or Terminal UI).",
		Keywords:    []string{"ui", "open", "display", "navigate", "focus", "camera"},
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"rel_path": map[string]interface{}{"type": "string"},
			},
			"required": []string{"rel_path"},
		},
	})

	c.Register(ToolDescriptor{
		Name:        "ui_focus_domain",
		Category:    "ui",
		Description: "Frames the Web Galaxy visualizer camera around a specific conceptual knowledge domain or nebula.",
		Keywords:    []string{"ui", "galaxy", "camera", "zoom", "sector", "domain", "nebula"},
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"domain": map[string]interface{}{"type": "string"},
			},
			"required": []string{"domain"},
		},
	})

	c.Register(ToolDescriptor{
		Name:        "ui_flash_notification",
		Category:    "ui",
		Description: "Renders a HUD notification toast on the active user surface.",
		Keywords:    []string{"ui", "toast", "notification", "hud", "flash", "message"},
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{"type": "string"},
				"title":   map[string]interface{}{"type": "string"},
			},
			"required": []string{"message"},
		},
	})

	c.Register(ToolDescriptor{
		Name:        "audit_agent_missed_opportunities",
		Category:    "guard",
		Description: "Audits agent session history to detect missed opportunities and knowledge gaps.",
		Keywords:    []string{"audit", "evals", "telemetry", "gaps", "agent"},
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"since":  map[string]interface{}{"type": "string"},
				"agent":  map[string]interface{}{"type": "string"},
				"format": formatProp,
			},
		},
	})
}
