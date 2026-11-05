// Design: docs/architecture/plugin/feature-gates.md -- declaration aliases cannot publish stale registration metadata.
package repoinventory

import (
	"strings"
	"testing"
)

// TestCommittedRegistrationValueSpecAliasCannotPublishStaleMetadata exercises
// a var declaration rather than the short declaration already covered elsewhere.
// Both public consumers must refuse mutation or report the value actually passed.
func TestCommittedRegistrationValueSpecAliasCannotPublishStaleMetadata(t *testing.T) {
	root := retainedControlFixture(t)
	const file = "internal/plugins/example/value_alias.go"
	const before = `package example
import r "github.com/ze-software/ze/internal/component/plugin/registry"
func aliasEngine() {}
func init() {
 reg := r.Registration{Name: "alias-before", RunEngine: aliasEngine}
 var alias = &reg
 _ = alias
 _ = r.Register(reg)
}
`
	write(t, root, file, before)
	commitInventoryFixture(t, root)
	for _, changed := range []bool{false, true} {
		want := "alias-before"
		if changed {
			want = "alias-after"
			write(t, root, file, strings.Replace(before, "_ = alias", `alias.Name = "alias-after"`, 1))
			commitInventoryFixture(t, root)
		}
		inv, reportErr := Collect(root)
		plugins, pluginErr := Plugins(root)
		if reportErr == nil {
			if pluginErr != nil {
				t.Fatalf("alias consumers disagree: report succeeded, plugins refused: %v", pluginErr)
			}
		} else {
			if !changed {
				t.Fatalf("ordinary alias declaration control refused: report %v, plugins %v", reportErr, pluginErr)
			}
			if pluginErr == nil {
				t.Fatalf("alias consumers disagree: report refused %v, plugins succeeded", reportErr)
			}
			if inv.Plugins != nil {
				t.Fatalf("alias report refusal leaked partial output: %+v", inv.Plugins)
			}
			if plugins != nil {
				t.Fatalf("alias plugin refusal leaked partial output: %+v", plugins)
			}
			for _, err := range []error{reportErr, pluginErr} {
				if !strings.Contains(err.Error(), file+":") {
					t.Fatalf("alias mutation lost source attribution: %v", err)
				}
				if !strings.Contains(err.Error(), "mutat") {
					t.Fatalf("alias mutation masked by unrelated refusal: %v", err)
				}
			}
			continue
		}
		for _, got := range [][]Plugin{inv.Plugins, plugins} {
			if len(got) != 2 {
				t.Fatalf("registration alias membership = %+v, want two plugins", got)
			}
			if got[0].Name != want {
				t.Fatalf("registration alias published stale membership: %+v; want %s", got, want)
			}
			if got[1].Name != "committed-linux" {
				t.Fatalf("registration alias published stale membership: %+v; want %s and committed-linux", got, want)
			}
		}
	}
}
