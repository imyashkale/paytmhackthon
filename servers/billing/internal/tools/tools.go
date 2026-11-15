// Package tools implements the billing MCP tools: invoices, refunds and
// credits. Amounts are in paise (1 INR = 100 paise) to avoid float rounding.
package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/imyashkale/paytmhackthon/internal/platform"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// DefaultApprovalLimit is the refund amount (in paise) above which a human
// must approve. ₹5,000.
const DefaultApprovalLimit int64 = 5_000_00

type Invoice struct {
	ID            string    `json:"id"`
	CustomerID    string    `json:"customer_id"`
	OrderID       string    `json:"order_id"`
	AmountPaise   int64     `json:"amount_paise"`
	RefundedPaise int64     `json:"refunded_paise"`
	Status        string    `json:"status"`
	IssuedAt      time.Time `json:"issued_at"`
}

type Refund struct {
	ID          string    `json:"id"`
	InvoiceID   string    `json:"invoice_id"`
	AmountPaise int64     `json:"amount_paise"`
	Reason      string    `json:"reason"`
	Status      string    `json:"status" jsonschema:"processed or pending_approval"`
	ApprovedBy  string    `json:"approved_by,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type Credit struct {
	ID          string    `json:"id"`
	CustomerID  string    `json:"customer_id"`
	AmountPaise int64     `json:"amount_paise"`
	Reason      string    `json:"reason"`
	CreatedAt   time.Time `json:"created_at"`
}

type GetInvoiceIn struct {
	ID string `json:"id" jsonschema:"invoice ID, e.g. INV-1001"`
}

type ListInvoicesIn struct {
	CustomerID string `json:"customer_id,omitempty" jsonschema:"only invoices for this customer"`
	OrderID    string `json:"order_id,omitempty" jsonschema:"only invoices for this order"`
}

type ListInvoicesOut struct {
	Invoices []Invoice `json:"invoices"`
}

type IssueRefundIn struct {
	InvoiceID   string `json:"invoice_id" jsonschema:"invoice to refund against"`
	AmountPaise int64  `json:"amount_paise" jsonschema:"refund amount in paise (₹1 = 100)"`
	Reason      string `json:"reason" jsonschema:"why the refund is being issued"`
}

type ApproveRefundIn struct {
	RefundID   string `json:"refund_id" jsonschema:"refund awaiting approval"`
	ApprovedBy string `json:"approved_by" jsonschema:"name or ID of the human approver"`
}

type ApplyCreditIn struct {
	CustomerID  string `json:"customer_id" jsonschema:"customer to credit"`
	AmountPaise int64  `json:"amount_paise" jsonschema:"credit amount in paise"`
	Reason      string `json:"reason" jsonschema:"why the credit is being applied"`
}

type Billing struct {
	invoices      *platform.Store[Invoice]
	refunds       *platform.Store[Refund]
	credits       *platform.Store[Credit]
	approvalLimit int64
	now           func() time.Time
}

func New(approvalLimit int64) *Billing {
	b := &Billing{
		invoices:      platform.NewStore[Invoice]("INV"),
		refunds:       platform.NewStore[Refund]("RFD"),
		credits:       platform.NewStore[Credit]("CRD"),
		approvalLimit: approvalLimit,
		now:           time.Now,
	}
	for _, inv := range []Invoice{
		{CustomerID: "CUST-1001", OrderID: "ORD-5001", AmountPaise: 1_499_00},
		{CustomerID: "CUST-1001", OrderID: "ORD-5001", AmountPaise: 1_499_00}, // duplicate charge
		{CustomerID: "CUST-1002", OrderID: "ORD-5002", AmountPaise: 24_999_00},
	} {
		inv.ID, inv.Status, inv.IssuedAt = b.invoices.NextID(), "paid", b.now()
		b.invoices.Put(inv.ID, inv)
	}
	return b
}

func Register(s *mcp.Server) { New(DefaultApprovalLimit).Register(s) }

func (b *Billing) Register(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{Name: "get_invoice", Description: "Get an invoice by ID, including how much has been refunded."}, b.GetInvoice)
	mcp.AddTool(s, &mcp.Tool{Name: "list_invoices", Description: "List invoices for a customer or order."}, b.ListInvoices)
	mcp.AddTool(s, &mcp.Tool{Name: "issue_refund", Description: fmt.Sprintf(
		"Refund part or all of a paid invoice. Refunds above %d paise are held with status pending_approval until a human approves them.", b.approvalLimit)}, b.IssueRefund)
	mcp.AddTool(s, &mcp.Tool{Name: "approve_refund", Description: "Approve a refund that is pending approval. Only call this on explicit instruction from a human approver."}, b.ApproveRefund)
	mcp.AddTool(s, &mcp.Tool{Name: "apply_credit", Description: "Apply account credit to a customer."}, b.ApplyCredit)
}

func (b *Billing) GetInvoice(_ context.Context, _ *mcp.CallToolRequest, in GetInvoiceIn) (*mcp.CallToolResult, Invoice, error) {
	inv, ok := b.invoices.Get(in.ID)
	if !ok {
		return nil, Invoice{}, fmt.Errorf("invoice %s not found", in.ID)
	}
	return nil, inv, nil
}

func (b *Billing) ListInvoices(_ context.Context, _ *mcp.CallToolRequest, in ListInvoicesIn) (*mcp.CallToolResult, ListInvoicesOut, error) {
	if in.CustomerID == "" && in.OrderID == "" {
		return nil, ListInvoicesOut{}, fmt.Errorf("customer_id or order_id is required")
	}
	invs := b.invoices.List(func(inv Invoice) bool {
		return (in.CustomerID == "" || inv.CustomerID == in.CustomerID) &&
			(in.OrderID == "" || inv.OrderID == in.OrderID)
	})
	return nil, ListInvoicesOut{Invoices: invs}, nil
}

func (b *Billing) IssueRefund(_ context.Context, _ *mcp.CallToolRequest, in IssueRefundIn) (*mcp.CallToolResult, Refund, error) {
	if in.AmountPaise <= 0 {
		return nil, Refund{}, fmt.Errorf("amount_paise must be positive")
	}
	if in.Reason == "" {
		return nil, Refund{}, fmt.Errorf("reason is required")
	}

	status := "processed"
	if in.AmountPaise > b.approvalLimit {
		status = "pending_approval"
	}

	// Reserve the amount on the invoice now, so concurrent refunds (including
	// pending ones) can never exceed what was paid.
	_, err := b.invoices.Update(in.InvoiceID, func(inv *Invoice) error {
		if remaining := inv.AmountPaise - inv.RefundedPaise; in.AmountPaise > remaining {
			return fmt.Errorf("refund of %d paise exceeds refundable balance of %d paise", in.AmountPaise, remaining)
		}
		inv.RefundedPaise += in.AmountPaise
		if inv.RefundedPaise == inv.AmountPaise {
			inv.Status = "refunded"
		} else {
			inv.Status = "partially_refunded"
		}
		return nil
	})
	if err != nil {
		return nil, Refund{}, err
	}

	r := Refund{
		ID: b.refunds.NextID(), InvoiceID: in.InvoiceID, AmountPaise: in.AmountPaise,
		Reason: in.Reason, Status: status, CreatedAt: b.now(),
	}
	b.refunds.Put(r.ID, r)
	return nil, r, nil
}

func (b *Billing) ApproveRefund(_ context.Context, _ *mcp.CallToolRequest, in ApproveRefundIn) (*mcp.CallToolResult, Refund, error) {
	if in.ApprovedBy == "" {
		return nil, Refund{}, fmt.Errorf("approved_by is required")
	}
	r, err := b.refunds.Update(in.RefundID, func(r *Refund) error {
		if r.Status != "pending_approval" {
			return fmt.Errorf("refund %s is %s, not pending_approval", r.ID, r.Status)
		}
		r.Status, r.ApprovedBy = "processed", in.ApprovedBy
		return nil
	})
	return nil, r, err
}

func (b *Billing) ApplyCredit(_ context.Context, _ *mcp.CallToolRequest, in ApplyCreditIn) (*mcp.CallToolResult, Credit, error) {
	if in.CustomerID == "" || in.AmountPaise <= 0 || in.Reason == "" {
		return nil, Credit{}, fmt.Errorf("customer_id, a positive amount_paise and reason are required")
	}
	c := Credit{ID: b.credits.NextID(), CustomerID: in.CustomerID, AmountPaise: in.AmountPaise, Reason: in.Reason, CreatedAt: b.now()}
	b.credits.Put(c.ID, c)
	return nil, c, nil
}
