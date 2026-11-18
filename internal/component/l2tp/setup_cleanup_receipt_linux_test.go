// Design: docs/architecture/l2tp/subscriber-session-model.md -- setup rollback outcomes
//go:build linux

package l2tp

import (
	"errors"
	"net/netip"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
)

func TestPPPoXSetupRollbackRetainsCloseFailure(t *testing.T) {
	connectFailure := errors.New("controlled connect refusal")
	closeFailure := errors.New("original socket close unconfirmed")
	closes := 0
	_, err := pppoxCreateWith(7, netip.MustParseAddrPort("192.0.2.1:1701"), 1, 2, 3, 4,
		func() (int, error) { return 10, nil },
		func(int, []byte) error { return connectFailure },
		func(fd int) error { require.Equal(t, 10, fd); closes++; return closeFailure })
	require.ErrorIs(t, err, connectFailure)
	require.ErrorIs(t, err, closeFailure)
	require.Equal(t, 1, closes)
}

func TestPPPTransportSetupRollbackRetainsCloseFailure(t *testing.T) {
	setupFailure := errors.New("controlled PPP channel refusal")
	closeFailure := errors.New("original PPPoX close unconfirmed")
	closes := 0
	_, err := pppSetupWith(kernelSetupEvent{}, nil, kernelPPPSetupOperations{
		create: func(int, netip.AddrPort, uint16, uint16, uint16, uint16) (*os.File, int, error) { return nil, 10, nil },
		setup: func(*pppkernel.UnitCreator, *os.File, string) (*pppkernel.CreatedChannel, *pppkernel.CreatedUnit, error) {
			return nil, nil, setupFailure
		},
		close: closeLogicalPPPForTest(func(fd int) error { require.Equal(t, 10, fd); closes++; return closeFailure }),
	})
	require.ErrorIs(t, err, setupFailure)
	require.ErrorIs(t, err, closeFailure)
	require.Equal(t, 1, closes)
}

func TestKernelSetupRetainsLowerRollbackCloseUncertainty(t *testing.T) {
	worker, _, event := workerCleanupBaseline(t)
	setupFailure := errors.New("controlled PPP setup refusal")
	closeFailure := errors.New("lower setup descriptor close unconfirmed")
	closes := 0
	worker.ops.pppSetup = func(setup kernelSetupEvent) (kernelPPPSetup, error) {
		return func() (pppSessionFDs, error) {
			return pppSetupWith(setup, nil, kernelPPPSetupOperations{
				create: func(int, netip.AddrPort, uint16, uint16, uint16, uint16) (*os.File, int, error) { return nil, 10, nil },
				setup: func(*pppkernel.UnitCreator, *os.File, string) (*pppkernel.CreatedChannel, *pppkernel.CreatedUnit, error) {
					return nil, nil, setupFailure
				},
				close: closeLogicalPPPForTest(func(int) error { closes++; return closeFailure }),
			})
		}, nil
	}
	event.lnsMode = false
	worker.setupSession(event)
	key := sessionKey{event.localTID, event.localSID}
	owner := worker.sessions[key]
	require.NotNil(t, owner, "lower rollback uncertainty must retain the original cleanup root")
	worker.teardownAll()
	require.Same(t, owner, worker.sessions[key])
	require.Equal(t, 1, closes, "no cleanup sweep may retry a consumed raw number")
	require.ErrorIs(t, worker.originalCleanupStatus(event.session), closeFailure)
}
