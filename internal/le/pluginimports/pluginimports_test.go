// Related: pluginimports.go -- the generator these tests call as a function

package pluginimports

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const module = "example.test/imports"

func write(t *testing.T, root, rel, content string) {
	t.Helper()

	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create the fixture directory for %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write the fixture %s: %v", rel, err)
	}
}

// fixture builds a checkout holding one package of each category the generator
// discovers, plus one it must skip.
func fixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	write(t, root, "go.mod", "module "+module+"\n\ngo 1.26\n")
	write(t, root, "feature-gates.txt", "ze_lg internal/plugins/looking\n")

	// A plugin: any register.go under a search root that is not schema/ or yang/.
	write(t, root, "internal/plugins/host/register.go", "package host\n")
	// A plugin the generator must skip, because importing it would cycle.
	write(t, root, "internal/plugins/cli/register.go", "package cli\n\n// codegen:skip -- wired by cmd/ze/main.go\n")
	// A schema package: a yang/register.go importing the registry.
	write(t, root, "internal/plugins/host/yang/register.go",
		"package yang\n\nimport (\n\t_ \""+module+"/internal/component/config/yang\"\n)\n")
	// A yang/register.go that imports nothing is not a schema package.
	write(t, root, "internal/plugins/quiet/yang/register.go", "package yang\n")
	// config/yang is the REGISTRY a schema package registers into. It names
	// itself, so only its exclusion keeps it out of the composition root -- and
	// naming it there would be a package blank-importing its own init().
	write(t, root, "internal/component/config/yang/register.go",
		"package yang\n\nimport (\n\t_ \""+module+"/internal/component/config/yang\"\n)\n")
	// An RPC command package.
	write(t, root, "internal/component/ping/cmd/rpc.go",
		"package cmd\n\nfunc init() {\n\tpluginserver.RegisterRPCs(handlers)\n}\n")
	// An event namespace package.
	write(t, root, "internal/component/bgp/events.go",
		"package bgp\n\nfunc init() {\n\tevents.RegisterNamespace(\"bgp\")\n}\n")
	// A gated package: feature-gates.txt names it, so it leaves all.go for its
	// own build-tag group.
	write(t, root, "internal/plugins/looking/register.go", "package looking\n")
	// The composition root's own directory.
	if err := os.MkdirAll(filepath.Join(root, allDir), 0o755); err != nil {
		t.Fatalf("create the composition root: %v", err)
	}

	return root
}

// VALIDATES: each of the four categories is discovered from the tree, the
// codegen:skip marker excludes a package, and a gated package leaves the
// universal file for its own tag group.
// PREVENTS: a plugin that registers nothing because nobody imports it, and an
// import cycle from a CLI-only package the composition root must not name.
func TestDiscoveryFindsEachCategory(t *testing.T) {
	root := fixture(t)

	found, err := derive(root)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}

	if strings.Join(found.Plugins, ",") != module+"/internal/plugins/host" {
		t.Errorf("the plugins are %v, want only host (cli carries codegen:skip, lg is gated)", found.Plugins)
	}
	if strings.Join(found.Schemas, ",") != module+"/internal/plugins/host/yang" {
		t.Errorf("the schemas are %v, want only host/yang", found.Schemas)
	}
	if strings.Join(found.RPCs, ",") != module+"/internal/component/ping/cmd" {
		t.Errorf("the rpcs are %v, want only ping/cmd", found.RPCs)
	}
	if strings.Join(found.Namespaces, ",") != module+"/internal/component/bgp" {
		t.Errorf("the namespaces are %v, want only bgp", found.Namespaces)
	}
	if strings.Join(found.ByTag["ze_lg"], ",") != module+"/internal/plugins/looking" {
		t.Errorf("the ze_lg group is %v, want the gated package", found.ByTag["ze_lg"])
	}
}

// VALIDATES: a nested component-domain plugin root is discovered structurally.
// PREVENTS: clustering edge plugins under l2tp/plugins or ike/plugins dropping
// their registrations, which is a feature that vanishes with no build error.
//
// This is the proof scripts/codegen/plugin_imports_test.go
// TestClusterPluginDiscovery carried as a subprocess run of `--check` over a
// fixture tree. It asserts the same fact, from the discovery function.
func TestNestedComponentDomainPluginsAreFound(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module "+module+"\n\ngo 1.26\n")
	write(t, root, "feature-gates.txt", "")
	write(t, root, "internal/component/ike/plugins/sitevpn/register.go", "package sitevpn\n")
	write(t, root, "internal/component/l2tp/plugins/edgeauth/register.go", "package edgeauth\n")

	found, err := derive(root)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}

	want := module + "/internal/component/ike/plugins/sitevpn," + module + "/internal/component/l2tp/plugins/edgeauth"
	if strings.Join(found.Plugins, ",") != want {
		t.Fatalf("the plugins are %v, want both nested cluster plugins", found.Plugins)
	}
}

// VALIDATES: the generated all.go carries the exact bytes the committed
// composition root holds, in the exact order and grouping.
// PREVENTS: a rewritten generator whose first run rewrites the composition root
// of the whole product.
func TestGeneratedAllGoIsByteExact(t *testing.T) {
	got := string(allSource(imports{
		Plugins: []string{"m/a"},
		Schemas: []string{"m/s/yang"},
		RPCs:    []string{"m/r/cmd"},
		Namespaces: []string{
			"m/n",
		},
	}))

	want := "// Code generated by scripts/codegen/plugin_imports.go; DO NOT EDIT.\n\n" +
		"// Package all imports all internal plugins and schema packages,\n" +
		"// triggering their init() registration.\n" +
		"//\n" +
		"// To add a plugin, create internal/component/bgp/plugins/<name>/register.go with an init()\n" +
		"// that calls registry.Register(). Then run: make generate\n" +
		"package all\n\n" +
		"import (\n" +
		"\t// Infrastructure schema packages — YANG module registration.\n" +
		"\t_ \"m/s/yang\"\n\n" +
		"\t// Plugin packages — plugin + schema registration.\n" +
		"\t_ \"m/a\"\n\n" +
		"\t// Event namespace packages -- events.RegisterNamespace registration.\n" +
		"\t_ \"m/n\"\n\n" +
		"\t// RPC command packages -- pluginserver.RegisterRPCs registration.\n" +
		"\t_ \"m/r/cmd\"\n" +
		")\n"

	if got != want {
		t.Errorf("all.go is\n%q\nwant\n%q", got, want)
	}
}

// VALIDATES: a gated package nested under other gates ANDs every ancestor tag,
// and the answer does not depend on the map iteration order Go randomizes.
// PREVENTS: a build requesting the child tag alone dragging an ancestor's whole
// subtree back in, and a generated file whose //go:build line changes between
// two runs over the same tree.
//
// This is the proof scripts/codegen/plugin_imports.go carried as its own
// --selftest flag, driven by a subprocess in
// TestPluginImportsConstraintSelftest. The tags are a parameter here rather than
// a package-level variable, so the cases are ordinary table rows and the flag
// has nothing left to do.
func TestConstraintAndsEveryAncestorAndIsDeterministic(t *testing.T) {
	nested := map[string]string{
		"internal/a":     "ze_gp",
		"internal/a/b":   "ze_p",
		"internal/a/b/c": "ze_c",
		"internal/solo":  "ze_solo",
	}
	cases := []struct {
		name   string
		imp    string
		tag    string
		expect string
	}{
		{"depth 0", "example.com/m/internal/a", "ze_gp", "ze_gp"},
		{"depth 1", "example.com/m/internal/a/b", "ze_p", "ze_gp && ze_p"},
		{"depth 2", "example.com/m/internal/a/b/c", "ze_c", "ze_gp && ze_p && ze_c"},
		{"independent", "example.com/m/internal/solo", "ze_solo", "ze_solo"},
	}
	for _, one := range cases {
		if got := constraintForImport(nested, one.imp, one.tag); got != one.expect {
			t.Errorf("%s: the constraint is %q, want %q", one.name, got, one.expect)
		}
	}

	// Two gate paths of the SAME length both segment-align as ancestors of one
	// import. Selecting the longest returned whichever the map yielded first,
	// so every ancestor is collected and sorted instead.
	tie := map[string]string{"seg/aaa": "ze_two", "seg/bbb": "ze_one"}
	const tieImport = "example.com/m/seg/aaa/seg/bbb/leaf"
	const tieWant = "ze_one && ze_two && ze_leaf"
	for range 200 {
		if got := constraintForImport(tie, tieImport, "ze_leaf"); got != tieWant {
			t.Fatalf("the constraint is %q on one of 200 runs, want %q every time", got, tieWant)
		}
	}

	order, groups := constraintGroups(tie, "ze_leaf", []string{tieImport})
	if strings.Join(order, "|") != tieWant {
		t.Errorf("the group order is %v, want one group named %q", order, tieWant)
	}
	if len(groups[tieWant]) != 1 {
		t.Errorf("the group holds %v, want the one import", groups[tieWant])
	}
}

// VALIDATES: a tag whose packages take different constraints is split into
// groups, and the plain-tag group keeps the historic file name.
// PREVENTS: a mixed tag forcing every one of its packages under an ancestor
// gate, so a feature usable alone stops compiling without it.
func TestAMixedTagSplitsAndKeepsTheHistoricName(t *testing.T) {
	tags := map[string]string{
		"internal/component/l2tp":                    "ze_l2tp",
		"internal/component/radius":                  "ze_radius",
		"internal/component/l2tp/plugins/authradius": "ze_radius",
	}
	imports := []string{
		"m/internal/component/radius",
		"m/internal/component/l2tp/plugins/authradius",
	}

	order, _ := constraintGroups(tags, "ze_radius", imports)
	if strings.Join(order, "|") != "ze_radius|ze_l2tp && ze_radius" {
		t.Fatalf("the group order is %v, want the plain tag first", order)
	}
	if name := taggedGroupFileName("ze_radius", "ze_radius", false); name != "all_ze_radius.go" {
		t.Errorf("the plain group is named %s, want all_ze_radius.go", name)
	}
	if name := taggedGroupFileName("ze_radius", "ze_l2tp && ze_radius", false); name != "all_ze_radius_ze_l2tp.go" {
		t.Errorf("the dependent group is named %s, want all_ze_radius_ze_l2tp.go", name)
	}
	if name := taggedGroupFileName("ze_lg", "ze_lg", true); name != "all_ze_lg.go" {
		t.Errorf("a sole group is named %s, want all_ze_lg.go", name)
	}
}

// VALIDATES: write generates every file, check then passes, and a second write
// reports nothing.
// PREVENTS: a generator whose own check rejects its output.
func TestWriteThenCheckAgree(t *testing.T) {
	root := fixture(t)

	written, code := runWrite(root)
	if code != 0 {
		t.Fatalf("write answered %d, want 0", code)
	}
	if len(written.Written) != 2 {
		t.Fatalf("write reported %v, want all.go and the ze_lg group", written.Written)
	}

	report, code := runCheck(root)
	if code != 0 {
		t.Fatalf("check after write answered %d; stale %q", code, report.Stale)
	}
	if report.Plugins != 1 || report.Schemas != 1 || report.RPCs != 1 || report.Namespaces != 1 || report.GatedGroups != 1 {
		t.Errorf("the counts are %+v, want one of each", report)
	}
}

// VALIDATES: check reports the composition root as stale when a plugin was added
// and all.go was not regenerated, and writes nothing.
// PREVENTS: --check passing while the product's composition root is missing a
// plugin, which is a feature that vanishes with no build error.
func TestCheckReportsAStaleCompositionRoot(t *testing.T) {
	root := fixture(t)
	if _, code := runWrite(root); code != 0 {
		t.Fatal("the first write failed")
	}

	before := readFile(t, root, filepath.Join(allDir, "all.go"))
	write(t, root, "internal/plugins/later/register.go", "package later\n")

	report, code := runCheck(root)
	if code != 1 {
		t.Fatalf("check answered %d over a tree whose all.go is stale, want 1", code)
	}
	if report.Stale != filepath.ToSlash(filepath.Join(allDir, "all.go")) || report.Reason != reasonStale {
		t.Errorf("check named %q %q, want all.go stale", report.Stale, report.Reason)
	}
	if after := readFile(t, root, filepath.Join(allDir, "all.go")); after != before {
		t.Error("check rewrote all.go")
	}
}

// VALIDATES: a generated tag file that is missing, and one no tag needs any
// more, are each reported by name; the write removes the second.
// PREVENTS: a gated group nobody generates, and a leftover file that keeps
// blank-importing a package the manifest no longer gates.
func TestTaggedFilesAreCheckedAndTidied(t *testing.T) {
	root := fixture(t)
	if _, code := runWrite(root); code != 0 {
		t.Fatal("the first write failed")
	}

	if err := os.Remove(filepath.Join(root, allDir, "all_ze_lg.go")); err != nil {
		t.Fatalf("remove the generated tag file: %v", err)
	}
	report, code := runCheck(root)
	if code != 1 || report.Reason != reasonMissing {
		t.Fatalf("check answered %d %q for a missing tag file, want 1 and missing", code, report.Reason)
	}

	if _, code := runWrite(root); code != 0 {
		t.Fatal("the second write failed")
	}

	// A group file left behind by a gate that is gone. all.go stays current, so
	// the check reaches the stray comparison rather than stopping before it.
	write(t, root, filepath.Join(allDir, "all_ze_gone.go"),
		generatedMarker+"\n\n//go:build ze_gone\n\npackage all\n")

	report, code = runCheck(root)
	if code != 1 || report.Reason != reasonUngated {
		t.Fatalf("check answered %d %q for an ungated tag file, want 1 and ungated", code, report.Reason)
	}

	if _, code := runWrite(root); code != 0 {
		t.Fatal("the third write failed")
	}
	if _, err := os.Stat(filepath.Join(root, allDir, "all_ze_gone.go")); !os.IsNotExist(err) {
		t.Errorf("the write left the ungated tag file behind: %v", err)
	}

	// A file the tool did not generate is left alone, whatever it is named.
	write(t, root, filepath.Join(allDir, "all_hand_written.go"), "package all\n")
	if _, code := runWrite(root); code != 0 {
		t.Fatal("the fourth write failed")
	}
	if _, err := os.Stat(filepath.Join(root, allDir, "all_hand_written.go")); err != nil {
		t.Errorf("the write removed a file it did not generate: %v", err)
	}
}

// VALIDATES: a source file the walk LISTS and cannot read stops the run, in each
// of the four discovery walks.
// PREVENTS: the fail-open this port fixes. The script's readers answer false on
// an unreadable file, so a schema package, an RPC package or an event namespace
// is silently DROPPED from the composition root -- and the write half then
// commits a composition root with it gone, which is a registration that vanishes
// with no build error and no message.
func TestAFileTheWalkCannotReadStopsTheRun(t *testing.T) {
	// A dangling symbolic link: the walk lists it as a file, os.Open fails with
	// ErrNotExist, and no permission or ownership makes it readable. That is
	// what makes this deterministic for root as well.
	cases := map[string]string{
		"a plugin register.go":  "internal/plugins/ghost/register.go",
		"a schema register.go":  "internal/plugins/ghost/yang/register.go",
		"an rpc command file":   "internal/component/ghost/cmd/rpc.go",
		"an event namespace go": "internal/component/ghost/events.go",
	}

	for name, rel := range cases {
		t.Run(name, func(t *testing.T) {
			root := fixture(t)
			link := filepath.Join(root, rel)
			if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
				t.Fatalf("create the fixture directory: %v", err)
			}
			if err := os.Symlink(filepath.Join(root, "nowhere"), link); err != nil {
				t.Fatalf("create the dangling link: %v", err)
			}

			if _, err := derive(root); err == nil {
				t.Fatal("derive passed over a file it could not read")
			}
			if _, code := runCheck(root); code != 1 {
				t.Error("check answered 0 for a tree it could not read")
			}
		})
	}
}

// VALIDATES: a line too long for the scanner stops the run rather than ending
// the read.
// PREVENTS: the same silent drop by a second route. bufio.Scanner stops at 64
// KiB and says so only through Err(), so a generated source with one long line
// would read as a file that imports nothing.
func TestALineTooLongToScanStopsTheRun(t *testing.T) {
	root := fixture(t)
	write(t, root, "internal/plugins/host/yang/register.go",
		"package yang\n\nimport (\n\t_ \""+module+"/internal/component/config/yang\"\n)\n\n"+
			"// "+strings.Repeat("x", 128*1024)+"\n")

	if _, err := derive(root); err == nil {
		t.Fatal("derive answered no error for a line it could not scan")
	}
}

// VALIDATES: both answers render themselves in the words the script printed.
// PREVENTS: a report a developer cannot read at a terminal.
func TestBothReportsRenderTheScriptWording(t *testing.T) {
	var current CheckReport
	current.Counts = Counts{Plugins: 2, Schemas: 3, RPCs: 4, Namespaces: 5, GatedGroups: 6}
	want := "internal/component/plugin/all/all.go is current (2 plugins, 3 schemas, 4 rpcs, 5 namespaces, 6 gated groups)\n"
	if got := current.Text(); got != want {
		t.Errorf("a current check renders\n%q\nwant\n%q", got, want)
	}

	stale := CheckReport{Stale: "internal/component/plugin/all/all.go", Reason: reasonStale}
	if got := stale.Text(); got != "plugin_imports: internal/component/plugin/all/all.go is stale; run make generate\n" {
		t.Errorf("a stale check renders %q", got)
	}

	var written WriteReport
	written.Plugins = 1
	wantWritten := "Generated internal/component/plugin/all/all.go with 1 plugins, 0 schemas, 0 rpcs, 0 namespaces, 0 gated groups\n"
	if got := written.Text(); got != wantWritten {
		t.Errorf("a write renders\n%q\nwant\n%q", got, wantWritten)
	}
}

func readFile(t *testing.T, root, rel string) string {
	t.Helper()

	body, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(body)
}
