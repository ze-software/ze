// Design: docs/architecture/testing/ci-format.md -- ze-peer connection scripts
package peer

import "testing"

// VALIDATES: a send that ends conn=1's script stops the sends there: conn=2's
// send is not written on the first connection, the connection ends
// (sequenceEnded), and the send goes out once the next connection starts (Init).
// PREVENTS: a restart test whose peer announces on two connections writing both
// UPDATEs into the first session, then holding it, so the restarted daemon's
// session gets nothing (test/plugin/irr-restart-keeps-bgp-list.ci).
//
// Method: drive the checker as runMessageLoop does over a two-connection
// script of one send each.
func TestSendStopsAtConnectionBoundary(t *testing.T) {
	c, err := newChecker([]string{
		"action=send:conn=1:seq=1:hex=AAAA",
		"action=send:conn=2:seq=1:hex=BBBB",
	})
	if err != nil {
		t.Fatal(err)
	}
	c.Init()

	ok, hexData := c.nextSendAction()
	if !ok || hexData != "AAAA" {
		t.Fatalf("first send %v %q, want conn=1's AAAA", ok, hexData)
	}
	if ok, hexData := c.nextSendAction(); ok {
		t.Fatalf("conn=2's send %q went out on conn=1", hexData)
	}
	if c.Completed() {
		t.Fatal("the checker reports completed with conn=2's send still owed")
	}
	if !c.sequenceEnded() {
		t.Fatal("conn=1's script ended with its send, want sequenceEnded")
	}

	c.Init()
	ok, hexData = c.nextSendAction()
	if !ok || hexData != "BBBB" {
		t.Fatalf("send on the second connection %v %q, want conn=2's BBBB", ok, hexData)
	}
	if !c.Completed() {
		t.Fatal("every send went out, want completed")
	}
}
