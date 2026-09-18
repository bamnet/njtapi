package njtapi

import (
	"context"
	"fmt"
	"time"

	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
	"google.golang.org/protobuf/proto"
)

// Alert is a service alert from the RailData GTFS-realtime feed.
//
// Route, stop and trip IDs are GTFS IDs from NJTransit's static GTFS feed
// (e.g. route "10", stop "168"), not the two-character station codes used by
// Client.
type Alert struct {
	ID          string // Feed entity ID.
	Header      string // Short summary.
	Description string // Full text; often identical to Header.
	URL         string // Link to more information, if any.

	// Cause, Effect and Severity are GTFS-realtime enum names, such as
	// "UNKNOWN_CAUSE", "DETOUR" or "WARNING". NJTransit has only been
	// observed to send "UNKNOWN_CAUSE" and "UNKNOWN_EFFECT".
	Cause    string
	Effect   string
	Severity string

	ActivePeriods []AlertPeriod // When the alert is in effect.

	// Entities lists exactly what the alert applies to. RouteIDs, StopIDs and
	// TripIDs flatten Entities into de-duplicated lists for convenience;
	// RouteIDs includes the routes of affected trips, so check Entities to
	// tell a route-wide alert from one about a single trip.
	Entities []AlertEntity
	RouteIDs []string
	StopIDs  []string
	TripIDs  []string
}

// AlertPeriod is a time range an alert is active for. A zero Start or End
// means the range is open on that side.
type AlertPeriod struct {
	Start time.Time
	End   time.Time
}

// AlertEntity is a single informed entity of an alert. Every non-empty field
// must match: {RouteID: "13", StopID: "63"} means route 13 at stop 63.
type AlertEntity struct {
	AgencyID string
	RouteID  string // Taken from the trip if the entity names a trip without a route.
	StopID   string
	TripID   string
}

// Alerts returns NJTransit's current rail service alerts.
//
// Header and Description are returned as sent. NJTransit's feed replaces some
// non-ASCII characters with '?', most often an en dash
// ("Concert ? Friday, September 25"); the text is not rewritten because a '?'
// can't be reliably told apart from a real question mark.
//
// Alerts come from the RailData GTFS-realtime API and need the WithRailData
// option; without it Alerts returns ErrRailDataNotConfigured.
func (c *Client) Alerts(ctx context.Context) ([]Alert, error) {
	if c.railData == nil {
		return nil, ErrRailDataNotConfigured
	}
	body, err := c.railData.fetch(ctx, railDataGTFSRT, "getAlerts")
	if err != nil {
		return nil, err
	}
	var feed gtfs.FeedMessage
	if err := proto.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("decoding alerts feed: %w", err)
	}

	var alerts []Alert
	for _, e := range feed.GetEntity() {
		if e.GetIsDeleted() || e.GetAlert() == nil {
			continue
		}
		alerts = append(alerts, c.convertAlert(e.GetId(), e.GetAlert()))
	}
	return alerts, nil
}

func (c *Client) convertAlert(id string, a *gtfs.Alert) Alert {
	out := Alert{
		ID:          id,
		Header:      translation(a.GetHeaderText()),
		Description: translation(a.GetDescriptionText()),
		URL:         translation(a.GetUrl()),
		Cause:       a.GetCause().String(),
		Effect:      a.GetEffect().String(),
		Severity:    a.GetSeverityLevel().String(),
	}

	for _, p := range a.GetActivePeriod() {
		out.ActivePeriods = append(out.ActivePeriods, AlertPeriod{
			Start: c.unixTime(p.GetStart()),
			End:   c.unixTime(p.GetEnd()),
		})
	}

	seen := map[string]bool{}
	add := func(list *[]string, kind, id string) {
		if id == "" || seen[kind+id] {
			return
		}
		seen[kind+id] = true
		*list = append(*list, id)
	}
	for _, s := range a.GetInformedEntity() {
		ent := AlertEntity{
			AgencyID: s.GetAgencyId(),
			RouteID:  s.GetRouteId(),
			StopID:   s.GetStopId(),
			TripID:   s.GetTrip().GetTripId(),
		}
		if ent.RouteID == "" {
			ent.RouteID = s.GetTrip().GetRouteId()
		}
		out.Entities = append(out.Entities, ent)
		add(&out.RouteIDs, "r", ent.RouteID)
		add(&out.StopIDs, "s", ent.StopID)
		add(&out.TripIDs, "t", ent.TripID)
	}
	return out
}

// unixTime converts a GTFS-realtime POSIX timestamp, where 0 means unset.
func (c *Client) unixTime(ts uint64) time.Time {
	if ts == 0 {
		return time.Time{}
	}
	loc := c.location
	if loc == nil {
		loc = time.UTC
	}
	return time.Unix(int64(ts), 0).In(loc) //nolint:gosec // POSIX timestamps fit in int64.
}

// translation picks the English (or untagged) text of a TranslatedString,
// falling back to the first translation.
func translation(s *gtfs.TranslatedString) string {
	ts := s.GetTranslation()
	for _, t := range ts {
		switch t.GetLanguage() {
		case "", "en", "en-US":
			return t.GetText()
		}
	}
	if len(ts) > 0 {
		return ts[0].GetText()
	}
	return ""
}
