package fixture

// plugin_fixture_traceroute_source_af.go drives `resolve traceroute` with a
// source address of each family. The handler reads the source before it
// resolves the target, so the family of the source decides which address a
// dual-stack name resolves to and which socket the probe opens.
//
// The guest carries localhost in both families (Alpine ships /etc/hosts with
// 127.0.0.1 first and ::1 second), so a resolution that ignores the source
// answers 127.0.0.1 for both polarities: the IPv4 run then passes by accident
// and the IPv6 run fails at the bind. Asserting the hop address in both
// polarities is what tells the two apart.
//
// Design: docs/architecture/diagnostics/active-probes.md -- the source decides the family
// Related: register_traceroute_source_af.go -- the scenario name
// Related: plugin_fixture_13.go -- observe13, command13 and requireStatus13

import (
	"context"
	"fmt"
	"strings"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// tracerouteSourceAFProbe holds each traceroute to one probe on one hop, so the
// scenario measures which address family was chosen and never a path length.
const tracerouteSourceAFProbe = " max-hops 1 probes 1 timeout 2s"

func tracerouteSourceAF(ctx context.Context, _ []string) error {
	return observe13(ctx, "traceroute-source-af-test", func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := waitEORSent13(ctx, plugin, "peer1"); err != nil {
			return err
		}
		if err := tracerouteSourceAFPolarity(ctx, plugin, "::1"); err != nil {
			return err
		}
		if err := tracerouteSourceAFPolarity(ctx, plugin, "127.0.0.1"); err != nil {
			return err
		}
		return tracerouteSourceAFConflict(ctx, plugin)
	})
}

// tracerouteSourceAFPolarity traces the dual-stack name from source and checks
// the first hop is the loopback of the source's own family. The source and the
// wanted hop are one address because each loopback answers its own family only.
func tracerouteSourceAFPolarity(ctx context.Context, plugin *sdk.Plugin, source string) error {
	command := "resolve traceroute localhost source " + source + tracerouteSourceAFProbe
	result := command13(ctx, plugin, command)
	if err := requireStatus13(command, result, statusDone); err != nil {
		return err
	}
	addr, err := tracerouteSourceAFFirstHop(result)
	if err != nil {
		return fmt.Errorf("%s: %w", command, err)
	}
	if addr != source {
		return fmt.Errorf("%s: first hop %q, want %q: the source family did not drive the resolution", command, addr, source)
	}
	return nil
}

// tracerouteSourceAFConflict checks a target with no address in the source's
// family is refused with a message naming both arguments, and that the refusal
// arrives before any socket is opened.
func tracerouteSourceAFConflict(ctx context.Context, plugin *sdk.Plugin) error {
	command := "resolve traceroute 127.0.0.1 source ::1" + tracerouteSourceAFProbe
	result := command13(ctx, plugin, command)
	if result.status != statusError {
		return fmt.Errorf("%s: status=%s, want error: an IPv6 source cannot reach an IPv4 target", command, result.status)
	}
	message := string(result.raw) + fmt.Sprint(result.err)
	for _, want := range []string{"source ::1 is IPv6", "has no IPv6 address"} {
		if !strings.Contains(message, want) {
			return fmt.Errorf("%s: message %q does not name the conflict (%q)", command, message, want)
		}
	}
	if strings.Contains(message, "CAP_NET_RAW") {
		return fmt.Errorf("%s: the conflict surfaced as a socket failure: %s", command, message)
	}
	return nil
}

// tracerouteSourceAFFirstHop reads the address of hop 1 out of the response.
func tracerouteSourceAFFirstHop(result commandResult13) (string, error) {
	data := result.object()
	if data == nil {
		return "", fmt.Errorf("no response object: %.200s", result.raw)
	}
	hops, ok := data["hops"].([]any)
	if !ok || len(hops) == 0 {
		return "", fmt.Errorf("no hops in the response: %.200s", result.raw)
	}
	hop, ok := hops[0].(map[string]any)
	if !ok {
		return "", fmt.Errorf("hop 1 is %T, want an object: %.200s", hops[0], result.raw)
	}
	addr, ok := hop["addr"].(string)
	if !ok {
		return "", fmt.Errorf("hop 1 carries no addr: %.200s", result.raw)
	}
	return addr, nil
}
