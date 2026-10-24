package migration

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

// TestReceivePolicyValuesMatchTheExaBGPModel proves the Go constants
// applyReceivePolicy compares against are exactly the enumeration values
// exabgp.yang declares for the neighbor leaves `as-set` and
// `tunnel-encapsulation`, read from the loaded model rather than from a copy.
//
// VALIDATES: each leaf's enumeration equals the set of constants spelled in
// migrate_receive_policy.go.
// PREVENTS: a value the model accepts that the migration refuses as unknown,
// and a value renamed in the model while the Go constant keeps the old
// spelling, so `as-set accept` silently migrates to a peer that withdraws.
func TestReceivePolicyValuesMatchTheExaBGPModel(t *testing.T) {
	cases := []struct {
		leaf  string
		known []string
	}{
		{"as-set", []string{exabgpASSetWithdraw, exabgpASSetAccept}},
		{"tunnel-encapsulation", []string{exabgpTunnelEncapAuto, exabgpTunnelEncapFilter, exabgpTunnelEncapAccept}},
	}
	neighbor := exabgpNeighborEntry(t)
	for _, tc := range cases {
		t.Run(tc.leaf, func(t *testing.T) {
			leaf := neighbor.Dir[tc.leaf]
			if leaf == nil {
				t.Fatalf("exabgp.yang declares no neighbor leaf %s", tc.leaf)
			}
			if leaf.Type == nil || leaf.Type.Kind != gyang.Yenum || leaf.Type.Enum == nil {
				t.Fatalf("exabgp.yang neighbor leaf %s is not an enumeration", tc.leaf)
			}
			declared := slices.Clone(leaf.Type.Enum.Names())
			slices.Sort(declared)
			known := slices.Clone(tc.known)
			slices.Sort(known)
			if !slices.Equal(declared, known) {
				t.Fatalf("exabgp.yang %s declares %v, migrate_receive_policy.go spells %v", tc.leaf, declared, known)
			}
		})
	}
}

// exabgpNeighborEntry answers the `neighbor` list of the exabgp.yang module,
// loaded the way exaBGPSchema loads it, and FAILS rather than answering nil.
func exabgpNeighborEntry(t *testing.T) *gyang.Entry {
	t.Helper()
	loader := configyang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		t.Fatalf("load embedded modules: %v", err)
	}
	if err := loader.AddModuleFromText("exabgp.yang", exabgpYANG); err != nil {
		t.Fatalf("add exabgp.yang: %v", err)
	}
	resolved, err := loader.Resolve()
	if err != nil {
		t.Fatalf("resolve exabgp.yang: %v", err)
	}
	module := resolved.GetEntry("exabgp")
	if module == nil {
		t.Fatal("exabgp.yang is not loaded")
	}
	neighbor := module.Dir["neighbor"]
	if neighbor == nil {
		t.Fatal("exabgp.yang declares no neighbor list")
	}
	return neighbor
}

// receivePolicyNeighbor is an ExaBGP 6.0 neighbor stating one receive-policy
// leaf, the ExaBGP f9bfb853b grammar (configuration/grammar/tree/neighbor.py).
func receivePolicyNeighbor(leaf, value string) string {
	return "neighbor 10.0.0.1 {\n\tlocal-as 65001\n\tpeer-as 65002\n\t" + leaf + " " + value + ";\n}\n"
}

// TestMigrateReceivePolicySameAsZe proves the values whose effect ze already
// produces migrate cleanly, write no leaf, and give a config ze parses.
//
// VALIDATES: `as-set withdraw` (RFC 9774 Section 3 treat-as-withdraw, ze's
// only behavior) and `tunnel-encapsulation accept` and `auto` (ze keeps a
// received attribute 23 on every session) are accepted.
// PREVENTS: An ExaBGP 6.0 config refused for a leaf whose effect ze matches.
func TestMigrateReceivePolicySameAsZe(t *testing.T) {
	cases := []struct{ leaf, value string }{
		{"as-set", "withdraw"},
		{"tunnel-encapsulation", "accept"},
		{"tunnel-encapsulation", "auto"},
	}
	for _, tc := range cases {
		t.Run(tc.leaf+"-"+tc.value, func(t *testing.T) {
			output := migrateNeighborOutput(t, receivePolicyNeighbor(tc.leaf, tc.value))
			if strings.Contains(output, tc.leaf) {
				t.Fatalf("migrated config names %s:\n%s", tc.leaf, output)
			}
			if err := parseZeConfig(t, output); err != nil {
				t.Fatalf("ze refuses the migrated config: %v\n%s", err, output)
			}
		})
	}
}

// TestMigrateReceivePolicyRefusedWithoutZeEquivalent proves a value ze cannot
// reproduce stops the migration with an error naming the leaf and the value,
// rather than writing a peer that treats received routes differently.
//
// VALIDATES: `tunnel-encapsulation filter` (ze has no attribute 23 filter) is
// refused by the migration, with the reason and the fix on their own lines; a
// value ExaBGP does not know is refused by the exabgp.yang enumeration at parse.
// PREVENTS: A migrated peer silently withdrawing routes ExaBGP accepted, or
// passing on an attribute ExaBGP dropped.
func TestMigrateReceivePolicyRefusedWithoutZeEquivalent(t *testing.T) {
	cases := []struct {
		leaf, value string
		atParse     bool
	}{
		{"tunnel-encapsulation", "filter", false},
		{"as-set", "keep", true},
		{"tunnel-encapsulation", "drop", true},
	}
	for _, tc := range cases {
		t.Run(tc.leaf+"-"+tc.value, func(t *testing.T) {
			tree, err := ParseExaBGPConfig(receivePolicyNeighbor(tc.leaf, tc.value))
			if tc.atParse {
				if err == nil {
					t.Fatalf("%s %s parsed, want the enumeration to refuse it", tc.leaf, tc.value)
				}
				for _, word := range []string{tc.leaf, tc.value} {
					if !strings.Contains(err.Error(), word) {
						t.Errorf("refusal %q does not name %q", err, word)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			_, err = MigrateFromExaBGP(tree)
			if err == nil {
				t.Fatalf("%s %s migrated, want a refusal", tc.leaf, tc.value)
			}
			for _, line := range []string{"\nwhy: ", "\nfix: "} {
				if !strings.Contains(err.Error(), line) {
					t.Errorf("refusal %q has no %q line", err, strings.TrimSpace(line))
				}
			}
			for _, word := range []string{tc.leaf, tc.value} {
				if !strings.Contains(err.Error(), word) {
					t.Errorf("refusal %q does not name %q", err, word)
				}
			}
		})
	}
}

// TestMigrateASSetAcceptKeepsTheSet proves ExaBGP's `as-set accept` becomes
// the ze peer leaf `session { as-set accept; }`, so the migrated peer keeps a
// received route whose AS_PATH holds an AS_SET, as the ExaBGP neighbor did.
//
// VALIDATES: RFC 9774 Section 3's operator configuration, "Unless explicitly
// configured by a network operator to do otherwise", is written by the
// migration for the neighbor that asked for it, and ze parses the result.
// PREVENTS: A migrated peer withdrawing routes its ExaBGP neighbor accepted,
// or the migration refusing a config ze can now reproduce.
func TestMigrateASSetAcceptKeepsTheSet(t *testing.T) {
	output := migrateNeighborOutput(t, receivePolicyNeighbor("as-set", "accept"))
	if !sessionASSetAccept.MatchString(output) {
		t.Fatalf("migrated config has no session { as-set accept; }:\n%s", output)
	}
	if err := parseZeConfig(t, output); err != nil {
		t.Fatalf("ze refuses the migrated config: %v\n%s", err, output)
	}
}

// sessionASSetAccept matches the migrated peer's `as-set accept` inside its
// session block.
var sessionASSetAccept = regexp.MustCompile(`session \{[^}]*as-set accept`)

// TestMigrateReceivePolicyFromTemplate proves a receive-policy leaf set in a
// template reaches the neighbor that inherits it.
//
// VALIDATES: expandInheritance carries as-set, and `as-set accept` from a
// template lands in the inheriting peer's session block.
// PREVENTS: `as-set accept` in a template migrating silently to a peer that
// withdraws the routes.
func TestMigrateReceivePolicyFromTemplate(t *testing.T) {
	input := "template {\n\tneighbor base {\n\t\tas-set accept;\n\t}\n}\n" +
		"neighbor 10.0.0.1 {\n\tinherit base;\n\tlocal-as 65001\n\tpeer-as 65002\n}\n"
	output := migrateNeighborOutput(t, input)
	if !sessionASSetAccept.MatchString(output) {
		t.Fatalf("as-set accept from a template did not reach the peer:\n%s", output)
	}
}
