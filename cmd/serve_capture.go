package cmd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"connectrpc.com/connect"
	"gopkg.in/yaml.v3"

	graphv1 "github.com/rjfonseca/daneel/api/v1"
)

const maxCaptureContent = 1 << 20

// CaptureNote saves one explicitly submitted note in the vault inbox.
func (s *GraphServer) CaptureNote(ctx context.Context, req *connect.Request[graphv1.CaptureNoteRequest]) (*connect.Response[graphv1.CaptureNoteResponse], error) {
	if req == nil || req.Msg == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("capture is required"))
	}
	title := strings.TrimSpace(req.Msg.Title)
	content := req.Msg.ContentMarkdown
	source := req.Msg.SourceUrl
	if title == "" || utf8.RuneCountInString(title) > 200 || hasControl(req.Msg.Title) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("title must be 1–200 characters without control characters"))
	}
	if strings.TrimSpace(content) == "" || len(content) > maxCaptureContent {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("content must be nonblank and at most 1 MiB"))
	}
	if len(source) > 2048 || hasControl(source) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("source URL is invalid"))
	}
	if source != "" {
		parsed, err := url.Parse(source)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.Opaque != "" || source != strings.TrimSpace(source) {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("source URL must be HTTP or HTTPS with a host"))
		}
	}

	meta := struct {
		Title     string `yaml:"title"`
		SourceURL string `yaml:"source_url,omitempty"`
	}{Title: title, SourceURL: source}
	frontmatter, err := yaml.Marshal(meta)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("encode note metadata: %w", err))
	}

	root, err := os.OpenRoot(s.vaultDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("open vault: %w", err))
	}
	defer func() { _ = root.Close() }()
	info, err := root.Lstat("inbox")
	if errors.Is(err, os.ErrNotExist) {
		if err := root.Mkdir("inbox", 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("create inbox: %w", err))
		}
		info, err = root.Lstat("inbox")
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("inbox must be a directory without a symlink"))
	}
	inbox, err := root.OpenRoot("inbox")
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("open inbox: %w", err))
	}
	defer func() { _ = inbox.Close() }()
	opened, err := inbox.Stat(".")
	current, currentErr := root.Lstat("inbox")
	if err != nil || currentErr != nil || current.Mode()&os.ModeSymlink != 0 || !os.SameFile(info, opened) || !os.SameFile(current, opened) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("inbox changed while opening"))
	}

	var suffix [16]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("name capture: %w", err))
	}
	name := time.Now().UTC().Format("20060102T150405.000000000Z") + "-" + hex.EncodeToString(suffix[:]) + ".md"
	file, err := inbox.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("create note: %w", err))
	}
	_, writeErr := io.WriteString(file, "---\n"+string(frontmatter)+"---\n# "+title+"\n\n"+content)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = inbox.Remove(name)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save note: %w", errors.Join(writeErr, closeErr)))
	}

	relPath := path.Join("inbox", name)
	response := &graphv1.CaptureNoteResponse{RelPath: relPath}
	if s.engine == nil {
		response.Warning = "Note saved, but search indexing is unavailable."
	} else if err := s.engine.IndexNote(ctx, relPath); err != nil {
		response.Warning = "Note saved, but search indexing failed: " + err.Error()
	}
	return connect.NewResponse(response), nil
}

func hasControl(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}
