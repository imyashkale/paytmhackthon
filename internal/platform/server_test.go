package platform

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type bearer struct{ token string }

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

func newTestServer(t *testing.T, token string) *httptest.Server {
	t.Helper()
	s := mcp.NewServer(&mcp.Implementation{Name: "test"}, nil)
	type in struct {
		Name string `json:"name"`
	}
	type out struct {
		Greeting string `json:"greeting"`
	}
	mcp.AddTool(s, &mcp.Tool{Name: "greet"}, func(_ context.Context, _ *mcp.CallToolRequest, i in) (*mcp.CallToolResult, out, error) {
		return nil, out{Greeting: "hi " + i.Name}, nil
	})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ts := httptest.NewServer(Handler(Config{Token: token}, s, logger))
	t.Cleanup(ts.Close)
	return ts
}

func connect(t *testing.T, url string, client *http.Client) (*mcp.ClientSession, error) {
	t.Helper()
	c := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil)
	return c.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: url + "/mcp", HTTPClient: client, MaxRetries: -1}, nil)
}

func TestHealthz(t *testing.T) {
	ts := newTestServer(t, "secret")
	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz status = %d, want 200 (no auth required)", resp.StatusCode)
	}
}

func TestMCPRequiresToken(t *testing.T) {
	ts := newTestServer(t, "secret")

	resp, err := http.Post(ts.URL+"/mcp", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no token: status = %d, want 401", resp.StatusCode)
	}

	if _, err := connect(t, ts.URL, &http.Client{Transport: bearer{"wrong"}}); err == nil {
		t.Fatal("wrong token: connect succeeded, want error")
	}
}

func TestMCPToolCallOverHTTP(t *testing.T) {
	ts := newTestServer(t, "secret")
	cs, err := connect(t, ts.URL, &http.Client{Transport: bearer{"secret"}})
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "greet", Arguments: map[string]any{"name": "asha"}})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := res.StructuredContent.(map[string]any)
	if res.IsError || got["greeting"] != "hi asha" {
		t.Fatalf("result = %+v, want greeting \"hi asha\"", res.StructuredContent)
	}
}
