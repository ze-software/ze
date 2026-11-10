// Design: docs/features/dns-resolver.md -- show system dns and the doctor checks over every dns-source selection
// Overview: register_l2tp_pool_dns_source.go -- the observers registered from these functions
// Related: plugin_fixture_06.go -- fixture06DispatchObject, the show doctor read

package fixture

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/dnssource"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The selections test/plugin/show-system-dns.ci configures: the LAN
// resolver's written list naming configured, and the pool's built-in one.
var (
	showDNSForwardServer = netip.MustParseAddr("198.51.100.53")
	showDNSPoolPrimary   = netip.MustParseAddr("203.0.113.53")
)

const (
	showDNSForwardPath = "service dns-resolver forward dns-source"
	showDNSPoolPath    = "l2tp pool ipv4 dns-source"
	// The pool path test/plugin/doctor-dns-source.ci's self list sits at.
	doctorDNSNamedPoolPath = "l2tp pool named-pool dns-source"
	operatorPollAttempts   = 120
	operatorPollDelay      = 250 * time.Millisecond
)

// showSystemDNSAnswer is the answer of show system dns.
type showSystemDNSAnswer struct {
	Records   []dnssource.Record      `json:"records"`
	Consumers []dnssource.Publication `json:"consumers"`
}

// showSystemDNSConsumers proves spec-home-cpe-23b AC-12 through the daemon
// dispatcher, the route every remote surface takes: show system dns answers
// the router's selection, the LAN resolver's written list with its class and
// server, and the pool's built-in list with the default marker. It polls,
// because each consumer publishes once its own config is applied.
func showSystemDNSConsumers(ctx context.Context, p *sdk.Plugin) error {
	var answer showSystemDNSAnswer
	var verdict error
	Poll(ctx, operatorPollAttempts, operatorPollDelay, func() bool {
		answer = showSystemDNSAnswer{}
		if _, err := Dispatch(ctx, p, "show system dns", &answer); err != nil {
			verdict = err
			return false
		}
		verdict = judgeSystemDNSAnswer(&answer)
		return verdict == nil
	})
	if verdict != nil {
		encoded, err := json.Marshal(answer)
		if err != nil {
			return fmt.Errorf("show system dns: %w", verdict)
		}
		return fmt.Errorf("show system dns: %w; last answer %s", verdict, encoded)
	}
	// Stderr: a plugin's stdout carries the plugin protocol.
	_, err := fmt.Fprintln(os.Stderr, "OK: show system dns answers the router, the LAN resolver's configured list and the pool's default list")
	return err
}

// judgeSystemDNSAnswer names the first expectation answer misses.
func judgeSystemDNSAnswer(answer *showSystemDNSAnswer) error {
	if showDNSSelection(answer, "system", "") == nil {
		return fmt.Errorf("no router selection (consumer system)")
	}
	forward := showDNSSelection(answer, "dns-resolver", showDNSForwardPath)
	if forward == nil {
		return fmt.Errorf("no dns-resolver selection at %q", showDNSForwardPath)
	}
	if forward.Default {
		return fmt.Errorf("%s carries the default marker, but the config writes it", showDNSForwardPath)
	}
	if err := judgeSelection(forward, dnssource.ClassConfigured, showDNSForwardServer); err != nil {
		return err
	}
	pool := showDNSSelection(answer, "l2tp-pool", showDNSPoolPath)
	if pool == nil {
		return fmt.Errorf("no l2tp-pool selection at %q", showDNSPoolPath)
	}
	if !pool.Default {
		return fmt.Errorf("%s lacks the default marker, but the config writes no dns-source", showDNSPoolPath)
	}
	if pool.Mode != dnssource.ModeFirstUsable.String() {
		return fmt.Errorf("%s mode %q, want the default's %q", showDNSPoolPath, pool.Mode, dnssource.ModeFirstUsable)
	}
	return judgeSelection(pool, dnssource.ClassConfigured, showDNSPoolPrimary)
}

// judgeSelection checks that selection used class alone and selected server alone.
func judgeSelection(selection *dnssource.ConsumerSelection, class dnssource.Class, server netip.Addr) error {
	if !slices.Equal(selection.Used, []dnssource.Class{class}) {
		return fmt.Errorf("%s used %v, want [%s]", selection.Path, selection.Used, class)
	}
	if !slices.Equal(selection.Servers, []netip.Addr{server}) {
		return fmt.Errorf("%s servers %v, want [%s]", selection.Path, selection.Servers, server)
	}
	return nil
}

// showDNSSelection finds consumer's selection at path; an empty path takes
// the consumer's first selection.
func showDNSSelection(answer *showSystemDNSAnswer, consumer, path string) *dnssource.ConsumerSelection {
	for i := range answer.Consumers {
		if answer.Consumers[i].Consumer != consumer {
			continue
		}
		for j := range answer.Consumers[i].Selections {
			selection := &answer.Consumers[i].Selections[j]
			if path == "" {
				return selection
			}
			if selection.Path == path {
				return selection
			}
		}
	}
	return nil
}

// doctorDNSSourceWant is one finding test/plugin/doctor-dns-source.ci provokes.
type doctorDNSSourceWant struct {
	code string
	path string
	fix  string
}

// doctorDNSSourceFindings proves the three dns-source doctor checks through
// show doctor, the daemon form of ze doctor (the local ze doctor holds an
// empty registry and does not run them): the pool's received-only list
// selects nothing (empty), its received-interface names an interface that
// holds no record (interface-silent), and the named pool's self list hands
// out the pool gateway with no LAN resolver listening there (self-unserved).
func doctorDNSSourceFindings(ctx context.Context, p *sdk.Plugin) error {
	wants := []doctorDNSSourceWant{
		{code: diagnostic.CodeDoctorDNSSourceEmpty, path: showDNSPoolPath, fix: "show system dns"},
		{code: diagnostic.CodeDoctorDNSSourceInterfaceSilent, path: showDNSPoolPath, fix: "show system dns"},
		// The named pool's gateway, which its self list hands out.
		{code: diagnostic.CodeDoctorDNSSourceSelfUnserved, path: doctorDNSNamedPoolPath, fix: "set service dns-resolver listener <name> ip 10.99.8.1"},
	}
	var missing []doctorDNSSourceWant
	var last map[string]any
	Poll(ctx, operatorPollAttempts, operatorPollDelay, func() bool {
		data, err := fixture06DispatchObject(ctx, p, "show doctor")
		if err != nil {
			missing = wants
			return false
		}
		last = data
		missing = doctorMissing(data, wants)
		return len(missing) == 0
	})
	if len(missing) != 0 {
		return fmt.Errorf("show doctor lacks %v; last answer %v", missing, last)
	}
	_, err := fmt.Fprintln(os.Stderr, "OK: show doctor reports doctor-dns-source-empty, doctor-dns-source-interface-silent and doctor-dns-source-self-unserved")
	return err
}

// doctorMissing answers the wants no diagnostic in data matches.
func doctorMissing(data map[string]any, wants []doctorDNSSourceWant) []doctorDNSSourceWant {
	diagnostics, ok := data["diagnostics"].([]any)
	if !ok {
		return wants
	}
	var missing []doctorDNSSourceWant
	for _, want := range wants {
		found := false
		for _, raw := range diagnostics {
			entry, isMap := raw.(map[string]any)
			if !isMap {
				continue
			}
			// The fix is the command alone under its own key, never a
			// "run ..." clause inside the message.
			if entry["code"] == want.code && entry["path"] == want.path && entry["fix"] == want.fix {
				found = true
			}
		}
		if !found {
			missing = append(missing, want)
		}
	}
	return missing
}
