// VALIDATES: small pure helpers in the OSPF root package -- interface_addr.go
// maskFromPrefixLength (the IPv4 netmask for a prefix length), config.go ospfConfig.Present
// (the parsed-presence flag), and gr.go grStorageKey / grFactKey / interfaceAreaType (the
// per-engine graceful-restart NVS key composition and the RFC 3623 §3.2 area-type lookup).
// PREVENTS: an off-by-one netmask (notably /0 and /32); Present reporting the wrong flag;
// a GR fact key that collides across instances or drops its family/instance suffix; an
// area-type lookup that ignores the configured area type.
package ospf

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestMaskFromPrefixLength(t *testing.T) {
	cases := []struct {
		prefix int
		want   [4]byte
	}{
		{0, [4]byte{0, 0, 0, 0}},
		{1, [4]byte{0x80, 0, 0, 0}},
		{8, [4]byte{255, 0, 0, 0}},
		{24, [4]byte{255, 255, 255, 0}},
		{25, [4]byte{255, 255, 255, 0x80}},
		{31, [4]byte{255, 255, 255, 0xfe}},
		{32, [4]byte{255, 255, 255, 255}},
	}
	for _, c := range cases {
		if got := maskFromPrefixLength(c.prefix); got != c.want {
			t.Errorf("maskFromPrefixLength(%d) = %v, want %v", c.prefix, got, c.want)
		}
	}
}

func TestOSPFConfigPresent(t *testing.T) {
	if (ospfConfig{}).Present() {
		t.Fatalf("a zero ospfConfig must report Present()=false")
	}
	if !(ospfConfig{present: true}).Present() {
		t.Fatalf("a parsed ospfConfig must report Present()=true")
	}
}

func TestGRStorageAndFactKey(t *testing.T) {
	// A nil dispatch resolves to the IPv4 family: the key is "v4-<instance-id>", and two
	// different OSPFv2 instances get distinct keys so they cannot clobber each other's fact.
	e0 := &engine{cfg: ospfConfig{InstanceID: 0}}
	e7 := &engine{cfg: ospfConfig{InstanceID: 7}}
	if got := e0.grStorageKey(); got != "v4-0" {
		t.Fatalf("grStorageKey(instance 0) = %q, want %q", got, "v4-0")
	}
	if got := e7.grStorageKey(); got != "v4-7" {
		t.Fatalf("grStorageKey(instance 7) = %q, want %q", got, "v4-7")
	}
	if got := e7.grFactKey(); got != grRestartFactKeyPrefix+"v4-7" {
		t.Fatalf("grFactKey(instance 7) = %q, want %q", got, grRestartFactKeyPrefix+"v4-7")
	}
}

func TestInterfaceAreaType(t *testing.T) {
	area := types.AreaID{0, 0, 0, 9}
	e := &engine{
		cfg: ospfConfig{Areas: []areaConfig{{AreaID: area, AreaType: areaTypeStub}}},
		running: map[string]interfaceConfig{
			"eth0": {Name: "eth0", AreaID: area},
		},
	}
	if got := e.interfaceAreaType("eth0"); got != "stub" {
		t.Fatalf("interfaceAreaType(eth0 in a stub area) = %q, want %q", got, "stub")
	}
	// An interface the engine is not running defaults to a normal area (RFC 3623 §3.2: the
	// strict-LSA stub exception only applies to a known stub/NSSA area).
	if got := e.interfaceAreaType("eth9"); got != "normal" {
		t.Fatalf("interfaceAreaType(unknown iface) = %q, want %q", got, "normal")
	}
}
