// Package monitor polls MCP servers for health and capabilities.
package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HistorySize is how many past checks are kept per server.
const HistorySize = 60

// Target is one MCP server to watch.
type Target struct {
	Name     string
	Endpoint string // e.g. http://mcp-crm.mcp.svc/mcp
}

// ParseTargets parses "crm=http://host/mcp,billing=http://host/mcp".
func ParseTargets(s string) ([]Target, error) {
	var ts []Target
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, url, ok := strings.Cut(part, "=")
		if !ok || name == "" || !strings.HasPrefix(url, "http") {
			return nil, fmt.Errorf("invalid server %q, want name=http://host/mcp", part)
		}
		ts = append(ts, Target{Name: strings.TrimSpace(name), Endpoint: strings.TrimSpace(url)})
	}
	if len(ts) == 0 {
		return nil, fmt.Errorf("no servers configured")
	}
	return ts, nil
}

type Param struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description,omitempty"`
}

type Tool struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Params      []Param `json:"params"`
}

type Check struct {
	At        time.Time `json:"at"`
	OK        bool      `json:"ok"`
	LatencyMS int64     `json:"latency_ms"`
}

type ServerStatus struct {
	Name      string    `json:"name"`
	Endpoint  string    `json:"endpoint"`
	Healthy   bool      `json:"healthy"`
	LatencyMS int64     `json:"latency_ms"`
	Error     string    `json:"error,omitempty"`
	Version   string    `json:"version,omitempty"`
	Tools     []Tool    `json:"tools"`
	History   []Check   `json:"history"`
	CheckedAt time.Time `json:"checked_at"`
}

type Snapshot struct {
	CheckedAt time.Time      `json:"checked_at"`
	Servers   []ServerStatus `json:"servers"`
}

type Monitor struct {
	targets []Target
	http    *http.Client
	timeout time.Duration
	logger  *slog.Logger

	mu     sync.RWMutex
	status map[string]ServerStatus
	last   time.Time
}

func New(targets []Target, token string, timeout time.Duration, logger *slog.Logger) *Monitor {
	client := &http.Client{Timeout: timeout}
	if token != "" {
		client.Transport = bearer{token: token, next: http.DefaultTransport}
	}
	m := &Monitor{targets: targets, http: client, timeout: timeout, logger: logger, status: map[string]ServerStatus{}}
	for _, t := range targets {
		m.status[t.Name] = ServerStatus{Name: t.Name, Endpoint: t.Endpoint, Tools: []Tool{}, History: []Check{}}
	}
	return m
}

// Run polls every interval until ctx is cancelled.
func (m *Monitor) Run(ctx context.Context, interval time.Duration) {
	m.PollOnce(ctx)
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			m.PollOnce(ctx)
		}
	}
}

// PollOnce checks every target in parallel.
func (m *Monitor) PollOnce(ctx context.Context) {
	var wg sync.WaitGroup
	for _, t := range m.targets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.record(t, m.check(ctx, t))
		}()
	}
	wg.Wait()
	m.mu.Lock()
	m.last = time.Now()
	m.mu.Unlock()
}

type result struct {
	at      time.Time
	ok      bool
	latency time.Duration
	err     error
	version string
	tools   []Tool
}

func (m *Monitor) check(ctx context.Context, t Target) result {
	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()
	r := result{at: time.Now()}

	// Health: GET /healthz next to the /mcp endpoint.
	start := time.Now()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(t.Endpoint, "/mcp")+"/healthz", nil)
	resp, err := m.http.Do(req)
	r.latency = time.Since(start)
	if err != nil {
		r.err = fmt.Errorf("health check failed: %w", err)
		return r
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		r.err = fmt.Errorf("health check returned %d", resp.StatusCode)
		return r
	}

	// Capabilities: connect as an MCP client and list tools.
	client := mcp.NewClient(&mcp.Implementation{Name: "mcp-monitor", Version: "0.1.0"}, nil)
	cs, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: t.Endpoint, HTTPClient: m.http, MaxRetries: -1}, nil)
	if err != nil {
		r.err = fmt.Errorf("MCP connect failed: %w", err)
		return r
	}
	defer cs.Close()
	if info := cs.InitializeResult(); info != nil && info.ServerInfo != nil {
		r.version = info.ServerInfo.Version
	}
	list, err := cs.ListTools(ctx, nil)
	if err != nil {
		r.err = fmt.Errorf("listing tools failed: %w", err)
		return r
	}
	for _, tool := range list.Tools {
		r.tools = append(r.tools, Tool{Name: tool.Name, Description: tool.Description, Params: params(tool.InputSchema)})
	}
	sort.Slice(r.tools, func(i, j int) bool { return r.tools[i].Name < r.tools[j].Name })
	r.ok = true
	return r
}

func (m *Monitor) record(t Target, r result) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.status[t.Name]
	s.Healthy, s.LatencyMS, s.CheckedAt = r.ok, r.latency.Milliseconds(), r.at
	s.Error = ""
	if r.err != nil {
		s.Error = r.err.Error()
		m.logger.Warn("server unhealthy", "server", t.Name, "err", r.err)
	}
	if r.ok {
		// Keep the last known tools while a server is down, so the catalog
		// still shows what the agent will get back once it recovers.
		s.Tools, s.Version = r.tools, r.version
	}
	s.History = append(s.History, Check{At: r.at, OK: r.ok, LatencyMS: s.LatencyMS})
	if len(s.History) > HistorySize {
		s.History = s.History[len(s.History)-HistorySize:]
	}
	m.status[t.Name] = s
}

// Snapshot returns the latest status of every server, in configured order.
func (m *Monitor) Snapshot() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	snap := Snapshot{CheckedAt: m.last, Servers: make([]ServerStatus, 0, len(m.targets))}
	for _, t := range m.targets {
		s := m.status[t.Name]
		s.Tools = append([]Tool{}, s.Tools...)
		s.History = append([]Check{}, s.History...)
		snap.Servers = append(snap.Servers, s)
	}
	return snap
}

// params flattens a tool's JSON input schema into a parameter list.
func params(schema any) []Param {
	obj, _ := schema.(map[string]any)
	props, _ := obj["properties"].(map[string]any)
	required := map[string]bool{}
	if req, ok := obj["required"].([]any); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				required[s] = true
			}
		}
	}
	out := []Param{}
	for name, raw := range props {
		p, _ := raw.(map[string]any)
		out = append(out, Param{Name: name, Type: typeOf(p), Required: required[name], Description: str(p["description"])})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Required != out[j].Required {
			return out[i].Required
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func typeOf(p map[string]any) string {
	switch t := p["type"].(type) {
	case string:
		if t == "array" {
			if items, ok := p["items"].(map[string]any); ok {
				return typeOf(items) + "[]"
			}
		}
		return t
	case []any: // e.g. ["null", "integer"] for optional pointers
		for _, v := range t {
			if s, ok := v.(string); ok && s != "null" {
				return s
			}
		}
	}
	return "any"
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

type bearer struct {
	token string
	next  http.RoundTripper
}

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return b.next.RoundTrip(r)
}
