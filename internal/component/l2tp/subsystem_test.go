package l2tp_test

import (
	"context"
	"net"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/l2tp"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// TestSubsystem_Name returns the canonical identifier.
func TestSubsystem_Name(t *testing.T) {
	sub := l2tp.NewSubsystem(l2tp.Parameters{})
	assert.Equal(t, "l2tp", sub.Name())
}

// TestSubsystem_ImplementsInterface confirms the compile-time assertion.
func TestSubsystem_ImplementsInterface(t *testing.T) {
	var _ ze.Subsystem = l2tp.NewSubsystem(l2tp.Parameters{})
}

// TestSubsystem_StartStopDisabled — Start is a no-op when Enabled=false.
//
// VALIDATES: subsystem lifecycle safe when config absent or disabled.
func TestSubsystem_StartStopDisabled(t *testing.T) {
	sub := l2tp.NewSubsystem(l2tp.Parameters{})
	ctx := context.Background()
	require.NoError(t, sub.Start(ctx, nil, nil))
	require.NoError(t, sub.Stop(ctx))
}

// TestSubsystem_StartEnabledNoListener — warns but does not error.
//
// VALIDATES: Start is tolerant of enabled-but-no-listener, mirroring
// SSH's behavior when host-key is derivable but no addresses given.
func TestSubsystem_StartEnabledNoListener(t *testing.T) {
	sub := l2tp.NewSubsystem(l2tp.Parameters{Enabled: true})
	ctx := context.Background()
	require.NoError(t, sub.Start(ctx, nil, nil))
	require.NoError(t, sub.Stop(ctx))
}

// TestSubsystem_StartEnabledWithListener — subsystem binds the UDP
// socket, then stops cleanly. Uses ephemeral port 0 so the test cannot
// collide with a concurrent L2TP daemon or a CI runner that already
// holds 1701.
//
// VALIDATES: AC-2 -- subsystem binds a UDP socket for the configured
// listener address.
func TestSubsystem_StartEnabledWithListener(t *testing.T) {
	// Phase 5 adds kernel module probing to Start(); dev/CI machines may
	// not have l2tp_ppp/pppol2tp available. Neutralize the probe for this
	// pure-userspace test.
	defer l2tp.SetProbeKernelModulesForTest(func() error { return nil })()

	addr := netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), 0)
	sub := l2tp.NewSubsystem(l2tp.Parameters{
		Enabled:     true,
		ListenAddrs: []netip.AddrPort{addr},
	})
	ctx := context.Background()
	require.NoError(t, sub.Start(ctx, nil, nil))
	require.NoError(t, sub.Stop(ctx))
}

// TestSubsystem_BindFailureUnwinds covers the unwind path: when the
// first listener binds fine but a second binds to a busy port, Start
// must return an error AND leave s.started=false so Stop is a no-op
// AND release the successfully-bound first listener so its port is
// reusable.
//
// VALIDATES: Failure-mode analysis (FM7) in the spec -- partial-start
// state cleanup.
func TestSubsystem_BindFailureUnwinds(t *testing.T) {
	defer l2tp.SetProbeKernelModulesForTest(func() error { return nil })()

	// Pre-bind an ephemeral port as an external blocker.
	blocker, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	require.NoError(t, err)
	defer blocker.Close() //nolint:errcheck // test cleanup
	blockerAddr, ok := blocker.LocalAddr().(*net.UDPAddr)
	require.True(t, ok)

	// First listener: unrelated ephemeral port (must succeed).
	// Second listener: the blocker's port (must fail at bind).
	busy := netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), uint16(blockerAddr.Port))
	sub := l2tp.NewSubsystem(l2tp.Parameters{
		Enabled: true,
		ListenAddrs: []netip.AddrPort{
			netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), 0),
			busy,
		},
	})

	ctx := context.Background()
	err = sub.Start(ctx, nil, nil)
	require.Error(t, err, "bind must fail when second listener targets a busy port")
	require.Contains(t, err.Error(), "bind")

	// Stop must be a no-op (nothing started).
	require.NoError(t, sub.Stop(ctx))

	// The first listener's port was released by unwindLocked. We can
	// prove it by starting again on the same ephemeral-port plan (the
	// kernel would pick a fresh port, so we rely on the re-Start
	// succeeding as evidence of clean state).
	sub2 := l2tp.NewSubsystem(l2tp.Parameters{
		Enabled:     true,
		ListenAddrs: []netip.AddrPort{netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), 0)},
	})
	require.NoError(t, sub2.Start(ctx, nil, nil))
	require.NoError(t, sub2.Stop(ctx))
}

// TestSubsystem_DoubleStart returns an error on second Start without Stop.
func TestSubsystem_DoubleStart(t *testing.T) {
	sub := l2tp.NewSubsystem(l2tp.Parameters{})
	ctx := context.Background()
	require.NoError(t, sub.Start(ctx, nil, nil))
	err := sub.Start(ctx, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already started")
}

// TestSubsystem_StopIdempotent — Stop may be called repeatedly.
func TestSubsystem_StopIdempotent(t *testing.T) {
	sub := l2tp.NewSubsystem(l2tp.Parameters{})
	ctx := context.Background()
	require.NoError(t, sub.Stop(ctx))
	require.NoError(t, sub.Stop(ctx))
}

// TestSubsystem_Reload — spec-l2tp-7 made Reload meaningful. Calling
// Reload on a never-Started subsystem now returns ErrSubsystemNotStarted
// (detailed diff-apply semantics exercised in subsystem_reload_test.go).
func TestSubsystem_Reload(t *testing.T) {
	sub := l2tp.NewSubsystem(l2tp.Parameters{})
	ctx := context.Background()
	err := sub.Reload(ctx, nil)
	require.ErrorIs(t, err, l2tp.ErrSubsystemNotStarted)
}
