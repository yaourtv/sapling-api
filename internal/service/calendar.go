package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/egoralekseychik/job-tracker-api/internal/db/query"
	"github.com/egoralekseychik/job-tracker-api/internal/model"
)

const (
	maxTitleLen       = 40
	maxDescriptionLen = 512
	slotGranule       = 30 * time.Minute
	// The public calendar page requests a ±4-week window (from = -4 weeks,
	// to = from + 9 weeks = 63 days). Keep this above that, with a little slack
	// for week-alignment and DST shifts, while still bounding the query.
	maxPublicRange = 70 * 24 * time.Hour
)

type CalendarService struct {
	pool *pgxpool.Pool
	q    *query.Queries
}

func NewCalendarService(pool *pgxpool.Pool) *CalendarService {
	return &CalendarService{pool: pool, q: query.New(pool)}
}

// ---- Events ----

type EventInput struct {
	Title       string     `json:"title"`
	Description *string    `json:"description"`
	StartsAt    *time.Time `json:"starts_at"`
	EndsAt      *time.Time `json:"ends_at"`
}

func validateEventFields(title string, description *string, starts, ends time.Time) error {
	if title == "" {
		return validationf("title is required")
	}
	if len(title) > maxTitleLen {
		return validationf("title must be at most %d characters", maxTitleLen)
	}
	if description != nil && len(*description) > maxDescriptionLen {
		return validationf("description must be at most %d characters", maxDescriptionLen)
	}
	if starts.IsZero() || ends.IsZero() {
		return validationf("starts_at and ends_at are required")
	}
	if !ends.After(starts) {
		return validationf("ends_at must be after starts_at")
	}
	return nil
}

func (s *CalendarService) CreateEvent(ctx context.Context, userID uuid.UUID, in EventInput) (*model.Event, error) {
	var starts, ends time.Time
	if in.StartsAt != nil {
		starts = in.StartsAt.UTC()
	}
	if in.EndsAt != nil {
		ends = in.EndsAt.UTC()
	}
	if err := validateEventFields(in.Title, in.Description, starts, ends); err != nil {
		return nil, err
	}
	row, err := s.q.CreateEvent(ctx, query.CreateEventParams{
		UserID:      userID,
		Title:       in.Title,
		Description: in.Description,
		StartsAt:    starts,
		EndsAt:      ends,
	})
	if err != nil {
		return nil, err
	}
	m := eventToModel(row)
	return &m, nil
}

func (s *CalendarService) ListEvents(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]model.Event, error) {
	rows, err := s.q.ListEventsInRange(ctx, query.ListEventsInRangeParams{
		UserID: userID,
		Until:  to,
		Since:  from,
	})
	if err != nil {
		return nil, err
	}
	out := make([]model.Event, 0, len(rows))
	for _, r := range rows {
		out = append(out, eventToModel(r))
	}
	return out, nil
}

func (s *CalendarService) UpdateEvent(ctx context.Context, userID, eventID uuid.UUID, in EventInput) (*model.Event, error) {
	current, err := s.q.GetEvent(ctx, query.GetEventParams{ID: eventID, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	params := query.UpdateEventParams{
		ID:          eventID,
		UserID:      userID,
		Title:       current.Title,
		Description: current.Description,
		StartsAt:    current.StartsAt,
		EndsAt:      current.EndsAt,
	}
	if in.Title != "" {
		params.Title = in.Title
	}
	if in.Description != nil {
		params.Description = in.Description
	}
	if in.StartsAt != nil {
		params.StartsAt = in.StartsAt.UTC()
	}
	if in.EndsAt != nil {
		params.EndsAt = in.EndsAt.UTC()
	}
	if err := validateEventFields(params.Title, params.Description, params.StartsAt, params.EndsAt); err != nil {
		return nil, err
	}
	row, err := s.q.UpdateEvent(ctx, params)
	if err != nil {
		return nil, err
	}
	m := eventToModel(row)
	return &m, nil
}

func (s *CalendarService) DeleteEvent(ctx context.Context, userID, eventID uuid.UUID) error {
	n, err := s.q.DeleteEvent(ctx, query.DeleteEventParams{ID: eventID, UserID: userID})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- Availability windows ----

type AvailabilityInput struct {
	IsRecurring  bool    `json:"is_recurring"`
	DayOfWeek    *int    `json:"day_of_week"`   // 0=Mon … 6=Sun
	SpecificDate *string `json:"specific_date"` // YYYY-MM-DD
	StartTime    string  `json:"start_time"`    // HH:MM (UTC)
	EndTime      string  `json:"end_time"`      // HH:MM (UTC)
}

func (s *CalendarService) CreateAvailability(ctx context.Context, userID uuid.UUID, in AvailabilityInput) (*model.AvailabilityWindow, error) {
	start, err := parseClock(in.StartTime)
	if err != nil {
		return nil, validationf("invalid start_time: %v", err)
	}
	end, err := parseClock(in.EndTime)
	if err != nil {
		return nil, validationf("invalid end_time: %v", err)
	}
	if end <= start {
		return nil, validationf("end_time must be after start_time")
	}

	params := query.CreateAvailabilityWindowParams{
		UserID:      userID,
		IsRecurring: in.IsRecurring,
		StartTime:   pgtype.Time{Microseconds: start.Microseconds(), Valid: true},
		EndTime:     pgtype.Time{Microseconds: end.Microseconds(), Valid: true},
	}
	if in.IsRecurring {
		if in.DayOfWeek == nil || *in.DayOfWeek < 0 || *in.DayOfWeek > 6 {
			return nil, validationf("day_of_week must be 0 (Mon) … 6 (Sun) for recurring windows")
		}
		dow := int16(*in.DayOfWeek)
		params.DayOfWeek = &dow
	} else {
		if in.SpecificDate == nil {
			return nil, validationf("specific_date is required for one-time windows")
		}
		d, err := time.ParseInLocation("2006-01-02", *in.SpecificDate, time.UTC)
		if err != nil {
			return nil, validationf("specific_date must be YYYY-MM-DD")
		}
		params.SpecificDate = pgtype.Date{Time: d, Valid: true}
	}

	row, err := s.q.CreateAvailabilityWindow(ctx, params)
	if err != nil {
		return nil, err
	}
	m := availabilityToModel(row)
	return &m, nil
}

func (s *CalendarService) ListAvailability(ctx context.Context, userID uuid.UUID) ([]model.AvailabilityWindow, error) {
	rows, err := s.q.ListAvailabilityWindows(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]model.AvailabilityWindow, 0, len(rows))
	for _, r := range rows {
		out = append(out, availabilityToModel(r))
	}
	return out, nil
}

func (s *CalendarService) DeleteAvailability(ctx context.Context, userID, windowID uuid.UUID) error {
	n, err := s.q.DeleteAvailabilityWindow(ctx, query.DeleteAvailabilityWindowParams{ID: windowID, UserID: userID})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- Invitations (authenticated side) ----

func (s *CalendarService) ListInvitations(ctx context.Context, userID uuid.UUID) ([]model.Invitation, error) {
	rows, err := s.q.ListInvitations(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]model.Invitation, 0, len(rows))
	for _, r := range rows {
		out = append(out, invitationToModel(r))
	}
	return out, nil
}

// AcceptInvitation converts the invitation into a calendar event and deletes it.
func (s *CalendarService) AcceptInvitation(ctx context.Context, userID, invitationID uuid.UUID) (*model.Event, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	qtx := s.q.WithTx(tx)

	inv, err := qtx.GetInvitation(ctx, query.GetInvitationParams{ID: invitationID, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	event, err := qtx.CreateEvent(ctx, query.CreateEventParams{
		UserID:      userID,
		Title:       inv.Title,
		Description: inv.Description,
		StartsAt:    inv.ProposedStarts,
		EndsAt:      inv.ProposedEnds,
	})
	if err != nil {
		return nil, err
	}
	if _, err := qtx.DeleteInvitation(ctx, query.DeleteInvitationParams{ID: invitationID, UserID: userID}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	m := eventToModel(event)
	return &m, nil
}

func (s *CalendarService) DeclineInvitation(ctx context.Context, userID, invitationID uuid.UUID) error {
	n, err := s.q.DeleteInvitation(ctx, query.DeleteInvitationParams{ID: invitationID, UserID: userID})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- Public calendar ----

// PublicCalendar resolves a calendar token and returns busy/tentative/available
// slots in [from, to). Titles are never included.
func (s *CalendarService) PublicCalendar(ctx context.Context, calToken string, from, to time.Time) (*model.PublicCalendar, error) {
	user, err := s.q.GetUserByCalendarToken(ctx, calToken)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if !to.After(from) {
		return nil, validationf("to must be after from")
	}
	if to.Sub(from) > maxPublicRange {
		return nil, validationf("date range too large")
	}

	busy, tentative, available, err := s.slotsForRange(ctx, user.ID, from, to)
	if err != nil {
		return nil, err
	}

	slots := make([]model.Slot, 0, len(busy)+len(tentative)+len(available))
	for _, r := range busy {
		slots = append(slots, model.Slot{StartsAt: r.start, EndsAt: r.end, Type: model.SlotBusy})
	}
	for _, r := range tentative {
		slots = append(slots, model.Slot{StartsAt: r.start, EndsAt: r.end, Type: model.SlotTentative})
	}
	for _, r := range available {
		slots = append(slots, model.Slot{StartsAt: r.start, EndsAt: r.end, Type: model.SlotAvailable})
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i].StartsAt.Before(slots[j].StartsAt) })

	return &model.PublicCalendar{Timezone: user.Timezone, Slots: slots}, nil
}

type PublicInviteInput struct {
	Title          string     `json:"title"`
	Description    *string    `json:"description"`
	ProposedStarts *time.Time `json:"proposed_starts"`
	ProposedEnds   *time.Time `json:"proposed_ends"`
}

// SubmitInvitation validates a public invitation against the user's available
// slots and stores it as pending.
func (s *CalendarService) SubmitInvitation(ctx context.Context, calToken string, in PublicInviteInput) (*model.Invitation, error) {
	user, err := s.q.GetUserByCalendarToken(ctx, calToken)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if in.ProposedStarts == nil || in.ProposedEnds == nil {
		return nil, validationf("proposed_starts and proposed_ends are required")
	}
	starts := in.ProposedStarts.UTC()
	ends := in.ProposedEnds.UTC()
	if err := validateEventFields(in.Title, in.Description, starts, ends); err != nil {
		return nil, err
	}
	dur := ends.Sub(starts)
	if dur <= 0 || dur%slotGranule != 0 {
		return nil, validationf("duration must be a positive multiple of 30 minutes")
	}
	if starts.Before(time.Now().UTC()) {
		return nil, validationf("proposed time is in the past")
	}

	// Recompute availability around the proposal and require every 30-minute
	// granule of the proposal to be an available slot.
	dayStart := starts.Truncate(24 * time.Hour).Add(-24 * time.Hour)
	dayEnd := ends.Truncate(24 * time.Hour).Add(48 * time.Hour)
	_, _, available, err := s.slotsForRange(ctx, user.ID, dayStart, dayEnd)
	if err != nil {
		return nil, err
	}
	availableStarts := make(map[int64]bool, len(available))
	for _, r := range available {
		availableStarts[r.start.Unix()] = true
	}
	for t := starts; t.Before(ends); t = t.Add(slotGranule) {
		if !availableStarts[t.Unix()] {
			return nil, validationf("proposed time is not within an available slot")
		}
	}

	row, err := s.q.CreateInvitation(ctx, query.CreateInvitationParams{
		UserID:         user.ID,
		Title:          in.Title,
		Description:    in.Description,
		ProposedStarts: starts,
		ProposedEnds:   ends,
	})
	if err != nil {
		return nil, err
	}
	m := invitationToModel(row)
	return &m, nil
}

// slotsForRange loads events, invitations, and availability windows for the
// user and computes the three slot classes in [from, to).
func (s *CalendarService) slotsForRange(ctx context.Context, userID uuid.UUID, from, to time.Time) (busy, tentative, available []timeRange, err error) {
	events, err := s.q.ListEventsInRange(ctx, query.ListEventsInRangeParams{UserID: userID, Until: to, Since: from})
	if err != nil {
		return nil, nil, nil, err
	}
	invites, err := s.q.ListInvitationsInRange(ctx, query.ListInvitationsInRangeParams{UserID: userID, Until: to, Since: from})
	if err != nil {
		return nil, nil, nil, err
	}
	windowRows, err := s.q.ListAvailabilityWindows(ctx, userID)
	if err != nil {
		return nil, nil, nil, err
	}

	for _, e := range events {
		busy = append(busy, timeRange{start: e.StartsAt, end: e.EndsAt})
	}
	for _, i := range invites {
		tentative = append(tentative, timeRange{start: i.ProposedStarts, end: i.ProposedEnds})
	}
	windows := make([]window, 0, len(windowRows))
	for _, w := range windowRows {
		windows = append(windows, windowFromRow(w))
	}
	blocked := append(append([]timeRange{}, busy...), tentative...)
	available = computeAvailableSlots(from, to, windows, blocked)
	return busy, tentative, available, nil
}

// ---- Slot computation (pure, unit-tested) ----

type timeRange struct {
	start, end time.Time
}

func (r timeRange) overlaps(o timeRange) bool {
	return r.start.Before(o.end) && o.start.Before(r.end)
}

// window is a normalized availability window (all times UTC).
type window struct {
	recurring bool
	dayOfWeek int           // 0=Mon … 6=Sun; valid when recurring
	date      time.Time     // UTC midnight; valid when !recurring
	start     time.Duration // offset from midnight
	end       time.Duration
}

func windowFromRow(w query.AvailabilityWindow) window {
	out := window{
		recurring: w.IsRecurring,
		start:     time.Duration(w.StartTime.Microseconds) * time.Microsecond,
		end:       time.Duration(w.EndTime.Microseconds) * time.Microsecond,
	}
	if w.DayOfWeek != nil {
		out.dayOfWeek = int(*w.DayOfWeek)
	}
	if w.SpecificDate.Valid {
		out.date = w.SpecificDate.Time.UTC().Truncate(24 * time.Hour)
	}
	return out
}

// mondayIndexed converts Go's Sunday=0 weekday to the spec's Monday=0.
func mondayIndexed(wd time.Weekday) int {
	return (int(wd) + 6) % 7
}

// expandWindows materializes window occurrences as concrete UTC ranges for
// every day intersecting [from, to).
func expandWindows(from, to time.Time, windows []window) []timeRange {
	var out []timeRange
	day := from.UTC().Truncate(24 * time.Hour)
	for ; day.Before(to); day = day.Add(24 * time.Hour) {
		for _, w := range windows {
			if w.recurring {
				if mondayIndexed(day.Weekday()) != w.dayOfWeek {
					continue
				}
			} else if !w.date.Equal(day) {
				continue
			}
			out = append(out, timeRange{start: day.Add(w.start), end: day.Add(w.end)})
		}
	}
	return out
}

// computeAvailableSlots returns every 30-minute granule inside an availability
// window occurrence that lies within [from, to) and does not overlap any
// blocked (busy/tentative) range. Granules are aligned to the window start.
func computeAvailableSlots(from, to time.Time, windows []window, blocked []timeRange) []timeRange {
	var out []timeRange
	seen := map[int64]bool{}
	for _, occ := range expandWindows(from, to, windows) {
		for t := occ.start; !t.Add(slotGranule).After(occ.end); t = t.Add(slotGranule) {
			granule := timeRange{start: t, end: t.Add(slotGranule)}
			if granule.start.Before(from) || granule.end.After(to) {
				continue
			}
			if seen[granule.start.Unix()] {
				continue
			}
			conflict := false
			for _, b := range blocked {
				if granule.overlaps(b) {
					conflict = true
					break
				}
			}
			if !conflict {
				seen[granule.start.Unix()] = true
				out = append(out, granule)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].start.Before(out[j].start) })
	return out
}

// ---- converters ----

func eventToModel(e query.CalendarEvent) model.Event {
	return model.Event{
		ID:          e.ID,
		Title:       e.Title,
		Description: e.Description,
		StartsAt:    e.StartsAt,
		EndsAt:      e.EndsAt,
		CreatedAt:   e.CreatedAt,
	}
}

func invitationToModel(i query.Invitation) model.Invitation {
	return model.Invitation{
		ID:             i.ID,
		Title:          i.Title,
		Description:    i.Description,
		ProposedStarts: i.ProposedStarts,
		ProposedEnds:   i.ProposedEnds,
		CreatedAt:      i.CreatedAt,
	}
}

func availabilityToModel(w query.AvailabilityWindow) model.AvailabilityWindow {
	out := model.AvailabilityWindow{
		ID:          w.ID,
		IsRecurring: w.IsRecurring,
		StartTime:   formatClock(time.Duration(w.StartTime.Microseconds) * time.Microsecond),
		EndTime:     formatClock(time.Duration(w.EndTime.Microseconds) * time.Microsecond),
	}
	if w.DayOfWeek != nil {
		d := int(*w.DayOfWeek)
		out.DayOfWeek = &d
	}
	if w.SpecificDate.Valid {
		s := w.SpecificDate.Time.UTC().Format("2006-01-02")
		out.SpecificDate = &s
	}
	return out
}

// parseClock parses "HH:MM" (or "HH:MM:SS") into an offset from midnight.
func parseClock(s string) (time.Duration, error) {
	layouts := []string{"15:04", "15:04:05"}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return time.Duration(t.Hour())*time.Hour +
				time.Duration(t.Minute())*time.Minute +
				time.Duration(t.Second())*time.Second, nil
		}
	}
	return 0, fmt.Errorf("expected HH:MM, got %q", s)
}

func formatClock(d time.Duration) string {
	return fmt.Sprintf("%02d:%02d", int(d.Hours()), int(d.Minutes())%60)
}
