package rpc

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestDeclarationCarriesAListOfModules: the Stage 1 declaration carries every
// module a plugin brings under `schemas`, in order, and the retired `schema`
// object is refused. Method: marshal a declaration of two modules, then decode
// a line in the old shape.
//
// VALIDATES: the wire of the command step of spec-plugin-external-yang-at-startup:
// one entry for each module, with its module, namespace and yang-text keys.
// PREVENTS: a second module dropped on the wire, a fallback that would still
// read the one-module object (no compatibility before the first release), and
// the old object dropped in silence so the plugin starts with no module.
func TestDeclarationCarriesAListOfModules(t *testing.T) {
	input := DeclareRegistrationInput{Schemas: []SchemaDecl{
		{Module: "acme-monitor-conf", Namespace: "urn:acme:monitor:conf", YANGText: "module acme-monitor-conf {}"},
		{Module: "acme-monitor-cmd", Namespace: "urn:acme:monitor:cmd", YANGText: "module acme-monitor-cmd {}"},
	}}
	line, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	text := string(line)
	want := `"schemas":[{"module":"acme-monitor-conf","namespace":"urn:acme:monitor:conf","yang-text":"module acme-monitor-conf {}"},` +
		`{"module":"acme-monitor-cmd","namespace":"urn:acme:monitor:cmd","yang-text":"module acme-monitor-cmd {}"}]`
	if !strings.Contains(text, want) {
		t.Fatalf("the declaration does not carry both modules in order:\n%s", text)
	}
	if strings.Contains(text, `"schema":`) {
		t.Fatalf("the declaration still carries a schema object:\n%s", text)
	}

	var decoded DeclareRegistrationInput
	if err := json.Unmarshal(line, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(decoded.Schemas) != 2 || decoded.Schemas[1].Module != "acme-monitor-cmd" {
		t.Fatalf("the decoded declaration holds %+v, want both modules in order", decoded.Schemas)
	}

	// The old shape is refused by the decoder every reader uses: it is never
	// read as a module, and never dropped in silence either.
	old, err := DecodeDeclaration("acme-monitor",
		[]byte(`{"protocol-version":1,"schema":{"module":"acme-monitor-conf","yang-text":"module acme-monitor-conf {}"}}`))
	if err == nil {
		t.Fatalf("the old schema object was accepted as %+v", old)
	}
	if len(old.Schemas) != 0 {
		t.Fatalf("the old schema object was read as %+v; the protocol keeps no fallback", old.Schemas)
	}
	if !strings.Contains(err.Error(), `under "schema"`) || !strings.Contains(err.Error(), `"schemas"`) {
		t.Fatalf("the refusal of the old shape does not name schema and schemas:\n%v", err)
	}
}
