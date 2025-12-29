package lock

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const (
	LockFileName   = "lock.json"
	DefaultTimeout = 15 * time.Second
	PollInterval   = 100 * time.Millisecond
)

type LockInfo struct {
	PID       int       `json:"pid"`
	CreatedAt time.Time `json:"created_at"`
}

type Lock struct {
	path string
}

// Acquire tries to acquire a lock in the given directory.
// It will block up to timeout waiting for the lock to become available.
// Returns a Lock that must be released when done, or an error if the lock
// could not be acquired within the timeout.
func Acquire(lockDir string, timeout time.Duration) (*Lock, error) {
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create lock directory: %w", err)
	}

	lockPath := filepath.Join(lockDir, LockFileName)
	deadline := time.Now().Add(timeout)

	for {
		// Try to acquire the lock
		acquired, err := tryAcquire(lockPath)
		if err != nil {
			return nil, err
		}
		if acquired {
			return &Lock{path: lockPath}, nil
		}

		// Check if we've exceeded the timeout
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for lock (another process may be running)")
		}

		// Wait before trying again
		time.Sleep(PollInterval)
	}
}

// tryAcquire attempts to acquire the lock once.
// Returns true if the lock was acquired, false if it's held by another process.
func tryAcquire(lockPath string) (bool, error) {
	// Check if lock file exists
	data, err := os.ReadFile(lockPath)
	if err == nil {
		// Lock file exists, check if the process is still alive
		var info LockInfo
		if err := json.Unmarshal(data, &info); err == nil {
			if isProcessAlive(info.PID) {
				// Process is still alive, lock is valid
				return false, nil
			}
			// Process is dead, remove stale lock
			os.Remove(lockPath)
		} else {
			// Corrupted lock file, remove it
			os.Remove(lockPath)
		}
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("failed to read lock file: %w", err)
	}

	// Try to create the lock file atomically
	info := LockInfo{
		PID:       os.Getpid(),
		CreatedAt: time.Now().UTC(),
	}
	data, err = json.Marshal(info)
	if err != nil {
		return false, fmt.Errorf("failed to marshal lock info: %w", err)
	}

	// Use O_EXCL for atomic creation
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			// Another process created the lock first
			return false, nil
		}
		return false, fmt.Errorf("failed to create lock file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		os.Remove(lockPath)
		return false, fmt.Errorf("failed to write lock file: %w", err)
	}

	return true, nil
}

// Release releases the lock.
func (l *Lock) Release() error {
	if l == nil || l.path == "" {
		return nil
	}

	// Verify we still own the lock before releasing
	data, err := os.ReadFile(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Lock already released
		}
		return fmt.Errorf("failed to read lock file: %w", err)
	}

	var info LockInfo
	if err := json.Unmarshal(data, &info); err != nil {
		// Corrupted, just remove it
		return os.Remove(l.path)
	}

	if info.PID != os.Getpid() {
		// Not our lock anymore (shouldn't happen, but be safe)
		return nil
	}

	return os.Remove(l.path)
}

// isProcessAlive checks if a process with the given PID is still running.
func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix, FindProcess always succeeds. Send signal 0 to check if process exists.
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
