package courier

import (
	"context"
	"encoding/json"
	"time"

	"wayfinder/internal/sharding"
)

// Broadcaster pushes messages to connected clients.
type Broadcaster interface {
	Broadcast(msg []byte)
}

// FanoutPublisher forwards locations to storage and broadcasts them.
type FanoutPublisher struct {
	storage     LocationPublisher
	broadcaster Broadcaster
}

// NewFanoutPublisher constructs a publisher that fan-outs to storage and broadcaster.
func NewFanoutPublisher(storage LocationPublisher, broadcaster Broadcaster) *FanoutPublisher {
	return &FanoutPublisher{storage: storage, broadcaster: broadcaster}
}

// Publish writes to storage then broadcasts the location (including shard).
func (p *FanoutPublisher) Publish(ctx context.Context, loc Location) error {
	if err := p.storage.Publish(ctx, loc); err != nil {
		return err
	}

	shard, err := sharding.GetShardID(loc.Lat, loc.Long)
	if err != nil {
		return err
	}

	payload := struct {
		Type      string    `json:"type"`
		DriverID  string    `json:"driver_id"`
		Lat       float64   `json:"lat"`
		Long      float64   `json:"long"`
		Shard     string    `json:"shard"`
		Timestamp time.Time `json:"timestamp"`
	}{
		Type:      "location",
		DriverID:  loc.DriverID,
		Lat:       loc.Lat,
		Long:      loc.Long,
		Shard:     shard,
		Timestamp: loc.Timestamp,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	if p.broadcaster != nil {
		p.broadcaster.Broadcast(data)
	}

	return nil
}
