package wikicatalog

import (
	"testing"

	"github.com/ze-software/ze/internal/component/command"
)

// VALIDATES: AC-8 at the wiki producer. Every column order, answer shape and
// address-field list this catalog publishes comes from
// command.DeclaredForCommand, which reads the declaration registries and then a
// plugin's registry.Registration.Commands. cmd/ze/help_command.go reads the
// same function, so the two catalogs agree by derivation rather than because
// compareWikiCatalogProducer reconciles them afterwards.
// PREVENTS: a second join in this package that reads the registries directly
// and drifts from the product binary's catalog.
func TestWikiCatalogDerivesEveryDeclarationFromOneReader(t *testing.T) {
	entries := Collect()
	if len(entries) == 0 {
		t.Fatal("the catalog is empty, so this test proves nothing")
	}

	checked := 0
	for _, entry := range entries {
		if entry.AnswerShape == "" && len(entry.ColumnOrders) == 0 && len(entry.AddressFields) == 0 {
			continue
		}
		checked++
		declared := command.DeclaredForCommand(entry.Path)
		assertSameOrders(t, entry.Path, command.ColumnNames(declared.Columns), entry.ColumnOrders)
		assertSameNames(t, entry.Path, "address field", declared.AddressFields, entry.AddressFields)
		if entry.AnswerShape == "" {
			continue
		}
		if !declared.ShapeDeclared {
			t.Errorf("%q publishes shape %q and the declaration reader calls it undeclared",
				entry.Path, entry.AnswerShape)
			continue
		}
		if got := declared.Shape.String(); got != entry.AnswerShape {
			t.Errorf("%q publishes shape %q and the declaration reader answers %q",
				entry.Path, entry.AnswerShape, got)
		}
	}
	if checked < 10 {
		t.Fatalf("only %d declaring commands were compared", checked)
	}
}

// VALIDATES: AC-5 at the wiki producer. The published catalog carries the
// column order a command declares. It published none before this, so the wiki
// stated which operators a command supports and never the order of its columns.
// PREVENTS: a catalog that names the rows and stays silent about their keys.
func TestWikiCatalogReportsTheDeclaredColumnOrder(t *testing.T) {
	const path = "show env list"
	want := []string{"key", "type", "default", "current", "description"}

	for _, entry := range Collect() {
		if entry.Path != path {
			continue
		}
		assertSameOrders(t, path, [][]string{want}, entry.ColumnOrders)
		return
	}
	t.Fatalf("the catalog names no %q", path)
}

func assertSameOrders(t *testing.T, path string, want, got [][]string) {
	t.Helper()
	if len(want) != len(got) {
		t.Errorf("%q publishes %d column orders, want %d", path, len(got), len(want))
		return
	}
	for index := range want {
		assertSameNames(t, path, "column", want[index], got[index])
	}
}

func assertSameNames(t *testing.T, path, subject string, want, got []string) {
	t.Helper()
	if len(want) != len(got) {
		t.Errorf("%q publishes %d %s names %v, want %v", path, len(got), subject, got, want)
		return
	}
	for index := range want {
		if want[index] != got[index] {
			t.Errorf("%q publishes %s names %v, want %v", path, subject, got, want)
			return
		}
	}
}

// VALIDATES: AC-3 at the wiki producer, the twin of
// TestHelpCommandNamesAPluginCommand in cmd/ze. The two catalogs are compared
// field for field (internal/le/docvalid, compareWikiCatalogProducer), so a
// plugin command reaching one and not the other is a gate failure with a
// confusing message; this states the requirement where a reader of this package
// meets it.
// PREVENTS: the wiki and the website publishing a command list that omits every
// purely plugin-provided command.
func TestWikiCatalogNamesAPluginCommand(t *testing.T) {
	const path = "show bgp rpki roa"

	for _, entry := range Collect() {
		if entry.Path != path {
			continue
		}
		if entry.AnswerShape != "tab" {
			t.Errorf("%q publishes shape %q, want tab", path, entry.AnswerShape)
		}
		assertSameOrders(t, path, [][]string{{"prefix", "max-length", "asn"}}, entry.ColumnOrders)
		assertSameNames(t, path, "address field", []string{"prefix"}, entry.AddressFields)
		if entry.Description == "" {
			t.Errorf("%q publishes no summary", path)
		}
		return
	}
	t.Fatalf("the catalog names no %q, so no plugin command reached it", path)
}

// VALIDATES: AC-2 at the wiki producer. A pipe alias a plugin declares reaches
// the published catalog, because registry.Registration.Pipes carries it into
// the compiled tree; until now it existed on the Stage 1 message alone.
// PREVENTS: the website and the wiki listing a plugin's commands without the
// names they answer to.
func TestWikiCatalogReportsAPluginDeclaredAlias(t *testing.T) {
	const path = "show bgp rpki"

	for _, entry := range Collect() {
		if entry.Path != path {
			continue
		}
		if len(entry.Aliases) != 1 {
			t.Fatalf("%q publishes %d aliases, want the one the plugin declares", path, len(entry.Aliases))
		}
		alias := entry.Aliases[0]
		if alias.Name != "summary" {
			t.Errorf("%q publishes alias %q, want summary", path, alias.Name)
		}
		if alias.Description == "" || alias.Expansion == "" {
			t.Errorf("alias %q publishes description %q and expansion %q; both are owed",
				alias.Name, alias.Description, alias.Expansion)
		}
		return
	}
	t.Fatalf("the catalog names no %q", path)
}
