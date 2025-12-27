package sharding

// GetShardID assigns a shard based on the latitude and longitude quadrant.
func GetShardID(lat, long float64) string {
	switch {
	case lat >= 0 && long >= 0:
		return "shard-1"
	case lat < 0 && long < 0:
		return "shard-2"
	case lat >= 0 && long < 0:
		return "shard-3"
	case lat < 0 && long >= 0:
		return "shard-4"
	}

	// Fallback for non-comparable inputs (e.g., NaN).
	return "shard-1"
}
