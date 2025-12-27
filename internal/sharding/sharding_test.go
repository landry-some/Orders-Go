package sharding

import "testing"

func TestGetShardID(t *testing.T) {
	cases := []struct {
		name     string
		lat      float64
		long     float64
		expected string
	}{
		{name: "North East", lat: 10.0, long: 10.0, expected: "shard-1"},
		{name: "South West", lat: -10.0, long: -10.0, expected: "shard-2"},
		{name: "North West", lat: 10.0, long: -10.0, expected: "shard-3"},
		{name: "South East", lat: -10.0, long: 10.0, expected: "shard-4"},
		{name: "Origin", lat: 0.0, long: 0.0, expected: "shard-1"},
		{name: "Edge North East", lat: 90.0, long: 180.0, expected: "shard-1"},
		{name: "Edge South West", lat: -90.0, long: -180.0, expected: "shard-2"},
		{name: "Edge North West", lat: 0.0, long: -180.0, expected: "shard-3"},
		{name: "Edge South East", lat: -90.0, long: 0.0, expected: "shard-4"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := GetShardID(tc.lat, tc.long)
			if got != tc.expected {
				t.Fatalf("GetShardID(%f, %f) = %s, want %s", tc.lat, tc.long, got, tc.expected)
			}
		})
	}
}
