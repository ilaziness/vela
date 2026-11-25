package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/firebase/genkit/go/genkit"
	genkitmcp "github.com/firebase/genkit/go/plugins/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/ai"
	"github.com/ilaziness/vela/internal/bookmark"
	"github.com/ilaziness/vela/internal/buildinfo"
	"github.com/ilaziness/vela/internal/ssh"
	"github.com/ilaziness/vela/internal/sshkey"
	"github.com/ilaziness/vela/internal/termio"
)

// Status is the runtime state of the MCP server, surfaced in settings.
type Status struct {
	Running bool   `json:"running"`
	Addr    string `json:"addr"`
	Err     string `json:"err,omitempty"`
}

// Deps are the engine services the MCP tools call into.
type Deps struct {
	Logger      *zap.Logger
	Bookmarks   *bookmark.Service
	SSH         *ssh.Manager
	Hub         *termio.Hub
	GlobalProxy func() ssh.GlobalProxy
	Keys        *sshkey.Service
	// Screen reads a session's terminal screen text; nil disables read_terminal.
	Screen func(linkID string, scrollbackLines int) (string, error)
}

func (d *Deps) logger() *zap.Logger {
	if d.Logger == nil {
		return zap.NewNop()
	}
	return d.Logger
}

// Server exposes Vela tools over MCP streamable HTTP on loopback. The
// Genkit instance is created lazily inside Start so the engine-free tool
// registry never registers the approval-gated builtin AI tools.
type Server struct {
	deps Deps

	mu       sync.Mutex
	cfg      ai.MCPServerConfig
	running  bool
	addr     string
	lastErr  string
	cancel   context.CancelFunc // cancels the genkit instance's background work (e.g. reflection)
	httpSrv  *http.Server
	listener net.Listener
	stream   *mcpgo.StreamableHTTPServer
}

// New builds a Server; nothing listens until Start.
func New(d Deps) *Server {
	return &Server{deps: d}
}

// sameConfig reports whether the running server already matches cfg
// (normalized; AccessKey plaintext is ignored — only its hash matters).
func (s *Server) sameConfig(cfg ai.MCPServerConfig) bool {
	return s.running &&
		s.cfg.Enabled == cfg.Enabled &&
		s.cfg.Port == cfg.Port &&
		s.cfg.AccessKeyHash == cfg.AccessKeyHash
}

// Start applies cfg: stops the server when disabled, restarts it when the
// effective configuration changed, no-ops when already matching.
func (s *Server) Start(ctx context.Context, cfg ai.MCPServerConfig) error {
	cfg.AccessKey = ""
	cfg.Normalize()

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sameConfig(cfg) {
		return nil
	}
	s.stopLocked()
	if !cfg.Enabled {
		s.cfg = cfg
		s.lastErr = ""
		return nil
	}
	return s.startLocked(ctx, cfg)
}

func (s *Server) startLocked(ctx context.Context, cfg ai.MCPServerConfig) error {
	// genkit.Init derives signal.NotifyContext from ctx and may spawn
	// background work (reflection API in dev); keep a cancel to release it.
	gctx, cancel := context.WithCancel(ctx)
	g := genkit.Init(gctx)
	defineTools(g, &s.deps)
	gm := genkitmcp.NewMCPServer(g, genkitmcp.MCPServerOptions{
		Name:    "vela",
		Version: buildinfo.Version,
	})
	mcpSrv := gm.GetServer()
	if mcpSrv == nil {
		cancel()
		return errors.New("mcp server setup failed: no MCP server instance")
	}
	stream := mcpgo.NewStreamableHTTPServer(mcpSrv, mcpgo.WithEndpointPath("/mcp"))

	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(cfg.Port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		cancel()
		s.lastErr = err.Error()
		return fmt.Errorf("mcp server listen %s: %w", addr, err)
	}
	httpSrv := &http.Server{
		Handler:           authMiddleware(stream, cfg.AccessKeyHash),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		if err := httpSrv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.deps.logger().Warn("mcp server serve failed", zap.Error(err))
			s.mu.Lock()
			if s.running {
				s.lastErr = err.Error()
			}
			s.mu.Unlock()
		}
	}()

	s.cancel = cancel
	s.stream = stream
	s.httpSrv = httpSrv
	s.listener = ln
	s.cfg = cfg
	s.running = true
	s.addr = addr
	s.lastErr = ""
	s.deps.logger().Info("mcp server started", zap.String("addr", addr), zap.Bool("auth", cfg.AccessKeyHash != ""))
	return nil
}

// stopLocked shuts down the listener and MCP sessions. Caller holds s.mu.
func (s *Server) stopLocked() {
	if s.httpSrv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = s.httpSrv.Shutdown(ctx)
		cancel()
	}
	if s.stream != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		s.stream.CloseSessions(ctx)
		cancel()
	}
	if s.listener != nil {
		_ = s.listener.Close()
	}
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.httpSrv = nil
	s.stream = nil
	s.listener = nil
	s.running = false
	s.addr = ""
}

// Stop shuts the server down regardless of config.
func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopLocked()
}

// Status returns a snapshot for the settings UI.
func (s *Server) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := Status{Running: s.running, Addr: s.addr, Err: s.lastErr}
	return out
}
