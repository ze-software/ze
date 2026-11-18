// Design: docs/research/l2tpv2-ze-integration.md -- one captured-pair admission.
// Related: session_unit_owned.go, created_channel.go -- sealed native claims.

package kernel

import (
	"errors"
	"fmt"
	"io"
	"sync"
)

// claimedSessionPair is private custody shared by both consumers. Its zero
// value is only constructor rollback state, never an admitted session. The
// lifecycle mutex precedes unit.mu, channel.mu and both original RawConns.
// Server s.mu and client sessionLink.mu precede this mutex. Public original
// owner Close takes no pair mutex. No public descriptor callback waits here.
type claimedSessionPair struct {
	mu            sync.Mutex
	channel       sessionChannelLease
	unit          sessionUnitLease
	info          sessionUnitDescription
	cfg           SessionConfig
	original      *originalPairReceipt
	channelStream io.ReadWriteCloser
	unitStream    io.ReadWriteCloser
	closers       sessionPairClosers
	channelClosed bool
	unitClosed    bool
	channelErr    error
	unitErr       error
}

type sessionPairClosers struct {
	channel func(sessionChannelLease, io.ReadWriteCloser) error
	unit    func(sessionUnitLease, io.ReadWriteCloser) error
}

// Both independent claims run before either error or panic is reported. Each
// caller MUST defer finishConstruction until it transfers the complete pair.
func newClaimedSessionPair(cfg SessionConfig, preflight func(SessionConfig) error,
	prepare func(*claimedSessionPair) error, closers sessionPairClosers,
) (pair *claimedSessionPair, err error) {
	pair = &claimedSessionPair{cfg: cfg, closers: closers}
	owned := pair
	transferred := false
	defer owned.finishConstruction(&transferred, &err)
	var channelErr, unitErr error
	var channelPanic, unitPanic any
	owned.channel, channelErr, channelPanic = claimSessionChannel(cfg.Channel)
	owned.unit, unitErr, unitPanic = claimSessionUnit(cfg.Unit)
	if failure := sessionClaimPanic(channelPanic, unitPanic); failure != nil {
		panic(failure)
	}
	if err := errors.Join(channelErr, unitErr); err != nil {
		return nil, err
	}
	if preflight != nil {
		if err := preflight(cfg); err != nil {
			return nil, err
		}
	}
	channelFD, err := owned.channel.describeChannel()
	if err != nil {
		return nil, fmt.Errorf("ppp: original channel: %w", err)
	}
	owned.info, err = owned.unit.describe()
	if err != nil {
		return nil, fmt.Errorf("ppp: original unit: %w", err)
	}
	if err := validateSessionNamespace(owned.channel.namespaceProof(), owned.info.namespace); err != nil {
		return nil, err
	}
	if channelFD != cfg.ChanFD {
		return nil, errors.New("ppp: channel label differs from captured original")
	}
	if err := validateSessionConfig(cfg, owned.info); err != nil {
		return nil, err
	}
	if prepare != nil {
		if err := prepare(owned); err != nil {
			return nil, err
		}
	}
	transferred = true
	return owned, nil
}

// This is a direct deferred call so recover observes the original panic. Both
// resource closes are panic-safe through the existing Stage-B close helpers.
func (pair *claimedSessionPair) finishConstruction(transferred *bool, err *error) {
	if *transferred {
		return
	}
	failure := recover()
	cleanupErr := pair.closeAll()
	if failure != nil {
		if cleanupErr != nil {
			panic(RecordSetupCleanup(sessionPanicFailure(failure), cleanupErr))
		}
		panic(failure)
	}
	*err = RecordSetupCleanup(*err, cleanupErr)
}

func (pair *claimedSessionPair) closeChannel() error {
	if pair.channelClosed {
		return pair.channelErr
	}
	pair.channelClosed = true
	if distinctSessionChannel(pair.channel, pair.unit) {
		pair.channelErr = closeOriginalSessionFile(func() error {
			return closeOwnedSessionChannel(pair.channel, pair.channelStream, pair.closers.channel)
		})
	}
	return pair.channelErr
}

func (pair *claimedSessionPair) closeUnit() error {
	if pair.unitClosed {
		return pair.unitErr
	}
	pair.unitClosed = true
	if pair.unit != nil {
		pair.unitErr = closeOriginalSessionFile(func() error {
			return closeOwnedSessionUnit(pair.unit, pair.unitStream, pair.closers.unit)
		})
	}
	return pair.unitErr
}

func (pair *claimedSessionPair) closeAll() error {
	pair.mu.Lock()
	defer pair.mu.Unlock()
	return errors.Join(pair.closeChannel(), pair.closeUnit())
}
