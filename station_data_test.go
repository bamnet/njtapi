package njtapi

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func ExampleClient_StationData() {
	// Update these values.
	username := "your username"
	password := "your password"

	client := NewClient("http://njttraindata_tst.njtransit.com:8090/njttraindata.asmx/", username, password)
	station, err := client.StationData(context.Background(), "NY")
	if err != nil {
		log.Fatalf("StationData() error: %v", err)
	}
	for _, departures := range station.Departures {
		fmt.Printf("Train to %s at %s", departures.Destination, departures.ScheduledDepartureDate)
	}
}

func TestStationList(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "testdata/getStationList.xml")
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "username", "pa$$word")

	got, err := c.StationList(context.Background())
	if err != nil {
		t.Errorf("StationList() got unexpected error: %v", err)
	}

	want := []Station{
		{ID: "NY", Name: "New York", Aliases: []string{"New York Penn Station"}},
		{ID: "NP", Name: "Newark Penn", Aliases: []string{"Newark Penn Station"}},
		{ID: "SE", Name: "Secaucus", Aliases: []string{"Secaucus Upper Lvl"}},
		{ID: "TS", Name: "Secaucus", Aliases: []string{"Secaucus Lower Lvl"}},
		{ID: "WL", Name: "Woodcliff Lake"},
		{ID: "SC", Name: "\n    "},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("StationList() mismatch (-want +got):\n%s", diff)
	}
}

func TestStationData(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("Error loading timezones: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("Error parsing request: %v", err)
		}

		if u, p := r.Form.Get("username"), r.Form.Get("password"); u != "username" || p != "pa$$word" {
			t.Errorf("Missing expected username & password: %v", r.Form)
		}
		s := r.Form.Get("station")
		switch s {
		case "SE":
			http.ServeFile(w, r, "testdata/getTrainSchedule1.xml")
		case "NY":
			http.ServeFile(w, r, "testdata/getTrainSchedule2.xml")
		}

	}))
	defer ts.Close()

	c := NewClient(ts.URL, "username", "pa$$word")

	for _, r := range []struct {
		station string
		want    *Station
	}{
		{
			station: "SE",
			want: &Station{
				ID:   "SE",
				Name: "Secaucus",
				Departures: []StationTrain{
					{
						Index:                  0,
						TrainID:                3883,
						Line:                   "Northeast Corridor Line",
						LineAbbrv:              "NEC",
						Destination:            "Trenton &#9992",
						ScheduledDepartureDate: time.Date(2019, 11, 18, 20, 17, 0, 0, loc),
						Track:                  "B",
						Status:                 "in 4 Min",
						SecondsLate:            4 * time.Minute,
						LatLng:                 &LatLng{Lat: 40.7706, Lng: -74.0403},
						LatLngTimestamp:        time.Date(2019, 11, 18, 20, 16, 45, 0, loc),
						InlineMsg:              "\n      ",
						Stops: []StationStop{
							{Name: "New York Penn Station", Time: time.Date(2019, 11, 18, 20, 7, 0, 0, loc), Departed: true},
							{Name: "Secaucus Upper Lvl", Time: time.Date(2019, 11, 18, 20, 20, 30, 0, loc), Departed: false},
							{Name: "Newark Penn Station", Time: time.Date(2019, 11, 18, 20, 28, 45, 0, loc), Departed: false},
							{Name: "Newark Airport", Time: time.Date(2019, 11, 18, 20, 35, 0, 0, loc), Departed: false},
							{Name: "North Elizabeth", Time: time.Date(2019, 11, 18, 20, 38, 45, 0, loc), Departed: false},
							{Name: "Elizabeth", Time: time.Date(2019, 11, 18, 20, 41, 30, 0, loc), Departed: false},
							{Name: "Linden", Time: time.Date(2019, 11, 18, 20, 46, 45, 0, loc), Departed: false},
							{Name: "Rahway", Time: time.Date(2019, 11, 18, 20, 51, 00, 0, loc), Departed: false},
							{Name: "Metropark", Time: time.Date(2019, 11, 18, 20, 59, 45, 0, loc), Departed: false},
							{Name: "Metuchen", Time: time.Date(2019, 11, 18, 21, 04, 15, 0, loc), Departed: false},
							{Name: "Edison", Time: time.Date(2019, 11, 18, 21, 9, 15, 0, loc), Departed: false},
							{Name: "New Brunswick", Time: time.Date(2019, 11, 18, 21, 13, 30, 0, loc), Departed: false},
							{Name: "Jersey Avenue", Time: time.Date(2019, 11, 18, 21, 18, 15, 0, loc), Departed: false},
							{Name: "Princeton Junction", Time: time.Date(2019, 11, 18, 21, 30, 45, 0, loc), Departed: false},
							{Name: "Hamilton", Time: time.Date(2019, 11, 18, 21, 37, 15, 0, loc), Departed: false},
							{Name: "Trenton", Time: time.Date(2019, 11, 18, 21, 50, 15, 0, loc), Departed: false},
						},
					}, {
						Index:                  1,
						TrainID:                3283,
						Line:                   "North Jersey Coast Line",
						LineAbbrv:              "NJCL",
						Destination:            "Long Branch-BH &#9992",
						ScheduledDepartureDate: time.Date(2019, 11, 18, 20, 31, 30, 0, loc),
						Track:                  "B",
						LatLngTimestamp:        time.Date(2019, 11, 18, 20, 05, 33, 0, loc),
						InlineMsg:              "\n      ",
						Stops: []StationStop{
							{Name: "New York Penn Station", Time: time.Date(2019, 11, 18, 20, 22, 0, 0, loc), Departed: false},
							{Name: "Secaucus Upper Lvl", Time: time.Date(2019, 11, 18, 20, 31, 0, 0, loc), Departed: false},
						},
					},
				},
			},
		}, {
			station: "NY",
			want: &Station{
				ID:   "NY",
				Name: "New York",
				Departures: []StationTrain{
					// The firsr train, index 0, is skipped because it's an Amtrak train.
					{
						Index:                  1,
						TrainID:                3283,
						Line:                   "North Jersey Coast Line",
						LineAbbrv:              "NJCL",
						Destination:            "Long Branch-BH -SEC &#9992",
						ScheduledDepartureDate: time.Date(2019, 11, 18, 20, 22, 0, 0, loc),
						Track:                  "7",
						Status:                 "BOARDING",
						SecondsLate:            -1 * time.Minute,
						LatLngTimestamp:        time.Date(2019, 11, 18, 20, 05, 33, 0, loc),
						InlineMsg:              "\n      ",
						Stops: []StationStop{
							{Name: "New York Penn Station", Time: time.Date(2019, 11, 18, 20, 22, 0, 0, loc), Departed: false},
							{Name: "Secaucus Upper Lvl", Time: time.Date(2019, 11, 18, 20, 31, 0, 0, loc), Departed: false},
						},
					},
				},
			},
		},
	} {
		got, err := c.StationData(context.Background(), r.station)
		if err != nil {
			t.Errorf("StationData(%s) unexpected error: %v", r.station, err)
		}
		if diff := cmp.Diff(r.want, got); diff != "" {
			t.Errorf("StationData(%s) mismatch (-want +got):\n%s", r.station, diff)
		}
	}
}
