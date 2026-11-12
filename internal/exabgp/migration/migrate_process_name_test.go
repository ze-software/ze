// GOAL: every ExaBGP process name migrates into a config ze loads, and every
// reference to the process follows the name its bridge block carries.
//
// METHOD: migrate processes named with an underscore, a dot, a leading dash,
// one of exactly 64 characters, and two longer names that cut to that same
// 64-character text, each bound to a neighbor by an api block. Check the
// bridge block names (unchanged where the bridge holds them, cut and
// disambiguated where it does not), that each feed sits under the renamed
// block, that the peer attaches the bridge, that the migration notes each
// rename, and that ze's parser, which judges every list key against its YANG
// key leaf, accepts the output and refuses it with a long name left unmapped.
//
// PREVENTS: an ExaBGP process name over 64 characters migrating into
// `exabgp { bridge { process <name> } }`, whose key holds 64 characters, so
// `ze config validate` refused the migrated config. And it pins the reading
// that an ExaBGP name is never written as `plugin external <name>`, the one
// key the plugin name form (ipc.PluginNameForm) judges: every process binds
// the in-process bridge.

package migration

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/config"
)

// TestBridgeProcessNamesCutAndDisambiguate guards bridgeProcessNames: a name
// the bridge holds is kept, a longer one is cut to 64 characters at a
// character, a cut already taken gets -2, then -3, and a kept name is
// reserved before any cut, whatever the order.
func TestBridgeProcessNamesCutAndDisambiguate(t *testing.T) {
	exact := strings.Repeat("p", bridgeProcessNameMax)
	longOne := exact + "-one"
	longTwo := exact + "-two"
	wide := strings.Repeat("日", bridgeProcessNameMax) + "x"
	got := bridgeProcessNames([]string{"watch_routes", longOne, "a.b", "-lead", longTwo, exact, wide})

	want := map[string]string{
		"watch_routes": "watch_routes",
		"a.b":          "a.b",
		"-lead":        "-lead",
		exact:          exact,
		longOne:        strings.Repeat("p", bridgeProcessNameMax-2) + "-2",
		longTwo:        strings.Repeat("p", bridgeProcessNameMax-2) + "-3",
		wide:           strings.Repeat("日", bridgeProcessNameMax),
	}
	if len(got) != len(want) {
		t.Fatalf("named %d processes, want %d: %q", len(got), len(want), got)
	}
	seen := map[string]string{}
	for exabgp, bridgeName := range want {
		if got[exabgp] != bridgeName {
			t.Errorf("bridge name of %q = %q, want %q", exabgp, got[exabgp], bridgeName)
		}
		if n := utf8.RuneCountInString(got[exabgp]); n > bridgeProcessNameMax {
			t.Errorf("bridge name %q holds %d characters, bound is %d", got[exabgp], n, bridgeProcessNameMax)
		}
		if other, dup := seen[got[exabgp]]; dup {
			t.Errorf("%q and %q both map to %q", exabgp, other, got[exabgp])
		}
		seen[got[exabgp]] = exabgp
	}
}

// TestMigrateProcessNamesZeLoads migrates the names through the whole
// migration and loads the result.
func TestMigrateProcessNamesZeLoads(t *testing.T) {
	exact := strings.Repeat("p", bridgeProcessNameMax)
	longOne := exact + "-one"
	longTwo := exact + "-two"
	names := []string{"watch_routes", "a.b", "-lead", exact, longOne, longTwo}

	var source strings.Builder
	for _, name := range names {
		source.WriteString("process " + name + " {\n\trun ./script.py;\n\tencoder json;\n}\n")
	}
	source.WriteString("neighbor 10.0.0.1 {\n\trouter-id 1.1.1.1;\n\tlocal-as 65001;\n\tpeer-as 65002;\n")
	source.WriteString("\tapi {\n\t\tprocesses [ " + strings.Join(names, " ") + " ];\n\t\treceive {\n\t\t\tupdate;\n\t\t}\n\t}\n}\n")

	tree, err := ParseExaBGPConfig(source.String())
	if err != nil {
		t.Fatalf("parse: %v\n%s", err, source.String())
	}
	result, err := MigrateFromExaBGP(tree)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	renamed := map[string]string{
		longOne: strings.Repeat("p", bridgeProcessNameMax-2) + "-2",
		longTwo: strings.Repeat("p", bridgeProcessNameMax-2) + "-3",
	}
	// Only the rename notes are judged: the migration writes other warnings
	// (the RFC 8212 policies, for one) that have nothing to do with names.
	renames := map[string]int{}
	for _, warning := range result.Warnings {
		if strings.Contains(warning, renamedProcessMark) {
			renames[warning]++
		}
	}
	for exabgp, bridgeName := range renamed {
		if renames[renamedProcessWarning(exabgp, bridgeName)] != 1 {
			t.Errorf("want one warning noting %q renamed %q: %q", exabgp, bridgeName, result.Warnings)
		}
	}
	if len(renames) != len(renamed) {
		t.Errorf("rename warnings = %v, want one per renamed process and none for a kept name", renames)
	}

	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("load ze schema: %v", err)
	}
	output := SerializeTree(result.Tree)
	if strings.Contains(output, "external ") {
		t.Errorf("an ExaBGP process became an external plugin:\n%s", output)
	}
	reparsed, err := config.NewParser(schema).Parse(output)
	if err != nil {
		t.Fatalf("ze refuses the migrated config: %v\n%s", err, output)
	}
	blocks := reparsed.GetContainer("exabgp").GetContainer("bridge").GetList("process")
	if len(blocks) != len(names) {
		t.Errorf("bridge carries %d process blocks, want %d:\n%s", len(blocks), len(names), output)
	}
	for _, name := range names {
		bridgeName := name
		if to, ok := renamed[name]; ok {
			bridgeName = to
		}
		block := blocks[bridgeName]
		if block == nil {
			t.Errorf("no bridge process %q for ExaBGP process %q:\n%s", bridgeName, name, output)
			continue
		}
		if len(block.GetList("feed")) != 1 {
			t.Errorf("bridge process %q carries %d feeds, want the one neighbor:\n%s",
				bridgeName, len(block.GetList("feed")), output)
		}
	}
	if !strings.Contains(output, "process "+bridgePluginName+" {") {
		t.Errorf("the peer does not attach %s:\n%s", bridgePluginName, output)
	}

	// The parse above is the check that judges the name: the config parser
	// validates every list key against its key leaf (parser_list.go,
	// ValidateListKey), the bridge key's `length "1..64"` included. The full
	// loader is not run here: this package's test binary links none of the
	// plugins the migrated config names (bgp-rib, bgp-watchdog,
	// exabgp-bridge), so config.ReadConfig refuses their `use` leaves first,
	// and the plugin composition imports this package, so linking it here is
	// an import cycle. Control: the same output with one long name left
	// unmapped is refused, so the parse is a check that can go red.
	unmapped := strings.Replace(output, "process "+renamed[longOne]+" {", "process "+longOne+" {", 1)
	if unmapped == output {
		t.Fatalf("control: no bridge process %q in the output:\n%s", renamed[longOne], output)
	}
	if _, err := config.NewParser(schema).Parse(unmapped); err == nil {
		t.Errorf("control: ze accepts a bridge process named with %d characters:\n%s",
			utf8.RuneCountInString(longOne), unmapped)
	}
}
