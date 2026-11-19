// Design: docs/functional-tests.md -- compiled observers refuse incomplete answers.
// Related: blackhole_honor.go -- typed replacement for the three live CLI assertions.

package fixture

import (
	"encoding/json"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

const blackholeHonorRIBControl = `[
 {"prefix":"1.1.1.0/24","family":"ipv4/unicast","protocol":"static","priority":10,"route-type":"unicast"},
 {"prefix":"10.0.0.1/32","family":"ipv4/unicast","protocol":"bgp","priority":200,"next-hop":"1.1.1.1","route-type":"blackhole"},
 {"prefix":"192.0.2.1/32","family":"ipv4/unicast","protocol":"bgp","priority":200,"next-hop":"1.1.1.1"},
 {"prefix":"10.0.0.2/32","family":"ipv4/unicast","protocol":"bgp","priority":200,"next-hop":"1.1.1.1","route-type":"blackhole"}
]`

func TestBlackholeHonorRIBFailsClosed(t *testing.T) {
	for _, name := range []string{"valid", "missing-cover", "missing-wire-route", "unknown-prefix", "duplicate-prefix", "missing-family", "missing-protocol", "missing-priority", "outside-null-type", "outside-discard", "outside-empty-ecmp", "custom-community-ordinary", "missing-next-hop", "wrong-next-hop", "changed-cover"} {
		t.Run(name, func(t *testing.T) {
			var rows []blackholeHonorRIBRow
			if err := json.Unmarshal([]byte(blackholeHonorRIBControl), &rows); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "valid":
			case "missing-cover":
				rows = rows[1:]
			case "missing-wire-route":
				rows = rows[:3]
			case "unknown-prefix":
				rows[3].Prefix = "203.0.113.1/32"
			case "duplicate-prefix":
				rows[3] = rows[1]
			case "missing-family":
				rows[1].Family = family.Family{}
			case "missing-protocol":
				rows[1].Protocol = ""
			case "missing-priority":
				rows[1].Priority = nil
			case "outside-null-type":
				rows[2].RouteType = json.RawMessage(`null`)
			case "outside-discard":
				rows[2].RouteType = json.RawMessage(`"blackhole"`)
			case "outside-empty-ecmp":
				rows[2].ECMPPaths = json.RawMessage(`[]`)
			case "custom-community-ordinary":
				rows[3].RouteType = nil
			case "missing-next-hop":
				rows[2].NextHop = nil
			case "wrong-next-hop":
				rows[2].NextHop = json.RawMessage(`"192.0.2.254"`)
			case "changed-cover":
				rows[0].RouteType = json.RawMessage(`"blackhole"`)
			}
			indexed, err := blackholeHonorIndex(rows)
			if err == nil {
				err = blackholeHonorRoutes(indexed)
			}
			if name == "valid" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil {
				t.Fatal("incomplete or contradictory RIB answer passed")
			}
		})
	}
}
