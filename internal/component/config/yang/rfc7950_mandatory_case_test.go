package yang

import (
	"strings"
	"testing"
)

// caseModule is a schema shaped to separate the case arm of RFC 7950 Section
// 7.6.5 from the other two: a mandatory leaf whose closest ancestor that is
// not a non-presence container is a case node. alpha holds it directly, beta
// holds it below two non-presence containers (the GRE tunnel's remote/ip
// shape), and gamma holds it inside a nested choice's case. The choice itself
// is not mandatory, so no case is owed until one of its nodes exists.
const caseModule = `module ze-test-mandatory-case-conf {
	namespace "urn:ze:test:mandatorycase";
	prefix tmc;
	container wrap {
		choice kind {
			case alpha {
				leaf a-flag { type string; }
				leaf a-need { type string; mandatory true; }
			}
			case beta {
				container beta {
					leaf ttl { type uint8; }
					container remote {
						leaf ip { type string; mandatory true; }
					}
				}
			}
			case gamma {
				choice inner {
					case g1 {
						leaf g-one { type string; }
						leaf g-need { type uint8; mandatory true; }
					}
				}
			}
		}
	}
}`

func newCaseValidator(t *testing.T) *Validator {
	t.Helper()
	loader := NewLoader()
	if err := loader.AddModuleFromText("ze-test-mandatory-case-conf.yang", caseModule); err != nil {
		t.Fatalf("add module: %v", err)
	}
	schema, err := loader.Resolve()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	return NewValidator(schema)
}

// TestRFC7950MandatoryRequiredWhenCaseExists proves the case arm of Section
// 7.6.5 refuses: once any node of a case exists, every mandatory leaf of that
// case is owed, whether it sits directly in the case, below non-presence
// containers the case holds, or in a nested choice's case.
//
// VALIDATES: a mandatory leaf of a present case is reported missing, with the
// `set` command that supplies it.
// PREVENTS: a GRE tunnel with no remote ip, or any case missing its mandatory
// leaf, passing validation.
//
// Method: walk wrap with one node of each case present and its mandatory leaf
// absent, and read the missing paths.
//
// RFC requirement: RFC7950-7.6.5-2 negative -- with one node of a case present (a-flag, beta, g-one), the case's mandatory leaf (wrap/a-need, wrap/beta/remote/ip, wrap/g-need) is reported missing with its set command.
func TestRFC7950MandatoryRequiredWhenCaseExists(t *testing.T) {
	v := newCaseValidator(t)

	cases := []struct {
		name string
		data map[string]any
		path string
		fix  string
	}{
		{"leaf in case", map[string]any{"a-flag": "x"}, "wrap/a-need", "add it with `set wrap a-need <value>`"},
		{"leaf below containers in case", map[string]any{"beta": map[string]any{"ttl": "8"}}, "wrap/beta/remote/ip", "add it with `set wrap beta remote ip <value>`"},
		{"empty container in case", map[string]any{"beta": map[string]any{}}, "wrap/beta/remote/ip", "add it with `set wrap beta remote ip <value>`"},
		{"leaf in nested case", map[string]any{"g-one": "x"}, "wrap/g-need", "add it with `set wrap g-need <value>`"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			missing := missingPaths(v.ValidateTree("wrap", tc.data))
			msg, ok := missing[tc.path]
			if !ok {
				t.Fatalf("%s of a present case was not required, got %v", tc.path, missing)
			}
			if !strings.Contains(msg, tc.fix) {
				t.Errorf("message %q does not carry the fix %q", msg, tc.fix)
			}
			if len(missing) != 1 {
				t.Errorf("want only %s missing, got %v", tc.path, missing)
			}
		})
	}
}

// TestRFC7950MandatoryNotRequiredForAbsentCase proves the case arm does not
// over-reach: a case none of whose nodes exists owes nothing, and a present
// case whose mandatory leaf is supplied raises no error.
//
// VALIDATES: only the case that has data is checked, and a complete case passes.
// PREVENTS: an over-eager walk refusing every case's mandatory leaves, which
// would refuse every valid tunnel of any kind.
//
// Method: walk wrap empty, then with each case complete.
//
// RFC requirement: RFC7950-7.6.5-2 positive -- a case with no node present owes no mandatory leaf, and a present case with its mandatory leaf supplied raises no missing error.
func TestRFC7950MandatoryNotRequiredForAbsentCase(t *testing.T) {
	v := newCaseValidator(t)

	cases := []struct {
		name string
		data map[string]any
	}{
		{"no case present", map[string]any{}},
		{"alpha complete", map[string]any{"a-flag": "x", "a-need": "y"}},
		{"beta complete", map[string]any{"beta": map[string]any{"remote": map[string]any{"ip": "192.0.2.1"}}}},
		{"gamma complete", map[string]any{"g-need": "7"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if errs := v.ValidateTree("wrap", tc.data); len(errs) != 0 {
				t.Errorf("want no error, got %v", errs)
			}
		})
	}
}

// TestWalkTreeValidatesCaseData proves walkTree reaches the data a case holds:
// a value under a case is checked against its leaf's type, as any other leaf.
//
// VALIDATES: a case leaf with a value out of its type is refused.
// PREVENTS: case data being skipped as unknown, which let any value through.
//
// Method: walk wrap with g-need, a uint8 inside a nested case, set to 300, and
// with beta/ttl, a uint8 inside a case's container, set to 300.
func TestWalkTreeValidatesCaseData(t *testing.T) {
	v := newCaseValidator(t)

	cases := map[string]map[string]any{
		"wrap/g-need":   {"g-need": "300"},
		"wrap/beta/ttl": {"beta": map[string]any{"ttl": "300", "remote": map[string]any{"ip": "x"}}},
	}
	for path, data := range cases {
		found := false
		for _, e := range v.ValidateTree("wrap", data) {
			if e.Path == path && e.Type != ErrTypeMissing {
				found = true
			}
		}
		if !found {
			t.Errorf("%s set to 300 was not refused as out of its uint8 type", path)
		}
	}
}
