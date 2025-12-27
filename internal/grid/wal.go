package grid

import (
	"fmt"
	"os"
	"sync"
)

// FileWAL appends serialized entries to a file for durability.
type FileWAL struct {
	mu sync.Mutex
	f  *os.File
}

// NewFileWAL constructs a FileWAL targeting the given path.
func NewFileWAL(path string) (*FileWAL, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return &FileWAL{f: f}, nil
}

// Write appends the provided data to the WAL file.
func (w *FileWAL) Write(data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	n, err := w.f.Write(append(data, '\n'))
	if err != nil {
		return err
	}
	if n != len(data)+1 {
		return fmt.Errorf("partial write: wrote %d of %d bytes", n, len(data)+1)
	}

	return w.f.Sync()
}

// Close releases the underlying file handle.
func (w *FileWAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.f.Close()
}
