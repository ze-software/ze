package topology_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
	_ "github.com/ze-software/ze/internal/le/interoplab/bgp"
	_ "github.com/ze-software/ze/internal/le/interoplab/ipsec"
	_ "github.com/ze-software/ze/internal/le/interoplab/l2tp"
	_ "github.com/ze-software/ze/internal/le/interoplab/pppoe"
	_ "github.com/ze-software/ze/internal/le/interoplab/radius"
	_ "github.com/ze-software/ze/internal/le/interoplab/radiussubscriber"
	_ "github.com/ze-software/ze/internal/le/interoplab/rsvpte"
	_ "github.com/ze-software/ze/internal/le/test/deployment"
)

// buildShape is an ImageBuild less its Name: what Docker builds or pulls.
// The registry renamed every image to a name unique across suites, so the
// name changed on purpose; everything Docker is asked to do must not.
type buildShape struct {
	Tag, Dockerfile, Context string
	Required, Pull           bool
}

func shapes(builds []interoplab.ImageBuild) []buildShape {
	out := make([]buildShape, 0, len(builds))
	for _, build := range builds {
		out = append(out, buildShape{Tag: build.Tag, Dockerfile: build.Dockerfile, Context: build.Context,
			Required: build.Required, Pull: build.Pull})
	}
	return out
}

// VALIDATES: AC-11. Each suite's registry-derived image list builds exactly
// what the list it declared by hand before the registry did: the same tag,
// Dockerfile, context, requirement and pull for every image, in the same
// order. The conditional images (FRR from the environment, the L2TP and
// IPsec images only some scenarios need) are selected inside each suite and
// covered by that suite's own tests.
// PREVENTS: a registration that builds from the wrong directory, or drops an
// image a suite ran before the move.
// METHOD: every suite package is linked, so every registration ran; each
// owner's SuiteImages under a fixed checkout is compared with the literal
// list each suite's builder held before the change, plus the images added
// since (ze-ipsec-dns, ze-topology-webftp, ze-topology-wireguard), in
// registration order: a package's init functions run in file-name order.
func TestSuiteImageListsUnchanged(t *testing.T) {
	root := "/checkout"
	join := func(parts ...string) string { return filepath.Join(append([]string{root}, parts...)...) }
	interop := join("test", "interop")
	l2tp := join("test", "interop-l2tp")
	pppoe := join("test", "interop-pppoe")
	ipsec := join("test", "interop-ipsec")
	rsvpte := join("test", "interop-rsvpte")
	subscriber := join("test", "interop-radius-subscriber")
	for _, tc := range []struct {
		owner string
		want  []buildShape
	}{
		{"interop", []buildShape{
			{"ze-interop", filepath.Join(interop, "Dockerfile.ze"), root, true, false},
			{"bird-interop", filepath.Join(interop, "Dockerfile.bird"), interop, true, false},
			{"gobgp-interop", filepath.Join(interop, "Dockerfile.gobgp"), interop, false, false},
			{"keepalived-interop", filepath.Join(interop, "Dockerfile.keepalived"), interop, true, false},
			{"dnsmasq-interop", filepath.Join(interop, "Dockerfile.dnsmasq"), interop, false, false},
			{"kea-interop", filepath.Join(interop, "Dockerfile.kea"), interop, false, false},
			{"knot-interop", filepath.Join(interop, "Dockerfile.knot"), interop, false, false},
			{"unbound-interop", filepath.Join(interop, "Dockerfile.unbound"), interop, false, false},
			{"stayrtr-interop", filepath.Join(interop, "Dockerfile.stayrtr"), interop, false, false},
			{"pmacct/pmbmpd:v1.8.0", "", "", false, true},
		}},
		{"interop-l2tp", []buildShape{
			{"ze-l2tp-interop", filepath.Join(l2tp, "Dockerfile.ze"), root, true, false},
			{"ze-l2tp-lac", filepath.Join(l2tp, "Dockerfile.lac"), l2tp, true, false},
			{"ze-l2tp-radius", join("internal", "le", "interoplab", "l2tp", "radiusmock", "Dockerfile"), root, true, false},
			{"ze-l2tp-avp-relay", join("internal", "le", "interoplab", "l2tp", "avprelay", "Dockerfile"), root, true, false},
			{"ze-l2tp-knot", filepath.Join(interop, "Dockerfile.knot"), interop, true, false},
		}},
		{"interop-radius", []buildShape{
			{"ze-radius-interop", join("test", "interop-radius", "Dockerfile.ze"), root, true, false},
		}},
		{"interop-radius-subscriber", []buildShape{
			{"ze-radius-subscriber-ze", filepath.Join(subscriber, "Dockerfile.ze"), root, true, false},
			{"ze-radius-subscriber-lac", filepath.Join(subscriber, "Dockerfile.lac"), subscriber, true, false},
		}},
		{"interoplab", []buildShape{
			{interoplab.FreeRADIUSImage, "", "", true, true},
		}},
		{"interop-pppoe", []buildShape{
			{"ze-pppoe-interop", filepath.Join(pppoe, "Dockerfile.ze"), root, true, false},
			{"ze-pppoe-accel", filepath.Join(pppoe, "Dockerfile.accel"), pppoe, true, false},
			{"ze-pppoe-client", filepath.Join(pppoe, "Dockerfile.client"), pppoe, true, false},
		}},
		{"interop-ipsec", []buildShape{
			{"ze-ipsec-dns", filepath.Join(ipsec, "Dockerfile.dns"), ipsec, true, false},
			{"ze-ipsec-interop", filepath.Join(ipsec, "Dockerfile.ze"), root, true, false},
			{"ze-ipsec-strongswan", filepath.Join(ipsec, "Dockerfile.strongswan"), ipsec, true, false},
			{"ze-ipsec-nat", filepath.Join(ipsec, "Dockerfile.nat"), ipsec, true, false},
		}},
		{"interop-rsvpte", []buildShape{
			{"ze-rsvpte-interop", filepath.Join(rsvpte, "Dockerfile.ze"), root, true, false},
			{"ze-rsvpte-freertr", filepath.Join(rsvpte, "Dockerfile.freertr"), rsvpte, true, false},
		}},
		{"interop-topology", []buildShape{
			{"ze-topology-webftp", join("test", "interop-topology", "home-lab", "Dockerfile.webftp"), join("test", "interop-topology", "home-lab"), true, false},
			{"ze-topology-wireguard", join("internal", "le", "test", "deployment", "testdata", "Dockerfile.wireguard"), join("internal", "le", "test", "deployment", "testdata"), true, false},
			{"ze-topology-freeradius", join("test", "interop-topology", "access-chain", "Dockerfile.freeradius"), join("test", "interop-topology", "access-chain"), true, false},
		}},
		{"docker-wireguard", []buildShape{
			{"ze-vpn-wireguard", join("internal", "le", "test", "deployment", "testdata", "Dockerfile.wireguard"), root, true, false},
		}},
	} {
		got := interoplab.SuiteImages(tc.owner, interoplab.ImageContext{Root: root})
		if !reflect.DeepEqual(shapes(got), tc.want) {
			t.Errorf("%s images:\n got  %+v\n want %+v", tc.owner, shapes(got), tc.want)
		}
	}
}
