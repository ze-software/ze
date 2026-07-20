package spf

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

func TestOSPFGraphBuild(t *testing.T) {
	area := testArea()
	db := testSource(t, area,
		routerLSA(t, "1.1.1.1", transitLink(t, "10.0.0.1"), stubLink(t, "198.51.100.0", 7)),
		routerLSA(t, "2.2.2.2", transitLink(t, "10.0.0.2")),
		networkLSA(t, "10.0.0.254", "2.2.2.2", "255.255.255.0", "1.1.1.1", "2.2.2.2"),
	)

	g := BuildGraph(db, area)
	if len(g.Routers) != 2 {
		t.Fatalf("routers = %d, want 2", len(g.Routers))
	}
	if len(g.Networks) != 1 {
		t.Fatalf("networks = %d, want 1", len(g.Networks))
	}
	root := g.Routers[testRID(t, "1.1.1.1")]
	if root == nil || len(root.Links) != 2 {
		t.Fatalf("root links = %+v, want transit and stub", root)
	}
	net := g.Networks[testLSID(t, "10.0.0.254")]
	if net == nil {
		t.Fatalf("Network-LSA keyed by DR interface address not found")
	}
	if net.ID != testLSID(t, "10.0.0.254") || net.NetworkMask != testIP(t, "255.255.255.0") {
		t.Fatalf("network vertex = %+v", net)
	}
	if len(net.AttachedRouters) != 2 || net.AttachedRouters[0] != testRID(t, "1.1.1.1") || net.AttachedRouters[1] != testRID(t, "2.2.2.2") {
		t.Fatalf("attached routers = %+v", net.AttachedRouters)
	}
}

// RFC requirement: RFC2328-14-1 negative -- a MaxAge LSA is excluded from the routing-table calculation: it never becomes a vertex, so no path can be built through it (BuildGraph age filter, graph.go:91-94).
func TestOSPFGraphSkipsMaxAge(t *testing.T) {
	area := testArea()
	lsa := routerLSA(t, "1.1.1.1")
	lsa.Header.Age = types.LSAge(types.MaxAge)
	db := testSource(t, area, lsa)
	if g := BuildGraph(db, area); len(g.Routers) != 0 {
		t.Fatalf("MaxAge router in graph: %+v", g.Routers)
	}
}
