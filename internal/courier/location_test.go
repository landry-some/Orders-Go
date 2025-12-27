package courier

import (
	"testing"
	"time"
)

func TestLocationCanBeInstantiated(t *testing.T) {
	driverID := "driver-123"
	lat := 37.7749
	long := -122.4194
	timestamp := time.Date(2024, 1, 2, 15, 4, 5, 0, time.UTC)

	loc := Location{
		DriverID:  driverID,
		Lat:       lat,
		Long:      long,
		Timestamp: timestamp,
	}

	if loc.DriverID != driverID {
		t.Fatalf("DriverID mismatch: got %s, want %s", loc.DriverID, driverID)
	}

	if loc.Lat != lat {
		t.Fatalf("Lat mismatch: got %f, want %f", loc.Lat, lat)
	}

	if loc.Long != long {
		t.Fatalf("Long mismatch: got %f, want %f", loc.Long, long)
	}

	if !loc.Timestamp.Equal(timestamp) {
		t.Fatalf("Timestamp mismatch: got %s, want %s", loc.Timestamp, timestamp)
	}
}

func TestNewLocationValidation(t *testing.T) {
	timestamp := time.Date(2024, 1, 2, 15, 4, 5, 0, time.UTC)

	cases := []struct {
		name      string
		driverID  string
		lat       float64
		long      float64
		timestamp time.Time
		wantErr   bool
	}{
		{
			name:      "Valid",
			driverID:  "driver-123",
			lat:       37.7749,
			long:      -122.4194,
			timestamp: timestamp,
			wantErr:   false,
		},
		{
			name:      "Lat too high",
			driverID:  "driver-123",
			lat:       91.0,
			long:      -122.4194,
			timestamp: timestamp,
			wantErr:   true,
		},
		{
			name:      "Lat too low",
			driverID:  "driver-123",
			lat:       -91.0,
			long:      -122.4194,
			timestamp: timestamp,
			wantErr:   true,
		},
		{
			name:      "Long too high",
			driverID:  "driver-123",
			lat:       37.7749,
			long:      181.0,
			timestamp: timestamp,
			wantErr:   true,
		},
		{
			name:      "Long too low",
			driverID:  "driver-123",
			lat:       37.7749,
			long:      -181.0,
			timestamp: timestamp,
			wantErr:   true,
		},
		{
			name:      "No Driver",
			driverID:  "",
			lat:       37.7749,
			long:      -122.4194,
			timestamp: timestamp,
			wantErr:   true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			loc, err := NewLocation(tc.driverID, tc.lat, tc.long, tc.timestamp)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if loc.DriverID != tc.driverID || loc.Lat != tc.lat || loc.Long != tc.long || !loc.Timestamp.Equal(tc.timestamp) {
				t.Fatalf("unexpected location: %+v", loc)
			}
		})
	}
}
