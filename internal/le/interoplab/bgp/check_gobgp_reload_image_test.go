// Design: docs/architecture/testing/interop.md -- native reload provenance before observations.
package bgp

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// TestGoBGPReloadRetainsConsumedImages requires all five live peer identities
// before any version/config observation, and retains an image failure instead
// of proceeding with versions or substituting declared tags.
func TestGoBGPReloadRetainsConsumedImages(t *testing.T) {
	for _, fail := range []bool{false, true} {
		lab := &gobgpReloadImageLab{fail: fail, observed: errors.New("stop after image observations")}
		proof := gobgpReloadProof{check: &interoplab.CheckContext{Lab: lab}, evidence: make(map[string]string)}
		err := proof.identify(t.Context())
		if fail {
			if !errors.Is(err, lab.observed) {
				t.Fatalf("image inspection failure was lost: %v", err)
			}
			if !slices.Equal(lab.calls, []string{"image:ze"}) {
				t.Fatalf("image refusal proceeded to observations: %v", lab.calls)
			}
			if proof.evidence["image-ze-error"] == "" {
				t.Fatal("image inspection failure was not retained")
			}
			continue
		}
		if !errors.Is(err, lab.observed) {
			t.Fatalf("did not reach the independent version boundary: %v", err)
		}
		want := []string{"image:ze", "image:" + peerGoBGP, "image:" + peerFRR, "image:" + peerFRRSink, "image:" + peerBIRD, "query"}
		if !slices.Equal(lab.calls, want) {
			t.Fatalf("identity/observation order=%v, want %v", lab.calls, want)
		}
		for index, peer := range []string{"ze", peerGoBGP, peerFRR, peerFRRSink, peerBIRD} {
			want := "sha256:" + strings.Repeat(string(rune('a'+index)), 64)
			if proof.evidence["image-"+peer] != want {
				t.Fatalf("%s actual identity missing or confused: %q", peer, proof.evidence["image-"+peer])
			}
		}
	}
}

type gobgpReloadImageLab struct {
	interoplab.CheckerLab
	calls    []string
	fail     bool
	observed error
}

func (lab *gobgpReloadImageLab) PeerImage(_ context.Context, peer string) (string, error) {
	lab.calls = append(lab.calls, "image:"+peer)
	if lab.fail {
		return "", lab.observed
	}
	return "sha256:" + strings.Repeat(string(rune('a'+len(lab.calls)-1)), 64), nil
}

func (lab *gobgpReloadImageLab) Query(context.Context, string, []string, []interoplab.EnvironmentVariable) (string, error) {
	lab.calls = append(lab.calls, "query")
	return "", lab.observed
}

// TestGoBGPReloadRefusesUnmeasuredImages supplies a working query boundary but
// no container-image capability. Version strings cannot substitute for the
// absent image observation, so identify must refuse before making any query.
func TestGoBGPReloadRefusesUnmeasuredImages(t *testing.T) {
	lab := &gobgpReloadUnmeasuredLab{}
	proof := gobgpReloadProof{check: &interoplab.CheckContext{Lab: lab}, evidence: make(map[string]string)}
	err := proof.identify(t.Context())
	if err == nil {
		t.Fatal("identity-free lab supplied a reload proof")
	}
	if !strings.Contains(err.Error(), "requires actual peer-image observation") {
		t.Fatalf("missing capability was not named: %v", err)
	}
	if lab.queries != 0 {
		t.Fatalf("missing image identity fell back to %d version/config queries", lab.queries)
	}
	for key := range proof.evidence {
		if strings.HasPrefix(key, "image-") {
			t.Fatalf("missing capability fabricated image evidence %s", key)
		}
	}
}

type gobgpReloadUnmeasuredLab struct {
	interoplab.CheckerLab
	queries int
}

func (lab *gobgpReloadUnmeasuredLab) Query(context.Context, string, []string, []interoplab.EnvironmentVariable) (string, error) {
	lab.queries++
	return "available version/config data\n", nil
}
