// VALIDATES: interfaceConfig stays within the gocritic rangeValCopy size budget so the
// many `for _, ic := range ...Interfaces` loops do not each copy a large struct.
// PREVENTS: a new interfaceConfig field silently pushing the struct past the copy
// threshold and regressing every range loop over it (keep large/optional fields behind
// a pointer, as IPsec/TE/NBMA already are).
package ospf

import (
	"testing"
	"unsafe"
)

func TestInterfaceConfigCopyBudget(t *testing.T) {
	// gocritic rangeValCopy sizeThreshold in .golangci.yml.
	const maxBytes = 160
	if got := unsafe.Sizeof(interfaceConfig{}); got > maxBytes {
		t.Fatalf("interfaceConfig is %d bytes, over the %d-byte copy budget; move a large/optional field behind a pointer", got, maxBytes)
	}
}

// TestOspfConfigCopyBudget guards ospfConfig against the gocritic hugeParam sizeThreshold in
// .golangci.yml (288). ospfConfig is an intentionally-copied immutable snapshot passed by
// value across ~20 read helpers; a new config field that grows it past the threshold would
// regress every one of those signatures into a lint failure. Keep new fields small (pack
// into existing padding) or behind a pointer; do not silently bump the shared threshold.
func TestOspfConfigCopyBudget(t *testing.T) {
	// gocritic hugeParam flags a by-value parameter whose size reaches the sizeThreshold
	// (288 in .golangci.yml), so the struct must stay strictly under it.
	const hugeParamThreshold = 288
	if got := unsafe.Sizeof(ospfConfig{}); got >= hugeParamThreshold {
		t.Fatalf("ospfConfig is %d bytes, at/over the %d-byte hugeParam threshold; keep new config fields small (pack into padding) or behind a pointer", got, hugeParamThreshold)
	}
}
