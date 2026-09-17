// Design: docs/architecture/api/process-protocol.md -- strict plugin state RPCs.
package server

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func stateInput(proc *process.Process, params json.RawMessage, list bool) (rpc.StateInput, error) {
	var in rpc.StateInput
	if len(params) > 2*rpc.StateDataMax {
		return in, errors.New("state request exceeds maximum encoded size")
	}
	if err := json.Unmarshal(params, &in); err != nil {
		return in, fmt.Errorf("decode state request: %w", err)
	}
	if err := rpc.ValidateStateInput(in); err != nil {
		return in, err
	}
	if proc == nil {
		return in, errors.New("state request has no plugin owner")
	}
	if !list {
		if !statestore.PluginKeyAllowed(proc.Name(), in.Key) {
			return in, fmt.Errorf("plugin %s does not own state key %s", proc.Name(), in.Key)
		}
	}
	return in, nil
}

func stateFailure(err error, failure rpc.StateStatus) rpc.StateOutput {
	if errors.Is(err, statestore.ErrUnavailable) {
		failure = rpc.StateUnavailable
	}
	if errors.Is(err, statestore.ErrCorrupt) {
		failure = rpc.StateCorrupt
	}
	if errors.Is(err, storage.ErrCorrupt) {
		failure = rpc.StateCorrupt
	}
	message := err.Error()
	if len(message) > rpc.StateMessageMax {
		message = message[:rpc.StateMessageMax]
	}
	return rpc.StateOutput{Status: failure, Message: message}
}

func (s *Server) opStateGet(proc *process.Process, params json.RawMessage) (any, error) {
	in, err := stateInput(proc, params, false)
	if err != nil {
		return nil, err
	}
	data, found, err := statestore.Read(in.Key)
	if err != nil {
		return stateFailure(err, rpc.StateReadFailed), nil
	}
	if !found {
		return rpc.StateOutput{Status: rpc.StateAbsent}, nil
	}
	if len(data) > rpc.StateDataMax {
		return stateFailure(errors.New("stored value exceeds state data limit"), rpc.StateReadFailed), nil
	}
	return rpc.StateOutput{Status: rpc.StateOK, Data: data}, nil
}

func (s *Server) opStatePut(proc *process.Process, params json.RawMessage) (any, error) {
	in, err := stateInput(proc, params, false)
	if err != nil {
		return nil, err
	}
	if err := statestore.Write(in.Key, in.Data); err != nil {
		return stateFailure(err, rpc.StatePersistFailed), nil
	}
	return rpc.StateOutput{Status: rpc.StateOK}, nil
}

func (s *Server) opStateRemove(proc *process.Process, params json.RawMessage) (any, error) {
	in, err := stateInput(proc, params, false)
	if err != nil {
		return nil, err
	}
	if err := statestore.Delete(in.Key); err != nil {
		return stateFailure(err, rpc.StatePersistFailed), nil
	}
	return rpc.StateOutput{Status: rpc.StateOK}, nil
}

func (s *Server) opStateIncrement(proc *process.Process, params json.RawMessage) (any, error) {
	in, err := stateInput(proc, params, false)
	if err != nil {
		return nil, err
	}
	value, err := statestore.Increment(in.Key)
	if err != nil {
		return stateFailure(err, rpc.StatePersistFailed), nil
	}
	return rpc.StateOutput{Status: rpc.StateOK, Value: value}, nil
}

func (s *Server) opStateList(proc *process.Process, params json.RawMessage) (any, error) {
	in, err := stateInput(proc, params, true)
	if err != nil {
		return nil, err
	}
	keys, err := statestore.PluginList(proc.Name(), in.Key)
	if err != nil {
		return stateFailure(err, rpc.StateReadFailed), nil
	}
	if len(keys) > rpc.StateListMax {
		return stateFailure(errors.New("state list exceeds key limit"), rpc.StateReadFailed), nil
	}
	for _, key := range keys {
		if len(key) > rpc.StateKeyMax {
			return stateFailure(errors.New("stored key exceeds state key limit"), rpc.StateReadFailed), nil
		}
	}
	return rpc.StateOutput{Status: rpc.StateOK, Keys: keys}, nil
}
