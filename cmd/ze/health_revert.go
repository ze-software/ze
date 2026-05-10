// Design: plan/spec-appliance-4-device-config.md — auto-revert on runtime failure after config push

package main

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

const (
	healthCheckWindow     = 30 * time.Second
	configPreviousPath    = "/perm/ze/config-previous.conf"
	lastKnownGoodPushPath = "/perm/ze/last-known-good-pushed"
)

var (
	readConfigPrevious  = defaultReadConfigPrevious
	writeConfigPrevious = defaultWriteConfigPrevious
	writeLKGPushed      = defaultWriteLKGPushed
)

func defaultReadConfigPrevious() ([]byte, error) {
	return os.ReadFile(configPreviousPath) //nolint:gosec // appliance persistent path
}

func defaultWriteConfigPrevious(data []byte) error {
	return os.WriteFile(configPreviousPath, data, 0o644) //nolint:gosec // appliance config
}

func defaultWriteLKGPushed(hash string) error {
	return os.WriteFile(lastKnownGoodPushPath, []byte(hash), 0o644) //nolint:gosec // informational
}

type HealthRevert struct {
	store      storage.Storage
	configName string
	timer      *time.Timer
	mu         sync.Mutex
	done       chan struct{}
	reverted   bool
	completed  bool
}

var _ registry.PeerLifecycleCallback = (*HealthRevert)(nil)

func (h *HealthRevert) OnPeerEstablished(_ any) {}

func NewHealthRevert(store storage.Storage, configName string) *HealthRevert {
	return &HealthRevert{
		store:      store,
		configName: configName,
		done:       make(chan struct{}),
	}
}

func (h *HealthRevert) Start(preChangeConfig []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if err := writeConfigPrevious(preChangeConfig); err != nil {
		slog.Warn("save previous config for revert", "error", err)
	}

	h.timer = time.AfterFunc(healthCheckWindow, func() {
		h.onHealthy()
	})
}

func (h *HealthRevert) OnPeerClosed(_ any, reason string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.timer == nil || h.completed {
		return
	}
	h.timer.Stop()
	h.reverted = true
	h.completed = true

	slog.Warn("BGP session flap during health check, reverting config", "reason", reason)
	h.revert()
	close(h.done)
}

func (h *HealthRevert) revert() {
	prev, err := readConfigPrevious()
	if err == nil && len(prev) > 0 {
		activeKey := zefs.KeyFileActive.Key(h.configName)
		if writeErr := h.store.WriteFile(activeKey, prev, 0); writeErr == nil {
			slog.Info("reverted to previous config")
			return
		}
		slog.Warn("revert to previous config failed, trying seed")
	}

	seed, err := h.store.ReadFile(zefs.KeyFileTemplate.Key("ze.conf"))
	if err != nil {
		slog.Error("revert failed: cannot read seed config", "error", err)
		return
	}
	activeKey := zefs.KeyFileActive.Key(h.configName)
	if writeErr := h.store.WriteFile(activeKey, seed, 0); writeErr != nil {
		slog.Error("revert to seed config failed", "error", writeErr)
		return
	}
	slog.Info("reverted to seed config")
}

func (h *HealthRevert) onHealthy() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.completed {
		return
	}
	h.completed = true

	activeData, err := h.store.ReadFile(zefs.KeyFileActive.Key(h.configName))
	if err != nil {
		slog.Warn("read active config for LKG update", "error", err)
	} else {
		hash := sha256.Sum256(activeData)
		hashStr := fmt.Sprintf("sha256:%x", hash)
		if writeErr := writeLKGPushed(hashStr); writeErr != nil {
			slog.Warn("write last-known-good-pushed", "error", writeErr)
		}
	}

	slog.Info("config health check passed, config confirmed")
	close(h.done)
}

func (h *HealthRevert) Wait() {
	<-h.done
}

func (h *HealthRevert) Reverted() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.reverted
}
