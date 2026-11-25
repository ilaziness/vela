package agent

import (
	"context"

	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/ai"
	"github.com/ilaziness/vela/internal/mcpserver"
)

// ensureMCPServer lazily builds the MCP server runtime.
func (s *Service) ensureMCPServer() *mcpserver.Server {
	s.mcpSrvMu.Lock()
	defer s.mcpSrvMu.Unlock()
	if s.mcpSrv == nil {
		s.mcpSrv = mcpserver.New(mcpserver.Deps{
			Logger:      s.logger,
			Bookmarks:   s.bookmarks,
			SSH:         s.sshMgr,
			Hub:         s.hub,
			GlobalProxy: s.globalProxyFn,
			Keys:        s.keys,
			Screen:      s.screenFn,
		})
	}
	return s.mcpSrv
}

// SyncMCPServer reconciles the built-in MCP server with the saved AI config.
// It never blocks callers for long: listen failures are recorded in Status.
func (s *Service) SyncMCPServer() {
	cfg := ai.MCPServerConfig{}
	if s.store != nil {
		cfg = s.store.Config.AI.MCPServer
	}
	cfg.AccessKey = ""
	cfg.Normalize()

	srv := s.ensureMCPServer()
	if !cfg.Enabled {
		srv.Stop()
		return
	}
	if err := srv.Start(context.Background(), cfg); err != nil {
		s.logger.Warn("mcp server start failed", zap.Error(err))
	}
}

// MCPServerStatus reports the MCP server runtime state for the settings UI.
func (s *Service) MCPServerStatus() mcpserver.Status {
	s.mcpSrvMu.Lock()
	srv := s.mcpSrv
	s.mcpSrvMu.Unlock()
	if srv == nil {
		return mcpserver.Status{}
	}
	return srv.Status()
}

// stopMCPServer shuts the MCP server down (app shutdown path).
func (s *Service) stopMCPServer() {
	s.mcpSrvMu.Lock()
	srv := s.mcpSrv
	s.mcpSrvMu.Unlock()
	if srv != nil {
		srv.Stop()
	}
}
