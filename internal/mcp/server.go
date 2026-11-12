//nolint:revive // package comment not required
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	graphv1 "github.com/rjfonseca/daneel/api/v1"
	"github.com/rjfonseca/daneel/internal/auditor"
	"github.com/rjfonseca/daneel/internal/codeact"
	"github.com/rjfonseca/daneel/internal/config"
	"github.com/rjfonseca/daneel/internal/daemon"
	"github.com/rjfonseca/daneel/internal/dream"
	"github.com/rjfonseca/daneel/internal/guard"
	"github.com/rjfonseca/daneel/internal/ingest"
	"github.com/rjfonseca/daneel/internal/ops/backlog"
	"github.com/rjfonseca/daneel/internal/ops/briefing"
	"github.com/rjfonseca/daneel/internal/ops/clone"
	"github.com/rjfonseca/daneel/internal/ops/dossier"
	"github.com/rjfonseca/daneel/internal/ops/meetings"
	"github.com/rjfonseca/daneel/internal/ops/workspace"
	"github.com/rjfonseca/daneel/internal/routine"
	"github.com/rjfonseca/daneel/internal/safefile"
	"github.com/rjfonseca/daneel/internal/search"
	"github.com/rjfonseca/daneel/internal/toon"
)

// JSONRPCRequest represents an incoming JSON-RPC 2.0 message.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents an outgoing JSON-RPC 2.0 message.
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError describes an RPC error.
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Tool describes an available MCP tool.
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

// ToolCallContent holds the return payload of a tool execution.
type ToolCallContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ToolCallResult represents the return structure of tools/call.
type ToolCallResult struct {
	Content []ToolCallContent `json:"content"`
	IsError bool              `json:"isError,omitempty"`
}

// Server is the Daneel MCP server instance.
type Server struct {
	vaultRoot    string
	engine       *search.Engine
	cfg          *config.Config
	catalog      *ToolCatalog
	dynamicTools bool
	in           io.Reader
	out          io.Writer
}

// NewServer creates a new MCP server.
func NewServer(vaultRoot string, engine *search.Engine, cfg ...*config.Config) *Server {
	var c *config.Config
	if len(cfg) > 0 {
		c = cfg[0]
	}
	dynamic := c != nil && strings.EqualFold(c.MCP.ToolExposure, "dynamic")
	return &Server{
		vaultRoot:    vaultRoot,
		engine:       engine,
		cfg:          c,
		catalog:      NewToolCatalog(),
		dynamicTools: dynamic,
		in:           os.Stdin,
		out:          os.Stdout,
	}
}

// SetDynamicTools enables or disables dynamic tool exposure (Tool-as-RAG).
func (s *Server) SetDynamicTools(enabled bool) {
	s.dynamicTools = enabled
}

// Serve runs the stdio JSON-RPC loop.
func (s *Server) Serve() error {
	scanner := bufio.NewScanner(s.in)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.sendError(nil, -32700, "Parse error")
			continue
		}

		s.handleRequest(&req)
	}

	return scanner.Err()
}

func (s *Server) handleRequest(req *JSONRPCRequest) {
	switch req.Method {
	case "initialize":
		s.sendResult(req.ID, map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "daneel-assistant",
				"version": "1.0.0",
			},
		})

	case "notifications/initialized":
		// No response required for notifications

	case "ping":
		s.sendResult(req.ID, map[string]interface{}{})

	case "tools/list":
		formatProp := map[string]interface{}{
			"type":        "string",
			"enum":        []string{"toon", "markdown"},
			"description": "Output serialization format: 'toon' (compact Token-Oriented Object Notation, default) or 'markdown'.",
		}

		tools := []Tool{
			{
				Name:        "resolve_tools",
				Description: "Dynamically discovers and retrieves matching MCP tool definitions based on intent or query (Tool-as-RAG).",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "Search query or task intent (e.g. 'workspace git', 'briefing', 'read note').",
						},
						"category": map[string]interface{}{
							"type":        "string",
							"description": "Filter by tool domain (vault, people, workspace, routines, ingest, guard, ui, tui, codeact).",
						},
						"limit": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum number of tools to return (default 5).",
						},
						"format": formatProp,
					},
					"required": []string{"query"},
				},
			},
			{
				Name:        "search_vault",
				Description: "Instant (< 3ms) full-text search across the Second Brain using SQLite FTS5, BM25 ranking, and faceted metadata filters.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "Search query or concept.",
						},
						"scope": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"all", "tech", "meta", "daily", "inbox", "sources"},
							"description": "Scope filter (all, tech, meta, daily, sources, inbox).",
						},
						"note_type": map[string]interface{}{
							"type":        "string",
							"description": "Faceted filter by OKF type (e.g. 'rfc', 'adr', 'arquitetura', 'person', 'processo').",
						},
						"tag": map[string]interface{}{
							"type":        "string",
							"description": "Filter by specific tag.",
						},
						"limit": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum number of results to return (default: 10).",
						},
						"format": formatProp,
					},
				},
			},
			{
				Name:        "resolve_note",
				Description: "Fuzzy resolve any note title, slug, or alias to its canonical vault path in < 2ms.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "The title, filename, or alias to resolve (e.g. 'scaffold build system', 'manifesto reativo').",
						},
					},
					"required": []string{"query"},
				},
			},
			{
				Name:        "search_people",
				Description: "Search and discover colleagues and leadership profiles by name, alias, squad, role, or email.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "Person name, nickname, or partial name (e.g. 'Ada Lovelace', 'Turing').",
						},
						"squad": map[string]interface{}{
							"type":        "string",
							"description": "Optional squad or tribe filter.",
						},
						"role": map[string]interface{}{
							"type":        "string",
							"description": "Optional role filter (e.g. 'Engineering Manager', 'Staff').",
						},
						"email": map[string]interface{}{
							"type":        "string",
							"description": "Optional email filter.",
						},
						"format": formatProp,
					},
				},
			},
			{
				Name:        "search_projects",
				Description: "Instant search across all repositories in ~/Work without filesystem overhead.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "Repository name, path segment, or service name (e.g. 'gateway', 'notification-service', 'identity-service').",
						},
						"dirty_only": map[string]interface{}{
							"type":        "boolean",
							"description": "Filter only repositories with uncommitted local changes.",
						},
						"clean_only": map[string]interface{}{
							"type":        "boolean",
							"description": "Filter only clean repositories.",
						},
						"stack": map[string]interface{}{
							"type":        "string",
							"description": "Filter by primary language (e.g. 'go', 'python', 'typescript').",
						},
						"format": formatProp,
					},
				},
			},
			{
				Name:        "person_briefing",
				Description: "Generates an executive strategic briefing for 1:1s and meetings, combining interaction history, active backlog, and recent mentions.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Name or slug of the person (e.g. 'jane-doe', 'alex-smith').",
						},
						"format": formatProp,
					},
					"required": []string{"name"},
				},
			},
			{
				Name:        "get_active_backlog",
				Description: "Returns active priority tasks (P0, P1, P2) extracted from weekly engineering summaries.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"priority": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"all", "P0", "P1", "P2"},
							"description": "Priority filter level (default: all).",
						},
						"format": formatProp,
					},
				},
			},
			{
				Name:        "get_note_content",
				Description: "Reads the full markdown content of a specific note by relative vault path.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"rel_path": map[string]interface{}{
							"type":        "string",
							"description": "Relative path of the note in the vault (e.g. 'notes/ia/plataforma-de-agentes-autonomos.md').",
						},
					},
					"required": []string{"rel_path"},
				},
			},
			{
				Name:        "get_backlinks",
				Description: "Returns all backlinks and cross-references pointing to a specific technical note.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"rel_path": map[string]interface{}{
							"type":        "string",
							"description": "Relative path or name of the target note (e.g. 'notes/go/arenas.md').",
						},
						"format": formatProp,
					},
					"required": []string{"rel_path"},
				},
			},
			{
				Name:        "vault_health",
				Description: "Returns Vault health metrics: total notes, indexed cross-links, and scope distribution.",
				InputSchema: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
			{
				Name:        "validate_public_export",
				Description: "Audits and validates that a note adheres to 100% of privacy and security guardrails before public export.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"file_path": map[string]interface{}{
							"type":        "string",
							"description": "Path to the markdown file to validate or empty to audit the entire public content directory.",
						},
					},
				},
			},
			{
				Name:        "compile_staff_dossier",
				Description: "Automatically compiles Staff Engineering Performance Review dossier, aggregating time telemetry [A-E], goals, and multiplier impact.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"year": map[string]interface{}{
							"type":        "integer",
							"description": "Reference year (e.g. 2026).",
						},
						"semester": map[string]interface{}{
							"type":        "integer",
							"enum":        []int{1, 2},
							"description": "Specific semester (1 or 2). Omit for full year.",
						},
					},
				},
			},
			{
				Name:        "scan_workspace_repos",
				Description: "Scans ~/Work in parallel, cataloging Git repositories, tech stacks, active branches, and connections to the Vault.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"sync_clean": map[string]interface{}{
							"type":        "boolean",
							"description": "Automatically runs git pull on clean repositories and prunes merged branches.",
						},
						"inactivity_threshold": map[string]interface{}{
							"type":        "string",
							"description": "Inactivity duration before resetting clean feature branches to main/master (e.g. '48h', '2d'). Default is 48h.",
						},
						"format": formatProp,
					},
				},
			},
			{
				Name:        "clone_repository",
				Description: "Clones a Git repository to ~/Work/<host>/<org>/<repo>, inspects code stack/documentation, and creates/updates a technical note in the Vault.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"repo": map[string]interface{}{
							"type":        "string",
							"description": "Git URL (SSH/HTTPS) or repository slug (e.g. 'git@github.com:foo/bar.git' or 'org/repo').",
						},
						"dream": map[string]interface{}{
							"type":        "boolean",
							"description": "Runs Dream Cycle to induce cognitive connections right after cloning (default: false).",
						},
						"fetch": map[string]interface{}{
							"type":        "boolean",
							"description": "Executes git fetch if the repository already exists locally (default: false).",
						},
					},
					"required": []string{"repo"},
				},
			},
			{
				Name:        "run_dream_cycle",
				Description: "Runs autonomous REM sleep cycle: self-heals links, maps strategic Tech ↔ Business radar, and generates morning handoff questions.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"date": map[string]interface{}{
							"type":        "string",
							"description": "ISO reference date (YYYY-MM-DD). Default: today.",
						},
						"auto_fix": map[string]interface{}{
							"type":        "boolean",
							"description": "Applies automated fixes to the Vault (default: true).",
						},
					},
				},
			},
			{
				Name:        "ingest_meeting_notes",
				Description: "Batch processes meeting notes and transcripts from Google Meet/Gemini into persistent archives.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"zip_path": map[string]interface{}{
							"type":        "string",
							"description": "Optional path to a .zip file containing meeting exports.",
						},
						"clean_inbox": map[string]interface{}{
							"type":        "boolean",
							"description": "Deletes raw files from inbox/ after successful processing (default: true).",
						},
					},
				},
			},
			{
				Name:        "ingest_note",
				Description: "Ingests raw markdown note content directly into the Vault, synthesizing YAML frontmatter and indexing in SQLite FTS5.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"title": map[string]interface{}{
							"type":        "string",
							"description": "Title of the note",
						},
						"content": map[string]interface{}{
							"type":        "string",
							"description": "Markdown body content",
						},
						"destination": map[string]interface{}{
							"type":        "string",
							"description": "Vault destination directory (default: 'inbox')",
						},
						"tags": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "string",
							},
							"description": "Optional tags to attach",
						},
						"source_url": map[string]interface{}{
							"type":        "string",
							"description": "Optional source reference URL",
						},
					},
					"required": []string{"title", "content"},
				},
			},
			{
				Name:        "ingest_path",
				Description: "Ingests an arbitrary file or directory from the host filesystem into the Vault, supporting recursive scans, frontmatter synthesis, and FTS5 indexing.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "Absolute or relative host path to file or directory",
						},
						"destination": map[string]interface{}{
							"type":        "string",
							"description": "Vault destination directory (default: 'inbox')",
						},
						"recursive": map[string]interface{}{
							"type":        "boolean",
							"description": "Recursively scan directories (default: true)",
						},
						"move": map[string]interface{}{
							"type":        "boolean",
							"description": "Delete source file after ingestion (default: false)",
						},
					},
					"required": []string{"path"},
				},
			},
			{
				Name:        "ui_get_active_context",
				Description: "Gets the active UI surface (web galaxy, TUI, or none), current note path, focused sector, and selected text.",
				InputSchema: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
			{
				Name:        "ui_open_note",
				Description: "Opens a note on the user's active UI screen (animates camera to the star, opens drawer, and isolates constellation on the Web Galaxy, or opens the note in the TUI). If no UI is active, returns the full note content directly.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"rel_path": map[string]interface{}{
							"type":        "string",
							"description": "Relative path of the note in the vault (e.g. 'notes/arquitetura/foo.md').",
						},
					},
					"required": []string{"rel_path"},
				},
			},
			{
				Name:        "ui_focus_domain",
				Description: "Focuses the Web Galaxy camera on a specific knowledge domain or nebula (e.g. 'notes/arquitetura', 'notes/pessoas', 'sources/gitlab').",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"domain": map[string]interface{}{
							"type":        "string",
							"description": "Domain, group, or category name to focus on.",
						},
					},
					"required": []string{"domain"},
				},
			},
			{
				Name:        "ui_focus_sector",
				Description: "Focuses the Web Galaxy camera on a specific sector or nebula (alias for ui_focus_domain).",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"sector": map[string]interface{}{
							"type":        "string",
							"description": "Sector or category name to focus on.",
						},
					},
					"required": []string{"sector"},
				},
			},
			{
				Name:        "ui_flash_notification",
				Description: "Displays a temporary holographic cybernetic HUD toast on the user's screen.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"message": map[string]interface{}{
							"type":        "string",
							"description": "Notification message to display.",
						},
						"title": map[string]interface{}{
							"type":        "string",
							"description": "Optional notification title (e.g. 'Security Audit').",
						},
					},
					"required": []string{"message"},
				},
			},
			{
				Name:        "ui_pin_note",
				Description: "Pins a high-priority note to the Seldon Vault Anchors temporal hotbar in the active Web Galaxy UI, making it accessible via an immediate HUD pill.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"rel_path": map[string]interface{}{
							"type":        "string",
							"description": "Relative path of the note in the vault to pin (e.g. 'notes/architecture/adr-001.md').",
						},
						"label": map[string]interface{}{
							"type":        "string",
							"description": "Optional display label for the anchor pill on the hotbar. Defaults to the note title or filename.",
						},
						"icon": map[string]interface{}{
							"type":        "string",
							"description": "Optional icon or emoji for the hotbar pill (e.g. '🏛️', '⚡', '📌'). Defaults to '📌'.",
						},
					},
					"required": []string{"rel_path"},
				},
			},
			{
				Name:        "ui_unpin_note",
				Description: "Unpins a note from the Seldon Vault Anchors temporal hotbar in the active Web Galaxy UI.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"rel_path": map[string]interface{}{
							"type":        "string",
							"description": "Relative path of the note in the vault to unpin.",
						},
					},
					"required": []string{"rel_path"},
				},
			},
			{
				Name:        "ui_list_pinned_anchors",
				Description: "Lists all declarative and active Seldon Vault Anchors configured for the temporal hotbar.",
				InputSchema: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
			{
				Name:        "tui_switch_view",
				Description: "Switches the active view pane in the running Daneel TUI application.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"view_name": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"search", "dashboard", "workspace", "people", "dream"},
							"description": "Name of the view to switch to.",
						},
					},
					"required": []string{"view_name"},
				},
			},
			{
				Name:        "tui_open_note",
				Description: "Opens a specific note or executes a search query directly in the active Daneel TUI.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"filepath": map[string]interface{}{
							"type":        "string",
							"description": "Path to the note or exact search query to open.",
						},
					},
					"required": []string{"filepath"},
				},
			},
			{
				Name:        "tui_get_current_state",
				Description: "Gets the current active tab and open file in the user's TUI to establish visual context.",
				InputSchema: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
			{
				Name:        "tui_highlight_lines",
				Description: "Highlights a specific range of lines in the currently open note in the TUI.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"start": map[string]interface{}{
							"type":        "integer",
							"description": "Start line number (1-indexed).",
						},
						"end": map[string]interface{}{
							"type":        "integer",
							"description": "End line number (inclusive).",
						},
					},
					"required": []string{"start", "end"},
				},
			},
			{
				Name:        "tui_flash_notification",
				Description: "Displays a temporary notification/toast in the user's TUI status bar.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"message": map[string]interface{}{
							"type":        "string",
							"description": "The message to display.",
						},
					},
					"required": []string{"message"},
				},
			},
			{
				Name:        "tui_update_widget",
				Description: "Pushes dynamic content to a custom widget on the user's TUI dashboard.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"content": map[string]interface{}{
							"type":        "string",
							"description": "Markdown/text content to inject.",
						},
					},
					"required": []string{"content"},
				},
			},
			{
				Name:        "audit_agent_missed_opportunities",
				Description: "Audits recent AI agent session history (Antigravity, Claude Code, Cursor) to detect Missed Opportunities (information exists in vault, but agent asked human) and Knowledge Gaps (vault lacks information).",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"since": map[string]interface{}{
							"type":        "string",
							"description": "Time window for analysis (e.g. '24h', '48h', '72h'). Defaults to '48h'.",
						},
						"agent": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"all", "antigravity", "claude-code", "cursor"},
							"description": "Filter by agent framework (default: 'all').",
						},
						"apply": map[string]interface{}{
							"type":        "boolean",
							"description": "Persist discovered learnings to meta/guias/aprendizados-agente.md and dashboard metrics (default: false).",
						},
						"format": formatProp,
					},
				},
			},
			{
				Name:        "codeact_execute",
				Description: "Executes ephemeral WebAssembly (Wasm) bytecode in a capability-bounded sandbox (< 1ms spin-up, 16MB memory limit, pure Go wazero, zero CGO).",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"wasm_bytes_base64": map[string]interface{}{
							"type":        "string",
							"description": "Base64-encoded Wasm binary module.",
						},
						"wasm_path": map[string]interface{}{
							"type":        "string",
							"description": "Relative path to a compiled Wasm binary in the vault or workspace.",
						},
						"stdin": map[string]interface{}{
							"type":        "string",
							"description": "Standard input payload (text, TOON, or JSON) to feed into the Wasm module.",
						},
						"timeout_ms": map[string]interface{}{
							"type":        "integer",
							"description": "Execution deadline in milliseconds (default 5000ms, max 10000ms).",
						},
						"format": formatProp,
					},
				},
			},
		}

		if s.dynamicTools {
			coreMap := map[string]bool{
				"resolve_tools":         true,
				"search_vault":          true,
				"get_note_content":      true,
				"ui_get_active_context": true,
				"vault_health":          true,
			}
			var pruned []Tool
			for _, t := range tools {
				if coreMap[t.Name] {
					pruned = append(pruned, t)
				}
			}
			tools = pruned
		}

		s.sendResult(req.ID, map[string]interface{}{
			"tools": tools,
		})

	case "tools/call":
		var callParams struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &callParams); err != nil {
			s.sendError(req.ID, -32602, "Invalid params")
			return
		}

		start := time.Now()
		resultText, isErr := s.executeTool(callParams.Name, callParams.Arguments)
		durationMs := time.Since(start).Milliseconds()

		if s.vaultRoot != "" {
			if store, err := routine.OpenDefaultStore(s.vaultRoot); err == nil && store != nil {
				_ = store.RecordCommandEvent(context.Background(), routine.CommandTelemetryEvent{
					Surface:    "mcp",
					Command:    callParams.Name,
					StartedAt:  start.Unix(),
					DurationMs: durationMs,
					IsError:    isErr,
				})
				_ = store.Close()
			}
		}

		s.sendResult(req.ID, ToolCallResult{
			Content: []ToolCallContent{
				{
					Type: "text",
					Text: resultText,
				},
			},
			IsError: isErr,
		})

	default:
		s.sendError(req.ID, -32601, fmt.Sprintf("Method '%s' not found", req.Method))
	}
}

func (s *Server) resolveFormat(args map[string]interface{}) string {
	if f, ok := args["format"].(string); ok && f != "" {
		return strings.ToLower(strings.TrimSpace(f))
	}
	if s.cfg != nil && s.cfg.MCP.DefaultFormat != "" {
		return strings.ToLower(strings.TrimSpace(s.cfg.MCP.DefaultFormat))
	}
	return "toon"
}

func (s *Server) executeTool(name string, args map[string]interface{}) (string, bool) {
	switch name {
	case "resolve_tools":
		query, _ := args["query"].(string)
		if query == "" {
			return "Parameter 'query' is required.", true
		}
		category, _ := args["category"].(string)
		limit := 5
		if l, ok := args["limit"].(float64); ok && l > 0 {
			limit = int(l)
		}
		if s.catalog == nil {
			s.catalog = NewToolCatalog()
		}
		results := s.catalog.Search(query, category, limit)
		if len(results) == 0 {
			return fmt.Sprintf("No tools found matching query '%s'", query), false
		}
		format := s.resolveFormat(args)
		if format == "json" {
			data, err := json.MarshalIndent(results, "", "  ")
			if err != nil {
				return fmt.Sprintf("Failed to marshal tools: %v", err), true
			}
			return string(data), false
		}
		return s.catalog.FormatTOON(results), false

	case "codeact_execute":
		b64, _ := args["wasm_bytes_base64"].(string)
		path, _ := args["wasm_path"].(string)
		stdin, _ := args["stdin"].(string)
		if b64 == "" && path == "" {
			return "Either 'wasm_bytes_base64' or 'wasm_path' must be provided.", true
		}

		timeoutMs := 5000
		if t, ok := args["timeout_ms"].(float64); ok && t > 0 {
			if t > 10000 {
				t = 10000
			}
			timeoutMs = int(t)
		}

		runner := codeact.NewRunner(codeact.Config{
			MaxMemoryMB: 16,
			Timeout:     time.Duration(timeoutMs) * time.Millisecond,
		})

		var res *codeact.Result
		var err error
		if b64 != "" {
			res, err = runner.ExecuteBase64(context.Background(), b64, stdin)
		} else {
			targetPath := path
			if !filepath.IsAbs(targetPath) && s.vaultRoot != "" {
				targetPath = filepath.Join(s.vaultRoot, targetPath)
			}
			res, err = runner.ExecuteFile(context.Background(), targetPath, stdin)
		}
		if err != nil {
			return fmt.Sprintf("Wasm execution error: %v", err), true
		}

		format := s.resolveFormat(args)
		if format == "json" {
			data, err := json.MarshalIndent(res, "", "  ")
			if err != nil {
				return fmt.Sprintf("Failed to marshal result: %v", err), true
			}
			return string(data), false
		}
		out, err := toon.MarshalString(res)
		if err != nil {
			return res.Stdout, false
		}
		return out, false

	case "search_vault":
		query, _ := args["query"].(string)
		scope, _ := args["scope"].(string)
		if scope == "" {
			scope = "all"
		}
		noteType, _ := args["note_type"].(string)
		tag, _ := args["tag"].(string)
		if query == "" && noteType == "" && tag == "" {
			return "Parameter 'query', 'note_type', or 'tag' is required.", true
		}
		limit := 10
		if l, ok := args["limit"].(float64); ok && l > 0 {
			limit = int(l)
		}

		results, err := s.engine.SearchFaceted(context.Background(), search.SearchOptions{
			Query:    query,
			Scope:    scope,
			NoteType: noteType,
			Tag:      tag,
			Limit:    limit,
		})
		if err != nil {
			return fmt.Sprintf("Failed to execute search: %v", err), true
		}

		format := s.resolveFormat(args)
		if format == "toon" {
			headers := []string{"title", "path", "scope", "type", "snippet", "score"}
			rows := make([][]string, len(results))
			for i, r := range results {
				cleanSnippet := strings.ReplaceAll(r.Snippet, "<mark>", "")
				cleanSnippet = strings.ReplaceAll(cleanSnippet, "</mark>", "")
				cleanSnippet = strings.ReplaceAll(cleanSnippet, "==", "")
				rows[i] = []string{
					r.Title,
					r.RelPath,
					r.Scope,
					r.Type,
					cleanSnippet,
					fmt.Sprintf("%.2f", r.Score),
				}
			}
			return toon.EncodeTabular("results", headers, rows), false
		}

		if len(results) == 0 {
			return fmt.Sprintf("No results found in vault (query: %q, scope: %s, note_type: %q, tag: %q).", query, scope, noteType, tag), false
		}

		var sb strings.Builder
		displayDesc := query
		if displayDesc == "" {
			displayDesc = fmt.Sprintf("type:%s tag:%s", noteType, tag)
		}
		fmt.Fprintf(&sb, "# 🔍 Search Results: %q (Scope: %s, %d found)\n\n", displayDesc, scope, len(results))
		for i, r := range results {
			fmt.Fprintf(&sb, "### %d. [%s](%s) `[%s]`\n", i+1, r.Title, r.RelPath, r.Scope)
			if r.Description != "" {
				fmt.Fprintf(&sb, "> *%s*\n\n", r.Description)
			}
			if r.Snippet != "" {
				cleanSnippet := strings.ReplaceAll(r.Snippet, "<mark>", "**")
				cleanSnippet = strings.ReplaceAll(cleanSnippet, "</mark>", "**")
				fmt.Fprintf(&sb, "%s\n\n", cleanSnippet)
			}
		}

		return sb.String(), false

	case "resolve_note":
		query, _ := args["query"].(string)
		if query == "" {
			return "Parameter 'query' is required.", true
		}

		res, err := s.engine.ResolveNote(context.Background(), query)
		if err != nil {
			return fmt.Sprintf("Failed to resolve note: %v", err), true
		}
		if !res.Found || res.Match == nil {
			return fmt.Sprintf("No note found matching query %q.", query), false
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "# 🧭 Note Resolved: %s\n\n", res.Match.Title)
		fmt.Fprintf(&sb, "- **Relative Path:** `%s`\n", res.Match.RelPath)
		fmt.Fprintf(&sb, "- **Absolute Path:** `%s`\n", res.Match.AbsPath)
		fmt.Fprintf(&sb, "- **Type:** `%s`\n", res.Match.Type)
		fmt.Fprintf(&sb, "- **Confidence Score:** `%.2f`\n", res.Match.Score)

		return sb.String(), false

	case "search_people":
		query, _ := args["query"].(string)
		squad, _ := args["squad"].(string)
		role, _ := args["role"].(string)
		email, _ := args["email"].(string)

		results, err := s.engine.SearchPeople(context.Background(), query, squad, role, email)
		if err != nil {
			return fmt.Sprintf("Failed to search people: %v", err), true
		}

		format := s.resolveFormat(args)
		if format == "toon" {
			headers := []string{"name", "role", "squad", "email", "path", "aliases"}
			rows := make([][]string, len(results))
			for i, p := range results {
				rows[i] = []string{
					p.Name,
					p.Role,
					p.Squad,
					p.Email,
					p.RelPath,
					strings.Join(p.Aliases, " "),
				}
			}
			return toon.EncodeTabular("people", headers, rows), false
		}

		if len(results) == 0 {
			return "No colleague profiles found matching criteria.", false
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "# 👤 People Profiles (%d found)\n\n", len(results))
		for i, p := range results {
			fmt.Fprintf(&sb, "### %d. %s\n", i+1, p.Name)
			if p.Role != "" || p.Squad != "" {
				fmt.Fprintf(&sb, "- **Role / Squad:** %s — %s\n", p.Role, p.Squad)
			}
			fmt.Fprintf(&sb, "- **Path:** `%s`\n", p.RelPath)
			if p.Email != "" {
				fmt.Fprintf(&sb, "- **Email:** [%s](mailto:%s)\n", p.Email, p.Email)
			}
			if len(p.Aliases) > 0 {
				fmt.Fprintf(&sb, "- **Aliases:** %s\n", strings.Join(p.Aliases, ", "))
			}
			sb.WriteString("\n")
		}

		return sb.String(), false

	case "search_projects":
		query, _ := args["query"].(string)
		dirtyOnly, _ := args["dirty_only"].(bool)
		cleanOnly, _ := args["clean_only"].(bool)
		stack, _ := args["stack"].(string)
		limit := 20
		if l, ok := args["limit"].(float64); ok && l > 0 {
			limit = int(l)
		}

		results, err := s.engine.SearchProjects(context.Background(), query, dirtyOnly, cleanOnly, stack, limit)
		if err != nil {
			return fmt.Sprintf("Failed to search projects: %v", err), true
		}

		format := s.resolveFormat(args)
		if format == "toon" {
			headers := []string{"name", "path", "branch", "status", "stack"}
			rows := make([][]string, len(results))
			for i, r := range results {
				st := "clean"
				if r.Dirty {
					st = "dirty"
				}
				rows[i] = []string{
					r.Name,
					r.RelPath,
					r.Branch,
					st,
					r.Stack,
				}
			}
			return toon.EncodeTabular("projects", headers, rows), false
		}

		if len(results) == 0 {
			return "No repositories found matching criteria in ~/Work.", false
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "# 📦 Repositories in ~/Work (%d found)\n\n", len(results))
		for i, r := range results {
			status := "🟢 Clean"
			if r.Dirty {
				status = "🟡 Dirty"
			}
			fmt.Fprintf(&sb, "### %d. %s (`%s`)\n", i+1, r.Name, r.RelPath)
			fmt.Fprintf(&sb, "- **Path:** `%s`\n", r.Path)
			fmt.Fprintf(&sb, "- **Branch & Status:** `%s` [%s]\n", r.Branch, status)
			if r.Stack != "" {
				fmt.Fprintf(&sb, "- **Stack:** %s\n", r.Stack)
			}
			if r.Description != "" {
				fmt.Fprintf(&sb, "- **Description:** %s\n", r.Description)
			}
			sb.WriteString("\n")
		}

		return sb.String(), false

	case "person_briefing":
		personName, _ := args["name"].(string)
		if personName == "" {
			return "Parameter 'name' is required.", true
		}

		service := briefing.NewPreflightService(s.vaultRoot)
		b, err := service.GenerateBriefing(personName)
		if err != nil {
			return fmt.Sprintf("Failed to generate briefing: %v", err), true
		}

		format := s.resolveFormat(args)
		if format == "toon" {
			out, err := toon.MarshalString(b)
			if err == nil {
				return out, false
			}
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "# 👤 1:1 Briefing: %s\n\n", b.Name)
		if b.Cargo != "" || b.Squad != "" {
			fmt.Fprintf(&sb, "**Role / Squad:** %s (%s)\n\n", b.Cargo, b.Squad)
		}

		if len(b.RecentInteractions) > 0 {
			sb.WriteString("## 🤝 Recent Interaction History:\n")
			for _, inter := range b.RecentInteractions {
				fmt.Fprintf(&sb, "- %s\n", inter)
			}
			sb.WriteString("\n")
		}

		if len(b.PendingTasks) > 0 {
			sb.WriteString("## 🎯 Shared Actions & Active Backlog:\n")
			for _, act := range b.PendingTasks {
				fmt.Fprintf(&sb, "- [ ] %s\n", act)
			}
			sb.WriteString("\n")
		}

		if len(b.RecentDailyMentions) > 0 {
			sb.WriteString("## 📅 Recent Daily Log Mentions:\n")
			for _, m := range b.RecentDailyMentions {
				fmt.Fprintf(&sb, "- %s\n", m)
			}
			sb.WriteString("\n")
		}

		return sb.String(), false

	case "get_active_backlog":
		prioFilter, _ := args["priority"].(string)
		rep, err := backlog.GetActiveBacklog(s.vaultRoot)
		if err != nil {
			return fmt.Sprintf("Failed to retrieve active backlog: %v", err), true
		}

		format := s.resolveFormat(args)
		if format == "toon" {
			headers := []string{"status", "priority", "title", "description"}
			var rows [][]string
			for _, item := range rep.Items {
				if prioFilter != "" && prioFilter != "all" && item.Priority != prioFilter {
					continue
				}
				st := "pending"
				if item.Completed {
					st = "completed"
				}
				rows = append(rows, []string{st, item.Priority, item.Title, item.Description})
			}
			return toon.EncodeTabular("tasks", headers, rows), false
		}

		var sb strings.Builder
		sb.WriteString("# 📌 Active Engineering Backlog\n\n")
		count := 0
		for _, item := range rep.Items {
			if prioFilter != "" && prioFilter != "all" && item.Priority != prioFilter {
				continue
			}
			status := "[ ]"
			if item.Completed {
				status = "[x]"
			}
			fmt.Fprintf(&sb, "- %s **[%s]** %s: %s\n", status, item.Priority, item.Title, item.Description)
			count++
		}

		if count == 0 {
			sb.WriteString("No tasks found with the selected filter.\n")
		}

		return sb.String(), false

	case "get_note_content":
		relPath, _ := args["rel_path"].(string)
		if relPath == "" {
			return "Parameter 'rel_path' is required.", true
		}

		data, err := safefile.ReadFile(s.vaultRoot, relPath)
		if err != nil {
			return fmt.Sprintf("Failed to read file '%s': %v", relPath, err), true
		}
		return string(data), false

	case "get_backlinks":
		relPath, _ := args["rel_path"].(string)
		if relPath == "" {
			return "Parameter 'rel_path' is required.", true
		}
		links, err := s.engine.GetBacklinks(relPath)
		if err != nil {
			return fmt.Sprintf("Failed to retrieve backlinks: %v", err), true
		}

		format := s.resolveFormat(args)
		if format == "toon" {
			headers := []string{"source_path", "link_text"}
			rows := make([][]string, len(links))
			for i, l := range links {
				rows[i] = []string{l.SourceRelPath, l.LinkText}
			}
			return toon.EncodeTabular("backlinks", headers, rows), false
		}

		if len(links) == 0 {
			return fmt.Sprintf("No backlinks found for '%s'.", relPath), false
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "# 🔗 Backlinks for '%s':\n\n", relPath)
		for _, l := range links {
			fmt.Fprintf(&sb, "- 📄 `%s` (link text: \"%s\")\n", l.SourceRelPath, l.LinkText)
		}
		return sb.String(), false

	case "vault_health":
		health, err := s.engine.HealthCheck()
		if err != nil {
			return fmt.Sprintf("Failed to retrieve vault health: %v", err), true
		}

		var sb strings.Builder
		sb.WriteString("# 🏥 Vault Health Report\n\n")
		fmt.Fprintf(&sb, "- **Total Indexed Notes:** %d\n", health.TotalNotes)
		fmt.Fprintf(&sb, "- **Total Indexed Links:** %d\n\n", health.TotalLinks)
		sb.WriteString("### 📊 Scope Distribution:\n")
		for sc, cnt := range health.NotesByScope {
			fmt.Fprintf(&sb, "- **%s:** %d notes\n", sc, cnt)
		}
		return sb.String(), false

	case "validate_public_export":
		filePath, _ := args["file_path"].(string)
		val := guard.NewValidator(s.vaultRoot)
		if filePath != "" {
			fileRep, err := val.CheckFile(filePath)
			if err != nil {
				return fmt.Sprintf("Failed to audit file: %v", err), true
			}
			bytes, _ := json.MarshalIndent(fileRep, "", "  ")
			return string(bytes), false
		}
		auditRep, err := val.AuditPublicVault()
		if err != nil {
			return fmt.Sprintf("Failed to audit public vault: %v", err), true
		}
		bytes, _ := json.MarshalIndent(auditRep, "", "  ")
		return string(bytes), false

	case "compile_staff_dossier":
		year := 2026
		if y, ok := args["year"].(float64); ok && y > 0 {
			year = int(y)
		}
		semester := 0
		if sem, ok := args["semester"].(float64); ok {
			semester = int(sem)
		}

		compiler := dossier.NewCompiler(s.vaultRoot)
		dossier, err := compiler.Compile(year, semester, "")
		if err != nil {
			return fmt.Sprintf("Failed to compile performance review (AVD): %v", err), true
		}
		bytes, _ := json.MarshalIndent(dossier, "", "  ")
		return string(bytes), false

	case "scan_workspace_repos":
		syncClean, _ := args["sync_clean"].(bool)
		workDir := ""
		threshold := 48 * time.Hour
		if s.cfg != nil {
			workDir = s.cfg.GetWorkDir()
			threshold = s.cfg.GetBranchInactivityThreshold()
		}
		if rawThreshold, ok := args["inactivity_threshold"].(string); ok && rawThreshold != "" {
			if d, err := config.ParseDuration(rawThreshold); err == nil && d > 0 {
				threshold = d
			}
		}
		scanner := workspace.NewScanner(workDir, s.vaultRoot)
		scanner.SetInactivityThreshold(threshold)
		rep, err := scanner.Scan(syncClean)
		if err != nil {
			return fmt.Sprintf("Failed to scan workspace: %v", err), true
		}

		format := s.resolveFormat(args)
		if format == "toon" {
			headers := []string{"name", "path", "branch", "status", "last_commit_date"}
			var rows [][]string
			for _, r := range rep.DirtyRepos {
				rows = append(rows, []string{r.Name, r.Path, r.Branch, "dirty", r.LastCommitDate})
			}
			for _, catRepos := range rep.Categories {
				for _, r := range catRepos {
					if !r.Dirty {
						rows = append(rows, []string{r.Name, r.Path, r.Branch, "clean", r.LastCommitDate})
					}
				}
			}
			return toon.EncodeTabular("workspace_repos", headers, rows), false
		}

		bytes, _ := json.MarshalIndent(rep, "", "  ")
		return string(bytes), false

	case "clone_repository":
		repo, _ := args["repo"].(string)
		if repo == "" {
			return "Parameter 'repo' is required.", true
		}
		runDream, _ := args["dream"].(bool)
		forceFetch, _ := args["fetch"].(bool)

		cloner := clone.NewCloner()
		opts := clone.CloneOptions{
			VaultRoot:  s.vaultRoot,
			DBPath:     s.engine.DBPath(),
			ForceFetch: forceFetch,
			RunDream:   runDream,
		}
		if s.cfg != nil && s.cfg.Workspace.WorkDir != "" {
			opts.WorkRoot = s.cfg.Workspace.WorkDir
		}

		res, err := cloner.Clone(context.Background(), repo, opts)
		if err != nil {
			return fmt.Sprintf("Failed to clone repository: %v", err), true
		}

		bytes, _ := json.MarshalIndent(res, "", "  ")
		return string(bytes), false

	case "run_dream_cycle":
		targetDate, _ := args["date"].(string)
		autoFix := true
		if af, ok := args["auto_fix"].(bool); ok {
			autoFix = af
		}
		dreamEng := dream.NewDreamEngine(s.vaultRoot)
		rep, err := dreamEng.RunFullCycle(targetDate, autoFix, false)
		if err != nil {
			return fmt.Sprintf("Failed to run dream cycle: %v", err), true
		}
		bytes, _ := json.MarshalIndent(rep, "", "  ")
		return string(bytes), false

	case "ingest_meeting_notes":
		zipPath, _ := args["zip_path"].(string)
		cleanInbox := true
		if ci, ok := args["clean_inbox"].(bool); ok {
			cleanInbox = ci
		}

		proc := meetings.NewProcessor(s.vaultRoot)
		var rep *meetings.IngestReport
		var err error
		if zipPath != "" {
			rep, err = proc.ProcessZip(zipPath)
		} else {
			rep, err = proc.ProcessInbox(cleanInbox)
		}
		if err != nil {
			return fmt.Sprintf("Failed to ingest meeting notes: %v", err), true
		}
		bytes, _ := json.MarshalIndent(rep, "", "  ")
		return string(bytes), false

	case "ingest_note":
		title, _ := args["title"].(string)
		title = strings.TrimSpace(title)
		if title == "" {
			return "Parameter 'title' is required.", true
		}

		content, _ := args["content"].(string)
		content = strings.TrimSpace(content)
		if content == "" {
			return "Parameter 'content' is required.", true
		}

		dest := "inbox"
		if d, ok := args["destination"].(string); ok && strings.TrimSpace(d) != "" {
			dest = strings.TrimSpace(d)
		}

		sourceURL, _ := args["source_url"].(string)
		var tags []string
		if rawTags, ok := args["tags"].([]interface{}); ok {
			for _, t := range rawTags {
				if s, ok := t.(string); ok && strings.TrimSpace(s) != "" {
					tags = append(tags, strings.TrimSpace(s))
				}
			}
		} else if strTags, ok := args["tags"].([]string); ok {
			tags = strTags
		}

		eng := ingest.NewEngine(s.vaultRoot, s.engine, daemon.GetDefaultHub())
		note, err := eng.IngestContent(context.Background(), title, []byte(content), ingest.IngestOptions{
			Destination: dest,
			SourceURL:   sourceURL,
			Tags:        tags,
		})
		if err != nil {
			return fmt.Sprintf("Failed to ingest note: %v", err), true
		}

		resBytes, _ := json.MarshalIndent(map[string]interface{}{
			"status":     "success",
			"rel_path":   note.RelPath,
			"title":      note.Title,
			"size_bytes": note.SizeBytes,
			"warning":    note.Warning,
		}, "", "  ")
		return string(resBytes), false

	case "ingest_path":
		path, _ := args["path"].(string)
		path = strings.TrimSpace(path)
		if path == "" {
			return "Parameter 'path' is required.", true
		}

		dest := "inbox"
		if d, ok := args["destination"].(string); ok && strings.TrimSpace(d) != "" {
			dest = strings.TrimSpace(d)
		}

		recursive := true
		if r, ok := args["recursive"].(bool); ok {
			recursive = r
		}

		move := false
		if m, ok := args["move"].(bool); ok {
			move = m
		}

		eng := ingest.NewEngine(s.vaultRoot, s.engine, daemon.GetDefaultHub())
		rep, err := eng.IngestPath(context.Background(), path, ingest.IngestOptions{
			Destination: dest,
			Recursive:   recursive,
			Move:        move,
		})
		if err != nil {
			return fmt.Sprintf("Failed to ingest path: %v", err), true
		}

		resBytes, _ := json.MarshalIndent(rep, "", "  ")
		return string(resBytes), false

	case "ui_get_active_context":
		client, err := daemon.NewClient()
		if err != nil {
			return "{\"surface\": \"none\", \"message\": \"No active daemon or UI connected\"}", false
		}
		defer func() { _ = client.Close() }()
		actCtx, err := client.GetActiveContext()
		if err != nil {
			return fmt.Sprintf("Failed to get active context: %v", err), true
		}
		b, _ := json.MarshalIndent(actCtx, "", "  ")
		return string(b), false

	case "ui_open_note":
		relPath, _ := args["rel_path"].(string)
		if relPath == "" {
			relPath, _ = args["filepath"].(string)
		}
		if relPath == "" {
			return "Parameter 'rel_path' is required.", true
		}

		if !filepath.IsLocal(relPath) || filepath.Clean(relPath) == "." {
			return "Note path must be local to the vault.", true
		}
		client, err := daemon.NewClient()
		if err == nil {
			defer func() { _ = client.Close() }()
			actCtx, cErr := client.GetActiveContext()
			if cErr == nil && actCtx != nil && (actCtx.Surface == daemon.SurfaceWeb || actCtx.Surface == daemon.SurfaceTUI) {
				_ = client.PublishUIEvent(1, relPath, "")
				return fmt.Sprintf("Successfully opened and focused note '%s' on active UI (%s).", relPath, actCtx.Surface), false
			}
		}

		// Fallback: no active UI running, read markdown content directly
		data, readErr := safefile.ReadFile(s.vaultRoot, relPath)
		if readErr != nil {
			return fmt.Sprintf("Note '%s' could not be opened (UI offline and file not found: %v)", relPath, readErr), true
		}
		return fmt.Sprintf("Note opened (no active graphical UI detected; displaying note content directly):\n\n%s", string(data)), false

	case "ui_focus_domain", "ui_focus_sector":
		domain, _ := args["domain"].(string)
		if domain == "" {
			domain, _ = args["sector"].(string)
		}
		if domain == "" {
			return "Parameter 'domain' is required.", true
		}
		client, err := daemon.NewClient()
		if err != nil {
			return "UI is offline (daemon not running).", true
		}
		defer func() { _ = client.Close() }()
		if err := client.PublishUIEvent(2, domain, ""); err != nil {
			return fmt.Sprintf("Failed to focus domain: %v", err), true
		}
		return fmt.Sprintf("Successfully focused domain '%s' on active UI.", domain), false

	case "ui_flash_notification":
		message, _ := args["message"].(string)
		title, _ := args["title"].(string)
		if message == "" {
			return "Parameter 'message' is required.", true
		}
		client, err := daemon.NewClient()
		if err != nil {
			return "UI is offline (daemon not running).", true
		}
		defer func() { _ = client.Close() }()
		payloadBytes, _ := json.Marshal(map[string]string{
			"title":   title,
			"message": message,
		})
		if err := client.PublishUIEvent(3, "", string(payloadBytes)); err != nil {
			return fmt.Sprintf("Failed to flash notification: %v", err), true
		}
		return fmt.Sprintf("Notification flashed on screen: %s", message), false

	case "ui_pin_note":
		relPath, _ := args["rel_path"].(string)
		if relPath == "" {
			return "Parameter 'rel_path' is required.", true
		}
		label, _ := args["label"].(string)
		icon, _ := args["icon"].(string)
		if label == "" {
			base := filepath.Base(relPath)
			label = strings.TrimSuffix(base, filepath.Ext(base))
		}
		if icon == "" {
			icon = "📌"
		}

		client, err := daemon.NewClient()
		if err != nil {
			return fmt.Sprintf("UI is offline (daemon not running). To make this anchor permanent across sessions, configure it in .daneel.yaml under hotbar.pinned_anchors (path: %s, label: %s, icon: %s).", relPath, label, icon), false
		}
		defer func() { _ = client.Close() }()

		payloadBytes, _ := json.Marshal(map[string]string{
			"label": label,
			"icon":  icon,
		})
		if err := client.PublishUIEvent(int32(graphv1.UIActionType_ACTION_PIN_NOTE), relPath, string(payloadBytes)); err != nil {
			return fmt.Sprintf("Failed to pin note on active UI: %v", err), true
		}
		return fmt.Sprintf("Successfully pinned note '%s' (%s %s) to Seldon Vault Anchors on active UI.", relPath, icon, label), false

	case "ui_unpin_note":
		relPath, _ := args["rel_path"].(string)
		if relPath == "" {
			return "Parameter 'rel_path' is required.", true
		}

		client, err := daemon.NewClient()
		if err != nil {
			return "UI is offline (daemon not running). If this note was configured in .daneel.yaml, remove it from hotbar.pinned_anchors.", false
		}
		defer func() { _ = client.Close() }()

		if err := client.PublishUIEvent(int32(graphv1.UIActionType_ACTION_UNPIN_NOTE), relPath, ""); err != nil {
			return fmt.Sprintf("Failed to unpin note on active UI: %v", err), true
		}
		return fmt.Sprintf("Successfully unpinned note '%s' from Seldon Vault Anchors on active UI.", relPath), false

	case "ui_list_pinned_anchors":
		var sb strings.Builder
		sb.WriteString("# ◈ Seldon Vault Anchors (Temporal Hotbar)\n\n")

		cfg := s.cfg
		if cfg == nil {
			if loaded, err := config.LoadConfig(s.vaultRoot, "", false); err == nil {
				cfg = loaded
			}
		}

		if cfg != nil && cfg.Hotbar.WeeklyPattern != "" {
			fmt.Fprintf(&sb, "**Weekly Synthesis Pattern:** `%s`\n\n", cfg.Hotbar.WeeklyPattern)
		} else {
			sb.WriteString("**Weekly Synthesis Pattern:** Default (resolves `weekly/YYYY-WW.md` or fuzzy matching)\n\n")
		}

		hasBackendAnchors := cfg != nil && len(cfg.Hotbar.PinnedAnchors) > 0
		if hasBackendAnchors {
			sb.WriteString("### Declarative Anchors (.daneel.yaml):\n")
			for i, a := range cfg.Hotbar.PinnedAnchors {
				icon := a.Icon
				if icon == "" {
					icon = "◈"
				}
				label := a.Label
				if label == "" {
					label = filepath.Base(a.Path)
				}
				fmt.Fprintf(&sb, "%d. %s **%s** (`%s`)\n", i+1, icon, label, a.Path)
			}
			sb.WriteString("\n")
		} else {
			sb.WriteString("### Declarative Anchors (.daneel.yaml):\n*None configured.*\n\n")
		}

		sb.WriteString("> **Note on Dual-Origin Anchors**: Pinned anchors are merged from declarative config (`.daneel.yaml`) and client-side pins (`localStorage`). Use `ui_pin_note` and `ui_unpin_note` to dynamically adjust anchors in the active UI session.")

		return sb.String(), false

	case "tui_switch_view":
		viewName, _ := args["view_name"].(string)
		if viewName == "" {
			return "Parameter 'view_name' is required.", true
		}

		payload := map[string]interface{}{
			"command": "tui_switch_view",
			"args": map[string]interface{}{
				"view_name": viewName,
			},
		}
		bodyBytes, _ := json.Marshal(payload)
		resp, err := http.Post("http://127.0.0.1:43210/command", "application/json", bytes.NewBuffer(bodyBytes))
		if err != nil {
			return fmt.Sprintf("Failed to contact TUI bridge: %v", err), true
		}
		defer resp.Body.Close() //nolint:errcheck,gosec // fully read or safe to ignore
		return fmt.Sprintf("Successfully switched TUI view to '%s'", viewName), false

	case "tui_open_note":
		filepath, _ := args["filepath"].(string)
		if filepath == "" {
			return "Parameter 'filepath' is required.", true
		}

		payload := map[string]interface{}{
			"command": "tui_open_note",
			"args": map[string]interface{}{
				"filepath": filepath,
			},
		}
		bodyBytes, _ := json.Marshal(payload)
		resp, err := http.Post("http://127.0.0.1:43210/command", "application/json", bytes.NewBuffer(bodyBytes))
		if err != nil {
			return fmt.Sprintf("Failed to contact TUI bridge: %v", err), true
		}
		defer resp.Body.Close() //nolint:errcheck,gosec // fully read or safe to ignore
		return fmt.Sprintf("Successfully opened note '%s' in TUI", filepath), false

	case "tui_get_current_state":
		resp, err := http.Get("http://127.0.0.1:43210/state")
		if err != nil {
			return fmt.Sprintf("Failed to contact TUI bridge: %v", err), true
		}
		defer resp.Body.Close() //nolint:errcheck,gosec // fully read or safe to ignore
		var state map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&state) //nolint:errcheck,gosec
		stateBytes, _ := json.MarshalIndent(state, "", "  ")
		return string(stateBytes), false

	case "tui_highlight_lines":
		startF, _ := args["start"].(float64)
		endF, _ := args["end"].(float64)
		payload := map[string]interface{}{
			"command": "tui_highlight_lines",
			"args": map[string]interface{}{
				"start": startF,
				"end":   endF,
			},
		}
		bodyBytes, _ := json.Marshal(payload)
		resp, err := http.Post("http://127.0.0.1:43210/command", "application/json", bytes.NewBuffer(bodyBytes))
		if err != nil {
			return fmt.Sprintf("Failed to contact TUI bridge: %v", err), true
		}
		defer resp.Body.Close() //nolint:errcheck,gosec // fully read or safe to ignore
		return fmt.Sprintf("Highlighted lines %v to %v", startF, endF), false

	case "tui_flash_notification":
		msgStr, _ := args["message"].(string)
		payload := map[string]interface{}{
			"command": "tui_flash_notification",
			"args": map[string]interface{}{
				"message": msgStr,
			},
		}
		bodyBytes, _ := json.Marshal(payload)
		resp, err := http.Post("http://127.0.0.1:43210/command", "application/json", bytes.NewBuffer(bodyBytes))
		if err != nil {
			return fmt.Sprintf("Failed to contact TUI bridge: %v", err), true
		}
		defer resp.Body.Close() //nolint:errcheck,gosec // fully read or safe to ignore
		return fmt.Sprintf("Flashed notification: %s", msgStr), false

	case "tui_update_widget":
		contentStr, _ := args["content"].(string)
		payload := map[string]interface{}{
			"command": "tui_update_widget",
			"args": map[string]interface{}{
				"content": contentStr,
			},
		}
		bodyBytes, _ := json.Marshal(payload)
		resp, err := http.Post("http://127.0.0.1:43210/command", "application/json", bytes.NewBuffer(bodyBytes))
		if err != nil {
			return fmt.Sprintf("Failed to contact TUI bridge: %v", err), true
		}
		defer resp.Body.Close() //nolint:errcheck,gosec // fully read or safe to ignore
		return "Updated custom widget on dashboard", false

	case "audit_agent_missed_opportunities":
		return s.handleAuditAgentMissedOpportunities(args)

	default:
		return fmt.Sprintf("Tool '%s' not recognized.", name), true
	}
}

func (s *Server) handleAuditAgentMissedOpportunities(args map[string]interface{}) (string, bool) {
	sinceStr := "48h"
	if v, ok := args["since"].(string); ok && v != "" {
		sinceStr = v
	}
	d, err := time.ParseDuration(sinceStr)
	if err != nil {
		return fmt.Sprintf("Invalid duration for since: %v", err), true
	}

	agentFilter := "all"
	if v, ok := args["agent"].(string); ok && v != "" {
		agentFilter = v
	}

	apply := false
	if v, ok := args["apply"].(bool); ok {
		apply = v
	}

	evaluator := auditor.NewAgentEvaluator(s.engine, auditor.AgentEvalOptions{
		Since:       d,
		AgentFilter: agentFilter,
		Apply:       apply,
		DryRun:      !apply,
		VaultRoot:   s.vaultRoot,
	})

	report, err := evaluator.Evaluate(context.Background())
	if err != nil {
		return fmt.Sprintf("Failed to evaluate agent sessions: %v", err), true
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# 🧬 Agent Evals & Telemetry (Last %s) [Agent: %s]\n\n", d.String(), agentFilter)
	fmt.Fprintf(&sb, "- **Scanned Interactions:** %d\n", report.ScannedInteractions)
	fmt.Fprintf(&sb, "- **Suspicious Deltas:** %d\n", report.SuspiciousDeltas)
	fmt.Fprintf(&sb, "- **Missed Opportunities (Agent Failures):** %d\n", report.MissedOpportunities)
	fmt.Fprintf(&sb, "- **Knowledge Gaps (Vault Failures):** %d\n\n", report.KnowledgeGaps)

	if len(report.Results) == 0 {
		sb.WriteString("✨ No missed opportunities or knowledge gaps detected in the given window.\n")
		return sb.String(), false
	}

	var missed []auditor.AgentEvalResult
	var gaps []auditor.AgentEvalResult
	for _, r := range report.Results {
		if r.Classification == auditor.ClassificationMissedOpportunity {
			missed = append(missed, r)
		} else {
			gaps = append(gaps, r)
		}
	}

	if len(missed) > 0 {
		fmt.Fprintf(&sb, "## 🔴 Missed Opportunities (%d)\n\n", len(missed))
		for i, m := range missed {
			fmt.Fprintf(&sb, "### %d. [%s] %s\n", i+1, m.AgentName, m.Doubt)
			fmt.Fprintf(&sb, "- **User Prompt:** %s\n", m.UserPrompt)
			fmt.Fprintf(&sb, "- **Agent Question:** %s\n", m.AgentReply)
			fmt.Fprintf(&sb, "- **Vault Note(s):** %s\n", strings.Join(m.MatchedNotes, ", "))
			fmt.Fprintf(&sb, "- **Recommendation:** %s\n\n", m.SuggestedAction)
		}
	}

	if len(gaps) > 0 {
		fmt.Fprintf(&sb, "## 🟣 Knowledge Gaps (%d)\n\n", len(gaps))
		for i, g := range gaps {
			fmt.Fprintf(&sb, "### %d. [%s] %s\n", i+1, g.AgentName, g.Doubt)
			fmt.Fprintf(&sb, "- **User Prompt:** %s\n", g.UserPrompt)
			fmt.Fprintf(&sb, "- **Agent Question:** %s\n", g.AgentReply)
			fmt.Fprintf(&sb, "- **Recommendation:** %s\n\n", g.SuggestedAction)
		}
	}

	if apply {
		fmt.Fprintf(&sb, "✅ Persisted %d eval findings to vault learnings and metrics.\n", report.AppliedCount)
	}

	return sb.String(), false
}

func (s *Server) sendResult(id interface{}, result interface{}) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	bytes, _ := json.Marshal(resp)
	_, _ = fmt.Fprintln(s.out, string(bytes))
}

func (s *Server) sendError(id interface{}, code int, message string) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
		},
	}
	bytes, _ := json.Marshal(resp)
	_, _ = fmt.Fprintln(s.out, string(bytes))
}
