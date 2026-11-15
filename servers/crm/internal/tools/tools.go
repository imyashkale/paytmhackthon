// Package tools implements the CRM MCP tools: leads and their lifecycle.
package tools

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/imyashkale/paytmhackthon/internal/platform"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Lead struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Company   string    `json:"company,omitempty"`
	Source    string    `json:"source,omitempty"`
	Status    string    `json:"status"`
	Score     int       `json:"score"`
	Notes     []string  `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

var validStatuses = []string{"new", "contacted", "qualified", "meeting_booked", "won", "lost"}

type CreateLeadIn struct {
	Name    string `json:"name" jsonschema:"full name of the lead"`
	Email   string `json:"email" jsonschema:"email address of the lead"`
	Company string `json:"company,omitempty" jsonschema:"company name"`
	Source  string `json:"source,omitempty" jsonschema:"where the lead came from, e.g. website_form, chat, referral"`
}

type GetLeadIn struct {
	ID string `json:"id" jsonschema:"lead ID, e.g. LEAD-1001"`
}

type UpdateLeadIn struct {
	ID     string `json:"id" jsonschema:"lead ID"`
	Status string `json:"status,omitempty" jsonschema:"new status: new, contacted, qualified, meeting_booked, won or lost"`
	Score  *int   `json:"score,omitempty" jsonschema:"fit and intent score from 0 to 100"`
	Note   string `json:"note,omitempty" jsonschema:"activity note to append to the lead"`
}

type ListLeadsIn struct {
	Status string `json:"status,omitempty" jsonschema:"only return leads with this status"`
	Query  string `json:"query,omitempty" jsonschema:"case-insensitive match on name, email or company"`
}

type ListLeadsOut struct {
	Leads []Lead `json:"leads"`
}

type CRM struct {
	leads *platform.Store[Lead]
	now   func() time.Time
}

func New() *CRM {
	c := &CRM{leads: platform.NewStore[Lead]("LEAD"), now: time.Now}
	c.createLead(CreateLeadIn{Name: "Asha Mehta", Email: "asha@acmeretail.in", Company: "Acme Retail", Source: "website_form"})
	return c
}

func Register(s *mcp.Server) { New().Register(s) }

func (c *CRM) Register(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{Name: "create_lead", Description: "Create a new lead in the CRM."}, c.CreateLead)
	mcp.AddTool(s, &mcp.Tool{Name: "get_lead", Description: "Get a lead by ID."}, c.GetLead)
	mcp.AddTool(s, &mcp.Tool{Name: "update_lead", Description: "Update a lead's status or score, or append an activity note."}, c.UpdateLead)
	mcp.AddTool(s, &mcp.Tool{Name: "list_leads", Description: "List leads, optionally filtered by status or a search query."}, c.ListLeads)
}

func (c *CRM) CreateLead(_ context.Context, _ *mcp.CallToolRequest, in CreateLeadIn) (*mcp.CallToolResult, Lead, error) {
	if strings.TrimSpace(in.Name) == "" || !strings.Contains(in.Email, "@") {
		return nil, Lead{}, fmt.Errorf("name and a valid email are required")
	}
	for _, l := range c.leads.List(nil) {
		if strings.EqualFold(l.Email, in.Email) {
			return nil, Lead{}, fmt.Errorf("lead with email %s already exists: %s", in.Email, l.ID)
		}
	}
	return nil, c.createLead(in), nil
}

func (c *CRM) createLead(in CreateLeadIn) Lead {
	now := c.now()
	l := Lead{
		ID: c.leads.NextID(), Name: in.Name, Email: in.Email, Company: in.Company,
		Source: in.Source, Status: "new", CreatedAt: now, UpdatedAt: now,
	}
	c.leads.Put(l.ID, l)
	return l
}

func (c *CRM) GetLead(_ context.Context, _ *mcp.CallToolRequest, in GetLeadIn) (*mcp.CallToolResult, Lead, error) {
	l, ok := c.leads.Get(in.ID)
	if !ok {
		return nil, Lead{}, fmt.Errorf("lead %s not found", in.ID)
	}
	return nil, l, nil
}

func (c *CRM) UpdateLead(_ context.Context, _ *mcp.CallToolRequest, in UpdateLeadIn) (*mcp.CallToolResult, Lead, error) {
	l, err := c.leads.Update(in.ID, func(l *Lead) error {
		if in.Status != "" {
			if !slices.Contains(validStatuses, in.Status) {
				return fmt.Errorf("invalid status %q; valid: %s", in.Status, strings.Join(validStatuses, ", "))
			}
			l.Status = in.Status
		}
		if in.Score != nil {
			if *in.Score < 0 || *in.Score > 100 {
				return fmt.Errorf("score must be between 0 and 100")
			}
			l.Score = *in.Score
		}
		if in.Note != "" {
			l.Notes = append(l.Notes, in.Note)
		}
		l.UpdatedAt = c.now()
		return nil
	})
	return nil, l, err
}

func (c *CRM) ListLeads(_ context.Context, _ *mcp.CallToolRequest, in ListLeadsIn) (*mcp.CallToolResult, ListLeadsOut, error) {
	q := strings.ToLower(in.Query)
	leads := c.leads.List(func(l Lead) bool {
		if in.Status != "" && l.Status != in.Status {
			return false
		}
		return q == "" || strings.Contains(strings.ToLower(l.Name+" "+l.Email+" "+l.Company), q)
	})
	return nil, ListLeadsOut{Leads: leads}, nil
}
