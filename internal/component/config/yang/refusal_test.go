package yang

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
)

// refusalProbeModule declares one leaf of each refusal kind: an enumeration
// whose declared order is not the sorted order, a secret enumeration, ranges
// with and without an error-message, lengths on a plain and a secret leaf,
// and patterns whose format comes from each source LeafValueFormat reads.
const refusalProbeModule = `module ze-test-refusal-conf {
    namespace "urn:ze:test-refusal:conf";
    prefix trf;

    import ze-extensions { prefix ze; }

    typedef rate {
        type string { pattern '[0-9]+(kbit|mbit)'; }
        ze:help "Traffic rate (for example 10mbit)";
    }
    typedef described {
        type string { pattern '[a-z]+'; }
        description "Lower-case letters only.  Digits are refused.";
    }
    typedef bare {
        type string { pattern '[A-Z]+'; }
    }

    container probe {
        leaf mode {
            type enumeration {
                enum enable { ze:help "Advertise it"; }
                enum disable { ze:help "Do not advertise it"; }
                enum require { ze:help "Refuse a peer without it"; }
                enum refuse { ze:help "Refuse a peer with it"; }
            }
        }
        leaf partial {
            type enumeration {
                enum zeta;
                enum alpha { ze:help "First letter"; }
            }
        }
        leaf secret-mode {
            type enumeration { enum alpha; enum beta; }
            ze:sensitive;
        }
        leaf cost { type uint16 { range "1..65535"; } }
        leaf vlan {
            type uint16 { range "1..4094" { error-message "A VLAN id is 1 to 4094."; } }
        }
        leaf run { type string { length "1..4096"; } }
        leaf secret-key {
            type string { length "8..64"; }
            ze:sensitive;
        }
        leaf rate { type rate; }
        leaf described { type described; }
        leaf bare-help { type bare; ze:help "An upper-case word"; }
        leaf bare { type bare; }
        leaf inline-help { type string { pattern '[0-9]+'; } ze:help "Digits only"; }
        leaf inline { type string { pattern '[0-9]+'; } }
    }
}`

// modeValueLines is the why of every mode refusal: the values in declared
// order, each help two columns after the longest name.
const modeValueLines = "mode accepts only these values:\n" +
	"  enable   Advertise it\n" +
	"  disable  Do not advertise it\n" +
	"  require  Refuse a peer without it\n" +
	"  refuse   Refuse a peer with it"

// refusalProbe answers a validator over the embedded modules and the probe.
func refusalProbe(t *testing.T) *Validator {
	t.Helper()
	loader := NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		t.Fatalf("load embedded modules: %v", err)
	}
	if err := loader.AddModuleFromText("ze-test-refusal-conf", refusalProbeModule); err != nil {
		t.Fatalf("add the probe module: %v", err)
	}
	schema, err := loader.Resolve()
	if err != nil {
		t.Fatalf("resolve the model: %v", err)
	}
	return NewValidator(schema)
}

// walkRefusal validates value at the probe leaf named leaf and answers the
// ValidationError the walk refused it with.
func walkRefusal(t *testing.T, v *Validator, leaf, value string) *ValidationError {
	t.Helper()
	err := v.Validate("probe/"+leaf, value)
	if err == nil {
		t.Fatalf("probe %s %q accepted, want a refusal", leaf, value)
	}
	refusal, found := errors.AsType[*ValidationError](err)
	if !found {
		t.Fatalf("probe %s %q refused with %T, want a *ValidationError: %v", leaf, value, err, err)
	}
	return refusal
}

// TestEnumRefusalSuggestion validates A-1: a value refusal suggests through
// suggest.Correction, whose bound is tighter than suggest.Command's. Method: a
// one-letter typo gets the hint and the corrected fix; a word far from every
// value, or only within Command's bound, gets neither, and its fix is the
// alternation.
func TestEnumRefusalSuggestion(t *testing.T) {
	v := refusalProbe(t)

	near := walkRefusal(t, v, "mode", "requre")
	if got := near.Correction(); got != "require" {
		t.Errorf("requre: correction %q, want require", got)
	}
	if !strings.HasSuffix(strings.SplitN(near.Error(), "\n", 2)[0], " (did you mean require?)") {
		t.Errorf("requre: the problem line carries no hint:\n%s", near.Error())
	}

	far := walkRefusal(t, v, "mode", "banana")
	if strings.Contains(far.Error(), "did you mean") {
		t.Errorf("banana: a hint was offered:\n%s", far.Error())
	}
	if got := far.Correction(); got != "<enable|disable|require|refuse>" {
		t.Errorf("banana: correction %q, want the alternation in declared order", got)
	}

	// "rqure" is five characters and two edits from "require": within
	// Command's half-length bound, past the third of its length a value may
	// be corrected by (suggest.Correction), so no hint and the alternation,
	// never a guessed value in the fix line.
	guess := walkRefusal(t, v, "mode", "rqure")
	if strings.Contains(guess.Error(), "did you mean") {
		t.Errorf("rqure: a hint was offered:\n%s", guess.Error())
	}
	if got := guess.Correction(); got != "<enable|disable|require|refuse>" {
		t.Errorf("rqure: correction %q, want the alternation in declared order", got)
	}
}

// TestValidationErrorOperatorError pins the operator error of each kind the
// walk refuses with (AC-16, AC-17, AC-35 to AC-37, AC-40). Method: compare
// the whole rendered refusal, so a word, a line or the fix out of place is
// red; then check the secret leaves never echo the value and the kinds that
// hold no operator error keep their text.
func TestValidationErrorOperatorError(t *testing.T) {
	v := refusalProbe(t)

	cases := []struct {
		name, leaf, value, want string
	}{
		{"enum with a hint", "mode", "requre",
			"probe mode: \"requre\" is not an accepted value (did you mean require?)\n" +
				modeValueLines + "\nfix: set probe mode require"},
		{"enum value with no help", "partial", "omega",
			"probe partial: \"omega\" is not an accepted value\n" +
				"partial accepts only these values:\n  zeta\n  alpha  First letter\n" +
				"fix: set probe partial <zeta|alpha>"},
		{"range", "cost", "0",
			"probe cost: \"0\" is outside the accepted range\ncost accepts a value in 1..65535\n" +
				"fix: set probe cost <1..65535>"},
		{"range with an error-message", "vlan", "0",
			"probe vlan: \"0\" is outside the accepted range\nA VLAN id is 1 to 4094.\n" +
				"fix: set probe vlan <1..4094>"},
		{"length", "run", "",
			"probe run: \"\" is outside the accepted length (0 characters)\nrun accepts 1..4096 characters\n" +
				"fix: set probe run <1..4096 characters>"},
		{"pattern, typedef help", "rate", "fast",
			"probe rate: \"fast\" is not in the accepted format\nrate accepts Traffic rate (for example 10mbit)\n" +
				"fix: set probe rate <rate>"},
		{"pattern, typedef description", "described", "Abc",
			"probe described: \"Abc\" is not in the accepted format\ndescribed accepts Lower-case letters only\n" +
				"fix: set probe described <described>"},
		{"pattern, leaf help over a bare typedef", "bare-help", "abc",
			"probe bare-help: \"abc\" is not in the accepted format\nbare-help accepts An upper-case word\n" +
				"fix: set probe bare-help <bare>"},
		{"pattern, no words", "bare", "abc",
			"probe bare: \"abc\" is not in the accepted format\nbare accepts a value of type bare\n" +
				"fix: set probe bare <bare>"},
		{"inline pattern, leaf help", "inline-help", "x",
			"probe inline-help: \"x\" is not in the accepted format\ninline-help accepts Digits only\n" +
				"fix: set probe inline-help <value>"},
		{"inline pattern, no words", "inline", "x",
			"probe inline: \"x\" is not in the accepted format\ninline does not accept this value\n" +
				"fix: set probe inline <value>"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			refusal := walkRefusal(t, v, c.leaf, c.value)
			if got := refusal.Error(); got != c.want {
				t.Fatalf("got:\n%s\nwant:\n%s", got, c.want)
			}
			operr, found := errors.AsType[*opererr.Error](refusal)
			if !found {
				t.Fatal("errors.As found no *opererr.Error in the refusal")
			}
			if operr.Error() != c.want {
				t.Errorf("the held operator error renders:\n%s", operr.Error())
			}
			for _, expression := range []string{"[0-9]", "[a-z]", "[A-Z]", "kbit|mbit"} {
				if strings.Contains(refusal.Error(), expression) {
					t.Errorf("the refusal shows the regular expression %q", expression)
				}
			}
		})
	}

	t.Run("secret enum", func(t *testing.T) {
		refusal := walkRefusal(t, v, "secret-mode", "alpah")
		const want = "probe secret-mode: /* SECRET-DATA */ is not an accepted value\n" +
			"secret-mode accepts only these values:\n  alpha\n  beta\n" +
			"fix: set probe secret-mode <alpha|beta>"
		if got := refusal.Error(); got != want {
			t.Fatalf("got:\n%s\nwant:\n%s", got, want)
		}
		if !refusal.Secret() {
			t.Error("the refusal does not report the secret")
		}
	})

	t.Run("secret length", func(t *testing.T) {
		refusal := walkRefusal(t, v, "secret-key", "hunter2")
		const want = "probe secret-key: /* SECRET-DATA */ is outside the accepted length\n" +
			"secret-key accepts 8..64 characters\nfix: set probe secret-key <8..64 characters>"
		if got := refusal.Error(); got != want {
			t.Fatalf("got:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("typed facts kept", func(t *testing.T) {
		refusal := walkRefusal(t, v, "mode", "requre")
		if refusal.Type != ErrTypeEnum || refusal.Got != "requre" {
			t.Errorf("Type %v Got %q", refusal.Type, refusal.Got)
		}
		if refusal.Expected != "enable, disable, require, refuse" {
			t.Errorf("Expected %q, want the declared order", refusal.Expected)
		}
		length := walkRefusal(t, v, "run", "")
		if length.Got != "0" || length.Expected != "1..4096" {
			t.Errorf("length Got %q Expected %q", length.Got, length.Expected)
		}
	})

	t.Run("other kinds keep their text", func(t *testing.T) {
		missing := &ValidationError{Path: "probe/x", Type: ErrTypeMissing, Message: "missing required field"}
		if got := missing.Error(); got != "missing error at probe/x: missing required field" {
			t.Errorf("missing: %q", got)
		}
		typed := walkRefusal(t, v, "cost", "abc")
		if typed.HoldsOperatorError() {
			t.Error("a type refusal holds an operator error")
		}
		if !strings.HasPrefix(typed.Error(), "type error at probe/cost: ") {
			t.Errorf("type: %q", typed.Error())
		}
		if _, found := errors.AsType[*opererr.Error](typed); found {
			t.Error("errors.As found an operator error in a type refusal")
		}
	})
}

// TestFrameKeepsProblemAndWhy proves a frame changes only what it holds (R-13,
// AC-21). Method: WithFix and WithPath copies are compared part by part with
// the original, which stays as built; errors.As must reach the
// ValidationError and the *opererr.Error through a wrapping layer and through
// each copy; the text prints its problem and its value list once.
func TestFrameKeepsProblemAndWhy(t *testing.T) {
	v := refusalProbe(t)
	original := walkRefusal(t, v, "mode", "requre")
	before := original.Error()

	framed := original.WithFix("mode require;")
	const problem = "probe mode: \"requre\" is not an accepted value (did you mean require?)"
	if got := framed.Error(); got != problem+"\n"+modeValueLines+"\nfix: mode require;" {
		t.Errorf("WithFix:\n%s", got)
	}
	if original.Error() != before {
		t.Error("WithFix changed the original")
	}

	path := []string{"bgp", "peer", "peer1", "session", "capability", "mode"}
	moved := original.WithPath(path)
	const movedProblem = "bgp peer peer1 session capability mode: \"requre\" is not an accepted value (did you mean require?)"
	if got := moved.Error(); got != movedProblem+"\n"+modeValueLines+"\nfix: set bgp peer peer1 session capability mode require" {
		t.Errorf("WithPath:\n%s", got)
	}
	path[0] = "changed"
	if moved.Segments()[0] != "bgp" {
		t.Error("WithPath kept the caller's slice")
	}
	if got := framed.WithPath([]string{"a", "mode"}).Error(); !strings.HasSuffix(got, "\nfix: mode require;") {
		t.Errorf("WithPath dropped the framed fix:\n%s", got)
	}

	lined := *original
	lined.LineNumber = 12
	if got := lined.Error(); got != "line 12: "+before {
		t.Errorf("a refusal with a line: %q", got)
	}

	for name, refusal := range map[string]*ValidationError{"original": original, "WithFix": framed, "WithPath": moved} {
		t.Run(name, func(t *testing.T) {
			wrapped := fmt.Errorf("parse config: %w", refusal)
			typed, found := errors.AsType[*ValidationError](wrapped)
			if !found {
				t.Fatal("errors.As found no *ValidationError")
			}
			if typed.Type != original.Type || typed.Got != original.Got || typed.Expected != original.Expected {
				t.Errorf("typed facts moved: %v %q %q", typed.Type, typed.Got, typed.Expected)
			}
			if typed.Leaf() != "mode" || typed.Correction() != "require" || typed.Secret() {
				t.Errorf("facts moved: leaf %q correction %q", typed.Leaf(), typed.Correction())
			}
			if len(typed.Values()) != 4 || typed.Values()[0].Name != "enable" {
				t.Errorf("values moved: %v", typed.Values())
			}
			operr, found := errors.AsType[*opererr.Error](wrapped)
			if !found {
				t.Fatal("errors.As found no *opererr.Error")
			}
			if operr.Why() != modeValueLines {
				t.Errorf("why moved:\n%s", operr.Why())
			}
			text := refusal.Error()
			if strings.Count(text, "is not an accepted value") != 1 || strings.Count(text, "accepts only these values") != 1 {
				t.Errorf("the refusal prints a part twice:\n%s", text)
			}
		})
	}
}
