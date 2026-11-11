// Design: docs/architecture/plugin/feature-gates.md -- a registered schema's directory comes from its registration.
package repoinventory

import (
	"context"
	"strings"
	"testing"
)

// moduleSourceFixture commits a tree whose registrations name their schema
// three ways: a plugin's //go:embed file with copies of the same module under
// a testdata directory, an embed of a file in a subdirectory, and a Go string
// literal whose module name a committed .yang file elsewhere also declares.
// extra adds or replaces files before the commit.
func moduleSourceFixture(t *testing.T, extra map[string]string) *Snapshot {
	t.Helper()
	root := committedInventoryFixture(t)
	write(t, root, "internal/component/config/yang/register.go", "package yang\nfunc RegisterModule(name, content string) {}\n")
	write(t, root, "internal/component/plugin/registry/external_witness.go", "package registry\nimport _ \"embed\"\n")
	write(t, root, "internal/component/config/yang/external_witness.go", "package yang\nimport _ \"embed\"\n")
	const aaaModule = "module ze-aaa-cmd { namespace urn:ze-aaa-cmd; prefix aaa; }\n"
	write(t, root, "internal/plugins/aaa-cmd/yang/embed.go", "package yang\nimport _ \"embed\"\n//go:embed ze-aaa-cmd.yang\nvar ZeAAACmdYANG string\n")
	write(t, root, "internal/plugins/aaa-cmd/yang/register.go", `package yang
import configyang "github.com/ze-software/ze/internal/component/config/yang"
func init() { configyang.RegisterModule("ze-aaa-cmd.yang", ZeAAACmdYANG) }
`)
	write(t, root, "internal/plugins/aaa-cmd/yang/ze-aaa-cmd.yang", aaaModule)
	write(t, root, "internal/le/yang/migration/testdata/commands/one/ze-aaa-cmd.yang", aaaModule)
	write(t, root, "internal/le/yang/migration/testdata/commands/two/ze-aaa-cmd.yang", aaaModule)
	write(t, root, "internal/extra/register.go", `package extra
import (
 _ "embed"
 schema "github.com/ze-software/ze/internal/component/config/yang"
)
//go:embed schema/sub.yang
var Sub string
func init() {
 schema.RegisterModule("sub.yang", Sub)
 schema.RegisterModule("literal.yang", "module literal { namespace urn:literal; prefix l; }")
}
`)
	write(t, root, "internal/extra/schema/sub.yang", "module sub { namespace urn:sub; prefix s; }\n")
	write(t, root, "internal/plugins/other/yang/literal.yang", "module literal { namespace urn:literal; prefix l; }\n")
	write(t, root, "cmd/ze/main.go", "package main\nimport (\n _ \"github.com/ze-software/ze/internal/thing\"\n _ \"github.com/ze-software/ze/internal/extra\"\n _ \"github.com/ze-software/ze/internal/plugins/aaa-cmd/yang\"\n)\nfunc main() {}\n")
	for name, body := range extra {
		write(t, root, name, body)
	}
	commitInventoryFixture(t, root)
	snapshot, err := ReadCommitted(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

// moduleDirectories reads the fixture's registrations and returns the
// directory recorded for each registered module filename.
func moduleDirectories(t *testing.T, snapshot *Snapshot) map[string]string {
	t.Helper()
	reader, err := snapshot.declaredRegistrations()
	if err != nil {
		t.Fatalf("registrations refused: %v", err)
	}
	return reader.modules
}

// TestRegisteredModuleDirectoryIsItsEmbeddedFile proves a plugin's embedded
// module is not refused as ambiguous when testdata holds committed copies of
// the same module, and that its directory, and so its plugin, is the embed's.
// MUTATION: return textOriginMixed instead of textOriginEmbed from
// embeddedText, and the registration is refused.
func TestRegisteredModuleDirectoryIsItsEmbeddedFile(t *testing.T) {
	snapshot := moduleSourceFixture(t, nil)
	if got := moduleDirectories(t, snapshot)["ze-aaa-cmd.yang"]; got != "internal/plugins/aaa-cmd/yang" {
		t.Fatalf("ze-aaa-cmd.yang directory = %q, want the embedded file's internal/plugins/aaa-cmd/yang", got)
	}
	inv, err := snapshot.collect()
	if err != nil {
		t.Fatal(err)
	}
	for _, module := range inv.YANGModules {
		if module.Name == "ze-aaa-cmd.yang" && module.Source == "plugin:aaa-cmd" {
			return
		}
	}
	t.Fatalf("ze-aaa-cmd.yang missing or not attributed to plugin:aaa-cmd: %+v", inv.YANGModules)
}

// TestRegisteredModuleDirectoryFollowsAnEmbeddedSubdirectory proves the
// directory is the embedded file's own, not the registering package's, when
// the embed pattern names a file in a subdirectory.
// MUTATION: record function.pkg.dir for textOriginEmbed in registerModule.
func TestRegisteredModuleDirectoryFollowsAnEmbeddedSubdirectory(t *testing.T) {
	snapshot := moduleSourceFixture(t, nil)
	if got := moduleDirectories(t, snapshot)["sub.yang"]; got != "internal/extra/schema" {
		t.Fatalf("sub.yang directory = %q, want the embedded file's internal/extra/schema", got)
	}
}

// TestRegisteredLiteralModuleBelongsToItsRegisteringPackage pins the
// non-embed decision: a Go string names no committed file, so the module's
// directory is the registering package, never a committed .yang file that
// happens to declare the same module name.
// MUTATION: record path.Dir(origin.file) for textOriginGo in registerModule.
func TestRegisteredLiteralModuleBelongsToItsRegisteringPackage(t *testing.T) {
	snapshot := moduleSourceFixture(t, nil)
	if got := moduleDirectories(t, snapshot)["literal.yang"]; got != "internal/extra" {
		t.Fatalf("literal.yang directory = %q, want the registering package internal/extra", got)
	}
}

// TestRegisteredModuleJoiningAnEmbedIsRefused proves content that joins an
// embedded file with other text is refused with its reason, because no single
// committed file is then its source.
// MUTATION: start the concatenation origin as textOriginGo in textFrom.
func TestRegisteredModuleJoiningAnEmbedIsRefused(t *testing.T) {
	snapshot := moduleSourceFixture(t, map[string]string{
		"internal/extra/mixed.go": "package extra\nimport schema \"github.com/ze-software/ze/internal/component/config/yang\"\nfunc init() { schema.RegisterModule(\"mixed.yang\", Sub + \"\\n\") }\n",
	})
	_, err := snapshot.declaredRegistrations()
	if err == nil || !strings.Contains(err.Error(), "joins an embedded file with other text") {
		t.Fatalf("mixed registration error = %v, want the joined-embed refusal", err)
	}
}
