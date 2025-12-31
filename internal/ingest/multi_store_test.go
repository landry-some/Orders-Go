package ingest

import (
	"context"
	"errors"
	"testing"
)

type spyStore struct {
	calls int
	err   error
}

func (s *spyStore) Update(ctx context.Context, loc Location) error {
	s.calls++
	return s.err
}

func TestMultiLocationStore_UpdateCallsAllStores(t *testing.T) {
	first := &spyStore{}
	second := &spyStore{}

	store := NewMultiLocationStore(first, second)
	if err := store.Update(context.Background(), Location{DriverID: "driver-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if first.calls != 1 || second.calls != 1 {
		t.Fatalf("expected both stores to be called, got first=%d second=%d", first.calls, second.calls)
	}
}

func TestMultiLocationStore_UpdateStopsOnError(t *testing.T) {
	first := &spyStore{err: errors.New("write failed")}
	second := &spyStore{}

	store := NewMultiLocationStore(first, second)
	if err := store.Update(context.Background(), Location{DriverID: "driver-1"}); err == nil {
		t.Fatalf("expected error")
	}

	if first.calls != 1 {
		t.Fatalf("expected first store to be called")
	}
	if second.calls != 0 {
		t.Fatalf("expected second store to not be called, got %d", second.calls)
	}
}

func TestMultiLocationStore_UpdatePropagatesLaterError(t *testing.T) {
	first := &spyStore{}
	second := &spyStore{err: errors.New("redis failed")}

	store := NewMultiLocationStore(first, second)
	if err := store.Update(context.Background(), Location{DriverID: "driver-1"}); err == nil {
		t.Fatalf("expected error")
	}

	if first.calls != 1 || second.calls != 1 {
		t.Fatalf("expected both stores to be called, got first=%d second=%d", first.calls, second.calls)
	}
}
