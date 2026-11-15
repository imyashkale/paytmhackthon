// Package tools implements the notifications MCP tools. Messages are recorded
// in an outbox; real Slack/email/SMS delivery plugs in behind Sender.
package tools

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/imyashkale/paytmhackthon/internal/platform"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var channels = []string{"slack", "email", "sms"}

type Notification struct {
	ID      string    `json:"id"`
	Channel string    `json:"channel"`
	To      string    `json:"to"`
	Subject string    `json:"subject,omitempty"`
	Message string    `json:"message"`
	Status  string    `json:"status"`
	Error   string    `json:"error,omitempty"`
	SentAt  time.Time `json:"sent_at"`
}

// Sender delivers a notification. Swap in Slack/SMTP/SMS providers here.
type Sender func(ctx context.Context, n Notification) error

// LogSender only logs the message. Used until real providers are configured.
func LogSender(_ context.Context, n Notification) error {
	slog.Info("notification", "id", n.ID, "channel", n.Channel, "to", n.To)
	return nil
}

type SendIn struct {
	Channel string `json:"channel" jsonschema:"slack, email or sms"`
	To      string `json:"to" jsonschema:"Slack channel (#name), email address or phone number in E.164 format"`
	Subject string `json:"subject,omitempty" jsonschema:"subject line, used for email"`
	Message string `json:"message" jsonschema:"message body"`
}

type ListIn struct {
	To string `json:"to,omitempty" jsonschema:"only notifications sent to this recipient"`
}

type ListOut struct {
	Notifications []Notification `json:"notifications"`
}

type Notifications struct {
	outbox *platform.Store[Notification]
	send   Sender
	now    func() time.Time
}

func New(send Sender) *Notifications {
	return &Notifications{outbox: platform.NewStore[Notification]("NTF"), send: send, now: time.Now}
}

func Register(s *mcp.Server) { New(LogSender).Register(s) }

func (n *Notifications) Register(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{Name: "send_notification", Description: "Send a message over Slack, email or SMS."}, n.Send)
	mcp.AddTool(s, &mcp.Tool{Name: "list_notifications", Description: "List sent notifications, optionally for one recipient."}, n.List)
}

func validate(in SendIn) error {
	if !slices.Contains(channels, in.Channel) {
		return fmt.Errorf("invalid channel %q; valid: %s", in.Channel, strings.Join(channels, ", "))
	}
	if strings.TrimSpace(in.Message) == "" {
		return fmt.Errorf("message is required")
	}
	switch in.Channel {
	case "slack":
		if !strings.HasPrefix(in.To, "#") && !strings.HasPrefix(in.To, "@") {
			return fmt.Errorf("slack recipient must start with # or @")
		}
	case "email":
		if !strings.Contains(in.To, "@") {
			return fmt.Errorf("invalid email address %q", in.To)
		}
	case "sms":
		if !strings.HasPrefix(in.To, "+") || len(in.To) < 8 {
			return fmt.Errorf("sms recipient must be in E.164 format, e.g. +919876543210")
		}
	}
	return nil
}

func (n *Notifications) Send(ctx context.Context, _ *mcp.CallToolRequest, in SendIn) (*mcp.CallToolResult, Notification, error) {
	if err := validate(in); err != nil {
		return nil, Notification{}, err
	}
	msg := Notification{
		ID: n.outbox.NextID(), Channel: in.Channel, To: in.To, Subject: in.Subject,
		Message: in.Message, Status: "sent", SentAt: n.now(),
	}
	if err := n.send(ctx, msg); err != nil {
		msg.Status, msg.Error = "failed", err.Error()
	}
	n.outbox.Put(msg.ID, msg)
	if msg.Status == "failed" {
		return nil, msg, fmt.Errorf("delivery failed: %s", msg.Error)
	}
	return nil, msg, nil
}

func (n *Notifications) List(_ context.Context, _ *mcp.CallToolRequest, in ListIn) (*mcp.CallToolResult, ListOut, error) {
	out := n.outbox.List(func(m Notification) bool { return in.To == "" || m.To == in.To })
	return nil, ListOut{Notifications: out}, nil
}
