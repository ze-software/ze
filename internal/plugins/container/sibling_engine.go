// Design: docs/architecture/container/runtime.md -- the sibling seam: the engine client
// Related: sibling.go -- the registration that hands a sibling this client
// Related: container.go -- the engine entry, which builds the client over its plugin connection

package container

import (
	"context"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// EngineClient is the part of the engine a sibling reaches: the owned
// interface devices, the firewall tables, and the state of the keys the
// sibling granted the plugin with statestore.RegisterPluginKeys. Each call is
// a request over the plugin's one engine connection, the same for an internal
// and a forked plugin, so a sibling never imports the interface or the
// firewall component. A sibling's test fakes this interface.
//
// Safe for concurrent use. Each call blocks until the engine answers or ctx
// ends. A refusal of the engine (a device name the sibling did not reserve,
// an interface or firewall component the config does not load) is the error.
type EngineClient interface {
	// OwnedBridgesReplace replaces the plugin's complete set of owned bridges.
	OwnedBridgesReplace(ctx context.Context, input *rpc.OwnedBridgesReplaceInput) (*rpc.OwnedBridgesReplaceOutput, error)
	// OwnedLinkCreate creates one link from an owned bridge into the network
	// namespace of a process, and answers the host end's name.
	OwnedLinkCreate(ctx context.Context, input *rpc.OwnedLinkCreateInput) (*rpc.OwnedLinkCreateOutput, error)
	// OwnedLinkRemove removes every link recorded under one key.
	OwnedLinkRemove(ctx context.Context, input *rpc.OwnedLinkRemoveInput) (*rpc.OwnedLinkRemoveOutput, error)
	// FirewallTablesReplace replaces the plugin's complete set of firewall
	// tables.
	FirewallTablesReplace(ctx context.Context, input *rpc.FirewallTablesReplaceInput) (*rpc.FirewallTablesReplaceOutput, error)
	// StateGet reads a granted key. An absent key answers false and no error.
	StateGet(ctx context.Context, key string) ([]byte, bool, error)
	// StatePut writes a granted key and waits for the durable acknowledgement.
	StatePut(ctx context.Context, key string, data []byte) error
}

// The plugin's own connection answers every call of the client.
var _ EngineClient = (*sdk.Plugin)(nil)

// startedEngine is the EngineClient the engine entry hands the siblings: the
// plugin's engine connection, which it refuses to use until open is called.
// Before the engine started the plugin a request is never answered, and the
// start-up sweep makes its ended calls before that, so a request then is
// refused at once rather than left to wait for its deadline. open MUST be
// called once, when the engine reports the plugin started. Safe for
// concurrent use.
type startedEngine struct {
	engine EngineClient
	opened atomic.Bool
}

// open lets every later request reach the engine.
func (e *startedEngine) open() {
	e.opened.Store(true)
}

// refuseBeforeOpen refuses request while the engine has not started the
// plugin.
func (e *startedEngine) refuseBeforeOpen(request string) error {
	if e.opened.Load() {
		return nil
	}
	return opererr.New(
		"the container plugin did not send "+request+": the engine has not started the plugin yet",
		"the engine answers a plugin's requests only once its startup ended, and the start-up sweep runs before it",
		exampleShowDoctor,
	)
}

func (e *startedEngine) OwnedBridgesReplace(ctx context.Context, input *rpc.OwnedBridgesReplaceInput) (*rpc.OwnedBridgesReplaceOutput, error) {
	if err := e.refuseBeforeOpen("its owned bridges"); err != nil {
		return nil, err
	}
	return e.engine.OwnedBridgesReplace(ctx, input)
}

func (e *startedEngine) OwnedLinkCreate(ctx context.Context, input *rpc.OwnedLinkCreateInput) (*rpc.OwnedLinkCreateOutput, error) {
	if err := e.refuseBeforeOpen("an owned link creation"); err != nil {
		return nil, err
	}
	return e.engine.OwnedLinkCreate(ctx, input)
}

func (e *startedEngine) OwnedLinkRemove(ctx context.Context, input *rpc.OwnedLinkRemoveInput) (*rpc.OwnedLinkRemoveOutput, error) {
	if err := e.refuseBeforeOpen("an owned link removal"); err != nil {
		return nil, err
	}
	return e.engine.OwnedLinkRemove(ctx, input)
}

func (e *startedEngine) FirewallTablesReplace(ctx context.Context, input *rpc.FirewallTablesReplaceInput) (*rpc.FirewallTablesReplaceOutput, error) {
	if err := e.refuseBeforeOpen("its firewall tables"); err != nil {
		return nil, err
	}
	return e.engine.FirewallTablesReplace(ctx, input)
}

func (e *startedEngine) StateGet(ctx context.Context, key string) ([]byte, bool, error) {
	if err := e.refuseBeforeOpen("a state read of " + key); err != nil {
		return nil, false, err
	}
	return e.engine.StateGet(ctx, key)
}

func (e *startedEngine) StatePut(ctx context.Context, key string, data []byte) error {
	if err := e.refuseBeforeOpen("a state write of " + key); err != nil {
		return err
	}
	return e.engine.StatePut(ctx, key, data)
}
