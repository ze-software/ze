// Design: docs/architecture/testing/interop.md -- the IPsec interop checkers

package ipsec

import (
	"strings"
	"testing"
)

// TestChildRekeyNarrowingNeedleIsThePeersDelete checks that the line
// checkChildRekeyNarrowing waits for proves strongSwan RECEIVED Ze's IKE SA Delete.
//
// Goal: strongSwan 6.1.0 ike_delete.c logs "deleting IKE_SA %s[%u] between ..." in
// build_i, when it deletes the IKE SA itself, and again in process_r, when it handles
// the peer's Delete. Only process_r logs "received DELETE for IKE_SA %s[%u]". A needle
// that matches the build_i lines lets the check pass with no Delete from Ze.
//
// Method: the lines each strongSwan path logs, as the lab sees them, are matched against
// swanLogPeerDeletedIKE.
//
// MUTATION: set swanLogPeerDeletedIKE back to "deleting IKE_SA ze[" and strongSwan's own
// deletion satisfies it.
func TestChildRekeyNarrowingNeedleIsThePeersDelete(t *testing.T) {
	ownDelete := "11[IKE] deleting IKE_SA ze[1] between 172.30.0.3[swan]...172.30.0.2[ze]\n" +
		"11[IKE] sending DELETE for IKE_SA ze[1]\n"
	peerDelete := "12[IKE] received DELETE for IKE_SA ze[1]\n" +
		"12[IKE] deleting IKE_SA ze[1] between 172.30.0.3[swan]...172.30.0.2[ze]\n"
	if strings.Contains(ownDelete, swanLogPeerDeletedIKE) {
		t.Fatalf("%q matches strongSwan deleting the IKE SA on its own initiative", swanLogPeerDeletedIKE)
	}
	if !strings.Contains(peerDelete, swanLogPeerDeletedIKE) {
		t.Fatalf("%q does not match strongSwan processing the peer's Delete", swanLogPeerDeletedIKE)
	}
}
