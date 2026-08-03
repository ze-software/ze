package ipsec

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/ike/crypto"
)

// VALIDATES: an ESP proposal that names a hash beside an AEAD cipher is refused at
// parse time, and the error names the algorithm.
// PREVENTS: the silent half-working tunnel. RFC 7296 Section 3.3 makes the integrity
// transform NONE for an AEAD cipher, so the hash names nothing ESP can carry. Ze once
// accepted the spelling. It then derived two integrity keys the peer never derives.
// That moved the responder encryption key 32 octets, and one direction stopped
// decrypting.
func TestParseESPProposalRejectsHashBesideAEAD(t *testing.T) {
	tree := makeESPTree("ESP-GCM", "", "", map[string][2]string{
		"1": {"aes256gcm", "sha256"},
	})

	_, err := ParseIPsecConfig(tree)
	if err == nil {
		t.Fatal("ParseIPsecConfig accepted a hash beside an AEAD cipher, want an error")
	}
	for _, want := range []string{"aes256gcm", "sha256", "AEAD"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}

// VALIDATES: an AEAD ESP proposal with no hash still parses.
// PREVENTS: a rejection that catches the correct spelling as well as the wrong one.
func TestParseESPProposalAcceptsAEADWithoutHash(t *testing.T) {
	tree := makeESPTree("ESP-GCM", "", "", map[string][2]string{
		"1": {"aes256gcm", ""},
	})

	cfg, err := ParseIPsecConfig(tree)
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}
	if got := cfg.ESPGroups["ESP-GCM"].Proposals[0].Hash; got != HashUnknown {
		t.Errorf("hash = %s, want the zero value for an AEAD proposal", got)
	}
}

// VALIDATES: a config naming an encryption algorithm the crypto registry does not
// implement is refused at parse time, and the error names the algorithm and the
// implemented set.
// PREVENTS: ENCR Transform ID 0 on the wire. RFC 7296 Section 3.3.2 reserves that ID.
// lookupEncryption returns the zero transform for an unknown name. Ze therefore once
// started, offered ID 0 with no Key Length attribute, derived a zero-length key, and
// handed it to the kernel (ai/rules/evidence.md).
func TestParseRejectsUnimplementedEncryption(t *testing.T) {
	for _, algo := range []string{"chacha20poly1305", "3des"} {
		t.Run(algo, func(t *testing.T) {
			esp := makeESPTree("ESP-X", "", "", map[string][2]string{"1": {algo, ""}})
			err := parseErr(t, esp)
			if !strings.Contains(err.Error(), algo) {
				t.Errorf("esp-group error %q does not name %q", err, algo)
			}
			if !strings.Contains(err.Error(), "aes256gcm") {
				t.Errorf("esp-group error %q does not name the implemented set", err)
			}

			ike := makeIKETree("IKE-X", ikeOpts{proposals: map[string]ikeProposalOpts{
				"1": {encryption: algo, hash: "sha256", dhGroup: "14"},
			}})
			err = parseErr(t, ike)
			if !strings.Contains(err.Error(), algo) {
				t.Errorf("ike-group error %q does not name %q", err, algo)
			}
		})
	}
}

// VALIDATES: a config naming a hash the crypto registry does not implement is refused
// at parse time.
// PREVENTS: an ESP SA installed with AUTH_NONE and a zero-length integrity key.
// lookupIntegrity returns the zero transform for "sha1", whose ID is AUTH_NONE, so a
// CBC proposal naming sha1 once keyed an SA that authenticates nothing.
func TestParseRejectsUnimplementedHash(t *testing.T) {
	esp := makeESPTree("ESP-X", "", "", map[string][2]string{"1": {"aes256", "sha1"}})
	err := parseErr(t, esp)
	if !strings.Contains(err.Error(), "sha1") {
		t.Errorf("esp-group error %q does not name sha1", err)
	}
	if !strings.Contains(err.Error(), "sha256") {
		t.Errorf("esp-group error %q does not name the implemented set", err)
	}

	ike := makeIKETree("IKE-X", ikeOpts{proposals: map[string]ikeProposalOpts{
		"1": {encryption: "aes256", hash: "sha1", dhGroup: "14"},
	}})
	err = parseErr(t, ike)
	if !strings.Contains(err.Error(), "sha1") {
		t.Errorf("ike-group error %q does not name sha1", err)
	}
}

// VALIDATES: every algorithm the parser accepts resolves to a real crypto transform,
// so no accepted config can reach a zero-value lookup.
// PREVENTS: a YANG enum added without a registry entry, which is how chacha20poly1305
// and 3des reached the wire as Transform ID 0.
func TestAcceptedAlgorithmsResolveToTransforms(t *testing.T) {
	for algo, name := range encryptionNames {
		if !EncryptionImplemented(algo) {
			continue
		}
		if _, err := encryptionTransformFor(algo); err != nil {
			t.Errorf("encryption %q is accepted but does not resolve: %v", name, err)
		}
	}
	for hash, name := range hashNames {
		if !HashImplemented(hash) {
			continue
		}
		// Both halves, because a proposal reads the hash as the integrity
		// algorithm for ESP and as the PRF for IKE. Checking the integrity half
		// alone leaves an accepted hash free to key IKE from a zero PRF.
		if _, err := integrityTransformFor(hash); err != nil {
			t.Errorf("hash %q is accepted but has no integrity transform: %v", name, err)
		}
		if _, err := crypto.LookupPRF(name); err != nil {
			t.Errorf("hash %q is accepted but has no PRF: %v", name, err)
		}
	}
}

// VALIDATES: SupportedHashNames omits a name that one registry holds and the other does
// not, which is exactly the set HashImplemented accepts.
// PREVENTS: an error message that offers a hash the parser then refuses. The integrity
// registry and the PRF registry hold the same three names today, so a build that names
// one of them alone reads as correct. The day one registry grows, the message starts
// advertising a hash whose other half is missing, and an operator follows it into a
// second rejection.
func TestSupportedHashNamesExcludesAHalfImplementedName(t *testing.T) {
	t.Cleanup(func() {
		integrityNames = crypto.SupportedIntegrityNames
		prfNames = crypto.SupportedPRFNames
	})
	integrityNames = func() []string { return []string{"integrity-only", "sha256"} }
	prfNames = func() []string { return []string{"prf-only", "sha256"} }

	got := SupportedHashNames()
	for _, odd := range []string{"integrity-only", "prf-only"} {
		if slices.Contains(got, odd) {
			t.Errorf("SupportedHashNames = %v, and %q is in one registry only", got, odd)
		}
	}
	if !slices.Equal([]string{"sha256"}, got) {
		t.Errorf("SupportedHashNames = %v, want the one name both registries hold", got)
	}
}

// VALIDATES: the advertised set and the predicate that refuses a config agree, name for
// name, over the registries this build ships.
// PREVENTS: the two drifting apart. HashImplemented decides what ParseIPsecConfig
// accepts, and SupportedHashNames writes what its error offers instead.
func TestSupportedHashNamesMatchesHashImplemented(t *testing.T) {
	advertised := make(map[string]bool, len(SupportedHashNames()))
	for _, name := range SupportedHashNames() {
		advertised[name] = true
	}
	for hash, name := range hashNames {
		if HashImplemented(hash) != advertised[name] {
			t.Errorf("HashImplemented(%s) = %v, advertised = %v",
				name, HashImplemented(hash), advertised[name])
		}
		delete(advertised, name)
	}
	if len(advertised) != 0 {
		t.Errorf("SupportedHashNames offers %v, which no HashAlgo can spell", advertised)
	}
}

// VALIDATES: no test file in this package calls t.Parallel.
// PREVENTS: a data race on the integrityNames and prfNames seams.
// TestSupportedHashNamesExcludesAHalfImplementedName writes both. go test runs the
// tests of one package sequentially until a test calls t.Parallel. This test turns that
// condition from a comment into a gate. The author who adds the first parallel test
// then learns about the seam here rather than from an intermittent -race report.
func TestThisPackageRunsItsTestsSequentially(t *testing.T) {
	paths, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no test file found, so this guard checks nothing")
	}
	// The needle is built from two pieces. Spelled whole, it would appear in this
	// file and the guard would report itself.
	needle := []byte("t." + "Parallel(")
	for _, path := range paths {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if bytes.Contains(source, needle) {
			t.Errorf("%s calls t.Parallel, and algorithm_support.go swaps "+
				"integrityNames and prfNames in a test. Give that test its own "+
				"non-global seam before this package runs tests in parallel", path)
		}
	}
}

// VALIDATES: for every algorithm this build implements, the config-enum AEAD
// predicate agrees with the wire Transform ID predicate.
// PREVENTS: the two domains disagreeing about an algorithm in use. The config enum
// knows ChaCha20-Poly1305 is AEAD, and the crypto registry does not carry it at all. A
// build that adds the transform to one domain alone keys ESP one way and negotiates it
// the other.
func TestAEADPredicatesAgreeAcrossDomains(t *testing.T) {
	for algo, name := range encryptionNames {
		if !EncryptionImplemented(algo) {
			continue
		}
		transform, err := encryptionTransformFor(algo)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if algo.IsAEAD() != transform.ID.IsAEAD() {
			t.Errorf("%s: ipsec.EncryptionAlgo.IsAEAD = %v, crypto.EncryptionID.IsAEAD = %v",
				name, algo.IsAEAD(), transform.ID.IsAEAD())
		}
	}
}

func parseErr(t *testing.T, tree *config.Tree) error {
	t.Helper()
	_, err := ParseIPsecConfig(tree)
	if err == nil {
		t.Fatal("ParseIPsecConfig accepted an unimplemented algorithm, want an error")
	}
	return err
}
