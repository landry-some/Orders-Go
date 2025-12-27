package grid

import (
	"testing"
	"time"

	"wayfinder/internal/courier"
)

func TestGrid_UpdateAndGet(t *testing.T) {
	grid := NewGridService()

	loc := courier.Location{
		DriverID:  "driver-123",
		Lat:       37.7749,
		Long:      -122.4194,
		Timestamp: time.Date(2024, 1, 2, 15, 4, 5, 0, time.UTC),
	}

	grid.Update(loc)

	got, ok := grid.Get(loc.DriverID)
	if !ok {
		t.Fatalf("expected location for driver %s", loc.DriverID)
	}

	if got != loc {
		t.Fatalf("got %+v, want %+v", got, loc)
	}
}
