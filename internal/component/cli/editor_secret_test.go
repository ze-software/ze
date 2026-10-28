//go:build ze_bgp

package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/secret"
)

// TestSetValueDecodesStoredSecret sets a peer's TCP MD5 key (a ze:sensitive
// leaf) in its $9$ form through Editor.SetValue, the write a gNMI Set, a
// REST or gRPC config session and the web editor make, once with no session
// and once inside one.
//
// VALIDATES: the editor's tree holds the decoded key, as every config parser
// stores it, and the content the editor writes carries the $9$ text the
// client sent, byte for byte.
// PREVENTS: a $9$ text a client set being held as the key itself, so the
// daemon would send the encoded text to the peer as its MD5 key.
func TestSetValueDecodesStoredSecret(t *testing.T) {
	const key = "md5-key-set-3d81a0-tail"
	stored, err := secret.Encode(key)
	require.NoError(t, err)
	md5Path := []string{"bgp", "peer", "peer1", "connection", "md5"}

	for _, withSession := range []bool{false, true} {
		configPath := writeTestConfig(t, validBGPConfig)
		ed, err := NewEditorWithStorage(newTestTreeStore(t, configPath), configPath)
		require.NoError(t, err)
		t.Cleanup(func() { ed.Close() }) //nolint:errcheck // test cleanup
		if withSession {
			ed.SetSession(NewEditSession("carol", "gnmi"))
		}

		require.NoError(t, ed.SetValue(md5Path, "password", stored))

		held, ok := ed.Tree().GetContainerPath("bgp").GetList("peer")["peer1"].GetContainerPath("connection/md5").Get("password")
		require.True(t, ok, "session=%v: no md5 password under peer1", withSession)
		require.Equal(t, key, held, "session=%v: the tree holds the client's text, not the decoded key", withSession)
		written := ed.WorkingContent()
		require.Contains(t, written, stored, "session=%v: the editor did not keep the $9$ text it was given", withSession)
		require.NotContains(t, written, key, "session=%v: the editor wrote the key in clear", withSession)
	}
}

// TestContestedSecretNeverWrittenInClear drives two editor sessions on one
// store: bob sets a peer's TCP MD5 key (a ze:sensitive leaf), then alice
// deletes it. The shared draft then holds no value for the leaf while bob's
// session entry still carries his key, which SerializeSetWithMeta writes as
// an orphan `set` line (writeDeleteMetaLines).
//
// VALIDATES: neither the draft nor either session's change file carries the
// key bob typed, nor its distinctive tail; the orphan line carries a $9$ form.
// PREVENTS: the one meta writer the printer encoding did not reach writing a
// contested credential in clear to disk.
func TestContestedSecretNeverWrittenInClear(t *testing.T) {
	const bobKey = "bob-md5-key-7f3e91-tail"
	const bobKeyTail = "7f3e91-tail"
	const oldKeyTail = "0c2a5d-tail"
	md5Path := []string{"bgp", "peer", "peer1", "connection", "md5"}

	// The committed key gives alice's session a leaf to delete.
	withKey := strings.Replace(validBGPConfig, "\t\tconnection {\n",
		"\t\tconnection {\n\t\t\tmd5 { password old-md5-key-"+oldKeyTail+"; }\n", 1)
	require.Contains(t, withKey, oldKeyTail, "the fixture lost its md5 key, so this case proves nothing")
	configPath := writeTestConfig(t, withKey)
	store := newTestTreeStore(t, configPath)

	bob, err := NewEditorWithStorage(store, configPath)
	require.NoError(t, err)
	t.Cleanup(func() { bob.Close() }) //nolint:errcheck // test cleanup
	bob.SetSession(NewEditSession("bob", "ssh"))

	alice, err := NewEditorWithStorage(store, configPath)
	require.NoError(t, err)
	t.Cleanup(func() { alice.Close() }) //nolint:errcheck // test cleanup
	alice.SetSession(NewEditSession("alice", "ssh"))

	require.NoError(t, bob.SetValue(md5Path, "password", bobKey))
	require.NoError(t, bob.SaveDraft())
	require.NoError(t, alice.DeleteValue(md5Path, "password"))
	require.NoError(t, alice.SaveDraft())

	draft, err := store.ReadFile(DraftPath(configPath))
	require.NoError(t, err)
	require.Contains(t, string(draft), "#bob", "the draft carries no entry of bob's, so this case proves nothing")
	require.Contains(t, string(draft), "md5 password $9$",
		"the draft carries no line for the contested key, so this case proves nothing")

	written := map[string]string{"draft": string(draft)}
	for _, user := range []string{"alice", "bob"} {
		change, readErr := store.ReadFile(ChangePath(configPath, user))
		if readErr == nil {
			written[user+" change file"] = string(change)
		}
	}
	for name, text := range written {
		if strings.Contains(text, bobKeyTail) {
			t.Errorf("the %s wrote bob's key in clear:\n%s", name, text)
		}
		if strings.Contains(text, oldKeyTail) {
			t.Errorf("the %s wrote the committed key in clear:\n%s", name, text)
		}
	}
}

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
