package ingest

import (
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"
)

type spyPublisher struct {
	called bool
	ctx    context.Context
	loc    Location
	err    error
}

func (s *spyPublisher) Publish(ctx context.Context, loc Location) error {
	s.called = true
	s.ctx = ctx
	s.loc = loc
	return s.err
}

type spyBroadcaster struct {
	called bool
	msg    []byte
}

func (s *spyBroadcaster) Broadcast(msg []byte) {
	s.called = true
	s.msg = msg
}

func TestFanoutPublisherPublishesAndBroadcasts(t *testing.T) {
	t.Parallel()

	inner := &spyPublisher{}
	bcaster := &spyBroadcaster{}
	pub := NewFanoutPublisher(inner, bcaster)

	loc := Location{
		DriverID:  "driver-123",
		Lat:       10,
		Long:      10,
		Timestamp: time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
	}

	ctx := context.Background()
	if err := pub.Publish(ctx, loc); err != nil {
		t.Fatalf("publish: %v", err)
	}

	if !inner.called {
		t.Fatalf("inner publisher not called")
	}

	if !bcaster.called {
		t.Fatalf("broadcaster not called")
	}

	var payload struct {
		Type     string  `json:"type"`
		DriverID string  `json:"driver_id"`
		Lat      float64 `json:"lat"`
		Long     float64 `json:"long"`
	}
	if err := json.Unmarshal(bcaster.msg, &payload); err != nil {
		t.Fatalf("unmarshal broadcast: %v", err)
	}

	if payload.DriverID != loc.DriverID {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestFanoutPublisherSkipsBroadcastOnInnerError(t *testing.T) {
	t.Parallel()

	inner := &spyPublisher{err: context.Canceled}
	bcaster := &spyBroadcaster{}
	pub := NewFanoutPublisher(inner, bcaster)

	loc := Location{DriverID: "driver-err"}

	err := pub.Publish(context.Background(), loc)
	if err == nil {
		t.Fatalf("expected error")
	}

	if bcaster.called {
		t.Fatalf("expected broadcaster not to be called on error")
	}
}

func TestFanoutPublisherHandlesNilBroadcaster(t *testing.T) {
	t.Parallel()

	inner := &spyPublisher{}
	pub := NewFanoutPublisher(inner, nil)

	loc := Location{DriverID: "driver-nil"}
	if err := pub.Publish(context.Background(), loc); err != nil {
		t.Fatalf("publish: %v", err)
	}

	if !inner.called {
		t.Fatalf("expected inner publisher to run")
	}
}

func TestFanoutPublisherReturnsMarshalError(t *testing.T) {
	t.Parallel()

	inner := &spyPublisher{}
	bcaster := &spyBroadcaster{}
	pub := NewFanoutPublisher(inner, bcaster)

	loc := Location{DriverID: "driver-nan", Lat: math.NaN()}
	err := pub.Publish(context.Background(), loc)
	if err == nil {
		t.Fatalf("expected marshal error")
	}
	if !inner.called {
		t.Fatalf("expected inner publisher to be called before marshal")
	}
	if bcaster.called {
		t.Fatalf("expected no broadcast on marshal error")
	}
}
