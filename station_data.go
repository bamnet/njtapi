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
}

// A StationStop is a stop this train will make, or has made, on it's route.
type StationStop struct {
	Name     string    // Station stop name
	Time     time.Time // Estimated arrival time at this stop
	Departed bool      // Indicates if the train has departed the stop or not
}

// StationData returns details about upcoming trains stopping at a station.
func (c *Client) StationData(ctx context.Context, station string) (*Station, error) {
	resp, err := c.fetch(ctx, stationDataEndpoint, map[string]string{"station": station})
	if err != nil {
		return nil, err
	}

	data := struct {
		XMLName      xml.Name `xml:"STATION"`
		Station2Char string   `xml:"STATION_2CHAR"`
		StationName  string   `xml:"STATIONNAME"`
		Items        []struct {
			Index                  int    `xml:"ITEM_INDEX"`
			ScheduledDepartureDate string `xml:"SCHED_DEP_DATE"`
			Destination            string `xml:"DESTINATION"`
			Track                  string `xml:"TRACK"`
			Line                   string `xml:"LINE"`
			TrainID                string `xml:"TRAIN_ID"`
			ConnectingTrainID      string `xml:"CONNECTING_TRAIN_ID"`
			Status                 string `xml:"STATUS"`
			SecondsLate            int    `xml:"SEC_LATE"`
			LastModified           string `xml:"LAST_MODIFIED"`
			BackColor              string `xml:"BACKCOLOR"`
			ForeColor              string `xml:"FORECOLOR"`
			ShadowColor            string `xml:"SHADOWCOLOR"`
			GPSTime                string `xml:"GPSTIME"`
			LineAbbreviation       string `xml:"LINEABBREVIATION"`
			InlineMsg              string `xml:"INLINEMSG"`
			Longitude              string `xml:"GPSLONGITUDE"`
			Latitude               string `xml:"GPSLATITUDE"`
			Stops                  []struct {
				Name     string `xml:"NAME"`
				Time     string `xml:"TIME"`
				Departed string `xml:"DEPARTED"`
			} `xml:"STOPS>STOP"`
		} `xml:"ITEMS>ITEM"`
	}{}

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
		train.ScheduledDepartureDate, _ = parseTime(r.ScheduledDepartureDate)
		train.LatLngTimestamp, _ = parseTime(r.GPSTime)
		train.LatLng, _ = parseLatLng(r.Latitude, r.Longitude)

		stops := make([]StationStop, len(r.Stops))
		for j, s := range r.Stops {
			stops[j] = StationStop{Name: strings.TrimSpace(s.Name)}
			stops[j].Time, _ = parseTime(s.Time)
			stops[j].Departed = (s.Departed == "YES")
		}
		train.Stops = stops
		trains = append(trains, train)
	}

	s := &Station{ID: data.Station2Char, Name: data.StationName, Departures: trains}
	return s, nil
}

// StationList returns a list of all the stations available.
func (c *Client) StationList(ctx context.Context) ([]Station, error) {
	resp, err := c.fetch(ctx, stationListEndpoint, nil)
	if err != nil {
		return nil, err
	}

	data := struct {
		XMLName xml.Name `xml:"STATIONS"`
		Station []struct {
			Name         string `xml:"STATIONNAME"`
			Station2Char string `xml:"STATION_2CHAR"`
		} `xml:"STATION"`
	}{}

	err = xml.Unmarshal(resp, &data)
	if err != nil {
		return nil, err
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

func parseLatLng(lat, lng string) (*LatLng, error) {
	if lat != " " && lng != " " {
		lt, err := strconv.ParseFloat(lat, 64)
		if err != nil {
			return nil, err
		}
		ln, err := strconv.ParseFloat(lng, 64)
		if err != nil {
			return nil, err
		}
		return &LatLng{Lat: lt, Lng: ln}, nil
	}
	return nil, nil
}
