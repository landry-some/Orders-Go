package ingest

import "context"

// LocationStore abstracts persistence for locations.
type LocationStore interface {
	Update(ctx context.Context, loc Location) error
}

// StorePublisher publishes locations to a LocationStore.
type StorePublisher struct {
	store LocationStore
}

// NewStorePublisher constructs a publisher targeting the given store.
func NewStorePublisher(store LocationStore) *StorePublisher {
	return &StorePublisher{store: store}
}

// Publish forwards the location to the grid store.
func (p *StorePublisher) Publish(ctx context.Context, loc Location) error {
	return p.store.Update(ctx, loc)
}
