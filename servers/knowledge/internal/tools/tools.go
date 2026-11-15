// Package tools implements the knowledge MCP tools: search and read internal
// docs such as policies and pricing.
package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Doc struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Category string   `json:"category"`
	Tags     []string `json:"tags,omitempty"`
	Body     string   `json:"body"`
}

type SearchDocsIn struct {
	Query    string `json:"query" jsonschema:"keywords to search for"`
	Category string `json:"category,omitempty" jsonschema:"limit to a category: policy, pricing or faq"`
	Limit    int    `json:"limit,omitempty" jsonschema:"max results (default 5)"`
}

type SearchHit struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Category string `json:"category"`
	Snippet  string `json:"snippet"`
	Score    int    `json:"score"`
}

type SearchDocsOut struct {
	Results []SearchHit `json:"results"`
}

type GetDocIn struct {
	ID string `json:"id" jsonschema:"doc ID from search_docs"`
}

// SeedDocs are sample docs until a real knowledge base is connected.
var SeedDocs = []Doc{
	{ID: "refund-policy", Title: "Refund Policy", Category: "policy", Tags: []string{"refund", "billing"},
		Body: "Customers can get a full refund within 30 days of purchase. Duplicate charges are always refunded in full. " +
			"Refunds above ₹5,000 need approval from a support lead. Refunds go back to the original payment method within 5–7 business days."},
	{ID: "escalation-policy", Title: "Escalation Policy", Category: "policy", Tags: []string{"escalation", "support"},
		Body: "Escalate to a human when: the customer asks for one, the issue involves fraud or legal threats, " +
			"a refund needs approval, or the issue is still unresolved after two attempts."},
	{ID: "pricing-plans", Title: "Pricing Plans", Category: "pricing", Tags: []string{"pricing", "sales"},
		Body: "Starter: ₹1,499/month, up to 5 users. Growth: ₹4,999/month, up to 25 users. " +
			"Enterprise: custom pricing, unlimited users, dedicated account executive. Annual plans get 2 months free."},
	{ID: "lead-qualification", Title: "Lead Qualification Guide", Category: "faq", Tags: []string{"sales", "leads"},
		Body: "A qualified lead has a business email, a company with 10+ employees, and a stated need within 90 days. " +
			"Score 70+ means book a meeting with an account executive; below 40 means send nurture content."},
}

type Knowledge struct {
	docs map[string]Doc
}

func New(docs []Doc) *Knowledge {
	k := &Knowledge{docs: map[string]Doc{}}
	for _, d := range docs {
		k.docs[d.ID] = d
	}
	return k
}

func Register(s *mcp.Server) { New(SeedDocs).Register(s) }

func (k *Knowledge) Register(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{Name: "search_docs", Description: "Search internal docs (policies, pricing, FAQs) by keyword. Returns ranked snippets; call get_doc for the full text."}, k.SearchDocs)
	mcp.AddTool(s, &mcp.Tool{Name: "get_doc", Description: "Get the full text of an internal doc by ID."}, k.GetDoc)
}

func (k *Knowledge) SearchDocs(_ context.Context, _ *mcp.CallToolRequest, in SearchDocsIn) (*mcp.CallToolResult, SearchDocsOut, error) {
	terms := strings.Fields(strings.ToLower(in.Query))
	if len(terms) == 0 {
		return nil, SearchDocsOut{}, fmt.Errorf("query is required")
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 5
	}

	hits := []SearchHit{}
	for _, d := range k.docs {
		if in.Category != "" && d.Category != in.Category {
			continue
		}
		title, body, tags := strings.ToLower(d.Title), strings.ToLower(d.Body), strings.ToLower(strings.Join(d.Tags, " "))
		score := 0
		for _, t := range terms {
			if strings.Contains(title, t) {
				score += 3
			}
			if strings.Contains(tags, t) {
				score += 2
			}
			score += strings.Count(body, t)
		}
		if score > 0 {
			hits = append(hits, SearchHit{ID: d.ID, Title: d.Title, Category: d.Category, Snippet: snippet(d.Body, 160), Score: score})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].ID < hits[j].ID
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return nil, SearchDocsOut{Results: hits}, nil
}

func (k *Knowledge) GetDoc(_ context.Context, _ *mcp.CallToolRequest, in GetDocIn) (*mcp.CallToolResult, Doc, error) {
	d, ok := k.docs[in.ID]
	if !ok {
		return nil, Doc{}, fmt.Errorf("doc %s not found", in.ID)
	}
	return nil, d, nil
}

func snippet(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
