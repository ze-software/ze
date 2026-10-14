package gnmi

import (
	"context"
	"strings"
	"testing"

	gpb "github.com/openconfig/gnmi/proto/gnmi"

	_ "github.com/ze-software/ze/internal/component/bfd/yang" // Register the BFD YANG: single-hop-session is keyed `peer vrf interface`.
	zeconfig "github.com/ze-software/ze/internal/component/config"
)

// sessionPath is bfd/single-hop-session with the given keys, then rest.
func sessionPath(keys map[string]string, rest ...string) *gpb.Path {
	p := &gpb.Path{Elem: []*gpb.PathElem{{Name: "bfd"}, {Name: "single-hop-session", Key: keys}}}
	for _, name := range rest {
		p.Elem = append(p.Elem, &gpb.PathElem{Name: name})
	}
	return p
}

// TestGetSelectsEachOfTwoMultiLeafKeyEntries is AC-15 and AC-15a of
// spec-bfd-link-local-peer-zone.
//
// GOAL: gNMI addresses each entry of a multi-leaf-key list by its named keys.
// METHOD: Get each AC-2 entry by `[peer][interface]`, with and without
// `[vrf=default]`, Get a key leaf, and apply a Set of a key leaf, over the
// registered BFD YANG.
//
// VALIDATES: each Get returns its own entry's profile; `vrf=default` names
// the same entry; the notification path names peer, vrf and interface from
// the schema; a Get of `interface` reads the zbl1 entry; a Set of
// `interface` is refused naming `interface` as given twice.
// PREVENTS: pathToSegments refusing every multi-key path (errMultipleKeys),
// and a Set rewriting a key leaf so the key and the body disagree.
func TestGetSelectsEachOfTwoMultiLeafKeyEntries(t *testing.T) {
	schema, err := zeconfig.YANGSchema()
	if err != nil {
		t.Fatalf("YANGSchema: %v", err)
	}
	tree, err := zeconfig.NewParser(schema).Parse("bfd {\n" +
		"\tsingle-hop-session fe80::b { interface zbl0; profile p0; }\n" +
		"\tsingle-hop-session fe80::b { interface zbl1; profile p1; }\n}\n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	srv := NewServer(Config{}, treeFunc(tree), nil, nil, nil)
	get := func(p *gpb.Path) *gpb.Update {
		t.Helper()
		resp, getErr := srv.Get(context.Background(), &gpb.GetRequest{Path: []*gpb.Path{p}})
		if getErr != nil {
			t.Fatalf("Get(%v): %v", p, getErr)
		}
		return resp.Notification[0].Update[0]
	}

	for _, link := range []string{"zbl0", "zbl1"} {
		want := "p" + link[len(link)-1:]
		for _, keys := range []map[string]string{
			{"peer": "fe80::b", "interface": link},
			{"peer": "fe80::b", "interface": link, "vrf": "default"},
		} {
			upd := get(sessionPath(keys, "profile"))
			if got := upd.Val.GetStringVal(); got != want {
				t.Errorf("Get %v profile = %q, want %q (never the other entry)", keys, got, want)
			}
			named := upd.Path.Elem[1].Key
			wantKeys := map[string]string{"peer": "fe80::b", "vrf": "default", "interface": link}
			if len(named) != len(wantKeys) {
				t.Errorf("notification keys = %v, want %v", named, wantKeys)
			}
			for k, v := range wantKeys {
				if named[k] != v {
					t.Errorf("notification key %s = %q, want %q (keys %v)", k, named[k], v, named)
				}
			}
		}
	}

	upd := get(sessionPath(map[string]string{"peer": "fe80::b", "interface": "zbl1"}, "interface"))
	if got := upd.Val.GetStringVal(); got != "zbl1" {
		t.Errorf("Get of the key leaf = %q, want zbl1, read from the zbl1 entry", got)
	}

	setErr := srv.applyUpdate(schema, "", &gpb.Update{
		Path: sessionPath(map[string]string{"peer": "fe80::b", "interface": "zbl1"}, "interface"),
		Val:  &gpb.TypedValue{Value: &gpb.TypedValue_StringVal{StringVal: "zbl2"}},
	})
	if setErr == nil || !strings.Contains(setErr.Error(), "interface is given twice") {
		t.Errorf("Set of the key leaf = %v, want a refusal naming interface as given twice", setErr)
	}
	sessions := tree.GetContainer("bfd").GetList("single-hop-session")
	if len(sessions) != 2 || sessions["fe80::b interface zbl0"] == nil || sessions["fe80::b interface zbl1"] == nil {
		t.Errorf("both entries must be unchanged, got %v", sessions)
	}

	if _, pathErr := pathToSegments(schema, sessionPath(map[string]string{"interface": "zbl1"})); pathErr == nil {
		t.Error("a path naming no peer must be refused: every address carries the first key leaf")
	}
}
