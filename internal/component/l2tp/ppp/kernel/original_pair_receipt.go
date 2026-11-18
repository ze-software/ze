// Design: docs/architecture/appliance/kernel-profiles.md -- retained PPP grant.
// Related: original_pair_abi_linux.go -- native positive-reply producer.

package kernel

// The nil receipt grants nothing. Only successful positive query51/query46
// replies from both actual originals mint this private pair-owned receipt.
// Cleanup keeps it after retirement, rather than requiring a live-only query.
type originalPairReceipt struct {
	unitFeatures    uint32
	channelFeatures uint32
}
