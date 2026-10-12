// Design: docs/guide/bfd.md -- a client opting into BFD needs BFD running
//
// The goal is that a candidate config in which a client opts into BFD while
// the bfd section is absent or sets `enabled false` is refused at commit, and
// the refusal names the clients. The method asks the api seam this plugin
// registers in init(), the one every client asks at commit.
// VALIDATES: a BFD opt-in is refused at commit while bfd is absent or disabled.
// PREVENTS: BFD removed from under a protected client, which then runs unmonitored.
package bfd

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/api"
)

func TestClientOptInNeedsBFDRunning(t *testing.T) {
	optIns := []string{"peer 192.0.2.1", "peer 192.0.2.2"}
	cases := []struct {
		name    string
		section string
		optIns  []string
		refusal string
	}{
		{"enabled by default", `{"bfd":{}}`, optIns, ""},
		{"enabled true", `{"bfd":{"enabled":"true"}}`, optIns, ""},
		{"enabled false", `{"bfd":{"enabled":"false"}}`, optIns, "bfd { enabled false }"},
		{"no bfd section", "", optIns, "the config has no bfd section"},
		{"no opt-in, no bfd section", "", nil, ""},
		{"no opt-in, enabled false", `{"bfd":{"enabled":"false"}}`, nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := api.CheckOptIns(tc.section, tc.optIns)
			if tc.refusal == "" {
				if err != nil {
					t.Fatalf("commit refused with BFD running or no opt-in: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.refusal) {
				t.Fatalf("commit error = %v, want %q", err, tc.refusal)
			}
			for _, client := range tc.optIns {
				if !strings.Contains(err.Error(), client) {
					t.Errorf("commit error = %v, want %q named", err, client)
				}
			}
		})
	}
}
