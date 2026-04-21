// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS AAA backend
// Related: client.go -- RADIUS client used by the authenticator

package radius

import "codeberg.org/thomas-mangin/ze/internal/component/aaa"

const (
	aaaName     = "radius"
	aaaPriority = 50
)

type radiusBackend struct{}

func (radiusBackend) Name() string  { return aaaName }
func (radiusBackend) Priority() int { return aaaPriority }

// Build returns an empty Contribution because RADIUS AAA config is
// delivered via the L2TP plugin's YANG config, not the system AAA
// config tree. The l2tp-auth-radius plugin creates its own client.
// Registration reserves the name and priority slot.
func (radiusBackend) Build(_ aaa.BuildParams) (aaa.Contribution, error) {
	return aaa.Contribution{}, nil
}
