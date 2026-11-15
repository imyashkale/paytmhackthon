// Package tools implements the calendar MCP tools: availability and booking.
package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/imyashkale/paytmhackthon/internal/platform"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Working hours used to compute free slots, in the calendar's time zone.
const (
	dayStart = 10
	dayEnd   = 18
)

type Meeting struct {
	ID        string    `json:"id"`
	Owner     string    `json:"owner"`
	Title     string    `json:"title"`
	Attendees []string  `json:"attendees"`
	Start     time.Time `json:"start"`
	End       time.Time `json:"end"`
}

type Slot struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type CheckAvailabilityIn struct {
	Owner           string `json:"owner" jsonschema:"calendar owner, e.g. an account executive's email"`
	Date            string `json:"date" jsonschema:"day to check, YYYY-MM-DD"`
	DurationMinutes int    `json:"duration_minutes,omitempty" jsonschema:"meeting length in minutes (default 30)"`
}

type CheckAvailabilityOut struct {
	Owner string `json:"owner"`
	Slots []Slot `json:"slots"`
}

type ScheduleMeetingIn struct {
	Owner           string   `json:"owner" jsonschema:"calendar owner"`
	Title           string   `json:"title" jsonschema:"meeting title"`
	Attendees       []string `json:"attendees" jsonschema:"attendee emails"`
	Start           string   `json:"start" jsonschema:"start time, RFC 3339, e.g. 2026-11-20T11:00:00+05:30"`
	DurationMinutes int      `json:"duration_minutes,omitempty" jsonschema:"meeting length in minutes (default 30)"`
}

type ListMeetingsIn struct {
	Owner string `json:"owner" jsonschema:"calendar owner"`
}

type ListMeetingsOut struct {
	Meetings []Meeting `json:"meetings"`
}

type Calendar struct {
	meetings *platform.Store[Meeting]
	loc      *time.Location
}

func New(loc *time.Location) *Calendar {
	return &Calendar{meetings: platform.NewStore[Meeting]("MTG"), loc: loc}
}

func Register(s *mcp.Server) {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		loc = time.FixedZone("IST", 5*60*60+30*60)
	}
	New(loc).Register(s)
}

func (c *Calendar) Register(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{Name: "check_availability", Description: fmt.Sprintf(
		"List free slots for a calendar owner on a given day, within working hours %02d:00–%02d:00 %s.", dayStart, dayEnd, c.loc)}, c.CheckAvailability)
	mcp.AddTool(s, &mcp.Tool{Name: "schedule_meeting", Description: "Book a meeting on an owner's calendar. Fails if the slot overlaps an existing meeting."}, c.ScheduleMeeting)
	mcp.AddTool(s, &mcp.Tool{Name: "list_meetings", Description: "List meetings on an owner's calendar."}, c.ListMeetings)
}

func duration(min int) (time.Duration, error) {
	if min == 0 {
		min = 30
	}
	if min < 5 || min > 8*60 {
		return 0, fmt.Errorf("duration_minutes must be between 5 and 480")
	}
	return time.Duration(min) * time.Minute, nil
}

func (c *Calendar) busy(owner string, start, end time.Time) bool {
	return len(c.meetings.List(func(m Meeting) bool {
		return m.Owner == owner && start.Before(m.End) && m.Start.Before(end)
	})) > 0
}

func (c *Calendar) CheckAvailability(_ context.Context, _ *mcp.CallToolRequest, in CheckAvailabilityIn) (*mcp.CallToolResult, CheckAvailabilityOut, error) {
	day, err := time.ParseInLocation("2006-01-02", in.Date, c.loc)
	if err != nil {
		return nil, CheckAvailabilityOut{}, fmt.Errorf("date must be YYYY-MM-DD: %v", err)
	}
	d, err := duration(in.DurationMinutes)
	if err != nil {
		return nil, CheckAvailabilityOut{}, err
	}
	out := CheckAvailabilityOut{Owner: in.Owner, Slots: []Slot{}}
	end := day.Add(dayEnd * time.Hour)
	for t := day.Add(dayStart * time.Hour); !t.Add(d).After(end); t = t.Add(30 * time.Minute) {
		if !c.busy(in.Owner, t, t.Add(d)) {
			out.Slots = append(out.Slots, Slot{Start: t, End: t.Add(d)})
		}
	}
	return nil, out, nil
}

func (c *Calendar) ScheduleMeeting(_ context.Context, _ *mcp.CallToolRequest, in ScheduleMeetingIn) (*mcp.CallToolResult, Meeting, error) {
	if in.Owner == "" || in.Title == "" {
		return nil, Meeting{}, fmt.Errorf("owner and title are required")
	}
	start, err := time.Parse(time.RFC3339, in.Start)
	if err != nil {
		return nil, Meeting{}, fmt.Errorf("start must be RFC 3339: %v", err)
	}
	d, err := duration(in.DurationMinutes)
	if err != nil {
		return nil, Meeting{}, err
	}
	start = start.In(c.loc)
	end := start.Add(d)
	if c.busy(in.Owner, start, end) {
		return nil, Meeting{}, fmt.Errorf("%s is busy between %s and %s; call check_availability for free slots",
			in.Owner, start.Format(time.Kitchen), end.Format(time.Kitchen))
	}
	m := Meeting{ID: c.meetings.NextID(), Owner: in.Owner, Title: in.Title, Attendees: in.Attendees, Start: start, End: end}
	c.meetings.Put(m.ID, m)
	return nil, m, nil
}

func (c *Calendar) ListMeetings(_ context.Context, _ *mcp.CallToolRequest, in ListMeetingsIn) (*mcp.CallToolResult, ListMeetingsOut, error) {
	ms := c.meetings.List(func(m Meeting) bool { return m.Owner == in.Owner })
	return nil, ListMeetingsOut{Meetings: ms}, nil
}
