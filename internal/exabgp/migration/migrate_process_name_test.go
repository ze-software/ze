// Design: docs/exabgp/exabgp-migration.md -- endpoint name sanitization and Ze schema validation tests
// Related: migrate_process_name.go -- endpointProcessNames and renamedProcessWarning
//
// VALIDATES: every ExaBGP process name sanitizes deterministically into a valid
// bus endpoint name accepted by ipc.CheckPluginName and config.ParseTreeWithYANG,
// with collision suffixes (-2, -3, ...) and migration warnings on every rename.
// PREVENTS: an ExaBGP process name with '_', '.', leading '-', non-ASCII
// characters, or >64 characters producing a `bus { endpoint <name> }` config
// that Ze refuses at load.

package migration

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
)

// TestEndpointProcessNamesSanitizeAndDisambiguate guards endpointProcessNames:
// valid names are reserved unchanged, invalid characters become '-', leading
// '-' is stripped (with "p" fallback), long names truncate to 64 characters,
// collisions append -2/-3, and every result passes ipc.CheckPluginName.
//
// VALIDATES: Deterministic sanitization and uniqueness under ipc.CheckPluginName.
// PREVENTS: Emitting invalid or colliding endpoint names in bus > endpoint.
func TestEndpointProcessNamesSanitizeAndDisambiguate(t *testing.T) {
	exact := strings.Repeat("p", endpointNameMax)
	longOne := exact + "-one"
	longTwo := exact + "-two"
	wide := strings.Repeat("日", endpointNameMax) + "x"
	got := endpointProcessNames([]string{"watch_routes", longOne, "a.b", "-lead", longTwo, exact, wide, "---", "___"})

	want := map[string]string{
		"watch_routes": "watch-routes",
		"a.b":          "a-b",
		"-lead":        "lead",
		exact:          exact,
		longOne:        strings.Repeat("p", endpointNameMax-2) + "-2",
		longTwo:        strings.Repeat("p", endpointNameMax-2) + "-3",
		wide:           "x",
		"---":          "p",
		"___":          "p-2",
	}
	if len(got) != len(want) {
		t.Fatalf("named %d processes, want %d: %q", len(got), len(want), got)
	}
	seen := map[string]string{}
	for exabgp, epName := range want {
		if got[exabgp] != epName {
			t.Errorf("endpoint name of %q = %q, want %q", exabgp, got[exabgp], epName)
		}
		if err := ipc.CheckPluginName(got[exabgp]); err != nil {
			t.Errorf("endpoint name %q fails ipc.CheckPluginName: %v", got[exabgp], err)
		}
		if other, dup := seen[got[exabgp]]; dup {
			t.Errorf("%q and %q both map to %q", exabgp, other, got[exabgp])
		}
		seen[got[exabgp]] = exabgp
	}
}

// TestMigrateProcessNamesZeLoads migrates names requiring sanitization and
// truncation through the full migration and validates the output with Ze's
// YANG parser and bus validator.
//
// VALIDATES: Sanitized names emit rename warnings, parse under external-plugin-name,
// and keep their receive/send filters on the renamed endpoints; unmapped invalid
// names are refused by the parser (control).
// PREVENTS: Silent name mismatches between bus > endpoint and neighbor api blocks.
func TestMigrateProcessNamesZeLoads(t *testing.T) {
	exact := strings.Repeat("p", endpointNameMax)
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
		"watch_routes": "watch-routes",
		"a.b":          "a-b",
		"-lead":        "lead",
		longOne:        strings.Repeat("p", endpointNameMax-2) + "-2",
		longTwo:        strings.Repeat("p", endpointNameMax-2) + "-3",
	}
	// Only the rename notes are judged: the migration writes other warnings
	// (the RFC 8212 policies, for one) that have nothing to do with names.
	renames := map[string]int{}
	for _, warning := range result.Warnings {
		if strings.Contains(warning, renamedProcessMark) {
			renames[warning]++
		}
	}
	for exabgp, epName := range renamed {
		if renames[renamedProcessWarning(exabgp, epName)] != 1 {
			t.Errorf("want one warning noting %q renamed %q: %q", exabgp, epName, result.Warnings)
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
	reparsed, err := config.NewParser(schema).Parse(output)
	if err != nil {
		t.Fatalf("ze refuses the migrated config: %v\n%s", err, output)
	}
	if err := config.ValidateBusConfig(reparsed); err != nil {
		t.Fatalf("bus validator refuses the migrated config: %v\n%s", err, output)
	}

	endpoints := reparsed.GetContainer("bus").GetList("endpoint")
	for _, name := range names {
		epName := name
		if to, ok := renamed[name]; ok {
			epName = to
		}
		ep := endpoints[epName]
		if ep == nil {
			t.Errorf("no bus endpoint %q for ExaBGP process %q:\n%s", epName, name, output)
			continue
		}
		if recv := ep.GetContainer("receive"); recv == nil || len(recv.GetSlice("from")) != 1 {
			t.Errorf("bus endpoint %q receive > from missing the one neighbor:\n%s", epName, output)
		}
		if send := ep.GetContainer("send"); send == nil || len(send.GetSlice("to")) != 1 {
			t.Errorf("bus endpoint %q send > to missing the one neighbor:\n%s", epName, output)
		}
	}

	// Control: leaving an invalid name unmapped (length > 64 or underscore)
	// must be refused by Ze's YANG list-key validator (external-plugin-name).
	for _, bad := range []struct {
		sanitized string
		raw       string
	}{
		{renamed[longOne], longOne},
		{"watch-routes", "watch_routes"},
	} {
		unmapped := strings.Replace(output, "endpoint "+bad.sanitized+" {", "endpoint "+bad.raw+" {", 1)
		if unmapped == output {
			t.Fatalf("control: no bus endpoint %q in the output:\n%s", bad.sanitized, output)
		}
		parsedBad, parseErr := config.NewParser(schema).Parse(unmapped)
		if parseErr == nil {
			parseErr = config.ValidateBusConfig(parsedBad)
		}
		if parseErr == nil {
			t.Errorf("control: ze accepted invalid endpoint name %q:\n%s", bad.raw, unmapped)
		}
	}
}
