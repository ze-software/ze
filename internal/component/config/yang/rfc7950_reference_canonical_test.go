// VALIDATES: a reference names a configured value, a list key or a default
// in use by the value's canonical form under the referred leaf's type, so a
// configured 17 and a default 17 answer "+17", "017" and "17" alike.
// PREVENTS: a reference accepted against a default and refused against the
// same value configured, because one side compared written text.

package yang

import (
	"maps"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// canonicalModule holds integer and decimal64 leaf-lists, configured and
// defaulted, and lists keyed by them, which an instance-identifier and a
// predicated leafref name.
const canonicalModule = `module cv-conf {
    yang-version 1.1;
    namespace "urn:ze:test:cv";
    prefix cv;
    container box {
        leaf-list num { type uint8; }
        leaf-list dnum { type uint8; default 17; }
        leaf-list rate { type decimal64 { fraction-digits 2; } }
        leaf-list drate { type decimal64 { fraction-digits 2; } default 1.5; }
        list port { key id; leaf id { type uint8; } leaf name { type string; } }
        list pair {
            key "a b";
            leaf a { type uint8; }
            leaf b { type decimal64 { fraction-digits 2; } }
            leaf name { type string; }
        }
        leaf target { type instance-identifier; }
        leaf sel { type uint8; }
        leaf rate-sel { type decimal64 { fraction-digits 2; } }
        leaf id-ref { type leafref { path "../port/id"; } }
        leaf name-ref { type leafref { path "../port[id = current()/../sel]/name"; } }
        leaf pair-ref { type leafref { path "../pair[a = current()/../sel][b = current()/../rate-sel]/name"; } }
    }
}`

// canonicalValidator loads canonicalModule.
func canonicalValidator(t *testing.T) *Validator {
	t.Helper()
	loader := NewLoader()
	require.NoError(t, loader.AddModuleFromText("cv-conf.yang", canonicalModule))
	resolved, err := loader.Resolve()
	require.NoError(t, err)
	return NewValidator(resolved)
}

// canonicalBox answers box with num holding 17, rate holding 1.5, port 17
// named "a" and pair "17 1.5" named "b", dnum and drate absent so their
// defaults are in use, plus the leaves in extra.
func canonicalBox(extra map[string]any) map[string]any {
	box := map[string]any{
		"num":  []string{"17"},
		"rate": []string{"1.5"},
		"port": map[string]any{"17": map[string]any{"name": "a"}},
		"pair": map[string]any{"17 b 1.5": map[string]any{"name": "b"}},
	}
	maps.Copy(box, extra)
	return map[string]any{"box": box}
}

// TestRFC7950InstanceIdentifierCanonicalValue accepts an instance-identifier
// whose leaf-list-predicate or key predicate writes the referred value in
// another lexical form, for a configured value as for a default in use.
//
// RFC requirement: RFC7950-9.13-1 positive -- ValidateReferences accepts target "/cv:box/cv:num[.='v']" (num configured holding "17") and "/cv:box/cv:dnum[.='v']" (dnum absent, default 17) for v "17", "+17" and "017", "/cv:box/cv:port[cv:id='v']" for the configured port key 17, "/cv:box/cv:rate[.='1.50']" and "/cv:box/cv:drate[.='1.50']" against the decimal64 1.5 configured and defaulted, and "/cv:box/cv:pair[cv:a='+17'][cv:b='1.50']" against the two-key entry "17 1.5".
func TestRFC7950InstanceIdentifierCanonicalValue(t *testing.T) {
	v := canonicalValidator(t)
	for _, value := range []string{"17", "+17", "017"} {
		for _, target := range []string{
			"/cv:box/cv:num[.='" + value + "']",
			"/cv:box/cv:dnum[.='" + value + "']",
			"/cv:box/cv:port[cv:id='" + value + "']",
		} {
			assert.Empty(t, v.ValidateReferences(canonicalBox(map[string]any{"target": target})), target)
		}
	}
	for _, target := range []string{
		"/cv:box/cv:rate[.='1.50']",
		"/cv:box/cv:drate[.='1.50']",
		"/cv:box/cv:pair[cv:a='+17'][cv:b='1.50']",
	} {
		assert.Empty(t, v.ValidateReferences(canonicalBox(map[string]any{"target": target})), target)
	}
}

// TestRFC7950LeafrefPredicateCanonicalValue accepts a leafref whose path
// predicate reads a key value written in another lexical form than the list
// key it selects.
//
// RFC requirement: RFC7950-9.9-1 positive -- ValidateReferences accepts name-ref "a" through the predicate port[id = current()/../sel] with sel "+17" and "017" against the configured port key 17, pair-ref "b" through pair[a = current()/../sel][b = current()/../rate-sel] with sel "+17" and rate-sel "1.50" against the entry "17 1.5", and id-ref "+17" against the port key 17.
func TestRFC7950LeafrefPredicateCanonicalValue(t *testing.T) {
	v := canonicalValidator(t)
	for _, sel := range []string{"17", "+17", "017"} {
		assert.Empty(t, v.ValidateReferences(canonicalBox(map[string]any{"sel": sel, "name-ref": "a"})), sel)
	}
	assert.Empty(t, v.ValidateReferences(canonicalBox(map[string]any{"sel": "+17", "rate-sel": "1.50", "pair-ref": "b"})))
	assert.Empty(t, v.ValidateReferences(canonicalBox(map[string]any{"id-ref": "+17"})))
}

// TestReferenceCanonicalValueRefusesOther refuses a reference whose value is
// another number than every configured and defaulted instance, so the
// canonical comparison widens no match beyond equal values.
func TestReferenceCanonicalValueRefusesOther(t *testing.T) {
	v := canonicalValidator(t)
	for _, target := range []string{
		"/cv:box/cv:num[.='+18']",
		"/cv:box/cv:dnum[.='018']",
		"/cv:box/cv:port[cv:id='+18']",
		"/cv:box/cv:rate[.='1.51']",
		"/cv:box/cv:drate[.='1.51']",
		"/cv:box/cv:pair[cv:a='+17'][cv:b='1.51']",
	} {
		requireRefused(t, v.ValidateReferences(canonicalBox(map[string]any{"target": target})), "target")
	}
	requireRefused(t, v.ValidateReferences(canonicalBox(map[string]any{"sel": "+18", "name-ref": "a"})), "name-ref")
	requireRefused(t, v.ValidateReferences(canonicalBox(map[string]any{"sel": "+17", "rate-sel": "1.51", "pair-ref": "b"})), "pair-ref")
}
