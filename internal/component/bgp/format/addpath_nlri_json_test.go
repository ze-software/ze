package format

import (
	"net"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
)

// wireNLRIIsAddPathAware pins the optional interface the formatter probes for.
// WireNLRI is the carrier ParseNLRIs wraps a plugin family's section in, so if
// it stops answering the question, every plugin family loses the flag at once.
var _ nlri.AddPathAware = (*nlri.WireNLRI)(nil)

// unclaimedFamilyForFormat answers a family the family registry holds that no
// plugin claims in this test binary, so the test can claim it itself. It is
// derived rather than named, because which NLRI plugins a test binary links
// depends on its build tags.
func unclaimedFamilyForFormat(t *testing.T) string {
	t.Helper()

	claimed := registry.FamilyMap()
	names := family.RegisteredFamilyNames()
	slices.Sort(names)
	for _, name := range names {
		if claimed[name] == "" {
			return name
		}
	}
	t.Fatal("every registered family is claimed by a plugin, so no family is free to claim")
	return ""
}

// TestAppendNLRIJSONValueCarriesAddPathToTheDecoder is the discrimination test
// for step 2 of the ADD-PATH decode chain. The formatter hands the decoder the
// octets as they arrived, Path Identifier included, and nothing in those octets
// says whether the first four are one.
//
// RFC 7911 Section 3: "In order to carry the Path Identifier in an UPDATE
// message, the NLRI encoding MUST be extended by prepending the Path Identifier
// field, which is of four octets."
//
// The synthetic decoder reports the flag it was given, so the assertion fails
// the moment appendNLRIJSONValue hard-codes false, drops the AddPathAware probe,
// or answers from a concrete-type switch that does not know WireNLRI.
//
// VALIDATES: appendNLRIJSONValue reads the ADD-PATH layout off the NLRI itself
// and passes it to registry.DecodeNLRIByFamily.
// PREVENTS: an ADD-PATH route in a plugin family rendering as
// {"parsed":false,"raw":"<hex>"} because the decoder read the Path Identifier
// as the first octets of the NLRI.
func TestAppendNLRIJSONValueCarriesAddPathToTheDecoder(t *testing.T) {
	claimable := unclaimedFamilyForFormat(t)

	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })

	err := registry.Register(registry.Registration{
		Name:        "test-format-addpath",
		Description: "synthetic NLRI plugin reporting the ADD-PATH flag it was handed",
		Families:    []string{claimable},
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
		InProcessNLRIDecoder: func(_, hex string, addPath bool) (any, error) {
			return map[string]any{"add-path": addPath, "hex": hex}, nil
		},
	})
	if err != nil {
		t.Fatalf("register synthetic plugin: %v", err)
	}

	fam, held := family.LookupFamily(claimable)
	if !held {
		t.Fatalf("family registry does not hold %s", claimable)
	}

	// Four octets of Path Identifier (10), then one NLRI of 24 bits.
	addPathSection := []byte{0x00, 0x00, 0x00, 0x0a, 0x18, 0x0a, 0x00, 0x00}
	plainSection := []byte{0x18, 0x0a, 0x00, 0x00}

	cases := []struct {
		name    string
		data    []byte
		addPath bool
		want    string
	}{
		{name: "add-path negotiated", data: addPathSection, addPath: true, want: `"add-path":true`},
		{name: "add-path not negotiated", data: plainSection, addPath: false, want: `"add-path":false`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wire, wireErr := nlri.NewWireNLRI(fam, tc.data, tc.addPath)
			if wireErr != nil {
				t.Fatalf("wrap NLRI: %v", wireErr)
			}

			got := string(appendNLRIJSONValue(nil, wire, fam))
			if !strings.Contains(got, tc.want) {
				t.Errorf("want the decoder to be told %s, got %s", tc.want, got)
			}
		})
	}
}
