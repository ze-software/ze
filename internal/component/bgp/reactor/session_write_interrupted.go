// Design: docs/architecture/behavior/peer-lifecycle.md -- panic recovery, what reaches the wire after a contained panic
// Related: session_wire.go -- observedBGPWriter, whose writeOpen marks a socket write that never returned
// Related: session_write.go -- releaseWrites, the release every write-path holder defers, which calls this first
// Related: peer_initial_sync.go -- endSessionAfterSyncPanic, whose NOTIFICATION follows the discard
package reactor

// discardInterruptedWrite drops the octets bufWriter holds when a panic
// unwound through the socket write that was carrying them, so no later flush
// offers them to the peer again. The caller MUST hold the write path, and
// releaseWrites calls it before it gives the path back: a writer waiting for
// the path (a KEEPALIVE, the recovery's own NOTIFICATION) then takes a buffer
// holding nothing of the interrupted write.
//
// bufio.Writer.Flush forgets the octets it offered only when the socket write
// returns. A panic inside that write (the socket, or an observer
// observedBGPWriter.Write calls after the socket accepted the octets) leaves
// every one of them staged, and the next flush, whoever makes it, sends them
// ahead of its own message. That put the interrupted UPDATE or End-of-RIB of an
// initial sync in front of the Cease its recovery sends: a marker for an
// initial update that never completed, or a duplicate of a frame the socket had
// already taken. Whether the socket took them cannot be known after the panic,
// so they are dropped, and the AIGP writes they carried are recorded as not
// delivered, as a failed write records them (retireWrite). The initial sync's
// and the read loop's recoveries then end the session with a Cease
// (endSessionAfterSyncPanic, ceaseAfterPanic). The forward worker's recovery
// keeps the session (fwdPool.safeBatchHandle), so there the dropped octets are
// lost to the peer, as the rest of the batch that recovery abandons already is.
//
// A session whose bufWriter does not write through wireWriter has no socket
// boundary to read, and keeps its octets: only a test wires one that way, since
// connectionEstablished always wires bufWriter to wireWriter.
func (s *Session) discardInterruptedWrite() {
	wire := s.wireWriter
	if wire == nil {
		return
	}
	if !wire.writeOpen {
		return
	}
	wire.writeOpen = false
	s.bufWriter.Reset(wire)
	s.writePending = false
	s.commitAIGPWrites(false)
}
