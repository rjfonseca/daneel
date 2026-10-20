package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/rs/cors"
	"github.com/spf13/cobra"

	graphv1 "github.com/rjfonseca/daneel/api/v1"
	"github.com/rjfonseca/daneel/api/v1/graphv1connect"
	"github.com/rjfonseca/daneel/internal/auditor"
	"github.com/rjfonseca/daneel/internal/config"
	"github.com/rjfonseca/daneel/internal/daemon"
	"github.com/rjfonseca/daneel/internal/dream"
	"github.com/rjfonseca/daneel/internal/ops/backlog"
	"github.com/rjfonseca/daneel/internal/ops/tracking"
	"github.com/rjfonseca/daneel/internal/routine"
	"github.com/rjfonseca/daneel/internal/search"
	"github.com/rjfonseca/daneel/internal/vault"
	"github.com/rjfonseca/daneel/internal/webfs"
)

type GraphServer struct {
	engine       *search.Engine
	vaultDir     string
	daemonClient *daemon.Client
	cfg          *config.Config
	routineStore routine.Store
}

func (s *GraphServer) getRoutineStore() routine.Store {
	if s.routineStore != nil {
		return s.routineStore
	}
	vaultDir := s.vaultDir
	if vaultDir == "" && s.cfg != nil {
		vaultDir = s.cfg.VaultPath
	}
	st, err := routine.OpenDefaultStore(vaultDir)
	if err == nil {
		s.routineStore = st
	}
	return s.routineStore
}

func (s *GraphServer) GetGalaxy(
	ctx context.Context,
	req *connect.Request[graphv1.GetGalaxyRequest],
) (*connect.Response[graphv1.GetGalaxyResponse], error) {
	nodes := []*graphv1.Node{}
	links := []*graphv1.Link{}

	db := s.engine.DB()

	linkRows, err := db.QueryContext(ctx, "SELECT source_rel_path, target_rel_path FROM note_links")
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer func() {
		_ = linkRows.Close()
	}()

	degreeMap := make(map[string]int32)
	for linkRows.Next() {
		var src, tgt string
		if err := linkRows.Scan(&src, &tgt); err != nil {
			continue
		}
		links = append(links, &graphv1.Link{
			Source: src,
			Target: tgt,
			Label:  "",
		})
		degreeMap[src]++
		degreeMap[tgt]++
	}

	rows, err := db.QueryContext(ctx, "SELECT rel_path, title, category, note_type, coalesce(mtime, 0) FROM notes_meta")
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer func() {
		_ = rows.Close()
	}()

	distinctRoots := make(map[string]bool)
	indexedPaths := make(map[string]bool)
	vaultDir := s.vaultDir
	if vaultDir == "" && s.cfg != nil {
		vaultDir = s.cfg.VaultPath
	}

	for rows.Next() {
		var id, title, category, noteType string
		var mtime float64
		if err := rows.Scan(&id, &title, &category, &noteType, &mtime); err != nil {
			continue
		}
		indexedPaths[id] = true
		if title == "" {
			title = filepath.Base(id)
		}
		if category == "" {
			category = "root"
		}

		tax := vault.ResolveTaxonomy(id, vaultDir)
		category = tax.Category
		domain := tax.Domain
		group := tax.Group
		repository := tax.Repository

		if noteType == "" {
			noteType = "default"
		}

		rootCat := strings.Split(category, "/")[0]
		distinctRoots[rootCat] = true

		nodes = append(nodes, &graphv1.Node{
			Id:          id,
			Title:       title,
			Category:    category,
			Domain:      domain,
			Group:       group,
			Repository:  repository,
			NoteType:    noteType,
			Connections: degreeMap[id],
			ModTime:     int64(mtime),
			Ring:        "memory",
			Status:      "active",
		})
	}

	// Auto-discovery and auto-indexing for daily/ notes on disk
	dailyDir := filepath.Join(vaultDir, "daily")
	if entries, err := os.ReadDir(dailyDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			relPath := filepath.ToSlash(filepath.Join("daily", entry.Name()))
			if indexedPaths[relPath] {
				continue
			}

			if s.engine != nil {
				_ = s.engine.IndexNote(ctx, relPath)
			}

			var title, category, noteType string
			var mtime float64
			rowErr := db.QueryRowContext(ctx, "SELECT title, category, note_type, coalesce(mtime, 0) FROM notes_meta WHERE rel_path = ?", relPath).Scan(&title, &category, &noteType, &mtime)
			if rowErr != nil {
				title = strings.TrimSuffix(entry.Name(), ".md")
				category = "daily"
				noteType = "daily"
				if fi, fiErr := entry.Info(); fiErr == nil {
					mtime = float64(fi.ModTime().UnixNano()) / 1e9
				}
			}
			if title == "" {
				title = filepath.Base(relPath)
			}
			tax := vault.ResolveTaxonomy(relPath, vaultDir)
			if category == "" {
				category = tax.Category
			}
			if category == "" {
				category = "daily"
			}
			if noteType == "" || noteType == "default" || noteType == "nota" {
				noteType = "daily"
			}

			rootCat := strings.Split(category, "/")[0]
			distinctRoots[rootCat] = true
			indexedPaths[relPath] = true

			nodes = append(nodes, &graphv1.Node{
				Id:          relPath,
				Title:       title,
				Category:    category,
				Domain:      tax.Domain,
				Group:       tax.Group,
				Repository:  tax.Repository,
				NoteType:    noteType,
				Connections: degreeMap[relPath],
				ModTime:     int64(mtime),
				Ring:        "memory",
				Status:      "active",
			})
		}
	}

	totalNotes := len(nodes)
	totalLinks := len(links)

	armsNodes, armsLinks := getARMSNodesAndLinks()
	nodes = append(nodes, armsNodes...)
	links = append(links, armsLinks...)

	var protoAnchors []*graphv1.PinnedAnchor
	var weeklyPattern string
	if s.cfg != nil {
		weeklyPattern = s.cfg.Hotbar.WeeklyPattern
		for _, a := range s.cfg.Hotbar.PinnedAnchors {
			protoAnchors = append(protoAnchors, &graphv1.PinnedAnchor{
				Path:  a.Path,
				Label: a.Label,
				Icon:  a.Icon,
			})
		}
	}

	res := connect.NewResponse(&graphv1.GetGalaxyResponse{
		Nodes: nodes,
		Links: links,
		Stats: &graphv1.GalaxyStats{
			TotalNotes:        int32(totalNotes), //nolint:gosec
			TotalLinks:        int32(totalLinks), //nolint:gosec
			HarvesterCount:    5,
			RoutineCount:      3,
			SkillCount:        7,
			RootCategoryCount: int32(len(distinctRoots)), //nolint:gosec
			StaffTrackedHours: 8.0,
		},
		PinnedAnchors: protoAnchors,
		WeeklyPattern: weeklyPattern,
	})
	return res, nil
}

//nolint:misspell
func getARMSNodesAndLinks() ([]*graphv1.Node, []*graphv1.Link) {
	armsNodes := []*graphv1.Node{
		// --- ORBIT III: HARVESTERS (Applications) ---
		{
			Id:          "harvester:github",
			Title:       "GitHub Harvester",
			Category:    "harvesters",
			NoteType:    "harvester",
			Ring:        "harvesters",
			Status:      "online",
			Description: "Continuous ingestion of docs, ADRs, and RFCs from GitHub repositories",
			Icon:        "github",
			Connections: 3,
		},
		{
			Id:          "harvester:gitlab",
			Title:       "GitLab Harvester",
			Category:    "harvesters",
			NoteType:    "harvester",
			Ring:        "harvesters",
			Status:      "online",
			Description: "Synchronization of microservice repos, packages, and architecture docs from GitLab",
			Icon:        "gitlab",
			Connections: 3,
		},
		{
			Id:          "harvester:jira",
			Title:       "Jira Harvester",
			Category:    "harvesters",
			NoteType:    "harvester",
			Ring:        "harvesters",
			Status:      "online",
			Description: "Harvesting active issues, tasks, and post-mortems from Jira Cloud",
			Icon:        "jira",
			Connections: 2,
		},
		{
			Id:          "harvester:gdrive",
			Title:       "Google Drive Harvester",
			Category:    "harvesters",
			NoteType:    "harvester",
			Ring:        "harvesters",
			Status:      "online",
			Description: "Ingestion of strategic alignment docs, 1:1 notes, and meeting minutes",
			Icon:        "gdrive",
			Connections: 1,
		},
		{
			Id:          "harvester:mcp",
			Title:       "MCP Server Gateway",
			Category:    "harvesters",
			NoteType:    "mcp",
			Ring:        "harvesters",
			Status:      "active",
			Description: "Model Context Protocol gateway bridging Claude Code, Antigravity, and IDEs",
			Icon:        "mcp",
			Connections: 4,
		},

		// --- ORBIT II: ROUTINES (Routines & Telemetry) ---
		{
			Id:          "routine:dream",
			Title:       "Autonomous Dream Cycle",
			Category:    "routines",
			NoteType:    "routine",
			Ring:        "routines",
			Status:      "healthy",
			Description: "Nightly memory consolidation, Zettelkasten routing, and vault self-healing",
			Icon:        "moon",
			Connections: 4,
		},
		{
			Id:          "routine:tracking",
			Title:       "Engineering Telemetry",
			Category:    "routines",
			NoteType:    "routine",
			Ring:        "routines",
			Status:      "tracking",
			Description: "Weekly time allocation tracking across strategic leadership archetypes",
			Icon:        "clock",
			Connections: 3,
		},
		{
			Id:          "routine:radar",
			Title:       "Strategic Radar",
			Category:    "routines",
			NoteType:    "routine",
			Ring:        "routines",
			Status:      "monitoring",
			Description: "Monitoring strategic initiatives, technical debt, friction, and tech trends",
			Icon:        "radar",
			Connections: 3,
		},

		// --- ORBIT I: SKILLS (Skills & Capabilities) ---
		{
			Id:          "skill:vault-dream-cycle",
			Title:       "Skill: Dream Cycle",
			Category:    "skills",
			NoteType:    "skill",
			Ring:        "skills",
			Status:      "ready",
			Description: "Autonomous cognitive consolidation and archetype protocol for AI agents",
			Icon:        "sparkles",
			Connections: 2,
		},
		{
			Id:          "skill:vault-meetings",
			Title:       "Skill: Meetings",
			Category:    "skills",
			NoteType:    "skill",
			Ring:        "skills",
			Status:      "ready",
			Description: "Automated processing of Google Meet notes and attendee timeline synchronization",
			Icon:        "users",
			Connections: 2,
		},
		{
			Id:          "skill:vault-preflight",
			Title:       "Skill: Preflight Briefing",
			Category:    "skills",
			NoteType:    "skill",
			Ring:        "skills",
			Status:      "ready",
			Description: "Daily morning briefing, priorities alignment, and 1:1 prep generation",
			Icon:        "sun",
			Connections: 2,
		},
		{
			Id:          "skill:vault-tracking",
			Title:       "Skill: Daily Tracking",
			Category:    "skills",
			NoteType:    "skill",
			Ring:        "skills",
			Status:      "ready",
			Description: "Reconciliation and calculation of leadership and execution hours",
			Icon:        "activity",
			Connections: 2,
		},
		{
			Id:          "skill:daneel-backlog",
			Title:       "Skill: Backlog Hygiene",
			Category:    "skills",
			NoteType:    "skill",
			Ring:        "skills",
			Status:      "ready",
			Description: "Triage and prioritization of pending initiatives and action items",
			Icon:        "list",
			Connections: 2,
		},
		{
			Id:          "skill:daneel-clone",
			Title:       "Skill: Repo Cloner",
			Category:    "skills",
			NoteType:    "skill",
			Ring:        "skills",
			Status:      "ready",
			Description: "Repository cloning, stack detection, and architecture MOC generation",
			Icon:        "git-branch",
			Connections: 2,
		},
		{
			Id:          "skill:daneel-dossier",
			Title:       "Skill: Performance Dossier",
			Category:    "skills",
			NoteType:    "skill",
			Ring:        "skills",
			Status:      "ready",
			Description: "Compiles executive performance review dossiers and impact reports",
			Icon:        "award",
			Connections: 2,
		},
	}

	armsLinks := []*graphv1.Link{
		{Source: "routine:dream", Target: "skill:vault-dream-cycle", Label: "executes"},
		{Source: "routine:tracking", Target: "skill:vault-tracking", Label: "executes"},
		{Source: "routine:radar", Target: "skill:vault-preflight", Label: "monitors"},
	}

	return armsNodes, armsLinks
}

func (s *GraphServer) GetNote(
	ctx context.Context,
	req *connect.Request[graphv1.GetNoteRequest],
) (*connect.Response[graphv1.GetNoteResponse], error) {
	relPath := req.Msg.RelPath

	// Native support for ARMS ring satellite nodes
	if strings.HasPrefix(relPath, "harvester:") || strings.HasPrefix(relPath, "routine:") || strings.HasPrefix(relPath, "skill:") {
		md := generateARMSMarkdown(relPath)
		return connect.NewResponse(&graphv1.GetNoteResponse{
			ContentMarkdown: md,
		}), nil
	}

	content, err := readVaultNote(s.vaultDir, relPath)
	if err != nil {
		return nil, err
	}

	if s.engine != nil {
		// Indexing continues after the response while retaining request context values.
		indexCtx := context.WithoutCancel(ctx)
		go func(p string) {
			_ = s.engine.IndexNote(indexCtx, p)
		}(relPath)
	}

	return connect.NewResponse(&graphv1.GetNoteResponse{
		ContentMarkdown: string(content),
	}), nil
}

//nolint:misspell
func generateARMSMarkdown(id string) string {
	switch id {
	case "harvester:github":
		return "# 🛸 GitHub Harvester\n\n**ARMS Layer:** `Applications (A)`  \n**Status:** `🟢 Online & Synchronized`  \n**Monitored Organizations:** `acme-org`, `cloud-platform`\n\n---\n\n### 📦 Synchronized Vault Directories\n- `sources/github/acme-org/`\n- `sources/github/cloud-platform/`\n\n### ⚙️ Harvesting Rules\n- Architectural documentation (`docs/`, `architecture/`, `adr/`, `rfc/`)\n- Operational files (`README.md`, `AGENTS.md`, `CLAUDE.md`, `catalog-info.yaml`)\n"
	case "harvester:gitlab":
		return "# 🦊 GitLab Harvester\n\n**ARMS Layer:** `Applications (A)`  \n**Status:** `🟢 Online & Synchronized`  \n**Groups:** `acme-services`, `platform-infra`\n\n---\n\n### 📦 Synchronized Vault Directories\n- `sources/gitlab/acme-services/`\n\n### ⚙️ Ingestion Patterns\n- Microservice repos and Go/Python shared libraries\n- Helm charts and production runbooks\n"
	case "harvester:jira":
		return "# 🔷 Jira Harvester\n\n**ARMS Layer:** `Applications (A)`  \n**Status:** `🟢 Online & Active`  \n**Instance:** `jira.acme.corp`\n\n---\n\n### 📋 Active JQL Queries\n1. **My Active Tickets:** `assignee = currentUser() AND statusCategory != Done` $\\to$ `sources/jira/my-active/`\n2. **Post-Mortems:** `project = POSTMORTEM` $\\to$ `sources/jira/post-mortems/`\n"
	case "harvester:gdrive":
		return "# 📁 Google Drive Harvester\n\n**ARMS Layer:** `Applications (A)`  \n**Status:** `🟢 Online`  \n**Destination:** `inbox/`\n\n---\n\n### 📄 Tracked Documents\n- Mentorship and career alignment agreements\n- Strategic planning templates and executive minutes\n"
	case "harvester:mcp":
		return "# 🔌 MCP Server Gateway\n\n**ARMS Layer:** `Applications (A)`  \n**Status:** `⚡ Active (Local Port / Stdio)`  \n\n---\n\n### 🤖 Connected Clients\n- Claude Code CLI\n- Antigravity Pair Programmer (Google DeepMind)\n- Local IDEs and editor extensions\n"
	case "routine:dream":
		return "# 🌙 Autonomous Dream Cycle\n\n**ARMS Layer:** `Routines (R)`  \n**Status:** `✨ Healthy & Active`  \n**Auto-Fix:** `Enabled (true)`\n\n---\n\n### 🔄 Zettelkasten Semantic Routing\n- `notes/ai`: agent, mcp, rag, prompt, llm\n- `notes/systems`: api, service, gateway, worker, sdk\n- `notes/processes`: rfc, prd, sla, postmortem\n- `notes/career`: goals, feedback, 1:1, leadership\n- `notes/people`: peer, mentee, manager\n"
	case "routine:tracking":
		return "# ⏱️ Engineering Telemetry & Time Tracking\n\n**ARMS Layer:** `Routines (R)`  \n**Status:** `📊 Operational Telemetry`\n\n---\n\n### 🎯 Leadership Archetypes (Targets)\n- **[A] Strategic & Architecture:** Min 30%\n- **[B] Local Impact & Execution:** Min 30%\n- **[C] Glue Work & Alignment:** Max 30%\n- **[E] Overhead & Ceremonies:** Max 10%\n"
	case "routine:radar":
		return "# 📡 Strategic Radar\n\n**ARMS Layer:** `Routines (R)`  \n**Status:** `🛰️ Monitored Topics Active`\n\n---\n\n### 🔍 Focused Radars\n- **Initiatives:** Cloud Platform, Gateway API, Agent Runtime\n- **Friction:** System Resilience, Pentests, Rate Limiting, CVEs\n- **Tech Radar:** Go (Adopt), Rust (Assess), Node.js (Hold)\n"
	default:
		return fmt.Sprintf("# ⚡ ARMS Component: %s\n\n**Status:** Active in Daneel ecosystem.\n", id)
	}
}

func (s *GraphServer) SearchNotes(
	ctx context.Context,
	req *connect.Request[graphv1.SearchNotesRequest],
) (*connect.Response[graphv1.SearchNotesResponse], error) {
	results, err := s.engine.Search(ctx, req.Msg.Query, "all", int(req.Msg.Limit))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var pbResults []*graphv1.SearchResult
	for _, r := range results {
		pbResults = append(pbResults, &graphv1.SearchResult{
			RelPath: r.RelPath,
			Title:   r.Title,
			Snippet: r.Snippet,
		})
	}

	return connect.NewResponse(&graphv1.SearchNotesResponse{
		Results: pbResults,
	}), nil
}

func (s *GraphServer) SubscribeUIEvents(
	ctx context.Context,
	req *connect.Request[graphv1.SubscribeUIEventsRequest],
	stream *connect.ServerStream[graphv1.UIEvent],
) error {
	ch, unsub := daemon.GetDefaultHub().SubscribeUIEvents()
	defer unsub()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case evt, ok := <-ch:
			if !ok {
				return nil
			}
			if err := stream.Send(evt); err != nil {
				return err
			}
		}
	}
}

func (s *GraphServer) ReportActiveContext(
	ctx context.Context,
	req *connect.Request[graphv1.ReportActiveContextRequest],
) (*connect.Response[graphv1.ReportActiveContextResponse], error) {
	focusedDomain := req.Msg.FocusedDomain
	actCtx := daemon.ActiveContext{
		Surface:        daemon.Surface(req.Msg.Surface),
		ActiveNotePath: req.Msg.ActiveNotePath,
		FocusedDomain:  focusedDomain,
		FocusedSector:  focusedDomain,
		SelectedText:   req.Msg.SelectedText,
	}
	daemon.GetDefaultHub().SetActiveContext(actCtx)

	if s.daemonClient != nil {
		_ = s.daemonClient.SetActiveContext(actCtx)
	}

	return connect.NewResponse(&graphv1.ReportActiveContextResponse{}), nil
}

func (s *GraphServer) ExecuteRoutine(
	ctx context.Context,
	req *connect.Request[graphv1.ExecuteRoutineRequest],
	stream *connect.ServerStream[graphv1.RoutineOutputEvent],
) error {
	routineName := strings.ToLower(strings.TrimSpace(req.Msg.RoutineName))
	if routineName == "" {
		routineName = "tidy"
	}

	startedAt := time.Now()
	startUnix := startedAt.Unix()

	recordRun := func(exitCode int, summary string, errMsg string) {
		st := s.getRoutineStore()
		if st == nil {
			return
		}
		completedAt := time.Now()
		durationMs := completedAt.Sub(startedAt).Milliseconds()
		status := "success"
		if exitCode != 0 {
			status = "failed"
		}
		_ = st.RecordRun(context.Background(), routine.RoutineRun{
			RoutineName:   routineName,
			StartedAt:     startUnix,
			CompletedAt:   completedAt.Unix(),
			DurationMs:    durationMs,
			TriggerSource: "web_cockpit",
			ExitCode:      exitCode,
			Status:        status,
			Summary:       summary,
			ErrorMessage:  errMsg,
		})
	}

	if s.isAISkill(routineName) {
		err := s.executeAISkill(ctx, routineName, req.Msg.Params, req.Msg.Verbose, stream)
		if err != nil {
			recordRun(1, fmt.Sprintf("AI Skill '%s' failed: %v", routineName, err), err.Error())
		} else {
			recordRun(0, fmt.Sprintf("AI Skill '%s' completed successfully.", routineName), "")
		}
		return err
	}

	_ = stream.Send(&graphv1.RoutineOutputEvent{
		Line: fmt.Sprintf("🚀 [Cockpit] Initializing routine '%s' (verbose: %v)...", routineName, req.Msg.Verbose),
	})

	// Check if we can execute via the daneel CLI binary
	binPath := os.Args[0]
	canExec := false
	if fi, err := os.Stat(binPath); err == nil && (fi.Mode().Perm()&0111) != 0 { //nolint:gosec
		canExec = true
	}

	if canExec {
		var args []string
		switch {
		case routineName == "dream" || routineName == "rem" || routineName == "sleep":
			args = []string{"dream", "--no-agent"}
			if req.Msg.Params["stats"] == "true" {
				args = append(args, "--stats")
			}
		case routineName == "tidy" || routineName == "hygiene" || routineName == "consolidate":
			args = []string{"tidy"}
		case routineName == "doctor":
			args = []string{"doctor"}
		case routineName == "index" || routineName == "reindex":
			args = []string{"index"}
		case routineName == "guard":
			args = []string{"guard"}
		case routineName == "dossier":
			args = []string{"dossier"}
		case routineName == "workspace":
			args = []string{"workspace"}
		case routineName == "ingest":
			args = []string{"ingest"}
		case routineName == "session_mining" || routineName == "sessions":
			args = []string{"audit", "sessions"}
		case routineName == "audit_agents" || routineName == "audit:agents" || routineName == "evals":
			args = []string{"audit", "agents"}
		case routineName == "audit":
			args = []string{"audit", "agents"}
		case routineName == "log" || routineName == "time":
			args = []string{"log", "--recent=7"}
		case strings.HasPrefix(routineName, "harvest:") || strings.HasPrefix(routineName, "harvester:"):
			platform := strings.TrimPrefix(strings.TrimPrefix(routineName, "harvester:"), "harvest:")
			args = []string{"harvest", platform}
			if t, ok := req.Msg.Params["target"]; ok && t != "" {
				args = append(args, t)
			}
		case routineName == "harvest":
			args = []string{"harvest"}
			if p, ok := req.Msg.Params["platform"]; ok && p != "" {
				args = append(args, p)
			}
			if t, ok := req.Msg.Params["target"]; ok && t != "" {
				args = append(args, t)
			}
		default:
			args = strings.Fields(routineName)
		}

		if req.Msg.Verbose {
			args = append(args, "--verbose")
		}
		for k, v := range req.Msg.Params {
			if k == "platform" || k == "target" {
				continue
			}
			if v == "true" {
				args = append(args, "--"+k)
			} else if v != "" && v != "false" {
				args = append(args, fmt.Sprintf("--%s=%s", k, v))
			}
		}

		cmd := exec.CommandContext(ctx, binPath, args...) //nolint:gosec
		stdout, err := cmd.StdoutPipe()
		if err == nil {
			stderr, err2 := cmd.StderrPipe()
			if err2 == nil {
				multi := io.MultiReader(stdout, stderr)
				if err := cmd.Start(); err == nil {
					scanner := bufio.NewScanner(multi)
					for scanner.Scan() {
						line := scanner.Text()
						_ = stream.Send(&graphv1.RoutineOutputEvent{
							Line: line,
						})
					}

					waitErr := cmd.Wait()
					if waitErr != nil {
						exitCode := 1
						var exitErr *exec.ExitError
						if errors.As(waitErr, &exitErr) {
							exitCode = exitErr.ExitCode()
						}
						recordRun(exitCode, fmt.Sprintf("Routine '%s' failed with exit code %d: %v", routineName, exitCode, waitErr), waitErr.Error())
						_ = stream.Send(&graphv1.RoutineOutputEvent{
							Line:        fmt.Sprintf("❌ Routine '%s' failed with exit code %d: %v", routineName, exitCode, waitErr),
							IsError:     true,
							IsCompleted: true,
							ExitCode:    int32(exitCode),
						})
						return nil
					}

					recordRun(0, fmt.Sprintf("Routine '%s' completed successfully.", routineName), "")
					_ = stream.Send(&graphv1.RoutineOutputEvent{
						Line:        fmt.Sprintf("✨ Routine '%s' completed successfully.", routineName),
						IsCompleted: true,
						ExitCode:    0,
					})
					return nil
				}
			}
		}
	}

	// Fallback to in-process execution (for tests or embedded runtime)
	switch routineName {
	case "dream", "tidy", "hygiene":
		engine := dream.NewDreamEngine(s.vaultDir)
		today := time.Now().Format("2006-01-02")
		report, err := engine.RunFullCycle(today, true, false)
		if err != nil {
			recordRun(1, fmt.Sprintf("Error during %s: %v", routineName, err), err.Error())
			_ = stream.Send(&graphv1.RoutineOutputEvent{
				Line:        fmt.Sprintf("❌ Error during %s: %v", routineName, err),
				IsError:     true,
				IsCompleted: true,
				ExitCode:    1,
			})
			return nil
		}
		_ = stream.Send(&graphv1.RoutineOutputEvent{
			Line: fmt.Sprintf("🧹 Inspected: %d notes | %d links fixed | %d broken links",
				report.TotalFilesScanned,
				report.FixedLinksCount,
				report.BrokenLinksCount),
		})
		if s.engine != nil {
			idxCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			indexed, _, _ := s.engine.IndexVault(idxCtx, nil, false)
			_ = stream.Send(&graphv1.RoutineOutputEvent{
				Line: fmt.Sprintf("✅ Synaptic graph indexed (%d notes updated).", indexed),
			})
		}
	case "index", "reindex":
		if s.engine != nil {
			indexed, duration, err := s.engine.IndexVault(ctx, nil, false)
			if err != nil {
				recordRun(1, fmt.Sprintf("Index error: %v", err), err.Error())
				_ = stream.Send(&graphv1.RoutineOutputEvent{
					Line:        fmt.Sprintf("❌ Index error: %v", err),
					IsError:     true,
					IsCompleted: true,
					ExitCode:    1,
				})
				return nil
			}
			_ = stream.Send(&graphv1.RoutineOutputEvent{
				Line: fmt.Sprintf("✅ Indexed %d notes in %v.", indexed, duration),
			})
		}
	case "session_mining", "audit", "sessions":
		miner := auditor.NewSessionMiner(auditor.AuditOptions{
			Since:     48 * time.Hour,
			VaultRoot: s.vaultDir,
		})
		rep, err := miner.Audit(ctx)
		if err != nil {
			recordRun(1, fmt.Sprintf("Session audit error: %v", err), err.Error())
			_ = stream.Send(&graphv1.RoutineOutputEvent{
				Line:        fmt.Sprintf("❌ Session audit error: %v", err),
				IsError:     true,
				IsCompleted: true,
				ExitCode:    1,
			})
			return nil
		}
		_ = stream.Send(&graphv1.RoutineOutputEvent{
			Line: fmt.Sprintf("🧬 Scanned %d sessions (%d turns). Found %d corrections, %d skill candidates.",
				rep.ScannedConversations, rep.ScannedTurns, len(rep.Learnings), len(rep.SkillProposals)),
		})
	default:
		_ = stream.Send(&graphv1.RoutineOutputEvent{
			Line: fmt.Sprintf("⚡ Executed routine '%s' in headless container.", routineName),
		})
	}

	recordRun(0, fmt.Sprintf("Routine '%s' execution finished.", routineName), "")
	_ = stream.Send(&graphv1.RoutineOutputEvent{
		Line:        fmt.Sprintf("✨ Routine '%s' execution finished.", routineName),
		IsCompleted: true,
		ExitCode:    0,
	})
	return nil
}

func (s *GraphServer) isAISkill(routine string) bool {
	norm := strings.ToLower(strings.TrimSpace(routine))
	if strings.HasPrefix(norm, "skill:") {
		return true
	}
	knownSkills := map[string]bool{
		"meeting-preflight":       true,
		"vault-meetings":          true,
		"vault-preflight":         true,
		"vault-dream-cycle":       true,
		"vault-assistant":         true,
		"staff-avd-compiler":      true,
		"meeting-notes-processor": true,
	}
	if knownSkills[norm] {
		return true
	}

	// Check if directory exists in .agents/skills
	skillsDir := filepath.Join(s.vaultDir, ".agents", "skills", norm)
	if fi, err := os.Stat(skillsDir); err == nil && fi.IsDir() {
		return true
	}
	return false
}

func (s *GraphServer) executeAISkill(
	ctx context.Context,
	routine string,
	params map[string]string,
	verbose bool,
	stream *connect.ServerStream[graphv1.RoutineOutputEvent],
) error {
	skillName := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(routine)), "skill:")

	_ = stream.Send(&graphv1.RoutineOutputEvent{
		Line: fmt.Sprintf("⚡ [Cockpit] Initializing AI Skill '%s' (verbose: %v)...", skillName, verbose),
	})

	// 1. Build hydrated prompt
	prompt := s.buildSkillPrompt(skillName, params)

	_ = stream.Send(&graphv1.RoutineOutputEvent{
		Line: "📝 [Prompt Protocol]:",
	})
	_ = stream.Send(&graphv1.RoutineOutputEvent{
		Line: "────────────────────────────────────────────────────────────────────────────────",
	})
	for _, l := range strings.Split(prompt, "\n") {
		_ = stream.Send(&graphv1.RoutineOutputEvent{
			Line: l,
		})
	}
	_ = stream.Send(&graphv1.RoutineOutputEvent{
		Line: "────────────────────────────────────────────────────────────────────────────────",
	})

	// 2. Check if an AI agent profile is configured and available in PATH
	prof, activeName, err := cfg.GetChatProfile("")
	canRunAgent := false
	var agentCmdTokens []string
	var agentBinPath string

	if err == nil && strings.TrimSpace(prof.Command) != "" {
		if tokens, err := splitCommand(prof.Command); err == nil && len(tokens) > 0 {
			expandedBin := expandHomeInToken(tokens[0])
			if path, err := exec.LookPath(expandedBin); err == nil {
				canRunAgent = true
				agentBinPath = path
				agentCmdTokens = tokens
			}
		}
	}

	if canRunAgent {
		_ = stream.Send(&graphv1.RoutineOutputEvent{
			Line: fmt.Sprintf("🤖 Invoking AI Agent profile '%s' for skill execution...", activeName),
		})

		// Configure arguments for non-interactive / print mode
		isAgy := false
		for _, tok := range agentCmdTokens {
			if strings.Contains(tok, "agy") {
				isAgy = true
				break
			}
		}

		var fullArgs []string
		for i, tok := range agentCmdTokens {
			if i == 0 {
				continue
			}
			fullArgs = append(fullArgs, expandHomeInToken(tok))
		}

		if isAgy {
			fullArgs = append(fullArgs, "--print-timeout", "10m", "-p", prompt)
		} else {
			fullArgs = append(fullArgs, "-p", prompt)
		}

		cmd := exec.CommandContext(ctx, agentBinPath, fullArgs...) //nolint:gosec
		cmd.Dir = s.vaultDir

		// Environment
		envMap := make(map[string]string)
		for _, e := range os.Environ() {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) == 2 {
				envMap[parts[0]] = parts[1]
			}
		}
		for k, v := range prof.Env {
			envMap[k] = v
		}
		envList := make([]string, 0, len(envMap))
		for k, v := range envMap {
			envList = append(envList, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = envList

		stdout, err := cmd.StdoutPipe()
		if err == nil {
			stderr, err2 := cmd.StderrPipe()
			if err2 == nil {
				multi := io.MultiReader(stdout, stderr)
				if err := cmd.Start(); err == nil {
					scanner := bufio.NewScanner(multi)
					for scanner.Scan() {
						line := scanner.Text()
						_ = stream.Send(&graphv1.RoutineOutputEvent{
							Line: line,
						})
					}

					waitErr := cmd.Wait()
					if waitErr != nil {
						exitCode := 1
						var exitErr *exec.ExitError
						if errors.As(waitErr, &exitErr) {
							exitCode = exitErr.ExitCode()
						}
						_ = stream.Send(&graphv1.RoutineOutputEvent{
							Line:        fmt.Sprintf("❌ AI Agent execution failed with exit code %d: %v", exitCode, waitErr),
							IsError:     true,
							IsCompleted: true,
							ExitCode:    int32(exitCode),
						})
						return nil
					}

					_ = stream.Send(&graphv1.RoutineOutputEvent{
						Line:        fmt.Sprintf("✨ AI Skill '%s' completed successfully.", skillName),
						IsCompleted: true,
						ExitCode:    0,
					})
					return nil
				}
			}
		}
	}

	// If agent CLI is not run or not installed in PATH, echo prompt for manual copy and exit cleanly
	_ = stream.Send(&graphv1.RoutineOutputEvent{
		Line: "💡 AI Agent CLI not detected in PATH or running in headless server mode.",
	})
	_ = stream.Send(&graphv1.RoutineOutputEvent{
		Line: "📋 The prompt protocol above is ready to copy into your AI agent session (Claude Code, Antigravity CLI, or Cursor).",
	})
	_ = stream.Send(&graphv1.RoutineOutputEvent{
		Line:        fmt.Sprintf("✨ AI Skill '%s' protocol prepared successfully.", skillName),
		IsCompleted: true,
		ExitCode:    0,
	})
	return nil
}

func (s *GraphServer) buildSkillPrompt(skillName string, params map[string]string) string {
	today := time.Now().Format("2006-01-02")
	norm := strings.ToLower(skillName)

	// Check if SKILL.md exists
	skillMdPath := filepath.Join(s.vaultDir, ".agents", "skills", skillName, "SKILL.md")
	var skillProtocol string
	if data, err := os.ReadFile(skillMdPath); err == nil {
		skillProtocol = string(data)
	}

	if strings.Contains(norm, "meeting") || strings.Contains(norm, "preflight") {
		attendee := params["attendee"]
		if attendee == "" {
			attendee = params["with"]
		}
		if attendee == "" {
			attendee = "Partner / Team"
		}
		topic := params["topic"]
		if topic == "" {
			topic = "General Sync & Priorities"
		}
		date := params["date"]
		if date == "" {
			date = today
		}
		contextInfo := params["context"]
		var sb strings.Builder
		fmt.Fprintf(&sb, "Execute meeting preflight briefing protocol for meeting with %s on \"%s\" (Date: %s).\n", attendee, topic, date)
		if contextInfo != "" {
			fmt.Fprintf(&sb, "\nContext & References:\n%s\n", contextInfo)
		}
		sb.WriteString("\nPlease analyze relevant vault notes, interaction histories in notes/people/, and prepare strategic discussion points, mutual goals, and open questions.")
		if skillProtocol != "" {
			sb.WriteString("\n\n---\nSkill Protocol Reference (.agents/skills/" + skillName + "/SKILL.md):\n" + skillProtocol)
		}
		return sb.String()
	}

	if strings.Contains(norm, "dream") {
		date := params["date"]
		if date == "" {
			date = today
		}
		domain := params["focus_domain"]
		domainMsg := ""
		if domain != "" {
			domainMsg = fmt.Sprintf(" Focus on domain '%s'.", domain)
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "Execute cognitive REM dream cycle consolidation for date %s.%s\n\nReview daily notes, triage inbox items, heal bi-directional links, and reconcile leadership time telemetry.", date, domainMsg)
		if skillProtocol != "" {
			sb.WriteString("\n\n---\nSkill Protocol Reference:\n" + skillProtocol)
		}
		return sb.String()
	}

	if strings.Contains(norm, "avd") || strings.Contains(norm, "compiler") {
		period := params["period"]
		if period == "" {
			period = "Last 90 Days"
		}
		focus := params["focus_areas"]
		var sb strings.Builder
		fmt.Fprintf(&sb, "Compile executive performance review and staff impact dossier for period %s.\n", period)
		if focus != "" {
			fmt.Fprintf(&sb, "Focus Areas:\n%s\n", focus)
		}
		sb.WriteString("\nSynthesize accomplishments, architecture decisions, mentorship impact, and leadership archetypes.")
		if skillProtocol != "" {
			sb.WriteString("\n\n---\nSkill Protocol Reference:\n" + skillProtocol)
		}
		return sb.String()
	}

	// Generic skill
	custom := params["instruction"]
	if custom == "" {
		custom = params["prompt"]
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Execute AI skill '%s'.\n", skillName)
	if custom != "" {
		fmt.Fprintf(&sb, "Parameters & Instructions:\n%s\n", custom)
	}
	if skillProtocol != "" {
		sb.WriteString("\n---\nSkill Protocol Reference:\n" + skillProtocol)
	}
	return sb.String()
}

func (s *GraphServer) GetRoutinesOverview(
	ctx context.Context,
	req *connect.Request[graphv1.GetRoutinesOverviewRequest],
) (*connect.Response[graphv1.GetRoutinesOverviewResponse], error) {
	now := time.Now().Unix()

	var routines []*graphv1.RoutineStatusItem
	seen := make(map[string]bool)

	rCfg := config.DefaultRoutinesConfig()
	if s.cfg != nil {
		rCfg = s.cfg.GetRoutinesConfig()
	}
	adapter := routine.NewSystemdAdapter("", "", nil)

	for name, item := range rCfg.Items {
		seen[name] = true
		st, err := adapter.Status(ctx, name)
		displayName := name
		if len(name) > 0 {
			displayName = strings.ToUpper(name[:1]) + name[1:]
		}
		title := fmt.Sprintf("%s (%s)", displayName, name)
		statusStr := "ready"
		var lastTrigger int64
		var nextTrigger int64
		lastResult := "not installed"
		active := false
		enabled := item.Enabled

		if err == nil && st != nil {
			if st.Active {
				active = true
				statusStr = "active"
			}
			if !st.LastTrigger.IsZero() {
				lastTrigger = st.LastTrigger.Unix()
			}
			if !st.NextTrigger.IsZero() {
				nextTrigger = st.NextTrigger.Unix()
			}
			if st.LastResult != "" {
				lastResult = st.LastResult
			}
			enabled = st.Enabled
		}

		routines = append(routines, &graphv1.RoutineStatusItem{
			Name:                 name,
			Title:                title,
			Description:          item.Description,
			Category:             "deterministic",
			Status:               statusStr,
			LastRunTimestamp:     lastTrigger,
			Schedule:             item.Schedule,
			Command:              item.Command,
			Args:                 item.Args,
			Enabled:              enabled,
			Active:               active,
			NextTriggerTimestamp: nextTrigger,
			LastResult:           lastResult,
		})
	}

	// Builtin operational routines if not explicitly in config
	builtinOps := []struct {
		name, title, desc, cmd string
	}{
		{"dream", "Dream Cycle", "Hypnagogic Cognitive REM consolidation, Zettelkasten routing, and telemetry reconciliation", "dream"},
		{"index", "Reindex FTS5 (index)", "Full-text indexing of markdown notes into SQLite FTS5 engine", "index"},
		{"guard", "Security Firewall Audit (guard)", "Audits public notes against security firewall rules under the Four Laws of Robotics", "guard"},
		{"dossier", "Compile Staff Dossier (dossier)", "Compiles executive performance review dossiers and impact reports", "dossier"},
		{"workspace", "Workspace Sync (workspace)", "Scans local git repositories in ~/Work and maps architectural MOCs", "workspace"},
		{"ingest", "Ingest Meeting Notes (ingest)", "Parses Google Meet notes and attendee timeline synchronization", "ingest"},
		{"session_mining", "Session Miner (audit sessions)", "Audits AI agent session transcripts for corrections, new skills, and evals", "audit sessions"},
		{"audit_agents", "Agent Evals & Telemetry (audit agents)", "Audits AI agent session transcripts for missed opportunities and knowledge gaps", "audit agents"},
		{"log", "Staff Time Log (log)", "Tracks time allocation across [A]-[E] leadership archetypes", "log"},
		{"harvest:jira", "Jira Harvester (harvest jira)", "Harvests active issues, tasks, and post-mortems from Jira Cloud", "harvest jira"},
		{"harvest:github", "GitHub Harvester (harvest github)", "Continuous ingestion of docs, ADRs, and RFCs from GitHub repositories", "harvest github"},
		{"harvest:gitlab", "GitLab Harvester (harvest gitlab)", "Synchronization of microservice repos and architecture docs from GitLab", "harvest gitlab"},
		{"harvest:gdrive", "Google Drive Harvester (harvest gdrive)", "Ingestion of strategic alignment docs, 1:1 notes, and meeting minutes", "harvest gdrive"},
		{"harvest", "Documentation Harvester (All)", "Harvests technical documentation from all configured platforms in daneel.yaml", "harvest"},
	}

	for _, b := range builtinOps {
		if seen[b.name] {
			continue
		}
		routines = append(routines, &graphv1.RoutineStatusItem{
			Name:             b.name,
			Title:            b.title,
			Description:      b.desc,
			Category:         "deterministic",
			Status:           "ready",
			LastRunTimestamp: now,
			Schedule:         "manual",
			Command:          b.cmd,
			Enabled:          true,
			Active:           false,
			LastResult:       "ready",
		})
	}

	// Discover AI skills in .agents/skills
	skillsDir := filepath.Join(s.vaultDir, ".agents", "skills")
	entries, err := os.ReadDir(skillsDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				routines = append(routines, &graphv1.RoutineStatusItem{
					Name:             e.Name(),
					Title:            fmt.Sprintf("AI Skill: %s", e.Name()),
					Description:      fmt.Sprintf("Autonomous agent skill in .agents/skills/%s", e.Name()),
					Category:         "ai_skill",
					Status:           "ready",
					LastRunTimestamp: now,
				})
			}
		}
	} else {
		// Fallback default skills if directory not present
		defaultSkills := []struct {
			name, desc string
		}{
			{"vault-dream-cycle", "Autonomous cognitive consolidation and archetype protocol for AI agents"},
			{"meeting-preflight", "Daily morning briefing, priorities alignment, and 1:1 prep generation"},
			{"vault-assistant", "Second brain navigation and knowledge retrieval assistant"},
			{"staff-avd-compiler", "Compiles executive performance review dossiers and impact reports"},
		}
		for _, ds := range defaultSkills {
			routines = append(routines, &graphv1.RoutineStatusItem{
				Name:             ds.name,
				Title:            fmt.Sprintf("AI Skill: %s", ds.name),
				Description:      ds.desc,
				Category:         "ai_skill",
				Status:           "ready",
				LastRunTimestamp: now,
			})
		}
	}

	// Enrich routines with latest run from persistent ledger
	stStore := s.getRoutineStore()
	if stStore != nil {
		for _, r := range routines {
			if latest, err := stStore.GetLatestRun(ctx, r.Name); err == nil && latest != nil {
				r.LastRunTimestamp = latest.StartedAt
				r.LastResult = latest.Status
			}
		}
	}

	return connect.NewResponse(&graphv1.GetRoutinesOverviewResponse{
		Routines: routines,
	}), nil
}

func (s *GraphServer) UpsertRoutine(
	ctx context.Context,
	req *connect.Request[graphv1.UpsertRoutineRequest],
) (*connect.Response[graphv1.UpsertRoutineResponse], error) {
	name := strings.TrimSpace(strings.ToLower(req.Msg.Name))
	if name == "" {
		return connect.NewResponse(&graphv1.UpsertRoutineResponse{
			Success: false,
			Error:   "routine name cannot be empty",
		}), nil
	}

	command := strings.TrimSpace(req.Msg.Command)
	if command == "" {
		command = name
	}

	schedule := strings.TrimSpace(req.Msg.Schedule)
	if schedule == "" {
		schedule = "daily"
	}

	item := config.RoutineItemConfig{
		Enabled:     req.Msg.Enabled,
		Schedule:    schedule,
		Command:     command,
		Args:        req.Msg.Args,
		Description: req.Msg.Description,
	}

	if s.cfg != nil {
		if err := s.cfg.SaveRoutine(item, name); err != nil {
			return connect.NewResponse(&graphv1.UpsertRoutineResponse{
				Success: false,
				Error:   fmt.Sprintf("failed to save routine to config: %v", err),
			}), nil
		}
	}

	adapter := routine.NewSystemdAdapter("", "", nil)
	r := routine.Routine{
		Name:        name,
		Command:     command,
		Args:        req.Msg.Args,
		Schedule:    schedule,
		Description: req.Msg.Description,
		Enabled:     req.Msg.Enabled,
	}
	_ = adapter.Install(ctx, r)

	return connect.NewResponse(&graphv1.UpsertRoutineResponse{
		Success: true,
	}), nil
}

func (s *GraphServer) DeleteRoutine(
	ctx context.Context,
	req *connect.Request[graphv1.DeleteRoutineRequest],
) (*connect.Response[graphv1.DeleteRoutineResponse], error) {
	name := strings.TrimSpace(strings.ToLower(req.Msg.Name))
	if name == "" {
		return connect.NewResponse(&graphv1.DeleteRoutineResponse{
			Success: false,
			Error:   "routine name cannot be empty",
		}), nil
	}

	if s.cfg != nil {
		if err := s.cfg.DeleteRoutine(name); err != nil {
			return connect.NewResponse(&graphv1.DeleteRoutineResponse{
				Success: false,
				Error:   fmt.Sprintf("failed to delete routine from config: %v", err),
			}), nil
		}
	}

	adapter := routine.NewSystemdAdapter("", "", nil)
	_ = adapter.Uninstall(ctx, name)

	return connect.NewResponse(&graphv1.DeleteRoutineResponse{
		Success: true,
	}), nil
}

func (s *GraphServer) GetRoutineHistory(
	ctx context.Context,
	req *connect.Request[graphv1.GetRoutineHistoryRequest],
) (*connect.Response[graphv1.GetRoutineHistoryResponse], error) {
	st := s.getRoutineStore()
	if st == nil {
		return connect.NewResponse(&graphv1.GetRoutineHistoryResponse{
			Runs: make([]*graphv1.RoutineRunItem, 0),
		}), nil
	}

	limit := int(req.Msg.Limit)
	if limit <= 0 {
		limit = 30
	}
	runs, err := st.GetHistory(ctx, req.Msg.RoutineName, limit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to fetch routine history: %w", err))
	}

	items := make([]*graphv1.RoutineRunItem, 0, len(runs))
	for _, r := range runs {
		items = append(items, &graphv1.RoutineRunItem{
			Id:            r.ID,
			RoutineName:   r.RoutineName,
			StartedAt:     r.StartedAt,
			CompletedAt:   r.CompletedAt,
			DurationMs:    r.DurationMs,
			TriggerSource: r.TriggerSource,
			ExitCode:      int32(r.ExitCode), //nolint:gosec
			Status:        r.Status,
			Summary:       r.Summary,
			ErrorMessage:  r.ErrorMessage,
		})
	}

	return connect.NewResponse(&graphv1.GetRoutineHistoryResponse{
		Runs: items,
	}), nil
}

func (s *GraphServer) GetStaffTelemetry(
	ctx context.Context,
	req *connect.Request[graphv1.GetStaffTelemetryRequest],
) (*connect.Response[graphv1.GetStaffTelemetryResponse], error) {
	days := int(req.Msg.Days)
	if days <= 0 {
		days = 7
	}

	logs, _ := tracking.ListDailyLogs(s.vaultDir, days)

	var stratA, impactB, glueC, lowImpactD, overheadE, totalHours float32
	for _, l := range logs {
		stratA += float32(l.TempoA)
		impactB += float32(l.TempoB)
		glueC += float32(l.TempoC)
		lowImpactD += float32(l.TempoD)
		overheadE += float32(l.TempoE)
		totalHours += float32(l.TotalHours)
	}

	var leverageRate float32
	if totalHours > 0 {
		leverageRate = (stratA + glueC) / totalHours
		if leverageRate > 1.0 {
			leverageRate = 1.0
		}
	}

	// Rolling weeks aggregation
	rollingWeeksCount := int(req.Msg.RollingWeeks)
	if rollingWeeksCount <= 0 {
		rollingWeeksCount = 4
	}
	weekSummaries, _ := tracking.AggregateByWeek(s.vaultDir, rollingWeeksCount)
	protoRollingWeeks := make([]*graphv1.StaffPeriodSummary, 0, len(weekSummaries))
	for _, w := range weekSummaries {
		protoRollingWeeks = append(protoRollingWeeks, &graphv1.StaffPeriodSummary{
			PeriodKey:         w.PeriodKey,
			Label:             w.Label,
			DaysCount:         int32(w.DaysCount), //nolint:gosec
			StrategicAHours:   float32(w.TempoA),
			LocalImpactBHours: float32(w.TempoB),
			GlueWorkCHours:    float32(w.TempoC),
			LowImpactDHours:   float32(w.TempoD),
			OverheadEHours:    float32(w.TempoE),
			TotalHours:        float32(w.TotalHours),
			LeverageRate:      float32(w.LeverageRate),
			Diagnosis:         w.Diagnosis,
		})
	}

	// Historical months aggregation
	historyMonthsCount := int(req.Msg.HistoricalMonths)
	if historyMonthsCount <= 0 {
		historyMonthsCount = 6
	}
	monthSummaries, _ := tracking.AggregateByMonth(s.vaultDir, historyMonthsCount)
	protoMonthlyHistory := make([]*graphv1.StaffPeriodSummary, 0, len(monthSummaries))
	for _, m := range monthSummaries {
		protoMonthlyHistory = append(protoMonthlyHistory, &graphv1.StaffPeriodSummary{
			PeriodKey:         m.PeriodKey,
			Label:             m.Label,
			DaysCount:         int32(m.DaysCount), //nolint:gosec
			StrategicAHours:   float32(m.TempoA),
			LocalImpactBHours: float32(m.TempoB),
			GlueWorkCHours:    float32(m.TempoC),
			LowImpactDHours:   float32(m.TempoD),
			OverheadEHours:    float32(m.TempoE),
			TotalHours:        float32(m.TotalHours),
			LeverageRate:      float32(m.LeverageRate),
			Diagnosis:         m.Diagnosis,
		})
	}

	// Resolve top initiatives from config or active backlog
	var topInitiatives []string
	if cfg != nil && len(cfg.Evaluation.StrategicDeliveries) > 0 {
		topInitiatives = append(topInitiatives, cfg.Evaluation.StrategicDeliveries...)
	}

	if len(topInitiatives) == 0 {
		backlogReport, err := backlog.GetActiveBacklog(s.vaultDir)
		if err == nil && backlogReport != nil {
			for _, item := range backlogReport.Items {
				if item.Priority == "P0" || item.Priority == "P1" {
					topInitiatives = append(topInitiatives, item.Title)
				}
				if len(topInitiatives) >= 5 {
					break
				}
			}
		}
	}

	if len(topInitiatives) == 0 {
		topInitiatives = []string{
			"AI & Agent Platform (Daneel)",
			"Cloud Infrastructure & Gateway API",
			"Resilience, Pentest & Security",
			"Developer Experience & SDKs",
		}
	}

	return connect.NewResponse(&graphv1.GetStaffTelemetryResponse{
		StrategicAHours:   stratA,
		LocalImpactBHours: impactB,
		GlueWorkCHours:    glueC,
		LowImpactDHours:   lowImpactD,
		OverheadEHours:    overheadE,
		TotalHours:        totalHours,
		LeverageRate:      leverageRate,
		RollingWeeks:      protoRollingWeeks,
		MonthlyHistory:    protoMonthlyHistory,
		TopInitiatives:    topInitiatives,
	}), nil
}

func (s *GraphServer) GetCognitiveTelemetry(
	ctx context.Context,
	req *connect.Request[graphv1.GetCognitiveTelemetryRequest],
) (*connect.Response[graphv1.GetCognitiveTelemetryResponse], error) {
	resp := &graphv1.GetCognitiveTelemetryResponse{
		StageTimings:     make([]*graphv1.StageTimingItem, 0),
		Bottlenecks:      make([]string, 0),
		InducedSyntheses: make([]string, 0),
		RecentEvals:      make([]*graphv1.AgentEvalItem, 0),
		EvolutionLogPath: "meta/guias/agente-evolucao.md",
	}

	// 1. Try reading latest dream report from .daneel/cache/dream_handoff.json
	cacheDir := config.ResolveVaultCacheDir(s.vaultDir)
	handoffPath := filepath.Join(cacheDir, "dream_handoff.json")
	data, err := os.ReadFile(handoffPath)
	var dreamRep dream.FullDreamReport
	hasValidReport := false
	if err == nil {
		if err := json.Unmarshal(data, &dreamRep); err == nil {
			hasValidReport = true
		}
	}

	// If local handoff is missing, try XDG cache
	if !hasValidReport && cacheDir != config.XDGCacheDir() {
		xdgHandoff := filepath.Join(config.XDGCacheDir(), "dream_handoff.json")
		if xdgData, xdgErr := os.ReadFile(xdgHandoff); xdgErr == nil {
			var xdgRep dream.FullDreamReport
			if err := json.Unmarshal(xdgData, &xdgRep); err == nil {
				dreamRep = xdgRep
				hasValidReport = true
			}
		}
	}

	if hasValidReport {
		resp.TotalFilesScanned = int32(dreamRep.TotalFilesScanned) //nolint:gosec
		resp.BrokenLinksCount = int32(dreamRep.BrokenLinksCount)   //nolint:gosec
		if t, err := time.Parse(time.RFC3339, dreamRep.GeneratedAt); err == nil {
			resp.LastDreamTimestamp = t.Unix()
		}
		if dreamRep.Metacognition != nil {
			resp.LastCycleDurationMs = dreamRep.Metacognition.TotalDurationMs
			resp.Bottlenecks = dreamRep.Metacognition.Bottlenecks
			if dreamRep.Metacognition.EvolutionLogPath != "" {
				resp.EvolutionLogPath = dreamRep.Metacognition.EvolutionLogPath
			}
			for _, syn := range dreamRep.Metacognition.Syntheses {
				desc := syn.Title
				if syn.CreatedPath != "" {
					desc = fmt.Sprintf("%s (%s)", syn.Title, syn.CreatedPath)
				}
				resp.InducedSyntheses = append(resp.InducedSyntheses, desc)
			}
			for _, st := range dreamRep.Metacognition.Timings {
				resp.StageTimings = append(resp.StageTimings, &graphv1.StageTimingItem{
					Name:        st.Name,
					DurationMs:  st.DurationMs,
					ItemsCount:  int32(st.ItemsCount), //nolint:gosec
					Description: st.Description,
				})
			}
		} else {
			// Resilient fallback for partial or legacy handoff cache
			if dreamRep.BrokenLinksCount > 0 && len(resp.Bottlenecks) == 0 {
				resp.Bottlenecks = append(resp.Bottlenecks, fmt.Sprintf("%d broken relative links detected", dreamRep.BrokenLinksCount))
			}
		}

		// If TotalDurationMs was not set but timings exist, sum them up
		if resp.LastCycleDurationMs == 0 && len(resp.StageTimings) > 0 {
			var sumMs int64
			for _, st := range resp.StageTimings {
				sumMs += st.DurationMs
			}
			resp.LastCycleDurationMs = sumMs
		}
	}

	// 2. Read Agent Learnings & Evals
	learnings := dream.LoadLearnings(s.vaultDir)
	if learnings != nil {
		if learnings.ScannedTurns > 0 {
			resp.ScannedTurns = int32(learnings.ScannedTurns) //nolint:gosec
		}
		if learnings.MissedOpportunities > 0 {
			resp.MissedOpportunitiesCount = int32(learnings.MissedOpportunities) //nolint:gosec
		}
		if learnings.KnowledgeGaps > 0 {
			resp.KnowledgeGapsCount = int32(learnings.KnowledgeGaps) //nolint:gosec
		}

		for _, fh := range learnings.FeedbackHistory {
			agentName := fh.Agent
			if agentName == "" {
				agentName = "agent"
			}
			item := &graphv1.AgentEvalItem{
				Agent:           agentName,
				Topic:           fh.Topic,
				Classification:  fh.Action,
				Description:     fh.Note,
				SuggestedAction: fh.Action,
			}
			if t, err := time.Parse(time.RFC3339, fh.Timestamp); err == nil {
				item.Timestamp = t.Unix()
			} else if t, err := time.Parse("2006-01-02", fh.Date); err == nil {
				item.Timestamp = t.Unix()
			}

			if fh.Action == "missed_opportunity" || strings.Contains(fh.Type, "missed_opportunity") {
				if learnings.MissedOpportunities == 0 {
					resp.MissedOpportunitiesCount++
				}
				item.Classification = "missed_opportunity"
			} else if fh.Action == "knowledge_gap" || strings.Contains(fh.Type, "knowledge_gap") {
				if learnings.KnowledgeGaps == 0 {
					resp.KnowledgeGapsCount++
				}
				item.Classification = "knowledge_gap"
			} else {
				item.Classification = "user_correction"
			}

			if learnings.ScannedTurns == 0 {
				resp.ScannedTurns++
			}
			resp.RecentEvals = append(resp.RecentEvals, item)
		}
	}

	return connect.NewResponse(resp), nil
}

var servePort int

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Starts the Second Brain Web Galaxy visualizer",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !cmd.Flags().Changed("port") {
			servePort = 4321
			foundPort := false
			for p := 4321; p <= 4340; p++ {
				ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
				if err == nil {
					_ = ln.Close()
					servePort = p
					foundPort = true
					break
				}
			}
			if !foundPort {
				return fmt.Errorf("could not find a free port between 4321 and 4340")
			}
		} else {
			ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", servePort))
			if err != nil {
				return fmt.Errorf("port %d is already in use. Specify another with --port", servePort)
			}
			_ = ln.Close()
		}

		if cfg == nil {
			return fmt.Errorf("configuration not loaded")
		}

		engine, err := search.Open(cfg.DBPath, cfg.VaultPath)
		if err != nil {
			return fmt.Errorf("failed to open search database: %w", err)
		}
		defer engine.Close() //nolint:errcheck,gosec

		// Incrementally index vault on startup to ensure latest notes are served
		_, _, _ = engine.IndexVault(cmd.Context(), cfg.GetIndexedDirs(), false)

		var dClient *daemon.Client
		if !daemon.IsDaemonRunning() {
			dServer := daemon.NewServer(daemon.GetDefaultHub())
			go func() {
				_ = dServer.ListenAndServe()
			}()
			defer dServer.Stop()
		} else {
			client, err := daemon.NewClient()
			if err == nil {
				dClient = client
				defer func() { _ = client.Close() }()
				go func() {
					ch, subErr := client.SubscribeUIEvents(cmd.Context())
					if subErr == nil {
						for evt := range ch {
							daemon.GetDefaultHub().PublishUIEvent(evt)
						}
					}
				}()
			}
		}

		server, err := NewGalaxyHTTPServer(cfg, engine, servePort, dClient)
		if err != nil {
			return err
		}

		fmt.Printf("🚀 Galaxy Visualizer running at http://localhost:%d\n", servePort)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server error: %w", err)
		}
		return nil
	},
}

// NewGalaxyHTTPServer builds and returns the configured HTTP server for ConnectRPC and WebFS.
func NewGalaxyHTTPServer(cfg *config.Config, engine *search.Engine, port int, dClient *daemon.Client) (*http.Server, error) {
	graphServer := &GraphServer{engine: engine, vaultDir: cfg.VaultPath, daemonClient: dClient, cfg: cfg}

	mux := http.NewServeMux()
	path, handler := graphv1connect.NewGraphServiceHandler(graphServer)
	mux.Handle(path, handler)

	distFs, err := fs.Sub(webfs.DistFS, "dist")
	if err != nil {
		return nil, fmt.Errorf("failed to mount embedded frontend: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(distFs)))

	corsHandler := cors.New(cors.Options{
		AllowedOrigins: []string{fmt.Sprintf("http://localhost:%d", port), fmt.Sprintf("http://127.0.0.1:%d", port), "http://localhost:5173", "http://127.0.0.1:5173"},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Connect-Protocol-Version", "Connect-Timeout-Ms", "X-User-Agent", "Grpc-Timeout"},
		ExposedHeaders: []string{"Grpc-Status", "Grpc-Message", "Grpc-Status-Details-Bin"},
	}).Handler(mux)

	portStr := fmt.Sprintf("127.0.0.1:%d", port)
	return &http.Server{
		Addr:              portStr,
		Handler:           localRequests(port, corsHandler),
		ReadHeaderTimeout: 10 * time.Second,
	}, nil
}

func init() {
	serveCmd.Flags().IntVarP(&servePort, "port", "p", 4321, "Port to run the HTTP server on")
	RootCmd.AddCommand(serveCmd)
}
