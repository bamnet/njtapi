package njtapi

import (
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
