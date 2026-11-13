// Design: docs/architecture/api/process-protocol.md -- authenticated instance identity.
package sdk

import (
	"encoding/json"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// InstanceName returns the configured plugin name acknowledged by the engine
// at Stage 1. Before that acknowledgment it returns an empty string.
func (p *Plugin) InstanceName() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.instanceName
}

func (p *Plugin) acceptInstanceName(answer json.RawMessage) error {
	var out rpc.DeclareRegistrationOutput
	if err := json.Unmarshal(answer, &out); err != nil {
		return err
	}
	p.mu.Lock()
	p.instanceName = out.PluginName
	p.mu.Unlock()
	return nil
}
