package ls_export

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/configneedtest"
)

// TestConfigNeedPathsExistInSchema checks that every config need this plugin
// declares names a leaf its own YANG holds (`bgp-ls-export domain <entry>
// source`), crossing the `domain` list through registry.AnyEntry. Method: walk
// each need path against the loaded schema. A renamed leaf, or the list read
// as a container, would make the need hold for no config, so the IGP would
// never be pulled and its absence never refused.
func TestConfigNeedPathsExistInSchema(t *testing.T) {
	configneedtest.RequirePathsInSchemaForTest(t, exporterName)
}

// TestDomainNamingACompiledOutSourceIsRefused proves AC-13 through the
// validate and commit door: a `domain` entry whose source names an IGP this
// build left out is refused, naming the entry, the plugin and its build tag
// on the fix line, while a source the build carries is accepted. Method: this
// test binary links the exporter and BGP but neither IS-IS nor OSPF, which is
// the build an operator without ze_isis or ze_ospf runs; parse real config
// text and run config.VerifyPluginConfig over it.
//
// VALIDATES: the exporter's isis and ospf needs, evaluated through a list.
// PREVENTS: an export domain mapping a source no compiled-in plugin publishes,
// accepted and then exporting nothing.
func TestDomainNamingACompiledOutSourceIsRefused(t *testing.T) {
	cases := []struct {
		source string
		tag    string
	}{
		{source: isisSource, tag: "ze_isis"},
		{source: ospfSource, tag: "ze_ospf"},
	}
	for _, tc := range cases {
		t.Run(tc.source, func(t *testing.T) {
			errs := config.VerifyPluginConfig(parseDomains(t, "core", tc.source))
			var refusal error
			for _, err := range errs {
				if errors.Is(err, config.ErrAbsentOptionalProvider) {
					refusal = err
				}
			}
			if refusal == nil {
				t.Fatalf("domain naming %s in a build without it: errors = %v, want the absent-provider refusal", tc.source, errs)
			}
			message := refusal.Error()
			setting := `"bgp-ls-export domain core source ` + tc.source + `"`
			if !strings.Contains(message, setting+" needs plugin "+tc.source) {
				t.Errorf("refusal does not name the entry %s and plugin %s:\n%s", setting, tc.source, message)
			}
			if !strings.Contains(message, "\n  fix: rebuild ze with build tag "+tc.tag) {
				t.Errorf("refusal does not carry the fix line naming %s:\n%s", tc.tag, message)
			}
		})
	}

	if errs := config.VerifyPluginConfig(parseDomains(t, "edge", "bgp-epe")); len(errs) != 0 {
		t.Fatalf("domain naming bgp-epe, which this build carries: errors = %v, want none", errs)
	}
}

// parseDomains parses one `bgp-ls-export` domain entry named name with source,
// carrying the leaves the exporter's verifier requires.
func parseDomains(t *testing.T, name, source string) *config.Tree {
	t.Helper()
	text := "bgp-ls-export {\n\tdomain " + name + " {\n\t\tsource " + source + ";\n\t\tprotocol-id 2;\n\t\tnative-instance 1;\n\t\tnative-area 0;\n\t\tinstance-id 7;\n\t}\n}\n"
	tree, err := config.ParseTreeForValidation(text)
	if err != nil {
		t.Fatalf("parse %q: %v", text, err)
	}
	return tree
}
