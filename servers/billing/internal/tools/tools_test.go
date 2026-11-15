package tools

import (
	"context"
	"testing"
)

func TestRefundGuardrails(t *testing.T) {
	ctx := context.Background()
	b := New(1_000_00) // ₹1,000 approval limit

	// INV-1003 is a ₹24,999 invoice.
	_, _, err := b.IssueRefund(ctx, nil, IssueRefundIn{InvoiceID: "INV-1003", AmountPaise: 500_00, Reason: "goodwill"})
	if err != nil {
		t.Fatal(err)
	}

	_, big, err := b.IssueRefund(ctx, nil, IssueRefundIn{InvoiceID: "INV-1003", AmountPaise: 20_000_00, Reason: "cancelled"})
	if err != nil {
		t.Fatal(err)
	}
	if big.Status != "pending_approval" {
		t.Fatalf("refund above limit: status = %q, want pending_approval", big.Status)
	}

	// Pending refunds still count against the balance: only ₹4,499 is left.
	if _, _, err := b.IssueRefund(ctx, nil, IssueRefundIn{InvoiceID: "INV-1003", AmountPaise: 5_000_00, Reason: "again"}); err == nil {
		t.Fatal("over-refund succeeded, want error")
	}

	if _, _, err := b.ApproveRefund(ctx, nil, ApproveRefundIn{RefundID: big.ID}); err == nil {
		t.Fatal("approval without approver succeeded, want error")
	}
	_, approved, err := b.ApproveRefund(ctx, nil, ApproveRefundIn{RefundID: big.ID, ApprovedBy: "support-lead"})
	if err != nil || approved.Status != "processed" {
		t.Fatalf("approve: status = %q, err = %v", approved.Status, err)
	}
	if _, _, err := b.ApproveRefund(ctx, nil, ApproveRefundIn{RefundID: big.ID, ApprovedBy: "support-lead"}); err == nil {
		t.Fatal("double approval succeeded, want error")
	}
}

func TestSmallRefundProcessesImmediately(t *testing.T) {
	b := New(DefaultApprovalLimit)
	_, r, err := b.IssueRefund(context.Background(), nil, IssueRefundIn{InvoiceID: "INV-1002", AmountPaise: 1_499_00, Reason: "duplicate charge"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "processed" {
		t.Fatalf("status = %q, want processed", r.Status)
	}
	_, inv, _ := b.GetInvoice(context.Background(), nil, GetInvoiceIn{ID: "INV-1002"})
	if inv.Status != "refunded" || inv.RefundedPaise != inv.AmountPaise {
		t.Fatalf("invoice = %+v, want fully refunded", inv)
	}
}
