// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS buffer pool

package radius

import "codeberg.org/thomas-mangin/ze/internal/core/bufpool"

const (
	poolBufSize = MaxPacketLen // 4096
	poolBufs    = 16
)

// Bufs is the shared buffer pool for RADIUS packet encoding/decoding.
var Bufs = bufpool.New(poolBufs, poolBufSize, "radius")
