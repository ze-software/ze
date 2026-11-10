package config

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/yang"
	// Register ze-isis-conf, whose system-id pattern carries the message.
	_ "github.com/ze-software/ze/internal/plugins/isis/yang"
)

// isisSystemIDMessage is the error-message of the system-id pattern
// (internal/plugins/isis/yang/ze-isis-conf.yang).
const isisSystemIDMessage = "An IS-IS system-id is 6 octets in hex, written as three groups of 4 hex digits, such as 0000.0000.0001. Write it in that form, or delete system-id so that Ze takes it from net."

// TestISISSystemIDRefusalSpeaksThePatternMessage proves a malformed IS-IS
// system-id is refused by the YANG pattern, the first guard on every config
// path, in the words its error-message gives.
//
// VALIDATES: AC-12 of spec-operator-errors-name-the-fix: on the parse path
// that `ze config validate` and the daemon start share (ValidateLeafValue,
// reached through LoadConfig) and on the tree walk (ValidateTreeAllModules),
// the refusal names the value and the format and how to fix it, with no
// regular expression in it.
// PREVENTS: `value "zzzz.0000.0001" does not match pattern
// "[0-9a-fA-F]{4}\.[0-9a-fA-F]{4}\.[0-9a-fA-F]{4}"`, a refusal the operator
// cannot read, spoken ahead of the isis-system-id validator's better text.
func TestISISSystemIDRefusalSpeaksThePatternMessage(t *testing.T) {
	const want = `invalid value "zzzz.0000.0001": ` + isisSystemIDMessage

	t.Run("parse path", func(t *testing.T) {
		const src = "isis {\n\tnet 49.0001.0000.0000.0001.00\n\tsystem-id zzzz.0000.0001\n}\n"
		_, err := LoadConfig(src, "test.conf", nil)
		if err == nil {
			t.Fatal("LoadConfig accepted a malformed IS-IS system-id")
		}
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not speak the pattern's error-message:\n%s", err.Error())
		}
		if strings.Contains(err.Error(), "{4}") {
			t.Errorf("the refusal still shows the regular expression:\n%s", err.Error())
		}
	})

	t.Run("tree walk", func(t *testing.T) {
		v := yang.NewValidator(newTestLoader(t))
		reg := yang.NewValidatorRegistry()
		RegisterValidators(reg)
		v.SetRegistry(reg)
		errs := systemIDRefusals(v.ValidateTreeAllModules("isis", map[string]any{"system-id": "zzzz.0000.0001"}))
		if len(errs) != 1 {
			t.Fatalf("want exactly one refusal of the system-id, one guard per input, got %d: %+v", len(errs), errs)
		}
		if errs[0].Message != want {
			t.Errorf("the refusal does not speak the pattern's error-message:\n%s", errs[0].Message)
		}
	})
}

// TestISISSystemIDPatternIsTheOneGuard proves the system-id pattern alone
// accepts the dotted-hex form and refuses every malformed one, each with
// exactly one refusal.
//
// VALIDATES: the cases TestISISSystemIDValidator held against the removed
// isis-system-id ValidateFn, now judged on the tree walk where the pattern and
// the ze:validate both ran: a well-formed value draws no refusal, and each
// malformed value draws one, in the pattern's words.
// PREVENTS: a malformed system-id accepted once the ValidateFn is gone, and a
// second refusal of the same value from a validator behind the pattern.
func TestISISSystemIDPatternIsTheOneGuard(t *testing.T) {
	v := yang.NewValidator(newTestLoader(t))
	reg := yang.NewValidatorRegistry()
	RegisterValidators(reg)
	v.SetRegistry(reg)

	for _, good := range []string{"0000.0000.0001", "abcd.ef01.2345", "ABCD.EF01.2345"} {
		t.Run("accepts "+good, func(t *testing.T) {
			errs := systemIDRefusals(v.ValidateTreeAllModules("isis", map[string]any{"system-id": good}))
			if len(errs) != 0 {
				t.Errorf("a well-formed system-id was refused: %+v", errs)
			}
		})
	}

	for name, bad := range map[string]string{
		"too few groups":  "0000.0001",
		"too many groups": "0000.0000.0000.0001",
		"short group":     "000.0000.0001",
		"non-hex":         "zzzz.0000.0001",
		"colons":          "0000:0000:0001",
		"empty":           "",
	} {
		t.Run(name, func(t *testing.T) {
			errs := systemIDRefusals(v.ValidateTreeAllModules("isis", map[string]any{"system-id": bad}))
			if len(errs) != 1 {
				t.Fatalf("want exactly one refusal of %q, got %d: %+v", bad, len(errs), errs)
			}
			if !strings.HasSuffix(errs[0].Message, isisSystemIDMessage) {
				t.Errorf("the refusal of %q does not speak the pattern's error-message:\n%s", bad, errs[0].Message)
			}
		})
	}
}

// systemIDRefusals keeps the refusals of the system-id leaf, so a refusal of
// another isis leaf (the absent net) does not count against one guard per input.
func systemIDRefusals(errs []yang.ValidationError) []yang.ValidationError {
	var kept []yang.ValidationError
	for i := range errs {
		if strings.HasSuffix(errs[i].Path, "system-id") {
			kept = append(kept, errs[i])
		}
	}
	return kept
}
