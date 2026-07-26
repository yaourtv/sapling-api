package service

import (
	"testing"
	"time"
)

// mustTime parses an RFC3339 timestamp for test fixtures.
func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return ts
}

func TestMondayIndexed(t *testing.T) {
	cases := map[time.Weekday]int{
		time.Monday:    0,
		time.Tuesday:   1,
		time.Wednesday: 2,
		time.Thursday:  3,
		time.Friday:    4,
		time.Saturday:  5,
		time.Sunday:    6,
	}
	for wd, want := range cases {
		if got := mondayIndexed(wd); got != want {
			t.Errorf("mondayIndexed(%v) = %d, want %d", wd, got, want)
		}
	}
}

func TestExpandWindowsRecurring(t *testing.T) {
	// 2026-07-13 is a Monday.
	from := mustTime(t, "2026-07-13T00:00:00Z")
	to := mustTime(t, "2026-07-20T00:00:00Z")
	windows := []window{
		{recurring: true, dayOfWeek: 0, start: 9 * time.Hour, end: 12 * time.Hour}, // Mondays 09-12
	}
	occ := expandWindows(from, to, windows)
	if len(occ) != 1 {
		t.Fatalf("got %d occurrences, want 1", len(occ))
	}
	if !occ[0].start.Equal(mustTime(t, "2026-07-13T09:00:00Z")) ||
		!occ[0].end.Equal(mustTime(t, "2026-07-13T12:00:00Z")) {
		t.Errorf("unexpected occurrence %v–%v", occ[0].start, occ[0].end)
	}
}

func TestExpandWindowsOneTime(t *testing.T) {
	from := mustTime(t, "2026-07-13T00:00:00Z")
	to := mustTime(t, "2026-07-20T00:00:00Z")
	date := mustTime(t, "2026-07-15T00:00:00Z")
	windows := []window{
		{recurring: false, date: date, start: 14 * time.Hour, end: 17 * time.Hour},
	}
	occ := expandWindows(from, to, windows)
	if len(occ) != 1 {
		t.Fatalf("got %d occurrences, want 1", len(occ))
	}
	if !occ[0].start.Equal(mustTime(t, "2026-07-15T14:00:00Z")) {
		t.Errorf("unexpected start %v", occ[0].start)
	}
	// Outside range: no occurrences.
	if got := expandWindows(from, mustTime(t, "2026-07-15T00:00:00Z"), windows); len(got) != 0 {
		t.Errorf("expected no occurrences before the date, got %d", len(got))
	}
}

func TestComputeAvailableSlotsGranules(t *testing.T) {
	from := mustTime(t, "2026-07-13T00:00:00Z")
	to := mustTime(t, "2026-07-14T00:00:00Z")
	windows := []window{
		{recurring: true, dayOfWeek: 0, start: 9 * time.Hour, end: 10*time.Hour + 30*time.Minute},
	}
	slots := computeAvailableSlots(from, to, windows, nil)
	if len(slots) != 3 {
		t.Fatalf("got %d slots, want 3 (09:00, 09:30, 10:00)", len(slots))
	}
	if !slots[0].start.Equal(mustTime(t, "2026-07-13T09:00:00Z")) {
		t.Errorf("first slot %v", slots[0].start)
	}
	if !slots[2].end.Equal(mustTime(t, "2026-07-13T10:30:00Z")) {
		t.Errorf("last slot end %v", slots[2].end)
	}
}

func TestComputeAvailableSlotsBlocked(t *testing.T) {
	from := mustTime(t, "2026-07-13T00:00:00Z")
	to := mustTime(t, "2026-07-14T00:00:00Z")
	windows := []window{
		{recurring: true, dayOfWeek: 0, start: 9 * time.Hour, end: 11 * time.Hour},
	}
	// Busy 09:45–10:15 blocks the 09:30 and 10:00 granules.
	blocked := []timeRange{{
		start: mustTime(t, "2026-07-13T09:45:00Z"),
		end:   mustTime(t, "2026-07-13T10:15:00Z"),
	}}
	slots := computeAvailableSlots(from, to, windows, blocked)
	var starts []string
	for _, s := range slots {
		starts = append(starts, s.start.UTC().Format("15:04"))
	}
	want := []string{"09:00", "10:30"}
	if len(starts) != len(want) {
		t.Fatalf("got slots %v, want %v", starts, want)
	}
	for i := range want {
		if starts[i] != want[i] {
			t.Fatalf("got slots %v, want %v", starts, want)
		}
	}
}

func TestComputeAvailableSlotsClampsToRange(t *testing.T) {
	// Range starts mid-window: granules before `from` are dropped.
	from := mustTime(t, "2026-07-13T09:30:00Z")
	to := mustTime(t, "2026-07-13T10:30:00Z")
	windows := []window{
		{recurring: true, dayOfWeek: 0, start: 9 * time.Hour, end: 12 * time.Hour},
	}
	slots := computeAvailableSlots(from, to, windows, nil)
	if len(slots) != 2 {
		t.Fatalf("got %d slots, want 2", len(slots))
	}
	if !slots[0].start.Equal(from) {
		t.Errorf("first slot %v, want %v", slots[0].start, from)
	}
}

func TestComputeAvailableSlotsOverlappingWindowsDeduped(t *testing.T) {
	from := mustTime(t, "2026-07-13T00:00:00Z")
	to := mustTime(t, "2026-07-14T00:00:00Z")
	windows := []window{
		{recurring: true, dayOfWeek: 0, start: 9 * time.Hour, end: 11 * time.Hour},
		{recurring: true, dayOfWeek: 0, start: 9 * time.Hour, end: 11 * time.Hour},
	}
	slots := computeAvailableSlots(from, to, windows, nil)
	if len(slots) != 4 {
		t.Fatalf("got %d slots, want 4 deduped granules", len(slots))
	}
}

func TestParseAndFormatClock(t *testing.T) {
	d, err := parseClock("09:30")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if d != 9*time.Hour+30*time.Minute {
		t.Errorf("got %v", d)
	}
	if formatClock(d) != "09:30" {
		t.Errorf("format: got %q", formatClock(d))
	}
	if _, err := parseClock("25:00"); err == nil {
		t.Error("expected error for 25:00")
	}
	if _, err := parseClock("bogus"); err == nil {
		t.Error("expected error for bogus input")
	}
}

func TestResolveWindow(t *testing.T) {
	dow := func(n int) *int { return &n }
	date := func(s string) *string { return &s }

	t.Run("recurring keeps the weekday and leaves specific_date null", func(t *testing.T) {
		w, err := resolveWindow(true, dow(2), nil, "09:00", "17:00")
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if w.dayOfWeek == nil || *w.dayOfWeek != 2 {
			t.Errorf("dayOfWeek = %v, want 2", w.dayOfWeek)
		}
		if w.specificDate.Valid {
			t.Error("specific_date should be null for a recurring window")
		}
	})

	t.Run("one-off keeps the date and clears a stale weekday", func(t *testing.T) {
		// A window flipped from recurring to one-off still carries its old
		// day_of_week; the table's CHECK rejects both being set.
		w, err := resolveWindow(false, dow(2), date("2026-07-20"), "09:00", "17:00")
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if w.dayOfWeek != nil {
			t.Errorf("dayOfWeek = %v, want nil", *w.dayOfWeek)
		}
		if !w.specificDate.Valid || w.specificDate.Time.Format("2006-01-02") != "2026-07-20" {
			t.Errorf("specificDate = %v", w.specificDate)
		}
	})

	t.Run("rejects invalid combinations", func(t *testing.T) {
		cases := map[string]struct {
			recurring bool
			dow       *int
			date      *string
			start     string
			end       string
		}{
			"recurring without a weekday": {true, nil, nil, "09:00", "17:00"},
			"weekday out of range":        {true, dow(7), nil, "09:00", "17:00"},
			"one-off without a date":      {false, nil, nil, "09:00", "17:00"},
			"malformed date":              {false, nil, date("20-07-2026"), "09:00", "17:00"},
			"end before start":            {true, dow(0), nil, "17:00", "09:00"},
			"end equal to start":          {true, dow(0), nil, "09:00", "09:00"},
			"malformed start":             {true, dow(0), nil, "bogus", "17:00"},
		}
		for name, c := range cases {
			if _, err := resolveWindow(c.recurring, c.dow, c.date, c.start, c.end); err == nil {
				t.Errorf("%s: expected a validation error", name)
			}
		}
	})
}

func TestFitsWindow(t *testing.T) {
	// One open window on the day, 09:00–17:00, with a meeting already at 11:00.
	occurrences := []timeRange{{
		start: mustTime(t, "2026-07-20T09:00:00Z"),
		end:   mustTime(t, "2026-07-20T17:00:00Z"),
	}}
	blocked := []timeRange{{
		start: mustTime(t, "2026-07-20T11:00:00Z"),
		end:   mustTime(t, "2026-07-20T12:00:00Z"),
	}}

	cases := []struct {
		name       string
		start, end string
		want       bool
	}{
		// The case the old 30-minute-granule rule refused.
		{"45 minutes straddling a granule boundary", "09:35:00", "10:20:00", true},
		{"1h45 panel", "13:00:00", "14:45:00", true},
		{"flush against the window start", "09:00:00", "09:15:00", true},
		{"flush against the window end", "16:45:00", "17:00:00", true},
		{"abuts the booked hour without touching it", "10:30:00", "11:00:00", true},
		{"starts before the window opens", "08:45:00", "09:30:00", false},
		{"runs past the window close", "16:30:00", "17:15:00", false},
		{"overlaps a booked meeting", "10:45:00", "11:15:00", false},
		{"swallows a booked meeting", "10:00:00", "13:00:00", false},
		{"entirely outside the window", "19:00:00", "19:30:00", false},
	}
	for _, c := range cases {
		proposal := timeRange{
			start: mustTime(t, "2026-07-20T"+c.start+"Z"),
			end:   mustTime(t, "2026-07-20T"+c.end+"Z"),
		}
		if got := fitsWindow(occurrences, blocked, proposal); got != c.want {
			t.Errorf("%s: fitsWindow = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestFitsWindowRejectsSpanningTwoOccurrences(t *testing.T) {
	// A morning and an afternoon window, back to back but distinct.
	occurrences := []timeRange{
		{start: mustTime(t, "2026-07-20T09:00:00Z"), end: mustTime(t, "2026-07-20T12:00:00Z")},
		{start: mustTime(t, "2026-07-20T12:00:00Z"), end: mustTime(t, "2026-07-20T17:00:00Z")},
	}
	proposal := timeRange{
		start: mustTime(t, "2026-07-20T11:45:00Z"),
		end:   mustTime(t, "2026-07-20T12:15:00Z"),
	}
	if fitsWindow(occurrences, nil, proposal) {
		t.Error("a proposal spanning two windows should be refused")
	}
}
