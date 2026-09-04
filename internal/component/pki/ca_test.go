// VALIDATES: the local certificate authority generates its root once, keeps it,
// and issues leaves with the properties a certificate authority owes: a unique
// 128-bit serial and a NotBefore backdated by the stated clock-skew margin.
// PREVENTS: a root regenerated on every restart (every distributed copy stops
// working), a repeated serial, a leaf a peer with a fast clock refuses as not
// yet valid, and a race between two in-process callers ending with two roots.
package pki

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/zefs"
)

// newRootStore opens a real blob store in a temporary directory and returns it
// with the path of its backing file. The daemon passes exactly this type, so the
// tests below exercise the production persistence rather than a fake.
func newRootStore(t *testing.T) (storage.Storage, string) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "database.zefs")
	store, err := storage.NewBlob(path, dir)
	if err != nil {
		t.Fatalf("open blob store: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := store.Close(); closeErr != nil {
			t.Errorf("close blob store: %v", closeErr)
		}
	})
	return store, path
}

func TestRootIsGeneratedOnceAndReused(t *testing.T) {
	store, path := newRootStore(t)

	first, err := LoadOrGenerateRoot(store)
	if err != nil {
		t.Fatalf("first LoadOrGenerateRoot: %v", err)
	}

	second, err := LoadOrGenerateRoot(store)
	if err != nil {
		t.Fatalf("second LoadOrGenerateRoot: %v", err)
	}
	if !bytes.Equal(first.cert.Raw, second.cert.Raw) {
		t.Fatal("a second call generated a new root instead of reading the stored one")
	}

	if !first.cert.IsCA {
		t.Fatal("the root must be a CA certificate")
	}
	if !first.cert.BasicConstraintsValid {
		t.Fatal("the root must carry valid basic constraints")
	}
	span := first.cert.NotAfter.Sub(first.cert.NotBefore)
	if span != rootValidity+clockSkewMargin {
		t.Fatalf("root validity span %v, want %v", span, rootValidity+clockSkewMargin)
	}

	// A restart: close the store, reopen the same file, and load again. The
	// daemon that comes back must present the root the operator already has.
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	dir := filepath.Dir(path)
	reopened, err := storage.NewBlob(path, dir)
	if err != nil {
		t.Fatalf("reopen blob store: %v", err)
	}
	defer reopened.Close() //nolint:errcheck // the test fails on the assertion below, not on close

	afterRestart, err := LoadOrGenerateRoot(reopened)
	if err != nil {
		t.Fatalf("LoadOrGenerateRoot after restart: %v", err)
	}
	if !bytes.Equal(first.cert.Raw, afterRestart.cert.Raw) {
		t.Fatal("the root did not survive a restart")
	}
}

func TestRootKeyIsWrittenPrivate(t *testing.T) {
	store, path := newRootStore(t)

	if _, err := LoadOrGenerateRoot(store); err != nil {
		t.Fatalf("LoadOrGenerateRoot: %v", err)
	}

	// The key entry is registered Private, so no listing shows it. zefs has no
	// per-key file mode, so the registry flag and the blob's own mode are what
	// AC-1 asserts.
	if !zefs.KeyCAKey.Private {
		t.Fatal("the root key entry must be registered Private")
	}
	for _, entry := range zefs.Entries() {
		if entry.Pattern == zefs.KeyCAKey.Pattern {
			t.Fatalf("%s appears in the public key listing", entry.Pattern)
		}
	}
	found := false
	for _, entry := range zefs.AllEntries() {
		if entry.Pattern == zefs.KeyCAKey.Pattern {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("%s is not registered at all", zefs.KeyCAKey.Pattern)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat blob file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("blob file mode %#o, want 0600", perm)
	}
}

func TestIssueLeafDrawsAUniqueSerial(t *testing.T) {
	store, _ := newRootStore(t)

	root, err := LoadOrGenerateRoot(store)
	if err != nil {
		t.Fatalf("LoadOrGenerateRoot: %v", err)
	}

	const issuances = 64
	limit := new(big.Int).Lsh(big.NewInt(1), serialBits)
	seen := make(map[string]bool, issuances)
	wideSerials := 0

	for i := range issuances {
		cert, err := root.IssueLeaf("ze-test", []string{"127.0.0.1"})
		if err != nil {
			t.Fatalf("IssueLeaf %d: %v", i, err)
		}
		leaf, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			t.Fatalf("parse leaf %d: %v", i, err)
		}

		serial := leaf.SerialNumber
		if serial.Sign() <= 0 {
			t.Fatalf("serial %d is not positive: %s", i, serial)
		}
		if serial.Cmp(limit) >= 0 {
			t.Fatalf("serial %d needs %d bits, more than the %d drawn", i, serial.BitLen(), serialBits)
		}
		if seen[serial.String()] {
			t.Fatalf("serial %s was issued twice", serial)
		}
		seen[serial.String()] = true

		if serial.BitLen() > serialBits-8 {
			wideSerials++
		}
	}

	// A serial drawn from the full 128-bit range lands in the top 1/256 of it
	// about 255 times in 256. Sixty-four draws all landing below that point has
	// probability (1/256)^64, so this assertion fails only if the draw is not
	// 128 bits wide.
	if wideSerials == 0 {
		t.Fatalf("no serial out of %d needed more than %d bits: the draw is narrower than %d bits",
			issuances, serialBits-8, serialBits)
	}
	// The root draws from the same range, so it is not a counter either.
	rootSerial := root.cert.SerialNumber
	if rootSerial.Sign() <= 0 || rootSerial.Cmp(limit) >= 0 {
		t.Fatalf("root serial %s is outside the %d-bit range", rootSerial, serialBits)
	}
}

// TestIssueLeafRefusesALeafThatIdentifiesNothing drives the two guards from the
// entry point. A leaf with no SAN is refused at every handshake, one process
// away from the caller that asked for it, so the CA refuses it where the caller
// can be named.
func TestIssueLeafRefusesALeafThatIdentifiesNothing(t *testing.T) {
	store, _ := newRootStore(t)

	root, err := LoadOrGenerateRoot(store)
	if err != nil {
		t.Fatalf("LoadOrGenerateRoot: %v", err)
	}

	if _, err := root.IssueLeaf("ze-test", nil); !errors.Is(err, errIssueLeafNoHosts) {
		t.Fatalf("IssueLeaf with no hosts: %v, want errIssueLeafNoHosts", err)
	}
	if _, err := root.IssueLeaf("", []string{"127.0.0.1"}); !errors.Is(err, errIssueLeafNoCommonName) {
		t.Fatalf("IssueLeaf with no common name: %v, want errIssueLeafNoCommonName", err)
	}
}

func TestIssueLeafBackdatesNotBefore(t *testing.T) {
	store, _ := newRootStore(t)

	root, err := LoadOrGenerateRoot(store)
	if err != nil {
		t.Fatalf("LoadOrGenerateRoot: %v", err)
	}

	before := time.Now()
	cert, err := root.IssueLeaf("ze-test", []string{"127.0.0.1"})
	if err != nil {
		t.Fatalf("IssueLeaf: %v", err)
	}
	after := time.Now()

	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("parse leaf: %v", err)
	}

	// X.509 encodes both times to a whole second, and both derive from one
	// time.Now, so the span is exact and pins the stated margin.
	span := leaf.NotAfter.Sub(leaf.NotBefore)
	if span != leafValidity+clockSkewMargin {
		t.Fatalf("leaf validity span %v, want %v (%v lifetime plus a %v skew margin)",
			span, leafValidity+clockSkewMargin, leafValidity, clockSkewMargin)
	}

	// NotBefore is in the past by the margin. One second of slack covers the
	// truncation to a whole second.
	earliest := before.Add(-clockSkewMargin).Add(-time.Second)
	latest := after.Add(-clockSkewMargin)
	if leaf.NotBefore.Before(earliest) || leaf.NotBefore.After(latest) {
		t.Fatalf("NotBefore %v is outside [%v, %v]: it is not backdated by %v",
			leaf.NotBefore, earliest, latest, clockSkewMargin)
	}
}

func TestConcurrentRootGenerationAgrees(t *testing.T) {
	store, _ := newRootStore(t)

	const callers = 16
	roots := make([]*Root, callers)
	errs := make([]error, callers)

	var start sync.WaitGroup
	var done sync.WaitGroup
	start.Add(1)
	done.Add(callers)
	for i := range callers {
		go func() {
			defer done.Done()
			start.Wait()
			roots[i], errs[i] = LoadOrGenerateRoot(store)
		}()
	}
	start.Done()
	done.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("caller %d: %v", i, err)
		}
	}
	for i := 1; i < callers; i++ {
		if !bytes.Equal(roots[0].cert.Raw, roots[i].cert.Raw) {
			t.Fatalf("caller %d ended with a different root: two racing callers generated two roots", i)
		}
	}

	stored, err := store.ReadFile(zefs.KeyCACert.Pattern)
	if err != nil {
		t.Fatalf("read stored root: %v", err)
	}
	block, _ := pem.Decode(stored)
	if block == nil {
		t.Fatal("the stored root is not PEM")
	}
	if !bytes.Equal(block.Bytes, roots[0].cert.Raw) {
		t.Fatal("the stored root is not the one the callers hold")
	}
}

// TestIssuanceRefusesANonPositiveValidity drives both lifetime-naming entry
// points. A certificate that has already expired when it is issued is refused
// by every peer, and the caller that asked for it is one process away by then,
// so the refusal belongs here.
func TestIssuanceRefusesANonPositiveValidity(t *testing.T) {
	store, _ := newRootStore(t)

	for _, validity := range []time.Duration{0, -time.Hour} {
		if _, err := LoadOrGenerateRootFor(store, validity); !errors.Is(err, errRootNoValidity) {
			t.Errorf("LoadOrGenerateRootFor(%s) error = %v, want errRootNoValidity", validity, err)
		}
	}

	root, err := LoadOrGenerateRoot(store)
	if err != nil {
		t.Fatalf("LoadOrGenerateRoot: %v", err)
	}

	for _, validity := range []time.Duration{0, -time.Hour} {
		if _, err := root.IssueLeafFor("ze-test", []string{"127.0.0.1"}, validity); !errors.Is(err, errIssueNoValidity) {
			t.Errorf("IssueLeafFor(%s) error = %v, want errIssueNoValidity", validity, err)
		}
	}
}

// TestIssueLeafForHonoursTheLifetimeItIsGiven holds what the appliance build
// host depends on: a leaf minted once lives as long as the caller asked, not
// the 24 hours a component reissuing at every start takes.
func TestIssueLeafForHonoursTheLifetimeItIsGiven(t *testing.T) {
	store, _ := newRootStore(t)

	root, err := LoadOrGenerateRoot(store)
	if err != nil {
		t.Fatalf("LoadOrGenerateRoot: %v", err)
	}

	const asked = 30 * 24 * time.Hour
	pair, err := root.IssueLeafFor("ze-test", []string{"127.0.0.1"}, asked)
	if err != nil {
		t.Fatalf("IssueLeafFor: %v", err)
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		t.Fatalf("parse the issued leaf: %v", err)
	}

	life := leaf.NotAfter.Sub(leaf.NotBefore)
	want := asked + clockSkewMargin
	if life < want-time.Minute || life > want+time.Minute {
		t.Errorf("leaf lives %s, want the asked %s plus the %s skew margin", life, asked, clockSkewMargin)
	}
}
