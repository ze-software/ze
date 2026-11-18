// Design: docs/architecture/api/process-protocol.md -- destination-constrained command dispatch.
package rpc

import (
	"errors"
	"time"
)

func (input DispatchCommandInput) DestinationBound() bool {
	if input.DestinationOwner != 0 {
		return true
	}
	return input.DestinationSession != 0
}
func (input DispatchCommandInput) ValidateDestination(now time.Time) error {
	if input.DestinationOwner == 0 {
		return errors.New("destination command has no original owner; preserve the captured peer UP")
	}
	if input.DestinationSession == 0 {
		return errors.New("destination command has no original session; preserve the captured peer UP")
	}
	if input.PeerOwner != 0 {
		return errors.New("destination command carries physical owner constraints; supply only its destination tuple")
	}
	if input.ConnectionID != 0 {
		return errors.New("destination command carries a physical connection; supply only its destination tuple")
	}
	if input.Command == "" {
		return errors.New("destination command is empty; send its registered command")
	}
	if input.DeadlineMS <= now.UnixMilli() {
		return errors.New("destination command deadline expired; use a live bounded deadline")
	}
	if input.DeadlineMS > now.Add(CancellableRequestTimeout).UnixMilli() {
		return errors.New("destination command deadline exceeds thirty seconds; use a bounded request")
	}
	return nil
}
func (output *DispatchCommandOutput) ValidateDestinationAnswer(input DispatchCommandInput) error {
	if output == nil {
		return errors.New("destination command has no completion receipt; return its settled original tuple")
	}
	if output.DestinationOwner != input.DestinationOwner {
		return errors.New("destination command receipt owner differs; return the original settled owner")
	}
	if output.DestinationSession != input.DestinationSession {
		return errors.New("destination command receipt session differs; return the original settled session")
	}
	if output.DeadlineMS != input.DeadlineMS {
		return errors.New("destination command receipt deadline differs; return the original settled deadline")
	}
	return nil
}
