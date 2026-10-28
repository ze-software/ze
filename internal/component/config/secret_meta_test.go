package config

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/secret"
)

// Distinctive tails of the keys these cases hold, asserted on their own: a
// writer that escapes the key still publishes every character of the tail.
const (
	metaSessionKey     = "bob-session-key-b81e40"
	metaSessionKeyTail = "b81e40"
	metaPreviousKey    = "committed-key-5d72ac"
	metaPreviousTail   = "5d72ac"
)

// secretMetaFixture answers a tree in which user alice holds no api-secret
// (a ze:sensitive leaf of maskTestSchema), and a meta in which bob's session
// set one and alice's session deleted it, both over the committed key.
func secretMetaFixture() (*Tree, *MetaTree) {
	tree := NewTree()
	auth := tree.GetOrCreateContainer("system").GetOrCreateContainer("authentication")
	entry := NewTree()
	entry.Set("shell", "/bin/sh")
	auth.AddListEntry("user", "alice", entry)

	meta := NewMetaTree()
	userMeta := meta.GetOrCreateContainer("system").GetOrCreateContainer("authentication").
		GetOrCreateContainer("user").GetOrCreateListEntry("alice")
	at := time.Date(2026, 10, 28, 10, 0, 0, 0, time.UTC)
	userMeta.SetEntry("api-secret", MetaEntry{User: "bob", Source: "ssh", Time: at, Previous: metaPreviousKey, Value: metaSessionKey})
	userMeta.SetEntry("api-secret", MetaEntry{User: "alice", Source: "ssh", Time: at.Add(time.Minute), Previous: metaPreviousKey})
	return tree, meta
}

// TestSetMetaWritesContestedSecretEncoded prints a meta whose ze:sensitive
// leaf one session set and another deleted, then reads it back.
//
// VALIDATES: the orphan `set` line writeDeleteMetaLines prints carries the
// key in the $9$ form, the ^previous field of both lines does too, and the
// set-meta parser reads both back as the keys the sessions held.
// PREVENTS: a draft or change file holding a session's key, or the
// committed key it replaced, in clear.
func TestSetMetaWritesContestedSecretEncoded(t *testing.T) {
	schema := maskTestSchema()
	tree, meta := secretMetaFixture()

	text := SerializeSetWithMeta(tree, meta, schema)

	require.Contains(t, text, "#bob", "bob's line is missing, so this case proves nothing")
	require.Contains(t, text, "#alice", "alice's line is missing, so this case proves nothing")
	assert.NotContains(t, text, metaSessionKeyTail, "the session's key was written in clear")
	assert.NotContains(t, text, metaPreviousTail, "the ^previous key was written in clear")
	assert.Equal(t, 2, strings.Count(text, "^$9$"), "both ^previous fields must be in the $9$ form:\n%s", text)
	assert.Contains(t, text, "api-secret $9$", "the orphan set line carries no $9$ key:\n%s", text)

	_, readMeta, err := NewSetParser(schema).ParseWithMeta(text)
	require.NoError(t, err)
	userMeta := readMeta.GetContainer("system").GetContainer("authentication").
		GetContainer("user").GetListEntry("alice")
	require.NotNil(t, userMeta, "the read meta lost alice's entry")
	entries := userMeta.GetAllEntries("api-secret")
	require.Len(t, entries, 2)
	for _, e := range entries {
		assert.Equal(t, metaPreviousKey, e.Previous, "%s's ^previous did not read back as the key", e.User)
		if e.User == "bob" {
			assert.Equal(t, metaSessionKey, e.Value, "bob's value did not read back as the key")
		}
	}
}

// TestSetMetaRefusesUndecodablePrevious reads a delete line whose ^previous
// field is in the $9$ form and does not decode.
//
// VALIDATES: the set-meta parser refuses the line, naming the leaf.
// PREVENTS: an editor comparing a corrupt encoding with the committed key.
func TestSetMetaRefusesUndecodablePrevious(t *testing.T) {
	encoded, err := secret.Encode(metaPreviousKey)
	require.NoError(t, err)
	corrupt := encoded + "\x01"
	line := "#alice @ssh %2026-10-28T10:00:00Z ^" + corrupt + " delete system authentication user alice api-secret\n"

	_, _, err = NewSetParser(maskTestSchema()).ParseWithMeta("set system authentication user alice shell /bin/sh\n" + line)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "api-secret")
}
