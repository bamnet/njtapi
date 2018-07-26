// Package njtapi provides an API to access NJTransit data.
package njtapi

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	username   string
	password   string
}

func NewClient(baseURL, username, password string) *Client {
	return &Client{&http.Client{}, baseURL, username, password}
}

type LatLng struct {
	Lat float64 // Latitude
	Lng float64 // Longitude
}

type Train struct {
	ID                     int    // Train number
	Line                   string // Train line
	Direction              string // Eastbound or Westbound
	LastModified           string
	ScheduledDepartureTime string
	SecondsLate            int // Train delay in seconds
	NextStop               string
	LatLng                 LatLng // Last identified latlng
}

const vehicleData = "getVehicleDataXML"

func (c *Client) VehicleData() error {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return err
	}

	u.Path = u.Path + vehicleData
	q := u.Query()
	q.Set("username", c.username)
	q.Set("password", c.password)
	u.RawQuery = q.Encode()

	resp, err := c.httpClient.Get(u.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
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

	err = xml.Unmarshal(body, &data)
	fmt.Printf("%+v", data.Trains[0])
	return err
}
