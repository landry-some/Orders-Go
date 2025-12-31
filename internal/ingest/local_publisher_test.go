package ingest_test

import (
	"context"
	"testing"
	"time"

	"wayfinder/internal/grid"
	"wayfinder/internal/ingest"
)

func TestLocalGridPublisherPublishes(t *testing.T) {
	ctx := context.Background()
	wal, err := grid.NewFileWAL(t.TempDir() + "/wal.log")
	if err != nil {
		t.Fatalf("new wal: %v", err)
	}
	t.Cleanup(func() { _ = wal.Close() })

	g := grid.NewGridService(wal)
	publisher := ingest.NewGridPublisher(g)

	loc := ingest.Location{
		DriverID:  "driver-123",
		Lat:       37.7749,
		Long:      -122.4194,
		Timestamp: time.Date(2024, 1, 2, 15, 4, 5, 0, time.UTC),
	}

	if err := publisher.Publish(ctx, loc); err != nil {
		t.Fatalf("publish: %v", err)
	}

	got, ok := g.Get(loc.DriverID)
	if !ok {
		t.Fatalf("expected location for driver %s", loc.DriverID)
	}

	if got != loc {
		t.Fatalf("got %+v, want %+v", got, loc)
	}
}
