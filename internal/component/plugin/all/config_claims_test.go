package all

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/claims"
	schemacli "github.com/ze-software/ze/internal/component/config/schema/cli"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// liveInventory resolves the config-schema tree and the claim union exactly as
// the daemon produces them: the YANG loader for the schema, the plugin registry
// for Registration.ConfigRoots (which becomes WantsConfigRoots at Stage 1,
// internal/component/plugin/server/startup.go:871), and the schema registry for
// hub handler paths.
//
// The `all` package imports every registration by construction, so both sides
// are the live inventories rather than a second list. Nothing here is written
// down twice.
func liveInventory(t *testing.T) (*claims.Node, []claims.Claim) {
	t.Helper()

	loader := configyang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		t.Fatalf("load embedded YANG modules: %v", err)
	}
	// LoadRegistered and Resolve are best-effort in DefaultLoader because the
	// command tree does not need every module. This gate does, so a failure
	// here is a failure of the gate: it would silently shrink the checked
	// surface, which is the exact defect class the gate exists to catch.
	if err := loader.LoadRegistered(); err != nil {
		t.Fatalf("load registered YANG modules: %v", err)
	}
	if err := loader.Resolve(); err != nil {
		t.Fatalf("resolve YANG modules: %v", err)
	}

	root, err := claims.SchemaTree(loader)
	if err != nil {
		t.Fatalf("build config schema tree: %v", err)
	}

	cs := claims.FromConfigRoots(registry.ConfigRootsMap())
	handlers, err := schemacli.ConfigHandlerPaths()
	if err != nil {
		t.Fatalf("build schema handler paths: %v", err)
	}
	cs = append(cs, claims.FromHubHandlers(handlers)...)

	return root, cs
}

// TestConfigSchemaRootsClaimed fails when a config subtree an operator can
// write is delivered to nobody.
//
// The daemon selects plugins for a config change by matching the changed path
// against Registration.WantsConfigRoots (Server.reloadConfig, reload.go:214-248,
// prefix rule in rootHasChanges, reload.go:297-319). When no surface matches, the
// producer logs Info "config reload: no affected plugins, updating config" and
// stores the tree anyway (reload.go:251-256). The config is accepted, validated
// by nothing that rejects it (validateContainerEntry validates only keys already
// in the schema dir, validator.go:527, no else) and delivered nowhere.
//
// VALIDATES: AC-1, AC-2, AC-6 -- every config-schema subtree is covered by a
// plugin config root, a hub handler path, or a recorded allowlist entry.
// PREVENTS: a YANG config module added without the matching ConfigRoots entry,
// which the operator experiences as config that saves cleanly and does nothing.
func TestConfigSchemaRootsClaimed(t *testing.T) {
	root, cs := liveInventory(t)

	allow, err := claims.Allowlist()
	if err != nil {
		t.Fatalf("load allowlist: %v", err)
	}

	report := claims.Audit(root, cs, allow)

	// Non-vacuity. Audit over an empty tree or an empty claim set reports an
	// unclassifiable finding rather than passing, but a tree that lost most of
	// its modules (a tag set that compiled them out, a loader change) would
	// still pass while guarding almost nothing. Floors, not counts: 36 roots
	// and 68 claiming plugins on 2026-08-03.
	const (
		minRoots  = 25
		minClaims = 50
	)
	if got := len(root.Children); got < minRoots {
		t.Errorf("config schema tree has only %d top-level roots (floor %d): enumeration is broken and this gate is guarding almost nothing", got, minRoots)
	}
	if len(cs) < minClaims {
		t.Errorf("only %d claims enumerated (floor %d): the registry is not populated, so every root would look unclaimed", len(cs), minClaims)
	}

	for _, f := range report.Findings {
		if f.Kind == claims.KindPhantomClaim {
			continue // owned by TestConfigRootsPhantomClaims
		}
		t.Errorf("%s", f.String())
	}
	if t.Failed() {
		t.Logf("claim union (%d) and allowlist (%d) are derived live; add the missing ConfigRoots entry, or record the consumer in %s",
			len(cs), len(allow), claims.AllowlistPath)
	}
}

// TestConfigRootsPhantomClaims fails when a plugin declares a config root that
// names no node in the resolved config schema.
//
// A phantom claim is silent in production for the mirror-image reason an
// unclaimed root is: rootHasChanges (reload.go:297) never matches the typo, so
// the plugin is never in `affected` and never receives the config it asked for.
//
// VALIDATES: AC-3.
// PREVENTS: a typo'd or renamed ConfigRoots entry leaving a plugin permanently
// unconfigured with no error anywhere.
func TestConfigRootsPhantomClaims(t *testing.T) {
	root, cs := liveInventory(t)

	report := claims.Audit(root, cs, nil)
	for _, f := range report.Findings {
		if f.Kind != claims.KindPhantomClaim {
			continue
		}
		t.Errorf("%s", f.String())
	}
}

// TestClaimAllowlistReasons checks the shipped allowlist itself.
//
// Every entry names a path, a reason, and the owner that consumes it. An entry
// whose path is claimed, or names no schema node, is stale and is reported: an
// allowlist that outlives its reason is how a gate stops gating.
//
// VALIDATES: AC-4, AC-5.
// PREVENTS: the allowlist becoming a dumping ground (spec improve-7 R-1).
func TestClaimAllowlistReasons(t *testing.T) {
	allow, err := claims.Allowlist()
	if err != nil {
		t.Fatalf("load allowlist: %v", err)
	}
	if len(allow) == 0 {
		t.Skip("allowlist is empty; nothing to check")
	}

	root, cs := liveInventory(t)
	report := claims.Audit(root, cs, allow)

	for _, f := range report.Findings {
		switch f.Kind {
		case claims.KindAllowlistNoReason, claims.KindAllowlistStale:
			t.Errorf("%s", f.String())
		default:
		}
	}

	// AC-4: a clean run still names what was skipped, so an allowlisted root is
	// visible without reading the JSON.
	if len(report.Allowlisted) != len(allow) {
		t.Errorf("allowlist has %d entries but the report lists %d as skipped: an entry was neither applied nor reported", len(allow), len(report.Allowlisted))
	}
	for _, p := range report.Allowlisted {
		t.Logf("allowlisted: %s", p)
	}
}

// TestFeatureGatedModulesEnumerated proves the gate above ran with the full
// feature set rather than a reduced one.
//
// A module behind //go:build ze_<feature> is registered only when its tag is
// on. Under a reduced tag set its YANG never reaches the loader, its roots are
// never enumerated, and TestConfigSchemaRootsClaimed passes having checked less
// than it claims. feature-gates.txt is the single source of truth for which
// packages are gated (its header says so), so the expectation derives from it.
//
// VALIDATES: AC-6.
// PREVENTS: the claim gate going quietly green because the tag set shrank.
func TestFeatureGatedModulesEnumerated(t *testing.T) {
	root := repoRoot(t)
	gated := gatedYANGModules(t, root)
	if len(gated) == 0 {
		t.Fatal("no feature-gated YANG modules derived from feature-gates.txt: the manifest layout changed and this check now guards nothing")
	}

	loader := configyang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		t.Fatalf("load embedded YANG modules: %v", err)
	}
	if err := loader.LoadRegistered(); err != nil {
		t.Fatalf("load registered YANG modules: %v", err)
	}
	if err := loader.Resolve(); err != nil {
		t.Fatalf("resolve YANG modules: %v", err)
	}

	loaded := make(map[string]bool)
	for _, name := range loader.ModuleNames() {
		loaded[name] = true
	}

	var missing []string
	for module, tag := range gated {
		if !loaded[module] {
			missing = append(missing, module+" (tag "+tag+")")
		}
	}
	sort.Strings(missing)
	for _, m := range missing {
		t.Errorf("feature-gated YANG module %s is absent from the loader: this test run compiled it out, so the config claim gate did not check its roots", m)
	}
}

// repoRoot returns the repository root relative to this package directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return root
}

// gatedYANGModules maps YANG module name to the build tag that gates it.
//
// Derivation, not a list: feature-gates.txt gives "<tag> <package>" per gated
// package; internal/le/yang/glue/yangglue.go gates <pkg>/yang alongside <pkg>, so a
// gated package's .yang files are exactly the modules that disappear with the
// tag. The module name is read from the .yang source.
func gatedYANGModules(t *testing.T, root string) map[string]string {
	t.Helper()

	f, err := os.Open(filepath.Join(root, "feature-gates.txt"))
	if err != nil {
		t.Fatalf("open feature-gates.txt: %v", err)
	}
	defer f.Close() //nolint:errcheck // read-only

	out := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		tag, pkg := fields[0], fields[1]
		yangDir := filepath.Join(root, filepath.FromSlash(pkg), "yang")
		entries, err := os.ReadDir(yangDir)
		if err != nil {
			continue // no yang/ sibling: nothing gated to enumerate
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".yang") {
				continue
			}
			module := yangModuleName(t, filepath.Join(yangDir, e.Name()))
			if module != "" {
				out[module] = tag
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read feature-gates.txt: %v", err)
	}
	return out
}

// yangModuleName reads the module name from a .yang file's module statement.
func yangModuleName(t *testing.T, path string) string {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close() //nolint:errcheck // read-only

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "module ") {
			continue
		}
		name := strings.TrimSpace(strings.TrimPrefix(line, "module "))
		name = strings.TrimSuffix(strings.TrimSpace(strings.TrimSuffix(name, "{")), " ")
		return name
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return ""
}
