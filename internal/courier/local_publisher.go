package courier

import "context"

// LocationStore abstracts persistence for locations.
type LocationStore interface {
	Update(loc Location) error
}

// LocalGridPublisher publishes locations to a local grid service.
type LocalGridPublisher struct {
	grid LocationStore
}

// NewLocalGridPublisher constructs a publisher targeting the given grid.
func NewLocalGridPublisher(g LocationStore) *LocalGridPublisher {
	return &LocalGridPublisher{grid: g}
}

// Publish forwards the location to the grid store.
func (p *LocalGridPublisher) Publish(ctx context.Context, loc Location) error {
	return p.grid.Update(loc)
}
