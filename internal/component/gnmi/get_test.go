package gnmi

import (
	"context"
	"testing"

	gpb "github.com/openconfig/gnmi/proto/gnmi"

	zeconfig "github.com/ze-software/ze/internal/component/config"
)

func TestGetLeafValue(t *testing.T) {
	tree := zeconfig.NewTree()
	bgp := zeconfig.NewTree()
	bgp.Set("router-id", "1.2.3.4")
	tree.SetContainer("bgp", bgp)

	srv := NewServer(Config{}, treeFunc(tree), nil, nil, nil)

	resp, err := srv.Get(context.Background(), &gpb.GetRequest{
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

	_, err := srv.Get(context.Background(), &gpb.GetRequest{})
	if err == nil {
		t.Fatal("expected error for empty path list")
	}
}
