package ingest

import (
	"context"
	"errors"
)

// MultiLocationStore writes to multiple location stores in order.
type MultiLocationStore struct {
	stores []LocationStore
}

// NewMultiLocationStore constructs a LocationStore that updates each store in sequence.
func NewMultiLocationStore(stores ...LocationStore) *MultiLocationStore {
	return &MultiLocationStore{stores: stores}
}

// Update forwards the location to each store, collecting errors so all stores get a chance to write.
func (m *MultiLocationStore) Update(ctx context.Context, loc Location) error {
	var errs []error
	for _, store := range m.stores {
		if err := store.Update(ctx, loc); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
