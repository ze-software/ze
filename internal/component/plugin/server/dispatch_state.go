// Design: docs/architecture/api/process-protocol.md -- strict plugin state RPCs.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// stateAccess names what a state request does with its key, so the grant it
// needs is chosen by the operation and never by the caller's claim. Zero is
// invalid.
type stateAccess uint8

const (
	stateAccessUnspecified stateAccess = iota
	// stateAccessList enumerates under a prefix; PluginList filters the answer.
	stateAccessList
	// stateAccessRead reads one key: a read grant or a write grant admits it.
	stateAccessRead
	// stateAccessWrite writes, removes or increments one key: only a write
	// grant admits it.
	stateAccessWrite
)

func stateInput(proc *process.Process, params json.RawMessage, access stateAccess) (rpc.StateInput, error) {
	var in rpc.StateInput
	// base64 grows a value by a third, so twice the largest declared bound
	// admits every valid request and refuses a hostile one before decoding.
	if len(params) > 2*rpc.StateDataLargest() {
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
	regName := proc.RegistrationName()
	switch access {
	case stateAccessList:
	case stateAccessRead:
		if regName == "" {
			return in, fmt.Errorf("plugin %s holds no grant to read state key %s", proc.Name(), in.Key)
		}
		if !statestore.PluginKeyReadable(regName, in.Key) {
			return in, fmt.Errorf("plugin %s holds no grant to read state key %s", proc.Name(), in.Key)
		}
	case stateAccessWrite:
		if regName == "" {
			return in, fmt.Errorf("plugin %s holds no grant to write state key %s", proc.Name(), in.Key)
		}
		if !statestore.PluginKeyWritable(regName, in.Key) {
			return in, fmt.Errorf("plugin %s holds no grant to write state key %s", proc.Name(), in.Key)
		}
	case stateAccessUnspecified:
		panic("BUG: state request dispatched with no access kind")
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

func (s *Server) opStateGet(ctx context.Context, proc *process.Process, params json.RawMessage) (any, error) {
	in, err := stateInput(proc, params, stateAccessRead)
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
	if len(data) > rpc.StateDataMaxFor(in.Key) {
		return stateFailure(errors.New("stored value exceeds state data limit"), rpc.StateReadFailed), nil
	}
	return rpc.StateOutput{Status: rpc.StateOK, Data: data}, nil
}

func (s *Server) opStatePut(ctx context.Context, proc *process.Process, params json.RawMessage) (any, error) {
	in, err := stateInput(proc, params, stateAccessWrite)
	if err != nil {
		return nil, err
	}
	if err := statestore.Write(in.Key, in.Data); err != nil {
		return stateFailure(err, rpc.StatePersistFailed), nil
	}
	return rpc.StateOutput{Status: rpc.StateOK}, nil
}

func (s *Server) opStateRemove(ctx context.Context, proc *process.Process, params json.RawMessage) (any, error) {
	in, err := stateInput(proc, params, stateAccessWrite)
	if err != nil {
		return nil, err
	}
	if err := statestore.Delete(in.Key); err != nil {
		return stateFailure(err, rpc.StatePersistFailed), nil
	}
	return rpc.StateOutput{Status: rpc.StateOK}, nil
}

func (s *Server) opStateIncrement(ctx context.Context, proc *process.Process, params json.RawMessage) (any, error) {
	in, err := stateInput(proc, params, stateAccessWrite)
	if err != nil {
		return nil, err
	}
	value, err := statestore.Increment(in.Key)
	if err != nil {
		return stateFailure(err, rpc.StatePersistFailed), nil
	}
	return rpc.StateOutput{Status: rpc.StateOK, Value: value}, nil
}

func (s *Server) opStateList(ctx context.Context, proc *process.Process, params json.RawMessage) (any, error) {
	in, err := stateInput(proc, params, stateAccessList)
	if err != nil {
		return nil, err
	}
	regName := proc.RegistrationName()
	if regName == "" {
		return rpc.StateOutput{Status: rpc.StateOK}, nil
	}
	keys, err := statestore.PluginList(regName, in.Key)
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
