//nolint:revive // package comment not required
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	ErrNoLocalVault = errors.New("no local .daneel.yaml found in current directory or ancestors") //nolint:revive
)

// VaultMode defines whether Daneel is operating on a local or global vault.
type VaultMode string

const (
	ModeLocal  VaultMode = "LOCAL" //nolint:revive
	ModeGlobal VaultMode = "GLOBAL"
)

// CategoryConfig defines a curated knowledge folder and its MOC description.
type CategoryConfig struct {
	ID    string `yaml:"id" json:"id"`
	Title string `yaml:"title" json:"title"`
	Desc  string `yaml:"desc" json:"desc"`
}

// VaultConfig defines core vault identity, categories, and indexed folders.
type VaultConfig struct {
	Title       string           `yaml:"title,omitempty" json:"title,omitempty"`
	Owner       string           `yaml:"owner,omitempty" json:"owner,omitempty"`
	Role        string           `yaml:"role,omitempty" json:"role,omitempty"`
	Squad       string           `yaml:"squad,omitempty" json:"squad,omitempty"`
	IndexedDirs []string         `yaml:"indexed_dirs,omitempty" json:"indexed_dirs,omitempty"`
	Categories  []CategoryConfig `yaml:"categories,omitempty" json:"categories,omitempty"`
}

// WorkspaceConfig defines local code directories and git parameters.
type WorkspaceConfig struct {
	WorkDir          string `yaml:"work_dir,omitempty" json:"work_dir,omitempty"`
	DefaultGitHost   string `yaml:"default_git_host,omitempty" json:"default_git_host,omitempty"`
	BranchInactivity string `yaml:"branch_inactivity,omitempty" json:"branch_inactivity,omitempty"`
}

// BlockConfig specifies restricted domains and terms for public publishing audits.
type BlockConfig struct {
	Domains  []string `yaml:"domains,omitempty" json:"domains,omitempty"`
	Keywords []string `yaml:"keywords,omitempty" json:"keywords,omitempty"`
}

// GuardConfig is retained as alias/backward compatibility for BlockConfig.
type GuardConfig struct {
	BlockedDomains  []string `yaml:"blocked_domains,omitempty" json:"blocked_domains,omitempty"`
	BlockedKeywords []string `yaml:"blocked_keywords,omitempty" json:"blocked_keywords,omitempty"`
}

// PublishingConfig defines rules for exporting and auditing public notes.
type PublishingConfig struct {
	ContentDir string      `yaml:"content_dir,omitempty" json:"content_dir,omitempty"`
	Block      BlockConfig `yaml:"block,omitempty" json:"block,omitempty"`
}

// ArchetypeConfig defines a staff engineering time classification archetype.
type ArchetypeConfig struct {
	ID          string   `yaml:"id" json:"id"`
	Name        string   `yaml:"name" json:"name"`
	Description string   `yaml:"description" json:"description"`
	Keywords    []string `yaml:"keywords" json:"keywords"`
	TargetMin   float64  `yaml:"target_min,omitempty" json:"target_min,omitempty"`
	TargetMax   float64  `yaml:"target_max,omitempty" json:"target_max,omitempty"`
}

// TelemetryConfig defines daily log tracking rules and taxonomy.
type TelemetryConfig struct {
	Taxonomy []ArchetypeConfig `yaml:"taxonomy,omitempty" json:"taxonomy,omitempty"`
}

// RadarItem represents an active strategic, technical, or friction monitoring topic.
type RadarItem struct {
	Name        string   `yaml:"name" json:"name"`
	Type        string   `yaml:"type" json:"type"` // "business" | "friction" | "tech_radar" | "ai_radar"
	Quadrant    string   `yaml:"quadrant,omitempty" json:"quadrant,omitempty"`
	Ring        string   `yaml:"ring,omitempty" json:"ring,omitempty"`
	Description string   `yaml:"description,omitempty" json:"description,omitempty"`
	Keywords    []string `yaml:"keywords" json:"keywords"`
}

// DreamConfig defines automated sleep cycle consolidation and Zettelkasten routing.
type DreamConfig struct {
	AutoFix bool                `yaml:"auto_fix,omitempty" json:"auto_fix,omitempty"`
	Routing map[string][]string `yaml:"routing,omitempty" json:"routing,omitempty"`
}

// ProfileConfig defines execution command and environment for an AI agent profile.
type ProfileConfig struct {
	Command string            `yaml:"command" json:"command"`
	Env     map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
}

// ChatConfig defines AI agent profiles and default profile.
type ChatConfig struct {
	Default  string                   `yaml:"default,omitempty" json:"default,omitempty"`
	Profiles map[string]ProfileConfig `yaml:"profiles,omitempty" json:"profiles,omitempty"`
}

type GDriveConfig struct { //nolint:revive
	FolderID  string   `yaml:"folder_id,omitempty" json:"folder_id,omitempty"`
	Query     string   `yaml:"query,omitempty" json:"query,omitempty"`
	TargetDir string   `yaml:"target_dir,omitempty" json:"target_dir,omitempty"`
	Formats   []string `yaml:"sync_formats,omitempty" json:"sync_formats,omitempty"`
}

type JiraQuery struct { //nolint:revive
	Name      string `yaml:"name" json:"name"`
	JQL       string `yaml:"jql" json:"jql"`
	TargetDir string `yaml:"target_dir" json:"target_dir"`
}

type JiraConfig struct { //nolint:revive
	Domain  string      `yaml:"domain,omitempty" json:"domain,omitempty"`
	Email   string      `yaml:"email,omitempty" json:"email,omitempty"`
	Queries []JiraQuery `yaml:"queries,omitempty" json:"queries,omitempty"`
}

type HarvestTarget struct { //nolint:revive
	Platform string `yaml:"platform" json:"platform"`
	Org      string `yaml:"org,omitempty" json:"org,omitempty"`
	Group    string `yaml:"group,omitempty" json:"group,omitempty"`
	User     string `yaml:"user,omitempty" json:"user,omitempty"`
	Type     string `yaml:"type,omitempty" json:"type,omitempty"` // "org", "group", "user"
}

// TargetType resolves whether the target is a user or org/group (defaults to org).
func (t HarvestTarget) TargetType() string {
	if strings.EqualFold(t.Type, "user") || t.User != "" {
		return "user"
	}
	return "org"
}

// TargetName resolves the target identifier (User, Group, or Org).
func (t HarvestTarget) TargetName() string {
	if t.User != "" {
		return t.User
	}
	if t.Group != "" {
		return t.Group
	}
	return t.Org
}

// HarvesterConfig configures the external documentation harvesting mechanism.
type HarvesterConfig struct {
	MatchDirectories   []string        `yaml:"match_directories,omitempty" json:"match_directories,omitempty"`
	MatchFilenames     []string        `yaml:"match_filenames,omitempty" json:"match_filenames,omitempty"`
	MatchExtensions    []string        `yaml:"match_extensions,omitempty" json:"match_extensions,omitempty"`
	IgnoredDirectories []string        `yaml:"ignored_directories,omitempty" json:"ignored_directories,omitempty"`
	SkipArchived       *bool           `yaml:"skip_archived,omitempty" json:"skip_archived,omitempty"`
	ExcludeGroups      []string        `yaml:"exclude_groups,omitempty" json:"exclude_groups,omitempty"`
	ExcludeProjects    []string        `yaml:"exclude_projects,omitempty" json:"exclude_projects,omitempty"`
	DownloadAllInDirs  bool            `yaml:"download_all_in_dirs,omitempty" json:"download_all_in_dirs,omitempty"`
	Targets            []HarvestTarget `yaml:"targets,omitempty" json:"targets,omitempty"`
	GoogleDrive        []GDriveConfig  `yaml:"google_drive,omitempty" json:"google_drive,omitempty"`
	Jira               *JiraConfig     `yaml:"jira,omitempty" json:"jira,omitempty"`
}

var defaultIgnoredDirectories = []string{
	"node_modules", "vendor", "wp-content", "dist", "build", "coverage", ".git", ".next",
}

// ShouldSkipArchived returns true by default if not specified.
func (c HarvesterConfig) ShouldSkipArchived() bool {
	if c.SkipArchived != nil {
		return *c.SkipArchived
	}
	return true
}

// GetIgnoredDirectories returns configured or default toxic directories to ignore.
func (c HarvesterConfig) GetIgnoredDirectories() []string {
	if len(c.IgnoredDirectories) > 0 {
		return c.IgnoredDirectories
	}
	return defaultIgnoredDirectories
}

// IsGroupExcluded checks if a namespace or group path matches the configured blocklist.
func (c HarvesterConfig) IsGroupExcluded(groupPath string) bool {
	norm := strings.Trim(strings.ToLower(filepath.ToSlash(groupPath)), "/")
	for _, eg := range c.ExcludeGroups {
		egNorm := strings.Trim(strings.ToLower(filepath.ToSlash(eg)), "/")
		if egNorm == "" {
			continue
		}
		if norm == egNorm || strings.HasPrefix(norm, egNorm+"/") || strings.Contains(norm, "/"+egNorm+"/") || strings.HasSuffix(norm, "/"+egNorm) {
			return true
		}
	}
	return false
}

// IsProjectExcluded checks if a project name or full path matches any configured glob pattern.
func (c HarvesterConfig) IsProjectExcluded(projectName string, fullPath string) bool {
	nameLower := strings.ToLower(projectName)
	pathLower := strings.ToLower(filepath.ToSlash(fullPath))
	baseName := filepath.Base(pathLower)

	for _, pattern := range c.ExcludeProjects {
		patLower := strings.ToLower(pattern)
		if matched, _ := filepath.Match(patLower, nameLower); matched {
			return true
		}
		if matched, _ := filepath.Match(patLower, baseName); matched {
			return true
		}
		for _, part := range strings.Split(pathLower, "/") {
			if part == "" {
				continue
			}
			if matched, _ := filepath.Match(patLower, part); matched {
				return true
			}
		}
		if patLower == nameLower || patLower == baseName {
			return true
		}
	}
	return false
}

// EvaluationConfig defines rules for generating performance review dossiers.
type EvaluationConfig struct {
	DossierTitle        string   `yaml:"dossier_title,omitempty" json:"dossier_title,omitempty"`
	StrategicDeliveries []string `yaml:"strategic_deliveries,omitempty" json:"strategic_deliveries,omitempty"`
}

// TuiTabsConfig defines the labels for TUI tabs.
type TuiTabsConfig struct {
	Search    string `yaml:"search,omitempty" json:"search,omitempty"`
	Dashboard string `yaml:"dashboard,omitempty" json:"dashboard,omitempty"`
	Workspace string `yaml:"workspace,omitempty" json:"workspace,omitempty"`
	People    string `yaml:"people,omitempty" json:"people,omitempty"`
	Routines  string `yaml:"routines,omitempty" json:"routines,omitempty"`
	Skills    string `yaml:"skills,omitempty" json:"skills,omitempty"`
}

// TuiConfig defines the TUI customization.
type TuiConfig struct {
	Tabs TuiTabsConfig `yaml:"tabs,omitempty" json:"tabs,omitempty"`
}

// PinnedAnchorConfig defines an anchored note in the hotbar.
type PinnedAnchorConfig struct {
	Path  string `yaml:"path" json:"path"`
	Label string `yaml:"label,omitempty" json:"label,omitempty"`
	Icon  string `yaml:"icon,omitempty" json:"icon,omitempty"`
}

// HotbarConfig defines customization for the temporal hotbar and quick anchors.
type HotbarConfig struct {
	WeeklyPattern string               `yaml:"weekly_pattern,omitempty" json:"weekly_pattern,omitempty"`
	PinnedAnchors []PinnedAnchorConfig `yaml:"pinned_anchors,omitempty" json:"pinned_anchors,omitempty"`
}

// RoutineItemConfig defines configuration for an individual scheduled routine.
type RoutineItemConfig struct {
	Enabled     bool     `yaml:"enabled" json:"enabled"`
	Schedule    string   `yaml:"schedule" json:"schedule"`
	Command     string   `yaml:"command" json:"command"`
	Args        []string `yaml:"args,omitempty" json:"args,omitempty"`
	Description string   `yaml:"description,omitempty" json:"description,omitempty"`
}

// RoutinesConfig defines scheduled background routines managed via systemd user timers.
type RoutinesConfig struct {
	Driver string                       `yaml:"driver,omitempty" json:"driver,omitempty"` // "systemd" (default)
	Items  map[string]RoutineItemConfig `yaml:"items,omitempty" json:"items,omitempty"`
}

// DaemonServeConfig defines HTTP web visualizer options for the daemon.
type DaemonServeConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Port    int    `yaml:"port,omitempty" json:"port,omitempty"`
	Host    string `yaml:"host,omitempty" json:"host,omitempty"`
}

// DaemonConfig defines the background daemon and unified multi-listener settings.
type DaemonConfig struct {
	SocketPath string            `yaml:"socket_path,omitempty" json:"socket_path,omitempty"`
	Serve      DaemonServeConfig `yaml:"serve,omitempty" json:"serve,omitempty"`
}

// Config holds runtime configuration for Daneel.
type Config struct {
	VaultPath  string           `yaml:"vault_path,omitempty" json:"vault_path,omitempty"`
	DBPath     string           `yaml:"db_path,omitempty" json:"db_path,omitempty"`
	Vault      VaultConfig      `yaml:"vault,omitempty" json:"vault,omitempty"`
	Workspace  WorkspaceConfig  `yaml:"workspace,omitempty" json:"workspace,omitempty"`
	Publishing PublishingConfig `yaml:"publishing,omitempty" json:"publishing,omitempty"`
	Telemetry  TelemetryConfig  `yaml:"telemetry,omitempty" json:"telemetry,omitempty"`
	Radar      []RadarItem      `yaml:"radar,omitempty" json:"radar,omitempty"`
	Dream      DreamConfig      `yaml:"dream,omitempty" json:"dream,omitempty"`
	Chat       ChatConfig       `yaml:"chat,omitempty" json:"chat,omitempty"`
	Harvester  HarvesterConfig  `yaml:"harvester,omitempty" json:"harvester,omitempty"`
	Evaluation EvaluationConfig `yaml:"evaluation,omitempty" json:"evaluation,omitempty"`
	Tui        TuiConfig        `yaml:"tui,omitempty" json:"tui,omitempty"`
	Hotbar     HotbarConfig     `yaml:"hotbar,omitempty" json:"hotbar,omitempty"`
	Daemon     DaemonConfig     `yaml:"daemon,omitempty" json:"daemon,omitempty"`
	Routines   RoutinesConfig   `yaml:"routines,omitempty" json:"routines,omitempty"`

	// Legacy Top-Level Fields (Mapped automatically for backward compatibility)
	LegacyAssistant   ChatConfig          `yaml:"assistant,omitempty" json:"-"`
	LegacyIndexedDirs []string            `yaml:"indexed_dirs,omitempty" json:"-"`
	LegacyTaxonomy    map[string][]string `yaml:"taxonomy,omitempty" json:"-"`
	LegacyGuard       GuardConfig         `yaml:"guard,omitempty" json:"-"`

	// Runtime metadata
	Mode         VaultMode `yaml:"-" json:"mode"`
	VaultSource  string    `yaml:"-" json:"vault_source"`
	ConfigSource string    `yaml:"-" json:"config_source"`
	ConfigFile   string    `yaml:"-" json:"config_file"`
	IsGlobal     bool      `yaml:"-" json:"is_global"`
}

// expandHome expands a leading tilde (~) to the user's home directory.
func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			if path == "~" {
				return home
			}
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// XDGConfigDir returns the XDG compliant configuration directory for Daneel (~/.config/daneel).
func XDGConfigDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(expandHome(xdg), "daneel")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "daneel")
}

// XDGCacheDir returns the XDG compliant cache directory for Daneel (~/.cache/daneel).
func XDGCacheDir() string {
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(expandHome(xdg), "daneel")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "daneel")
}

// XDGDataDir returns the XDG compliant data directory for Daneel (~/.local/share/daneel).
func XDGDataDir() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(expandHome(xdg), "daneel")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "daneel")
}

// DefaultGlobalVaultPath returns the fallback global vault path (~/Documents/second-brain).
func DefaultGlobalVaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Documents", "second-brain")
}

// GlobalConfigFile returns the path to the global XDG configuration file (~/.config/daneel/config.yaml).
func GlobalConfigFile() string {
	return filepath.Join(XDGConfigDir(), "config.yaml")
}

// DefaultCategories returns the standard OKF knowledge categories.
func DefaultCategories() []CategoryConfig {
	return []CategoryConfig{
		{ID: "arquitetura", Title: "Software Architecture", Desc: "C4 model, DDD, hexagonal architecture, microservices, sagas, caching, and resilience"},
		{ID: "dados", Title: "Data & Messaging", Desc: "Elasticsearch, sharding, CDC, Apache Beam, Protobuf, CBOR, and gRPC"},
		{ID: "engenharia", Title: "Software Engineering", Desc: "Clean Code, SOLID, 12-Factor App, design patterns, and technical debt"},
		{ID: "ferramentas", Title: "Tooling & Productivity", Desc: "Neovim, Emacs, Tmux, Git, CLI Tools, Taskfile, and Mise"},
		{ID: "go", Title: "Golang", Desc: "Runtime, memory arenas, struct padding, profiling, SDK facade, and concurrency"},
		{ID: "ia", Title: "Artificial Intelligence", Desc: "RAG, LLMs, autonomous agents, MCP, prompts, SPDD, and workflows"},
		{ID: "infra", Title: "Infrastructure & SRE", Desc: "Kubernetes, KEDA, Docker, Linux, SystemD, BTRFS, and Terraform"},
		{ID: "linguagens", Title: "Programming Languages", Desc: "Rust, Bash, Lua, Haskell, Scala, Java, Clojure, and PHP"},
		{ID: "observabilidade", Title: "Observability", Desc: "OpenTelemetry, Prometheus, distributed tracing, and 4 Key Metrics"},
		{ID: "organizacao", Title: "Organization & Tribes", Desc: "Squad topologies, engineering epics, and company initiatives"},
		{ID: "pessoas", Title: "People & Network", Desc: "1:1s history, mentorships, peer relationships, and team profiles"},
		{ID: "carreira", Title: "Career & Leadership", Desc: "Staff engineering, goals, evaluations, career frameworks, and personal growth"},
		{ID: "processors", Title: "Processes & Methodologies", Desc: "RFCs, PRDs, code reviews, agile methodologies, and GTD"},
		{ID: "seguranca", Title: "Security & Networking", Desc: "Real IP rate limiting, SSL/TLS, SPIFFE, SSH Agent, and CVE management"},
		{ID: "sistemas", Title: "Systems & Services", Desc: "Local repositories, API catalogs, and microservices"},
	}
}

// DefaultTaxonomyArchetypes returns the standard Staff Engineering archetypes.
func DefaultTaxonomyArchetypes() []ArchetypeConfig {
	return []ArchetypeConfig{
		{
			ID:          "A_Estrategico",
			Name:        "Strategic & Architecture",
			Description: "Technical vision, RFCs, product strategy, and long-term initiatives",
			Keywords:    []string{"rfc", "arquitetura", "architecture", "visao", "vision", "longo prazo", "long-term", "estrategia", "strategy", "plataforma", "platform"},
			TargetMin:   30.0,
		},
		{
			ID:          "B_ImpactoLocal",
			Name:        "Local Impact & Execution",
			Description: "Bug fixes, PRs, refactors, SDKs, PoCs, and direct team support",
			Keywords:    []string{"fix", "bug", "pentest", "rate limit", "sdk", "mr", "pr", "poc", "refactor"},
			TargetMin:   30.0,
		},
		{
			ID:          "C_GlueWork",
			Name:        "Glue Work & Leadership",
			Description: "1:1s, mentorship, organizational alignments, code reviews, and interviews",
			Keywords:    []string{"1:1", "mentoria", "mentorship", "alinhamento", "alignment", "code review", "facilita", "entrevista", "interview"},
			TargetMax:   30.0,
		},
		{
			ID:          "E_Overhead",
			Name:        "Overhead & Ceremonies",
			Description: "Company-wide all-hands, institutional meetings, Slack and email communications",
			Keywords:    []string{"all-hands", "bimestral", "institucional", "slack", "email"},
			TargetMax:   10.0,
		},
	}
}

// DefaultRadarItems returns standard strategic monitoring radar items.
func DefaultRadarItems() []RadarItem {
	return []RadarItem{
		{
			Name:        "Plataforma de Agentes",
			Type:        "business",
			Description: "Autonomous agents and engineering copilots platform via MCP",
			Keywords:    []string{"agentes", "agents", "mcp", "jarvis", "daneel", "llm platform"},
		},
		{
			Name:        "Dívida Técnica & Fricção",
			Type:        "friction",
			Description: "Early detection of operational bottlenecks, defects, and security issues",
			Keywords:    []string{"pentest", "rate limit", "resiliencia", "resilience", "ofensores", "cve", "seguranca", "security", "incidente", "incident", "bug", "bloqueio", "blocker"},
		},
	}
}

// DefaultRoutingMap returns standard Zettelkasten category routing rules.
func DefaultRoutingMap() map[string][]string {
	return map[string][]string{
		"notes/engineering": {"architecture", "design", "patterns", "api", "service"},
		"notes/research":    {"agent", "mcp", "llm", "rag", "prompt"},
		"notes/career":      {"goals", "feedback", "growth", "1:1"},
		"notes/people":      {"peer", "manager", "mentee", "leadership"},
		"notes/processes":   {"rfc", "prd", "sla", "postmortem"},
	}
}

// DefaultChatConfig returns standard AI agent profiles.
func DefaultChatConfig() ChatConfig {
	return ChatConfig{
		Default: "agy",
		Profiles: map[string]ProfileConfig{
			"agy": {
				Command: "agy",
				Env: map[string]string{
					"TERM": "xterm-256color",
				},
			},
			"claude": {
				Command: "claude",
			},
		},
	}
}

// DefaultHarvesterConfig returns the default configuration for remote documentation harvesting.
func DefaultHarvesterConfig() HarvesterConfig {
	defaultSkipArchived := true
	return HarvesterConfig{
		MatchDirectories: []string{
			"docs", "doc", "adrs", "adr", "architecture", "decisions",
			"runbooks", "playbooks", "specs", "spec", "rfcs", "rfc", "catalog-info",
		},
		MatchFilenames: []string{
			"readme.md", "readme.org", "architecture.md", "contributing.md",
			"catalog-info.yaml", "catalog-info.yml", "agents.md", "claude.md", "gemini.md",
		},
		MatchExtensions: []string{
			".md", ".org",
		},
		IgnoredDirectories: defaultIgnoredDirectories,
		SkipArchived:       &defaultSkipArchived,
		DownloadAllInDirs:  true,
	}
}

// DefaultTuiConfig returns the default configuration for the TUI.
func DefaultTuiConfig() TuiConfig {
	return TuiConfig{
		Tabs: TuiTabsConfig{
			Search:    "1: 🔍 Search",
			Dashboard: "2: 📊 Dashboard",
			Workspace: "3: 🗺️ Workspace",
			People:    "4: 🤝 People",
			Routines:  "5: 🌙 Routines",
			Skills:    "6: ⚡ Skills",
		},
	}
}

// GetHarvesterConfig returns the configured or default harvester configuration.
func (c *Config) GetHarvesterConfig() HarvesterConfig {
	def := DefaultHarvesterConfig()
	if len(c.Harvester.MatchDirectories) > 0 {
		def.MatchDirectories = c.Harvester.MatchDirectories
	}
	if len(c.Harvester.MatchFilenames) > 0 {
		def.MatchFilenames = c.Harvester.MatchFilenames
	}
	if len(c.Harvester.MatchExtensions) > 0 {
		def.MatchExtensions = c.Harvester.MatchExtensions
	}
	if len(c.Harvester.IgnoredDirectories) > 0 {
		def.IgnoredDirectories = c.Harvester.IgnoredDirectories
	}
	if c.Harvester.SkipArchived != nil {
		def.SkipArchived = c.Harvester.SkipArchived
	}
	if len(c.Harvester.ExcludeGroups) > 0 {
		def.ExcludeGroups = c.Harvester.ExcludeGroups
	}
	if len(c.Harvester.ExcludeProjects) > 0 {
		def.ExcludeProjects = c.Harvester.ExcludeProjects
	}
	if c.Harvester.DownloadAllInDirs || len(c.Harvester.MatchDirectories) > 0 {
		def.DownloadAllInDirs = c.Harvester.DownloadAllInDirs
	}
	if len(c.Harvester.GoogleDrive) > 0 {
		def.GoogleDrive = c.Harvester.GoogleDrive
	}
	if len(c.Harvester.Targets) > 0 {
		def.Targets = c.Harvester.Targets
	}
	if c.Harvester.Jira != nil {
		def.Jira = c.Harvester.Jira
	}
	return def
}

// GetTuiConfig returns the configured or default TUI configuration.
func (c *Config) GetTuiConfig() TuiConfig {
	def := DefaultTuiConfig()
	if c.Tui.Tabs.Search != "" {
		def.Tabs.Search = c.Tui.Tabs.Search
	}
	if c.Tui.Tabs.Dashboard != "" {
		def.Tabs.Dashboard = c.Tui.Tabs.Dashboard
	}
	if c.Tui.Tabs.Workspace != "" {
		def.Tabs.Workspace = c.Tui.Tabs.Workspace
	}
	if c.Tui.Tabs.People != "" {
		def.Tabs.People = c.Tui.Tabs.People
	}
	if c.Tui.Tabs.Routines != "" {
		def.Tabs.Routines = c.Tui.Tabs.Routines
	}
	if c.Tui.Tabs.Skills != "" {
		def.Tabs.Skills = c.Tui.Tabs.Skills
	}
	return def
}

// DefaultRoutinesConfig returns the default background routines.
func DefaultRoutinesConfig() RoutinesConfig {
	return RoutinesConfig{
		Driver: "systemd",
		Items: map[string]RoutineItemConfig{
			"tidy": {
				Enabled:     true,
				Schedule:    "hourly",
				Command:     "tidy",
				Description: "Deterministic link self-healing and MOC reconciliation",
			},
			"harvest": {
				Enabled:     true,
				Schedule:    "0 */6 * * *",
				Command:     "harvest",
				Description: "Harvest remote documentation from GitLab/GitHub",
			},
			"doctor": {
				Enabled:     true,
				Schedule:    "*-*-* 06:00:00",
				Command:     "doctor",
				Description: "Daily vault health audit and OKF compliance check",
			},
		},
	}
}

// DefaultDaemonConfig returns default configuration for the daemon.
func DefaultDaemonConfig() DaemonConfig {
	return DaemonConfig{
		Serve: DaemonServeConfig{
			Enabled: false,
			Port:    4321,
			Host:    "127.0.0.1",
		},
	}
}

// GetRoutinesConfig returns the configured or default routines configuration.
func (c *Config) GetRoutinesConfig() RoutinesConfig {
	def := DefaultRoutinesConfig()
	if c.Routines.Driver != "" {
		def.Driver = c.Routines.Driver
	}
	if c.Routines.Items != nil {
		for name, item := range c.Routines.Items {
			def.Items[name] = item
		}
	}
	return def
}

// SaveRoutine updates or inserts a routine into the config and writes to ConfigFile if present.
func (c *Config) SaveRoutine(r RoutineItemConfig, name string) error {
	if c.Routines.Items == nil {
		c.Routines.Items = make(map[string]RoutineItemConfig)
	}
	c.Routines.Items[name] = r

	if c.ConfigFile == "" {
		if c.VaultPath != "" {
			c.ConfigFile = filepath.Join(c.VaultPath, ".daneel.yaml")
		} else {
			c.ConfigFile = filepath.Join(XDGConfigDir(), "config.yaml")
		}
	}
	_ = os.MkdirAll(filepath.Dir(c.ConfigFile), 0750)

	data, err := os.ReadFile(c.ConfigFile)
	var rawMap map[string]interface{}
	if err == nil {
		_ = yaml.Unmarshal(data, &rawMap)
	}
	if rawMap == nil {
		rawMap = make(map[string]interface{})
	}

	routinesRaw, ok := rawMap["routines"].(map[string]interface{})
	if !ok {
		routinesRaw = make(map[string]interface{})
		rawMap["routines"] = routinesRaw
	}
	itemsRaw, ok := routinesRaw["items"].(map[string]interface{})
	if !ok {
		itemsRaw = make(map[string]interface{})
		routinesRaw["items"] = itemsRaw
	}

	itemsRaw[name] = map[string]interface{}{
		"enabled":     r.Enabled,
		"schedule":    r.Schedule,
		"command":     r.Command,
		"args":        r.Args,
		"description": r.Description,
	}

	out, err := yaml.Marshal(rawMap)
	if err != nil {
		return err
	}
	return os.WriteFile(c.ConfigFile, out, 0600)
}

// DeleteRoutine removes a routine from the config and updates ConfigFile if present.
func (c *Config) DeleteRoutine(name string) error {
	if c.Routines.Items != nil {
		delete(c.Routines.Items, name)
	}

	if c.ConfigFile == "" {
		if c.VaultPath != "" {
			c.ConfigFile = filepath.Join(c.VaultPath, ".daneel.yaml")
		} else {
			c.ConfigFile = filepath.Join(XDGConfigDir(), "config.yaml")
		}
	}
	_ = os.MkdirAll(filepath.Dir(c.ConfigFile), 0750)

	data, err := os.ReadFile(c.ConfigFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var rawMap map[string]interface{}
	if err := yaml.Unmarshal(data, &rawMap); err != nil {
		return err
	}

	if routinesRaw, ok := rawMap["routines"].(map[string]interface{}); ok {
		if itemsRaw, ok := routinesRaw["items"].(map[string]interface{}); ok {
			delete(itemsRaw, name)
		}
	}

	out, err := yaml.Marshal(rawMap)
	if err != nil {
		return err
	}
	return os.WriteFile(c.ConfigFile, out, 0600)
}

// GetDaemonConfig returns the configured or default daemon configuration.
func (c *Config) GetDaemonConfig() DaemonConfig {
	def := DefaultDaemonConfig()
	if c.Daemon.SocketPath != "" {
		def.SocketPath = c.Daemon.SocketPath
	}
	if c.Daemon.Serve.Enabled {
		def.Serve.Enabled = true
	}
	if c.Daemon.Serve.Port != 0 {
		def.Serve.Port = c.Daemon.Serve.Port
	}
	if c.Daemon.Serve.Host != "" {
		def.Serve.Host = c.Daemon.Serve.Host
	}
	return def
}

// DefaultIndexedDirs returns the default directories to index.
func DefaultIndexedDirs() []string {
	return []string{
		"notes",
		"sources",
		"meta",
		"daily",
		"inbox",
		"public/content",
	}
}

// GetIndexedDirs returns the configured or fallback indexed directories.
func (c *Config) GetIndexedDirs() []string {
	if len(c.Vault.IndexedDirs) > 0 {
		return c.Vault.IndexedDirs
	}
	if len(c.LegacyIndexedDirs) > 0 {
		return c.LegacyIndexedDirs
	}
	return DefaultIndexedDirs()
}

// GetCategories returns the configured or fallback categories.
func (c *Config) GetCategories() []CategoryConfig {
	if len(c.Vault.Categories) > 0 {
		return c.Vault.Categories
	}
	return DefaultCategories()
}

// GetWorkDir returns the configured or default ~/Work directory.
func (c *Config) GetWorkDir() string {
	if c.Workspace.WorkDir != "" {
		return expandHome(c.Workspace.WorkDir)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Work")
}

// GetDefaultGitHost returns the configured git host or defaults to github.com.
func (c *Config) GetDefaultGitHost() string {
	if c.Workspace.DefaultGitHost != "" {
		return c.Workspace.DefaultGitHost
	}
	return "github.com"
}

// GetBranchInactivityThreshold returns the duration before a clean feature branch is reset to main/master.
// Defaults to 48 hours.
func (c *Config) GetBranchInactivityThreshold() time.Duration {
	if c != nil && c.Workspace.BranchInactivity != "" {
		if d, err := ParseDuration(c.Workspace.BranchInactivity); err == nil && d > 0 {
			return d
		}
	}
	return 48 * time.Hour
}

// ParseDuration parses duration strings supporting standard Go units (h, m, s) and day suffixes (e.g. "2d", "48h").
func ParseDuration(val string) (time.Duration, error) {
	val = strings.TrimSpace(val)
	if val == "" {
		return 0, fmt.Errorf("empty duration")
	}
	if strings.HasSuffix(val, "d") || strings.HasSuffix(val, "D") {
		daysStr := strings.TrimSuffix(strings.TrimSuffix(val, "d"), "D")
		days, err := strconv.Atoi(daysStr)
		if err != nil {
			return 0, fmt.Errorf("invalid day duration %q: %w", val, err)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(val)
}

// GetContentDir returns the public publishing content directory.
func (c *Config) GetContentDir() string {
	if c.Publishing.ContentDir != "" {
		return c.Publishing.ContentDir
	}
	return "public/content"
}

// GetBlockedDomains returns blocked domains from publishing.block or legacy guard.
func (c *Config) GetBlockedDomains() []string {
	if len(c.Publishing.Block.Domains) > 0 {
		return c.Publishing.Block.Domains
	}
	if len(c.LegacyGuard.BlockedDomains) > 0 {
		return c.LegacyGuard.BlockedDomains
	}
	return nil
}

// GetBlockedKeywords returns blocked keywords from publishing.block or legacy guard.
func (c *Config) GetBlockedKeywords() []string {
	if len(c.Publishing.Block.Keywords) > 0 {
		return c.Publishing.Block.Keywords
	}
	if len(c.LegacyGuard.BlockedKeywords) > 0 {
		return c.LegacyGuard.BlockedKeywords
	}
	return nil
}

// GetTaxonomyMap returns the archetype map (ID -> []keywords).
func (c *Config) GetTaxonomyMap() map[string][]string {
	res := make(map[string][]string)

	if len(c.Telemetry.Taxonomy) > 0 {
		for _, arch := range c.Telemetry.Taxonomy {
			res[arch.ID] = arch.Keywords
		}
		return res
	}

	if len(c.LegacyTaxonomy) > 0 {
		return c.LegacyTaxonomy
	}

	for _, arch := range DefaultTaxonomyArchetypes() {
		res[arch.ID] = arch.Keywords
	}
	return res
}

// GetTaxonomyArchetypes returns the structured archetype list.
func (c *Config) GetTaxonomyArchetypes() []ArchetypeConfig {
	if len(c.Telemetry.Taxonomy) > 0 {
		return c.Telemetry.Taxonomy
	}
	return DefaultTaxonomyArchetypes()
}

// GetRadarItems returns the radar items, optionally filtered by type.
func (c *Config) GetRadarItems(radarType string) []RadarItem {
	items := c.Radar
	if len(items) == 0 {
		items = DefaultRadarItems()
	}
	if radarType == "" || radarType == "all" {
		return items
	}
	var filtered []RadarItem
	for _, it := range items {
		if strings.EqualFold(it.Type, radarType) {
			filtered = append(filtered, it)
		}
	}
	return filtered
}

// GetFrictionKeywords returns friction keywords configured in radar or fallback defaults.
func (c *Config) GetFrictionKeywords() []string {
	for _, it := range c.GetRadarItems("friction") {
		if len(it.Keywords) > 0 {
			return it.Keywords
		}
	}
	return []string{
		"incidente", "bug", "falha", "bloqueio", "lento", "lentidao",
		"reteste", "gargalo", "defeito", "cve", "vulnerabilidade", "suporte",
	}
}

// GetInitiativesMap returns a map of initiative name to keywords.
func (c *Config) GetInitiativesMap() map[string][]string {
	res := make(map[string][]string)
	for _, it := range c.GetRadarItems("business") {
		res[it.Name] = it.Keywords
	}
	if len(res) == 0 {
		for _, it := range DefaultRadarItems() {
			if it.Type == "business" {
				res[it.Name] = it.Keywords
			}
		}
	}
	return res
}

// GetRoutingMap returns the Zettelkasten category routing map.
func (c *Config) GetRoutingMap() map[string][]string {
	if len(c.Dream.Routing) > 0 {
		return c.Dream.Routing
	}
	return DefaultRoutingMap()
}

// GetChatProfile resolves a profile by name (or default) and returns ProfileConfig and profile name.
func (c *Config) GetChatProfile(name string) (ProfileConfig, string, error) {
	profiles := c.Chat.Profiles
	if len(profiles) == 0 && len(c.LegacyAssistant.Profiles) > 0 {
		profiles = c.LegacyAssistant.Profiles
	}

	if len(profiles) == 0 {
		profiles = DefaultChatConfig().Profiles
	}

	target := name
	if target == "" {
		target = c.Chat.Default
		if target == "" {
			target = c.LegacyAssistant.Default
		}
		if target == "" {
			target = "agy"
		}
	}

	if prof, ok := profiles[target]; ok {
		return prof, target, nil
	}

	// Fallback to first available profile
	for k, v := range profiles {
		return v, k, nil
	}

	return ProfileConfig{}, "", errors.New("no chat profile configured")
}

// ResolveVaultCacheDir returns the cache directory for a given vault without polluting $HOME.
func ResolveVaultCacheDir(vaultRoot string) string {
	if vaultRoot != "" {
		localDaneel := filepath.Join(vaultRoot, ".daneel", "cache")
		if stat, err := os.Stat(localDaneel); err == nil && stat.IsDir() {
			return localDaneel
		}
		localAgents := filepath.Join(vaultRoot, ".agents", "cache")
		if stat, err := os.Stat(localAgents); err == nil && stat.IsDir() {
			return localAgents
		}
		if _, err := os.Stat(filepath.Join(vaultRoot, ".daneel.yaml")); err == nil {
			return localDaneel
		}
	}
	xdg := XDGCacheDir()
	return xdg
}

// FindLocalVault searches for a local vault marked by .daneel.yaml walking upwards from startDir.
func FindLocalVault(startDir string) (vaultRoot string, configPath string, err error) {
	if startDir == "" {
		startDir, err = os.Getwd()
		if err != nil {
			return "", "", err
		}
	}

	absDir, err := filepath.Abs(expandHome(startDir))
	if err != nil {
		return "", "", err
	}

	curr := absDir
	for {
		candidate1 := filepath.Join(curr, ".daneel.yaml")
		if stat, err := os.Stat(candidate1); err == nil && !stat.IsDir() {
			return curr, candidate1, nil
		}
		candidate2 := filepath.Join(curr, ".daneel.yml")
		if stat, err := os.Stat(candidate2); err == nil && !stat.IsDir() {
			return curr, candidate2, nil
		}

		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}

	return "", "", ErrNoLocalVault
}

// LoadConfig resolves configuration according to strict industry standard hierarchy.
func LoadConfig(customVaultPath string, customConfigPath string, forceGlobal bool) (*Config, error) {
	var vaultRoot string
	var configFile string
	var dbPath string
	mode := ModeGlobal
	vaultSource := "DEFAULT"
	configSource := "NONE"

	// 1. Explicit CLI flag for vault path overrides everything
	if customVaultPath != "" {
		vaultRoot = expandHome(customVaultPath)
		vaultSource = "FLAG (--vault-path)"
	}

	// 2. Explicit Environment Variable
	if vaultRoot == "" {
		if envVault := os.Getenv("DANEEL_VAULT_PATH"); envVault != "" {
			vaultRoot = expandHome(envVault)
			vaultSource = "ENV (DANEEL_VAULT_PATH)"
		}
	}

	// 3. Check for Local Vault (if not forced to global)
	if vaultRoot == "" && !forceGlobal {
		localRoot, localCfg, err := FindLocalVault("")
		if err == nil && localRoot != "" {
			vaultRoot = localRoot
			configFile = localCfg
			mode = ModeLocal
			vaultSource = "LOCAL_VAULT (.daneel.yaml)"
			configSource = "LOCAL_CONFIG (.daneel.yaml)"
		}
	}

	// 4. Resolve Global Vault if no local vault is active
	if vaultRoot == "" {
		mode = ModeGlobal

		// 4.1 Check global XDG config file (~/.config/daneel/config.yaml)
		globalCfgPath := GlobalConfigFile()
		if customConfigPath != "" {
			globalCfgPath = expandHome(customConfigPath)
		} else if envCfg := os.Getenv("DANEEL_CONFIG"); envCfg != "" {
			globalCfgPath = expandHome(envCfg)
		}

		if _, err := os.Stat(globalCfgPath); err == nil { //nolint:gosec // intentional
			configFile = globalCfgPath
			configSource = "GLOBAL_XDG (~/.config/daneel/config.yaml)"
			data, err := os.ReadFile(globalCfgPath) //nolint:gosec // intentional
			if err == nil {
				var gCfg Config
				if err := yaml.Unmarshal(data, &gCfg); err == nil {
					if vaultRoot == "" && gCfg.VaultPath != "" {
						vaultRoot = expandHome(gCfg.VaultPath)
						vaultSource = "GLOBAL_CONFIG (vault_path)"
					}
					if gCfg.DBPath != "" {
						dbPath = expandHome(gCfg.DBPath)
					}
				}
			}
		}

		// 4.2 Default Fallback
		if vaultRoot == "" {
			vaultRoot = DefaultGlobalVaultPath()
			vaultSource = "XDG_DEFAULT (~/Documents/second-brain)"
		}
	}

	absVault, _ := filepath.Abs(vaultRoot)

	// 5. Resolve Config File from explicit flags, env, or vault root
	if customConfigPath != "" {
		configFile = expandHome(customConfigPath)
		configSource = "FLAG (--config)"
	} else if envCfg := os.Getenv("DANEEL_CONFIG"); envCfg != "" {
		configFile = expandHome(envCfg)
		configSource = "ENV (DANEEL_CONFIG)"
	} else if configFile == "" && !forceGlobal && absVault != "" {
		// Check for local vault configuration in vault root
		candidate1 := filepath.Join(absVault, ".daneel.yaml")
		candidate2 := filepath.Join(absVault, ".daneel.yml")
		if stat, err := os.Stat(candidate1); err == nil && !stat.IsDir() {
			configFile = candidate1
			configSource = "LOCAL_CONFIG (.daneel.yaml)"
			mode = ModeLocal
		} else if stat, err := os.Stat(candidate2); err == nil && !stat.IsDir() {
			configFile = candidate2
			configSource = "LOCAL_CONFIG (.daneel.yml)"
			mode = ModeLocal
		}
	}

	// If still no config file found and not in local mode, check global config
	if configFile == "" {
		globalCfgPath := GlobalConfigFile()
		if stat, err := os.Stat(globalCfgPath); err == nil && !stat.IsDir() {
			configFile = globalCfgPath
			configSource = "GLOBAL_XDG (~/.config/daneel/config.yaml)"
		}
	}

	if forceGlobal {
		mode = ModeGlobal
	}

	// 6. Default Database Path
	if dbPath == "" {
		if envDB := os.Getenv("DANEEL_DB_PATH"); envDB != "" {
			dbPath = expandHome(envDB)
		} else {
			// Resolve vault-isolated cache if .daneel/cache exists or is marked by .daneel.yaml,
			// otherwise fall back to standard XDG Cache (~/.cache/daneel/vault_index.db)
			cacheDir := ResolveVaultCacheDir(absVault)
			dbPath = filepath.Join(cacheDir, "vault_index.db")
		}
	}

	cfg := &Config{
		VaultPath:    absVault,
		DBPath:       dbPath,
		Vault:        VaultConfig{IndexedDirs: DefaultIndexedDirs(), Categories: DefaultCategories()},
		Workspace:    WorkspaceConfig{WorkDir: "~/Work", DefaultGitHost: "github.com"},
		Publishing:   PublishingConfig{ContentDir: "public/content"},
		Telemetry:    TelemetryConfig{Taxonomy: DefaultTaxonomyArchetypes()},
		Radar:        DefaultRadarItems(),
		Dream:        DreamConfig{AutoFix: true, Routing: DefaultRoutingMap()},
		Chat:         DefaultChatConfig(),
		Harvester:    DefaultHarvesterConfig(),
		Tui:          DefaultTuiConfig(),
		Daemon:       DefaultDaemonConfig(),
		Routines:     DefaultRoutinesConfig(),
		Mode:         mode,
		VaultSource:  vaultSource,
		ConfigSource: configSource,
		ConfigFile:   configFile,
		IsGlobal:     (mode == ModeGlobal),
	}

	// 5. Load file configuration if present (Local or Global)
	if configFile != "" {
		data, err := os.ReadFile(configFile) //nolint:gosec // intentional
		if err == nil {
			var fileCfg Config
			if err := yaml.Unmarshal(data, &fileCfg); err == nil {
				if fileCfg.DBPath != "" && os.Getenv("DANEEL_DB_PATH") == "" {
					cfg.DBPath = expandHome(fileCfg.DBPath)
				}
				if fileCfg.Vault.Title != "" || len(fileCfg.Vault.IndexedDirs) > 0 || len(fileCfg.Vault.Categories) > 0 {
					cfg.Vault = fileCfg.Vault
				}
				if fileCfg.Workspace.WorkDir != "" || fileCfg.Workspace.DefaultGitHost != "" {
					cfg.Workspace = fileCfg.Workspace
				}
				if fileCfg.Publishing.ContentDir != "" || len(fileCfg.Publishing.Block.Domains) > 0 || len(fileCfg.Publishing.Block.Keywords) > 0 {
					cfg.Publishing = fileCfg.Publishing
				}
				if len(fileCfg.Telemetry.Taxonomy) > 0 {
					cfg.Telemetry = fileCfg.Telemetry
				}
				if len(fileCfg.Radar) > 0 {
					cfg.Radar = fileCfg.Radar
				}
				if len(fileCfg.Dream.Routing) > 0 {
					cfg.Dream = fileCfg.Dream
				}
				if len(fileCfg.Chat.Profiles) > 0 || fileCfg.Chat.Default != "" {
					cfg.Chat = fileCfg.Chat
				}
				if len(fileCfg.Harvester.MatchDirectories) > 0 || len(fileCfg.Harvester.MatchFilenames) > 0 || len(fileCfg.Harvester.MatchExtensions) > 0 || fileCfg.Harvester.DownloadAllInDirs || len(fileCfg.Harvester.GoogleDrive) > 0 || len(fileCfg.Harvester.Targets) > 0 || fileCfg.Harvester.Jira != nil || fileCfg.Harvester.SkipArchived != nil || len(fileCfg.Harvester.IgnoredDirectories) > 0 || len(fileCfg.Harvester.ExcludeGroups) > 0 || len(fileCfg.Harvester.ExcludeProjects) > 0 {
					cfg.Harvester = fileCfg.Harvester
				}
				if fileCfg.Evaluation.DossierTitle != "" || len(fileCfg.Evaluation.StrategicDeliveries) > 0 {
					cfg.Evaluation = fileCfg.Evaluation
				}
				if fileCfg.Tui.Tabs.Search != "" || fileCfg.Tui.Tabs.Dashboard != "" || fileCfg.Tui.Tabs.Workspace != "" || fileCfg.Tui.Tabs.People != "" || fileCfg.Tui.Tabs.Routines != "" || fileCfg.Tui.Tabs.Skills != "" {
					cfg.Tui = fileCfg.Tui
				}
				if fileCfg.Hotbar.WeeklyPattern != "" || len(fileCfg.Hotbar.PinnedAnchors) > 0 {
					cfg.Hotbar = fileCfg.Hotbar
				}
				if fileCfg.Daemon.SocketPath != "" || fileCfg.Daemon.Serve.Enabled || fileCfg.Daemon.Serve.Port != 0 || fileCfg.Daemon.Serve.Host != "" {
					cfg.Daemon = fileCfg.Daemon
				}
				if fileCfg.Routines.Driver != "" || len(fileCfg.Routines.Items) > 0 {
					cfg.Routines = fileCfg.Routines
				}
				if len(fileCfg.LegacyAssistant.Profiles) > 0 || fileCfg.LegacyAssistant.Default != "" {
					cfg.LegacyAssistant = fileCfg.LegacyAssistant
				}

				// Legacy mappings
				if len(fileCfg.LegacyIndexedDirs) > 0 && len(fileCfg.Vault.IndexedDirs) == 0 {
					cfg.Vault.IndexedDirs = fileCfg.LegacyIndexedDirs
				}
				if len(fileCfg.LegacyTaxonomy) > 0 && len(fileCfg.Telemetry.Taxonomy) == 0 {
					cfg.LegacyTaxonomy = fileCfg.LegacyTaxonomy
					cfg.Telemetry.Taxonomy = nil
				}
				if (len(fileCfg.LegacyGuard.BlockedDomains) > 0 || len(fileCfg.LegacyGuard.BlockedKeywords) > 0) &&
					len(fileCfg.Publishing.Block.Domains) == 0 && len(fileCfg.Publishing.Block.Keywords) == 0 {
					cfg.Publishing.Block.Domains = fileCfg.LegacyGuard.BlockedDomains
					cfg.Publishing.Block.Keywords = fileCfg.LegacyGuard.BlockedKeywords
				}
			}
		}
	}

	// Ensure DBPath is absolute
	if !filepath.IsAbs(cfg.DBPath) {
		cfg.DBPath = filepath.Join(cfg.VaultPath, cfg.DBPath)
	}

	return cfg, nil
}
