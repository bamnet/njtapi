package njtapi

import (
	"context"
	"encoding/xml"
	"strconv"
	"strings"
	"time"
)

const (
	stationDataEndpoint = "getTrainScheduleXML"
	stationListEndpoint = "getStationListXML"
)

// A Station provides information about the next trains stopping at a station.
type Station struct {
	ID         string         // Station character code
	Name       string         // Station name
	Aliases    []string       // Additional names for this station
	Departures []StationTrain // Trains departing from this station

	// Messages are station-level banner messages (service advisories)
	// shown alongside the departures. Only populated by StationData.
	Messages []StationMessage
}

// A StationTrain models a train which is scheduled to depart from a station.
type StationTrain struct {
	Index                  int           // Row index
	TrainID                int           // Train ID
	Line                   string        // Train line
	LineAbbrv              string        // Train line abbreviation
	Destination            string        // Destination for the train
	ScheduledDepartureDate time.Time     // Scheduled departure time from the station
	Track                  string        // Track number/letter
	Status                 string        // Current train status
	SecondsLate            time.Duration // Train delay
	LatLng                 *LatLng       // Train location
	LatLngTimestamp        time.Time     // Time the train location was measured
	InlineMsg              string        // In-line message for the train at this station
	Stops                  []StationStop // List of all stops for this train.
	ParseErrors            []error       // Errors encountered while parsing this train
}

// A StationStop is a stop this train will make, or has made, on it's route.
type StationStop struct {
	Name          string    // Station stop name
	Time          time.Time // Actual (if already left) or planned (if upcoming) departure time from this stop
	Departed      bool      // Indicates if the train has departed the stop or not
	DepartureTime time.Time // Time the train was intially scheduled to depart this station
	Lines         []Line    // Connecting lines available at this station
	Status        string    // Status of the train at this stop, e.g. "OnTime", "Late", "BOARDING", "2 HOURS LATE"; often empty
	ParseErrors   []error   // Errors encountered while parsing this stop
}

// A Line is train line, like the North Jersey Coast Line.
type Line struct {
	Name string // Train line
}

// stationDataResponse mirrors the XML shape returned by the getTrainScheduleXML endpoint.
type stationDataResponse struct {
	XMLName      xml.Name          `xml:"STATION"`
	Station2Char string            `xml:"STATION_2CHAR"`
	StationName  string            `xml:"STATIONNAME"`
	Messages     []stationMessage  `xml:"BANNERMSGS>MSG"`
	Items        []stationDataItem `xml:"ITEMS>ITEM"`
}

// stationDataItem is a single train entry within a stationDataResponse.
type stationDataItem struct {
	Index                  int               `xml:"ITEM_INDEX"`
	ScheduledDepartureDate string            `xml:"SCHED_DEP_DATE"`
	Destination            string            `xml:"DESTINATION"`
	Track                  string            `xml:"TRACK"`
	Line                   string            `xml:"LINE"`
	TrainID                string            `xml:"TRAIN_ID"`
	ConnectingTrainID      string            `xml:"CONNECTING_TRAIN_ID"`
	Status                 string            `xml:"STATUS"`
	SecondsLate            int               `xml:"SEC_LATE"`
	LastModified           string            `xml:"LAST_MODIFIED"`
	BackColor              string            `xml:"BACKCOLOR"`
	ForeColor              string            `xml:"FORECOLOR"`
	ShadowColor            string            `xml:"SHADOWCOLOR"`
	GPSTime                string            `xml:"GPSTIME"`
	LineAbbreviation       string            `xml:"LINEABBREVIATION"`
	InlineMsg              string            `xml:"INLINEMSG"`
	Longitude              string            `xml:"GPSLONGITUDE"`
	Latitude               string            `xml:"GPSLATITUDE"`
	Stops                  []stationDataStop `xml:"STOPS>STOP"`
}

// stationDataStop is a single stop entry within a stationDataItem.
type stationDataStop struct {
	Name     string `xml:"NAME"`
	Time     string `xml:"TIME"`
	Departed string `xml:"DEPARTED"`
	Status   string `xml:"STOP_STATUS"`
}

// StationData returns details about upcoming trains stopping at a station.
func (c *Client) StationData(ctx context.Context, station string) (*Station, error) {
	resp, err := c.fetch(ctx, stationDataEndpoint, map[string]string{"station": station})
	if err != nil {
		return nil, err
	}

	data := stationDataResponse{}

	err = xml.Unmarshal(resp, &data)
	if err != nil {
		return nil, err
	}

	trains := []StationTrain{}
	for _, r := range data.Items {
		tID, err := strconv.Atoi(r.TrainID)
		if err != nil {
			// Skip trains that don't have a numeric ID.
			// These are Amtrak trains with "A123" style IDs.
			continue
		}
		train := StationTrain{
			Index:       r.Index,
			Destination: r.Destination,
			Track:       strings.TrimSpace(r.Track),
			Line:        r.Line,
			TrainID:     tID,
			Status:      strings.TrimSpace(r.Status),
			SecondsLate: time.Duration(r.SecondsLate) * time.Second,
			LineAbbrv:   r.LineAbbreviation,
			InlineMsg:   strings.TrimSpace(r.InlineMsg),
		}
		train.ScheduledDepartureDate, err = c.parseTime(r.ScheduledDepartureDate)
		if err != nil {
			train.ParseErrors = append(train.ParseErrors, &ParseError{
				Field: "SCHED_DEP_DATE", Value: r.ScheduledDepartureDate, Err: err,
			})
		}
		train.LatLngTimestamp, err = c.parseTime(r.GPSTime)
		if err != nil {
			train.ParseErrors = append(train.ParseErrors, &ParseError{
				Field: "GPSTime", Value: r.GPSTime, Err: err,
			})
		}
		train.LatLng, err = parseLatLng(r.Latitude, r.Longitude)
		if err != nil {
			train.ParseErrors = append(train.ParseErrors, &ParseError{
				Field: "LatLng", Value: r.Latitude + "," + r.Longitude, Err: err,
			})
		}

		stops := make([]StationStop, len(r.Stops))
		for j, s := range r.Stops {
			stops[j] = StationStop{Name: strings.TrimSpace(s.Name)}
			stops[j].Time, err = c.parseTime(s.Time)
			if err != nil {
				stops[j].ParseErrors = append(stops[j].ParseErrors, &ParseError{
					Field: "Time", Value: s.Time, Err: err,
				})
			}
			stops[j].Departed = (s.Departed == "YES")
			stops[j].Status = strings.TrimSpace(s.Status)
		}
		train.Stops = stops
		trains = append(trains, train)
	}

	s := &Station{
		ID:         data.Station2Char,
		Name:       data.StationName,
		Departures: trains,
		Messages:   c.parseStationMessages(data.Messages),
	}
	return s, nil
}

// stationListResponse mirrors the XML shape returned by the getStationListXML endpoint.
type stationListResponse struct {
	XMLName xml.Name           `xml:"STATIONS"`
	Station []stationListEntry `xml:"STATION"`
}

// stationListEntry is a single station entry within a stationListResponse.
type stationListEntry struct {
	Name         string `xml:"STATIONNAME"`
	Station2Char string `xml:"STATION_2CHAR"`
}

// StationList returns a list of all the stations available.
func (c *Client) StationList(ctx context.Context) ([]Station, error) {
	resp, err := c.fetch(ctx, stationListEndpoint, nil)
	if err != nil {
		return nil, err
	}

	data := stationListResponse{}

	err = xml.Unmarshal(resp, &data)
	if err != nil {
		return nil, err
	}

	// extraStations provides aliases for stations that have multiple names
	// depending on which endpoint you are invoking and how it's feeling.
	// It's declared locally (rather than as a package-level variable) so it
	// can't be mutated at runtime by other code in the package.
	extraStations := map[string][]string{
		"AM": {"Aberdeen-Matawan"},
		"CN": {"Convent Station"},
		"HI": {"Highland Avenue"},
		"JA": {"Jersey Avenue"},
		"HS": {"Montclair Heights"},
		"UV": {"Montclair State U"},
		"MS": {"Mountain Avenue"},
		"MT": {"Mountain Station"},
		"NY": {"New York Penn Station"},
		"ND": {"Newark Broad Street"},
		"NP": {"Newark Penn Station"},
		"NZ": {"North Elizabeth"},
		"PP": {"Point Pleasant Beach"},
		"PJ": {"Princeton Junction"},
		"SE": {"Secaucus Upper Lvl"},
		"TS": {"Secaucus Lower Lvl"},
		"UM": {"Upper Montclair"},
		"WG": {"Watchung Avenue"},
		"WT": {"Watsessing Avenue"},
	}

	stations := []Station{}
	for _, r := range data.Station {
		stations = append(stations, Station{
			Name:    strings.TrimSpace(r.Name),
			ID:      r.Station2Char,
			Aliases: extraStations[r.Station2Char],
		})
	}
	return stations, nil
}
