// Design: docs/architecture/api/process-protocol.md -- daemon-owned plugin state.
package sdk

import (
	"context"
	"io/fs"
	"time"
)

// StateKeys adapts daemon state RPCs to raw-key consumers. The caller MUST keep
// the supplied context alive for the consumer lifetime and cancel it at shutdown.
// Calls MUST begin no earlier than OnStarted. Safe for concurrent use.
type StateKeys struct {
	plugin *Plugin
	ctx    context.Context
}

// StateKeys binds raw-key operations to a plugin lifetime. Each call is bounded
// to five seconds; the caller MUST cancel ctx when its consumer stops.
func (p *Plugin) StateKeys(ctx context.Context) *StateKeys {
	return &StateKeys{plugin: p, ctx: ctx}
}

// ReadKey returns owned bytes, or fs.ErrNotExist when the key is absent.
func (s *StateKeys) ReadKey(key string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()
	data, found, err := s.plugin.StateGet(ctx, key)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fs.ErrNotExist
	}
	return data, nil
}

// WriteKey waits for the daemon's durable acknowledgement.
func (s *StateKeys) WriteKey(key string, data []byte) error {
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()
	return s.plugin.StatePut(ctx, key, data)
}

// RemoveKey waits for durable removal; an absent key is already removed.
func (s *StateKeys) RemoveKey(key string) error {
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()
	return s.plugin.StateRemove(ctx, key)
}

// ListKeys returns owned keys below prefix, or an error instead of a partial list.
func (s *StateKeys) ListKeys(prefix string) ([]string, error) {
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()
	return s.plugin.StateList(ctx, prefix)
}
