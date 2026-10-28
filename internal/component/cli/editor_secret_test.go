//go:build ze_bgp

package cli

import (
	"strings"
	"testing"
)

// TestEditorKeepsStoredSecretEncoding loads a set config whose TCP MD5 key
// (a ze:sensitive leaf) is in the $9$ form into the editor, as the store
// holds it, and reads back what the editor would write.
//
// VALIDATES: the editor's tree holds the decoded key, as it does for a
// hierarchical file, and the content it writes carries the $9$ string it
// read byte for byte.
// PREVENTS: the editor writing a key it read encoded back in clear, or with
// a fresh salt on every commit.
func TestEditorKeepsStoredSecretEncoding(t *testing.T) {
	const stored = "$9$QI1cF39tpBEhrAtM87Nbw"
	const content = `set bgp router-id 10.0.0.1
set bgp session asn local 65533
set bgp peer peer3 connection remote ip 10.0.0.3
set bgp peer peer3 connection md5 password ` + stored + `
set bgp peer peer3 session asn remote 65534
`
	ed, err := NewEditorFromContent([]byte(content), "-")
	if err != nil {
		t.Fatalf("editor: %v", err)
	}
	key, ok := ed.tree.GetContainerPath("bgp").GetList("peer")["peer3"].GetContainerPath("connection/md5").Get("password")
	if !ok {
		t.Fatal("no md5 password under peer3")
	}
	if strings.HasPrefix(key, "$9$") {
		t.Errorf("the editor holds the key in the $9$ form %q, want it decoded", key)
	}

	written := ed.WorkingContent()
	if !strings.Contains(written, "password "+stored) {
		t.Errorf("the editor did not keep the stored encoding %s:\n%s", stored, written)
	}
	if strings.Contains(written, key) {
		t.Errorf("the editor wrote the key in clear:\n%s", written)
	}
}
