//nolint:revive // package comment not required
package dossier

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/rjfonseca/daneel/internal/config"
)

// Telemetry holds aggregated Staff Engineering hours and activities.
type Telemetry struct {
	LoggedDays      int                 `json:"logged_days"`
	TotalHours      float64             `json:"total_hours"`
	Hours           map[string]float64  `json:"hours"`
	Percentages     map[string]float64  `json:"percentages"`
	ActivitiesByCat map[string][]string `json:"activities_by_cat"`
}

// MetaItem represents an individual career goal.
type MetaItem struct {
	File        string `json:"file"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// PersonImpact represents a colleague mentored or aligned with.
type PersonImpact struct {
	Name              string `json:"name"`
	Slug              string `json:"slug"`
	Cargo             string `json:"cargo"`
	Squad             string `json:"squad"`
	InteractionsCount int    `json:"interactions_count"`
}

// DossierData holds the complete structured performance review dossier.
type DossierData struct { //nolint:revive
	Year                int            `json:"year"`
	Period              string         `json:"period"`
	GeneratedAt         string         `json:"generated_at"`
	Telemetry           Telemetry      `json:"telemetry"`
	Metas               []MetaItem     `json:"metas"`
	PeopleImpacted      []PersonImpact `json:"people_impacted"`
	StrategicDeliveries []string       `json:"strategic_deliveries"`
	OutputFile          string         `json:"output_file,omitempty"`
}

// Compiler manages the generation of Staff AVD Dossiers.
type Compiler struct {
	vaultRoot string
}

// NewCompiler creates a new compiler instance.
func NewCompiler(vaultRoot string) *Compiler {
	return &Compiler{vaultRoot: vaultRoot}
}

// CollectTelemetry scans daily notes and aggregates time metrics dynamically.
func (c *Compiler) CollectTelemetry(year int, semester int) (*Telemetry, error) {
	cfg, _ := config.LoadConfig(c.vaultRoot, "", false)
	archetypes := cfg.GetTaxonomyArchetypes()

	hours := make(map[string]float64)
	activities := make(map[string][]string)
	for _, arch := range archetypes {
		hours[arch.ID] = 0
		activities[arch.ID] = []string{}
	}

	dailyDir := filepath.Join(c.vaultRoot, "daily")
	if _, err := os.Stat(dailyDir); os.IsNotExist(err) {
		return &Telemetry{
			Hours:           hours,
			Percentages:     make(map[string]float64),
			ActivitiesByCat: activities,
		}, nil
	}

	entries, err := os.ReadDir(dailyDir)
	if err != nil {
		return nil, err
	}

	loggedDays := 0
	datePrefix := fmt.Sprintf("%d-", year)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") || !strings.HasPrefix(entry.Name(), datePrefix) {
			continue
		}

		dateStr := strings.TrimSuffix(entry.Name(), ".md")

		// Semester filter
		if semester == 1 {
			month := dateStr[5:7]
			if month > "06" {
				continue
			}
		} else if semester == 2 {
			month := dateStr[5:7]
			if month <= "06" {
				continue
			}
		}

		data, err := os.ReadFile(filepath.Join(dailyDir, entry.Name())) //nolint:gosec // intentional
		if err != nil {
			continue
		}
		content := string(data)

		// Parse frontmatter
		fmRegex := regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\s*\n`)
		var dayTotal float64
		if m := fmRegex.FindStringSubmatch(content); len(m) > 1 {
			var rawFM map[string]interface{}
			if err := yaml.Unmarshal([]byte(m[1]), &rawFM); err == nil {
				for _, arch := range archetypes {
					// We support standard `tempo-ID` format
					key := fmt.Sprintf("tempo-%s", arch.ID)
					t := toFloat(rawFM[key])
					if t == 0 {
						// Fallback to strict ID for custom targets
						t = toFloat(rawFM[arch.ID])
					}
					hours[arch.ID] += t
					dayTotal += t
				}
			}
		}

		if dayTotal > 0 {
			loggedDays++
		}

		// Extract activity highlights dynamically by checking prefix markers like `[A]` or `[M_People]`
		lines := strings.Split(content, "\n")
		for _, l := range lines {
			lTrim := strings.TrimSpace(l)
			if strings.HasPrefix(lTrim, "-") && len(lTrim) > 15 {
				clean := regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`).ReplaceAllString(lTrim, "$1")
				clean = strings.Trim(clean, "- *")

				for _, arch := range archetypes {
					// Check for short marker (first char before _) or exact ID. E.g. [A] for A_Estrategico or [A_Estrategico]
					markerShort := ""
					if parts := strings.SplitN(arch.ID, "_", 2); len(parts) > 0 {
						markerShort = fmt.Sprintf("[%s]", parts[0])
					}
					markerExact := fmt.Sprintf("[%s]", arch.ID)

					if strings.Contains(lTrim, markerExact) || (markerShort != "" && strings.Contains(lTrim, markerShort)) {
						activities[arch.ID] = append(activities[arch.ID], fmt.Sprintf("[%s] %s", dateStr, clean))
						break
					}
				}
			}
		}
	}

	totalHours := 0.0
	for _, v := range hours {
		totalHours += v
	}

	percentages := make(map[string]float64)
	if totalHours > 0 {
		for k, v := range hours {
			percentages[k] = roundFloat((v/totalHours)*100, 1)
		}
	}

	return &Telemetry{
		LoggedDays:      loggedDays,
		TotalHours:      roundFloat(totalHours, 1),
		Hours:           hours,
		Percentages:     percentages,
		ActivitiesByCat: activities,
	}, nil
}

// LoadMetas loads all individual career goal documents for the year.
func (c *Compiler) LoadMetas(year int) ([]MetaItem, error) {
	metasDir := filepath.Join(c.vaultRoot, "notes", "carreira")
	if _, err := os.Stat(metasDir); os.IsNotExist(err) {
		metasDir = filepath.Join(c.vaultRoot, "meta", "carreira")
	}
	var items []MetaItem

	if _, err := os.Stat(metasDir); os.IsNotExist(err) {
		return items, nil
	}

	entries, err := os.ReadDir(metasDir)
	if err != nil {
		return nil, err
	}

	prefix := fmt.Sprintf("meta-individual-%d-", year)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(metasDir, entry.Name())) //nolint:gosec // intentional
		if err != nil {
			continue
		}
		content := string(data)

		title := strings.TrimSuffix(entry.Name(), ".md")
		if m := regexp.MustCompile(`(?m)^#\s+(.*?)$`).FindStringSubmatch(content); len(m) > 1 {
			title = strings.TrimSpace(m[1])
		}

		desc := ""
		if m := regexp.MustCompile(`(?m)^description:\s*["']?(.*?)["']?$`).FindStringSubmatch(content); len(m) > 1 {
			desc = strings.TrimSpace(m[1])
		}

		items = append(items, MetaItem{
			File:        filepath.ToSlash(filepath.Join("notes", "carreira", entry.Name())),
			Title:       title,
			Description: desc,
		})
	}

	return items, nil
}

// CollectPeopleImpact scans people profiles and counts interaction evidence.
func (c *Compiler) CollectPeopleImpact() ([]PersonImpact, error) {
	peopleDir := filepath.Join(c.vaultRoot, "notes", "pessoas")
	if _, err := os.Stat(peopleDir); os.IsNotExist(err) {
		peopleDir = filepath.Join(c.vaultRoot, "meta", "pessoas")
	}
	var people []PersonImpact

	if _, err := os.Stat(peopleDir); os.IsNotExist(err) {
		return people, nil
	}

	entries, err := os.ReadDir(peopleDir)
	if err != nil {
		return nil, err
	}

	datePattern := regexp.MustCompile(`-\s+\*\*\d{4}-\d{2}-\d{2}\*\*`)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") || entry.Name() == "moc-pessoas.md" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(peopleDir, entry.Name())) //nolint:gosec // intentional
		if err != nil {
			continue
		}
		content := string(data)

		name := strings.TrimSuffix(entry.Name(), ".md")
		if m := regexp.MustCompile(`(?m)^#\s+(.*?)$`).FindStringSubmatch(content); len(m) > 1 {
			name = strings.TrimSpace(m[1])
		}

		cargo := "Engenharia"
		if m := regexp.MustCompile(`\*\*Cargo / Papel:\*\*\s*(.*?)$`).FindStringSubmatch(content); len(m) > 1 {
			cargo = strings.TrimSpace(m[1])
		}

		squad := "Engenharia"
		if m := regexp.MustCompile(`\*\*Squad / Tribo:\*\*\s*(.*?)$`).FindStringSubmatch(content); len(m) > 1 {
			squad = strings.TrimSpace(m[1])
		}

		matches := datePattern.FindAllString(content, -1)
		count := len(matches)

		if count > 0 {
			people = append(people, PersonImpact{
				Name:              name,
				Slug:              strings.TrimSuffix(entry.Name(), ".md"),
				Cargo:             cargo,
				Squad:             squad,
				InteractionsCount: count,
			})
		}
	}

	sort.Slice(people, func(i, j int) bool {
		return people[i].InteractionsCount > people[j].InteractionsCount
	})

	return people, nil
}

// Compile generates the complete AVD dossier document.
func (c *Compiler) Compile(year int, semester int, outputFile string) (*DossierData, error) {
	telemetry, err := c.CollectTelemetry(year, semester)
	if err != nil {
		return nil, fmt.Errorf("failed to collect telemetry: %w", err)
	}

	metas, err := c.LoadMetas(year)
	if err != nil {
		return nil, fmt.Errorf("failed to load goals: %w", err)
	}

	people, err := c.CollectPeopleImpact()
	if err != nil {
		return nil, fmt.Errorf("failed to map people interactions: %w", err)
	}

	semLabel := "Full Year"
	switch semester {
	case 1:
		semLabel = "Semester 1"
	case 2:
		semLabel = "Semester 2"
	}
	periodTitle := fmt.Sprintf("%d (%s)", year, semLabel)

	if outputFile == "" {
		outputFile = filepath.Join(c.vaultRoot, "notes", "carreira", "avaliacoes", fmt.Sprintf("dossie-avd-%d-consolidado.md", year))
	}
	_ = os.MkdirAll(filepath.Dir(outputFile), 0750)

	cfg, _ := config.LoadConfig(c.vaultRoot, "", false)
	archetypes := cfg.GetTaxonomyArchetypes()

	dossierTitle := "Staff Engineering Impact & Performance Review Dossier"
	if cfg.Evaluation.DossierTitle != "" {
		dossierTitle = cfg.Evaluation.DossierTitle
	}

	strategicDeliveries := cfg.Evaluation.StrategicDeliveries
	if len(strategicDeliveries) == 0 {
		// Fallback defaults if none configured
		strategicDeliveries = []string{
			"Controlled rollout of core services in production with zero incidents.",
			"Full mitigation of security vulnerabilities, ensuring compliance.",
			"Enterprise Knowledge Engine & Local Semantic Retrieval via Daneel (< 5ms latency via SQLite FTS5 / BM25).",
			"Autonomous Agents Platform RFC and Native MCP Server for Engineering Copilots.",
			"Structural refactoring in shared SDKs and dependency vulnerability elimination.",
		}
	}

	dossier := &DossierData{
		Year:                year,
		Period:              periodTitle,
		GeneratedAt:         time.Now().Format(time.RFC3339),
		Telemetry:           *telemetry,
		Metas:               metas,
		PeopleImpacted:      people,
		StrategicDeliveries: strategicDeliveries,
		OutputFile:          outputFile,
	}

	// Format OKF Markdown Document
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString("type: evaluation\n")
	fmt.Fprintf(&sb, "description: %s - %s.\n", dossierTitle, periodTitle)
	sb.WriteString("tags:\n")
	sb.WriteString("  - notes/carreira/avaliacoes\n")
	sb.WriteString("  - notes/carreira\n")
	sb.WriteString("---\n\n")

	fmt.Fprintf(&sb, "# 🏆 %s — %s\n\n", dossierTitle, periodTitle)
	fmt.Fprintf(&sb, "This document consolidates **formal impact evidence, technical leadership telemetry, architectural deliveries, and multiplier impact** during %s.\n\n", periodTitle)
	sb.WriteString("---\n\n")

	sb.WriteString("## 📊 1. Staff Telemetry & Time Allocation\n\n")
	sb.WriteString("| Taxonomy Archetype | Actual Hours | Ratio (%) | Ideal Target | Alignment Assessment |\n")
	sb.WriteString("| :--- | :---: | :---: | :---: | :--- |\n")
	for _, arch := range archetypes {
		h := telemetry.Hours[arch.ID]
		p := telemetry.Percentages[arch.ID]

		targetStr := "—"
		assessment := "—"
		if arch.TargetMin > 0 {
			targetStr = fmt.Sprintf(">= %.0f%%", arch.TargetMin)
			if p >= arch.TargetMin {
				assessment = "🟢 **Aligned**"
			} else {
				assessment = "🔴 **Below Target**"
			}
		} else if arch.TargetMax > 0 {
			targetStr = fmt.Sprintf("<= %.0f%%", arch.TargetMax)
			if p <= arch.TargetMax {
				assessment = "🟢 **Optimized**"
			} else {
				assessment = "🟡 **Overhead**"
			}
		}

		fmt.Fprintf(&sb, "| **[%s] %s** | **%.1fh** | **%.1f%%** | %s | %s |\n", arch.ID, arch.Name, h, p, targetStr, assessment)
	}
	fmt.Fprintf(&sb, "| **TOTAL CONSOLIDATED** | **%.1fh** | **100%%** | — | **%d days with detailed logs** |\n\n", telemetry.TotalHours, telemetry.LoggedDays)
	sb.WriteString("---\n\n")

	sb.WriteString("## 🎯 2. Career Goals Alignment\n\n")
	for _, m := range metas {
		base := filepath.Base(m.File)
		fmt.Fprintf(&sb, "### 📌 [%s](../%s)\n", m.Title, base)
		fmt.Fprintf(&sb, "- **Description:** %s\n", m.Description)
		fmt.Fprintf(&sb, "- **Evidence in Vault:** [View Goal](../%s)\n\n", base)
	}
	sb.WriteString("---\n\n")

	sb.WriteString("## 🚀 3. Key Deliveries & Structural Impact\n\n")
	for i, d := range strategicDeliveries {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, d)
	}
	sb.WriteString("\n---\n\n")

	sb.WriteString("## 🤝 4. Multiplier Leadership & Supported Colleagues\n\n")
	sb.WriteString("| Colleague | Role & Squad | Interactions | Profile Link |\n")
	sb.WriteString("| :--- | :--- | :---: | :--- |\n")
	limit := 10
	if len(people) < limit {
		limit = len(people)
	}
	for _, p := range people[:limit] {
		fmt.Fprintf(&sb, "| **%s** | %s (%s) | %d alignments | [%s](../../pessoas/%s.md) |\n", p.Name, p.Cargo, p.Squad, p.InteractionsCount, p.Slug, p.Slug)
	}
	sb.WriteString("\n---\n")

	if err := os.MkdirAll(filepath.Dir(outputFile), 0750); err != nil {
		return nil, err
	}

	if err := os.WriteFile(outputFile, []byte(sb.String()), 0600); err != nil {
		return nil, fmt.Errorf("failed to save dossier: %w", err)
	}

	return dossier, nil
}

func toFloat(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	}
	return 0
}

func roundFloat(val float64, precision uint) float64 {
	ratio := 1.0
	for i := uint(0); i < precision; i++ {
		ratio *= 10.0
	}
	return float64(int(val*ratio+0.5)) / ratio
}
