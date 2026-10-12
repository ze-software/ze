package yang

import (
	"strings"
	"testing"
)

// mandatoryModule is a schema shaped to separate the three arms of RFC 7950
// Section 7.6.5: a mandatory leaf whose ancestors are all non-presence
// containers (top/mid/need), one below a non-presence container inside a list
// entry (top/item/inner/must), and one below a non-presence container inside a
// presence container (top/knob/sub/val).
const mandatoryModule = `module ze-test-mandatory-conf {
	namespace "urn:ze:test:mandatory";
	prefix tm;
	container top {
		container mid {
			leaf need { type string; mandatory true; }
		}
		list item {
			key name;
			leaf name { type string; }
			container inner {
				leaf must { type string; mandatory true; }
			}
		}
		container knob {
			presence "the knob is on";
			container sub {
				leaf val { type string; mandatory true; }
			}
		}
	}
}`

func newMandatoryValidator(t *testing.T) *Validator {
	t.Helper()
	loader := NewLoader()
	if err := loader.AddModuleFromText("ze-test-mandatory-conf.yang", mandatoryModule); err != nil {
		t.Fatalf("add module: %v", err)
	}
	schema, err := loader.Resolve()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	return NewValidator(schema)
}

func missingPaths(errs []ValidationError) map[string]string {
	paths := make(map[string]string)
	for _, e := range errs {
		if e.Type == ErrTypeMissing {
			paths[e.Path] = e.Message
		}
	}
	return paths
}

// TestRFC7950MandatoryWithNoQualifyingAncestor proves the first arm of
// Section 7.6.5 in both polarities: top and mid are non-presence containers, so
// need has no ancestor that is not one, and it MUST exist even when mid is
// absent from the data.
//
// VALIDATES: a mandatory leaf under absent non-presence containers is required.
// PREVENTS: a config missing such a leaf passing validation.
//
// Method: walk the top section with mid absent, then with mid/need present.
//
// RFC requirement: RFC7950-7.6.5-1 negative -- a mandatory leaf whose ancestors are all non-presence containers absent from the data (top/mid/need) is reported missing.
// RFC requirement: RFC7950-7.6.5-1 positive -- the same leaf, present below its non-presence containers, raises no missing error.
func TestRFC7950MandatoryWithNoQualifyingAncestor(t *testing.T) {
	v := newMandatoryValidator(t)

	absent := missingPaths(v.ValidateTree("top", map[string]any{}))
	msg, ok := absent["top/mid/need"]
	if !ok {
		t.Fatalf("top/mid/need under an absent non-presence container was not required, got %v", absent)
	}
	if want := "add it with `set top mid need <value>`"; !strings.Contains(msg, want) {
		t.Errorf("message %q does not carry the fix %q", msg, want)
	}

	present := missingPaths(v.ValidateTree("top", map[string]any{
		"mid": map[string]any{"need": "x"},
	}))
	if len(present) != 0 {
		t.Errorf("a present mandatory leaf was reported missing: %v", present)
	}
}

// TestRFC7950MandatoryRequiredWhenAncestorExists proves the third arm of
// Section 7.6.5 in both polarities. inner is a non-presence container whose
// closest qualifying ancestor is the item list entry, and sub's is the knob
// presence container: each leaf MUST exist when that ancestor exists, and is
// not required when it does not.
//
// VALIDATES: the ancestor rule for list entries and presence containers.
// PREVENTS: requiring a leaf whose ancestor is absent, or missing one whose ancestor exists.
//
// Method: walk with a list entry and the presence container present but the
// non-presence containers below them absent, then with the ancestors absent,
// then with every leaf supplied.
//
// RFC requirement: RFC7950-7.6.5-3 negative -- a mandatory leaf below an absent non-presence container is reported missing when its closest ancestor that is not a non-presence container (a list entry, a presence container) exists.
// RFC requirement: RFC7950-7.6.5-3 positive -- the same leaf is not required when that ancestor is absent, and raises no error when supplied.
func TestRFC7950MandatoryRequiredWhenAncestorExists(t *testing.T) {
	v := newMandatoryValidator(t)
	need := map[string]any{"need": "x"}

	withAncestors := missingPaths(v.ValidateTree("top", map[string]any{
		"mid":  need,
		"item": map[string]any{"a": map[string]any{"name": "a"}},
		"knob": map[string]any{},
	}))
	for _, path := range []string{"top/item[a]/inner/must", "top/knob/sub/val"} {
		if _, ok := withAncestors[path]; !ok {
			t.Errorf("%s was not required although its ancestor exists, got %v", path, withAncestors)
		}
	}
	if msg := withAncestors["top/item[a]/inner/must"]; !strings.Contains(msg, "`set top item a inner must <value>`") {
		t.Errorf("list entry message %q does not name the set command", msg)
	}

	withoutAncestors := missingPaths(v.ValidateTree("top", map[string]any{"mid": need}))
	if len(withoutAncestors) != 0 {
		t.Errorf("a leaf whose ancestor is absent was required: %v", withoutAncestors)
	}

	supplied := missingPaths(v.ValidateTree("top", map[string]any{
		"mid":  need,
		"item": map[string]any{"a": map[string]any{"name": "a", "inner": map[string]any{"must": "y"}}},
		"knob": map[string]any{"sub": map[string]any{"val": "z"}},
	}))
	if len(supplied) != 0 {
		t.Errorf("supplied mandatory leaves were reported missing: %v", supplied)
	}
}

// TestMandatoryChoiceSatisfiedByACaseNode proves a mandatory choice counts as
// present when a node of one of its cases is in the data. A choice never
// appears in the data under its own name, so a lookup by that name alone
// reported every tunnel with an encapsulation kind as missing one.
//
// VALIDATES: a case node, nested choices included, satisfies a mandatory choice.
// PREVENTS: refusing a valid config now that a missing mandatory node refuses.
//
// Method: walk a container holding a mandatory choice with one case node
// present, with a nested-choice case node present, and with none.
func TestMandatoryChoiceSatisfiedByACaseNode(t *testing.T) {
	loader := NewLoader()
	module := `module ze-test-choice-conf {
	namespace "urn:ze:test:choice";
	prefix tc;
	container wrap {
		choice kind {
			mandatory true;
			case gre { container gre { leaf ttl { type uint8; } } }
			case other {
				choice deep {
					case ipip { leaf ipip { type string; } }
				}
			}
		}
	}
}`
	if err := loader.AddModuleFromText("ze-test-choice-conf.yang", module); err != nil {
		t.Fatalf("add module: %v", err)
	}
	schema, err := loader.Resolve()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	v := NewValidator(schema)

	if got := missingPaths(v.ValidateTree("wrap", map[string]any{"gre": map[string]any{}})); len(got) != 0 {
		t.Errorf("a case node did not satisfy the mandatory choice: %v", got)
	}
	if got := missingPaths(v.ValidateTree("wrap", map[string]any{"ipip": "x"})); len(got) != 0 {
		t.Errorf("a nested case node did not satisfy the mandatory choice: %v", got)
	}
	if _, ok := missingPaths(v.ValidateTree("wrap", map[string]any{}))["wrap/kind"]; !ok {
		t.Error("an empty container did not report its mandatory choice missing")
	}
}

// TestMandatoryChoiceMessageNamesTypeableCases proves the refusal for a missing
// mandatory choice prints commands an operator can type. A choice never
// appears in the data under its own name, so `set <parent> <choice> <value>`
// is refused by the parser it is meant to satisfy.
//
// VALIDATES: the message names every case data node, nested choices included,
// each in a `set` command shaped by its node kind (a non-presence container
// takes more words, an empty leaf and a presence container none, a valued leaf
// a value), and never the choice name
// as a word of a command.
// PREVENTS: telling the operator to type `set wrap kind <value>`.
//
// Method: walk a container whose mandatory choice has a container case, an
// empty-leaf case, a presence-container case and a nested-choice leaf case,
// with none present.
func TestMandatoryChoiceMessageNamesTypeableCases(t *testing.T) {
	loader := NewLoader()
	module := `module ze-test-choice-msg-conf {
	namespace "urn:ze:test:choicemsg";
	prefix tm;
	container wrap {
		choice kind {
			mandatory true;
			case gre { container gre { leaf ttl { type uint8; } } }
			case drop { leaf drop { type empty; } }
			case stop { container stop { presence "stop"; leaf code { type uint8; } } }
			case other {
				choice deep {
					case ipip { leaf ipip { type string; } }
				}
			}
		}
	}
}`
	if err := loader.AddModuleFromText("ze-test-choice-msg-conf.yang", module); err != nil {
		t.Fatalf("add module: %v", err)
	}
	schema, err := loader.Resolve()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	v := NewValidator(schema)

	var message string
	for _, ve := range v.ValidateTree("wrap", map[string]any{}) {
		if ve.Type == ErrTypeMissing && ve.Path == "wrap/kind" {
			message = ve.Message
		}
	}
	if message == "" {
		t.Fatal("an empty container did not report its mandatory choice missing")
	}
	for _, want := range []string{"`set wrap drop`", "`set wrap stop`", "`set wrap gre ...`", "`set wrap ipip <value>`", "Section 7.9.4"} {
		if !strings.Contains(message, want) {
			t.Errorf("message %q does not carry %q", message, want)
		}
	}
	if strings.Contains(message, "set wrap kind") {
		t.Errorf("message %q tells the operator to type the choice name, which no parser accepts", message)
	}
}

// TestSetCommandWordsKeepsSlashInKey proves a list key holding a slash, a
// prefix such as 10.0.0.0/8, survives into the suggested `set` command whole.
//
// VALIDATES: path separators become spaces, a slash inside a [key] does not.
// PREVENTS: suggesting `set static table default route 10.0.0.0 8 ...`, which
// the parser refuses.
//
// Method: convert walk paths with and without a prefix key.
func TestSetCommandWordsKeepsSlashInKey(t *testing.T) {
	cases := map[string]string{
		"static/table[default]/route[10.0.0.0/8]":    "static table default route 10.0.0.0/8",
		"bgp/peer[london]/session":                   "bgp peer london session",
		"static/table[default]/route[2001:db8::/32]": "static table default route 2001:db8::/32",
	}
	for path, want := range cases {
		if got := setCommandWords(path); got != want {
			t.Errorf("setCommandWords(%q) = %q, want %q", path, got, want)
		}
	}
}
