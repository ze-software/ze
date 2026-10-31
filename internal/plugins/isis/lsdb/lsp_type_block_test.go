// Design: docs/architecture/isis/isis-6-lsdb.md -- the LSP type block.
//
// VALIDATES: the IS Type bits of every LSP the node originates hold a defined
// value: 1 (Level 1 IS) in a Level 1 LSP and 3 (Level 2 IS) in a Level 2 LSP.
// PREVENTS: a Level 2 LSP carrying the unused IS Type value 2, which FRR
// reports as "invalid LSP is type 0x2" on every L2 LSP Ze floods to it.

package lsdb

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
)

// isTypeMask selects the two IS Type bits of the LSP type block.
const isTypeMask = packet.LSPFlagISTypeL1 | packet.LSPFlagISTypeL2

func TestLSPTypeBlockISTypeIsDefinedValue(t *testing.T) {
	for _, tc := range []struct {
		name  string
		level Level
		want  uint8
	}{
		{"level-1", Level1, 0x01},
		{"level-2", Level2, 0x03},
	} {
		for _, overload := range []bool{false, true} {
			got := lspTypeBlock(tc.level, overload, true) & isTypeMask
			if got != tc.want {
				t.Fatalf("%s overload=%v: IS Type = %#x, want %#x", tc.name, overload, got, tc.want)
			}
		}
	}
}
