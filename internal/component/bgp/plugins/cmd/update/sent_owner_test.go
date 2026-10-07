// Design: docs/architecture/plugin/rib-storage-design.md -- captured sent ownership.
package update

import (
	"encoding/json"
	"testing"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
)

// TestSentOwnerMetadataKeepsExactReceipt passes the maximum receipt through
// the external JSON representation and the same parser used by UPDATE dispatch.
func TestSentOwnerMetadataKeepsExactReceipt(t *testing.T) {
	data, err := json.Marshal(map[string]any{bgptypes.SentOwnerMessageMeta: "18446744073709551615", "rib-lifecycle": true})
	if err != nil {
		t.Fatal(err)
	}
	var meta map[string]any
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatal(err)
	}
	message, err := sentOwnerMessageFromMeta(meta)
	if err != nil {
		t.Fatal(err)
	}
	if message != ^uint64(0) {
		t.Fatalf("receipt = %d, want full uint64 precision", message)
	}
}

// TestSentOwnerMetadataRejectsImplicitLocalCleanup prevents a missing or
// malformed automatic-owner receipt from taking unrestricted operator authority.
func TestSentOwnerMetadataRejectsImplicitLocalCleanup(t *testing.T) {
	for _, meta := range []map[string]any{
		{"rib-lifecycle": true},
		{bgptypes.SentOwnerMessageMeta: float64(1)},
		{bgptypes.SentOwnerMessageMeta: "0"},
		{bgptypes.SentOwnerMessageMeta: "-1"},
		{bgptypes.SentOwnerMessageMeta: "18446744073709551616"},
	} {
		if _, err := sentOwnerMessageFromMeta(meta); err == nil {
			t.Errorf("accepted malformed sent owner metadata: %v", meta)
		}
	}
	message, err := sentOwnerMessageFromMeta(nil)
	if err != nil || message != 0 {
		t.Fatalf("explicit operator command changed authority: receipt=%d, err=%v", message, err)
	}
}
