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
	train.LastModified, _ = parseTime(t.LastModified)

	t.Longitude = strings.TrimSpace(t.Longitude)
	t.Latitude = strings.TrimSpace(t.Latitude)
	if t.Longitude != "" && t.Latitude != "" {
		lat, _ := strconv.ParseFloat(t.Latitude, 64)
		lng, _ := strconv.ParseFloat(t.Longitude, 64)
		train.LatLng = &LatLng{lat, lng}
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
	train.LastModified, _ = parseTime(data.GPSTime)

	data.Longitude = strings.TrimSpace(data.Longitude)
	data.Latitude = strings.TrimSpace(data.Latitude)
	if data.Longitude != "" && data.Latitude != "" {
		lat, _ := strconv.ParseFloat(data.Latitude, 64)
		lng, _ := strconv.ParseFloat(data.Longitude, 64)
		train.LatLng = &LatLng{lat, lng}
	}

	for _, s := range data.Stops {
		stop := StationStop{
			Name:     s.Name,
			Departed: (s.Departed == "YES"),
			Status:   s.Status,
		}
		stop.Time, _ = parseTime(s.Time)
		stop.DepartureTime, _ = parseTime(s.DepartureTime)

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

	trains := make([]Train, len(data.Trains))
	for i, d := range data.Trains {
		// Use a Regex to extract the numbers from the ID.
		// Some trains have an "a" (amtrak suffix). Others
		// randomly have leading or trailing ".".
		//
		// https://github.com/bamnet/njtapi/issues/7
		idM := trainIDRe.FindStringSubmatch(d.ID)
		d.ID = idM[1]

		id, err := strconv.Atoi(d.ID)
		if err != nil {
			return nil, err
		}

		// Sometimes the lat lng fields contain " ".
		lat, _ := parseDegrees(d.Latitude)
		lng, _ := parseDegrees(d.Longitude)

		trains[i] = Train{
			ID:           id,
			Line:         d.Line,
			Direction:    d.Direction,
			SecondsLate:  time.Duration(d.SecondsLate) * time.Second,
			NextStop:     strings.TrimSpace(d.NextStop),
			LatLng:       &LatLng{lat, lng},
			TrackCircuit: strings.TrimSpace(d.TrackCircuit),
		}
		trains[i].LastModified, _ = parseTime(d.LastModified)
		trains[i].ScheduledDepartureTime, _ = parseTime(d.ScheduledDepartureTime)
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
