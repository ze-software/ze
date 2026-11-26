// Design: docs/architecture/core-design.md -- a service whose configuration cannot be served is not started, and never stops ze
// Related: register_l2tp.go -- registerBNGSubsystems, the independent BNG registration path under test
// Related: service_not_started.go -- serviceNameL2TP, serviceNamePPPoE, blockUnservable, buildFailed

//go:build ze_l2tp

package hub

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/engine"
	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe"
	"github.com/ze-software/ze/pkg/ze"
)

const testBNGConfigPath = "/etc/ze/ze.conf"

type bngTestStubSubsystem struct {
	name string
}

func (s *bngTestStubSubsystem) Name() string { return s.name }
func (*bngTestStubSubsystem) Start(context.Context, ze.EventBus, ze.ConfigProvider) error {
	return nil
}
func (*bngTestStubSubsystem) Stop(context.Context) error                      { return nil }
func (*bngTestStubSubsystem) Reload(context.Context, ze.ConfigProvider) error { return nil }

func validL2TPTree() *zeconfig.Tree {
	tree := zeconfig.NewTree()
	tree.GetOrCreateContainer(bngBlockL2TP).Set("enabled", "true")
	return tree
}

func invalidL2TPTree() *zeconfig.Tree {
	tree := zeconfig.NewTree()
	tree.GetOrCreateContainer(bngBlockL2TP).Set("auth-method", "none")
	return tree
}

func validPPPoEMap() map[string]any {
	return map[string]any{
		bngBlockPPPoE: map[string]any{
			"enabled": true,
		},
	}
}

func invalidPPPoEMap() map[string]any {
	return map[string]any{
		bngBlockPPPoE: map[string]any{
			"auth-method": "none",
		},
	}
}

// requireBNGSubsystemRegistered probes eng by attempting to register a stub
// under name: Engine.RegisterSubsystem refuses a name that is already
// registered and accepts one that is not. Call only after registerBNGSubsystems
// has returned.
func requireBNGSubsystemRegistered(t *testing.T, eng *engine.Engine, name string, want bool) {
	t.Helper()
	err := eng.RegisterSubsystem(&bngTestStubSubsystem{name: name})
	if want {
		require.Error(t, err, "subsystem %q should already be registered on the engine", name)
		require.Contains(t, err.Error(), "already registered")
		return
	}
	require.NoError(t, err, "subsystem %q should not have registered on the engine", name)
}

func TestRegisterBNGSubsystemsInvalidL2TPLeavesPPPoERegistered(t *testing.T) {
	// VALIDATES: an unservable l2tp block (auth-method none without
	// allow-no-auth true) leaves only the L2TP subsystem in down with its
	// block name and config path, advertises no L2TP portal, and still
	// registers the PPPoE subsystem on the engine.
	// PREVENTS: an L2TP parse error returning early before PPPoE registers or
	// stopping daemon startup.
	eng := engine.NewEngine(nil, nil, nil)

	portals, down := registerBNGSubsystems(invalidL2TPTree(), validPPPoEMap(), testBNGConfigPath, eng)

	require.Empty(t, portals)
	require.Len(t, down, 1)
	require.Equal(t, serviceNameL2TP, down[0].service)
	require.Contains(t, down[0].effect, "the L2TP subsystem is not started")
	require.Contains(t, down[0].problem.Problem(), "`l2tp` cannot be served as written")
	require.Contains(t, down[0].problem.Fix(), testBNGConfigPath)
	require.Error(t, down[0].problem.Unwrap())
	requireBNGSubsystemRegistered(t, eng, l2tp.SubsystemName, false)
	requireBNGSubsystemRegistered(t, eng, pppoe.SubsystemName, true)
}

func TestRegisterBNGSubsystemsInvalidPPPoELeavesL2TPRegistered(t *testing.T) {
	// VALIDATES: an unservable pppoe block (auth-method none without
	// allow-no-auth true) leaves only the PPPoE subsystem in down with its
	// block name and config path, while valid L2TP still registers on the
	// engine and returns the L2TP web portal.
	// PREVENTS: a PPPoE parse error discarding the L2TP portal or stopping
	// daemon startup.
	eng := engine.NewEngine(nil, nil, nil)

	portals, down := registerBNGSubsystems(validL2TPTree(), invalidPPPoEMap(), testBNGConfigPath, eng)

	require.Equal(t, []webPortalService{{Key: bngBlockL2TP, Title: "L2TP Sessions", Path: "/l2tp"}}, portals)
	require.Len(t, down, 1)
	require.Equal(t, serviceNamePPPoE, down[0].service)
	require.Contains(t, down[0].effect, "the PPPoE subsystem is not started")
	require.Contains(t, down[0].problem.Problem(), "`pppoe` cannot be served as written")
	require.Contains(t, down[0].problem.Fix(), testBNGConfigPath)
	require.ErrorIs(t, down[0].problem, pppoe.ErrAuthMethodNoneRequiresAllow)
	requireBNGSubsystemRegistered(t, eng, l2tp.SubsystemName, true)
	requireBNGSubsystemRegistered(t, eng, pppoe.SubsystemName, false)
}

func TestRegisterBNGSubsystemsBothInvalidRegistersNeither(t *testing.T) {
	// VALIDATES: when both l2tp and pppoe blocks are unservable, both are
	// reported in down in order (L2TP then PPPoE), no portals are returned,
	// and neither subsystem is registered on the engine.
	// PREVENTS: stopping after the first BNG parse error and hiding the second
	// subsystem's refusal from logs and ze doctor.
	eng := engine.NewEngine(nil, nil, nil)

	portals, down := registerBNGSubsystems(invalidL2TPTree(), invalidPPPoEMap(), testBNGConfigPath, eng)

	require.Empty(t, portals)
	require.Len(t, down, 2)
	require.Equal(t, serviceNameL2TP, down[0].service)
	require.Contains(t, down[0].problem.Problem(), "`l2tp`")
	require.Contains(t, down[0].problem.Fix(), testBNGConfigPath)
	require.Equal(t, serviceNamePPPoE, down[1].service)
	require.Contains(t, down[1].problem.Problem(), "`pppoe`")
	require.Contains(t, down[1].problem.Fix(), testBNGConfigPath)
	requireBNGSubsystemRegistered(t, eng, l2tp.SubsystemName, false)
	requireBNGSubsystemRegistered(t, eng, pppoe.SubsystemName, false)
}

func TestRegisterBNGSubsystemsDuplicateRegistrationLeavesOnlyThatSubsystemDown(t *testing.T) {
	// VALIDATES: when Engine.RegisterSubsystem rejects one BNG subsystem (such
	// as a duplicate registration), only that subsystem is recorded in down
	// with buildFailed (and no portal is added for a failed L2TP registration),
	// while the other subsystem still registers.
	// PREVENTS: a registration failure in one BNG subsystem blocking the other
	// or advertising a portal for an unregistered L2TP subsystem.
	t.Run("duplicate l2tp leaves only L2TP down and registers PPPoE", func(t *testing.T) {
		eng := engine.NewEngine(nil, nil, nil)
		require.NoError(t, eng.RegisterSubsystem(&bngTestStubSubsystem{name: l2tp.SubsystemName}))

		portals, down := registerBNGSubsystems(validL2TPTree(), validPPPoEMap(), testBNGConfigPath, eng)

		require.Empty(t, portals)
		require.Len(t, down, 1)
		require.Equal(t, serviceNameL2TP, down[0].service)
		require.Contains(t, down[0].problem.Problem(), "The L2TP subsystem could not be started.")
		require.Contains(t, down[0].problem.Fix(), "`l2tp`")
		require.Contains(t, down[0].problem.Unwrap().Error(), "already registered")
		requireBNGSubsystemRegistered(t, eng, pppoe.SubsystemName, true)
	})

	t.Run("duplicate pppoe leaves only PPPoE down and registers L2TP", func(t *testing.T) {
		eng := engine.NewEngine(nil, nil, nil)
		require.NoError(t, eng.RegisterSubsystem(&bngTestStubSubsystem{name: pppoe.SubsystemName}))

		portals, down := registerBNGSubsystems(validL2TPTree(), validPPPoEMap(), testBNGConfigPath, eng)

		require.Equal(t, []webPortalService{{Key: bngBlockL2TP, Title: "L2TP Sessions", Path: "/l2tp"}}, portals)
		require.Len(t, down, 1)
		require.Equal(t, serviceNamePPPoE, down[0].service)
		require.Contains(t, down[0].problem.Problem(), "The PPPoE subsystem could not be started.")
		require.Contains(t, down[0].problem.Fix(), "`pppoe`")
		require.Contains(t, down[0].problem.Unwrap().Error(), "already registered")
		requireBNGSubsystemRegistered(t, eng, l2tp.SubsystemName, true)
	})
}
