// Design: docs/architecture/ike/ipsec-14-responder.md -- unauthenticated input during the handshake, both roles
// Related: protected_input.go -- protectedPayload and openProtected, the producers these tests hold

package engine

import (
	"bytes"
	"encoding/binary"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// testReaderReason is the suppressed-notify reason the tests' reader drops under. No
// engine site counts under it, so a test asserting the rise of an engine reason never
// sees the reader's drops.
const testReaderReason = "test-reader-unauthenticated"

// errTestReaderDropped is what the tests' reader answers for a message openProtected
// dropped: one that does not prove it came from the SA's peer.
var errTestReaderDropped = errors.New("test reader: openProtected dropped the message as unauthenticated")

// decryptAndParse is the tests' reader of an SK-protected message, the far end's view of
// what Ze sent or the near end's of a crafted datagram. The engine function of this name
// is gone: openProtected replaced it. The reader opens raw for sa through openProtected,
// the engine's one opener, so a test that asserts a message is refused proves the
// production entry refuses it. A drop is answered errTestReaderDropped, which does not
// wrap errInnerParse; an authenticated malformed message answers openProtected's error,
// which does.
func decryptAndParse(sa *SA, msg *wire.Message, raw []byte) ([]wire.PayloadEntry, error) {
	inner, dropped, err := openProtected(sa, msg, raw, testReaderReason, slogutil.DiscardLogger())
	if dropped {
		return nil, errTestReaderDropped
	}
	return inner, err
}

// VALIDATES: AC-10. In the package's non-test files, decryptSKPayload is called exactly
// once, from openProtected, and neither function openProtected replaced is declared.
//
// METHOD: every non-test .go file of the package directory is parsed with go/parser,
// whatever its build tags, and each call expression whose function is the identifier
// decryptSKPayload is attributed to the function declaration that encloses it. The
// declaration of decryptSKPayload must be found, so a read that saw no file fails.
//
// PREVENTS: a handler calling the primitive itself and acting on an unauthenticated
// failure, which RFC 7296 Section 2.4 forbids. It cannot see a function value or a method
// expression that aliases the primitive, so it proves no more than that no handler calls
// it by name.
func TestOneFunctionOpensEveryProtectedMessage(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read the package directory: %v", err)
	}
	fset := token.NewFileSet()
	declared := false
	var callers []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			switch fn.Name.Name {
			case "decryptSKPayload":
				declared = true
			case "decryptAndParse", "discardUnauthenticatedHandshakeRequest":
				t.Errorf("%s declares %s, which openProtected replaced", name, fn.Name.Name)
			}
			if fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "decryptSKPayload" {
					callers = append(callers, fset.Position(call.Pos()).String()+" in "+fn.Name.Name)
				}
				return true
			})
		}
	}
	if !declared {
		t.Fatal("no non-test file declares decryptSKPayload, so this test read nothing")
	}
	if len(callers) != 1 {
		t.Fatalf("decryptSKPayload is called %d times in non-test files, want once, from openProtected: %v", len(callers), callers)
	}
	if !strings.HasSuffix(callers[0], " in openProtected") {
		t.Fatalf("decryptSKPayload is called from %s, want openProtected", callers[0])
	}
}

// VALIDATES: a message whose chain holds two Encrypted payloads is opened by its first
// one, the one every opener takes.
//
// METHOD: a 52-octet message is built by hand: the fixed header, an Encrypted payload of
// eight 0x11 octets whose Next Payload is 46, and a second Encrypted payload of eight
// 0x22 octets. wire.Message.ReadFrom parses both, and protectedPayload must answer the
// first.
//
// PREVENTS: two openers taking different payloads of one message (the initiator
// handlers took the last, the established-SA opener the first). RFC 7296 Section 3.14 makes the
// Encrypted payload the last payload, and ReadFrom does not enforce it, so the choice is
// the opener's and is pinned here.
func TestProtectedPayloadTakesTheFirstEncryptedPayload(t *testing.T) {
	const bodyOctets = 8
	payloadOctets := wire.GenericHeaderLen + bodyOctets
	total := wire.HeaderLen + 2*payloadOctets
	raw := make([]byte, total)
	raw[16] = wire.PayloadTypeSK
	raw[17] = 0x20 // Version 2.0.
	raw[18] = wire.ExchangeIKEAuth
	raw[19] = wire.FlagResponse
	binary.BigEndian.PutUint32(raw[24:28], uint32(total))
	first := wire.HeaderLen
	raw[first] = wire.PayloadTypeSK
	binary.BigEndian.PutUint16(raw[first+2:first+4], uint16(payloadOctets))
	copy(raw[first+wire.GenericHeaderLen:], bytes.Repeat([]byte{0x11}, bodyOctets))
	second := first + payloadOctets
	binary.BigEndian.PutUint16(raw[second+2:second+4], uint16(payloadOctets))
	copy(raw[second+wire.GenericHeaderLen:], bytes.Repeat([]byte{0x22}, bodyOctets))

	msg := parseMsg(t, raw)
	if len(msg.Payloads) != 2 {
		t.Fatalf("ReadFrom parsed %d payloads, want the 2 Encrypted payloads", len(msg.Payloads))
	}
	sk := protectedPayload(msg)
	if sk == nil {
		t.Fatal("protectedPayload found no Encrypted payload")
	}
	if !bytes.Equal(sk.CipherText, bytes.Repeat([]byte{0x11}, bodyOctets)) {
		t.Fatalf("protectedPayload took the Encrypted payload holding % x, want the first", sk.CipherText)
	}
}
