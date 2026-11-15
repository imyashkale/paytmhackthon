package monitor

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/imyashkale/paytmhackthon/internal/platform"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestParseTargets(t *testing.T) {
	ts, err := ParseTargets(" crm=http://a/mcp, billing=http://b/mcp ,")
	if err != nil || len(ts) != 2 || ts[0].Name != "crm" || ts[1].Endpoint != "http://b/mcp" {
		t.Fatalf("got %+v, %v", ts, err)
	}
	for _, bad := range []string{"", "crm", "crm=ftp://x", "=http://x/mcp"} {
		if _, err := ParseTargets(bad); err == nil {
			t.Errorf("ParseTargets(%q) succeeded, want error", bad)
		}
	}
}

func TestPollHealthyAndDown(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	srv := mcp.NewServer(&mcp.Implementation{Name: "crm", Version: "1.2.3"}, nil)
	type in struct {
		Name  string `json:"name" jsonschema:"full name"`
		Score *int   `json:"score,omitempty"`
	}
	mcp.AddTool(srv, &mcp.Tool{Name: "create_lead", Description: "Create a lead."},
		func(context.Context, *mcp.CallToolRequest, in) (*mcp.CallToolResult, any, error) {
			return nil, nil, nil
		})
	up := httptest.NewServer(platform.Handler(platform.Config{Token: "secret"}, srv, logger))
	defer up.Close()

	gone := httptest.NewServer(nil)
	gone.Close() // nothing listening

	m := New([]Target{
		{Name: "crm", Endpoint: up.URL + "/mcp"},
		{Name: "billing", Endpoint: gone.URL + "/mcp"},
	}, "secret", 2*time.Second, logger)
	m.PollOnce(context.Background())
	snap := m.Snapshot()

	crm, billing := snap.Servers[0], snap.Servers[1]
	if !crm.Healthy || crm.Version != "1.2.3" || len(crm.Tools) != 1 {
		t.Fatalf("crm = %+v, want healthy with 1 tool and version 1.2.3", crm)
	}
	got := crm.Tools[0].Params
	if len(got) != 2 || got[0] != (Param{Name: "name", Type: "string", Required: true, Description: "full name"}) || got[1].Name != "score" || got[1].Type != "integer" || got[1].Required {
		t.Fatalf("params = %+v", got)
	}
	if billing.Healthy || billing.Error == "" || len(billing.History) != 1 || billing.History[0].OK {
		t.Fatalf("billing = %+v, want unhealthy with an error", billing)
	}
}

func TestWrongTokenIsUnhealthy(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := mcp.NewServer(&mcp.Implementation{Name: "crm"}, nil)
	ts := httptest.NewServer(platform.Handler(platform.Config{Token: "secret"}, srv, logger))
	defer ts.Close()

	m := New([]Target{{Name: "crm", Endpoint: ts.URL + "/mcp"}}, "wrong", 2*time.Second, logger)
	m.PollOnce(context.Background())
	if s := m.Snapshot().Servers[0]; s.Healthy {
		t.Fatal("healthy with wrong token, want unhealthy (healthz passes but MCP rejects)")
	}
}
