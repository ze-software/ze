package cli

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// TestPeerMissingFieldHintIsAnEditorCommand proves the missing-mandatory hint
// for a peer is a set command the editor takes, with the peer's key and, for a
// peer in a group, the group's, never the walk's slash path.
// Method: missingFieldIssue is called with the path under the peer the walk
// produces, for a standalone and a grouped peer, and the peer walk itself
// is driven through Validate on a peer that lacks connection remote ip; each
// message is compared whole.
func TestPeerMissingFieldHintIsAnEditorCommand(t *testing.T) {
	missing := yang.ValidationError{Type: yang.ErrTypeMissing, Path: "bgp/peer/connection/remote/ip"}
	under, inPeer := missingUnderPeer(missing)
	if !inPeer {
		t.Fatalf("%s is not read as a path under the peer", missing.Path)
	}
	field := strings.Split(under, "/")

	standalone := missingFieldIssue(0, "test_peer", "", field).Message
	wantStandalone := `peer test_peer: missing required field "connection/remote/ip" (set bgp peer test_peer connection remote ip <value>)`
	if standalone != wantStandalone {
		t.Errorf("standalone peer:\n got %s\nwant %s", standalone, wantStandalone)
	}

	grouped := missingFieldIssue(0, "test_peer", "upstream", field).Message
	wantGrouped := `peer test_peer: missing required field "connection/remote/ip" (set bgp group upstream peer test_peer connection remote ip <value>)`
	if grouped != wantGrouped {
		t.Errorf("grouped peer:\n got %s\nwant %s", grouped, wantGrouped)
	}

	v, err := NewConfigValidator()
	if err != nil {
		t.Fatal(err)
	}
	result := v.Validate(`bgp {
  router-id 1.1.1.1
  session {
    asn {
      local 65000
    }
  }
  peer test_peer {
    session {
      asn {
        remote 65001
      }
    }
  }
}`)
	found := false
	for _, w := range result.Warnings {
		if w.Message == wantStandalone {
			found = true
		}
	}
	if !found {
		t.Errorf("the peer walk produced no %q; warnings: %v", wantStandalone, result.Warnings)
	}
}
