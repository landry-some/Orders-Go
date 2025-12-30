package sharding

import (
	"errors"
	"math"
)

var ErrInvalidCoordinate = errors.New("invalid coordinate")

// GetShardID assigns a shard based on the latitude and longitude quadrant.
func GetShardID(lat, long float64) (string, error) {
	if math.IsNaN(lat) || math.IsNaN(long) || math.IsInf(lat, 0) || math.IsInf(long, 0) {
		return "", ErrInvalidCoordinate
	}

	switch {
	case lat >= 0 && long >= 0:
		return "shard-1", nil
	case lat < 0 && long < 0:
		return "shard-2", nil
	case lat >= 0 && long < 0:
		return "shard-3", nil
	case lat < 0 && long >= 0:
		return "shard-4", nil
	}

	return "", ErrInvalidCoordinate
}
