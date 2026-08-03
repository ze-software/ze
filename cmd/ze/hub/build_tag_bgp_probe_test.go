// Design: ai/rules/plugins.md -- shared ze_bgp present/absent probe config

package hub

// bgpAbsenceProbeConfig is the one config snippet both halves of the ze_bgp
// build-tag pair use: build_tag_bgp_present_test.go asserts it PARSES, and
// build_tag_bgp_absent_test.go asserts it is REJECTED as an unknown field.
//
// It lives in an untagged file because those two test files carry mutually
// exclusive constraints (ze_bgp / !ze_bgp) and so can never share a declaration.
// Keeping one literal is what makes the pair meaningful: a snippet that drifted
// into being syntactically invalid would still be "rejected" on the absent side
// and the compile-out proof would silently become vacuous. That is not
// hypothetical -- the first draft of this const used leaf names that do not
// exist, so the absent test passed while proving nothing, and only the present
// half caught it.
//
// The body mirrors test/parse/cli-config-archive-no-location.ci: a router-id,
// a local ASN and one peer with both ends of its connection.
const bgpAbsenceProbeConfig = `bgp {
	router-id 10.0.0.1
	session {
		asn {
			local 65533
		}
	}
	peer peer1 {
		connection {
			remote {
				ip 127.0.0.1
			}
			local {
				ip 127.0.0.1
			}
		}
		session {
			asn {
				remote 65534
			}
		}
	}
}
`
