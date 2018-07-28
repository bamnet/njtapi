package njtapi

import (
	"encoding/xml"
	"strconv"
	"time"
)

const stationDataEndpoint = "getTrainScheduleXML"

// A station provides information about the next trains stopping at a station.
type Station struct {
	ID         string         // Station character code
	Name       string         // Station name
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
	SecondsLate            int           // Train delay in seconds
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
func (c *Client) StationData(station string) (*Station, error) {
	resp, err := c.fetch(stationDataEndpoint, map[string]string{"station": station})
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
			TrainID                int    `xml:"TRAIN_ID"`
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

	trains := make([]StationTrain, len(data.Items))
	for i, r := range data.Items {
		trains[i] = StationTrain{
			Index:       r.Index,
			Destination: r.Destination,
			Track:       r.Track,
			Line:        r.Line,
			TrainID:     r.TrainID,
			Status:      r.Status,
			SecondsLate: r.SecondsLate,
			LineAbbrv:   r.LineAbbreviation,
			InlineMsg:   r.InlineMsg,
		}
		trains[i].ScheduledDepartureDate, _ = parseTime(r.ScheduledDepartureDate)
		trains[i].LatLngTimestamp, _ = parseTime(r.GPSTime)
		trains[i].LatLng, _ = parseLatLng(r.Latitude, r.Longitude)

		stops := make([]StationStop, len(r.Stops))
		for j, s := range r.Stops {
			stops[j] = StationStop{Name: s.Name}
			stops[j].Time, _ = parseTime(s.Time)
			stops[j].Departed = (s.Departed == "YES")
		}
		trains[i].Stops = stops
	}

	s := &Station{ID: data.Station2Char, Name: data.StationName, Departures: trains}
	return s, nil
}

func parseLatLng(lat, lng string) (*LatLng, error) {
	if lat != " " && lng != " " {
		lt, err := strconv.ParseFloat(lat, 64)
		ln, err := strconv.ParseFloat(lng, 64)
		if err != nil {
			return nil, err
		}
		return &LatLng{Lat: lt, Lng: ln}, nil
	}
	return nil, nil
}
