//nolint:revive // package comment not required
package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFindLocalVault(t *testing.T) {
	tempDir := t.TempDir()

	// Create .daneel.yaml in root
	_ = os.WriteFile(filepath.Join(tempDir, ".daneel.yaml"), []byte("vault:\n  indexed_dirs: [notes]\n"), 0600)

	subDir := filepath.Join(tempDir, "notes", "ia", "subdir")
	_ = os.MkdirAll(subDir, 0750)

	root, cfgPath, err := FindLocalVault(subDir)
	if err != nil {
		t.Fatalf("FindLocalVault failed: %v", err)
	}

	if root != tempDir {
		t.Errorf("expected root %s, got %s", tempDir, root)
	}
	if cfgPath != filepath.Join(tempDir, ".daneel.yaml") {
		t.Errorf("expected config path %s, got %s", filepath.Join(tempDir, ".daneel.yaml"), cfgPath)
	}
}

func TestFindLocalVault_NoLocalVault(t *testing.T) {
	tempDir := t.TempDir()

	_, _, err := FindLocalVault(tempDir)
	if err != ErrNoLocalVault {
		t.Errorf("expected ErrNoLocalVault, got %v", err)
	}
}

func TestLoadConfig_SemanticYAML(t *testing.T) {
	t.Setenv("DANEEL_VAULT_PATH", "")
	t.Setenv("DANEEL_CONFIG", "")
	tempDir := t.TempDir()

	semanticContent := `
vault:
  title: "Test Vault"
  owner: "Alice"
  role: "Principal Engineer"
  squad: "Core"
  indexed_dirs:
    - notes
    - meta
workspace:
  work_dir: "~/MyWork"
  default_git_host: "gitlab.com"
publishing:
  content_dir: "public/content"
  block:
    domains:
      - "internal.corp"
    keywords:
      - "secret"
telemetry:
  taxonomy:
    - id: "A_Estrategico"
      name: "Estratégico"
      description: "Visão técnica"
      keywords: ["rfc", "arquitetura"]
radar:
  - name: "AI Platform"
    type: "business"
    description: "Agents and MCP"
    keywords: ["agent", "mcp"]
  - name: "Tech Debt"
    type: "friction"
    description: "Blockers"
    keywords: ["bug", "incident"]
dream:
  auto_fix: true
  routing:
    "notes/ia": ["mcp", "llm"]
chat:
  default: "custom_agy"
  profiles:
    custom_agy:
      command: "sandbox-exec agy"
      env:
        TERM: "xterm-256color"
    claude:
      command: "claude"
`
	_ = os.WriteFile(filepath.Join(tempDir, ".daneel.yaml"), []byte(semanticContent), 0600)

	origWd, _ := os.Getwd()
	_ = os.Chdir(tempDir)
	defer func() { _ = os.Chdir(origWd) }()

	cfg, err := LoadConfig("", "", false)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Vault.Title != "Test Vault" {
		t.Errorf("expected title 'Test Vault', got '%s'", cfg.Vault.Title)
	}
	if cfg.GetDefaultGitHost() != "gitlab.com" {
		t.Errorf("expected default git host 'gitlab.com', got '%s'", cfg.GetDefaultGitHost())
	}
	if len(cfg.GetBlockedDomains()) != 1 || cfg.GetBlockedDomains()[0] != "internal.corp" {
		t.Errorf("expected blocked domain 'internal.corp', got %v", cfg.GetBlockedDomains())
	}
	if len(cfg.GetBlockedKeywords()) != 1 || cfg.GetBlockedKeywords()[0] != "secret" {
		t.Errorf("expected blocked keyword 'secret', got %v", cfg.GetBlockedKeywords())
	}

	taxMap := cfg.GetTaxonomyMap()
	if len(taxMap["A_Estrategico"]) != 2 {
		t.Errorf("expected 2 keywords for A_Estrategico, got %v", taxMap["A_Estrategico"])
	}

	frictionKws := cfg.GetFrictionKeywords()
	if len(frictionKws) != 2 || frictionKws[0] != "bug" {
		t.Errorf("expected friction keywords [bug incident], got %v", frictionKws)
	}

	initMap := cfg.GetInitiativesMap()
	if len(initMap["AI Platform"]) != 2 {
		t.Errorf("expected 2 keywords for AI Platform, got %v", initMap["AI Platform"])
	}

	routingMap := cfg.GetRoutingMap()
	if len(routingMap["notes/ia"]) != 2 {
		t.Errorf("expected routing for notes/ia, got %v", routingMap["notes/ia"])
	}

	prof, profName, err := cfg.GetChatProfile("")
	if err != nil || profName != "custom_agy" || prof.Command != "sandbox-exec agy" {
		t.Errorf("expected default profile 'custom_agy', got '%s', err: %v", profName, err)
	}

	claudeProf, claudeName, err := cfg.GetChatProfile("claude")
	if err != nil || claudeName != "claude" || claudeProf.Command != "claude" {
		t.Errorf("expected claude profile, got '%s', err: %v", claudeName, err)
	}
}

func TestLoadConfig_LegacyBackwardCompatibility(t *testing.T) {
	t.Setenv("DANEEL_VAULT_PATH", "")
	t.Setenv("DANEEL_CONFIG", "")
	tempDir := t.TempDir()

	legacyContent := `
indexed_dirs:
  - notes
  - daily
taxonomy:
  A_Estrategico: ["rfc", "arquitetura"]
guard:
  blocked_domains: ["legacy.corp"]
  blocked_keywords: ["legacy_secret"]
`
	_ = os.WriteFile(filepath.Join(tempDir, ".daneel.yaml"), []byte(legacyContent), 0600)

	origWd, _ := os.Getwd()
	_ = os.Chdir(tempDir)
	defer func() { _ = os.Chdir(origWd) }()

	cfg, err := LoadConfig("", "", false)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if len(cfg.GetIndexedDirs()) != 2 {
		t.Errorf("expected 2 indexed dirs, got %v", cfg.GetIndexedDirs())
	}
	if len(cfg.GetBlockedDomains()) != 1 || cfg.GetBlockedDomains()[0] != "legacy.corp" {
		t.Errorf("expected blocked domain 'legacy.corp', got %v", cfg.GetBlockedDomains())
	}
	if len(cfg.GetBlockedKeywords()) != 1 || cfg.GetBlockedKeywords()[0] != "legacy_secret" {
		t.Errorf("expected blocked keyword 'legacy_secret', got %v", cfg.GetBlockedKeywords())
	}
	if len(cfg.GetTaxonomyMap()["A_Estrategico"]) != 2 {
		t.Errorf("expected 2 keywords for A_Estrategico, got %v", cfg.GetTaxonomyMap()["A_Estrategico"])
	}
}

func TestLoadConfig_ForceGlobal(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, ".daneel.yaml"), []byte("vault:\n  indexed_dirs: [notes]\n"), 0600)

	origWd, _ := os.Getwd()
	_ = os.Chdir(tempDir)
	defer func() { _ = os.Chdir(origWd) }()

	cfg, err := LoadConfig("", "", true)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Mode != ModeGlobal {
		t.Errorf("expected ModeGlobal, got %s", cfg.Mode)
	}
	if cfg.VaultPath == tempDir {
		t.Errorf("expected global vault, not local tempDir %s", tempDir)
	}
}

func TestLoadConfig_EnvVaultPathWithLocalYaml(t *testing.T) {
	t.Setenv("DANEEL_CONFIG", "")
	vaultDir := t.TempDir()
	otherDir := t.TempDir()

	yamlContent := `vault:
  title: "Env Vault"
  indexed_dirs:
    - notes
    - sources
`
	_ = os.WriteFile(filepath.Join(vaultDir, ".daneel.yaml"), []byte(yamlContent), 0600)

	t.Setenv("DANEEL_VAULT_PATH", vaultDir)

	origWd, _ := os.Getwd()
	_ = os.Chdir(otherDir)
	defer func() { _ = os.Chdir(origWd) }()

	cfg, err := LoadConfig("", "", false)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.VaultPath != vaultDir {
		t.Errorf("expected VaultPath %s, got %s", vaultDir, cfg.VaultPath)
	}
	expectedCfgFile := filepath.Join(vaultDir, ".daneel.yaml")
	if cfg.ConfigFile != expectedCfgFile {
		t.Errorf("expected ConfigFile %s, got %s", expectedCfgFile, cfg.ConfigFile)
	}
	if cfg.ConfigSource != "LOCAL_CONFIG (.daneel.yaml)" {
		t.Errorf("expected ConfigSource 'LOCAL_CONFIG (.daneel.yaml)', got '%s'", cfg.ConfigSource)
	}
	if cfg.Mode != ModeLocal {
		t.Errorf("expected ModeLocal, got %s", cfg.Mode)
	}
	expectedDB := filepath.Join(vaultDir, ".daneel", "cache", "vault_index.db")
	if cfg.DBPath != expectedDB {
		t.Errorf("expected DBPath %s, got %s", expectedDB, cfg.DBPath)
	}
	if cfg.Vault.Title != "Env Vault" {
		t.Errorf("expected Vault.Title 'Env Vault', got '%s'", cfg.Vault.Title)
	}
	if len(cfg.GetIndexedDirs()) != 2 {
		t.Errorf("expected 2 indexed dirs, got %v", cfg.GetIndexedDirs())
	}
}

func TestLoadConfig_FlagVaultPathWithLocalYaml(t *testing.T) {
	t.Setenv("DANEEL_VAULT_PATH", "")
	t.Setenv("DANEEL_CONFIG", "")
	vaultDir := t.TempDir()
	otherDir := t.TempDir()

	yamlContent := `vault:
  title: "Flag Vault"
`
	_ = os.WriteFile(filepath.Join(vaultDir, ".daneel.yaml"), []byte(yamlContent), 0600)

	origWd, _ := os.Getwd()
	_ = os.Chdir(otherDir)
	defer func() { _ = os.Chdir(origWd) }()

	cfg, err := LoadConfig(vaultDir, "", false)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.VaultPath != vaultDir {
		t.Errorf("expected VaultPath %s, got %s", vaultDir, cfg.VaultPath)
	}
	expectedCfgFile := filepath.Join(vaultDir, ".daneel.yaml")
	if cfg.ConfigFile != expectedCfgFile {
		t.Errorf("expected ConfigFile %s, got %s", expectedCfgFile, cfg.ConfigFile)
	}
	if cfg.Mode != ModeLocal {
		t.Errorf("expected ModeLocal, got %s", cfg.Mode)
	}
	if cfg.Vault.Title != "Flag Vault" {
		t.Errorf("expected Vault.Title 'Flag Vault', got '%s'", cfg.Vault.Title)
	}
}

func TestLoadConfig_EnvVaultPathWithoutLocalYaml(t *testing.T) {
	vaultDir := t.TempDir()
	otherDir := t.TempDir()

	t.Setenv("DANEEL_VAULT_PATH", vaultDir)

	origWd, _ := os.Getwd()
	_ = os.Chdir(otherDir)
	defer func() { _ = os.Chdir(origWd) }()

	cfg, err := LoadConfig("", "", false)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.VaultPath != vaultDir {
		t.Errorf("expected VaultPath %s, got %s", vaultDir, cfg.VaultPath)
	}
	if cfg.Mode != ModeGlobal {
		t.Errorf("expected ModeGlobal, got %s", cfg.Mode)
	}
}

func TestHarvestTarget(t *testing.T) {
	tests := []struct {
		target   HarvestTarget
		wantType string
		wantName string
	}{
		{
			target:   HarvestTarget{Platform: "github", Org: "my-org"},
			wantType: "org",
			wantName: "my-org",
		},
		{
			target:   HarvestTarget{Platform: "gitlab", Group: "my-group"},
			wantType: "org",
			wantName: "my-group",
		},
		{
			target:   HarvestTarget{Platform: "github", User: "octocat"},
			wantType: "user",
			wantName: "octocat",
		},
		{
			target:   HarvestTarget{Platform: "github", Org: "some-user", Type: "user"},
			wantType: "user",
			wantName: "some-user",
		},
	}

	for _, tt := range tests {
		gotType := tt.target.TargetType()
		gotName := tt.target.TargetName()
		if gotType != tt.wantType {
			t.Errorf("TargetType() = %q, want %q for target %+v", gotType, tt.wantType, tt.target)
		}
		if gotName != tt.wantName {
			t.Errorf("TargetName() = %q, want %q for target %+v", gotName, tt.wantName, tt.target)
		}
	}
}

func TestHarvesterFilteringConfig(t *testing.T) {
	// 1. Defaults
	defCfg := DefaultHarvesterConfig()
	if !defCfg.ShouldSkipArchived() {
		t.Errorf("expected default ShouldSkipArchived() to be true")
	}
	ignored := defCfg.GetIgnoredDirectories()
	if len(ignored) < 5 {
		t.Fatalf("expected default ignored directories to contain toxic dirs, got %v", ignored)
	}

	// 2. Custom configuration & methods
	falseVal := false
	custom := HarvesterConfig{
		SkipArchived:       &falseVal,
		IgnoredDirectories: []string{"node_modules", "custom_vendor"},
		ExcludeGroups:      []string{"acme/marketing", "acme/sandbox"},
		ExcludeProjects:    []string{"*poc*", "*-sandbox", "test-*"},
	}

	if custom.ShouldSkipArchived() {
		t.Errorf("expected ShouldSkipArchived() to be false when set to false")
	}

	// Test IsGroupExcluded
	if !custom.IsGroupExcluded("acme/marketing") {
		t.Errorf("expected acme/marketing to be excluded")
	}
	if !custom.IsGroupExcluded("acme/marketing/campaigns") {
		t.Errorf("expected subgroup acme/marketing/campaigns to be excluded")
	}
	if !custom.IsGroupExcluded("sources/gitlab/acme/sandbox/test-proj") {
		t.Errorf("expected path containing excluded group to be excluded")
	}
	if custom.IsGroupExcluded("acme/saas/auth") {
		t.Errorf("expected acme/saas/auth NOT to be excluded")
	}

	// Test IsProjectExcluded
	if !custom.IsProjectExcluded("service-poc", "acme/saas/service-poc") {
		t.Errorf("expected service-poc to be excluded")
	}
	if !custom.IsProjectExcluded("payment-sandbox", "acme/payment-sandbox") {
		t.Errorf("expected payment-sandbox to be excluded")
	}
	if !custom.IsProjectExcluded("test-runner", "acme/test-runner") {
		t.Errorf("expected test-runner to be excluded")
	}
	if custom.IsProjectExcluded("core-auth", "acme/saas/core-auth") {
		t.Errorf("expected core-auth NOT to be excluded")
	}

	// 3. YAML Unmarshal test
	yamlContent := `
harvester:
  skip_archived: false
  ignored_directories:
    - "node_modules"
    - "vendor"
  exclude_groups:
    - "corp/marketing"
  exclude_projects:
    - "*poc*"
`
	tempDir := t.TempDir()
	t.Setenv("DANEEL_VAULT_PATH", "")
	t.Setenv("DANEEL_CONFIG", "")
	_ = os.WriteFile(filepath.Join(tempDir, ".daneel.yaml"), []byte(yamlContent), 0600)
	origWd, _ := os.Getwd()
	_ = os.Chdir(tempDir)
	defer func() { _ = os.Chdir(origWd) }()

	loaded, err := LoadConfig("", "", false)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	hc := loaded.GetHarvesterConfig()
	if hc.ShouldSkipArchived() {
		t.Errorf("expected loaded skip_archived to be false")
	}
	if !hc.IsGroupExcluded("corp/marketing") {
		t.Errorf("expected corp/marketing to be excluded")
	}
	if !hc.IsProjectExcluded("test-poc", "corp/test-poc") {
		t.Errorf("expected test-poc to be excluded")
	}
}

func TestRoutinesConfig(t *testing.T) {
	cfg := Config{}
	routines := cfg.GetRoutinesConfig()

	if routines.Driver != "systemd" {
		t.Errorf("expected default driver 'systemd', got %q", routines.Driver)
	}
	if len(routines.Items) != 3 {
		t.Fatalf("expected 3 default routines, got %d", len(routines.Items))
	}
	if tidy, ok := routines.Items["tidy"]; !ok || !tidy.Enabled || tidy.Schedule != "hourly" {
		t.Errorf("expected tidy routine configured, got %+v", tidy)
	}
	if harvest, ok := routines.Items["harvest"]; !ok || !harvest.Enabled || harvest.Schedule != "0 */6 * * *" {
		t.Errorf("expected harvest routine configured, got %+v", harvest)
	}
	if doc, ok := routines.Items["doctor"]; !ok || !doc.Enabled || doc.Schedule != "*-*-* 06:00:00" {
		t.Errorf("expected doctor routine configured, got %+v", doc)
	}

	// Custom config test
	customYaml := `
routines:
  driver: custom_driver
  items:
    backup:
      enabled: true
      schedule: "daily"
      command: "backup"
`
	tempDir := t.TempDir()
	t.Setenv("DANEEL_VAULT_PATH", "")
	t.Setenv("DANEEL_CONFIG", "")
	_ = os.WriteFile(filepath.Join(tempDir, ".daneel.yaml"), []byte(customYaml), 0600)
	origWd, _ := os.Getwd()
	_ = os.Chdir(tempDir)
	defer func() { _ = os.Chdir(origWd) }()

	loaded, err := LoadConfig("", "", false)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	loadedRoutines := loaded.GetRoutinesConfig()
	if loadedRoutines.Driver != "custom_driver" {
		t.Errorf("expected driver 'custom_driver', got %q", loadedRoutines.Driver)
	}
	if bkp, ok := loadedRoutines.Items["backup"]; !ok || !bkp.Enabled || bkp.Schedule != "daily" {
		t.Errorf("expected custom backup routine, got %+v", bkp)
	}
	if tidy, ok := loadedRoutines.Items["tidy"]; !ok || !tidy.Enabled {
		t.Errorf("expected default tidy routine to be preserved after merging custom routine, got %+v", tidy)
	}
}

func TestDaemonConfig(t *testing.T) {
	cfg := Config{}
	daemonCfg := cfg.GetDaemonConfig()

	if daemonCfg.Serve.Port != 4321 {
		t.Errorf("expected default serve port 4321, got %d", daemonCfg.Serve.Port)
	}
	if daemonCfg.Serve.Host != "127.0.0.1" {
		t.Errorf("expected default serve host 127.0.0.1, got %q", daemonCfg.Serve.Host)
	}

	customYaml := `
daemon:
  socket_path: "/custom/daneel.sock"
  serve:
    enabled: true
    port: 8080
    host: "0.0.0.0"
`
	tempDir := t.TempDir()
	t.Setenv("DANEEL_VAULT_PATH", "")
	t.Setenv("DANEEL_CONFIG", "")
	_ = os.WriteFile(filepath.Join(tempDir, ".daneel.yaml"), []byte(customYaml), 0600)
	origWd, _ := os.Getwd()
	_ = os.Chdir(tempDir)
	defer func() { _ = os.Chdir(origWd) }()

	loaded, err := LoadConfig("", "", false)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	loadedDaemon := loaded.GetDaemonConfig()
	if loadedDaemon.SocketPath != "/custom/daneel.sock" {
		t.Errorf("expected socket path '/custom/daneel.sock', got %q", loadedDaemon.SocketPath)
	}
	if !loadedDaemon.Serve.Enabled || loadedDaemon.Serve.Port != 8080 || loadedDaemon.Serve.Host != "0.0.0.0" {
		t.Errorf("expected custom serve config, got %+v", loadedDaemon.Serve)
	}
}

func TestBranchInactivityConfig(t *testing.T) {
	// Default
	var emptyCfg Config
	if d := emptyCfg.GetBranchInactivityThreshold(); d != 48*time.Hour {
		t.Errorf("expected default 48h, got %v", d)
	}

	// Custom valid duration
	cfgCustom := Config{
		Workspace: WorkspaceConfig{
			BranchInactivity: "72h",
		},
	}
	if d := cfgCustom.GetBranchInactivityThreshold(); d != 72*time.Hour {
		t.Errorf("expected 72h, got %v", d)
	}

	// Custom days format
	cfgDays := Config{
		Workspace: WorkspaceConfig{
			BranchInactivity: "3d",
		},
	}
	if d := cfgDays.GetBranchInactivityThreshold(); d != 72*time.Hour {
		t.Errorf("expected 72h for 3d, got %v", d)
	}

	// Invalid fallback
	cfgInvalid := Config{
		Workspace: WorkspaceConfig{
			BranchInactivity: "invalid",
		},
	}
	if d := cfgInvalid.GetBranchInactivityThreshold(); d != 48*time.Hour {
		t.Errorf("expected 48h fallback on invalid, got %v", d)
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		hasError bool
	}{
		{"48h", 48 * time.Hour, false},
		{"24h", 24 * time.Hour, false},
		{"30m", 30 * time.Minute, false},
		{"2d", 48 * time.Hour, false},
		{"1D", 24 * time.Hour, false},
		{"", 0, true},
		{"invalid", 0, true},
		{"xyz d", 0, true},
	}

	for _, tt := range tests {
		d, err := ParseDuration(tt.input)
		if tt.hasError && err == nil {
			t.Errorf("expected error for %q, got nil", tt.input)
		}
		if !tt.hasError && err != nil {
			t.Errorf("unexpected error for %q: %v", tt.input, err)
		}
		if !tt.hasError && d != tt.expected {
			t.Errorf("input %q: expected %v, got %v", tt.input, tt.expected, d)
		}
	}
}

func TestDefaultFinOpsConfig(t *testing.T) {
	finops := DefaultFinOpsConfig()
	if finops.DefaultModel != "gemini-pro" {
		t.Errorf("expected default model gemini-pro, got %s", finops.DefaultModel)
	}

	expectedModels := []struct {
		name     string
		input1M  float64
		cache1M  float64
		output1M float64
	}{
		{"gemini-pro", 1.25, 0.3125, 5.00},
		{"pro", 1.25, 0.3125, 5.00},
		{"gemini-flash", 0.15, 0.0375, 0.60},
		{"flash", 0.15, 0.0375, 0.60},
		{"gemini-flash-lite", 0.075, 0.01875, 0.30},
		{"flash_lite", 0.075, 0.01875, 0.30},
		{"claude-opus", 15.00, 1.50, 75.00},
		{"opus", 15.00, 1.50, 75.00},
		{"claude-sonnet", 3.00, 0.30, 15.00},
		{"sonnet", 3.00, 0.30, 15.00},
		{"claude-haiku", 0.80, 0.08, 4.00},
		{"haiku", 0.80, 0.08, 4.00},
		{"gpt-codex", 2.50, 1.25, 10.00},
		{"codex", 2.50, 1.25, 10.00},
		{"gpt-4o", 2.50, 1.25, 10.00},
	}

	for _, m := range expectedModels {
		p, ok := finops.Pricing[m.name]
		if !ok {
			t.Fatalf("missing pricing for model: %s", m.name)
		}
		if p.InputPer1M != m.input1M {
			t.Errorf("[%s] expected input %v, got %v", m.name, m.input1M, p.InputPer1M)
		}
		if p.CacheReadPer1M != m.cache1M {
			t.Errorf("[%s] expected cache read %v, got %v", m.name, m.cache1M, p.CacheReadPer1M)
		}
		if p.OutputPer1M != m.output1M {
			t.Errorf("[%s] expected output %v, got %v", m.name, m.output1M, p.OutputPer1M)
		}
	}
}

func TestGetFinOpsConfig_Merging(t *testing.T) {
	var nilCfg *Config
	def := nilCfg.GetFinOpsConfig()
	if def.DefaultModel != "gemini-pro" {
		t.Errorf("nil config should return default finops")
	}

	cfg := &Config{
		Telemetry: TelemetryConfig{
			FinOps: FinOpsConfig{
				DailyBudgetUSD:   20.0,
				MonthlyBudgetUSD: 300.0,
				DefaultModel:     "claude-sonnet",
				Pricing: map[string]ModelPricingConfig{
					"gemini-flash": {
						InputPer1M:     0.20,
						CacheReadPer1M: 0.05,
						OutputPer1M:    0.80,
					},
					"custom-local-llm": {
						InputPer1M:     0.0,
						CacheReadPer1M: 0.0,
						OutputPer1M:    0.0,
					},
				},
			},
		},
	}

	merged := cfg.GetFinOpsConfig()
	if merged.DailyBudgetUSD != 20.0 || merged.MonthlyBudgetUSD != 300.0 {
		t.Errorf("expected budgets 20/300, got %v/%v", merged.DailyBudgetUSD, merged.MonthlyBudgetUSD)
	}
	if merged.DefaultModel != "claude-sonnet" {
		t.Errorf("expected default model claude-sonnet, got %s", merged.DefaultModel)
	}
	// Overridden model
	if flash := merged.Pricing["gemini-flash"]; flash.InputPer1M != 0.20 {
		t.Errorf("expected overridden flash pricing 0.20, got %v", flash.InputPer1M)
	}
	// Custom model
	if _, ok := merged.Pricing["custom-local-llm"]; !ok {
		t.Errorf("expected custom-local-llm in pricing")
	}
	// Non-overridden model should retain default
	if sonnet := merged.Pricing["claude-sonnet"]; sonnet.InputPer1M != 3.00 {
		t.Errorf("expected default sonnet pricing 3.00, got %v", sonnet.InputPer1M)
	}
}

func TestResolveModelPricing(t *testing.T) {
	cfg := &Config{
		Telemetry: TelemetryConfig{
			FinOps: FinOpsConfig{
				Pricing: map[string]ModelPricingConfig{
					"special-custom-agent": {
						InputPer1M:     1.0,
						CacheReadPer1M: 0.2,
						OutputPer1M:    2.0,
					},
				},
			},
		},
	}

	tests := []struct {
		modelName      string
		expectedInput  float64
		expectedOutput float64
	}{
		{"gemini-2.0-flash-lite-preview-02-05", 0.075, 0.30},
		{"GEMINI_FLASH_LITE", 0.075, 0.30},
		{"gemini-1.5-flash", 0.15, 0.60},
		{"FLASH", 0.15, 0.60},
		{"claude-3-5-sonnet-20241022", 3.00, 15.00},
		{"Claude-3-Haiku-20240307", 0.80, 4.00},
		{"claude-3-opus", 15.00, 75.00},
		{"gpt-4o", 2.50, 10.00},
		{"openai/gpt-4-turbo", 2.50, 10.00},
		{"codex-v2", 2.50, 10.00},
		{"unknown-future-model", 1.25, 5.00}, // defaults to gemini-pro
		{"special-custom-agent", 1.0, 2.0},
	}

	for _, tt := range tests {
		p := cfg.ResolveModelPricing(tt.modelName)
		if p.InputPer1M != tt.expectedInput {
			t.Errorf("[%s] expected input %v, got %v", tt.modelName, tt.expectedInput, p.InputPer1M)
		}
		if p.OutputPer1M != tt.expectedOutput {
			t.Errorf("[%s] expected output %v, got %v", tt.modelName, tt.expectedOutput, p.OutputPer1M)
		}
	}
}

func TestCalculateCostUSD(t *testing.T) {
	cfg := &Config{}

	// 1M tokens of each for gemini-flash: input $0.15, cacheRead $0.0375, output $0.60
	cost := cfg.CalculateCostUSD("gemini-flash", 1_000_000, 1_000_000, 1_000_000)
	expected := 0.15 + 0.0375 + 0.60
	if cost != expected {
		t.Errorf("expected cost %v, got %v", expected, cost)
	}

	// 100k input, 50k cache read, 20k output on flash:
	// (100000*0.15 + 50000*0.0375 + 20000*0.60) / 1000000 = (15000 + 1875 + 12000) / 1000000 = 0.028875
	cost = cfg.CalculateCostUSD("flash", 100_000, 50_000, 20_000)
	expected = 0.028875
	if cost != expected {
		t.Errorf("expected cost %v, got %v", expected, cost)
	}
}

func TestTuiTabsConfig_Telemetry(t *testing.T) {
	def := DefaultTuiConfig()
	if def.Tabs.Telemetry == "" {
		t.Fatal("expected default TUI telemetry tab to be non-empty")
	}
	if def.Tabs.Telemetry != "8: 📈 Telemetry" && def.Tabs.Telemetry != "8:Telemetry" {
		t.Errorf("unexpected telemetry tab format: %s", def.Tabs.Telemetry)
	}

	cfg := &Config{
		Tui: TuiConfig{
			Tabs: TuiTabsConfig{
				Telemetry: "8: Custom FinOps",
			},
		},
	}
	merged := cfg.GetTuiConfig()
	if merged.Tabs.Telemetry != "8: Custom FinOps" {
		t.Errorf("expected custom telemetry tab, got %s", merged.Tabs.Telemetry)
	}
}

func TestLoadConfig_PreservesFinOps(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Config with only telemetry.finops (taxonomy omitted)
	finopsOnlyYAML := `
vault:
  title: "FinOps Vault"
telemetry:
  finops:
    daily_budget_usd: 12.50
    monthly_budget_usd: 250.00
    default_model: "gemini-flash"
    pricing:
      custom-agent:
        input_per_1m: 1.50
        cache_read_per_1m: 0.30
        output_per_1m: 6.00
`
	configFile := filepath.Join(tempDir, ".daneel.yaml")
	if err := os.WriteFile(configFile, []byte(finopsOnlyYAML), 0600); err != nil {
		t.Fatalf("failed to write test yaml: %v", err)
	}

	cfg, err := LoadConfig(tempDir, configFile, false)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Telemetry.FinOps.DailyBudgetUSD != 12.50 {
		t.Errorf("expected daily budget 12.50, got %v", cfg.Telemetry.FinOps.DailyBudgetUSD)
	}
	if cfg.Telemetry.FinOps.MonthlyBudgetUSD != 250.00 {
		t.Errorf("expected monthly budget 250.00, got %v", cfg.Telemetry.FinOps.MonthlyBudgetUSD)
	}
	if cfg.Telemetry.FinOps.DefaultModel != "gemini-flash" {
		t.Errorf("expected default model gemini-flash, got %s", cfg.Telemetry.FinOps.DefaultModel)
	}
	if p, ok := cfg.Telemetry.FinOps.Pricing["custom-agent"]; !ok || p.InputPer1M != 1.50 {
		t.Errorf("expected custom-agent pricing loaded, got %+v", p)
	}
	// Taxonomy should have fallen back to default archetypes
	if len(cfg.Telemetry.Taxonomy) == 0 {
		t.Errorf("expected default taxonomy archetypes preserved when omitted in YAML")
	}

	// 2. Config with both taxonomy and finops
	bothYAML := `
vault:
  title: "Both Vault"
telemetry:
  taxonomy:
    - id: "Custom_Archetype"
      name: "Custom Archetype"
      description: "Testing taxonomy preservation"
  finops:
    daily_budget_usd: 5.00
`
	if err := os.WriteFile(configFile, []byte(bothYAML), 0600); err != nil {
		t.Fatalf("failed to write test yaml: %v", err)
	}

	cfgBoth, err := LoadConfig(tempDir, configFile, false)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if len(cfgBoth.Telemetry.Taxonomy) != 1 || cfgBoth.Telemetry.Taxonomy[0].ID != "Custom_Archetype" {
		t.Errorf("expected custom taxonomy preserved, got %+v", cfgBoth.Telemetry.Taxonomy)
	}
	if cfgBoth.Telemetry.FinOps.DailyBudgetUSD != 5.00 {
		t.Errorf("expected daily budget 5.00 preserved, got %v", cfgBoth.Telemetry.FinOps.DailyBudgetUSD)
	}
}
