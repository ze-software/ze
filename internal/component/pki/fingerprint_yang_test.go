// Design: show.go -- each hash word selects the digest certFingerprint takes
//
// Goal: prove the algorithms `show pki certificate fingerprint` offers an
// operator are the ones certFingerprint computes, so a word cannot exist on
// one side alone. Method: read the enumeration out of the loaded model with
// configyang.EnumValues, which fails on a leaf that declares no enumeration,
// drive certFingerprint with each word, and walk the three Go words back to
// the model.

package pki

import (
	"slices"
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"

	// ze-pki-cmd augments the show tree of ze-cli-show-cmd, so both modules
	// register with the loader: the leaf read below resolves only when the
	// tree it hangs from is loaded too.
	_ "github.com/ze-software/ze/internal/component/cmd/show/yang"
	_ "github.com/ze-software/ze/internal/plugins/pki-cmd/yang"
)

const fingerprintAlgorithmLeaf = "show/pki/certificate/name/fingerprint/algorithm/algorithm"

// TestFingerprintAlgorithmsMatchTheModel holds certFingerprint's words to the
// model in both directions.
func TestFingerprintAlgorithmsMatchTheModel(t *testing.T) {
	model, err := configyang.EnumValues(fingerprintAlgorithmLeaf)
	if err != nil {
		t.Fatalf("read the enumeration at %s: %v", fingerprintAlgorithmLeaf, err)
	}

	hashed := []string{algoSHA256, algoSHA384, algoSHA512}
	slices.Sort(hashed)
	if !slices.Equal(model, hashed) {
		t.Errorf("the algorithms disagree: the model at %s holds %v and show.go hashes with %v. "+
			"A word only the model carries is refused as unsupported, and a word only Go carries is one no operator can ask for",
			fingerprintAlgorithmLeaf, model, hashed)
	}

	entry := &CertificateEntry{Raw: []byte("not a certificate, and the digest does not care")}
	for _, algo := range model {
		resp, err := certFingerprint("test", nil, entry, algo)
		if err != nil {
			t.Fatalf("algorithm %q: %v", algo, err)
		}
		if resp.Status != plugin.StatusDone {
			t.Errorf("%q is offered at %s and certFingerprint refused it: %s", algo, fingerprintAlgorithmLeaf, resp.Error)
			continue
		}
		data, ok := resp.Data.(plugin.Map)
		if !ok {
			t.Fatalf("algorithm %q answered %T, not a plugin.Map", algo, resp.Data)
		}
		if data["fingerprint"] == "" {
			t.Errorf("algorithm %q answered an empty fingerprint", algo)
		}
	}
}
