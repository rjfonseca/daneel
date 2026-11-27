// Package overview provides shared, daemon-compatible aggregation builders for TUI and Web surfaces.
package overview

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	graphv1 "github.com/rjfonseca/daneel/api/v1"
	"github.com/rjfonseca/daneel/internal/config"
	"github.com/rjfonseca/daneel/internal/dream"
	"github.com/rjfonseca/daneel/internal/ops/backlog"
	"github.com/rjfonseca/daneel/internal/ops/briefing"
	"github.com/rjfonseca/daneel/internal/ops/daily"
	"github.com/rjfonseca/daneel/internal/ops/tracking"
	"github.com/rjfonseca/daneel/internal/ops/workspace"
	"github.com/rjfonseca/daneel/internal/routine"
	"github.com/rjfonseca/daneel/internal/search"
	"github.com/rjfonseca/daneel/internal/spooler"
	"github.com/rjfonseca/daneel/internal/telemetry"
	"github.com/rjfonseca/daneel/internal/vault"
)

const activeSessionWindow = 15 * time.Minute

// BuildDashboardSummary aggregates 7-day Staff taxonomy, Morning Handoff digest, vault health, and backlog.
func BuildDashboardSummary(ctx context.Context, vaultDir string, cfg *config.Config, engine *search.Engine, days int) *graphv1.GetDashboardSummaryResponse {
	if days <= 0 {
		days = 7
	}
	if cfg == nil && vaultDir != "" {
		if loaded, err := config.LoadConfig(vaultDir, "", false); err == nil {
			cfg = loaded
		}
	}

	resp := &graphv1.GetDashboardSummaryResponse{
		WindowDays:      int32(days), //nolint:gosec
		Taxonomy:        make([]*graphv1.TaxonomyMetricItem, 0),
		DriftAlerts:     make([]string, 0),
		FrictionSignals: make([]*graphv1.FrictionSignalItem, 0),
		BacklogItems:    make([]*graphv1.BacklogItemProto, 0),
	}

	logs, _ := tracking.ListDailyLogs(vaultDir, days)
	resp.DaysWithLogs = int32(len(logs)) //nolint:gosec

	var sumA, sumB, sumC, sumE, totalHours float64
	for _, l := range logs {
		sumA += l.TempoA
		sumB += l.TempoB
		sumC += l.TempoC
		sumE += l.TempoE
		totalHours += l.TotalHours
	}
	resp.TotalHours = math.Round(totalHours*10) / 10

	archetypes := config.DefaultTaxonomyArchetypes()
	if cfg != nil {
		if configured := cfg.GetTaxonomyArchetypes(); len(configured) > 0 {
			archetypes = configured
		}
	}

	for _, arch := range archetypes {
		hours := hoursForArchetype(arch.ID, sumA, sumB, sumC, sumE)
		var pct float64
		if totalHours > 0 {
			pct = math.Round((hours/totalHours)*1000) / 10
		}
		resp.Taxonomy = append(resp.Taxonomy, &graphv1.TaxonomyMetricItem{
			Id:         arch.ID,
			Name:       arch.Name,
			Hours:      math.Round(hours*10) / 10,
			Percentage: pct,
			TargetMin:  arch.TargetMin,
			TargetMax:  arch.TargetMax,
		})
		if totalHours >= 15.0 {
			if arch.TargetMin > 0 && pct < arch.TargetMin {
				resp.DriftAlerts = append(resp.DriftAlerts, fmt.Sprintf("%s (%.1f%%) < %.0f%%", arch.Name, pct, arch.TargetMin))
			}
			if arch.TargetMax > 0 && pct > arch.TargetMax {
				resp.DriftAlerts = append(resp.DriftAlerts, fmt.Sprintf("%s (%.1f%%) > %.0f%%", arch.Name, pct, arch.TargetMax))
			}
		}
	}

	today := time.Now().Format("2006-01-02")
	if temp, err := dream.AnalyzeTemporalWindow(vaultDir, today, days); err == nil && temp != nil {
		limit := min(2, len(temp.FrictionSignals))
		for _, fs := range temp.FrictionSignals[:limit] {
			resp.FrictionSignals = append(resp.FrictionSignals, &graphv1.FrictionSignalItem{
				Date: fs.Date,
				Line: fs.Line,
			})
		}
	}

	if mRep, err := daily.RunMorningWithOptions(vaultDir, today, cfg, daily.MorningOptions{DryRun: true}); err == nil && mRep != nil {
		resp.Handoff = &graphv1.HandoffDigest{
			Date:              mRep.Date,
			DreamCyclesCount:  int32(mRep.DreamCyclesCount), //nolint:gosec
			AutonomousActions: append([]string(nil), mRep.AutonomousActions...),
			InboxItems:        append([]string(nil), mRep.InboxItems...),
			Alerts:            append([]string(nil), mRep.Alerts...),
			Suggestions:       append([]string(nil), mRep.Suggestions...),
		}
	}

	totalNotes, totalLinks, orphanNotes := computeVaultHealth(ctx, vaultDir, engine)
	resp.TotalNotes = int32(totalNotes)   //nolint:gosec
	resp.TotalLinks = int32(totalLinks)   //nolint:gosec
	resp.OrphanNotes = int32(orphanNotes) //nolint:gosec

	if bRep, err := backlog.GetActiveBacklog(vaultDir); err == nil && bRep != nil {
		for _, item := range bRep.Items {
			resp.BacklogItems = append(resp.BacklogItems, &graphv1.BacklogItemProto{
				Priority:    item.Priority,
				Title:       item.Title,
				Description: item.Description,
				Completed:   item.Completed,
			})
		}
	}

	return resp
}

func hoursForArchetype(id string, a, b, c, e float64) float64 {
	upper := strings.ToUpper(strings.TrimSpace(id))
	switch {
	case strings.HasPrefix(upper, "A"):
		return a
	case strings.HasPrefix(upper, "B"):
		return b
	case strings.HasPrefix(upper, "C"):
		return c
	case strings.HasPrefix(upper, "E"):
		return e
	default:
		return 0
	}
}

func computeVaultHealth(ctx context.Context, vaultDir string, engine *search.Engine) (int, int, int) {
	if engine != nil && engine.DB() != nil {
		var notes, links, orphans int
		if err := engine.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM notes_meta`).Scan(&notes); err == nil {
			_ = engine.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM note_links`).Scan(&links)
			_ = engine.DB().QueryRowContext(ctx, `
				SELECT COUNT(*) FROM notes_meta m
				WHERE m.rel_path NOT LIKE 'daily/%'
				  AND m.rel_path NOT LIKE 'inbox/%'
				  AND m.rel_path NOT LIKE 'sources/%'
				  AND NOT EXISTS (SELECT 1 FROM note_links l WHERE l.source_rel_path = m.rel_path)
				  AND NOT EXISTS (SELECT 1 FROM note_links l WHERE l.target_rel_path = m.rel_path)
			`).Scan(&orphans)
			return notes, links, orphans
		}
	}
	vm := vault.CalculateVaultMetrics(vaultDir)
	return vm.TotalNotes, vm.TotalLinks, vm.OrphanNotes
}

// BuildWorkspaceOverview returns repositories from cached SQLite store or live Git scan.
func BuildWorkspaceOverview(ctx context.Context, vaultDir string, cfg *config.Config, engine *search.Engine, forceRescan bool) (*graphv1.GetWorkspaceOverviewResponse, error) {
	if cfg == nil && vaultDir != "" {
		if loaded, err := config.LoadConfig(vaultDir, "", false); err == nil {
			cfg = loaded
		}
	}

	workDir := ""
	if cfg != nil {
		workDir = cfg.GetWorkDir()
	}
	if workDir == "" {
		home, _ := os.UserHomeDir()
		projectsDir := filepath.Join(home, "Projects")
		if info, err := os.Stat(projectsDir); err == nil && info.IsDir() {
			workDir = projectsDir
		} else {
			workDir = filepath.Join(home, "Work")
		}
	}

	if !forceRescan && engine != nil {
		cached, err := engine.SearchProjects(ctx, "", false, false, "", 0)
		if err == nil && len(cached) > 0 {
			return workspaceResponseFromRepos(workDir, cached, "", true), nil
		}
	}

	scanner := workspace.NewScanner(workDir, vaultDir)
	rep, err := scanner.Scan(false)
	if err != nil {
		return nil, err
	}

	var allRepos []workspace.RepoInfo
	for _, repos := range rep.Categories {
		allRepos = append(allRepos, repos...)
	}
	sort.Slice(allRepos, func(i, j int) bool {
		return allRepos[i].RelPath < allRepos[j].RelPath
	})

	if engine != nil && len(allRepos) > 0 {
		_ = engine.SaveWorkspaceRepos(ctx, allRepos)
	}

	return workspaceResponseFromRepos(workDir, allRepos, rep.ScannedAt, false), nil
}

func workspaceResponseFromRepos(workDir string, repos []workspace.RepoInfo, scannedAt string, fromCache bool) *graphv1.GetWorkspaceOverviewResponse {
	cleanCount := 0
	dirtyCount := 0
	pbRepos := make([]*graphv1.WorkspaceRepoItem, 0, len(repos))
	for _, r := range repos {
		if r.Dirty {
			dirtyCount++
		} else {
			cleanCount++
		}
		pbRepos = append(pbRepos, &graphv1.WorkspaceRepoItem{
			Name:              r.Name,
			RelPath:           r.RelPath,
			Category:          r.Category,
			Branch:            r.Branch,
			Stack:             r.Stack,
			ComponentType:     r.ComponentType,
			Commands:          r.Commands,
			Description:       r.Description,
			VaultLink:         r.VaultLink,
			LastCommitDate:    r.LastCommitDate,
			DaysSinceActivity: int32(r.DaysSinceActivity), //nolint:gosec
			Dirty:             r.Dirty,
			ModifiedCount:     int32(r.ModifiedCount),  //nolint:gosec
			UntrackedCount:    int32(r.UntrackedCount), //nolint:gosec
			FilesPreview:      append([]string(nil), r.FilesPreview...),
		})
	}
	return &graphv1.GetWorkspaceOverviewResponse{
		WorkDir:    workDir,
		TotalRepos: int32(len(repos)), //nolint:gosec
		CleanCount: int32(cleanCount), //nolint:gosec
		DirtyCount: int32(dirtyCount), //nolint:gosec
		ScannedAt:  scannedAt,
		FromCache:  fromCache,
		Repos:      pbRepos,
	}
}

// BuildPeopleDirectory returns a lightweight directory of people profiles without running N full briefings.
func BuildPeopleDirectory(ctx context.Context, vaultDir string, engine *search.Engine) (*graphv1.GetPeopleDirectoryResponse, error) {
	resp := &graphv1.GetPeopleDirectoryResponse{
		People: make([]*graphv1.PersonSummaryItem, 0),
	}

	if engine != nil {
		results, err := engine.SearchPeople(ctx, "", "", "", "")
		if err == nil && len(results) > 0 {
			for _, p := range results {
				base := filepath.Base(p.RelPath)
				if strings.EqualFold(base, "moc-pessoas.md") || strings.Contains(p.RelPath, "/candidatos/") {
					continue
				}
				slug := strings.TrimSuffix(base, ".md")
				resp.People = append(resp.People, &graphv1.PersonSummaryItem{
					Name:    p.Name,
					Slug:    slug,
					RelPath: p.RelPath,
					Role:    p.Role,
					Squad:   p.Squad,
				})
			}
			sort.Slice(resp.People, func(i, j int) bool {
				return strings.ToLower(resp.People[i].Name) < strings.ToLower(resp.People[j].Name)
			})
			return resp, nil
		}
	}

	peopleDir := filepath.Join(vaultDir, "notes", "pessoas")
	if _, err := os.Stat(peopleDir); os.IsNotExist(err) {
		peopleDir = filepath.Join(vaultDir, "meta", "pessoas")
	}

	entries, err := os.ReadDir(peopleDir)
	if err != nil {
		if os.IsNotExist(err) {
			return resp, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") || entry.Name() == "moc-pessoas.md" {
			continue
		}
		absPath := filepath.Join(peopleDir, entry.Name())
		slug := strings.TrimSuffix(entry.Name(), ".md")
		name := slug
		role := ""
		squad := ""
		relPath := filepath.Join("notes", "pessoas", entry.Name())

		if note, parseErr := vault.ParseNote(absPath, vaultDir); parseErr == nil && note != nil {
			if strings.TrimSpace(note.Title) != "" {
				name = strings.TrimSpace(strings.TrimPrefix(note.Title, "👤"))
				name = strings.TrimSpace(name)
			}
			if note.RelPath != "" {
				relPath = note.RelPath
			}
			role = firstExtra(note.FM.Extra, "role", "cargo")
			squad = firstExtra(note.FM.Extra, "squad", "equipe", "time")
		}

		resp.People = append(resp.People, &graphv1.PersonSummaryItem{
			Name:    name,
			Slug:    slug,
			RelPath: relPath,
			Role:    role,
			Squad:   squad,
		})
	}

	sort.Slice(resp.People, func(i, j int) bool {
		return strings.ToLower(resp.People[i].Name) < strings.ToLower(resp.People[j].Name)
	})
	return resp, nil
}

func firstExtra(extra map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := extra[k]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

// BuildPersonBriefing generates the 1:1 briefing for a specific colleague slug.
func BuildPersonBriefing(vaultDir, slug string) (*graphv1.GetPersonBriefingResponse, error) {
	svc := briefing.NewPreflightService(vaultDir)
	prof, err := svc.GenerateBriefing(slug)
	if err != nil {
		return nil, err
	}
	return &graphv1.GetPersonBriefingResponse{
		Name:                prof.Name,
		Slug:                prof.Slug,
		RelPath:             prof.RelPath,
		Cargo:               prof.Cargo,
		Squad:               prof.Squad,
		Relacao:             prof.Relacao,
		RecentInteractions:  append([]string(nil), prof.RecentInteractions...),
		Topics:              append([]string(nil), prof.Topics...),
		PendingTasks:        append([]string(nil), prof.PendingTasks...),
		RecentDailyMentions: append([]string(nil), prof.RecentDailyMentions...),
	}, nil
}

// BuildAgentActivity aggregates recent sessions across Antigravity, Claude Code, Codex, and Spooler.
func BuildAgentActivity(
	ctx context.Context,
	vaultDir string,
	cfg *config.Config,
	store routine.Store,
	spoolDir string,
	hours int,
	sessionID string,
) (*graphv1.GetAgentActivityResponse, error) {
	if hours <= 0 {
		hours = 24
	}
	if spoolDir == "" {
		spoolDir = spooler.DefaultDir()
	}

	sinceDuration := time.Duration(hours) * time.Hour
	cutoff := time.Now().Add(-sinceDuration)
	sinceUnix := cutoff.Unix()

	resp := &graphv1.GetAgentActivityResponse{
		Sessions: make([]*graphv1.AgentSessionItem, 0),
		Events:   make([]*graphv1.AgentEventItem, 0),
	}

	if cfg == nil && vaultDir != "" {
		if loaded, err := config.LoadConfig(vaultDir, "", false); err == nil {
			cfg = loaded
		}
	}

	sessionMap := make(map[string]*graphv1.AgentSessionItem)

	if store != nil {
		if sqlStore, ok := store.(*routine.SQLiteStore); ok {
			syncCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
			_, _ = telemetry.SyncSessionsToStore(syncCtx, sqlStore, telemetry.ParserOptions{
				SpoolDir: spoolDir,
				Since:    sinceDuration,
				Config:   cfg,
			})
			cancel()
		}
		if recs, err := store.GetRecentSessionsTelemetry(ctx, sinceUnix, 50); err == nil {
			for _, rec := range recs {
				if rec.SessionID == "" || rec.SessionID == "history" {
					continue
				}
				updatedAt := rec.UpdatedAt
				if updatedAt == 0 {
					updatedAt = rec.StartedAt
				}
				sessionMap[rec.SessionID] = &graphv1.AgentSessionItem{
					SessionId:        rec.SessionID,
					Agent:            rec.Agent,
					Model:            rec.Model,
					StartedAt:        rec.StartedAt,
					UpdatedAt:        updatedAt,
					TurnsCount:       int32(rec.TurnsCount), //nolint:gosec
					InputTokens:      rec.InputTokens,
					CacheReadTokens:  rec.CacheReadTokens,
					OutputTokens:     rec.OutputTokens,
					EstimatedCostUsd: rec.EstimatedCostUSD,
					SkillsUsed:       rec.SkillsUsed,
					IsActive:         time.Since(time.Unix(updatedAt, 0)) <= activeSessionWindow,
				}
			}
		}
	}

	mergeSpoolSessions(spoolDir, cutoff, sessionMap)

	for _, item := range sessionMap {
		resp.Sessions = append(resp.Sessions, item)
	}
	sort.Slice(resp.Sessions, func(i, j int) bool {
		if resp.Sessions[i].UpdatedAt != resp.Sessions[j].UpdatedAt {
			return resp.Sessions[i].UpdatedAt > resp.Sessions[j].UpdatedAt
		}
		return resp.Sessions[i].SessionId < resp.Sessions[j].SessionId
	})

	targetID := strings.TrimSpace(sessionID)
	if targetID == "" && len(resp.Sessions) > 0 {
		targetID = resp.Sessions[0].SessionId
	}
	if targetID != "" {
		resp.Events = readSpoolEvents(spoolDir, targetID, 12)
	}

	return resp, nil
}

func mergeSpoolSessions(spoolDir string, cutoff time.Time, sessionMap map[string]*graphv1.AgentSessionItem) {
	entries, err := os.ReadDir(spoolDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		isRaw := strings.HasSuffix(name, ".jsonl")
		isCompiled := strings.HasSuffix(name, ".jsonl.compiled")
		if !isRaw && !isCompiled {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		sid := strings.TrimSuffix(name, ".compiled")
		sid = strings.TrimSuffix(sid, ".jsonl")
		if sid == "" || sid == "history" {
			continue
		}

		modUnix := info.ModTime().Unix()
		existing, exists := sessionMap[sid]
		if !exists && info.ModTime().Before(cutoff) {
			continue
		}

		filePath := filepath.Join(spoolDir, name)
		evCount, firstAgent := inspectSpoolFile(filePath)
		if exists {
			if existing.SpoolFile == "" || isRaw {
				existing.SpoolFile = filePath
			}
			existing.EventCount += int32(evCount) //nolint:gosec
			if modUnix > existing.UpdatedAt {
				existing.UpdatedAt = modUnix
				existing.IsActive = time.Since(info.ModTime()) <= activeSessionWindow
			}
			continue
		}

		if firstAgent == "" {
			firstAgent = "agent"
		}
		sessionMap[sid] = &graphv1.AgentSessionItem{
			SessionId:  sid,
			Agent:      firstAgent,
			StartedAt:  modUnix,
			UpdatedAt:  modUnix,
			EventCount: int32(evCount), //nolint:gosec
			IsActive:   time.Since(info.ModTime()) <= activeSessionWindow,
			SpoolFile:  filePath,
		}
	}
}

func inspectSpoolFile(filePath string) (int, string) {
	f, err := os.Open(filePath) //nolint:gosec // controlled spool directory path
	if err != nil {
		return 0, ""
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 16*1024), 4*1024*1024)
	count := 0
	agent := ""
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		count++
		if agent == "" {
			var entry spooler.SpoolEntry
			if err := json.Unmarshal(line, &entry); err == nil && entry.Agent != "" {
				agent = entry.Agent
			}
		}
	}
	return count, agent
}

func readSpoolEvents(spoolDir, sessionID string, maxEvents int) []*graphv1.AgentEventItem {
	if maxEvents <= 0 {
		maxEvents = 12
	}
	cleanID := filepath.Base(sessionID)
	candidates := []string{
		filepath.Join(spoolDir, cleanID+".jsonl"),
		filepath.Join(spoolDir, cleanID+".jsonl.compiled"),
	}

	var all []*graphv1.AgentEventItem
	for _, path := range candidates {
		f, err := os.Open(path) //nolint:gosec // controlled spool directory path
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 16*1024), 4*1024*1024)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			var entry spooler.SpoolEntry
			if err := json.Unmarshal(line, &entry); err != nil {
				continue
			}
			snippet := string(entry.Payload)
			if len(snippet) > 120 {
				snippet = snippet[:117] + "..."
			}
			all = append(all, &graphv1.AgentEventItem{
				Timestamp:      entry.Timestamp,
				Event:          entry.Event,
				Agent:          entry.Agent,
				Cwd:            entry.CWD,
				PayloadSnippet: snippet,
			})
		}
		_ = f.Close()
		if len(all) > 0 {
			break
		}
	}

	if len(all) > maxEvents {
		all = all[len(all)-maxEvents:]
	}
	return all
}
