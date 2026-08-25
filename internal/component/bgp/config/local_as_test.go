package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// TestGlobalLocalASReadsTheSchemaPath verifies the speaker's own AS is read
// from the leaf the YANG schema declares it at.
//
// VALIDATES: a config that sets bgp/session/asn/local reaches the reactor as
// that number. That leaf is the only place ze-bgp-conf.yang declares the
// global AS, and the schema makes it mandatory.
//
// PREVENTS: AS 0 reported for every deployment. The lookup used to read
// bgp/local/as. The schema declares no leaf named `as`. Its only `local`
// container sits under `connection` and holds an IP endpoint. So the lookup
// matched no valid config, and the answer stayed 0.
//
// Reactor.Stats carried that 0 into `show bgp` as local-as, and the monitor
// dashboard printed AS 0. RFC 7607 reserves AS 0, and no speaker originates
// it.
func TestGlobalLocalASReadsTheSchemaPath(t *testing.T) {
	const input = `
bgp {
    router-id 192.0.2.1
    session {
        asn {
            local 65000
        }
    }
    peer edge-a {
        connection {
            remote {
                ip 127.0.0.2
            }
            local {
                ip 127.0.0.1
            }
        }
        session {
            asn {
                local 65000
                remote 65001
            }
        }
    }
}
`
	tree, err := config.ParseTreeWithYANG(input, nil)
	require.NoError(t, err)

	bgpContainer := tree.GetContainer("bgp")
	require.NotNil(t, bgpContainer, "the parsed tree has no bgp container")

	require.Equal(t, uint32(65000), globalLocalAS(bgpContainer))
}

// TestGlobalLocalASWithoutTheLeaf verifies a bgp container that declares no
// global AS answers 0 rather than guessing one.
//
// VALIDATES: the absent-leaf path, which the schema makes unreachable through
// a validated config but which the loader must still answer for.
//
// PREVENTS: a panic, or an AS invented from a peer's own asn/local. The
// caller treats 0 as "not configured". It MUST NOT become a number a peer
// can see.
func TestGlobalLocalASWithoutTheLeaf(t *testing.T) {
	const input = `
bgp {
    router-id 192.0.2.1
}
`
	tree, err := config.ParseTreeWithYANG(input, nil)
	require.NoError(t, err)

	bgpContainer := tree.GetContainer("bgp")
	require.NotNil(t, bgpContainer, "the parsed tree has no bgp container")

	require.Equal(t, uint32(0), globalLocalAS(bgpContainer))
}
