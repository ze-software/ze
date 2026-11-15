// Design: docs/architecture/plugin/feature-gates.md -- registration references require exact lexical ownership.
package pluginregistrationsites

import (
	"strings"
	"testing"
)

// TestRegistrationSitesLexicalShadows preserves cross-file init chains while
// excluding same-spelled local values, parameters and registry qualifiers.
func TestRegistrationSitesLexicalShadows(t *testing.T) {
	const register = "internal/plugins/example/register.go"
	report := committedSites(t, map[string]string{
		register: "package example\n" + registryLine +
			"func install() { _ = registry.Register(registry.Registration{Name: \"lexical\"}) }\n" +
			"func init() { install() }\n",
		"internal/plugins/example/register_other.go": `package example
func init() { install() }
func local() { install := func() {}; _ = install; install() }
func parameter(install func()) { _ = install }
type localRegistry struct { Register func(string) }
func qualifier(registry localRegistry) { registry.Register("local") }
`,
	})
	if len(report.Refused) != 0 {
		t.Fatalf("proven local shadows contaminated initialization: %s", report.Text())
	}
	if len(report.Accepted) != 1 {
		t.Fatalf("direct cross-file initialization lost its registering site: %s", report.Text())
	}
	if report.Accepted[0].File != register {
		t.Fatalf("local registry qualifier became a registration: %+v", report.Accepted)
	}
}

// TestRegistrationSitesLexicalUnknowns refuses an unprovable registry import
// alias and a cross-file registering helper used as a value.
func TestRegistrationSitesLexicalUnknowns(t *testing.T) {
	for _, row := range []struct{ name, body, cause string }{
		{"registry alias", "var alias = registry.Register\nfunc init() { _ = alias }\n", "used as a value"},
		{"cross-file helper value", "func install() { _ = registry.Register(registry.Registration{Name: \"x\"}) }\n", "used as a value"},
	} {
		t.Run(row.name, func(t *testing.T) {
			files := map[string]string{
				"internal/plugins/example/register.go": "package example\n" + registryLine + row.body,
			}
			if row.name == "cross-file helper value" {
				files["internal/plugins/example/use.go"] = "package example\nvar runtime = install\n"
			}
			report := committedSites(t, files)
			if len(report.Refused) != 1 {
				t.Fatalf("uncertain reference escaped refusal: %s", report.Text())
			}
			if !strings.Contains(report.Refused[0].Rule, row.cause) {
				t.Fatalf("refusal lost its reference cause: %s", report.Text())
			}
		})
	}
}

// TestRegistrationSitesUnresolvedRegistryQualifier refuses a syntactically
// retained import alias whose duplicate declarations make ownership ambiguous.
// An initialization call alone cannot establish which package owns the callee.
func TestRegistrationSitesUnresolvedRegistryQualifier(t *testing.T) {
	report := committedSites(t, map[string]string{
		"internal/plugins/example/register.go": `package example
import registry "github.com/ze-software/ze/internal/component/plugin/registry"
import registry "strings"
func init() { _ = registry.Register(registry.Registration{Name: "uncertain"}) }
`,
	})
	if len(report.Refused) != 1 {
		t.Fatalf("unresolved registry qualifier escaped refusal: %s", report.Text())
	}
	if !strings.Contains(report.Refused[0].Rule, "unresolved import ownership") {
		t.Fatalf("unresolved registry qualifier lost its ownership cause: %s", report.Text())
	}
	if len(report.Accepted) != 0 {
		t.Fatalf("unresolved registry qualifier was accepted: %s", report.Text())
	}
}
