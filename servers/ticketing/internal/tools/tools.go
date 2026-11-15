// Package tools implements the ticketing MCP tools.
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

type Comment struct {
	Author string    `json:"author"`
	Body   string    `json:"body"`
	At     time.Time `json:"at"`
}

type Ticket struct {
	ID         string    `json:"id"`
	CustomerID string    `json:"customer_id"`
	Subject    string    `json:"subject"`
	Body       string    `json:"body"`
	Priority   string    `json:"priority"`
	Status     string    `json:"status"`
	Assignee   string    `json:"assignee,omitempty"`
	Comments   []Comment `json:"comments,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

var (
	priorities = []string{"low", "normal", "high", "urgent"}
	statuses   = []string{"open", "pending", "escalated", "resolved", "closed"}
)

type OpenTicketIn struct {
	CustomerID string `json:"customer_id" jsonschema:"ID of the customer who raised the issue"`
	Subject    string `json:"subject" jsonschema:"short summary of the issue"`
	Body       string `json:"body" jsonschema:"full description of the issue"`
	Priority   string `json:"priority,omitempty" jsonschema:"low, normal, high or urgent (default normal)"`
}

type GetTicketIn struct {
	ID string `json:"id" jsonschema:"ticket ID, e.g. TCK-1001"`
}

type UpdateTicketIn struct {
	ID       string `json:"id" jsonschema:"ticket ID"`
	Status   string `json:"status,omitempty" jsonschema:"open, pending, escalated, resolved or closed"`
	Assignee string `json:"assignee,omitempty" jsonschema:"agent or team to assign the ticket to"`
	Priority string `json:"priority,omitempty" jsonschema:"low, normal, high or urgent"`
	Comment  string `json:"comment,omitempty" jsonschema:"comment to add to the ticket"`
	Author   string `json:"author,omitempty" jsonschema:"who wrote the comment (default ai-agent)"`
}

type ListTicketsIn struct {
	CustomerID string `json:"customer_id,omitempty" jsonschema:"only tickets for this customer"`
	Status     string `json:"status,omitempty" jsonschema:"only tickets with this status"`
}

type ListTicketsOut struct {
	Tickets []Ticket `json:"tickets"`
}

type Ticketing struct {
	tickets *platform.Store[Ticket]
	now     func() time.Time
}

func New() *Ticketing {
	t := &Ticketing{tickets: platform.NewStore[Ticket]("TCK"), now: time.Now}
	t.open(OpenTicketIn{CustomerID: "CUST-1001", Subject: "Refund for duplicate charge", Body: "I was charged twice for order ORD-5001.", Priority: "high"})
	return t
}

func Register(s *mcp.Server) { New().Register(s) }

func (t *Ticketing) Register(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{Name: "open_ticket", Description: "Open a new support ticket for a customer."}, t.OpenTicket)
	mcp.AddTool(s, &mcp.Tool{Name: "get_ticket", Description: "Get a ticket and its comments by ID."}, t.GetTicket)
	mcp.AddTool(s, &mcp.Tool{Name: "update_ticket", Description: "Change a ticket's status, assignee or priority, and optionally add a comment. Set status to escalated to hand off to a human."}, t.UpdateTicket)
	mcp.AddTool(s, &mcp.Tool{Name: "list_tickets", Description: "List tickets, optionally filtered by customer or status."}, t.ListTickets)
}

func (t *Ticketing) OpenTicket(_ context.Context, _ *mcp.CallToolRequest, in OpenTicketIn) (*mcp.CallToolResult, Ticket, error) {
	if in.CustomerID == "" || strings.TrimSpace(in.Subject) == "" {
		return nil, Ticket{}, fmt.Errorf("customer_id and subject are required")
	}
	if in.Priority != "" && !slices.Contains(priorities, in.Priority) {
		return nil, Ticket{}, fmt.Errorf("invalid priority %q; valid: %s", in.Priority, strings.Join(priorities, ", "))
	}
	return nil, t.open(in), nil
}

func (t *Ticketing) open(in OpenTicketIn) Ticket {
	if in.Priority == "" {
		in.Priority = "normal"
	}
	now := t.now()
	tk := Ticket{
		ID: t.tickets.NextID(), CustomerID: in.CustomerID, Subject: in.Subject, Body: in.Body,
		Priority: in.Priority, Status: "open", CreatedAt: now, UpdatedAt: now,
	}
	t.tickets.Put(tk.ID, tk)
	return tk
}

func (t *Ticketing) GetTicket(_ context.Context, _ *mcp.CallToolRequest, in GetTicketIn) (*mcp.CallToolResult, Ticket, error) {
	tk, ok := t.tickets.Get(in.ID)
	if !ok {
		return nil, Ticket{}, fmt.Errorf("ticket %s not found", in.ID)
	}
	return nil, tk, nil
}

func (t *Ticketing) UpdateTicket(_ context.Context, _ *mcp.CallToolRequest, in UpdateTicketIn) (*mcp.CallToolResult, Ticket, error) {
	tk, err := t.tickets.Update(in.ID, func(tk *Ticket) error {
		if in.Status != "" {
			if !slices.Contains(statuses, in.Status) {
				return fmt.Errorf("invalid status %q; valid: %s", in.Status, strings.Join(statuses, ", "))
			}
			tk.Status = in.Status
		}
		if in.Priority != "" {
			if !slices.Contains(priorities, in.Priority) {
				return fmt.Errorf("invalid priority %q; valid: %s", in.Priority, strings.Join(priorities, ", "))
			}
			tk.Priority = in.Priority
		}
		if in.Assignee != "" {
			tk.Assignee = in.Assignee
		}
		if in.Comment != "" {
			author := in.Author
			if author == "" {
				author = "ai-agent"
			}
			tk.Comments = append(tk.Comments, Comment{Author: author, Body: in.Comment, At: t.now()})
		}
		tk.UpdatedAt = t.now()
		return nil
	})
	return nil, tk, err
}

func (t *Ticketing) ListTickets(_ context.Context, _ *mcp.CallToolRequest, in ListTicketsIn) (*mcp.CallToolResult, ListTicketsOut, error) {
	tickets := t.tickets.List(func(tk Ticket) bool {
		return (in.CustomerID == "" || tk.CustomerID == in.CustomerID) &&
			(in.Status == "" || tk.Status == in.Status)
	})
	return nil, ListTicketsOut{Tickets: tickets}, nil
}
