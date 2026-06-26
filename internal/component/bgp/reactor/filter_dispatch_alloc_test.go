package reactor

import (
	"testing"
	"unsafe"
)

// BenchmarkFilterDispatch_ZeroAlloc measures the full filter-dispatch path from
// scratch buffer through PolicyFilterChain with a mock accept-all filter.
// After spec-plugin-ipc-raw-bytes, this reports 0 allocs/op because the
// string(scratch) conversion at the IPC boundary is replaced with unsafe.String.
//
// Safety: unsafe.String is valid because PolicyFilterChain and CallRPC are
// synchronous; json.Marshal copies string bytes before the function returns,
// and scratchArr (stack-local) outlives the entire call chain.
func BenchmarkFilterDispatch_ZeroAlloc(b *testing.B) {
	attrs := buildAttrsWireFixture(b)
	scratch := make([]byte, 0, 4096)

	acceptAll := func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
		return PolicyResponse{Action: PolicyAccept}
	}

	filters := []string{"test:accept"}

	b.ReportAllocs()
	b.ResetTimer()
	var action PolicyAction
	for range b.N {
		scratch = AppendUpdateForFilter(scratch[:0], attrs, nil, nil)
		updateText := unsafe.String(unsafe.SliceData(scratch), len(scratch)) //nolint:gosec // audited: scratch outlives synchronous PolicyFilterChain
		action = PolicyFilterChain(filters, "import", "10.0.0.1", 65001, updateText, acceptAll).Action
	}
	_ = action
}
