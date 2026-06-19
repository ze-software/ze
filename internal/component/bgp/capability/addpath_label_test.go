package capability

import "testing"

// TestAddPathModeLabel pins the single source of truth for ADD-PATH direction
// labels shared by the RS capability view and the format/decode display.
//
// VALIDATES: AddPathMode.Label returns the RFC 7911 direction string for the
// three valid negotiated modes and "" for None/unknown so callers skip them.
// PREVENTS: the rs/server.go and format/decode.go copies of this mapping
// drifting apart (e.g. one emitting "send-receive" while the other emits
// "both") now that both derive the label from this one method.
func TestAddPathModeLabel(t *testing.T) {
	tests := []struct {
		mode AddPathMode
		want string
	}{
		{AddPathReceive, "receive"},
		{AddPathSend, "send"},
		{AddPathBoth, "send-receive"},
		{AddPathNone, ""},
		{AddPathMode(99), ""},
	}
	for _, tt := range tests {
		if got := tt.mode.Label(); got != tt.want {
			t.Errorf("AddPathMode(%d).Label() = %q, want %q", tt.mode, got, tt.want)
		}
	}
}
