//nolint:revive // package comment not required
package search

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rjfonseca/daneel/internal/ops/workspace"
)

// SearchProjects queries cached repositories in workspace_repos in < 3ms.
func (e *Engine) SearchProjects(ctx context.Context, query string, dirtyOnly, cleanOnly bool, stack string, limit int) ([]workspace.RepoInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	// Auto-hydrate if workspace_repos is empty
	e.ensureProjectsHydrated(ctx)

	var queryBuilder strings.Builder
	var args []interface{}

	queryBuilder.WriteString(`
		SELECT json_data
		FROM workspace_repos
		WHERE 1=1
	`)

	if dirtyOnly {
		queryBuilder.WriteString(" AND is_clean = 0")
	} else if cleanOnly {
		queryBuilder.WriteString(" AND is_clean = 1")
	}

	cleanStack := strings.ToLower(strings.TrimSpace(stack))
	if cleanStack != "" {
		queryBuilder.WriteString(" AND LOWER(stack) LIKE ?")
		args = append(args, "%"+cleanStack+"%")
	}

	cleanQuery := strings.ToLower(strings.TrimSpace(query))
	if cleanQuery != "" {
		queryBuilder.WriteString(" AND (LOWER(name) LIKE ? OR LOWER(rel_path) LIKE ? OR LOWER(description) LIKE ?)")
		likeArg := "%" + cleanQuery + "%"
		args = append(args, likeArg, likeArg, likeArg)

		queryBuilder.WriteString(`
			ORDER BY 
				CASE WHEN LOWER(name) = ? THEN 0 
				     WHEN LOWER(name) LIKE ? THEN 1 
				     ELSE 2 END ASC,
				mtime DESC
		`)
		args = append(args, cleanQuery, cleanQuery+"%")
	} else if dirtyOnly {
		queryBuilder.WriteString(" ORDER BY mtime DESC")
	} else {
		queryBuilder.WriteString(" ORDER BY rel_path ASC")
	}

	if limit > 0 {
		queryBuilder.WriteString(" LIMIT ?")
		args = append(args, limit)
	}

	rows, err := e.db.QueryContext(ctx, queryBuilder.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query workspace repos: %w", err)
	}
	defer rows.Close() //nolint:errcheck,gosec

	var repos []workspace.RepoInfo
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err == nil && raw != "" {
			var r workspace.RepoInfo
			if err := json.Unmarshal([]byte(raw), &r); err == nil {
				repos = append(repos, r)
			}
		}
	}

	return repos, nil
}

// SaveWorkspaceRepos persists scanned repositories into workspace_repos table.
func (e *Engine) SaveWorkspaceRepos(ctx context.Context, repos []workspace.RepoInfo) error {
	if ctx == nil {
		ctx = context.Background()
	}

	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO workspace_repos(rel_path, name, host, abs_path, branch, is_clean, stack, description, mtime, json_data)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(rel_path) DO UPDATE SET
			name=excluded.name,
			host=excluded.host,
			abs_path=excluded.abs_path,
			branch=excluded.branch,
			is_clean=excluded.is_clean,
			stack=excluded.stack,
			description=excluded.description,
			mtime=excluded.mtime,
			json_data=excluded.json_data;
	`)
	if err != nil {
		return err
	}
	defer stmt.Close() //nolint:errcheck

	nowUnix := float64(time.Now().Unix())

	for _, r := range repos {
		isClean := 0
		if !r.Dirty {
			isClean = 1
		}

		host := ""
		parts := strings.Split(r.RelPath, "/")
		if len(parts) > 0 {
			host = parts[0]
		}

		data, err := json.Marshal(r)
		if err != nil {
			continue
		}

		_, _ = stmt.ExecContext(ctx, r.RelPath, r.Name, host, r.Path, r.Branch, isClean, r.Stack, r.Description, nowUnix, string(data))
	}

	return tx.Commit()
}

func (e *Engine) ensureProjectsHydrated(ctx context.Context) {
	var count int
	err := e.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM workspace_repos").Scan(&count)
	if err != nil || count > 0 {
		return
	}

	home, _ := os.UserHomeDir()
	workRoot := filepath.Join(home, "Work")
	if _, err := os.Stat(workRoot); os.IsNotExist(err) {
		return
	}

	scanner := workspace.NewScanner(workRoot, e.vaultRoot)
	rep, err := scanner.Scan(false)
	if err != nil || rep == nil {
		return
	}

	var allRepos []workspace.RepoInfo
	for _, catRepos := range rep.Categories {
		allRepos = append(allRepos, catRepos...)
	}

	_ = e.SaveWorkspaceRepos(ctx, allRepos)
}
