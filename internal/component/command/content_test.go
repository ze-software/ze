// Design: docs/architecture/api/commands.md -- bounded caller-supplied bytes.

package command

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

// TestDecodeContentPreservesThePacketBoundary checks the common codec used by
// supplied readers and existing store writers at both sides of the cap.
func TestDecodeContentPreservesThePacketBoundary(t *testing.T) {
	want := bytes.Repeat([]byte{'x'}, ContentOctetsMax)
	encoded := base64.StdEncoding.EncodeToString(want)
	if len(encoded) != ContentEncodedMax {
		t.Fatalf("encoded/decoded packet budgets disagree: %d, %d", len(encoded), ContentEncodedMax)
	}
	got, err := DecodeContent(encoded)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("exact packet boundary did not round-trip: %d bytes, %v", len(got), err)
	}
	tooLong := base64.StdEncoding.EncodeToString(append(want, 'x'))
	got, err = DecodeContent(tooLong)
	if err == nil || got != nil || strings.Contains(err.Error(), tooLong) {
		t.Fatalf("oversize content was not refused without echoing it: %d bytes, %v", len(got), err)
	}
}

// TestDecodeContentRefusesMalformedBytesWithoutEcho checks decoder failure and
// leaves the meaning of an empty value with the owner, as existing writes do.
func TestDecodeContentRefusesMalformedBytesWithoutEcho(t *testing.T) {
	const malformed = "c2VjcmV0!!"
	data, err := DecodeContent(malformed)
	if err == nil || data != nil || strings.Contains(err.Error(), malformed) {
		t.Fatalf("malformed content was not refused safely: %q, %v", data, err)
	}
	data, err = DecodeContent("")
	if err != nil || len(data) != 0 {
		t.Fatalf("empty content changed the owning command's contract: %q, %v", data, err)
	}
}
