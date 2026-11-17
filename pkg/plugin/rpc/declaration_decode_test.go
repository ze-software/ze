package rpc

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
)

// declarationRefusal decodes params for the plugin "acme" and answers the
// operator error the decoder refused it with, failing the test when it
// accepted it or refused it with another kind of error.
func declarationRefusal(t *testing.T, params string) *opererr.Error {
	t.Helper()
	_, err := DecodeDeclaration("acme", []byte(params))
	if err == nil {
		t.Fatalf("the declaration was accepted: %s", params)
	}
	refusal, operator := errors.AsType[*opererr.Error](err)
	if !operator {
		t.Fatalf("the refusal is no operator error: %v", err)
	}
	return refusal
}

// TestDecodeDeclarationRefusesTheRetiredSchemaKey: a declaration that still
// carries its module under "schema" is refused, with the fix that names
// "schemas". Method: decode a versioned declaration in the old shape.
//
// VALIDATES: the refusal names the plugin and the key, says the modules are a
// list under "schemas" now, and names the page and section of the keys.
// PREVENTS: a plugin built on the old shape starting silently with no module
// and no commands.
func TestDecodeDeclarationRefusesTheRetiredSchemaKey(t *testing.T) {
	refusal := declarationRefusal(t,
		`{"protocol-version":1,"schema":{"module":"acme-conf","yang-text":"module acme-conf {}"}}`)

	if want := `plugin "acme" sent its YANG module under "schema", a key ze no longer reads`; refusal.Problem() != want {
		t.Fatalf("problem = %q, want %q", refusal.Problem(), want)
	}
	if !strings.Contains(refusal.Why(), `as a list under "schemas"`) {
		t.Fatalf("why does not say the modules are a list under schemas: %q", refusal.Why())
	}
	if !strings.Contains(refusal.Fix(), `"schemas":[{`) || !strings.Contains(refusal.Fix(), protocolPage+`, "`+DeclarationSection+`"`) {
		t.Fatalf("fix does not show the schemas list and the page: %q", refusal.Fix())
	}
}

// TestDecodeDeclarationRefusesAnUnknownKey: a key ze does not know, at the top
// of the declaration or inside an object it defines, is refused and named.
// Method: decode a versioned declaration with an unknown key at each depth.
//
// VALIDATES: the refusal names the plugin, carries the key the plugin sent as
// its evidence, and names where the keys are documented.
// PREVENTS: a misspelled key, or one this ze does not have, dropped while the
// plugin starts without what it declared.
func TestDecodeDeclarationRefusesAnUnknownKey(t *testing.T) {
	cases := []struct {
		name   string
		params string
		key    string
	}{
		{"top level", `{"protocol-version":1,"shade":"blue"}`, `"shade"`},
		{"inside a module entry", `{"protocol-version":1,"schemas":[{"module":"acme-conf","yang":"module acme-conf {}"}]}`, `"yang"`},
		{"inside a command", `{"protocol-version":1,"commands":[{"name":"show acme","shade":"blue"}]}`, `"shade"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			refusal := declarationRefusal(t, tc.params)
			if want := `plugin "acme" sent a Stage 1 declaration that carries a key ze does not know`; refusal.Problem() != want {
				t.Fatalf("problem = %q, want %q", refusal.Problem(), want)
			}
			if !strings.Contains(refusal.Error(), "unknown field "+tc.key) {
				t.Fatalf("the refusal does not name the key %s:\n%s", tc.key, refusal.Error())
			}
			if !strings.Contains(refusal.Fix(), protocolPage) || !strings.Contains(refusal.Fix(), DeclarationSection) {
				t.Fatalf("fix does not name the page of the keys: %q", refusal.Fix())
			}
		})
	}
}

// TestDecodeDeclarationKeepsTheRetiredHelpKey: a command entry's `help`, the
// summary's spelling before `short-help`, is a key CommandDecl knows
// (RetiredHelp), so the decoder reads it and keeps its bytes. Its refusal is
// later and has its own text naming the replacement: validateHelpDecls, tested
// by TestValidateHelpDeclsRefusesTheRetiredKey (plugin/server startup_test.go).
// Method: decode a declaration whose command carries the key, empty and not.
//
// VALIDATES: the strict decoder does not refuse `help` with the generic
// unknown-key text, which would hide the replacement the later check names.
func TestDecodeDeclarationKeepsTheRetiredHelpKey(t *testing.T) {
	for _, help := range []string{`"Show acme."`, `""`} {
		params := `{"protocol-version":1,"commands":[{"name":"show acme","help":` + help + `}]}`
		declaration, err := DecodeDeclaration("acme", []byte(params))
		if err != nil {
			t.Fatalf("the declaration was refused: %v", err)
		}
		if len(declaration.Commands) != 1 {
			t.Fatalf("commands = %d, want 1", len(declaration.Commands))
		}
		if got := string(declaration.Commands[0].RetiredHelp); got != help {
			t.Fatalf("RetiredHelp = %q, want %q: the later check reads it", got, help)
		}
	}
}

// TestDecodeDeclarationReadsWhatItKnows: the decoder accepts a declaration of
// known keys, places a decode error for its caller, and leaves a declaration
// with no protocol version to the version refusal. Method: decode one of each.
//
// VALIDATES: the permitted survivors of the strict read: every known key at
// every depth, and the text-era declaration whose one refusal is the version
// (D-2). JSON that is no declaration is a decode error, not an operator error.
// PREVENTS: the strict read refusing what the SDK writes, and a text-era
// plugin getting a key refusal in place of the version refusal.
func TestDecodeDeclarationReadsWhatItKnows(t *testing.T) {
	declaration, err := DecodeDeclaration("acme",
		[]byte(`{"protocol-version":1,"schemas":[{"module":"acme-conf","namespace":"urn:acme","yang-text":"module acme-conf {}"}],`+
			`"commands":[{"name":"show acme","short-help":"Show acme."}]}`))
	if err != nil {
		t.Fatalf("a declaration of known keys was refused: %v", err)
	}
	if len(declaration.Schemas) != 1 || declaration.Schemas[0].Module != "acme-conf" || len(declaration.Commands) != 1 {
		t.Fatalf("decoded %+v", declaration)
	}

	textEra, err := DecodeDeclaration("acme", []byte(`{"filters":[{"name":"legacy","attributes":["as-path"]}]}`))
	if err != nil {
		t.Fatalf("a declaration with no protocol version was refused before the version check: %v", err)
	}
	if textEra.ProtocolVersion != 0 || len(textEra.Filters) != 1 {
		t.Fatalf("decoded %+v", textEra)
	}
	if _, versionErr := NegotiateProtocolVersion("acme", textEra.ProtocolVersion); versionErr == nil {
		t.Fatal("the version check accepted the declaration the decoder left to it")
	}

	for _, broken := range []string{`{"protocol-version":`, `{"protocol-version":1} {}`, `{"protocol-version":"one"}`} {
		_, err := DecodeDeclaration("acme", []byte(broken))
		if err == nil {
			t.Fatalf("JSON that is no declaration was accepted: %s", broken)
		}
		if _, operator := errors.AsType[*opererr.Error](err); operator {
			t.Fatalf("a decode error came back as an operator error for %s: %v", broken, err)
		}
	}
}
