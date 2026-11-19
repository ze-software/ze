// Design: docs/architecture/rib-transition.md -- admitted local origins use configured static preference.
// Related: test/plugin/bgp-best-route-origin-distance.ci -- AC-16 original-recipient proof.
package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/bgp-best-route-origin-distance", bgpBestRouteOriginDistanceDriver, StopsDaemon)
}

func bgpBestRouteOriginDistanceDriver(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("origin distance requires <BGP listen address> <active config file>")
	}
	return Observe(ctx, "bgp-selected-test", &sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := bgpBestRouteOriginDistance(ctx, plugin, args[0], args[1]); err != nil {
			return err
		}
		_, err := fmt.Fprintln(os.Stderr, "OK: configured origin preference and losing withdrawal preserve original learned output")
		return err
	})
}

// bgpBestRouteOriginDistance changes only the declared static distance while
// the same local and learned contributions remain admitted. The return to10
// proves the local loser was retained, without another API announcement.
func bgpBestRouteOriginDistance(ctx context.Context, plugin *sdk.Plugin, address, activeConfig string) error {
	source, target, err := bgpBestRouteConnect(ctx, plugin, address, 65001, 65002)
	if err != nil {
		return err
	}
	defer source.Close() //nolint:errcheck // Original sockets stay open through every assertion.
	defer target.Close() //nolint:errcheck // Original sockets stay open through every assertion.
	prefix, control := []byte{24, 10, 254, 16}, []byte{24, 10, 254, 20}
	learnedPath := []byte{2, 2, 0xfd, 0xe8, 0xfd, 0xe9}
	localPath := []byte{2, 1, 0xfd, 0xe8}
	for _, route := range []struct {
		text string
		nlri []byte
	}{{"10.254.16.0/24", prefix}, {"10.254.20.0/24", control}} {
		if err := bgpBestRouteSend(source, route.nlri, false, false); err != nil {
			return err
		}
		if err := rfc8212ReloadBest(ctx, plugin, route.text, addrLoopbackSecond); err != nil {
			return err
		}
		if err := bgpBestRouteExpect(target, route.nlri, nil, learnedPath); err != nil {
			return fmt.Errorf("initial learned %s: %w", route.text, err)
		}
	}
	// U must remain admitted through every preference change. Its eventual
	// withdrawal is the wire fence after removing the losing local P.
	unrelated := []byte{24, 10, 253, 16}
	if err := liveStagesRoute(ctx, plugin, addrLoopbackThird+" update text nhop 192.0.2.1 nlri add 10.253.16.0/24"); err != nil {
		return fmt.Errorf("unrelated admitted API control: %w", err)
	}
	initialUnrelated, err := atomicAggregatePolicyRead(target)
	if err != nil {
		return err
	}
	if err := bgpBestRouteOriginDistanceAnnouncement(initialUnrelated, unrelated, localPath); err != nil {
		return fmt.Errorf("initial unrelated admitted API output: %w", err)
	}
	unrelatedAttributes := append([]byte(nil), initialUnrelated.PathAttributes...)
	output, err := plugin.UpdateRouteResult(ctx, addrLoopbackThird,
		"update text origin igp nhop 192.0.2.3 nlri add 10.254.16.0/24")
	if err != nil {
		return fmt.Errorf("admit local P: %w", err)
	}
	if err := output.Err(); err != nil {
		return err
	}
	if output.Announced != 1 {
		return fmt.Errorf("local P announcement count=%d, want one selected announcement", output.Announced)
	}
	if output.Withdrawn != 0 {
		return fmt.Errorf("local P admission withdrew %d routes, want none", output.Withdrawn)
	}
	if len(output.Admissions) != 1 {
		return fmt.Errorf("local P admission count=%d, want one ordinary selected admission", len(output.Admissions))
	}
	admission := output.Admissions[0]
	if admission.Outcome != rpc.RouteAdmissionRetained {
		return fmt.Errorf("local P was not retained: %+v", admission)
	}
	if admission.Disposition != rpc.RouteAdmissionSelected {
		return fmt.Errorf("local P was not selected: %+v", admission)
	}
	if admission.Destination != addrLoopbackThird {
		return fmt.Errorf("local P admission destination=%s, want %s", admission.Destination, addrLoopbackThird)
	}
	if admission.Family != "ipv4/unicast" {
		return fmt.Errorf("local P admission family=%s, want ipv4/unicast", admission.Family)
	}
	if !bytes.Equal(admission.KeyNLRI, prefix) {
		return fmt.Errorf("local P admission native key=%x, want %x", admission.KeyNLRI, prefix)
	}
	if err := bgpBestRouteExpect(target, prefix, nil, localPath); err != nil {
		return fmt.Errorf("static10 did not prefer admitted local P over eBGP20: %w", err)
	}
	before, err := peerCounter03(ctx, plugin, addrLoopbackSecond, "updates-received")
	if err != nil {
		return err
	}
	data, err := os.ReadFile(activeConfig) //nolint:gosec // The .ci names this daemon's own active file.
	if err != nil {
		return err
	}
	schema, err := config.YANGSchema()
	if err != nil {
		return err
	}
	tree, err := config.NewParser(schema).Parse(string(data))
	if err != nil {
		return err
	}
	distances := tree.GetContainerPath("rib/distance")
	if distances == nil {
		return errors.New("origin distance configuration omitted rib/distance")
	}
	for _, step := range []struct {
		distance string
		path     []byte
	}{{"200", learnedPath}, {"10", localPath}, {"200", learnedPath}} {
		distances.Set("static", step.distance)
		if err := bgpBestRouteReload(ctx, plugin, activeConfig, config.Serialize(tree, schema)); err != nil {
			return fmt.Errorf("apply static distance %s: %w", step.distance, err)
		}
		// Preference publication renews every admitted-origin scope, including
		// U's unchanged winner. The two keys have no inter-prefix order.
		if err := bgpBestRouteOriginDistanceRenewal(target, prefix, step.path, unrelated, unrelatedAttributes); err != nil {
			return fmt.Errorf("static%s reselection without reannouncing either candidate: %w", step.distance, err)
		}
	}
	after, err := peerCounter03(ctx, plugin, addrLoopbackSecond, "updates-received")
	if err != nil {
		return err
	}
	if after != before {
		return fmt.Errorf("preference reselection required new source UPDATEs: %d -> %d", before, after)
	}
	if err := liveStagesRoute(ctx, plugin, addrLoopbackThird+" update text nlri del 10.254.16.0/24"); err != nil {
		return fmt.Errorf("withdraw losing local P: %w", err)
	}
	if err := liveStagesRoute(ctx, plugin, addrLoopbackThird+" update text nlri del 10.253.16.0/24"); err != nil {
		return err
	}
	if err := bgpBestRouteExpect(target, nil, unrelated, nil); err != nil {
		return fmt.Errorf("losing withdrawal changed learned P/K, or unrelated admitted U was lost: %w", err)
	}
	// Actual source withdrawals must still retire both useful learned routes.
	// An earlier spurious withdrawal or duplicate cannot hide before this fence.
	for _, route := range [][]byte{prefix, control} {
		if err := bgpBestRouteSend(source, route, false, true); err != nil {
			return err
		}
		if err := bgpBestRouteExpect(target, nil, route, nil); err != nil {
			return fmt.Errorf("learned survivor no longer followed its original source: %w", err)
		}
	}
	if err := quiesce(ctx, plugin); err != nil {
		return err
	}
	if err := bgpBestRouteAPI(ctx, plugin, target, "10.253.20.0/24", []byte{24, 10, 253, 20}, localPath); err != nil {
		return fmt.Errorf("final API fence on original recipient: %w", err)
	}
	return policyFinalSessions(ctx, plugin, 2)
}

// bgpBestRouteOriginDistanceRenewal requires the complete two-key publication,
// in either order. It consumes no unrelated, duplicate or withdrawal output.
func bgpBestRouteOriginDistanceRenewal(target net.Conn, prefix, path, unrelated, unrelatedAttributes []byte) error {
	var selectedSeen, unrelatedSeen bool
	for range 2 {
		update, err := atomicAggregatePolicyRead(target)
		if err != nil {
			return err
		}
		if len(update.WithdrawnRoutes) != 0 {
			return fmt.Errorf("preference publication withdrew %x, want no withdrawal", update.WithdrawnRoutes)
		}
		switch {
		case bytes.Equal(update.NLRI, prefix):
			if selectedSeen {
				return fmt.Errorf("preference publication duplicated selected prefix %x", prefix)
			}
			if err := bgpBestRouteOriginDistanceAnnouncement(update, prefix, path); err != nil {
				return err
			}
			selectedSeen = true
		case bytes.Equal(update.NLRI, unrelated):
			if unrelatedSeen {
				return fmt.Errorf("preference publication duplicated unrelated prefix %x", unrelated)
			}
			if !bytes.Equal(update.PathAttributes, unrelatedAttributes) {
				return fmt.Errorf("preference publication changed unrelated attributes=%x, want original %x", update.PathAttributes, unrelatedAttributes)
			}
			unrelatedSeen = true
		default:
			return fmt.Errorf("preference publication announced %x, want only %x and %x", update.NLRI, prefix, unrelated)
		}
	}
	if !selectedSeen {
		return errors.New("preference publication omitted the newly selected prefix")
	}
	if !unrelatedSeen {
		return errors.New("preference publication omitted the unrelated admitted origin")
	}
	return nil
}

// bgpBestRouteOriginDistanceAnnouncement preserves the selected-wire helper's
// exact native prefix, no-withdrawal, ORIGIN and two-octet AS_PATH assertions.
func bgpBestRouteOriginDistanceAnnouncement(update *message.Update, prefix, path []byte) error {
	if !bytes.Equal(update.NLRI, prefix) {
		return fmt.Errorf("wire NLRI=%x, want %x", update.NLRI, prefix)
	}
	if len(update.WithdrawnRoutes) != 0 {
		return fmt.Errorf("wire withdrew %x, want none", update.WithdrawnRoutes)
	}
	for _, expected := range []struct {
		code  attribute.AttributeCode
		value []byte
	}{{attribute.AttrOrigin, []byte{0}}, {attribute.AttrASPath, path}} {
		_, _, value, present := attribute.AttrFind(update.PathAttributes, expected.code)
		if !present {
			return fmt.Errorf("useful selected output omitted attribute %d", expected.code)
		}
		if !bytes.Equal(value, expected.value) {
			return fmt.Errorf("useful selected attribute %d=%x, want exact %x", expected.code, value, expected.value)
		}
	}
	return nil
}
