// Design: docs/guide/configuration.md -- Blackhole Honoring (RFC 7999).
// Related: sysrib_forwarding_capture_fixture.go -- acknowledged SDK route installation.
// Related: ../../../test/plugin/bgp-blackhole-honor.ci -- three unchanged wire announcements.

package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	blackholeHonorCover    = "1.1.1.0/24"
	blackholeHonorHop      = "1.1.1.1"
	blackholeHonorDevice   = "blackhole-fixture0"
	blackholeHonorInstance = 7999
	blackholeHonorInside   = "10.0.0.1/32"
	blackholeHonorOutside  = "192.0.2.1/32"
	blackholeHonorCustom   = "10.0.0.2/32"
)

func init() {
	Register("plugin/bgp-blackhole-honor", observer02("blackhole-honor-test", blackholeHonor), StopsDaemon)
}

// blackholeHonor supplies a real terminal path, not a BGP origination without
// a forwarding target. The named device is route metadata: this portable case
// checks the system RIB and makes no claim about a kernel device or FIB write.
func blackholeHonor(ctx context.Context, p *sdk.Plugin) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := sysribInstall(ctx, p, rpc.RouteInstallEntry{Protocol: protocolStatic,
		Instance: blackholeHonorInstance, AFI: 1, SAFI: 1, Prefix: blackholeHonorCover,
		Interface: blackholeHonorDevice, RouteType: 1}); err != nil {
		return err
	}
	last := errors.New("terminal cover has not been observed")
	if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
		last = blackholeHonorCoverState(ctx, p)
		return last == nil
	}) {
		return fmt.Errorf("blackhole next hop never acquired its terminal SDK cover: %w", last)
	}
	last = errors.New("the three wire routes have not been observed")
	if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
		rows, err := blackholeHonorReadRIB(ctx, p)
		if err != nil {
			last = err
			return false
		}
		last = blackholeHonorRoutes(rows)
		return last == nil
	}) {
		return fmt.Errorf("BLACKHOLE agreement did not reach the system RIB: %w", last)
	}
	_, err := fmt.Fprintln(os.Stderr, "OK: BLACKHOLE and operator community discard inside the authorized block; outside route stays ordinary through the verified SDK cover")
	return err
}

type blackholeHonorCoverRow struct {
	Prefix string                    `json:"prefix"`
	Family family.Family             `json:"family"`
	Paths  []blackholeHonorCoverPath `json:"paths"`
}

type blackholeHonorCoverPath struct {
	Source    string          `json:"source"`
	Instance  uint32          `json:"instance"`
	Interface string          `json:"interface"`
	NextHop   json.RawMessage `json:"next-hop"`
	Distance  uint8           `json:"distance"`
	Metric    *uint32         `json:"metric"`
	Selected  bool            `json:"selected"`
	Hidden    *bool           `json:"hidden"`
}

func blackholeHonorCoverState(ctx context.Context, p *sdk.Plugin) error {
	var candidates []blackholeHonorCoverRow
	status, err := Dispatch(ctx, p, "show rib candidates prefix "+blackholeHonorCover, &candidates)
	if err != nil {
		return err
	}
	if status != rpc.StatusDone {
		return fmt.Errorf("cover inspection answered %q instead of done", status)
	}
	if len(candidates) != 1 {
		return fmt.Errorf("cover inspection returned %d prefixes, want one", len(candidates))
	}
	row := &candidates[0]
	if row.Prefix != blackholeHonorCover {
		return fmt.Errorf("cover inspection returned prefix %q", row.Prefix)
	}
	if row.Family != family.IPv4Unicast {
		return fmt.Errorf("cover inspection returned family %s", row.Family)
	}
	if len(row.Paths) != 1 {
		return fmt.Errorf("cover inspection returned %d paths, want one", len(row.Paths))
	}
	path := &row.Paths[0]
	if path.Source != protocolStatic {
		return fmt.Errorf("cover source is %q, want static", path.Source)
	}
	if path.Instance != blackholeHonorInstance {
		return fmt.Errorf("cover instance is %d, want %d", path.Instance, blackholeHonorInstance)
	}
	if path.Interface != blackholeHonorDevice {
		return fmt.Errorf("cover device is %q, want %s", path.Interface, blackholeHonorDevice)
	}
	if len(path.NextHop) != 0 {
		return errors.New("terminal cover unexpectedly names a gateway")
	}
	if path.Distance != 10 {
		return fmt.Errorf("cover distance is %d, want the configured static default 10", path.Distance)
	}
	if path.Metric == nil {
		return errors.New("cover inspection omitted its metric")
	}
	if *path.Metric != 0 {
		return fmt.Errorf("terminal cover metric is %d, want zero", *path.Metric)
	}
	if !path.Selected {
		return errors.New("terminal cover is not selected")
	}
	if path.Hidden == nil {
		return errors.New("cover inspection omitted its hidden state")
	}
	if *path.Hidden {
		return errors.New("terminal cover is hidden")
	}
	metrics, err := p.RouteMetrics(ctx, []string{blackholeHonorHop})
	if err != nil {
		return err
	}
	if len(metrics.Distances) != 1 {
		return fmt.Errorf("next-hop inspection returned %d distances, want one", len(metrics.Distances))
	}
	if metrics.Distances[0] != (rpc.RouteMetric{Resolved: true}) {
		return fmt.Errorf("next hop %s has metric %+v, want resolved, nonrecursive and zero cost", blackholeHonorHop, metrics.Distances[0])
	}
	rows, err := blackholeHonorReadRIB(ctx, p)
	if err != nil {
		return err
	}
	return blackholeHonorCoverRIB(rows)
}

func blackholeHonorCoverRIB(rows map[string]blackholeHonorRIBRow) error {
	cover, present := rows[blackholeHonorCover]
	if !present {
		return errors.New("system RIB omitted the SDK cover")
	}
	if *cover.Priority != 10 {
		return fmt.Errorf("system cover priority is %d, want 10", *cover.Priority)
	}
	if len(cover.NextHop) != 0 {
		return errors.New("system cover unexpectedly names a gateway")
	}
	if len(cover.ECMPPaths) != 0 {
		return errors.New("terminal system cover unexpectedly has ecmp-paths")
	}
	return blackholeHonorType(&cover, "unicast")
}

type blackholeHonorRIBRow struct {
	Prefix    string          `json:"prefix"`
	Family    family.Family   `json:"family"`
	Protocol  string          `json:"protocol"`
	Priority  *int            `json:"priority"`
	NextHop   json.RawMessage `json:"next-hop"`
	RouteType json.RawMessage `json:"route-type"`
	ECMPPaths json.RawMessage `json:"ecmp-paths"`
}

func blackholeHonorReadRIB(ctx context.Context, p *sdk.Plugin) (map[string]blackholeHonorRIBRow, error) {
	var rows []blackholeHonorRIBRow
	status, err := Dispatch(ctx, p, "show rib", &rows)
	if err != nil {
		return nil, err
	}
	if status != rpc.StatusDone {
		return nil, fmt.Errorf("system RIB inspection answered %q instead of done", status)
	}
	return blackholeHonorIndex(rows)
}

func blackholeHonorIndex(rows []blackholeHonorRIBRow) (map[string]blackholeHonorRIBRow, error) {
	indexed := make(map[string]blackholeHonorRIBRow, len(rows))
	for _, row := range rows {
		protocol := namespaceBGP
		switch row.Prefix {
		case blackholeHonorCover:
			protocol = protocolStatic
		case blackholeHonorInside, blackholeHonorOutside, blackholeHonorCustom:
		default:
			return nil, fmt.Errorf("system RIB returned unknown prefix %q", row.Prefix)
		}
		if _, duplicate := indexed[row.Prefix]; duplicate {
			return nil, fmt.Errorf("system RIB duplicated %s", row.Prefix)
		}
		if row.Family != family.IPv4Unicast {
			return nil, fmt.Errorf("system RIB %s has family %s", row.Prefix, row.Family)
		}
		if row.Protocol != protocol {
			return nil, fmt.Errorf("system RIB %s has protocol %q, want %s", row.Prefix, row.Protocol, protocol)
		}
		if row.Priority == nil {
			return nil, fmt.Errorf("system RIB %s omitted its priority", row.Prefix)
		}
		if *row.Priority < 0 {
			return nil, fmt.Errorf("system RIB %s has negative priority %d", row.Prefix, *row.Priority)
		}
		indexed[row.Prefix] = row
	}
	return indexed, nil
}

func blackholeHonorRoutes(rows map[string]blackholeHonorRIBRow) error {
	if len(rows) != 4 {
		return fmt.Errorf("system RIB returned %d routes, want the cover and three wire routes", len(rows))
	}
	if err := blackholeHonorCoverRIB(rows); err != nil {
		return err
	}
	for _, prefix := range []string{blackholeHonorInside, blackholeHonorOutside, blackholeHonorCustom} {
		row, present := rows[prefix]
		if !present {
			return fmt.Errorf("system RIB omitted wire route %s", prefix)
		}
		var hop string
		if err := json.Unmarshal(row.NextHop, &hop); err != nil {
			return fmt.Errorf("system RIB %s next hop: %w", prefix, err)
		}
		if hop != blackholeHonorHop {
			return fmt.Errorf("system RIB %s next hop is %q, want %s", prefix, hop, blackholeHonorHop)
		}
		if prefix == blackholeHonorOutside {
			if len(row.RouteType) != 0 {
				return errors.New("outside BLACKHOLE route unexpectedly has route-type")
			}
			if len(row.ECMPPaths) != 0 {
				return errors.New("outside BLACKHOLE route unexpectedly has ecmp-paths")
			}
			continue
		}
		if err := blackholeHonorType(&row, "blackhole"); err != nil {
			return err
		}
	}
	return nil
}

func blackholeHonorType(row *blackholeHonorRIBRow, want string) error {
	var got string
	if err := json.Unmarshal(row.RouteType, &got); err != nil {
		return fmt.Errorf("system RIB %s route-type: %w", row.Prefix, err)
	}
	if got != want {
		return fmt.Errorf("system RIB %s has route-type %q, want %s", row.Prefix, got, want)
	}
	return nil
}
