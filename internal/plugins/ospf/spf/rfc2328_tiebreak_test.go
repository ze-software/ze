// VALIDATES: RFC 2328 Section 16.1 candidate ordering -- among vertices equally close
// to the root a network vertex leaves the candidate list before a router vertex, so
// every equal-cost path through a transit network is found.
// PREVENTS: a router vertex settling first at a tie and hiding the equal-cost path
// that runs through the network vertex at the same distance.
package spf

import "testing"

// RFC requirement: RFC2328-16.1-3 positive -- with a router vertex and a network vertex at the same distance from the root, the candidate list yields the network vertex first (spfHeap.less orders VertexNetwork before VertexRouter at equal dist, spf.go).
func TestRFC2328EqualDistanceNetworkVertexFirst(t *testing.T) {
	h := &spfHeap{}
	h.push(heapItem{id: routerVertex(testRID(t, "1.1.1.1")), dist: 10})
	h.push(heapItem{id: networkVertex(testLSID(t, "10.0.0.1")), dist: 10})
	h.push(heapItem{id: routerVertex(testRID(t, "0.0.0.1")), dist: 10})
	first := h.pop()
	if first.id.Kind != VertexNetwork || first.id.Network != testLSID(t, "10.0.0.1") {
		t.Fatalf("first candidate at a tie = %+v, want the network vertex 10.0.0.1", first.id)
	}
	if second := h.pop(); second.id.Kind != VertexRouter {
		t.Fatalf("second candidate = %+v, want a router vertex", second.id)
	}
}

// RFC requirement: RFC2328-16.1-3 negative -- the network-before-router preference applies only at equal distance: a router vertex closer to the root leaves the candidate list before a farther network vertex (spfHeap.less compares dist before Kind, spf.go).
func TestRFC2328CloserRouterVertexBeforeFartherNetwork(t *testing.T) {
	h := &spfHeap{}
	h.push(heapItem{id: networkVertex(testLSID(t, "10.0.0.1")), dist: 11})
	h.push(heapItem{id: routerVertex(testRID(t, "1.1.1.1")), dist: 10})
	first := h.pop()
	if first.id.Kind != VertexRouter || first.dist != 10 {
		t.Fatalf("first candidate = %+v dist %d, want the router vertex at distance 10", first.id, first.dist)
	}
	if second := h.pop(); second.id.Kind != VertexNetwork || second.dist != 11 {
		t.Fatalf("second candidate = %+v dist %d, want the network vertex at distance 11", second.id, second.dist)
	}
}
