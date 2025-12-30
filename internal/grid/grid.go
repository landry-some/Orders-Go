package grid

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"sync"

	"wayfinder/internal/courier"
)

// WAL writes serialized locations for durability.
type WAL interface {
	Write(ctx context.Context, data []byte) error
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

// NewGridServiceWithRecovery constructs a GridService and replays the WAL into memory.
func NewGridServiceWithRecovery(wal *FileWAL) (*GridService, error) {
	g := &GridService{
		locations: make(map[string]courier.Location),
		wal:       wal,
	}
	if err := g.loadFromWAL(wal); err != nil {
		return nil, err
	}
	return g, nil
}

// Update writes the location to the WAL before updating memory.
func (g *GridService) Update(ctx context.Context, loc courier.Location) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	payload, err := json.Marshal(loc)
	if err != nil {
		return err
	}

	if g.wal != nil {
		if err := g.wal.Write(ctx, payload); err != nil {
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

func (g *GridService) loadFromWAL(w *FileWAL) (err error) {
	file, err := os.Open(w.f.Name())
	if err != nil {
		return err
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			err = cerr
		}
	}()

	g.mu.Lock()
	defer g.mu.Unlock()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var loc courier.Location
		if err := json.Unmarshal(scanner.Bytes(), &loc); err != nil {
			return err
		}
		g.locations[loc.DriverID] = loc
	}

	return scanner.Err()
}
