package lock

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAcquireRelease(t *testing.T) {
	dir := t.TempDir()

	lck, err := Acquire(dir, DefaultTimeout)
	if err != nil {
		t.Fatalf("failed to acquire lock: %v", err)
	}

	// Lock file should exist
	lockPath := filepath.Join(dir, LockFileName)
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("lock file should exist: %v", err)
	}

	// Release the lock
	if err := lck.Release(); err != nil {
		t.Fatalf("failed to release lock: %v", err)
	}

	// Lock file should be gone
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("lock file should be removed after release")
	}
}

func TestAcquireBlocks(t *testing.T) {
	dir := t.TempDir()

	// Acquire first lock
	lck1, err := Acquire(dir, DefaultTimeout)
	if err != nil {
		t.Fatalf("failed to acquire first lock: %v", err)
	}
	defer lck1.Release()

	// Second acquire should timeout quickly
	start := time.Now()
	_, err = Acquire(dir, 200*time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("expected second acquire to fail")
	}
	if elapsed < 150*time.Millisecond {
		t.Fatalf("expected acquire to wait before timing out, elapsed: %v", elapsed)
	}
}

func TestStaleLockRemoved(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, LockFileName)

	// Create a lock file with a non-existent PID
	info := LockInfo{
		PID:       999999999, // Very unlikely to exist
		CreatedAt: time.Now().UTC(),
	}
	data, _ := json.Marshal(info)
	if err := os.WriteFile(lockPath, data, 0o644); err != nil {
		t.Fatalf("failed to create stale lock: %v", err)
	}

	// Should be able to acquire despite stale lock
	lck, err := Acquire(dir, DefaultTimeout)
	if err != nil {
		t.Fatalf("failed to acquire lock with stale lock present: %v", err)
	}
	defer lck.Release()
}

func TestCorruptedLockRemoved(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, LockFileName)

	// Create a corrupted lock file
	if err := os.WriteFile(lockPath, []byte("not valid json"), 0o644); err != nil {
		t.Fatalf("failed to create corrupted lock: %v", err)
	}

	// Should be able to acquire despite corrupted lock
	lck, err := Acquire(dir, DefaultTimeout)
	if err != nil {
		t.Fatalf("failed to acquire lock with corrupted lock present: %v", err)
	}
	defer lck.Release()
}

func TestDoubleRelease(t *testing.T) {
	dir := t.TempDir()

	lck, err := Acquire(dir, DefaultTimeout)
	if err != nil {
		t.Fatalf("failed to acquire lock: %v", err)
	}

	// First release should work
	if err := lck.Release(); err != nil {
		t.Fatalf("first release failed: %v", err)
	}

	// Second release should be a no-op (no error)
	if err := lck.Release(); err != nil {
		t.Fatalf("second release should not error: %v", err)
	}
}

func TestNilRelease(t *testing.T) {
	var lck *Lock
	if err := lck.Release(); err != nil {
		t.Fatalf("nil release should not error: %v", err)
	}
}
