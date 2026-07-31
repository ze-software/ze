// Design: plan/learned/734-ipsec-3-data-model.md -- IPsec data model types
// Related: types.go -- the algorithm enums these predicates decide
// Related: config.go -- the parser that refuses an algorithm no build implements

package ipsec

import (
	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The YANG enum offers every algorithm the data model can name. A build implements a
// smaller set: the crypto transform registry decides it. The two are allowed to differ,
// and the parser is the one place that says so.
//
// Nothing downstream can say it. lookupEncryption, lookupIntegrity and lookupPRF all
// return the zero transform for a name the registry does not hold, and a zero
// EncryptionTransform carries Transform ID 0, which RFC 7296 Section 3.3.2 reserves. A
// zero IntegrityTransform carries AUTH_NONE, which reads as a valid "no integrity"
// answer. Both are the zero-value trap of ai/rules/fail-closed-guards.md, and both
// reach the wire and the kernel without an error. The guard therefore belongs at the
// producer, which is config parse (ai/rules/exact-or-reject.md).

// EncryptionImplemented reports whether this build carries a transform for the
// algorithm. ParseIPsecConfig refuses a proposal that names one it does not.
func EncryptionImplemented(e EncryptionAlgo) bool {
	_, err := encryptionTransformFor(e)
	return err == nil
}

// HashImplemented reports whether this build carries an integrity transform and a PRF
// for the algorithm. An IKE proposal reads its hash as the PRF and an ESP proposal
// reads it as the integrity algorithm, so a usable hash needs both.
func HashImplemented(h HashAlgo) bool {
	if _, err := integrityTransformFor(h); err != nil {
		return false
	}
	_, err := crypto.LookupPRF(h.String())
	return err == nil
}

// DHGroupImplemented reports whether this build carries a Diffie-Hellman transform for
// the group. ParseIPsecConfig refuses a proposal that names one it does not.
//
// ValidDHGroup alone is not enough. It answers whether the NUMBER is in the range RFC
// 7296 Section 3.3.2 assigns Transform Type 4, which is 1..31, while the registry holds
// three groups. A proposal naming group 5 therefore passed parse and reached the
// negotiator, where LookupDHGroup returns the ZERO DHGroupTransform -- Transform ID 0,
// which RFC 7296 Section 3.3.2 reserves. That is the zero-value trap of
// ai/rules/fail-closed-guards.md reaching the wire, and it is the same failure the
// encryption and hash gates above already close.
func DHGroupImplemented(g DHGroup) bool {
	_, err := crypto.LookupDHGroup(uint8(g))
	return err == nil
}

func encryptionTransformFor(e EncryptionAlgo) (crypto.EncryptionTransform, error) {
	return crypto.LookupEncryption(e.String())
}

func integrityTransformFor(h HashAlgo) (crypto.IntegrityTransform, error) {
	return crypto.LookupIntegrity(h.String())
}

// SupportedEncryptionNames and SupportedHashNames name the implemented sets for an
// error message. Both derive from the crypto registry, so neither can drift from what
// the daemon can actually key (ai/rules/derive-not-hardcode.md).
func SupportedEncryptionNames() []string { return crypto.SupportedEncryptionNames() }

// SupportedDHGroupIDs lists the Diffie-Hellman groups this build implements, for the
// error DHGroupImplemented's caller returns. Derived for the same reason.
func SupportedDHGroupIDs() []uint8 { return crypto.SupportedDHGroupIDs() }

// joinDHGroupIDs renders the implemented group numbers as "14, 19, 20" for an error
// message. The sibling predicates name a []string set, so they can use textbuf.Join.
// A group is a number instead, so joinDHGroupIDs builds the list in one buffer rather
// than through an intermediate []string (ai/rules/no-sprintf-alloc.md).
func joinDHGroupIDs(ids []uint8) string {
	var b textbuf.Buffer
	for i, id := range ids {
		if i > 0 {
			b.Str(", ")
		}
		b.Uint8(id)
	}
	return b.String()
}

// integrityNames and prfNames name the two registries a usable hash needs an entry in.
// They are variables rather than direct calls so a test can make the two disagree. No
// build ships a divergence. Without one, the intersection below returns the same list
// as either registry alone, so nothing else can tell a correct answer from a wrong one.
//
// Nothing writes them at run time. A test that swaps them restores both with t.Cleanup,
// and TestThisPackageRunsItsTestsSequentially keeps that swap free of a data race.
var (
	integrityNames = crypto.SupportedIntegrityNames
	prfNames       = crypto.SupportedPRFNames
)

// SupportedHashNames lists the hash algorithms this build implements. HashImplemented
// requires both an integrity transform and a PRF, so the answer is the intersection of
// the two registries.
//
// The two hold the same names today. Naming one of them would still be wrong, because
// the message would then advertise a hash the parser refuses on the other half. The
// intersection cannot say that, whichever registry grows first.
func SupportedHashNames() []string {
	prfs := make(map[string]bool, len(prfNames()))
	for _, name := range prfNames() {
		prfs[name] = true
	}
	both := make([]string, 0, len(prfs))
	for _, name := range integrityNames() {
		if prfs[name] {
			both = append(both, name)
		}
	}
	return both
}
