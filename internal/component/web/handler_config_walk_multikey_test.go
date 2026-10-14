package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/ze-software/ze/internal/component/bfd/yang" // Register the BFD YANG: single-hop-session is keyed `peer vrf interface`.
	"github.com/ze-software/ze/internal/component/config"
)

// twoLinkSessionsConfig is the AC-2 config of spec-bfd-link-local-peer-zone:
// one link-local peer on two links, each entry with its own profile.
const twoLinkSessionsConfig = `bfd {
	single-hop-session fe80::b {
		interface zbl0
		profile p0
	}
	single-hop-session fe80::b {
		interface zbl1
		profile p1
	}
}
`

// sessionList returns the bfd single-hop-session entries of tree.
func sessionList(t *testing.T, tree *config.Tree) map[string]*config.Tree {
	t.Helper()
	bfd := tree.GetContainer("bfd")
	require.NotNil(t, bfd, "bfd container")
	return bfd.GetList("single-hop-session")
}

// TestWebWalkAddressesMultiLeafKeyEntry is AC-13 of
// spec-bfd-link-local-peer-zone.
//
// GOAL: the web editor reaches the second entry of the AC-2 tree by its
// address as path segments. METHOD: walk the schema and the tree, build the
// context heading and the list table over the registered BFD YANG.
//
// VALIDATES: `.../single-hop-session/fe80::b/interface/zbl1` resolves to the
// zbl1 entry (never zbl0); the heading names one single-hop-session entry
// keyed `fe80::b interface zbl1`, never a list named `interface`; the list
// table heads its key column `peer` and links each row to its own address.
// PREVENTS: the walkers reading `interface` as a child name after a one-word
// key, which addressed the zbl0 entry or nothing.
func TestWebWalkAddressesMultiLeafKeyEntry(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree, err := config.NewParser(schema).Parse(twoLinkSessionsConfig)
	require.NoError(t, err)

	zbl1Path := []string{"bfd", "single-hop-session", "fe80::b", "interface", "zbl1"}
	entry := walkTree(tree, schema, zbl1Path)
	require.NotNil(t, entry, "the zbl1 entry is reached by its address segments")
	profile, _ := entry.Get("profile")
	assert.Equal(t, "p1", profile, "the walk reaches the zbl1 entry, never the zbl0 one")
	zbl0 := walkTree(tree, schema, []string{"bfd", "single-hop-session", "fe80::b", "interface", "zbl0"})
	require.NotNil(t, zbl0)
	profile, _ = zbl0.Get("profile")
	assert.Equal(t, "p0", profile)
	assert.Nil(t, walkTree(tree, schema, []string{"bfd", "single-hop-session", "fe80::b"}),
		"the address with no interface is the separate entry fe80::b, absent here")

	assert.True(t, isListEntryPath(schema, zbl1Path), "the path ends at an entry address")
	node, err := walkSchema(schema, append(append([]string{}, zbl1Path...), "profile"))
	require.NoError(t, err)
	_, isLeaf := node.(*config.LeafNode)
	assert.True(t, isLeaf, "profile after the address is the entry's leaf")

	assert.Equal(t, []ContextEntry{{ListName: "single-hop-session", Key: "fe80::b interface zbl1"}},
		buildContextHeading(schema, zbl1Path))

	listPath := []string{"bfd", "single-hop-session"}
	listNode, ok := findListNode(schema, listPath)
	require.True(t, ok)
	keys := collectListKeys(tree, schema, listPath)
	table := buildListTable(tree, schema, listPath, listNode, keys, nil, "/show/bfd/single-hop-session/")
	require.NotEmpty(t, table.Columns)
	assert.Equal(t, "peer", table.Columns[0].Name, "the key column is headed by the first key leaf")
	urls := make(map[string]string, len(table.Rows))
	for _, row := range table.Rows {
		urls[row.KeyValue] = row.URL
	}
	assert.Equal(t, map[string]string{
		"fe80::b interface zbl0": "/show/bfd/single-hop-session/fe80::b/interface/zbl0/",
		"fe80::b interface zbl1": "/show/bfd/single-hop-session/fe80::b/interface/zbl1/",
	}, urls, "each row links to its own address segments")
}

// TestWebRenameOfMultiLeafKeyEntryChangesOnlyTheFirstValue is the web half of
// AC-18 of spec-bfd-link-local-peer-zone.
//
// GOAL: the web rename form and the web terminal move a multi-leaf-key entry
// by its first value only. METHOD: build the AC-2 entries in an
// EditorManager, then POST /config/rename/ and run execListEntryOp.
//
// VALIDATES: a rename to `fe80::c` moves the zbl0 entry to
// `fe80::c interface zbl0`, which still holds `interface zbl0`; a destination
// changing `interface` is refused on both routes naming the list and the leaf,
// and nothing changes; a destination equal to the source after carry-over is
// refused with "new key must differ from current key".
// PREVENTS: the form storing the entry under a key whose body disagrees with
// it, or reporting a usage error that names neither the list nor the leaf.
func TestWebRenameOfMultiLeafKeyEntryChangesOnlyTheFirstValue(t *testing.T) {
	mgr, schema := newHandlerTestManager(t)
	for _, link := range []string{"zbl0", "zbl1"} {
		require.NoError(t, mgr.SetValue("alice", []string{"bfd", "single-hop-session", "fe80::b", "interface", link}, "profile", "p-"+link))
	}
	handler := handleConfigRename(mgr, schema)
	post := func(newKey string) *httptest.ResponseRecorder {
		req := postConfigRequest(t, "/config/rename/bfd/single-hop-session/fe80::b/interface/zbl0/", url.Values{"new-key": {newKey}}, "alice")
		req.Header.Set("HX-Request", "true")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	rec := post("fe80::b interface zbl2")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "rename refused: interface is part of the key of single-hop-session")
	assert.Contains(t, rec.Body.String(), "delete bfd single-hop-session fe80::b interface zbl0")
	assert.Contains(t, rec.Body.String(), "set bfd single-hop-session fe80::b interface zbl2")

	rec = post("fe80::b")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "new key must differ from current key")
	assert.Len(t, sessionList(t, mgr.Tree("alice")), 2, "the refusals change nothing")

	_, out := execListEntryOp(schema, []string{"bfd", "single-hop-session", "fe80::b", "interface", "zbl1", "to", "fe80::b", "interface", "zbl2"},
		nil, listEntryCopy, func(parent []string, list, src string, dst []string) error {
			return mgr.CopyListEntry("alice", parent, list, src, dst)
		})
	assert.Contains(t, out, "copy refused: interface is part of the key of single-hop-session")
	assert.Len(t, sessionList(t, mgr.Tree("alice")), 2, "the terminal refusal changes nothing")

	rec = post("fe80::c")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	sessions := sessionList(t, mgr.Tree("alice"))
	assert.Nil(t, sessions["fe80::b interface zbl0"])
	moved := sessions["fe80::c interface zbl0"]
	require.NotNil(t, moved, "the rename changes only the first value: %v", sessions)
	link, _ := moved.Get("interface")
	assert.Equal(t, "zbl0", link, "the key leaf carries over into the body")
	profile, _ := sessions["fe80::b interface zbl1"].Get("profile")
	assert.Equal(t, "p-zbl1", profile, "the zbl1 entry is untouched")
}

// keylessSchema holds a keyless list `rule` with a keyed list `match` under
// it, the shape AC-19 refuses.
func keylessSchema() *config.Schema {
	match := config.List(config.TypeString, config.Field("value", config.Leaf(config.TypeString)))
	match.KeyName = "name"
	rule := config.List(config.TypeString,
		config.Field("action", config.Leaf(config.TypeString)),
		config.Field("match", match),
	)
	schema := config.NewSchema()
	schema.Define("policy", config.Container(config.Field("rule", rule)))
	return schema
}

// TestWebRenameAndTerminalRefuseAKeylessListEntry is the web half of AC-19 of
// spec-bfd-link-local-peer-zone.
//
// GOAL: the web rename form and the web terminal refuse a rename or copy of
// a keyless list's entry, and of a keyed entry under one, with the CLI's
// message. METHOD: POST /config/rename/ and run execListEntryOp over a
// schema holding a keyless list with a keyed list under it.
//
// VALIDATES: each route answers "<verb> refused: rule has no key, so its
// entries cannot be renamed or copied." and the op is never called.
// PREVENTS: the terminal renaming a keyless entry (it had no check), and the
// form answering with a message that names no list.
func TestWebRenameAndTerminalRefuseAKeylessListEntry(t *testing.T) {
	schema := keylessSchema()
	mgr, _ := newHandlerTestManager(t)
	handler := handleConfigRename(mgr, schema)
	called := false
	op := func([]string, string, string, []string) error { called = true; return nil }

	for _, path := range [][]string{
		{"policy", "rule", "1"},
		{"policy", "rule", "1", "match", "m1"},
	} {
		req := postConfigRequest(t, "/config/rename/"+strings.Join(path, "/")+"/", url.Values{"new-key": {"2"}}, "alice")
		req.Header.Set("HX-Request", "true")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "%v", path)
		assert.Contains(t, rec.Body.String(), "rename refused: rule has no key, so its entries cannot be renamed or copied.", "%v", path)

		args := append(append([]string{}, path...), "to", "2")
		_, out := execListEntryOp(schema, args, nil, listEntryRename, op)
		assert.Equal(t, "error: rename refused: rule has no key, so its entries cannot be renamed or copied.", out, "%v", path)
		_, out = execListEntryOp(schema, args, nil, listEntryCopy, op)
		assert.Equal(t, "error: copy refused: rule has no key, so its entries cannot be renamed or copied.", out, "%v", path)
	}
	assert.False(t, called, "a refused rename or copy never reaches the editor")
}

// TestWebAddKeepsTheCaseOfTheFirstKeyValue is AC-20 of
// spec-bfd-link-local-peer-zone, with the multi-leaf address the add form
// builds from its posted key leaves.
//
// GOAL: the web add form stores an entry under the key the operator typed.
// METHOD: POST /config/add/ for a single-key list with `name` `Eth0` and its
// required fields, then
// for single-hop-session with `name` `fe80::b` and `field:interface` `zbl1`.
//
// VALIDATES: the peer entry is stored under `Eth0`, never `eth0`; the session
// is stored under `fe80::b interface zbl1` and holds `interface zbl1`.
// PREVENTS: HandleConfigAddWithAuthorizer lowercasing the first key value,
// and a multi-leaf entry being created under its first value alone.
func TestWebAddKeepsTheCaseOfTheFirstKeyValue(t *testing.T) {
	mgr, schema := newHandlerTestManager(t)
	renderer, err := NewRenderer()
	require.NoError(t, err)
	handler := handleConfigAdd(mgr, schema, renderer)
	add := func(urlPath string, form url.Values) *httptest.ResponseRecorder {
		req := postConfigRequest(t, urlPath, form, "alice")
		req.Header.Set("HX-Request", "true")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	rec := add("/config/add/bgp/peer/", url.Values{
		"name":                       {"Eth0"},
		"field:connection/remote/ip": {"10.0.0.7"},
		"field:session/asn/local":    {"65000"},
		"field:session/asn/remote":   {"65001"},
	})
	require.Less(t, rec.Code, http.StatusBadRequest, rec.Body.String())
	require.NotNil(t, mgr.Tree("alice"), "the add reached the editor: %s", rec.Body.String())
	peers := mgr.Tree("alice").GetContainer("bgp").GetList("peer")
	assert.NotNil(t, peers["Eth0"], "the first key value keeps its case: %v", peers)
	assert.Nil(t, peers["eth0"])

	rec = add("/config/add/bfd/single-hop-session/", url.Values{"name": {"fe80::b"}, "field:interface": {"zbl1"}})
	require.Less(t, rec.Code, http.StatusBadRequest, rec.Body.String())
	assert.Equal(t, showPathPrefix+"bfd/single-hop-session/", rec.Header().Get("HX-Redirect"),
		"a list with no table to swap reloads its list view")
	sessions := sessionList(t, mgr.Tree("alice"))
	entry := sessions["fe80::b interface zbl1"]
	require.NotNil(t, entry, "the posted key leaf completes the address: %v", sessions)
	link, _ := entry.Get("interface")
	assert.Equal(t, "zbl1", link)
	assert.Nil(t, sessions["fe80::b"], "never created under the first value alone")
}
