package njtapi

import (
	"encoding/xml"
	"time"
)

const vehicleDataEndpoint = "getVehicleDataXML"

// A train summarizes the latest information about a train.
type Train struct {
	ID                     int       // Train number
	Line                   string    // Train line
	Direction              string    // Eastbound or Westbound
	LastModified           time.Time // ???
	ScheduledDepartureTime time.Time // ???
	SecondsLate            int       // Train delay in seconds
	NextStop               string    // ???
	LatLng                 *LatLng   // Last identified latlng
}

// VehicleData returns up the most recent information about trains.
func (c *Client) VehicleData() ([]Train, error) {
	resp, err := c.fetch(vehicleDataEndpoint, nil)
	if err != nil {
		return nil, err
	}

	data := struct {
		XMLName xml.Name `xml:"TRAINS"`
		Trains  []struct {
			ID                     int     `xml:"ID"`
			Line                   string  `xml:"TRAIN_LINE"`
			Direction              string  `xml:"DIRECTION"`
			LastModified           string  `xml:"LAST_MODIFIED"`
			ScheduledDepartureTime string  `xml:"SCHED_DEP_TIME"`
			SecondsLate            int     `xml:"SEC_LATE"`
			NextStop               string  `xml:"NEXT_STOP"`
			Longitude              float64 `xml:"LONGITUDE"`
			Latitude               float64 `xml:"LATITUDE"`
		} `xml:"TRAIN"`
	}{}

	err = xml.Unmarshal(resp, &data)
	if err != nil {
		return nil, err
	}

	trains := make([]Train, len(data.Trains))
	for i, d := range data.Trains {
		trains[i] = Train{
			ID:          d.ID,
			Line:        d.Line,
			Direction:   d.Direction,
			SecondsLate: d.SecondsLate,
			NextStop:    d.NextStop,
			LatLng:      &LatLng{d.Latitude, d.Longitude},
		}
		trains[i].LastModified, _ = parseTime(d.LastModified)
		trains[i].ScheduledDepartureTime, _ = parseTime(d.ScheduledDepartureTime)
	}
	return trains, nil
}
