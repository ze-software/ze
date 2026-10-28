package config

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/secret"
)

// secretWalkSchema answers a schema that puts a ze:sensitive leaf under each
// node kind a schema allows to hold one: a container, a list, a flex node
// and an inline list. No shipped YANG module puts one under the last two
// today; the schema allows it, so the encode walks must follow it.
func secretWalkSchema() *Schema {
	sensitiveLeaf := func() *LeafNode {
		leaf := Leaf(TypeString)
		leaf.Sensitive = true
		return leaf
	}
	schema := NewSchema()
	schema.Define("box", Container(Field("key", sensitiveLeaf())))
	schema.Define("peer", List(TypeString, Field("key", sensitiveLeaf())))
	schema.Define("auth", Flex(Field("key", sensitiveLeaf())))
	schema.Define("user", InlineList(TypeString, Field("key", sensitiveLeaf())))
	return schema
}

// secretWalkKeys names each place secretWalkSchema holds a key, with the key
// the tests store there. Each key is distinctive, so a substring search
// finds one written in clear.
var secretWalkKeys = map[string]string{
	"box":  "box-key-a41f0c",
	"peer": "peer-key-6e2b9d",
	"auth": "auth-key-90c3e7",
	"user": "user-key-15d8b4",
}

// secretWalkTree answers a tree that holds a key at each place of
// secretWalkSchema, stored where the parsers store it: a container's and a
// flex node's children in the container of that name, a list's and an
// inline list's in the entry of the list of that name.
func secretWalkTree() *Tree {
	tree := NewTree()
	tree.GetOrCreateContainer("box").Set("key", secretWalkKeys["box"])
	tree.GetOrCreateContainer("auth").Set("key", secretWalkKeys["auth"])
	for _, list := range []string{"peer", "user"} {
		entry := NewTree()
		entry.Set("key", secretWalkKeys[list])
		tree.AddListEntry(list, "e1", entry)
	}
	return tree
}

// TestEncodeSecretsFollowsEveryNodeKind prints a tree that holds a
// ze:sensitive key under a container, a list, a flex node and an inline list.
//
// VALIDATES: each of the four keys is printed in a $9$ form that decodes to
// it, by the hierarchical and the set printer, and none in clear.
// PREVENTS: a walk that reads a flex node's or an inline list's children at
// the parent's level, where the tree holds nothing, printing the key in clear.
func TestEncodeSecretsFollowsEveryNodeKind(t *testing.T) {
	schema := secretWalkSchema()
	tree := secretWalkTree()

	printed := EncodeSecrets(tree, schema)
	read := map[string]string{
		"box":  mustGet(t, printed.GetContainer("box"), "box"),
		"auth": mustGet(t, printed.GetContainer("auth"), "auth"),
		"peer": mustGet(t, printed.GetList("peer")["e1"], "peer"),
		"user": mustGet(t, printed.GetList("user")["e1"], "user"),
	}
	for place, got := range read {
		decoded, err := secret.Decode(got)
		if err != nil {
			t.Errorf("%s: printed %q, not a $9$ form: %v", place, got, err)
			continue
		}
		if decoded != secretWalkKeys[place] {
			t.Errorf("%s: printed %q decodes to %q, want %q", place, got, decoded, secretWalkKeys[place])
		}
	}
	for _, text := range []string{Serialize(tree, schema), SerializeSet(tree, schema)} {
		for place, key := range secretWalkKeys {
			if strings.Contains(text, key) {
				t.Errorf("%s: the printer wrote the key in clear:\n%s", place, text)
			}
		}
	}
}

// TestEncodeSecretMetaFollowsEveryNodeKind encodes a meta that holds a
// session entry for the key under each of the four node kinds, as the
// editor records it (cli walkOrCreateMeta): a container's and a flex node's
// entries in meta.containers[name], a list's and an inline list's in
// meta.containers[name].lists[key].
//
// VALIDATES: every entry's value and ^previous key leave in a $9$ form that
// decodes to the key, none in clear.
// PREVENTS: a set-meta writer printing a session's key in clear under a flex
// node or an inline list.
func TestEncodeSecretMetaFollowsEveryNodeKind(t *testing.T) {
	schema := secretWalkSchema()
	tree := secretWalkTree()
	meta := NewMetaTree()
	places := map[string]*MetaTree{
		"box":  meta.GetOrCreateContainer("box"),
		"auth": meta.GetOrCreateContainer("auth"),
		"peer": meta.GetOrCreateContainer("peer").GetOrCreateListEntry("e1"),
		"user": meta.GetOrCreateContainer("user").GetOrCreateListEntry("e1"),
	}
	for place, at := range places {
		at.SetEntry("key", MetaEntry{User: "bob", Value: secretWalkKeys[place], Previous: secretWalkKeys[place]})
	}

	encoded := encodeSecretMeta(EncodeSecrets(tree, schema), meta, schema)
	for place, at := range map[string]*MetaTree{
		"box":  encoded.GetContainer("box"),
		"auth": encoded.GetContainer("auth"),
		"peer": encoded.GetContainer("peer").GetListEntry("e1"),
		"user": encoded.GetContainer("user").GetListEntry("e1"),
	} {
		entry, ok := at.GetEntry("key")
		if !ok {
			t.Fatalf("%s: the encoded meta lost the entry", place)
		}
		for field, got := range map[string]string{"value": entry.Value, "previous": entry.Previous} {
			decoded, err := secret.Decode(got)
			if err != nil {
				t.Errorf("%s %s: %q is not a $9$ form: %v", place, field, got, err)
				continue
			}
			if decoded != secretWalkKeys[place] {
				t.Errorf("%s %s: %q decodes to %q, want %q", place, field, got, decoded, secretWalkKeys[place])
			}
		}
	}
}

// mustGet answers the key leaf of tree, failing the test when tree or the
// leaf is absent.
func mustGet(t *testing.T, tree *Tree, place string) string {
	t.Helper()
	if tree == nil {
		t.Fatalf("%s: the printed tree lost the node", place)
	}
	value, ok := tree.Get("key")
	if !ok {
		t.Fatalf("%s: the printed tree lost the key", place)
	}
	return value
}
