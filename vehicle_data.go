package njtapi

import (
	"context"
	"encoding/xml"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	vehicleDataEndpoint = "getVehicleDataXML"
	trainMapEndpoint    = "getTrainMapXML"
	trainStopsEndpoint  = "getTrainStopListXML"
)

var (
	ErrTrainNotFound = errors.New("train not found")

	trainIDRe = regexp.MustCompile(`(\d+)`)
)

// A Train summarizes the latest information about a train.
type Train struct {
	ID                     int           // Train number
	Line                   string        // Train line
	Direction              string        // Eastbound or Westbound
	LastModified           time.Time     // ???
	ScheduledDepartureTime time.Time     // ???
	SecondsLate            time.Duration // Train delay
	NextStop               string        // Next station the train is stopping at, like "New York" or "Dover".
	LatLng                 *LatLng       // Last identified latlng
	TrackCircuit           string        // Track Circuit ID, like "CL-2WAK" or "BC-8251TK".
	Stops                  []StationStop // Stations the train stops at.
	ParseErrors            []error       // Errors encountered while parsing this train
}

// Get information about a specific train from the "Map" API endpoint.
//
// The `Train` object returned will not have all the fields set. It will
// typically only have `ID`, `Line`, `Direction`, `LastModified`, `LatLng`,
// and `TrackCircuit`.
func (c *Client) GetTrainMap(ctx context.Context, trainID int) (*Train, error) {
	resp, err := c.fetch(ctx, trainMapEndpoint, map[string]string{"trainID": strconv.Itoa(trainID), "station": "-"})
	if err != nil {
		return nil, err
	}

	data := struct {
		XMLName xml.Name `xml:"Trains"`
		Trains  []struct {
			ID           string `xml:"Train_ID"`
			Line         string `xml:"TrainLine"`
			Direction    string `xml:"DIRECTION"`
			LastModified string `xml:"LAST_MODIFIED"`
			Longitude    string `xml:"longitude"`
			Latitude     string `xml:"latitude"`
			TrackCircuit string `xml:"TrackCKT"`
		} `xml:"Train"`
	}{}

	err = xml.Unmarshal(resp, &data)
	if err != nil {
		return nil, err
	}

	// There is always 1 train returned, even when it doesn't exist.
	// We use 'Direction' and 'Line' as good signals for a real train.
	t := data.Trains[0]
	if t.Direction == "" && t.Line == "" {
		return nil, ErrTrainNotFound
	}

	train := Train{
		ID:           trainID,
		Line:         t.Line,
		Direction:    t.Direction,
		TrackCircuit: t.TrackCircuit,
	}
	train.LastModified, err = parseTime(t.LastModified)
	if err != nil {
		train.ParseErrors = append(train.ParseErrors, &ParseError{
			Field: "LAST_MODIFIED", Value: t.LastModified, Err: err,
		})
	}

	train.LatLng, err = parseLatLng(t.Latitude, t.Longitude)
	if err != nil {
		train.ParseErrors = append(train.ParseErrors, &ParseError{
			Field: "LatLng", Value: t.Latitude + "," + t.Longitude, Err: err,
		})
	}

	return &train, nil
}

// Get information about a specific train from the "Stops" API endpoint.
//
// The `Train` object returned will not have all the fields set. It will
// typically only have `ID`, `LastModified`, `LatLng`, and `Stops`.
func (c *Client) GetTrainStops(ctx context.Context, trainID int) (*Train, error) {
	resp, err := c.fetch(ctx, trainStopsEndpoint, map[string]string{"trainID": strconv.Itoa(trainID)})
	if err != nil {
		return nil, err
	}

	data := struct {
		XMLName     xml.Name `xml:"Train"`
		ID          string   `xml:"Train_ID"`
		Destination string   `xml:"DESTINATION"`
		GPSTime     string   `xml:"GPSTIME"`
		Longitude   string   `xml:"GPSLONGITUDE"`
		Latitude    string   `xml:"GPSLATITUDE"`
		Stops       []struct {
			Name          string `xml:"NAME"`
			Station2Char  string `xml:"STATION_2CHAR"`
			Time          string `xml:"TIME"`
			Departed      string `xml:"DEPARTED"`
			Status        string `xml:"STOP_STATUS"`
			DepartureTime string `xml:"DEP_TIME"`
			Lines         []struct {
				Code string `xml:"LINE_CODE"`
				Name string `xml:"LINE_NAME"`
			} `xml:"STOP_LINES>STOP_LINE"`
		} `xml:"STOPS>STOP"`
	}{}

	err = xml.Unmarshal(resp, &data)
	if err != nil {
		return nil, err
	}

	if data.ID == "" {
		return nil, ErrTrainNotFound
	}

	train := Train{
		ID:    trainID,
		Stops: []StationStop{},
	}
	train.LastModified, err = parseTime(data.GPSTime)
	if err != nil {
		train.ParseErrors = append(train.ParseErrors, &ParseError{
			Field: "GPSTIME", Value: data.GPSTime, Err: err,
		})
	}

	train.LatLng, err = parseLatLng(data.Latitude, data.Longitude)
	if err != nil {
		train.ParseErrors = append(train.ParseErrors, &ParseError{
			Field: "LatLng", Value: data.Latitude + "," + data.Longitude, Err: err,
		})
	}

	for _, s := range data.Stops {
		stop := StationStop{
			Name:     s.Name,
			Departed: (s.Departed == "YES"),
			Status:   s.Status,
		}
		stop.Time, err = parseTime(s.Time)
		if err != nil {
			stop.ParseErrors = append(stop.ParseErrors, &ParseError{
				Field: "Time", Value: s.Time, Err: err,
			})
		}
		stop.DepartureTime, err = parseTime(s.DepartureTime)
		if err != nil {
			stop.ParseErrors = append(stop.ParseErrors, &ParseError{
				Field: "DEP_TIME", Value: s.DepartureTime, Err: err,
			})
		}

		if len(s.Lines) > 0 {
			stop.Lines = make([]Line, len(s.Lines))
			for i, l := range s.Lines {
				stop.Lines[i] = Line{Name: l.Name}
			}
		}
		train.Stops = append(train.Stops, stop)
	}

	return &train, nil
}

// VehicleData returns up the most recent information about all "active" trains.
func (c *Client) VehicleData(ctx context.Context) ([]Train, error) {
	resp, err := c.fetch(ctx, vehicleDataEndpoint, nil)
	if err != nil {
		return nil, err
	}

	data := struct {
		XMLName xml.Name `xml:"TRAINS"`
		Trains  []struct {
			ID                     string `xml:"ID"`
			Line                   string `xml:"TRAIN_LINE"`
			Direction              string `xml:"DIRECTION"`
			LastModified           string `xml:"LAST_MODIFIED"`
			ScheduledDepartureTime string `xml:"SCHED_DEP_TIME"`
			SecondsLate            int    `xml:"SEC_LATE"`
			NextStop               string `xml:"NEXT_STOP"`
			Longitude              string `xml:"LONGITUDE"`
			Latitude               string `xml:"LATITUDE"`
			TrackCircuit           string `xml:"ICS_TRACK_CKT"`
		} `xml:"TRAIN"`
	}{}

	err = xml.Unmarshal(resp, &data)
	if err != nil {
		return nil, err
	}

	trains := make([]Train, 0, len(data.Trains))
	for _, d := range data.Trains {
		// Use a Regex to extract the numbers from the ID.
		// Some trains have an "a" (amtrak suffix). Others
		// randomly have leading or trailing ".".
		//
		// https://github.com/bamnet/njtapi/issues/7
		idM := trainIDRe.FindStringSubmatch(d.ID)
		if len(idM) < 2 {
			continue
		}
		d.ID = idM[1]

		id, err := strconv.Atoi(d.ID)
		if err != nil {
			return nil, err
		}

		latlng, err := parseLatLng(d.Latitude, d.Longitude)

		var parseErrs []error
		if err != nil {
			parseErrs = append(parseErrs, &ParseError{
				Field: "LatLng", Value: d.Latitude + "," + d.Longitude, Err: err,
			})
		}

		t := Train{
			ID:           id,
			Line:         d.Line,
			Direction:    d.Direction,
			SecondsLate:  time.Duration(d.SecondsLate) * time.Second,
			NextStop:     strings.TrimSpace(d.NextStop),
			LatLng:       latlng,
			TrackCircuit: strings.TrimSpace(d.TrackCircuit),
		}
		t.LastModified, err = parseTime(d.LastModified)
		if err != nil {
			parseErrs = append(parseErrs, &ParseError{
				Field: "LAST_MODIFIED", Value: d.LastModified, Err: err,
			})
		}
		t.ScheduledDepartureTime, err = parseTime(d.ScheduledDepartureTime)
		if err != nil {
			parseErrs = append(parseErrs, &ParseError{
				Field: "SCHED_DEP_TIME", Value: d.ScheduledDepartureTime, Err: err,
			})
		}
		t.ParseErrors = parseErrs
		trains = append(trains, t)
	}
	return removeDupTrains(trains), nil
}

// removeDupTrains ensures there is only 1 train per ID in the array.
// If duplicates are found, the train with the most recent LastModified time is kept.
func removeDupTrains(trains []Train) []Train {
	ts := map[int]Train{}

	for _, t := range trains {
		if val, ok := ts[t.ID]; !ok || val.LastModified.Before(t.LastModified) {
			ts[t.ID] = t
		}
	}

	if len(ts) == len(trains) {
		return trains
	}

	unique := []Train{}
	for _, t := range ts {
		unique = append(unique, t)
	}
	return unique
}
