// Design: docs/architecture/api/commands.md -- the live command catalog
//go:build ze_bgp

package clicatalog

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
)

// TestCatalogFlowspecComponentAppearsInEveryProjection follows the actual
// compiled product catalog, including the flowspec module's augment. Its
// traffic-class component must survive as a subcommand, a grammar group and
// the invocation that renders that same grammar.
func TestCatalogFlowspecComponentAppearsInEveryProjection(t *testing.T) {
	const path = "send bgp flowspec"
	for _, entry := range collectForTest(t) {
		if entry.Path != path {
			continue
		}
		if !slices.Contains(entry.Subcommands, "traffic-class") {
			t.Errorf("%s subcommands omit traffic-class: %v", path, entry.Subcommands)
		}
		index := slices.IndexFunc(entry.Grammar, func(token command.UsageToken) bool {
			return token.Text == "traffic-class"
		})
		if index < 0 {
			t.Fatalf("%s grammar omits traffic-class: %+v", path, entry.Grammar)
		}
		component := entry.Grammar[index]
		if component.Kind != command.UsageGroupRepeat {
			t.Errorf("traffic-class kind = %s, want %s", component.Kind, command.UsageGroupRepeat)
		}
		if len(component.Group) != 1 {
			t.Fatalf("traffic-class values = %+v, want one value", component.Group)
		}
		if component.Group[0].Text != "value" {
			t.Errorf("traffic-class value = %q, want value", component.Group[0].Text)
		}
		if !strings.Contains(entry.Usage, "[traffic-class <value> ...]") {
			t.Errorf("%s usage omits the complete traffic-class form: %s", path, entry.Usage)
		}
		if got := command.UsageLine(entry.Grammar); entry.Usage != got {
			t.Errorf("catalog usage = %q, captured grammar renders %q", entry.Usage, got)
		}
		return
	}
	t.Fatalf("the compiled product catalog names no %s", path)
}
