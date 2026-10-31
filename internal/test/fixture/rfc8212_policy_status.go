// Design: docs/architecture/api/process-protocol.md -- effective policy through registered commands.
package fixture

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

type rfc8212PolicyProjection struct {
	Filters []any  `json:"filters"`
	Final   string `json:"final"`
	Source  string `json:"final-source"`
}

func rfc8212PolicyStatus(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("RFC8212 status observer needs the runner BGP port")
	}
	return Observe(ctx, "rfc8212-policy-status", &sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		if !waitPeersEOR03(ctx, plugin, 4) {
			return errors.New("status peers did not establish")
		}
		// Connect without sending OPEN. The real accept path creates a dynamic
		// peer, but no remote AS has been negotiated or guessed by this fixture.
		dialer := net.Dialer{LocalAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.6")}}
		conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.5", args[0]))
		if err != nil {
			return err
		}
		defer conn.Close() //nolint:errcheck // Closing the deliberately incomplete session is cleanup.
		if !Poll(ctx, 60, liveStagesPollDelay, func() bool {
			rows, err := peerRows03(ctx, plugin, "127.0.0.6")
			return err == nil && len(rows) == 1
		}) {
			return errors.New("dynamic accept did not publish the unresolved peer")
		}
		for _, row := range []struct{ peer, final, source, action string }{
			{addrLoopback, actionReject, "rfc8212", actionReject},
			{addrLoopbackSecond, actionAccept, "", actionAccept},
			{addrLoopbackThird, actionReject, "", actionReject},
			{addrLoopbackFourth, "", "", actionAccept},
			{"127.0.0.6", rfc8212VerdictUndetermined, "", rfc8212VerdictUndetermined},
		} {
			for _, direction := range []string{rfc8212DirectionImport, directionExport} {
				final := row.final
				if row.peer == "127.0.0.6" && direction == rfc8212DirectionImport {
					final = actionAccept
				}
				if err := rfc8212PolicyStatusDirection(ctx, plugin, row.peer, direction, final, row.source, row.action); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func rfc8212PolicyProjectionCheck(policy *rfc8212PolicyProjection, final, source string) error {
	if policy == nil {
		if final == "" {
			return nil
		}
		return fmt.Errorf("missing effective policy: want final=%s source=%s", final, source)
	}
	if policy.Final != final {
		return fmt.Errorf("policy final=%q, want %q", policy.Final, final)
	}
	if policy.Source != source {
		return fmt.Errorf("policy source=%q, want %q", policy.Source, source)
	}
	return nil
}

func rfc8212PolicyStatusDirection(ctx context.Context, plugin *sdk.Plugin, peer, direction, final, source, action string) error {
	for _, suffix := range []string{"", " | json"} {
		var chain struct {
			Chains []struct {
				Import *rfc8212PolicyProjection `json:"import"`
				Export *rfc8212PolicyProjection `json:"export"`
			} `json:"chains"`
		}
		command := "show policy chain peer " + peer + " " + direction + suffix
		status, err := Dispatch(ctx, plugin, command, &chain)
		if err != nil {
			return fmt.Errorf("%s: %w", command, err)
		}
		if status != statusDone {
			return fmt.Errorf("%s status=%s", command, status)
		}
		if len(chain.Chains) != 1 {
			return fmt.Errorf("%s chains=%v", command, chain.Chains)
		}
		policy := chain.Chains[0].Import
		if direction == directionExport {
			policy = chain.Chains[0].Export
		}
		if err := rfc8212PolicyProjectionCheck(policy, final, source); err != nil {
			return fmt.Errorf("%s: %w", command, err)
		}
		if direction == rfc8212DirectionImport {
			if policy == nil {
				return fmt.Errorf("%s lost automatic loop filter", command)
			}
			if len(policy.Filters) == 0 {
				return fmt.Errorf("%s lost automatic loop filter", command)
			}
		}
	}
	var detail struct {
		Peers map[string]struct {
			Import *rfc8212PolicyProjection `json:"import-policy"`
			Export *rfc8212PolicyProjection `json:"export-policy"`
		} `json:"peers"`
	}
	status, err := Dispatch(ctx, plugin, "show bgp peer "+peer+" detail", &detail)
	if err != nil {
		return err
	}
	if status != statusDone {
		return fmt.Errorf("detail %s status=%s", peer, status)
	}
	row, exists := detail.Peers[peer]
	if !exists {
		return fmt.Errorf("detail omitted %s", peer)
	}
	policy := row.Import
	if direction == directionExport {
		policy = row.Export
	}
	if err := rfc8212PolicyProjectionCheck(policy, final, source); err != nil {
		return fmt.Errorf("detail %s %s: %w", peer, direction, err)
	}
	for _, update := range []struct{ name, wire, action string }{
		{"announce", "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002F02000000144001010040020602010000FDE9400304C0000201180A5201", action},
		{"withdraw", "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001B020004180A52C80000", actionAccept},
		{"mixed", "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0033020004180A52C800144001010040020602010000FDE9400304C0000201180A5201", rfc8212ActionModify},
	} {
		want := update.action
		if action == rfc8212VerdictUndetermined {
			want = action
		}
		if update.name == "mixed" && action == actionAccept {
			want = actionAccept
		}
		var result struct {
			Action string                   `json:"action"`
			Text   string                   `json:"text-after"`
			Trace  []any                    `json:"trace"`
			Policy *rfc8212PolicyProjection `json:"policy"`
		}
		command := "show policy test peer " + peer + " " + direction + " update " + update.wire
		status, err := Dispatch(ctx, plugin, command, &result)
		if err != nil {
			return err
		}
		if status != statusDone {
			return fmt.Errorf("%s status=%s", command, status)
		}
		if result.Action != want {
			return fmt.Errorf("%s %s %s action=%s, want %s", peer, direction, update.name, result.Action, want)
		}
		if err := rfc8212PolicyProjectionCheck(result.Policy, final, source); err != nil {
			return fmt.Errorf("dryrun %s %s: %w", peer, direction, err)
		}
		if want == rfc8212VerdictUndetermined {
			if len(result.Trace) != 0 {
				return errors.New("unknown AS executed filters")
			}
			if result.Text != "" {
				return errors.New("unknown AS fabricated a resulting route")
			}
		}
		if want == actionReject && result.Text != "" {
			return fmt.Errorf("denied announcement survived: %s", result.Text)
		}
		if want == rfc8212ActionModify {
			if strings.Contains(result.Text, prefix82First) {
				return fmt.Errorf("denied mixed announcement survived: %s", result.Text)
			}
			if !strings.Contains(result.Text, "10.82.200.0/24") {
				return fmt.Errorf("mixed withdrawal disappeared: %s", result.Text)
			}
		}
		if update.name == "withdraw" && want == actionAccept && !strings.Contains(result.Text, "10.82.200.0/24") {
			return fmt.Errorf("explicit withdrawal disappeared: %s", result.Text)
		}
	}
	return nil
}
