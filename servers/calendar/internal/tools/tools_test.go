package tools

import (
	"context"
	"testing"
	"time"
)

func TestScheduleRejectsOverlap(t *testing.T) {
	ctx := context.Background()
	c := New(time.FixedZone("IST", 5*60*60+30*60))
	owner := "ae@example.com"

	_, _, err := c.ScheduleMeeting(ctx, nil, ScheduleMeetingIn{Owner: owner, Title: "Demo", Start: "2026-11-20T11:00:00+05:30", DurationMinutes: 60})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.ScheduleMeeting(ctx, nil, ScheduleMeetingIn{Owner: owner, Title: "Clash", Start: "2026-11-20T11:30:00+05:30"}); err == nil {
		t.Fatal("overlapping meeting booked, want error")
	}
	// Back-to-back is fine.
	if _, _, err := c.ScheduleMeeting(ctx, nil, ScheduleMeetingIn{Owner: owner, Title: "Next", Start: "2026-11-20T12:00:00+05:30"}); err != nil {
		t.Fatalf("back-to-back meeting: %v", err)
	}

	_, avail, err := c.CheckAvailability(ctx, nil, CheckAvailabilityIn{Owner: owner, Date: "2026-11-20"})
	if err != nil {
		t.Fatal(err)
	}
	// 10:00–18:00 in 30-min steps = 16 slots, minus 11:00, 11:30, 12:00.
	if len(avail.Slots) != 13 {
		t.Fatalf("free slots = %d, want 13", len(avail.Slots))
	}
	for _, s := range avail.Slots {
		if h := s.Start.Hour(); h == 11 || (h == 12 && s.Start.Minute() == 0) {
			t.Fatalf("busy slot %v reported free", s.Start)
		}
	}
}
