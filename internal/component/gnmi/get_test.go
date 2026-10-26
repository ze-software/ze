package gnmi

import (
	"context"
	"strings"
	"testing"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	zeconfig "github.com/ze-software/ze/internal/component/config"
)

func TestGetLeafValue(t *testing.T) {
	tree := zeconfig.NewTree()
	bgp := zeconfig.NewTree()
	bgp.Set("router-id", "1.2.3.4")
	tree.SetContainer("bgp", bgp)

	srv := NewServer(Config{}, treeFunc(tree), nil, nil, nil)

	resp, err := srv.Get(context.Background(), &gpb.GetRequest{
		Encoding: gpb.Encoding_JSON_IETF,
		Path: []*gpb.Path{{
			Elem: []*gpb.PathElem{
				{Name: "bgp"},
				{Name: "router-id"},
			},
		}},
	})
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if len(resp.Notification) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(resp.Notification))
	}
	upd := resp.Notification[0].Update
	if len(upd) != 1 {
		t.Fatalf("expected 1 update, got %d", len(upd))
	}
	sv := upd[0].Val.GetStringVal()
	if sv != "1.2.3.4" {
		t.Errorf("expected 1.2.3.4, got %q", sv)
	}
}

func TestGetContainer(t *testing.T) {
	tree := zeconfig.NewTree()
	bgp := zeconfig.NewTree()
	bgp.Set("router-id", "1.2.3.4")
	bgp.Set("as-number", "65000")
	tree.SetContainer("bgp", bgp)

	srv := NewServer(Config{}, treeFunc(tree), nil, nil, nil)

	resp, err := srv.Get(context.Background(), &gpb.GetRequest{
		Encoding: gpb.Encoding_JSON_IETF,
		Path: []*gpb.Path{{
			Elem: []*gpb.PathElem{{Name: "bgp"}},
		}},
	})
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if len(resp.Notification) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(resp.Notification))
	}
	val := resp.Notification[0].Update[0].Val
	if val.GetJsonIetfVal() == nil {
		t.Error("expected JSON_IETF value for container")
	}
}

func TestGetNotFound(t *testing.T) {
	tree := zeconfig.NewTree()
	srv := NewServer(Config{}, treeFunc(tree), nil, nil, nil)

	_, err := srv.Get(context.Background(), &gpb.GetRequest{
		Encoding: gpb.Encoding_JSON_IETF,
		Path: []*gpb.Path{{
			Elem: []*gpb.PathElem{
				{Name: "nonexistent"},
				{Name: "path"},
			},
		}},
	})
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}

func TestGetEmptyPath(t *testing.T) {
	srv := NewServer(Config{}, treeFunc(zeconfig.NewTree()), nil, nil, nil)

	_, err := srv.Get(context.Background(), &gpb.GetRequest{Encoding: gpb.Encoding_JSON_IETF})
	if err == nil {
		t.Fatal("expected error for empty path list")
	}
}

// TestGetRefusesUnadvertisedEncoding checks that Get answers an encoding
// Capabilities does not advertise with Unimplemented, naming the encoding and
// the two it can use instead.
// gNMI specification Section 3.3.1: "If the target does not support the
// specified encoding, the target MUST return an error of `Unimplemented`."
// Method: one request per Encoding value Ze cannot serialize.
func TestGetRefusesUnadvertisedEncoding(t *testing.T) {
	tree := zeconfig.NewTree()
	bgp := zeconfig.NewTree()
	bgp.Set("router-id", "1.2.3.4")
	tree.SetContainer("bgp", bgp)
	srv := NewServer(Config{}, treeFunc(tree), nil, nil, nil)
	path := []*gpb.Path{{Elem: []*gpb.PathElem{{Name: "bgp"}, {Name: "router-id"}}}}

	for _, encoding := range []gpb.Encoding{gpb.Encoding_BYTES, gpb.Encoding_PROTO, gpb.Encoding_ASCII, gpb.Encoding(99)} {
		_, err := srv.Get(context.Background(), &gpb.GetRequest{Path: path, Encoding: encoding})
		if status.Code(err) != codes.Unimplemented {
			t.Fatalf("encoding %s: code = %v (err %v), want Unimplemented", encoding, status.Code(err), err)
		}
		if !strings.Contains(err.Error(), "encoding "+encoding.String()+" is not supported") {
			t.Errorf("encoding %s: error %q does not name the encoding", encoding, err)
		}
		if !strings.Contains(err.Error(), "--encoding json_ietf") {
			t.Errorf("encoding %s: error %q does not name the fix", encoding, err)
		}
	}
}

// TestGetAnswersEachJSONEncoding checks that Get answers a container in the
// JSON member the request asked for, JSON when the request leaves the field
// unset, over the same serialization.
// gNMI specification Section 3.3.1: "If the encoding is not specified, JSON MUST
// be used."
// Method: one Get per encoding over the bgp container; JSON and an unset field
// must carry JsonVal and no JsonIetfVal, JSON_IETF the reverse, all equal bytes.
func TestGetAnswersEachJSONEncoding(t *testing.T) {
	tree := zeconfig.NewTree()
	bgp := zeconfig.NewTree()
	bgp.Set("router-id", "1.2.3.4")
	tree.SetContainer("bgp", bgp)
	srv := NewServer(Config{}, treeFunc(tree), nil, nil, nil)
	path := []*gpb.Path{{Elem: []*gpb.PathElem{{Name: "bgp"}}}}

	answer := func(request *gpb.GetRequest) *gpb.TypedValue {
		t.Helper()
		resp, err := srv.Get(context.Background(), request)
		if err != nil {
			t.Fatalf("encoding %s: %v", request.GetEncoding(), err)
		}
		return resp.GetNotification()[0].GetUpdate()[0].GetVal()
	}

	ietf := answer(&gpb.GetRequest{Path: path, Encoding: gpb.Encoding_JSON_IETF})
	if ietf.GetJsonIetfVal() == nil || ietf.GetJsonVal() != nil {
		t.Fatalf("JSON_IETF answered %v, want a JsonIetfVal", ietf)
	}
	for name, request := range map[string]*gpb.GetRequest{
		"JSON":  {Path: path, Encoding: gpb.Encoding_JSON},
		"unset": {Path: path},
	} {
		val := answer(request)
		if val.GetJsonVal() == nil || val.GetJsonIetfVal() != nil {
			t.Fatalf("%s answered %v, want a JsonVal", name, val)
		}
		if string(val.GetJsonVal()) != string(ietf.GetJsonIetfVal()) {
			t.Errorf("%s answered %s, JSON_IETF %s: the serialization differs", name, val.GetJsonVal(), ietf.GetJsonIetfVal())
		}
	}
}
