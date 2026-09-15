package fixture

// plugin_fixture_ping_df.go drives `show ping ... do-not-fragment` from inside
// a daemon whose namespace sits behind a router that clamps the far link to
// 1400 octets. The .ci builds that topology; this scenario asks the three
// questions the DF mode exists to answer and reads the payload the daemon
// returns.
//
// A probe that fits the clamp is answered, so the path itself is proven before
// any refusal is read. A probe that exceeds it is refused by the router, and
// the payload names the reported MTU under `next-hop-mtu` with
// `next-hop-mtu-reported` true. The bypass-cache value gives the same answer,
// because the router refuses the probe on the wire whatever the kernel's cache
// holds. The keyword always carries its value: the RPC layer reads every
// declared leaf as keyword-then-value, so a bare `do-not-fragment` followed by
// another keyword is refused before any handler runs. The same scenario serves
// the privileged and the unprivileged daemon: AC-7 wants the same answer from
// both, so one body asserts it for both.
//
// Design: docs/architecture/diagnostics/active-probes.md -- the Don't Fragment mode
// Related: register_ping_df.go -- the scenario name
// Related: test/plugin/ping-do-not-fragment-reports-mtu.ci -- the privileged daemon
// Related: test/plugin/ping-do-not-fragment-unprivileged.ci -- the daemon without CAP_NET_RAW
// Related: plugin_fixture_13.go -- observe13, command13 and requireStatus13

import (
	"context"
	"fmt"
	"os"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	// pingDFFarAddr is the far namespace's address, two hops from the daemon.
	pingDFFarAddr = "10.99.2.1"
	// pingDFClampMTU is the MTU the router's far link is clamped to.
	pingDFClampMTU = 1400
	// pingDFNearMTU is the sender's own link MTU, which is the kernel's path
	// MTU estimate for the far address until a router reports a smaller one.
	pingDFNearMTU = 1600
	// pingDFFitsSize is a payload that fits the clamp: 1300 + 8 ICMP + 20 IP.
	pingDFFitsSize = "1300"
	// pingDFExceedsSize is a payload the clamp refuses: 1500 + 28 > 1400.
	pingDFExceedsSize = "1500"
	// pingDFProbe holds each ping to one probe so a refusal is one reply.
	pingDFProbe = " count 1 timeout 3s"
)

func pingDoNotFragment(ctx context.Context, _ []string) error {
	return observe13(ctx, "ping-df-test", func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := pingDFFits(ctx, plugin); err != nil {
			return err
		}
		if err := pingDFRefused(ctx, plugin, "do-not-fragment honor-cache"); err != nil {
			return err
		}
		if err := pingDFRefused(ctx, plugin, "do-not-fragment bypass-cache"); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "OK: a DF probe over the clamp reported next-hop-mtu 1400 in both modes, and one under it was answered")
		return nil
	})
}

// pingDFFits is the positive control: a DF probe under the clamp is answered,
// so a later refusal is the size and not the path. No router has reported
// yet, so the summary's path-mtu is the kernel's estimate before any
// refusal: the sender's own link MTU.
func pingDFFits(ctx context.Context, plugin *sdk.Plugin) error {
	command := "show ping " + pingDFFarAddr + " size " + pingDFFitsSize + " do-not-fragment honor-cache" + pingDFProbe
	data, err := pingDFResult(ctx, plugin, command)
	if err != nil {
		return err
	}
	reply, err := pingDFReplyOf(data)
	if err != nil {
		return fmt.Errorf("%s: %w", command, err)
	}
	if reply["status"] != "ok" {
		return fmt.Errorf("%s: reply status %v, want ok: the clamped path did not answer a probe that fits it", command, reply["status"])
	}
	if err := pingDFPathMTU(data, pingDFNearMTU); err != nil {
		return fmt.Errorf("%s: summary: %w", command, err)
	}
	return nil
}

// pingDFRefused sends a probe over the clamp with the given DF words and reads
// the router's answer off the reply and the summary. The router's answer
// also lowered the kernel's estimate, so the summary's path-mtu is the clamp.
func pingDFRefused(ctx context.Context, plugin *sdk.Plugin, words string) error {
	command := "show ping " + pingDFFarAddr + " size " + pingDFExceedsSize + " " + words + pingDFProbe
	data, err := pingDFResult(ctx, plugin, command)
	if err != nil {
		return err
	}
	reply, err := pingDFReplyOf(data)
	if err != nil {
		return fmt.Errorf("%s: %w", command, err)
	}
	if reply["status"] != "too-big" {
		return fmt.Errorf("%s: reply status %v, want too-big: %v", command, reply["status"], data)
	}
	if err := pingDFReportedMTU(reply); err != nil {
		return fmt.Errorf("%s: reply: %w", command, err)
	}
	if err := pingDFReportedMTU(data); err != nil {
		return fmt.Errorf("%s: summary: %w", command, err)
	}
	if err := pingDFPathMTU(data, pingDFClampMTU); err != nil {
		return fmt.Errorf("%s: summary: %w", command, err)
	}
	return nil
}

// pingDFPathMTU checks the summary carries path-mtu, the kernel's estimate
// for the far address once the probes have run, with the value want.
func pingDFPathMTU(summary map[string]any, want int) error {
	if _, present := summary["path-mtu"]; !present {
		return fmt.Errorf("path-mtu absent, want %d: %v", want, summary)
	}
	if mtu := number13(summary["path-mtu"]); mtu != want {
		return fmt.Errorf("path-mtu %d, want %d: %v", mtu, want, summary)
	}
	return nil
}

// pingDFReportedMTU checks one map, a reply or the summary, carries the
// reported MTU under both keys with the clamp's value.
func pingDFReportedMTU(m map[string]any) error {
	if m["next-hop-mtu-reported"] != true {
		return fmt.Errorf("next-hop-mtu-reported %v, want true: %v", m["next-hop-mtu-reported"], m)
	}
	if mtu := number13(m["next-hop-mtu"]); mtu != pingDFClampMTU {
		return fmt.Errorf("next-hop-mtu %d, want %d: %v", mtu, pingDFClampMTU, m)
	}
	return nil
}

// pingDFResult runs one show ping command and decodes its result.
func pingDFResult(ctx context.Context, plugin *sdk.Plugin, command string) (map[string]any, error) {
	result := command13(ctx, plugin, command)
	if err := requireStatus13(command, result, statusDone); err != nil {
		return nil, err
	}
	var data map[string]any
	if err := decodeJSON13(result.raw, &data); err != nil {
		return nil, fmt.Errorf("%s: decode: %w", command, err)
	}
	return data, nil
}

func pingDFReplyOf(data map[string]any) (map[string]any, error) {
	replies, ok := data["replies"].([]any)
	if !ok || len(replies) != 1 {
		return nil, fmt.Errorf("want one reply, got %v", data["replies"])
	}
	reply, ok := replies[0].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("reply is not an object: %v", replies[0])
	}
	return reply, nil
}
