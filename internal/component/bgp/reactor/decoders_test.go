// Design: docs/architecture/wire/attributes.md -- carrier-aware tunnel validation.
// Related: rfc9012_receive_storage_test.go -- the storage tests these decoders serve.
// RFC naming: untagged -- this file holds no test, only the decoder imports the RFC 9012 storage tests in rfc9012_receive_storage_test.go need.

package reactor

// The RFC 9012 storage tests start a peer for every Section 6 carrier, and
// peer start refuses a configured family with no decoder ("family
// ipv4/mpls-label has no decoder plugin"). The composition root that
// registers every NLRI plugin, `internal/component/plugin/all`, is linked into
// this test binary only by `config_direction_test.go`, which carries
// `//go:build ze_bgp`, so a run under any other tag set (`-tags integration`
// in CI) registered none of them. These imports register the labeled, VPN and
// EVPN decoders in every build; unicast needs no plugin.
import (
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/evpn"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/labeled"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/vpn"
)
