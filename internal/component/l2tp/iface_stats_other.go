// Design: docs/architecture/l2tp/bng-1-radius-attributes.md -- idle timeout traffic detection
//go:build !linux

package l2tp

// readIfaceRXBytes is a no-op on non-Linux platforms. Returns 0,
// which causes the idle timer to fire unconditionally after the
// configured idle period.
func readIfaceRXBytes(_ string) uint64 {
	return 0
}
