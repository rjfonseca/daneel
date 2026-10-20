//nolint:revive // package comment not required
package clone

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	ErrInvalidGitURL = errors.New("URL ou identificador de repositório Git inválido") //nolint:revive
)

// GitTarget contains the parsed components of a remote repository.
type GitTarget struct {
	Host     string `json:"host"`
	Org      string `json:"org"`
	Repo     string `json:"repo"`
	RelPath  string `json:"rel_path"`
	CloneURL string `json:"clone_url"`
}

// ParseGitTarget parses SSH URLs, HTTPS URLs, and short slugs into a normalized GitTarget.
func ParseGitTarget(raw string) (*GitTarget, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, ErrInvalidGitURL
	}

	// 1. SSH format: git@github.com:org/repo.git or git@gitlab.com:group/subgroup/repo.git
	sshRegex := regexp.MustCompile(`^git@([^:]+):(.+?)(?:\.git)?$`)
	if matches := sshRegex.FindStringSubmatch(raw); len(matches) == 3 {
		host := matches[1]
		fullPath := strings.Trim(matches[2], "/")
		parts := strings.Split(fullPath, "/")
		if len(parts) < 2 {
			return nil, fmt.Errorf("%w: incomplete SSH path '%s'", ErrInvalidGitURL, raw)
		}
		repo := parts[len(parts)-1]
		org := strings.Join(parts[:len(parts)-1], "/")
		relPath := filepath.Join(host, org, repo)

		return &GitTarget{
			Host:     host,
			Org:      org,
			Repo:     repo,
			RelPath:  filepath.ToSlash(relPath),
			CloneURL: raw,
		}, nil
	}

	// 2. HTTPS / HTTP format: https://github.com/org/repo.git
	if strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "http://") {
		u, err := url.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidGitURL, err)
		}
		host := u.Host
		cleanPath := strings.Trim(strings.TrimSuffix(u.Path, ".git"), "/")
		parts := strings.Split(cleanPath, "/")
		if len(parts) < 2 {
			return nil, fmt.Errorf("%w: incomplete HTTPS path '%s'", ErrInvalidGitURL, raw)
		}
		repo := parts[len(parts)-1]
		org := strings.Join(parts[:len(parts)-1], "/")
		relPath := filepath.Join(host, org, repo)

		return &GitTarget{
			Host:     host,
			Org:      org,
			Repo:     repo,
			RelPath:  filepath.ToSlash(relPath),
			CloneURL: raw,
		}, nil
	}

	// 3. Slugs: github.com/org/repo or gitlab.com/org/subgroup/repo or org/repo
	cleanSlug := strings.Trim(strings.TrimSuffix(raw, ".git"), "/")
	parts := strings.Split(cleanSlug, "/")

	var host, org, repo string
	if len(parts) >= 3 && strings.Contains(parts[0], ".") {
		// e.g. github.com/rjfonseca/daneel
		host = parts[0]
		org = strings.Join(parts[1:len(parts)-1], "/")
		repo = parts[len(parts)-1]
	} else if len(parts) == 2 {
		// e.g. rjfonseca/daneel -> default to github.com
		host = "github.com"
		org = parts[0]
		repo = parts[1]
	} else if len(parts) > 2 {
		// e.g. org/subgroup/repo -> default to github.com
		host = "github.com"
		org = strings.Join(parts[:len(parts)-1], "/")
		repo = parts[len(parts)-1]
	} else {
		return nil, fmt.Errorf("%w: unrecognized format '%s' (use org/repo or full URL)", ErrInvalidGitURL, raw)
	}

	relPath := filepath.Join(host, org, repo)
	cloneURL := fmt.Sprintf("git@%s:%s/%s.git", host, org, repo)

	return &GitTarget{
		Host:     host,
		Org:      org,
		Repo:     repo,
		RelPath:  filepath.ToSlash(relPath),
		CloneURL: cloneURL,
	}, nil
}
