package grid

import (
	"sync"

	"wayfinder/internal/courier"
)

// GridService stores driver locations in memory with concurrency safety.
type GridService struct {
	mu        sync.RWMutex
	locations map[string]courier.Location
}

// NewGridService constructs a GridService with an initialized map.
func NewGridService() *GridService {
	return &GridService{
		locations: make(map[string]courier.Location),
	}
}

// Update upserts the driver's location.
func (g *GridService) Update(loc courier.Location) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.locations[loc.DriverID] = loc
}

// Get retrieves the driver's location if present.
func (g *GridService) Get(driverID string) (courier.Location, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	loc, ok := g.locations[driverID]
	return loc, ok
}
