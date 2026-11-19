// Design: docs/architecture/l2tp/subscriber-session-model.md -- exact rate ownership
// Related: session.go -- pure subscriber snapshots; handler_registry.go -- handlers

package subscriber

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/grammar"
)

// RateOwnerCommand is a validated, immutable command declaration. A snapshot
// carries Command's string value across a plugin boundary, never this local
// capability or a controller. Consumers MUST reconstruct a declaration from
// the received value before dispatch; a missing value never selects a default.
type RateOwnerCommand interface {
	Command() string
	rateOwnerCommand()
}

type rateOwnerCommand struct{ command string }

// NewRateOwnerCommand validates the existing command grammar and a bounded
// operational request path. It neither loads a plugin nor selects a backend.
func NewRateOwnerCommand(path string) (RateOwnerCommand, error) {
	const commandBytesMax = 256
	if len(path) > commandBytesMax {
		return nil, errors.New("subscriber rate owner command exceeds 256 bytes")
	}
	if findings := grammar.CheckName(path); len(findings) != 0 {
		return nil, fmt.Errorf("subscriber rate owner command is invalid: %s", findings[0].Message)
	}
	if !strings.HasPrefix(path, command.VerbRequest+" ") {
		return nil, errors.New("subscriber rate owner must declare an operational request command")
	}
	return &rateOwnerCommand{command: path}, nil
}

func (c *rateOwnerCommand) Command() string { return c.command }
func (*rateOwnerCommand) rateOwnerCommand() {}

// Rates holds the complete authorized pair in bits per second. Its private
// implementation can be constructed only by NewRates. An omitted CoA direction
// MUST be resolved against the current accepted profile before construction.
type Rates interface {
	Download() uint64
	Upload() uint64
	rates()
}

type ratePair struct{ download, upload uint64 }

// NewRates refuses absent directions; native units and implementation limits
// remain the selected controller's responsibility. Safe for concurrent use.
func NewRates(download, upload uint64) (Rates, error) {
	if download == 0 {
		return nil, errors.New("subscriber download rate must be positive")
	}
	if upload == 0 {
		return nil, errors.New("subscriber upload rate must be positive")
	}
	return &ratePair{download: download, upload: upload}, nil
}

func (r *ratePair) Download() uint64 { return r.download }
func (r *ratePair) Upload() uint64   { return r.upload }
func (*ratePair) rates()             {}

// RateTransaction is a local borrowed authorization capability. Its caller
// MUST Apply then Commit, or Rollback after any failed multi-owner change.
// The concrete owner MUST reject aliases after the session lifetime ends or
// a newer transaction supersedes them. No method crosses a plugin boundary.
type RateTransaction interface {
	Apply(context.Context) error
	Commit() error
	Rollback(context.Context) error
}

// RateController belongs to one created session. Prepare changes no native
// state and MUST return an owner-bound transaction or an error. A nonnil result
// remains owned by the caller even on error and MUST be rolled back. The session
// retains sole cleanup ownership: a RatePolicy borrows this controller and
// never closes it independently. Safe for concurrent use.
type RateController interface {
	Prepare(context.Context, Rates) (RateTransaction, error)
}

// RatePolicy is a sealed choice between a local session controller and a
// remote command owner. A successful session constructor MUST validate it
// with ValidateRatePolicy before publication. Both variants expose the same
// pure command declaration for subsequent synchronous CoA dispatch.
type RatePolicy interface {
	Owner() RateOwnerCommand
	ratePolicy()
}

// LocalRatePolicy borrows its session's actual controller. Only the session's
// data-plane owner closes native resources; this policy has no Close method.
type LocalRatePolicy interface {
	RatePolicy
	Prepare(context.Context, Rates) (RateTransaction, error)
	localRatePolicy()
}

type localRatePolicy struct {
	owner      RateOwnerCommand
	controller RateController
}

// NewLocalRatePolicy requires both a declared command and its real controller.
// The session constructor MUST retain this policy with its owning handle.
func NewLocalRatePolicy(owner RateOwnerCommand, controller RateController) (LocalRatePolicy, error) {
	if err := validateRateOwner(owner); err != nil {
		return nil, err
	}
	if nilRateCapability(controller) {
		return nil, errors.New("subscriber rate policy has no local controller")
	}
	return &localRatePolicy{owner: owner, controller: controller}, nil
}

func (p *localRatePolicy) Owner() RateOwnerCommand { return p.owner }
func (*localRatePolicy) ratePolicy()               {}
func (*localRatePolicy) localRatePolicy()          {}

// Prepare invokes only this session's borrowed controller, before forwarding
// publication. The caller MUST finish the returned transaction before it
// acknowledges assignment. A nonnil result MUST be rolled back on failure,
// including when this method returns it with an error. No absent or forged rate
// can reach native state.
func (p *localRatePolicy) Prepare(ctx context.Context, rates Rates) (RateTransaction, error) {
	if ctx == nil {
		return nil, errors.New("subscriber rate preparation requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := ValidateRatePolicy(p); err != nil {
		return nil, err
	}
	pair, valid := rates.(*ratePair)
	if !valid {
		return nil, errors.New("subscriber rate preparation requires constructed rates")
	}
	if pair == nil {
		return nil, errors.New("subscriber rate preparation has no rates")
	}
	transaction, err := p.controller.Prepare(ctx, pair)
	if err != nil {
		if nilRateCapability(transaction) {
			return nil, err
		}
		return transaction, err
	}
	if nilRateCapability(transaction) {
		return nil, errors.New("subscriber rate controller returned no prepared transaction")
	}
	return transaction, nil
}

// RemoteRatePolicy carries no local controller. Initial installation and CoA
// MUST reach Owner's command through the authenticated dispatcher and await its
// confirmed result, including when that plugin runs in another process.
type RemoteRatePolicy interface {
	RatePolicy
	remoteRatePolicy()
}

type remoteRatePolicy struct{ owner RateOwnerCommand }

func NewRemoteRatePolicy(owner RateOwnerCommand) (RemoteRatePolicy, error) {
	if err := validateRateOwner(owner); err != nil {
		return nil, err
	}
	return &remoteRatePolicy{owner: owner}, nil
}

func (p *remoteRatePolicy) Owner() RateOwnerCommand { return p.owner }
func (*remoteRatePolicy) ratePolicy()               {}
func (*remoteRatePolicy) remoteRatePolicy()         {}

// ValidateRatePolicy rejects nil, uninitialized variants and an embedding that
// merely satisfies the sealed interface. It is the session publication guard.
func ValidateRatePolicy(policy RatePolicy) error {
	switch p := policy.(type) {
	case *localRatePolicy:
		if p == nil {
			return errors.New("subscriber rate policy is a typed nil")
		}
		if err := validateRateOwner(p.owner); err != nil {
			return err
		}
		if nilRateCapability(p.controller) {
			return errors.New("subscriber rate policy has no local controller")
		}
		return nil
	case *remoteRatePolicy:
		if p == nil {
			return errors.New("subscriber rate policy is a typed nil")
		}
		return validateRateOwner(p.owner)
	default:
		// A plugin may embed the public interface. Only these package-owned
		// constructor variants carry a validated policy, so this set is open.
		return errors.New("subscriber rate policy was not constructed by its owner")
	}
}

func validateRateOwner(owner RateOwnerCommand) error {
	declaration, valid := owner.(*rateOwnerCommand)
	if !valid {
		return errors.New("subscriber rate policy requires a declared command owner")
	}
	if declaration == nil {
		return errors.New("subscriber rate command owner is a typed nil")
	}
	return nil
}

func nilRateCapability(value any) bool {
	if value == nil {
		return true
	}
	dynamic := reflect.ValueOf(value)
	// Capabilities are interfaces supplied by a registered provider. Only
	// nil-capable dynamic kinds have an IsNil operation.
	//exhaustive:ignore
	switch dynamic.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return dynamic.IsNil()
	default:
		return false
	}
}
