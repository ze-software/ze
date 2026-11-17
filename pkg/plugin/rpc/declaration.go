// Design: docs/architecture/api/process-protocol.md — Stage 1 declare-registration
// Related: types.go — MethodDeclareRegistration, DeclareRegistrationInput
// Related: framing.go — FrameWriter, the newline framing this line goes out in

package rpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/ze-software/ze/internal/core/opererr"
)

// declarationRequestID is the request id a declaration line carries. A live
// start numbers Stage 1 as #1, and a query answer is that same message on a
// second carrier, so it carries the same id.
const declarationRequestID = 1

// WriteDeclaration writes reg to out as one newline-framed Stage 1 request
// line, and returns nil once the line is written.
//
// This is the whole definition of that line: the id, the method, the framing
// and the JSON encoding are stated here and nowhere else. Two binaries write
// it, the ze binary for a plugin it carries and the SDK for a plugin binary it
// does not, and one reader parses both. A second definition would let the two
// disagree with nothing to arbitrate them.
//
// A declaration that names no command and no pipe still writes the line.
// "Declared nothing" and "sent nothing" are different answers, and a reader
// that cannot tell them apart reports a working plugin as silent.
func WriteDeclaration(out io.Writer, reg *DeclareRegistrationInput) error {
	params, err := json.Marshal(reg)
	if err != nil {
		return fmt.Errorf("marshal declaration: %w", err)
	}

	line := FormatRequest(declarationRequestID, MethodDeclareRegistration, params)
	if writeErr := NewFrameWriter(out).Write(line); writeErr != nil {
		return fmt.Errorf("write declaration: %w", writeErr)
	}
	return nil
}

// DeclarationSection is the section of protocolPage that lists the keys of a
// Stage 1 declaration. A refusal of a declaration names it as the place its
// fix is read.
const DeclarationSection = "Stage 1: Registration (Plugin to Engine)"

// retiredSchemaKey is the key under which a declaration carried its one YANG
// module before it carried a list under "schemas". No release shipped it, so
// ze reads no value under it; it names it in a refusal of its own because its
// fix is known.
const retiredSchemaKey = "schema"

// errTrailingDeclaration is what DecodeDeclaration answers for a declaration
// followed by more text, which json.Unmarshal refuses and a json.Decoder
// stops before.
var errTrailingDeclaration = errors.New("the declaration is followed by more text")

// DecodeDeclaration decodes params, the JSON of the Stage 1 declaration the
// plugin named plugin sent. It is the one statement of how a declaration is
// read, and every reader of one calls it: the daemon's startup handshake, its
// early read at `ze start`, `show plugin declarations` and the conformance
// checker, for a plugin compiled into ze and an external one alike. So none of
// them can accept a declaration another refuses.
//
// A declaration is read whole or refused. JSON that does not decode answers
// its decode error, for the caller to place in its own sentence. A
// declaration that carries a key ze does not know, at its top or inside an
// object it defines, answers an operator error that names the plugin: ze would
// otherwise start the plugin without what that key declares, with no word to
// anyone. No type of the declaration decodes itself (no UnmarshalJSON), so the
// strict decoder reaches every nested object.
//
// A declaration that declares no protocol version is answered as it decodes.
// It speaks the text-era contract, whose keys are not these, and the version
// refusal is the one refusal it gets (D-2, NegotiateProtocolVersion), which
// the Stage 1 check table gives it first.
func DecodeDeclaration(plugin string, params []byte) (DeclareRegistrationInput, error) {
	var declaration DeclareRegistrationInput
	strict := json.NewDecoder(bytes.NewReader(params))
	strict.DisallowUnknownFields()
	strictErr := strict.Decode(&declaration)
	if strictErr == nil {
		if _, endErr := strict.Token(); errors.Is(endErr, io.EOF) {
			return declaration, nil
		}
	}

	// The strict read failed or stopped before the end. The lenient read
	// tells JSON that is no declaration from one that carries an unknown key.
	var lenient DeclareRegistrationInput
	if err := json.Unmarshal(params, &lenient); err != nil {
		return DeclareRegistrationInput{}, err
	}
	if strictErr == nil {
		// json.Unmarshal refuses the trailing text the strict read stopped
		// before, so this is not reached; it keeps that read from ever being
		// accepted if the two decoders came to disagree.
		return DeclareRegistrationInput{}, errTrailingDeclaration
	}
	if lenient.ProtocolVersion < 1 {
		// A guard rests on this zero: the protocol-version check, first in the
		// Stage 1 table (NegotiateProtocolVersion), refuses what this answers,
		// in the handshake before the plugin runs and in the checker's report.
		return lenient, nil
	}
	if carriesTopLevelKey(params, retiredSchemaKey) {
		return DeclareRegistrationInput{}, retiredSchemaRefusal(plugin)
	}
	return DeclareRegistrationInput{}, opererr.Wrap(strictErr,
		"plugin \""+plugin+"\" sent a Stage 1 declaration that carries a key ze does not know",
		"ze reads every key of a declaration, and a key it skipped would start the plugin without what the key declares",
		"remove the key, or spell it as "+protocolPage+" lists the keys under \""+DeclarationSection+"\"")
}

// carriesTopLevelKey reports whether params, a JSON object, holds key at its
// top level. It is false for JSON that is not an object.
func carriesTopLevelKey(params []byte, key string) bool {
	var keys map[string]json.RawMessage
	if json.Unmarshal(params, &keys) != nil {
		return false
	}
	_, carries := keys[key]
	return carries
}

// retiredSchemaRefusal is the refusal of a declaration that carries its YANG
// module under the retired "schema" key.
func retiredSchemaRefusal(plugin string) error {
	return opererr.New(
		"plugin \""+plugin+"\" sent its YANG module under \""+retiredSchemaKey+"\", a key ze no longer reads",
		"a Stage 1 declaration carries the plugin's YANG modules as a list under \"schemas\", one entry for each module, "+
			"and ze would otherwise start the plugin without its module and its commands",
		"send \"schemas\":[{\"module\":...,\"namespace\":...,\"yang-text\":...}] in place of \"schema\":{...} ("+
			protocolPage+", \""+DeclarationSection+"\")")
}

// DeclareRegistrationOutput is the engine's answer to MethodDeclareRegistration.
type DeclareRegistrationOutput struct {
	// ProtocolVersion is the plugin protocol version both sides speak from
	// here on: the lower of the plugin's declared version and the engine's.
	ProtocolVersion int `json:"protocol-version"`
	// PluginName is the configured identity of the authenticated process.
	PluginName string `json:"plugin-name"`
}

// protocolPage is where a plugin author reads the versioned contract.
const protocolPage = "docs/plugin-development/protocol.md"

// NegotiateProtocolVersion answers the protocol version the engine speaks with
// plugin, which declared declared at Stage 1: the lower of the two. A plugin
// that declared no version (zero or negative) speaks the text-era contract,
// which this engine no longer speaks, so it is refused with an error naming
// the plugin, the version it must declare and the page that documents it.
func NegotiateProtocolVersion(plugin string, declared int) (int, error) {
	if declared < 1 {
		return 0, fmt.Errorf("plugin %s declared no protocol version in declare-registration, "+
			"so it speaks the text-era filter contract this engine refused: "+
			"declare \"protocol-version\": %d and speak that version (%s)",
			plugin, ProtocolVersion, protocolPage)
	}
	return min(declared, ProtocolVersion), nil
}
