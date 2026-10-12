// Design: docs/guide/bfd.md -- a client opting into BFD needs BFD running
//
// The goal is the owner decision of 2026-10-12: an OSPF interface with
// `bfd { enabled true }` opts into BFD, so a candidate config whose bfd section
// is absent or sets `enabled false` is refused at commit, and the refusal names
// every interface still opting in, on both address families. The method runs
// the OSPF commit check over the sections a commit delivers: the ospf section
// and the bfd section OSPF reads.
// VALIDATES: a BFD opt-in is refused at commit while bfd is absent or disabled.
// PREVENTS: BFD removed from under a protected client, which then runs unmonitored.
package ospf

import (
	"strings"
	"testing"

	// The bfd plugin registers the real enabled check (api.CheckOptIns).
	_ "github.com/ze-software/ze/internal/component/bfd"
)

const ospfBFDOptInConfig = `{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},` +
	`"interfaces":{"interface":{"eth0":{"area":"0","bfd":{"enabled":"true"}},"eth1":{"area":"0"}}},` +
	`"address-family":{"ipv6":{"areas":{"area":{"0":{"area-id":"0"}}},` +
	`"interfaces":{"interface":{"eth2":{"area":"0","bfd":{"enabled":"true"}}}}}}}}`

func TestOSPFCommitRefusesBFDOptInWithoutBFD(t *testing.T) {
	cases := []struct {
		name    string
		bfd     string
		refusal string
	}{
		{"bfd running", `{"bfd":{}}`, ""},
		{"bfd disabled", `{"bfd":{"enabled":"false"}}`, "bfd { enabled false }"},
		{"no bfd section", "", "the config has no bfd section"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sections := []configSection{{Root: "ospf", Data: ospfBFDOptInConfig}}
			if tc.bfd != "" {
				sections = append(sections, configSection{Root: "bfd", Data: tc.bfd})
			}
			cfg, err := parseOSPFConfig(sections, nil)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			err = checkBFDOptIns(cfg, sections)
			if tc.refusal == "" {
				if err != nil {
					t.Fatalf("commit refused with BFD running: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.refusal) {
				t.Fatalf("commit error = %v, want %q", err, tc.refusal)
			}
			for _, iface := range []string{"ospf interface eth0", "interface eth2"} {
				if !strings.Contains(err.Error(), iface) {
					t.Errorf("commit error = %v, want %q named", err, iface)
				}
			}
			if strings.Contains(err.Error(), "eth1") {
				t.Errorf("commit error = %v names eth1, which does not opt in", err)
			}
		})
	}
}
