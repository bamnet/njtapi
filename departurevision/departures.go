// Package departurevision parses an HTML table showing departure information.
//
// Consider this package highly experimental and subject to break without warning
// due to the fragile parsing logic used here.  Prefer to use the njtapi package
// wherever possible for a more stable and robust interface.
package departurevision

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bamnet/njtapi"
	"golang.org/x/net/html"
)

const userAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/75.0.3770.100 Safari/537.36"

// Client provides access to HTML-based departure information.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient creates a new client to access data from a supplied endpoint.
//
// Sample baseURL: http://dv.njtransit.com/mobile/tid-mobile.aspx.
func NewClient(baseURL string) *Client {
	return &Client{&http.Client{}, baseURL}
}

// Departures finds a simplified list of departures given a 2-character station code.
//
// Sample station: NY
//
// This datasource provides less detail than the full API. As a result,
// most of the fields in the response will not be set or may have values
// optimized for visual consumption instead of programmatic use. The
// `ScheduledDepartureDate` field, for example, will only have
// hour + minute information, not a full date and time.
// The `Destination` field often includes extra stop indicators and
// unsupported unicode symbols for the airport and Secaucus Junction.
func (c *Client) Departures(ctx context.Context, station string) ([]njtapi.StationTrain, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("sid", station)
	u.RawQuery = q.Encode()

	table, err := c.extractTable(ctx, u.String())
	if err != nil {
		return nil, err
	}

	// Map the expected column names back to IDs.
	cols := map[string]int{
		"DEP":    0,
		"TO":     -1,
		"TRK":    -1,
		"LINE":   -1,
		"TRAIN":  -1,
		"STATUS": -1,
	}

	if len(table) == 0 {
		return nil, errors.New("error parsing departurevision table")
	}

	header := table[0]
	for i, val := range header {
		for col := range cols {
			if col == val {
				cols[col] = i
			}
		}
	}

	colFound := false
	for _, idx := range cols {
		if idx > -1 {
			colFound = true
			break
		}
	}
	if !colFound {
		return nil, errors.New("unable to identify any columns in response")
	}

	results := []njtapi.StationTrain{}
	for _, row := range table[1:] {
		train := njtapi.StationTrain{}

		if len(row) != len(table[0]) {
			if len(table[0])-1 == len(row) {
				// If we are just missing 1 field, assume it is the Track and inject it.
				t := cols["TRK"]
				row = append(row, "")
				copy(row[(t+1):], row[t:])
				row[t] = ""
			} else {
				// Missing too many fields, skipping.
				continue
			}
		}

		if row[cols["TRAIN"]] != "" {
			train.TrainID, err = strconv.Atoi(row[cols["TRAIN"]])
			if err != nil {
				continue
			}
		}

		if row[cols["DEP"]] != "" {
			train.ScheduledDepartureDate, err = time.Parse("3:04", row[cols["DEP"]])
			if err != nil {
				log.Printf("Error parsing departure time: %s", row[cols["DEP"]])
			}
		}

		if row[cols["TO"]] != "" {
			train.Destination = row[cols["TO"]]

		}

		if row[cols["TRK"]] != "" {
			train.Track = row[cols["TRK"]]

		}

		if row[cols["LINE"]] != "" {
			train.Line = row[cols["LINE"]]

		}

		if row[cols["STATUS"]] != "" {
			train.Status = row[cols["STATUS"]]

		}

		results = append(results, train)
	}

	return results, nil
}

// extractTable parses out the departure table into a 2d array of strings.
func (c *Client) extractTable(ctx context.Context, url string) ([][]string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	// We need to fake a modern user agent here to get a nearly well-formatted reply.
	// Without this, we get a very old-skool output where each row is its own <table>
	// and each td has an extra <p> tag too.
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	z := html.NewTokenizer(resp.Body)
	table := [][]string{}
	row := []string{}

	for z.Token().Data != "html" {
		tt := z.Next()
		if tt == html.StartTagToken {
			t := z.Token()

			if t.Data == "tr" {
				if len(row) > 0 {
					table = append(table, row)
					row = make([]string, 0)
				}
			}

			if t.Data == "td" {
				inner := z.Next()

				if inner == html.TextToken {
					text := (string)(z.Text())
					t := strings.TrimSpace(text)
					row = append(row, t)
				}
			}

		}
	}
	if len(row) > 0 {
		table = append(table, row)
	}

	return table, nil
}
