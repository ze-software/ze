//go:build !linux

// Design: docs/research/l2tpv2-ze-integration.md -- Linux-only native client.

package kernel

import "errors"

func originalPairUnsupported() error {
	return errors.New("ppp: original-file PPP attachment requires Linux; use the Ze runtime kernel")
}
func prepareOriginalSessionPair(*claimedSessionPair) error    { return originalPairUnsupported() }
func connectOriginalSessionPair(*claimedSessionPair) error    { return originalPairUnsupported() }
func disconnectOriginalSessionPair(*claimedSessionPair) error { return originalPairUnsupported() }
func productionClientPairOps() clientPairOps {
	return clientPairOps{prepare: prepareOriginalSessionPair}
}
