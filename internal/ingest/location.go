package ingest

import (
	"errors"
	"time"
)

type Location struct {
	DriverID  string
	Lat       float64
	Long      float64
	Timestamp time.Time
}

// NewLocation constructs a Location with validation on input fields.
func NewLocation(driverID string, lat, long float64, timestamp time.Time) (Location, error) {
	if driverID == "" {
		return Location{}, ErrInvalidDriverID
	}

	if lat < -90 || lat > 90 {
		return Location{}, ErrInvalidLatitude
	}

	if long < -180 || long > 180 {
		return Location{}, ErrInvalidLongitude
	}

	return Location{
		DriverID:  driverID,
		Lat:       lat,
		Long:      long,
		Timestamp: timestamp,
	}, nil
}

var (
	ErrInvalidDriverID  = errors.New("driver id is required")
	ErrInvalidLatitude  = errors.New("latitude must be between -90 and 90")
	ErrInvalidLongitude = errors.New("longitude must be between -180 and 180")
)
