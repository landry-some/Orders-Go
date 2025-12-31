package ingest

import "context"

// MultiLocationStore writes to multiple location stores in order.
type MultiLocationStore struct {
	stores []LocationStore
}

// NewMultiLocationStore constructs a LocationStore that updates each store in sequence.
func NewMultiLocationStore(stores ...LocationStore) *MultiLocationStore {
	return &MultiLocationStore{stores: stores}
}

// Update forwards the location to each store until an error occurs.
func (m *MultiLocationStore) Update(ctx context.Context, loc Location) error {
	for _, store := range m.stores {
		if err := store.Update(ctx, loc); err != nil {
			return err
		}
	}
	return nil
}
