// Design: docs/architecture/api/process-protocol.md -- persistent plugin state.
package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// StateGet reads a registered runtime key owned by this plugin. An absent key
// returns false, nil; unavailable or corrupt storage returns an error.
func (p *Plugin) StateGet(ctx context.Context, key string) ([]byte, bool, error) {
	out, err := p.stateCall(ctx, rpc.MethodStateGet, rpc.StateInput{Key: key})
	if err != nil {
		return nil, false, err
	}
	return out.Data, out.Status == rpc.StateOK, nil
}

// StatePut waits for durable acknowledgement. Callers MUST treat any error,
// including cancellation, as unacknowledged; it may have committed before a
// transport failure. The daemon does not cancel an already-started disk write.
func (p *Plugin) StatePut(ctx context.Context, key string, data []byte) error {
	_, err := p.stateCall(ctx, rpc.MethodStatePut, rpc.StateInput{Key: key, Data: data})
	return err
}

// StateRemove deletes an owned key. An absent key is already removed.
func (p *Plugin) StateRemove(ctx context.Context, key string) error {
	_, err := p.stateCall(ctx, rpc.MethodStateRemove, rpc.StateInput{Key: key})
	return err
}

// StateList lists at most StateListMax owned keys under prefix; excess is an
// explicit error, never a partial enumeration.
func (p *Plugin) StateList(ctx context.Context, prefix string) ([]string, error) {
	out, err := p.stateCall(ctx, rpc.MethodStateList, rpc.StateInput{Key: prefix})
	return out.Keys, err
}

// StateIncrement atomically increments a durable big-endian uint32 counter.
// Callers MUST NOT retry an unacknowledged increment as if it had not committed.
func (p *Plugin) StateIncrement(ctx context.Context, key string) (uint32, error) {
	out, err := p.stateCall(ctx, rpc.MethodStateIncrement, rpc.StateInput{Key: key})
	if err != nil {
		return 0, err
	}
	if out.Value == 0 {
		return 0, errors.New("state increment acknowledged an invalid zero counter")
	}
	return out.Value, nil
}

func (p *Plugin) stateCall(ctx context.Context, method string, in rpc.StateInput) (rpc.StateOutput, error) {
	if err := ctx.Err(); err != nil {
		return rpc.StateOutput{}, err
	}
	if err := rpc.ValidateStateInput(in); err != nil {
		return rpc.StateOutput{}, err
	}
	raw, err := p.callEngineRaw(ctx, method, in)
	if err != nil {
		return rpc.StateOutput{}, err
	}
	if err := ctx.Err(); err != nil {
		return rpc.StateOutput{}, err
	}
	var out rpc.StateOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		return rpc.StateOutput{}, fmt.Errorf("decode state result: %w", err)
	}
	if len(out.Data) > rpc.StateDataMax {
		return rpc.StateOutput{}, fmt.Errorf("state result exceeds %d bytes", rpc.StateDataMax)
	}
	if len(out.Keys) > rpc.StateListMax {
		return rpc.StateOutput{}, fmt.Errorf("state result exceeds %d keys", rpc.StateListMax)
	}
	if out.Status == rpc.StateOK {
		return out, nil
	}
	if method == rpc.MethodStateGet {
		if out.Status == rpc.StateAbsent {
			return out, nil
		}
	}
	return rpc.StateOutput{}, &rpc.StateError{Status: out.Status, Message: out.Message}
}
