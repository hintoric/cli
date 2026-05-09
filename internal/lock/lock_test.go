package lock

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestAcquireRelease(t *testing.T) {
	dir := t.TempDir()
	l := New(filepath.Join(dir, "test.lock"))

	if err := l.Acquire(context.Background()); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if err := l.Release(); err != nil {
		t.Fatalf("release: %v", err)
	}
}

func TestSecondAcquireBlocksUntilRelease(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.lock")
	a := New(path)
	b := New(path)

	if err := a.Acquire(context.Background()); err != nil {
		t.Fatalf("a acquire: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := b.Acquire(ctx)
	if err == nil {
		t.Fatal("expected b.Acquire to fail while a holds the lock")
	}

	if err := a.Release(); err != nil {
		t.Fatalf("a release: %v", err)
	}

	if err := b.Acquire(context.Background()); err != nil {
		t.Fatalf("b acquire after release: %v", err)
	}
	if err := b.Release(); err != nil {
		t.Fatalf("b release: %v", err)
	}
}
