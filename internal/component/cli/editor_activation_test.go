package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// activationNexthopConfig carries a positional list entry (capability
// nexthop ipv4/unicast), whose children are all leaves.
const activationNexthopConfig = `bgp {
	session {
		asn {
			local 65000;
		}
	}
	router-id 1.2.3.4;
	peer peer1 {
		connection {
			remote {
				ip 1.1.1.1;
			}
		}
		session {
			asn {
				remote 65001;
			}
			capability {
				nexthop ipv4/unicast {
					nhafi ipv6;
					mode enable;
				}
			}
		}
	}
}
`

// VALIDATES: ApplyActivation, the one dispatch every editor and the offline
// `ze config deactivate` verb use, refuses a positional list entry and a path
// the schema does not know, in file mode and in session mode, and changes
// nothing.
// PREVENTS: the SSH and web editors handing a positional list entry to
// DeactivatePath, which only `ze config deactivate` refused before the two
// dispatches became one.
func TestApplyActivationRefusals(t *testing.T) {
	cases := []struct {
		name string
		path []string
		want string
	}{
		{"positional list entry", []string{"bgp", "peer", "peer1", "session", "capability", "nexthop", "ipv4/unicast"}, "positional list entry"},
		{"unknown path", []string{"bgp", "no-such-node"}, "path not found: bgp no-such-node"},
	}
	for _, mode := range []string{"file", "session"} {
		for _, tc := range cases {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				configPath := writeTestConfig(t, activationNexthopConfig)
				ed, err := NewEditorWithStorage(newTestTreeStore(t, configPath), configPath)
				require.NoError(t, err)
				t.Cleanup(func() { _ = ed.Close() })
				if mode == "session" {
					ed.SetSession(NewEditSession("thomas", "ssh"))
				}

				status, err := ed.ApplyActivation(tc.path, false)
				require.Error(t, err, "refused, got status %q", status)
				assert.Contains(t, err.Error(), tc.want)
				assert.False(t, ed.Dirty(), "a refused deactivate changes nothing")
			})
		}
	}
}
