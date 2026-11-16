// Design: docs/research/l2tpv2-ze-integration.md -- transport-independent PPP control delivery

package ppp

import "io"

// FrameChannel lends the session's PPP control streams to the driver. The
// SessionHandle owns their lifetime: callers MUST NOT close a borrowed stream.
// A transport MUST provide one Control reader and may provide a distinct Opened
// reader. Linux delivers NCP frames on its unit only after Connect; a socket
// backend returns nil Opened and keeps all control protocols on Control.
// Neither a nil FrameChannel nor a nil Control stream is valid.
type FrameChannel interface {
	Control() io.ReadWriter
	Opened() io.Reader
	// InterruptRead wakes Control without disabling writes. The driver MUST
	// join its reader before handing the underlying transport to a relay.
	InterruptRead() error
}
