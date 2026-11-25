package cmd

import (
	"context"
	"strings"

	"connectrpc.com/connect"

	graphv1 "github.com/rjfonseca/daneel/api/v1"
	"github.com/rjfonseca/daneel/internal/ops/overview"
)

func (s *GraphServer) resolvedVaultDir() string {
	if s.vaultDir != "" {
		return s.vaultDir
	}
	if s.cfg != nil {
		return s.cfg.VaultPath
	}
	return ""
}

func (s *GraphServer) GetDashboardSummary(
	ctx context.Context,
	req *connect.Request[graphv1.GetDashboardSummaryRequest],
) (*connect.Response[graphv1.GetDashboardSummaryResponse], error) {
	resp := overview.BuildDashboardSummary(ctx, s.resolvedVaultDir(), s.cfg, s.engine, int(req.Msg.Days))
	return connect.NewResponse(resp), nil
}

func (s *GraphServer) GetWorkspaceOverview(
	ctx context.Context,
	req *connect.Request[graphv1.GetWorkspaceOverviewRequest],
) (*connect.Response[graphv1.GetWorkspaceOverviewResponse], error) {
	resp, err := overview.BuildWorkspaceOverview(ctx, s.resolvedVaultDir(), s.cfg, s.engine, req.Msg.ForceRescan)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

func (s *GraphServer) GetPeopleDirectory(
	ctx context.Context,
	_ *connect.Request[graphv1.GetPeopleDirectoryRequest],
) (*connect.Response[graphv1.GetPeopleDirectoryResponse], error) {
	resp, err := overview.BuildPeopleDirectory(ctx, s.resolvedVaultDir(), s.engine)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

func (s *GraphServer) GetPersonBriefing(
	_ context.Context,
	req *connect.Request[graphv1.GetPersonBriefingRequest],
) (*connect.Response[graphv1.GetPersonBriefingResponse], error) {
	slug := strings.TrimSpace(req.Msg.Slug)
	if slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, nil)
	}
	resp, err := overview.BuildPersonBriefing(s.resolvedVaultDir(), slug)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewResponse(resp), nil
}

func (s *GraphServer) GetAgentActivity(
	ctx context.Context,
	req *connect.Request[graphv1.GetAgentActivityRequest],
) (*connect.Response[graphv1.GetAgentActivityResponse], error) {
	resp, err := overview.BuildAgentActivity(
		ctx,
		s.resolvedVaultDir(),
		s.cfg,
		s.getRoutineStore(),
		"",
		int(req.Msg.Hours),
		req.Msg.SessionId,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}
