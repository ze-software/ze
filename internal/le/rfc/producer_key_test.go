package rfc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// producerKeySource is the shape of internal/core/bgp/attribute/text_append.go:
// several types each declare an AppendText method, one with a pointer
// receiver, and one function's name is unique in the file.
const producerKeySource = `package attribute

// AppendText of a standard community.
func (c Community) AppendText(buf []byte) []byte {
	return append(buf, 'c')
}

// AppendText of a large community.
func (l LargeCommunity) AppendText(buf []byte) []byte {
	return append(buf, 'l')
}

// AppendText of an extended community, through a pointer receiver.
func (e *ExtendedCommunity) AppendText(buf []byte) []byte {
	return append(buf, 'e')
}

// Unique is the only function of its name.
func Unique() int {
	return 1
}
`

// producerKeyTree writes producerKeySource as attr.go in a fresh tree.
func producerKeyTree(t *testing.T) string {
	t.Helper()
	tree := t.TempDir()
	if err := os.WriteFile(filepath.Join(tree, "attr.go"), []byte(producerKeySource), 0o600); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}
	return tree
}

// TestProducerLineSpanResolvesTheReceiversMethod checks the coverage span a
// revert producer is judged over: a `Type.Method` key spans that receiver's
// method and no other, a pointer receiver is named `Type.Method` too, a unique
// bare name still resolves, and a bare name three methods share is refused
// naming each exact key rather than spanning the first of them. Method: the
// fixture's line numbers are fixed, so each span is compared with them.
func TestProducerLineSpanResolvesTheReceiversMethod(t *testing.T) {
	tree := producerKeyTree(t)
	for _, tc := range []struct {
		symbol string
		want   lineRange
	}{
		{"LargeCommunity.AppendText", lineRange{first: 8, last: 11}},
		{"ExtendedCommunity.AppendText", lineRange{first: 13, last: 16}},
		{"Community.AppendText", lineRange{first: 3, last: 6}},
		{"Unique", lineRange{first: 18, last: 21}},
	} {
		got, refusal := producerLineSpan(tree, "attr.go", tc.symbol)
		if refusal != "" {
			t.Errorf("producerLineSpan(%q) refused: %s", tc.symbol, refusal)
			continue
		}
		if got != tc.want {
			t.Errorf("producerLineSpan(%q) = %+v, want %+v", tc.symbol, got, tc.want)
		}
	}

	_, refusal := producerLineSpan(tree, "attr.go", "AppendText")
	for _, want := range []string{"attr.go::AppendText is ambiguous", "attr.go::Community.AppendText",
		"attr.go::LargeCommunity.AppendText", "attr.go::ExtendedCommunity.AppendText"} {
		if !strings.Contains(refusal, want) {
			t.Errorf("a bare name three methods share: refusal %q does not hold %q", refusal, want)
		}
	}

	_, refusal = producerLineSpan(tree, "attr.go", "Missing.AppendText")
	for _, want := range []string{"attr.go declares no Missing.AppendText",
		"attr.go::LargeCommunity.AppendText"} {
		if !strings.Contains(refusal, want) {
			t.Errorf("a receiver with no such method: refusal %q does not hold %q", refusal, want)
		}
	}
}

// TestRevertBreakDisablesOnlyTheNamedMethod checks the overlay the revert
// route builds: `LargeCommunity.AppendText` disables that method's body, and
// the two other AppendText methods in the file keep theirs.
func TestRevertBreakDisablesOnlyTheNamedMethod(t *testing.T) {
	tree := producerKeyTree(t)
	record := DiscriminationRecord{Producer: "attr.go::LargeCommunity.AppendText", Source: "w"}
	broken, err := revertBreak(newSourceReader(tree), newScopeIndex(), &record)
	if err != nil {
		t.Fatalf("revertBreak refused a method key: %v", err)
	}
	if strings.Count(broken.content, revertBody) != 1 {
		t.Fatalf("the overlay disables %d bodies, want 1:\n%s", strings.Count(broken.content, revertBody), broken.content)
	}
	for _, kept := range []string{"append(buf, 'c')", "append(buf, 'e')"} {
		if !strings.Contains(broken.content, kept) {
			t.Errorf("the overlay lost %q, a method the key did not name", kept)
		}
	}
	if strings.Contains(broken.content, "append(buf, 'l')") {
		t.Error("the named method's body survived the revert")
	}

	record = DiscriminationRecord{Producer: "attr.go::AppendText", Source: "w"}
	if _, err := revertBreak(newSourceReader(tree), newScopeIndex(), &record); err == nil ||
		!strings.Contains(err.Error(), "is ambiguous") {
		t.Errorf("a bare name three methods share was not refused as ambiguous: %v", err)
	}
}

// TestFingerprintKeyRefusesTheMethodExpression checks that `Type.Method` is
// the one spelling a key takes: the Go method expression, pointer or value, is
// refused with the key to write instead.
func TestFingerprintKeyRefusesTheMethodExpression(t *testing.T) {
	rel, symbol, err := fingerprintKey("rpki/blackhole.go::ROACache.invalidByLengthOnly", "w")
	if err != nil {
		t.Fatalf("the canonical key was refused: %v", err)
	}
	if rel != "rpki/blackhole.go" || symbol != "ROACache.invalidByLengthOnly" {
		t.Errorf("fingerprintKey split it into %q and %q", rel, symbol)
	}
	for _, key := range []string{"rpki/blackhole.go::(*ROACache).invalidByLengthOnly",
		"rpki/blackhole.go::(ROACache).invalidByLengthOnly"} {
		_, _, err := fingerprintKey(key, "w")
		if err == nil {
			t.Errorf("the method expression %q was accepted", key)
			continue
		}
		if !strings.Contains(err.Error(), "write 'rpki/blackhole.go::ROACache.invalidByLengthOnly'") {
			t.Errorf("the refusal of %q does not name the canonical key: %v", key, err)
		}
	}
}

// TestResolveProducerTakesTheKeySpelling checks an annotation's producer
// resolves as a record's does: `Type.Method` is found, a pointer receiver
// included, a bare name several methods share is refused as ambiguous, and the
// method expression is refused naming the canonical spelling.
func TestResolveProducerTakesTheKeySpelling(t *testing.T) {
	reader := newSourceReader(producerKeyTree(t))
	for _, producer := range []string{"attr.go::LargeCommunity.AppendText",
		"attr.go::ExtendedCommunity.AppendText", "attr.go::Unique"} {
		if state, why := resolveProducer(reader, producer); state != producerFound {
			t.Errorf("resolveProducer(%q) = %v (%s), want found", producer, state, why)
		}
	}
	for _, tc := range []struct{ producer, want string }{
		{"attr.go::AppendText", "is ambiguous"},
		{"attr.go::(*ExtendedCommunity).AppendText", "write attr.go::ExtendedCommunity.AppendText"},
		{"attr.go::Missing.AppendText", "attr.go declares no Missing.AppendText"},
	} {
		state, why := resolveProducer(reader, tc.producer)
		if state != producerUnresolved || !strings.Contains(why, tc.want) {
			t.Errorf("resolveProducer(%q) = %v %q, want unresolved holding %q", tc.producer, state, why, tc.want)
		}
	}
}
