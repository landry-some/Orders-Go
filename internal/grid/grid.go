package grid

import (
	"encoding/json"
	"sync"

	"wayfinder/internal/courier"
)

// WAL writes serialized locations for durability.
type WAL interface {
	Write(data []byte) error
}

// GridService stores driver locations in memory with concurrency safety.
type GridService struct {
	mu        sync.RWMutex
	locations map[string]courier.Location
	wal       WAL
}

// NewGridService constructs a GridService with an initialized map and WAL.
func NewGridService(wal WAL) *GridService {
	return &GridService{
		locations: make(map[string]courier.Location),
		wal:       wal,
	}
}

// Update writes the location to the WAL before updating memory.
func (g *GridService) Update(loc courier.Location) error {
	payload, err := json.Marshal(loc)
	if err != nil {
		return err
	}

	if g.wal != nil {
		if err := g.wal.Write(payload); err != nil {
			return err
		}
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	g.locations[loc.DriverID] = loc
	return nil
}

// Get retrieves the driver's location if present.
func (g *GridService) Get(driverID string) (courier.Location, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	loc, ok := g.locations[driverID]
	return loc, ok
}
