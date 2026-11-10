package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"connectrpc.com/connect"

	graphv1 "github.com/rjfonseca/daneel/api/v1"
	"github.com/rjfonseca/daneel/internal/daemon"
	"github.com/rjfonseca/daneel/internal/ingest"
)

// IngestPath ingests a file or directory from the host filesystem into the vault.
func (s *GraphServer) IngestPath(
	ctx context.Context,
	req *connect.Request[graphv1.IngestPathRequest],
) (*connect.Response[graphv1.IngestPathResponse], error) {
	if req == nil || req.Msg == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("ingest request is required"))
	}

	hostPath := strings.TrimSpace(req.Msg.Path)
	if hostPath == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("path is required"))
	}

	vaultDir := s.vaultDir
	if vaultDir == "" && s.cfg != nil {
		vaultDir = s.cfg.VaultPath
	}
	if vaultDir == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("vault path is not configured"))
	}

	opts := ingest.IngestOptions{
		Destination: strings.TrimSpace(req.Msg.Destination),
		Recursive:   req.Msg.Recursive,
		Move:        req.Msg.Move,
	}

	engine := ingest.NewEngine(vaultDir, s.engine, daemon.GetDefaultHub())
	report, err := engine.IngestPath(ctx, hostPath, opts)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, connect.NewError(connect.CodeCanceled, err)
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, connect.NewError(connect.CodeDeadlineExceeded, err)
		}
		if errors.Is(err, os.ErrNotExist) || strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "not found") {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		if strings.Contains(err.Error(), "escapes") || strings.Contains(err.Error(), "invalid") {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("ingest path failed: %w", err))
	}

	items := make([]*graphv1.IngestedItem, 0, len(report.Items))
	for _, item := range report.Items {
		items = append(items, &graphv1.IngestedItem{
			SourcePath: item.SourcePath,
			RelPath:    item.RelPath,
			Title:      item.Title,
			SizeBytes:  item.SizeBytes,
			Warning:    item.Warning,
		})
	}

	var warning string
	if len(report.Errors) > 0 {
		warning = strings.Join(report.Errors, "; ")
	}

	return connect.NewResponse(&graphv1.IngestPathResponse{
		Items:          items,
		TotalProcessed: int32(report.TotalProcessed), //nolint:gosec // bounded count of processed items
		Warning:        warning,
	}), nil
}
