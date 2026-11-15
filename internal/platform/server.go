// Package platform hosts an MCP server over Streamable HTTP with health
// checks, optional bearer-token auth, request logging and graceful shutdown.
package platform

import (
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Config is read from the environment.
//
//	PORT       listen port (default 8080)
//	MCP_TOKEN  if set, clients must send "Authorization: Bearer <MCP_TOKEN>"
type Config struct {
	Port  string
	Token string
}

func LoadConfig() Config {
	cfg := Config{Port: os.Getenv("PORT"), Token: os.Getenv("MCP_TOKEN")}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	return cfg
}

// Run builds the MCP server, registers its tools and serves it until
// SIGINT/SIGTERM.
func Run(name, version string, register func(*mcp.Server)) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("server", name)
	slog.SetDefault(logger)

	cfg := LoadConfig()
	server := mcp.NewServer(&mcp.Implementation{Name: name, Version: version}, &mcp.ServerOptions{Logger: logger})
	register(server)

	if err := Serve(context.Background(), cfg, server, logger); err != nil {
		logger.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

// Handler returns the HTTP routes: POST /mcp (MCP) and GET /healthz.
func Handler(cfg Config, server *mcp.Server, logger *slog.Logger) http.Handler {
	// Stateless: every request is self-contained, so replicas can scale
	// horizontally behind a load balancer without sticky sessions.
	var mcpHandler http.Handler = mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, Logger: logger},
	)
	if cfg.Token != "" {
		mcpHandler = auth.RequireBearerToken(staticToken(cfg.Token), &auth.RequireBearerTokenOptions{
			AllowMissingExpiration: true,
		})(mcpHandler)
	}

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpHandler)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok"))
	})
	return logRequests(logger, mux)
}

func Serve(ctx context.Context, cfg Config, server *mcp.Server, logger *slog.Logger) error {
	if cfg.Token == "" {
		logger.Warn("MCP_TOKEN not set: /mcp is unauthenticated")
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           Handler(cfg, server, logger),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", srv.Addr)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func staticToken(want string) auth.TokenVerifier {
	return func(_ context.Context, got string, _ *http.Request) (*auth.TokenInfo, error) {
		if subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
			return nil, auth.ErrInvalidToken
		}
		return &auth.TokenInfo{}, nil
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func logRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if r.URL.Path == "/healthz" {
			return
		}
		logger.Info("request", "method", r.Method, "path", r.URL.Path,
			"status", rec.status, "duration_ms", time.Since(start).Milliseconds())
	})
}
