package config

import (
	"errors"
	"strings"
	"testing"
)

// TestRFC7950MandatoryRefusesBGPConfig proves RFC7950-7.6.5-1 at LoadConfig,
// the <copy-config> window: bgp, bgp/session and bgp/session/asn are
// non-presence containers, so bgp/session/asn/local has no ancestor that is
// not one and MUST exist whenever the bgp section exists.
//
// VALIDATES: a bgp section with no local ASN anywhere is refused, naming the
// leaf and the set command that supplies it; one with the leaf is accepted.
// PREVENTS: the walk stopping at the absent session container, which let the
// config load with no local ASN.
//
// Method: load the two configs through LoadConfig and read the refusal.
//
// RFC requirement: RFC7950-7.6.5-1 negative -- LoadConfig refuses a bgp section whose mandatory bgp/session/asn/local sits under absent non-presence containers, naming the path and the set command.
// RFC requirement: RFC7950-7.6.5-1 positive -- LoadConfig accepts the same section once bgp/session/asn/local is present.
func TestRFC7950MandatoryRefusesBGPConfig(t *testing.T) {
	_, err := LoadConfig("bgp {\n\trouter-id 192.0.2.1\n}\n", "test.conf", nil)
	if err == nil {
		t.Fatal("LoadConfig accepted a bgp section with no local ASN")
	}
	if !errors.Is(err, ErrCustomValidation) {
		t.Fatalf("refusal is not a validation refusal: %v", err)
	}
	for _, want := range []string{"bgp/session/asn/local", "`set bgp session asn local <value>`"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not carry %q", err, want)
		}
	}

	complete := "bgp {\n\trouter-id 192.0.2.1\n\tsession {\n\t\tasn {\n\t\t\tlocal 65000\n\t\t}\n\t}\n}\n"
	if _, err := LoadConfig(complete, "test.conf", nil); err != nil {
		t.Errorf("LoadConfig refused a bgp section carrying every mandatory leaf: %v", err)
	}
}

// TestMandatoryBGPLeafInheritedBySessions proves the owner's ruling of
// 2026-10-12: a global mandatory leaf that every session resolves from its
// group or carries itself counts as present, and one session without it
// brings the refusal back.
//
// VALIDATES: router-id and the local ASN supplied per peer or per group are
// accepted with no global value; a peer that resolves neither is refused.
// PREVENTS: refusing the per-peer and per-group configs Ze's inheritance was
// built for, and accepting a session left with no router-id.
//
// Method: LoadConfig over three configs.
func TestMandatoryBGPLeafInheritedBySessions(t *testing.T) {
	perPeer := `bgp {
	peer p1 {
		connection { remote { ip 192.0.2.2 } }
		session {
			asn { local 65000; remote 65001; }
			router-id 192.0.2.1
		}
	}
}
`
	if _, err := LoadConfig(perPeer, "test.conf", nil); err != nil {
		t.Errorf("a peer carrying router-id and local ASN was refused: %v", err)
	}

	perGroup := `bgp {
	group g {
		session {
			asn { local 65000; }
			router-id 192.0.2.1
		}
		peer p1 {
			connection { remote { ip 192.0.2.2 } }
			session { asn { remote 65001; } }
		}
	}
}
`
	if _, err := LoadConfig(perGroup, "test.conf", nil); err != nil {
		t.Errorf("a peer inheriting router-id and local ASN from its group was refused: %v", err)
	}

	oneWithout := `bgp {
	peer p1 {
		connection { remote { ip 192.0.2.2 } }
		session {
			asn { local 65000; remote 65001; }
			router-id 192.0.2.1
		}
	}
	peer p2 {
		connection { remote { ip 192.0.2.3 } }
		session { asn { local 65000; remote 65002; } }
	}
}
`
	_, err := LoadConfig(oneWithout, "test.conf", nil)
	if err == nil {
		t.Fatal("a peer resolving no router-id was accepted with no global router-id")
	}
	if !strings.Contains(err.Error(), "bgp/router-id") {
		t.Errorf("refusal %q does not name bgp/router-id", err)
	}
}
