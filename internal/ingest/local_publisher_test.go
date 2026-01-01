package ingest_test

import (
	"context"
	"testing"
	"time"

	"wayfinder/internal/ingest"
)

type spyLocationStore struct {
	called   bool
	received ingest.Location
	err      error
}

func (s *spyLocationStore) Update(ctx context.Context, loc ingest.Location) error {
	s.called = true
	s.received = loc
	return s.err
}

func TestLocalGridPublisherPublishes(t *testing.T) {
	ctx := context.Background()
	store := &spyLocationStore{}
	publisher := ingest.NewStorePublisher(store)

	loc := ingest.Location{
		DriverID:  "driver-123",
		Lat:       37.7749,
		Long:      -122.4194,
		Timestamp: time.Date(2024, 1, 2, 15, 4, 5, 0, time.UTC),
	}

	if err := publisher.Publish(ctx, loc); err != nil {
		t.Fatalf("publish: %v", err)
	}

	if !store.called {
		t.Fatalf("expected store update to be called")
	}
	if store.received != loc {
		t.Fatalf("got %+v, want %+v", store.received, loc)
	}
}
