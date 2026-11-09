package termws

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/command"
	"github.com/ilaziness/vela/internal/system"
	"github.com/ilaziness/vela/internal/termio"
)

type Client struct {
	conn       *websocket.Conn
	stdin      io.WriteCloser
	sessionID  string
	done       chan struct{}
	closeOnce  sync.Once
	pongMissed int
	pongMu     sync.Mutex
	logger     *zap.Logger
	capture    *command.LineCapture
}

type Server struct {
	logger     *zap.Logger
	hub        *termio.Hub
	onClose    func(sessionID string)
	httpServer *http.Server
	clients    sync.Map
	addr       string
}

func NewServer(logger *zap.Logger, hub *termio.Hub, onClose func(sessionID string)) *Server {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Server{logger: logger, hub: hub, onClose: onClose}
}

// AcceptHistory reports whether a frontend-submitted shell line may be stored.
func (s *Server) AcceptHistory(sessionID, cmd string) bool {
	v, ok := s.clients.Load(sessionID)
	if !ok {
		return false
	}
	return v.(*Client).capture.AcceptHistory(cmd)
}

func (s *Server) Addr() string { return s.addr }

func (s *Server) pickListen() (net.Listener, error) {
	base := 10697
	var last error
	for i := range 100 {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", base+i))
		if err == nil {
			return ln, nil
		}
		last = err
	}
	if last == nil {
		last = fmt.Errorf("no free port for terminal websocket")
	}
	return nil, last
}

func (s *Server) Start() error {
	ln, err := s.pickListen()
	if err != nil {
		return err
	}
	s.addr = ln.Addr().String()
	s.logger.Debug("Starting WebSocket server", zap.String("addr", s.addr))
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/terminal", s.handle)
	s.httpServer = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		defer system.RecoverFromPanic()
		if err := s.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			s.logger.Error("WebSocket server error", zap.Error(err))
		}
	}()
	return nil
}

func (s *Server) Stop() {
	s.logger.Debug("Stopping WebSocket server")
	s.CloseAllClients()
	if s.httpServer != nil {
		_ = s.httpServer.Close()
	}
}

func (s *Server) CloseClient(sessionID string) {
	v, ok := s.clients.Load(sessionID)
	if !ok {
		return
	}
	v.(*Client).closeDone()
}

func (s *Server) CloseAllClients() {
	s.clients.Range(func(_, value any) bool {
		value.(*Client).closeDone()
		return true
	})
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("id")
	if sessionID == "" {
		http.Error(w, "Session ID is required", http.StatusBadRequest)
		return
	}
	cols, rows := 80, 24
	if v := r.URL.Query().Get("cols"); v != "" {
		_, _ = fmt.Sscanf(v, "%d", &cols)
	}
	if v := r.URL.Query().Get("rows"); v != "" {
		_, _ = fmt.Sscanf(v, "%d", &rows)
	}
	cleanup := func() {
		if s.onClose != nil {
			s.onClose(sessionID)
		}
	}
	sess, ok := s.hub.Get(sessionID)
	if !ok || sess == nil {
		cleanup()
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}
	if err := sess.Start(cols, rows); err != nil {
		s.logger.Error("Failed to start session", zap.Error(err), zap.String("id", sessionID))
		cleanup()
		http.Error(w, "Failed to start session", http.StatusInternalServerError)
		return
	}
	if sess.Stdin() == nil {
		cleanup()
		http.Error(w, "Session stdin unavailable", http.StatusNotFound)
		return
	}
	client := &Client{
		done:      make(chan struct{}),
		stdin:     sess.Stdin(),
		sessionID: sessionID,
		logger:    s.logger,
		capture:   &command.LineCapture{},
	}
	connWS, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
		OnPongReceived: func(_ context.Context, _ []byte) {
			client.pongMu.Lock()
			client.pongMissed = 0
			client.pongMu.Unlock()
		},
	})
	if err != nil {
		s.logger.Error("Failed to accept WebSocket", zap.Error(err))
		cleanup()
		return
	}
	client.conn = connWS
	s.clients.Store(sessionID, client)
	go client.readLoop()
	go client.writeLoop(sess.Output())
	client.ping()
	client.closeDone()
	s.clients.Delete(sessionID)
	if s.onClose != nil {
		s.onClose(sessionID)
	}
	_ = client.conn.Close(websocket.StatusNormalClosure, "")
}

func (c *Client) closeDone() {
	c.closeOnce.Do(func() { close(c.done) })
}

func (c *Client) ping() {
	ctx := context.Background()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.pongMu.Lock()
			c.pongMissed++
			if c.pongMissed > 2 {
				c.pongMu.Unlock()
				return
			}
			c.pongMu.Unlock()
			if err := c.conn.Ping(ctx); err != nil {
				return
			}
		case <-c.done:
			return
		}
	}
}

func (c *Client) readLoop() {
	defer c.closeDone()
	ctx := context.Background()
	for {
		_, data, err := c.conn.Read(ctx)
		if err != nil {
			return
		}
		_ = c.capture.FeedInput(data)
		if c.stdin != nil {
			if _, err := c.stdin.Write(data); err != nil {
				return
			}
		}
	}
}

func (c *Client) writeLoop(outputChan <-chan []byte) {
	defer c.closeDone()
	ctx := context.Background()
	for {
		select {
		case data, ok := <-outputChan:
			if !ok {
				return
			}
			c.capture.FeedOutput(data)
			if err := c.conn.Write(ctx, websocket.MessageBinary, data); err != nil {
				return
			}
		case <-c.done:
			return
		}
	}
}
