// Design: docs/architecture/system-architecture.md — temporary filesystem management

package tmpfs

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// WriteToTempWithContext creates temp dir, writes files, returns path and cleanup.
// Cleanup is called automatically on ctx.Done() or SIGINT/SIGTERM.
// The returned cleanup function should still be deferred to handle normal exit.
func (v *Tmpfs) WriteToTempWithContext(ctx context.Context) (dir string, cleanup func(), err error) {
	dir, err = os.MkdirTemp("", "ze-tmpfs-*")
	if err != nil {
		return "", nil, err
	}

	var once sync.Once
	cleanup = func() {
		once.Do(func() {
			_ = os.RemoveAll(dir)
		})
	}

	// Cleanup on context cancel
	go func() {
		<-ctx.Done()
		cleanup()
	}()

	// Cleanup on signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			cleanup()
			// Re-raise the signal for proper exit code
			signal.Stop(sigCh)
			// Don't os.Exit here - let the main program handle it
		case <-ctx.Done():
			// Context canceled, signal handler no longer needed
			signal.Stop(sigCh)
		}
	}()

	if err := v.WriteTo(dir); err != nil {
		cleanup()
		return "", nil, err
	}

	return dir, cleanup, nil
}

// CleanupManager manages temp directories with signal handling.
// Use this when you need to track multiple temp directories.
type CleanupManager struct {
	mu      sync.Mutex
	dirs    []string
	cleanup func()
	once    sync.Once
}

// NewCleanupManager creates a cleanup manager that handles signals.
func NewCleanupManager(ctx context.Context) *CleanupManager {
	cm := &CleanupManager{}

	cm.cleanup = func() {
		cm.mu.Lock()
		defer cm.mu.Unlock()
		for _, dir := range cm.dirs {
			_ = os.RemoveAll(dir)
		}
		cm.dirs = nil
	}

	// Cleanup on context cancel
	go func() {
		<-ctx.Done()
		cm.once.Do(cm.cleanup)
	}()

	// Cleanup on signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			cm.once.Do(cm.cleanup)
			signal.Stop(sigCh)
		case <-ctx.Done():
			signal.Stop(sigCh)
		}
	}()

	return cm
}

// Register adds a directory to be cleaned up.
func (cm *CleanupManager) Register(dir string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.dirs = append(cm.dirs, dir)
}

// Cleanup removes all registered directories.
// Safe to call multiple times.
func (cm *CleanupManager) Cleanup() {
	cm.once.Do(cm.cleanup)
}
