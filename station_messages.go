package njtapi

import (
	"context"
	"encoding/xml"
	"html"
	"strings"
	"time"
)

const stationMessagesEndpoint = "getStationMSGXML"

// A StationMessage is a service advisory published for a station, such as
// the banner messages shown above departure boards.
type StationMessage struct {
	ID          string    // Message identifier
	Text        string    // Message text
	Type        string    // Message type, e.g. "banner"
	Agency      string    // Publishing agency, e.g. "NJT"
	PublishedAt time.Time // Time the message was published
	ParseErrors []error   // Errors encountered while parsing this message
}

// stationMessagesResponse mirrors the XML shape returned by the
// getStationMSGXML endpoint. When a station is requested the root element is
// STATION; without one the API answers with a FULLSCREENMSGS element instead.
type stationMessagesResponse struct {
	Station2Char string           `xml:"STATION_2CHAR"`
	StationName  string           `xml:"STATIONNAME"`
	Messages     []stationMessage `xml:"BANNERMSGS>MSG"`
}

// stationMessage is a single message within a BANNERMSGS element. The same
// shape is used by getStationMSGXML and getTrainScheduleXML.
type stationMessage struct {
	PubDate string `xml:"PubDate"`
	Text    string `xml:"MSGText"`
	ID      string `xml:"MSGID"`
	Type    string `xml:"MSGType"`
	Agency  string `xml:"MSGAgency"`
}

// StationMessages returns the service messages published for a station.
//
// station is a two character station code (e.g. "NY"). line optionally
// narrows the messages to a train line; pass "" for all lines. A nil slice
// is returned when there are no messages.
func (c *Client) StationMessages(ctx context.Context, station, line string) ([]StationMessage, error) {
	resp, err := c.fetch(ctx, stationMessagesEndpoint, map[string]string{
		"station":   station,
		"trainLine": line,
	})
	if err != nil {
		return nil, err
	}

	data := stationMessagesResponse{}
	if err := xml.Unmarshal(resp, &data); err != nil {
		return nil, err
	}
	return c.parseStationMessages(data.Messages), nil
}

// parseStationMessages converts raw BANNERMSGS entries into StationMessages.
func (c *Client) parseStationMessages(raw []stationMessage) []StationMessage {
	if len(raw) == 0 {
		return nil
	}
	msgs := make([]StationMessage, 0, len(raw))
	for _, r := range raw {
		m := StationMessage{
			ID:     strings.TrimSpace(r.ID),
			Text:   cleanMessageText(r.Text),
			Type:   strings.TrimSpace(r.Type),
			Agency: strings.TrimSpace(r.Agency),
		}
		if pd := strings.TrimSpace(r.PubDate); pd != "" {
			var err error
			m.PublishedAt, err = c.parseMessageTime(pd)
			if err != nil {
				m.ParseErrors = append(m.ParseErrors, &ParseError{
					Field: "PubDate", Value: r.PubDate, Err: err,
				})
			}
		}
		msgs = append(msgs, m)
	}
	return msgs
}

// cleanMessageText trims message text and undoes the extra layer of HTML
// escaping the API applies (an ampersand arrives as "&amp;amp;").
func cleanMessageText(s string) string {
	return strings.TrimSpace(html.UnescapeString(s))
}

// parseMessageTime parses the "9/17/2026 10:32:35 AM" style timestamps used
// by station messages, which differ from the format used elsewhere in the API.
func (c *Client) parseMessageTime(ts string) (time.Time, error) {
	loc := time.UTC
	if c != nil && c.location != nil {
		loc = c.location
	}
	return time.ParseInLocation("1/2/2006 3:04:05 PM", ts, loc)
}
