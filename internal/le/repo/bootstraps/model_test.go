// Related: tracked.go -- the model of the composition these cases pin
//
// The three ways the model drifted from the le it judges: a command three
// words deep, the launcher rows on the root page, and a hidden tool. Each case
// builds the smallest input that shows one drift, so a red names its cause.

package repobootstraps

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// leRootImport is the import a fixture package declares to reach leroot.
const leRootImport = `import leroot "github.com/ze-software/ze/internal/le/le/root"
`

// writeTree writes files, keyed by their path under root, into root.
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

// VALIDATES: the walk reads a package at every depth the dispatcher reads a
// command word, so `le setup docker kernel` (internal/le/setup/docker/kernel)
// is a tool, and stops there, so a package one directory deeper is not.
// PREVENTS: a three-word tool the composition root imports being reported as
// "registers no command", which the walk said of ai/rules/report and
// setup/docker/kernel while it stopped at two directories.
func TestTheWalkReachesEveryCommandWordTheDispatcherReads(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/le/setup/register.go": "package setup\n" + leRootImport +
			"func init() { leroot.RegisterNamespace(\"setup\", \"s\", \"d\") }\n",
		"internal/le/setup/docker/register.go": "package docker\n" + leRootImport +
			"func init() { leroot.RegisterNamespace(\"setup docker\", \"s\", \"d\") }\n",
		"internal/le/setup/docker/kernel/register.go": "package kernel\n" + leRootImport +
			"func init() { leroot.Register(\"setup docker kernel\", 0, nil, nil) }\n",
		"internal/le/setup/docker/kernel/deeper/register.go": "package deeper\n" + leRootImport +
			"func init() { leroot.Register(\"setup docker kernel deeper\", 0, nil, nil) }\n",
		"internal/le/setup/docker/testdata/broken.go": "this is not Go\n",
	})

	found, err := registeringPackages(root)
	if err != nil {
		t.Fatalf("registeringPackages: %v", err)
	}
	if !slices.Equal(found.tools, []string{"setup/docker/kernel"}) {
		t.Errorf("tools = %v, want [setup/docker/kernel]", found.tools)
	}
	if !slices.Equal(found.namespaces, []string{"setup", "setup/docker"}) {
		t.Errorf("namespaces = %v, want [setup setup/docker]", found.namespaces)
	}
}

// launcherHelpPage is the root page's tail as leroot prints it: the last
// command section, then the launcher block, whose heading carries a summary
// after its colon and whose rows are words the ./le script reads.
const launcherHelpPage = `le - the Ze repository and development entry point

Usage:
  le <command> [options] ['|' json|yaml|table]

yang: YANG modules, their generated glue and path migrations
  yang glue        [generate] check or write the generated YANG glue

Launcher: words ./le reads before le starts; they come first
  --name <name>    build the working tree as bin/le-<name>/le and run it
  --name=<name>    the same as --name <name>, written as one word
`

// VALIDATES: the launcher block's rows are not read as commands, and the
// command section above it, whose heading also carries a summary, still is.
// PREVENTS: the two `--name` rows counted as offered commands, which made the
// binary offer two more commands than the composition wires.
func TestTheLauncherSectionListsNoCommand(t *testing.T) {
	got := commandNames(launcherHelpPage)
	if !slices.Equal(got, []string{"yang"}) {
		t.Errorf("CommandNames = %v, want [yang]", got)
	}
}

// VALIDATES: a package that registers its tool and marks it hidden is a tool
// the composition root must import, and gives the root page no row, so a
// commit that wires it and offers only the visible tool is clean.
// PREVENTS: the spawned mocks (`le mock engine-steps`) counted as rows the
// root page leaves out.
func TestAHiddenToolIsWiredAndOffersNoRow(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/le/mock/spawned/register.go": "package spawned\n" + leRootImport +
			"func init() {\n\tleroot.Register(\"mock spawned\", 0, nil, nil)\n\tleroot.RegisterHidden(\"mock spawned\")\n}\n",
		"internal/le/visible/register.go": "package visible\n" + leRootImport +
			"func init() { leroot.Register(\"visible\", 0, nil, nil) }\n",
	})

	found, err := registeringPackages(root)
	if err != nil {
		t.Fatalf("registeringPackages: %v", err)
	}
	if !slices.Equal(found.tools, []string{"mock/spawned", "visible"}) {
		t.Errorf("tools = %v, want [mock/spawned visible]", found.tools)
	}
	if !slices.Equal(found.hidden, []string{"mock/spawned"}) {
		t.Errorf("hidden = %v, want [mock/spawned]", found.hidden)
	}

	imported := []string{"mock/spawned", "visible"}
	if broken := Compare(imported, found.tools, found.namespaces, found.hidden, []string{"visible"}); len(broken) != 0 {
		t.Errorf("a wired hidden tool with no row was reported broken: %v", broken)
	}
	if broken := Compare([]string{"visible"}, found.tools, found.namespaces, found.hidden, []string{"visible"}); len(broken) != 1 {
		t.Errorf("a hidden tool the root does not import gave %v, want one finding", broken)
	}
}
