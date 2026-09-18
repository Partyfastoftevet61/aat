package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gburgyan/aat/config"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// DefaultHost is the interface the web server binds when ServerOptions.Host is
// empty: loopback only, so run archives and the rename and import routes are
// not reachable from other machines unless a caller asks for it.
const DefaultHost = "127.0.0.1"

// ServerOptions configures the web server.
type ServerOptions struct {
	Host          string // interface to bind; empty means DefaultHost (use 0.0.0.0 for all interfaces)
	Port          int
	ArchiveDir    string
	TracesDir     string
	VisualizerDir string
	DevMode       bool
	ViteURL       string // Vite dev server URL for dev proxy (default http://localhost:5173)
}

// Server is the AAT web API server.
type Server struct {
	opts         ServerOptions
	service      ArchiveService
	traceService *TraceService
	visualizers  []config.VisualizerDef
	router       chi.Router
	httpServer   *http.Server
	mu           sync.Mutex
	addr         string
	// silent holds the connections that have been accepted and have sent
	// nothing yet; see trackConn.
	silent map[net.Conn]struct{}
}

// NewServer creates a Server with the given options.
// Port defaults to 9119 if not set.
func NewServer(opts ServerOptions) *Server {
	if opts.Port == 0 {
		opts.Port = 9119
	}

	vizDefs, err := config.LoadVisualizers(opts.VisualizerDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "aat web: warning: loading visualizers: %s\n", err)
	}

	s := &Server{
		opts:         opts,
		service:      NewArchiveService(opts.ArchiveDir),
		traceService: NewTraceService(opts.TracesDir),
		visualizers:  vizDefs,
	}
	s.router = s.buildRouter()
	return s
}

// NewServerWithService creates a Server with a pre-built ArchiveService.
// This is used for static/in-memory archive viewing where no disk access is needed.
func NewServerWithService(opts ServerOptions, svc ArchiveService) *Server {
	if opts.Port == 0 {
		opts.Port = 9119
	}

	s := &Server{
		opts:         opts,
		service:      svc,
		traceService: NewTraceService(opts.TracesDir),
	}
	s.router = s.buildRouter()
	return s
}

func (s *Server) buildRouter() chi.Router {
	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	if s.opts.DevMode {
		r.Use(middleware.Logger)
	}

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api", func(r chi.Router) {
		r.Get("/runs", s.handleListRuns)
		r.Get("/runs/latest", s.handleLatestRun)
		r.Get("/runs/{id}", s.handleGetRun)
		r.Put("/runs/{id}/name", s.handleRenameRun)
		r.Delete("/runs/{id}/name", s.handleUnnameRun)
		r.Get("/runs/{id}/attempts/{attempt}", s.handleGetAttempt)
		r.Get("/runs/{id}/attempts/{attempt}/steps/{stepId}", s.handleGetAttemptStep)
		r.Get("/runs/{id}/attempts/{attempt}/steps/{stepId}/iterations/{index}", s.handleGetAttemptStepIteration)
		r.Get("/runs/{id}/steps/{stepId}", s.handleGetStep)
		r.Get("/runs/{id}/steps/{stepId}/iterations/{index}", s.handleGetStepIteration)
		r.Get("/runs/{id}/export", s.handleExportRun)

		r.Get("/batches", s.handleListBatches)
		r.Get("/batches/{id}", s.handleGetBatch)
		r.Put("/batches/{id}/name", s.handleRenameBatch)
		r.Delete("/batches/{id}/name", s.handleUnnameBatch)
		r.Get("/batches/{id}/export", s.handleExportBatch)

		r.Post("/import", s.handleImport)

		r.Get("/traces", s.handleListTraces)
		r.Get("/traces/{id}", s.handleGetTrace)

		r.Get("/visualizers/{id}", s.handleGetVisualizer)
	})

	// Resolve /runs/latest to the actual run ID so the SPA has a real ID in the URL.
	r.Get("/runs/latest", s.handleLatestRunRedirect)

	// Catch-all: serve frontend (dev proxy or embedded SPA).
	if s.opts.DevMode {
		r.NotFound(s.devProxy())
	} else {
		r.NotFound(spaFileServer().ServeHTTP)
	}

	return r
}

// Handler returns the HTTP handler for use with httptest.
func (s *Server) Handler() http.Handler {
	return s.router
}

// Addr returns the actual listening address. Only valid after ListenAndServe has started.
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addr
}

// BrowseURL returns the URL a browser on this machine uses to reach a server
// bound to host and port. Loopback, unspecified (all interfaces), and empty
// hosts map to localhost.
func BrowseURL(host string, port int) string {
	if ip := net.ParseIP(host); host == "" || host == "localhost" || (ip != nil && (ip.IsLoopback() || ip.IsUnspecified())) {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, strconv.Itoa(port))
}

// ListenAndServe starts serving HTTP requests. It blocks until the server is
// shut down, at which point it returns http.ErrServerClosed.
func (s *Server) ListenAndServe() error {
	host := s.opts.Host
	if host == "" {
		host = DefaultHost
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(s.opts.Port)))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	srv := &http.Server{
		Handler:           s.router,
		ReadHeaderTimeout: 10 * time.Second,
		ConnState:         s.trackConn,
	}

	s.mu.Lock()
	s.addr = ln.Addr().String()
	s.httpServer = srv
	s.mu.Unlock()

	port := ln.Addr().(*net.TCPAddr).Port
	if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
		fmt.Fprintf(os.Stderr, "aat web: listening on %s (all interfaces)\n", BrowseURL(host, port))
	} else {
		fmt.Fprintf(os.Stderr, "aat web: listening on %s\n", BrowseURL(host, port))
	}

	return srv.Serve(ln)
}

// trackConn keeps the set of connections that have sent nothing yet. A browser
// opens spare connections and leaves them silent, and net/http's Shutdown will
// not close one until it is five seconds old, in case a request is on its way.
// Shutdown closes them itself instead of waiting.
func (s *Server) trackConn(conn net.Conn, state http.ConnState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if state == http.StateNew {
		if s.silent == nil {
			s.silent = make(map[net.Conn]struct{})
		}
		s.silent[conn] = struct{}{}
		return
	}
	delete(s.silent, conn)
}

// silentConns counts the connections that have sent nothing yet.
func (s *Server) silentConns() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.silent)
}

// Shutdown gracefully shuts down the server: requests in flight finish, within
// ctx. A connection that never sent a request has nothing to finish and is
// closed at once; left to net/http it would hold the shutdown for up to five
// seconds, which is all the time `aat web` allows.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	srv := s.httpServer
	silent := make([]net.Conn, 0, len(s.silent))
	for conn := range s.silent {
		silent = append(silent, conn)
	}
	s.mu.Unlock()
	if srv == nil {
		return nil
	}
	for _, conn := range silent {
		_ = conn.Close()
	}
	return srv.Shutdown(ctx)
}
