// Design: docs/architecture/l2tp/subscriber-session-model.md -- partial PPP setup cleanup
//go:build linux

package kernel

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDevPPPSetupRollbackRetainsCloseFailures(t *testing.T) {
	setupFailure := errors.New("controlled unit allocation refusal")
	closeFailure := errors.New("original channel or unit close unconfirmed")
	var closed []int
	channel := &CreatedChannel{}
	unit := &CreatedUnit{}
	creator := &UnitCreator{create: func(string) (*CreatedUnit, error) { return unit, setupFailure }}
	_, _, err := devPPPSetup(creator, new(os.File), "", devPPPSetupOps{
		createUnit: func(creator *UnitCreator, _ *CreatedChannel, name string) (*CreatedUnit, error) {
			return creator.Create(name)
		},
		channel: func(*os.File) (*CreatedChannel, error) { return channel, nil },
		closeUnit: func(owner *CreatedUnit) error {
			require.Same(t, unit, owner)
			closed = append(closed, 11)
			return closeFailure
		},
		closeChannel: func(owner *CreatedChannel) error {
			require.Same(t, channel, owner)
			closed = append(closed, 10)
			return closeFailure
		},
		rejected: func(*CreatedUnit) bool { return false },
		pause:    func(time.Duration) { t.Fatal("unnamed failed setup retried") },
	})

	require.ErrorIs(t, err, setupFailure)
	require.ErrorIs(t, err, closeFailure)
	require.Equal(t, []int{11, 10}, closed)
}
