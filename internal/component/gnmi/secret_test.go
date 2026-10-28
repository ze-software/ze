//go:build ze_gnmi

package gnmi

import (
	"strings"
	"testing"
	"time"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/ze-software/ze/internal/component/api"
	_ "github.com/ze-software/ze/internal/component/bgp/yang"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/secret"
	_ "github.com/ze-software/ze/internal/component/gnmi/yang"
)

// The keys the secret tests configure. Each tail is distinctive, so a reply
// that carries a key in clear is found by a substring search.
const (
	md5KeyTail   = "md5-9b41e7-tail"
	tokenKeyTail = "token-5c08d2-tail"
)

// secretTree parses a config that holds a BGP TCP MD5 key in its $9$ form
// and a gNMI token in clear, both ze:sensitive leaves, the way the daemon
// loads its running tree: the tree then holds both keys decoded. It answers
// the tree and the $9$ text the file carried for the MD5 key.
func secretTree(t *testing.T) (*zeconfig.Tree, string) {
	t.Helper()
	md5Stored, err := secret.Encode("key-" + md5KeyTail)
	if err != nil {
		t.Fatalf("encode md5 key: %v", err)
	}
	schema, err := zeconfig.YANGSchema()
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	text := "bgp {\n peer peer1 {\n  connection {\n   md5 {\n    password \"" + md5Stored +
		"\"\n   }\n  }\n }\n}\nenvironment {\n gnmi {\n  token key-" + tokenKeyTail + "\n }\n}\n"
	tree, err := zeconfig.NewParser(schema).Parse(text)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got, _ := tree.GetContainer("environment").GetContainer("gnmi").Get("token"); got != "key-"+tokenKeyTail {
		t.Fatalf("fixture: the tree holds token %q, not the decoded key, so this case proves nothing", got)
	}
	return tree, md5Stored
}

// assertSecretSpelling fails when got is not a $9$ text that decodes to key.
func assertSecretSpelling(t *testing.T, what, got, key string) {
	t.Helper()
	if !secret.IsEncoded(got) {
		t.Fatalf("%s: a gNMI client received %q, not the $9$ form", what, got)
	}
	decoded, err := secret.Decode(got)
	if err != nil {
		t.Fatalf("%s: %q does not decode: %v", what, got, err)
	}
	if decoded != key {
		t.Fatalf("%s: %q decodes to %q, want %q", what, got, decoded, key)
	}
}

// assertNoClearKey fails when payload carries either configured key in clear.
func assertNoClearKey(t *testing.T, what, payload string) {
	t.Helper()
	for _, tail := range []string{md5KeyTail, tokenKeyTail} {
		if strings.Contains(payload, tail) {
			t.Fatalf("%s: a gNMI client received a key in clear: %s", what, payload)
		}
	}
	if !strings.Contains(payload, "$9$") {
		t.Fatalf("%s: the reply carries no $9$ form, so it lost the key: %s", what, payload)
	}
}

// TestGetAnswersSecretsEncoded drives the Get handler over the running tree,
// which holds every ze:sensitive key decoded.
//
// VALIDATES: a leaf Get answers the $9$ text the config file carried, byte for
// byte; a key given in clear is answered in a $9$ form that decodes to it, the
// same text on a second Get; a container Get carries no key in clear.
// PREVENTS: a gNMI client reading a BGP MD5 key or the gNMI token in clear.
func TestGetAnswersSecretsEncoded(t *testing.T) {
	tree, md5Stored := secretTree(t)
	srv := NewServer(Config{}, treeFunc(tree), nil, nil, nil)
	md5Path := &gpb.Path{Elem: []*gpb.PathElem{
		{Name: "bgp"},
		{Name: "peer", Key: map[string]string{"name": "peer1"}},
		{Name: "connection"}, {Name: "md5"}, {Name: "password"},
	}}
	tokenPath := &gpb.Path{Elem: []*gpb.PathElem{
		{Name: "environment"}, {Name: "gnmi"}, {Name: "token"},
	}}

	get := func(path *gpb.Path) *gpb.TypedValue {
		t.Helper()
		resp, err := srv.Get(t.Context(), &gpb.GetRequest{Encoding: gpb.Encoding_JSON_IETF, Path: []*gpb.Path{path}})
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		return resp.GetNotification()[0].GetUpdate()[0].GetVal()
	}

	if got := get(md5Path).GetStringVal(); got != md5Stored {
		t.Fatalf("md5 password: got %q, want the stored text %q", got, md5Stored)
	}
	first := get(tokenPath).GetStringVal()
	assertSecretSpelling(t, "token", first, "key-"+tokenKeyTail)
	if second := get(tokenPath).GetStringVal(); second != first {
		t.Fatalf("token: a second Get answered %q, the first %q", second, first)
	}
	whole := get(&gpb.Path{Elem: []*gpb.PathElem{{Name: "bgp"}}})
	assertNoClearKey(t, "bgp container", string(whole.GetJsonIetfVal()))
	env := get(&gpb.Path{Elem: []*gpb.PathElem{{Name: "environment"}}})
	assertNoClearKey(t, "environment container", string(env.GetJsonIetfVal()))
}

// subscribeOnce opens a ONCE subscription over a real gRPC connection and
// answers the payload of every update before the sync response.
func subscribeOnce(t *testing.T, srv *Server, subs []*gpb.Subscription) []string {
	t.Helper()
	conn, err := grpc.NewClient(srv.Address(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck // test cleanup
	stream, err := gpb.NewGNMIClient(conn).Subscribe(t.Context())
	if err != nil {
		t.Fatalf("Subscribe RPC: %v", err)
	}
	err = stream.Send(&gpb.SubscribeRequest{Request: &gpb.SubscribeRequest_Subscribe{
		Subscribe: &gpb.SubscriptionList{Encoding: gpb.Encoding_JSON_IETF, Mode: gpb.SubscriptionList_ONCE, Subscription: subs},
	}})
	if err != nil {
		t.Fatalf("send subscribe: %v", err)
	}
	var payloads []string
	for {
		resp, err := stream.Recv()
		if err != nil {
			t.Fatalf("recv: %v", err)
		}
		if resp.GetSyncResponse() {
			return payloads
		}
		for _, upd := range resp.GetUpdate().GetUpdate() {
			payloads = append(payloads, string(upd.GetVal().GetJsonIetfVal())+upd.GetVal().GetStringVal())
		}
	}
}

// TestSubscribeOnceAnswersSecretsEncoded drives Subscribe ONCE, with no path
// (the whole tree) and with a path to a sensitive leaf.
//
// VALIDATES: neither reply carries a key in clear; the leaf reply is the $9$
// text the config file carried.
// PREVENTS: a subscriber reading the keys Get no longer shows.
func TestSubscribeOnceAnswersSecretsEncoded(t *testing.T) {
	tree, md5Stored := secretTree(t)
	srv := NewServer(Config{ListenAddr: "127.0.0.1:0"}, treeFunc(tree), nil, nil, nil)
	startTestServer(t, srv)

	whole := subscribeOnce(t, srv, nil)
	if len(whole) != 1 {
		t.Fatalf("whole tree: got %d updates, want 1", len(whole))
	}
	assertNoClearKey(t, "whole tree", whole[0])

	leaf := subscribeOnce(t, srv, []*gpb.Subscription{{Path: &gpb.Path{Elem: []*gpb.PathElem{
		{Name: "bgp"},
		{Name: "peer", Key: map[string]string{"name": "peer1"}},
		{Name: "connection"}, {Name: "md5"}, {Name: "password"},
	}}}})
	if len(leaf) != 1 {
		t.Fatalf("md5 leaf: got %d updates, want 1", len(leaf))
	}
	if leaf[0] != md5Stored {
		t.Fatalf("md5 leaf: got %q, want the stored text %q", leaf[0], md5Stored)
	}
}

// TestSetEchoesSecretsEncoded subscribes in STREAM mode, then sets the gNMI
// token in clear through Set: the server tells every subscriber what a Set
// changed.
//
// VALIDATES: the change notification carries the token in a $9$ form that
// decodes to the key set, never the key in clear.
// PREVENTS: one client's Set publishing a credential to every other client.
func TestSetEchoesSecretsEncoded(t *testing.T) {
	editor := &configSessionEditor{values: make(map[string]string)}
	sessions := api.NewConfigSessionManager(func() (api.ConfigEditor, error) {
		return editor, nil
	})
	notifier := NewChangeNotifier()
	srv := NewServer(Config{}, nil, sessions, nil, notifier)
	ch := notifier.Subscribe()
	t.Cleanup(func() { notifier.Unsubscribe(ch) })

	_, err := srv.Set(t.Context(), &gpb.SetRequest{Update: []*gpb.Update{{
		Path: &gpb.Path{Elem: []*gpb.PathElem{{Name: "environment"}, {Name: "gnmi"}, {Name: "token"}}},
		Val:  &gpb.TypedValue{Value: &gpb.TypedValue_StringVal{StringVal: "key-" + tokenKeyTail}},
	}}})
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	select {
	case n := <-ch:
		assertSecretSpelling(t, "set notification", n.GetUpdate()[0].GetVal().GetStringVal(), "key-"+tokenKeyTail)
	case <-time.After(5 * time.Second):
		t.Fatal("no change notification after Set")
	}
}
