package grid

import (
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

func (s *SpyWAL) Write(data []byte) error {
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

	if err := grid.Update(loc); err != nil {
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
	t.Cleanup(func() { wal.Close() })

	first := []byte(`{"driver":"1"}`)
	second := []byte(`{"driver":"2"}`)

	if err := wal.Write(first); err != nil {
		t.Fatalf("write first: %v", err)
	}
	if err := wal.Write(second); err != nil {
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
