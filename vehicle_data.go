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

// trainMapResponse mirrors the XML shape returned by the getTrainMapXML endpoint.
type trainMapResponse struct {
	XMLName xml.Name        `xml:"Trains"`
	Trains  []trainMapEntry `xml:"Train"`
}

// trainMapEntry is a single train entry within a trainMapResponse.
type trainMapEntry struct {
	ID           string `xml:"Train_ID"`
	Line         string `xml:"TrainLine"`
	Direction    string `xml:"DIRECTION"`
	LastModified string `xml:"LAST_MODIFIED"`
	Longitude    string `xml:"longitude"`
	Latitude     string `xml:"latitude"`
	TrackCircuit string `xml:"TrackCKT"`
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

	data := trainMapResponse{}

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
	train.LastModified, err = c.parseTime(t.LastModified)
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

// trainStopsResponse mirrors the XML shape returned by the getTrainStopListXML endpoint.
type trainStopsResponse struct {
	XMLName     xml.Name         `xml:"Train"`
	ID          string           `xml:"Train_ID"`
	Destination string           `xml:"DESTINATION"`
	GPSTime     string           `xml:"GPSTIME"`
	Longitude   string           `xml:"GPSLONGITUDE"`
	Latitude    string           `xml:"GPSLATITUDE"`
	Stops       []trainStopsStop `xml:"STOPS>STOP"`
}

// trainStopsStop is a single stop entry within a trainStopsResponse.
type trainStopsStop struct {
	Name          string           `xml:"NAME"`
	Station2Char  string           `xml:"STATION_2CHAR"`
	Time          string           `xml:"TIME"`
	Departed      string           `xml:"DEPARTED"`
	Status        string           `xml:"STOP_STATUS"`
	DepartureTime string           `xml:"DEP_TIME"`
	Lines         []trainStopsLine `xml:"STOP_LINES>STOP_LINE"`
}

// trainStopsLine is a single connecting line entry within a trainStopsStop.
type trainStopsLine struct {
	Code string `xml:"LINE_CODE"`
	Name string `xml:"LINE_NAME"`
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

	data := trainStopsResponse{}

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
	train.LastModified, err = c.parseTime(data.GPSTime)
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
		stop.Time, err = c.parseTime(s.Time)
		if err != nil {
			stop.ParseErrors = append(stop.ParseErrors, &ParseError{
				Field: "Time", Value: s.Time, Err: err,
			})
		}
		stop.DepartureTime, err = c.parseTime(s.DepartureTime)
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

// vehicleDataResponse mirrors the XML shape returned by the getVehicleDataXML endpoint.
type vehicleDataResponse struct {
	XMLName xml.Name           `xml:"TRAINS"`
	Trains  []vehicleDataEntry `xml:"TRAIN"`
}

// vehicleDataEntry is a single train entry within a vehicleDataResponse.
type vehicleDataEntry struct {
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
}

// VehicleData returns up the most recent information about all "active" trains.
func (c *Client) VehicleData(ctx context.Context) ([]Train, error) {
	resp, err := c.fetch(ctx, vehicleDataEndpoint, nil)
	if err != nil {
		return nil, err
	}

	data := vehicleDataResponse{}

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
		t.LastModified, err = c.parseTime(d.LastModified)
		if err != nil {
			parseErrs = append(parseErrs, &ParseError{
				Field: "LAST_MODIFIED", Value: d.LastModified, Err: err,
			})
		}
		t.ScheduledDepartureTime, err = c.parseTime(d.ScheduledDepartureTime)
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
// The order of the returned trains matches the order in which each train ID
// first appears in the input.
func removeDupTrains(trains []Train) []Train {
	seen := make(map[int]int) // trainID -> index in result
	result := make([]Train, 0, len(trains))
	for _, t := range trains {
		if idx, ok := seen[t.ID]; ok {
			if result[idx].LastModified.Before(t.LastModified) {
				result[idx] = t
			}
		} else {
			seen[t.ID] = len(result)
			result = append(result, t)
		}
	}
	return result
}
