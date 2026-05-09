// Package lock provides a process-level file lock with context-based cancellation.
package lock

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

// Lock guards access to a shared resource via a flock-style advisory file lock.
type Lock struct {
	path string
	fl   *flock.Flock
}

// New creates a Lock anchored to path. The file is created on first Acquire.
func New(path string) *Lock {
	return &Lock{path: path}
}

// Acquire blocks until the lock is held or ctx expires.
// Polls every 50ms; ctx cancellation returns ctx.Err().
func (l *Lock) Acquire(ctx context.Context) error {
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return fmt.Errorf("ensure lock dir: %w", err)
	}
	l.fl = flock.New(l.path)

	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		got, err := l.fl.TryLock()
		if err != nil {
			return fmt.Errorf("flock: %w", err)
		}
		if got {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
}

// Release releases the lock. Safe to call only once after a successful Acquire.
func (l *Lock) Release() error {
	if l.fl == nil {
		return errors.New("lock not acquired")
	}
	if err := l.fl.Unlock(); err != nil {
		return fmt.Errorf("unlock: %w", err)
	}
	return nil
}
