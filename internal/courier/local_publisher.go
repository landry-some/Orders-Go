package courier

import "context"

// LocationStore abstracts persistence for locations.
type LocationStore interface {
	Update(ctx context.Context, loc Location) error
}

// GridPublisher publishes locations to a grid-backed store.
type GridPublisher struct {
	grid LocationStore
}

// NewGridPublisher constructs a publisher targeting the given grid.
func NewGridPublisher(g LocationStore) *GridPublisher {
	return &GridPublisher{grid: g}
}

// Publish forwards the location to the grid store.
func (p *GridPublisher) Publish(ctx context.Context, loc Location) error {
	return p.grid.Update(ctx, loc)
}
