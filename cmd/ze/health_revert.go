// Design: plan/learned/678-appliance-4-device-config.md — auto-revert on runtime failure after config push

//go:build ze_core

package main

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/zefs"
)

const healthCheckWindow = 30 * time.Second

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

	if err := h.store.WriteFile(zefs.KeyConfigPreviousActive.Pattern, preChangeConfig, 0); err != nil {
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
	prev, err := h.store.ReadFile(zefs.KeyConfigPreviousActive.Pattern)
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
		if writeErr := h.store.WriteFile(zefs.KeyConfigLastGoodPushed.Pattern, []byte(hashStr), 0); writeErr != nil {
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
