// The test package is external so it can register a real config module: an
// in-package test cannot import one, because every module package imports this
// one to register itself.
package yang_test

import (
	"slices"
	"strings"
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"

	_ "github.com/ze-software/ze/internal/component/config/system/yang" // registers ze-system-conf.yang, whose conntrack leaf-list is an enumeration
)

// TestEnumValuesAnswersTheModel proves the reader every derived Go set depends
// on answers the values the model declares, sorted.
//
// The assertions name one value and the shape of the answer rather than the
// whole list: a test holding the twelve helper names would be the third copy of
// the set this reader exists to remove.
//
// VALIDATES: EnumValues resolves a config path to the enumeration at that leaf.
// PREVENTS: a guard reading its allowed set from the model and getting an empty
// one, which reads as "nothing is allowed" with no line saying why.
func TestEnumValuesAnswersTheModel(t *testing.T) {
	values, err := configyang.EnumValues("system/conntrack/module")
	if err != nil {
		t.Fatalf("EnumValues on the conntrack module leaf-list: %v", err)
	}
	if len(values) == 0 {
		t.Fatal("EnumValues answered no value for an enumeration the model declares")
	}
	if !slices.Contains(values, "ftp") {
		t.Errorf("EnumValues answered %v, which does not carry the ftp helper the model declares", values)
	}
	if !slices.IsSorted(values) {
		t.Errorf("EnumValues answered %v, which is not sorted", values)
	}
}

// TestEnumValuesRefusesWhatIsNotAnEnumeration proves the ways a path can fail
// are errors rather than an empty set a caller reads as "nothing allowed".
//
// VALIDATES: a non-enumeration leaf and an unknown path each produce an error
// and a nil set.
// PREVENTS: a typo in a path turning a guard into one that refuses every value
// an operator writes, with no line saying why.
func TestEnumValuesRefusesWhatIsNotAnEnumeration(t *testing.T) {
	cases := []struct {
		name string
		path string
		want string
	}{
		{"a number leaf", "system/conntrack/table-size", "is not an enumeration"},
		{"an unknown leaf", "system/conntrack/absent-leaf", "resolve"},
		{"an unknown section", "absent-section/mode", "resolve"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			values, err := configyang.EnumValues(c.path)
			if err == nil {
				t.Fatalf("EnumValues(%q) answered %v, want an error", c.path, values)
			}
			if values != nil {
				t.Errorf("EnumValues(%q) answered %v beside its error, want nil", c.path, values)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("EnumValues(%q) said %q, want it to name %q", c.path, err, c.want)
			}
		})
	}
}

// typedefEnumTypes is the imported module of TestEnumValueSummariesFollowATypedef:
// its typedef is reached through the tt prefix.
const typedefEnumTypes = `
module ze-typedef-enum-types {
    namespace "urn:test:typedef-enum-types";
    prefix tt;

    import ze-extensions { prefix ze; }

    typedef origin {
        type enumeration {
            enum igp { ze:help "Learned from an interior protocol"; }
            enum egp { ze:help "Learned from an exterior protocol"; }
        }
    }
}
`

// typedefEnumConf is the module of TestEnumValueSummariesFollowATypedef. Every
// enumeration its leaves carry arrives through a typedef: declared at module
// level, in an imported module, in a grouping, through a second typedef, or as
// the members of a union.
const typedefEnumConf = `
module ze-typedef-enum-conf {
    namespace "urn:test:typedef-enum-conf";
    prefix te;

    import ze-extensions { prefix ze; }
    import ze-typedef-enum-types { prefix tt; }

    typedef mode {
        type enumeration {
            enum active { ze:help "Open the session"; }
            enum passive { ze:help "Wait for the peer"; }
            enum silent;
        }
    }

    typedef mode-alias {
        type mode;
    }

    grouping scoped {
        typedef local-kind {
            type enumeration {
                enum fast { ze:help "Answer within a second"; }
                enum slow { ze:help "Answer within a minute"; }
            }
        }
        leaf kind {
            type local-kind;
            ze:help "How fast to answer";
        }
    }

    container settings {
        ze:help "Session settings";
        leaf mode {
            type mode;
            ze:help "How the session opens";
        }
        leaf origin {
            type tt:origin;
            ze:help "Where the route was learned";
        }
        leaf alias {
            type mode-alias;
            ze:help "How the session opens, through a second typedef";
        }
        leaf either {
            type union {
                type mode;
                type tt:origin;
            }
            ze:help "Either a mode or an origin";
        }
        uses scoped;
    }
}
`

// TestEnumValueSummariesFollowATypedef proves the reader of an enum value's
// ze:help follows a typedef reference to the enum statements the typedef
// declares, through every YANG scope a typedef can sit in, and that a value
// the typedef declares without a ze:help stays absent.
//
// VALIDATES: EnumValueSummaries answers the summary for a leaf whose
// enumeration arrives through a module-level typedef, an imported-prefix
// typedef, a grouping-scoped typedef, a typedef of a typedef, and a union of
// typedefs.
// PREVENTS: a typedef-borne enumeration reaching the completion row, the
// analysis tree and the site reference with no summary while the model
// declares one.
func TestEnumValueSummariesFollowATypedef(t *testing.T) {
	loader := configyang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}
	if err := loader.AddModuleFromText("ze-typedef-enum-types.yang", typedefEnumTypes); err != nil {
		t.Fatalf("AddModuleFromText types: %v", err)
	}
	if err := loader.AddModuleFromText("ze-typedef-enum-conf.yang", typedefEnumConf); err != nil {
		t.Fatalf("AddModuleFromText conf: %v", err)
	}
	if err := loader.Resolve(); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	entry := loader.GetEntry("ze-typedef-enum-conf")
	if entry == nil || entry.Dir["settings"] == nil {
		t.Fatal("the fixture module has no settings container")
	}
	settings := entry.Dir["settings"]

	cases := []struct {
		name string
		leaf string
		want map[string]string
	}{
		{"a module-level typedef", "mode", map[string]string{
			"active": "Open the session", "passive": "Wait for the peer",
		}},
		{"an imported-prefix typedef", "origin", map[string]string{
			"igp": "Learned from an interior protocol", "egp": "Learned from an exterior protocol",
		}},
		{"a grouping-scoped typedef", "kind", map[string]string{
			"fast": "Answer within a second", "slow": "Answer within a minute",
		}},
		{"a typedef of a typedef", "alias", map[string]string{
			"active": "Open the session", "passive": "Wait for the peer",
		}},
		{"a union of typedefs", "either", map[string]string{
			"active": "Open the session", "passive": "Wait for the peer",
			"igp": "Learned from an interior protocol", "egp": "Learned from an exterior protocol",
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			leaf := settings.Dir[c.leaf]
			if leaf == nil {
				t.Fatalf("the fixture declares no settings/%s leaf", c.leaf)
			}
			got := configyang.EnumValueSummaries(leaf)
			if len(got) != len(c.want) {
				t.Fatalf("EnumValueSummaries(settings/%s) = %v, want %v", c.leaf, got, c.want)
			}
			for name, summary := range c.want {
				if got[name] != summary {
					t.Errorf("EnumValueSummaries(settings/%s)[%s] = %q, want %q", c.leaf, name, got[name], summary)
				}
			}
			if _, held := got["silent"]; held {
				t.Errorf("EnumValueSummaries(settings/%s) carries silent, a value declared with no ze:help", c.leaf)
			}
		})
	}
}
