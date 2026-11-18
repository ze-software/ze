//go:build linux

// Design: docs/research/l2tpv2-ze-integration.md -- atomic original-pair operation.
// Related: session_pair_linux.go -- bounded leases on both originals.

package kernel

import (
	"errors"
	"os"
)

// Production supplies nil. The private native control performs one bounded
// deletion/reuse after live checking and before the SAME actual operation.
// No public hook, replacement file or numeric fallback can grant attachment.
func connectOriginalPairAfterLive(pair *claimedSessionPair, before func() error) error {
	if pair.original == nil {
		return errors.New("ppp: original pair has no native capability receipt")
	}
	return pair.withOriginalFiles(func(_ *os.File, channelFD, unitFD int) error {
		if before != nil {
			if err := before(); err != nil {
				return err
			}
		}
		if err := pppPairPeerOperation(channelFD, pppPairConnect, unitFD); err != nil {
			return originalAttachmentRefusal(err)
		}
		return nil
	})
}
