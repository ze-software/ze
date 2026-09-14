// Design: docs/architecture/exabgp-bridge.md -- internal exabgp bridge config
//
// Detail: config.go reads the ADD-PATH modes off the loaded model, so this test
// reads the same leaf by a second route and holds the parser to it.

package bridgeplugin

import (
	"errors"
	"slices"
	"strings"
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/require"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

// modelAddPathModes answers the values of the add-path enumeration, read by
// walking the loaded model rather than through the EnumValues call the parser
// uses, so the two routes to one leaf check each other.
//
// It FAILS rather than answering an empty set for a leaf it cannot reach: a
// parser compared against nothing passes over every drift there is.
func modelAddPathModes(t *testing.T) []string {
	t.Helper()

	loader, err := configyang.DefaultLoader()
	require.NoError(t, err)
	entry := loader.GetEntry("ze-exabgp-bridge-conf")
	require.NotNil(t, entry, "the loaded model holds no module ze-exabgp-bridge-conf: this binary registered nothing to compare against")
	for _, step := range []string{configRoot, bridgeContainer, "add-path"} {
		entry = entry.Dir[step]
		require.NotNil(t, entry, "ze-exabgp-bridge-conf declares no %s on the way to add-path", step)
	}
	require.NotNil(t, entry.Type, "add-path has no type")
	require.Equal(t, gyang.Yenum, entry.Type.Kind, "add-path is not an enumeration")
	require.NotNil(t, entry.Type.Enum, "add-path declares no enumeration values")

	modes := slices.Clone(entry.Type.Enum.Names())
	slices.Sort(modes)
	require.NotEmpty(t, modes)
	return modes
}

// TestParseConfigAddPathReadsTheModel proves the parser accepts exactly the
// ADD-PATH modes the model declares, and that every declared mode other than
// none reaches the wire as a capability.
//
// PREVENTS: a mode added to the model that the parser refuses, so a config the
// schema accepts fails at the bridge; and a mode the parser accepts that the
// encoder does not know, so the bridge silently negotiates no ADD-PATH for a
// mode the operator asked for.
func TestParseConfigAddPathReadsTheModel(t *testing.T) {
	modes := modelAddPathModes(t)
	require.Contains(t, modes, addPathNone, "the model no longer declares the mode the default names")

	for _, mode := range modes {
		cfg, err := parseConfig(`{"exabgp":{"bridge":{"process":{"main":{"run":"./p.py"}},"add-path":"` + mode + `"}}}`)
		require.NoError(t, err, "the model declares add-path %q and parseConfig refuses it", mode)
		require.Equal(t, mode, cfg.AddPath)

		caps := capabilityDecls(cfg)
		if mode == addPathNone {
			require.Empty(t, caps, "add-path none asked for no capability")
			continue
		}
		require.Len(t, caps, 1, "the model declares add-path %q and the bridge encodes no capability for it", mode)
		require.Equal(t, uint8(69), caps[0].Code, "RFC 7911 ADD-PATH capability code")
	}

	// A word no leaf holds is refused, and the refusal offers the model's
	// words rather than a list written in Go.
	_, err := parseConfig(`{"exabgp":{"bridge":{"process":{"main":{"run":"./p.py"}},"add-path":"sideways"}}}`)
	require.Error(t, err)
	require.Contains(t, err.Error(), strings.Join(modes, ", "))
}

// TestParseConfigAddPathRefusesWhenTheModelCannotAnswer proves the parser fails
// closed: a model that answers no mode set refuses every add-path word rather
// than accepting all of them, and the refusal carries the model's reason.
func TestParseConfigAddPathRefusesWhenTheModelCannotAnswer(t *testing.T) {
	answered := addPathModes
	t.Cleanup(func() { addPathModes = answered })
	addPathModes = func() ([]string, error) { return nil, errors.New("the model did not load") }

	_, err := parseConfig(`{"exabgp":{"bridge":{"process":{"main":{"run":"./p.py"}},"add-path":"` + addPathNone + `"}}}`)
	require.Error(t, err, "the model answered nothing and parseConfig accepted a mode anyway")
	require.Contains(t, err.Error(), "the model did not load")

	// A config that states no add-path never asks the model, so it still
	// parses: the default is the leaf's default, not a validated word.
	cfg, err := parseConfig(`{"exabgp":{"bridge":{"process":{"main":{"run":"./p.py"}}}}}`)
	require.NoError(t, err)
	require.Equal(t, addPathNone, cfg.AddPath)
}
