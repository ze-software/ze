// Design: docs/research/l2tpv2-ze-integration.md -- synthetic actor fault model
// Related: ppp_session_fixture_linux_test.go -- explicitly owned test streams

//go:build linux

package l2tp

import (
	"errors"
	"os"

	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
)

// logicalPPPConfigForTest snapshots the declared logical stream fixture. It
// never constructs a valid kernel CreatedChannel/CreatedUnit or invokes the
// real public kernel NewSession. Its only consumer is an explicit private
// protocol constructor which owns recorder tokens or the fixture's own files.
func logicalPPPConfigForTest(original *pppSessionFDs) func(*kernelPPPOwners, pppkernel.InterfaceOperations) (pppkernel.SessionConfig, error) {
	original.ppp = &kernelPPPOwners{}
	resources := original.ppp
	config := pppkernel.SessionConfig{ChanFD: original.chanFD, UnitFD: original.unitFD, UnitNum: original.unitNum}
	return func(actual *kernelPPPOwners, _ pppkernel.InterfaceOperations) (pppkernel.SessionConfig, error) {
		if actual != resources {
			return pppkernel.SessionConfig{}, errors.New("logical stream fixture received another original")
		}
		return config, nil
	}
}

// closeLogicalPPPForTest adapts recorder tokens or fixture-owned files into
// the SAME cleanup traversal used by closeOriginalKernelPPP. The fixture owns
// no traversal, ordering, alias suppression or exception handling. Tokens
// never construct valid public CreatedChannel/CreatedUnit grants.
func closeLogicalPPPForTest(closeFD func(int) error) func(*kernelPPPOwners, *os.File, [3]int) [3]error {
	return func(_ *kernelPPPOwners, _ *os.File, labels [3]int) [3]error {
		var operations [3]kernelPPPCloseOperation
		for index, fd := range labels {
			if fd < 0 {
				continue
			}
			operations[index] = kernelPPPCloseOperation{identity: fd, close: func() error { return closeFD(fd) }}
		}
		return closeKernelPPPResources(operations)
	}
}
