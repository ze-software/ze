package rtproto

import "testing"

func TestRouteProtocolsAreDistinct(t *testing.T) {
	seen := map[int]string{}
	protocols := []struct {
		protocol int
		name     string
	}{
		{FIBKernel, "fib-kernel"},
		{Static, "static"},
		{PolicyRoute, "policy-route"},
	}
	for _, p := range protocols {
		protocol, name := p.protocol, p.name
		if prev, ok := seen[protocol]; ok {
			t.Fatalf("protocol %d used by both %s and %s", protocol, prev, name)
		}
		seen[protocol] = name
	}
}

func TestIsZe(t *testing.T) {
	for protocol := range zeNames {
		if !IsZe(protocol) {
			t.Fatalf("IsZe(%d) = false, want true", protocol)
		}
	}
	if IsZe(4) {
		t.Fatal("IsZe(RTPROT_STATIC=4) = true, want false")
	}
}
