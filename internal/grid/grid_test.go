package grid

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"wayfinder/internal/courier"
)

type SpyWAL struct {
	called bool
	data   []byte
}

func (s *SpyWAL) Write(ctx context.Context, data []byte) error {
	s.called = true
	s.data = data
	return nil
}

func TestGrid_UpdateAndGet(t *testing.T) {
	wal := &SpyWAL{}
	grid := NewGridService(wal)

	loc := courier.Location{
		DriverID:  "driver-123",
		Lat:       37.7749,
		Long:      -122.4194,
		Timestamp: time.Date(2024, 1, 2, 15, 4, 5, 0, time.UTC),
	}

	if err := grid.Update(context.Background(), loc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !wal.called {
		t.Fatalf("expected WAL Write to be called")
	}

	expectedJSON, err := json.Marshal(loc)
	if err != nil {
		t.Fatalf("failed to marshal location: %v", err)
	}

	if string(wal.data) != string(expectedJSON) {
		t.Fatalf("expected WAL data %s, got %s", string(expectedJSON), string(wal.data))
	}

	got, ok := grid.Get(loc.DriverID)
	if !ok {
		t.Fatalf("expected location for driver %s", loc.DriverID)
	}

	if got != loc {
		t.Fatalf("got %+v, want %+v", got, loc)
	}
}

func TestFileWAL_WritesAndAppends(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "wal.log")
	wal, err := NewFileWAL(path)
	if err != nil {
		t.Fatalf("new wal: %v", err)
	}
	t.Cleanup(func() { _ = wal.Close() })

	first := []byte(`{"driver":"1"}`)
	second := []byte(`{"driver":"2"}`)

	if err := wal.Write(context.Background(), first); err != nil {
		t.Fatalf("write first: %v", err)
	}
	if err := wal.Write(context.Background(), second); err != nil {
		t.Fatalf("write second: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read wal: %v", err)
	}

	expected := string(first) + "\n" + string(second) + "\n"
	if string(data) != expected {
		t.Fatalf("wal contents = %s, want %s", string(data), expected)
	}
}

func TestGrid_RecoversFromWAL(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "wal.log")

	locs := []courier.Location{
		{
			DriverID:  "driver-1",
			Lat:       10.0,
			Long:      20.0,
			Timestamp: time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
		},
		{
			DriverID:  "driver-2",
			Lat:       -10.0,
			Long:      -20.0,
			Timestamp: time.Date(2024, 1, 2, 4, 5, 6, 0, time.UTC),
		},
	}

	var data []byte
	for _, loc := range locs {
		b, err := json.Marshal(loc)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		data = append(data, b...)
		data = append(data, '\n')
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write wal: %v", err)
	}

	wal, err := NewFileWAL(path)
	if err != nil {
		t.Fatalf("new wal: %v", err)
	}
	t.Cleanup(func() { _ = wal.Close() })

	grid, err := NewGridServiceWithRecovery(wal)
	if err != nil {
		t.Fatalf("new grid with recovery: %v", err)
	}

	for _, expected := range locs {
		got, ok := grid.Get(expected.DriverID)
		if !ok {
			t.Fatalf("expected location for driver %s", expected.DriverID)
		}
		if got != expected {
			t.Fatalf("got %+v, want %+v", got, expected)
		}
	}
}

func TestUpdate_RespectsContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	wal := &SpyWAL{}
	g := NewGridService(wal)

	loc := courier.Location{
		DriverID:  "driver-ctx",
		Lat:       1.0,
		Long:      2.0,
		Timestamp: time.Now(),
	}

	err := g.Update(ctx, loc)
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	if wal.called {
		t.Fatalf("expected WAL.Write not to be called on canceled context")
	}
}
