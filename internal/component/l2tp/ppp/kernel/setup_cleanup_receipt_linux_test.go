// Design: docs/architecture/l2tp/subscriber-session-model.md -- partial PPP setup cleanup
//go:build linux

package kernel

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDevPPPSetupRollbackRetainsCloseFailures(t *testing.T) {
	setupFailure := errors.New("controlled unit allocation refusal")
	closeFailure := errors.New("original channel or unit close unconfirmed")
	var closed []int
	next := 9
	_, _, _, err := devPPPSetup(7, "", devPPPSetupOps{
		get:     func(int, uint) (int, error) { return 8, nil },
		set:     func(int, uint, int) error { return nil },
		getSet:  func(int, uint, int) (int, error) { return -1, setupFailure },
		open:    func() (int, error) { next++; return next, nil },
		closeFD: func(fd int) error { closed = append(closed, fd); return closeFailure },
	})
	require.ErrorIs(t, err, setupFailure)
	require.ErrorIs(t, err, closeFailure)
	require.Equal(t, []int{11, 10}, closed)
}
