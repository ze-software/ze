// Design: docs/architecture/web-interface.md -- separate transport and route cleanup causes.
package web

import "errors"

// serverShutdownFailure retains both cleanup phases without classifying an
// owned handler's failure as a socket failure. Its zero value is invalid;
// newServerShutdownFailure constructs it only for a nonempty failure.
type serverShutdownFailure struct {
	network error
	owners  error
}

func newServerShutdownFailure(network, owners error) error {
	if network == nil {
		if owners == nil {
			return nil
		}
	}
	return &serverShutdownFailure{network: network, owners: owners}
}

func (failure *serverShutdownFailure) Error() string {
	return errors.Join(failure.network, failure.owners).Error()
}

func (failure *serverShutdownFailure) Unwrap() error {
	return errors.Join(failure.network, failure.owners)
}

// ShutdownNetworkError selects the original network cause after Shutdown has
// finished joining all requests and owned handlers. Its caller MUST use the
// complete Shutdown result for propagation; this projection is only for a
// network-specific diagnostic. A route-only cleanup failure returns nil.
func ShutdownNetworkError(err error) error {
	if failure, ok := errors.AsType[*serverShutdownFailure](err); ok {
		return failure.network
	}
	return err
}
