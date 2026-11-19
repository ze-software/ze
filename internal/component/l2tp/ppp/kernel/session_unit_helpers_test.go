// Design: docs/research/l2tpv2-ze-integration.md -- explicitly logical test owners

package kernel

import (
	"errors"
	"io"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// All existing logical-descriptor fixtures still reach the actual constructor.
// Their already-constructed Unit is mandatory; this adapter never creates an
// owner from labels or supplies one when absent. It only preserves their
// existing wrap/close fault injections on BOTH lanes.
func sessionTestNewSession(cfg SessionConfig, ops sessionOps) (ppp.FrameChannel, ppp.SessionHandle, error) {
	if !nilSessionDependency(cfg.Unit) {
		if _, logical := cfg.Unit.(*sessionTestUnitSource); !logical {
			panic("logical session fixture cannot adapt a production unit owner")
		}
	}
	if !nilSessionDependency(cfg.Channel) {
		if _, logical := cfg.Channel.(*sessionTestChannelSource); !logical {
			panic("logical fixture cannot adapt a production channel owner")
		}
	}
	if logical, ok := cfg.Unit.(*sessionTestUnitSource); ok {
		if logical.info.mutations == nil {
			logical.info.mutations = sessionTestMutationOperations{backend: cfg.Backend, identity: logical.info.identity, ops: &ops}
		}
	}
	ops.wrapChannel = func(lease sessionChannelLease) (io.ReadWriteCloser, error) {
		return ops.wrap(lease.descriptor(), "ppp.chan")
	}
	ops.closeChannel = func(lease sessionChannelLease, stream io.ReadWriteCloser) error {
		return closeSessionStream(stream, lease.descriptor(), ops.closeFD)
	}
	ops.wrapUnit = func(lease sessionUnitLease) (io.ReadWriteCloser, error) {
		return ops.wrap(lease.descriptor(), "ppp.unit")
	}
	ops.closeUnit = sessionTestUnitCloser(ops.closeFD)
	ops.preparePair = func(*claimedSessionPair) error { return nil }
	ops.connectPair = func(pair *claimedSessionPair) error { return ops.connect(pair.channel.descriptor(), pair.info.number) }
	ops.disconnectPair = func(pair *claimedSessionPair) error { return ops.disconnect(pair.channel.descriptor()) }
	frames, handle, err := newSession(cfg, ops)
	if owner, admitted := handle.(*session); admitted {
		if operations, logical := owner.mutations.(sessionTestMutationOperations); logical {
			// Preserve fault hooks changed on the actual logical owner after admission.
			operations.ops = &owner.ops
			owner.mutations = operations
		}
	}
	return frames, handle, err
}

// This type exists only in kernel's internal tests. Its logical descriptors
// are never os.NewFile inputs, and it makes no native PPP counter claim.
// The real public NewSession route is tested separately with UnitCreator.
type sessionTestUnitSource struct {
	info           sessionUnitDescription
	readCounters   func() (ppp.SessionCounters, error)
	describeErr    error
	describePanic  any
	beforeDescribe func()
	claimed        bool
	wrapFile       func(int, string) (io.ReadWriteCloser, error)
	closeFile      func(int) error
}

type sessionTestUnitLease struct{ source *sessionTestUnitSource }

func newSessionTestUnit(fd, number int, identity ppp.SessionIdentity) *sessionTestUnitSource {
	return &sessionTestUnitSource{info: sessionUnitDescription{fd: fd, number: number, identity: identity, namespace: sessionFixtureNamespace},
		readCounters: func() (ppp.SessionCounters, error) {
			return ppp.SessionCounters{RxBytes: 11, TxBytes: 13, RxPackets: 17, TxPackets: 19}, nil
		}}
}

func sessionTestUnit(cfg SessionConfig) *sessionTestUnitSource {
	unit, ok := cfg.Unit.(*sessionTestUnitSource)
	if !ok || unit == nil {
		panic("kernel fixture requires its explicitly constructed logical unit owner")
	}
	return unit
}

func (unit *sessionTestUnitSource) claimSession() (sessionUnitLease, error) {
	if unit.claimed {
		return nil, errors.New("logical unit owner already consumed")
	}
	unit.claimed = true
	return &sessionTestUnitLease{source: unit}, nil
}

func (lease *sessionTestUnitLease) describe() (sessionUnitDescription, error) {
	if lease.source.beforeDescribe != nil {
		lease.source.beforeDescribe()
	}
	if lease.source.describePanic != nil {
		panic(lease.source.describePanic)
	}
	if lease.source.readCounters == nil {
		return sessionUnitDescription{}, errors.New("logical original counter provider unavailable")
	}
	return lease.source.info, lease.source.describeErr
}

func (lease *sessionTestUnitLease) descriptor() int { return lease.source.info.fd }
func (lease *sessionTestUnitLease) aliasesChannel(fd int) bool {
	return fd >= 0 && fd == lease.descriptor()
}

func (lease *sessionTestUnitLease) wrap() (io.ReadWriteCloser, error) {
	if lease.source.wrapFile == nil {
		return nil, errors.New("logical unit requires an explicit test wrapper")
	}
	return lease.source.wrapFile(lease.descriptor(), "ppp.unit")
}

func (lease *sessionTestUnitLease) closeStream(stream io.ReadWriteCloser) error {
	if stream == nil && lease.source.closeFile == nil {
		return errors.New("logical unit requires an explicit test closer")
	}
	return closeSessionStream(stream, lease.descriptor(), lease.source.closeFile)
}

func (lease *sessionTestUnitLease) Counters() (ppp.SessionCounters, error) {
	if lease.source.readCounters == nil {
		return ppp.SessionCounters{}, errors.New("logical original counter provider unavailable")
	}
	return lease.source.readCounters()
}

func sessionTestUnitCloser(closeFD func(int) error) func(sessionUnitLease, io.ReadWriteCloser) error {
	return func(lease sessionUnitLease, stream io.ReadWriteCloser) error {
		return closeSessionStream(stream, lease.descriptor(), closeFD)
	}
}

type sessionTestNamespace struct{ id int }

var sessionFixtureNamespace = &sessionTestNamespace{id: 1}

func (namespace *sessionTestNamespace) sameSessionNamespace(other sessionNamespaceProof) error {
	if namespace != other {
		return errors.New("logical channel/unit namespace mismatch")
	}
	return nil
}

type sessionTestChannelSource struct {
	fd      int
	claimed bool
}
type sessionTestChannelLease struct{ source *sessionTestChannelSource }

func sessionTestChannel(cfg SessionConfig) *sessionTestChannelSource {
	channel, ok := cfg.Channel.(*sessionTestChannelSource)
	if !ok || channel == nil {
		panic("fixture requires its explicitly constructed logical channel owner")
	}
	return channel
}
func (channel *sessionTestChannelSource) claimSessionChannel() (sessionChannelLease, error) {
	if channel.claimed {
		return nil, errors.New("logical channel owner already consumed")
	}
	channel.claimed = true
	return &sessionTestChannelLease{source: channel}, nil
}
func (lease *sessionTestChannelLease) descriptor() int { return lease.source.fd }
func (lease *sessionTestChannelLease) describeChannel() (int, error) {
	if lease.source.fd < 0 {
		return -1, errors.New("logical channel absent")
	}
	return lease.source.fd, nil
}
func (lease *sessionTestChannelLease) namespaceProof() sessionNamespaceProof {
	return sessionFixtureNamespace
}
func (lease *sessionTestChannelLease) wrapChannel() (io.ReadWriteCloser, error) {
	return nil, errors.New("logical channel requires explicit test wrapper")
}
func (lease *sessionTestChannelLease) closeStream(io.ReadWriteCloser) error {
	return errors.New("logical channel requires explicit test closer")
}
