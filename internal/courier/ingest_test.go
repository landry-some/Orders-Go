package courier

import (
	"context"
	"errors"
	"testing"
	"time"
)

type SpyPublisher struct {
	called           bool
	receivedCtx      context.Context
	receivedLocation Location
	err              error
}

func (s *SpyPublisher) Publish(ctx context.Context, loc Location) error {
	s.called = true
	s.receivedCtx = ctx
	s.receivedLocation = loc
	return s.err
}

func TestIngest_PublishesEvent(t *testing.T) {
	ctx := context.Background()
	publisher := &SpyPublisher{}
	ingest := NewIngestService(publisher)

	loc := Location{
		DriverID:  "driver-123",
		Lat:       37.7749,
		Long:      -122.4194,
		Timestamp: time.Date(2024, 1, 2, 15, 4, 5, 0, time.UTC),
	}

	err := ingest.Ingest(ctx, loc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !publisher.called {
		t.Fatalf("expected publisher to be called")
	}

	if publisher.receivedLocation != loc {
		t.Fatalf("publisher received wrong location: %+v", publisher.receivedLocation)
	}

	if publisher.receivedCtx != ctx {
		t.Fatalf("publisher received wrong context")
	}
}

func TestIngest_ReturnsPublishError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("publish failed")
	publisher := &SpyPublisher{err: expectedErr}
	ingest := NewIngestService(publisher)

	loc := Location{
		DriverID:  "driver-123",
		Lat:       37.7749,
		Long:      -122.4194,
		Timestamp: time.Date(2024, 1, 2, 15, 4, 5, 0, time.UTC),
	}

	err := ingest.Ingest(ctx, loc)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !errors.Is(err, expectedErr) {
		t.Fatalf("unexpected error: %v", err)
	}
}
