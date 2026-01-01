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

func TestMultiLocationStore_UpdateContinuesOnErrors(t *testing.T) {
	firstErr := errors.New("write failed")
	secondErr := errors.New("redis failed")

	first := &spyStore{err: firstErr}
	second := &spyStore{err: secondErr}

	store := NewMultiLocationStore(first, second)
	err := store.Update(context.Background(), Location{DriverID: "driver-1"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if first.calls != 1 || second.calls != 1 {
		t.Fatalf("expected both stores to be called, got first=%d second=%d", first.calls, second.calls)
	}
	for _, target := range []error{firstErr, secondErr} {
		if !errors.Is(err, target) {
			t.Fatalf("expected returned error to include %v, got %v", target, err)
		}
	}
}
