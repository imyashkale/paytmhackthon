// The monitor checks every MCP server's health and tools and serves the
// result as JSON at GET /api/status for the frontend.
//
//	PORT           listen port (default 8080)
//	MCP_SERVERS    comma-separated name=url list, e.g. crm=http://mcp-crm.mcp.svc/mcp
//	MCP_TOKEN      bearer token sent to the MCP servers
//	POLL_INTERVAL  how often to check servers (default 5s)
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/imyashkale/paytmhackthon/monitor/internal/monitor"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("server", "monitor")
	if err := run(logger); err != nil {
		logger.Error("monitor stopped", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	targets, err := monitor.ParseTargets(os.Getenv("MCP_SERVERS"))
	if err != nil {
		return err
	}
	interval := 5 * time.Second
	if v := os.Getenv("POLL_INTERVAL"); v != "" {
		if interval, err = time.ParseDuration(v); err != nil || interval < time.Second {
			return errors.New("POLL_INTERVAL must be a duration of at least 1s, e.g. 5s")
		}
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mon := monitor.New(targets, os.Getenv("MCP_TOKEN"), 3*time.Second, logger)
	go mon.Run(ctx, interval)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(struct {
			monitor.Snapshot
			IntervalMS int64 `json:"interval_ms"`
		}{mon.Snapshot(), interval.Milliseconds()})
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })

	srv := &http.Server{Addr: ":" + port, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	errc := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", srv.Addr, "servers", len(targets))
		errc <- srv.ListenAndServe()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
