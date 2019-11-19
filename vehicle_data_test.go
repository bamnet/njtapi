package njtapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TrainLess(t1 Train, t2 Train) bool {
	return t1.ID < t2.ID
}

func TestRemoveDupTrains(t *testing.T) {
	t1 := time.Date(2018, 1, 2, 3, 4, 0, 0, time.UTC)
	t2 := time.Date(2018, 1, 2, 3, 4, 0, 1, time.UTC)
	for _, r := range []struct {
		input []Train
		want  []Train
	}{
		{
			input: []Train{{ID: 1, LastModified: t1}},
			want:  []Train{{ID: 1, LastModified: t1}},
		}, {
			input: []Train{
				{ID: 1, LastModified: t1},
				{ID: 1, LastModified: t2},
			},
			want: []Train{{ID: 1, LastModified: t2}},
		}, {
			input: []Train{
				{ID: 1, LastModified: t1},
				{ID: 1, LastModified: t2},
				{ID: 2, LastModified: t1},
			},
			want: []Train{
				{ID: 1, LastModified: t2},
				{ID: 2, LastModified: t1},
			},
		},
	} {
		if got := removeDupTrains(r.input); !cmp.Equal(got, r.want, cmpopts.SortSlices(TrainLess)) {
			t.Errorf("removeDupTrains(%v) got %v want %v", r.input, got, r.want)
		}
	}
}

func TestVehicleData(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("Error loading timezones: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "testdata/getVehicleData.xml")
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "username", "pa$$word")
	got, err := c.VehicleData(context.Background())
	if err != nil {
		t.Errorf("VehicleData() got unexpected error: %v", err)
	}

	want := []Train{
		{
			ID:                     41,
			Line:                   "Bergen County Line",
			Direction:              "Westbound",
			LastModified:           time.Date(2019, 11, 18, 0, 0, 53, 0, loc),
			ScheduledDepartureTime: time.Date(2019, 11, 19, 0, 40, 0, 0, loc),
			NextStop:               "Hoboken",
			LatLng:                 &LatLng{Lat: 40.7347, Lng: -74.0311},
		}, {
			ID:                     65,
			Line:                   "Bergen County Line",
			Direction:              "Westbound",
			LastModified:           time.Date(2019, 11, 18, 22, 01, 18, 0, loc),
			ScheduledDepartureTime: time.Date(2019, 11, 18, 22, 8, 0, 0, loc),
			SecondsLate:            310 * time.Second,
			NextStop:               "Port Jervis",
			LatLng:                 &LatLng{Lat: 41.374876, Lng: -74.694672},
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("VehicleData() mismatch (-want +got):\n%s", diff)
	}
}
