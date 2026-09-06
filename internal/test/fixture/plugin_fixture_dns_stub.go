// Design: docs/functional-tests.md -- the two ze-test dns stub scenarios
// Related: internal/test/mock/dns/server.go -- the stub these fixtures drive

package fixture

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"strings"

	mdns "github.com/miekg/dns"

	dnsmock "github.com/ze-software/ze/internal/test/mock/dns"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// dnsStubAddress is where both scenarios serve. The port is 53 and not a
// negotiated one because `system name-server` is declared `type zt:ip-address`
// (internal/component/config/system/yang/ze-system-conf.yang) and carries no
// port, so a daemon pointed at a stub reaches it there or nowhere. Both `.ci`
// files therefore declare option=needs-linux:caps=net-bind and serialize on
// option=exclusive:group=dns-stub-port-53.
//
// Both scenarios need a suite that runs in the guest's own network namespace.
// dnsStubAnswerChange binds this address INSIDE the fixture, and a fixture is
// forked by ze: under the per-test netns launch mode ze is dropped to an
// ordinary uid (runOrchestrated, internal/test/runner/runner_exec.go) and only
// the ze and ze-stripped copies are given cap_net_bind_service
// (prepareNetnsBinaries, internal/le/qemu/netns_linux.go), so the fixture would
// inherit the uid without the capability and the bind would fail EACCES. The
// `plugin` suite declares Namespace: guestRoot (vmSuites,
// internal/le/qemu/alltests.go), which is what keeps that out of reach. A
// scenario moved to a per-test-namespace suite needs the port seam instead.
const dnsStubAddress = "127.0.0.1:53"

// dnsStubLookupAttempts bounds the wait for the first answer. The stub is
// started by its own `cmd=background` line, so the daemon may ask before it
// listens; every later lookup is dispatched once and asserted immediately.
const dnsStubLookupAttempts = 40

// dnsStubLookup asserts the daemon reads the stub's zone through the operator
// command, for both families and for a name that does not exist.
//
// The stub is the separate `ze-test dns --port 53` process the `.ci` file
// starts, so this scenario proves the subcommand serves a real daemon.
func dnsStubLookup(ctx context.Context, p *sdk.Plugin) error {
	// The first lookup waits for the stub, which its own `cmd=background` line
	// starts. A daemon that reached it answers with the zone's address; one that
	// did not carries an error field.
	const first = "show dns lookup web.example.test type A"
	data, err := fixture06PollObject(ctx, p, first, dnsStubLookupAttempts,
		func(answer map[string]any) bool { return dnsRecordsEqual(answer, "203.0.113.10") })
	if err != nil {
		return fmt.Errorf("the daemon never resolved web.example.test through the stub: %w", err)
	}
	fmt.Fprintf(os.Stderr, "OK: %s -> %s\n", first, dnsAnswerLine(data))

	for _, test := range []struct {
		command string
		records []string
		status  string
	}{
		{"show dns lookup web.example.test type AAAA", []string{"2001:db8::10"}, ""},
		{"show dns lookup v4only.example.test type A", []string{"198.51.100.10"}, ""},
		// A name that holds an A record and no AAAA answers NOERROR with no
		// records (RFC 2308 Section 2.2). handleDNSLookup
		// (internal/component/resolve/cmd/show_dns.go) writes the status field
		// only for a non-success answer, so its ABSENCE here is what says the
		// stub answered NODATA rather than NXDOMAIN. Reading NXDOMAIN for this
		// name is what would empty a live firewall set.
		{"show dns lookup v4only.example.test type AAAA", nil, ""},
		{"show dns lookup absent.example.test type A", nil, "NXDOMAIN"},
	} {
		answer, dispatchErr := fixture06DispatchObject(ctx, p, test.command)
		if dispatchErr != nil {
			return dispatchErr
		}
		if checkErr := dnsCheckAnswer(test.command, answer, test.records, test.status); checkErr != nil {
			return checkErr
		}
		fmt.Fprintf(os.Stderr, "OK: %s -> %s\n", test.command, dnsAnswerLine(answer))
	}
	return nil
}

// dnsStubAnswerChange asserts a daemon reads the NEW address after the stub is
// reprogrammed while it runs, with no cache-clearing command in between.
//
// The zone answers with TTL 0, and cache.put
// (internal/component/resolve/dns/cache.go) returns before storing a zero-TTL
// answer, so the second lookup reaches the stub rather than the daemon's cache.
// The stub runs in this process, which is what a `cmd=background` process with
// no control channel cannot do.
func dnsStubAnswerChange(ctx context.Context, p *sdk.Plugin) error {
	server, err := dnsmock.Start(dnsStubAddress)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := server.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "dns stub close: %v\n", closeErr)
		}
	}()
	fmt.Fprintf(os.Stderr, "OK: dns stub listening on %s\n", server.Addr())

	const command = "show dns lookup web.example.test type A"

	before, err := fixture06PollObject(ctx, p, command, dnsStubLookupAttempts,
		func(answer map[string]any) bool { return dnsRecordsEqual(answer, "203.0.113.10") })
	if err != nil {
		return fmt.Errorf("the daemon never resolved web.example.test through the stub: %w", err)
	}
	fmt.Fprintf(os.Stderr, "OK: before the change -> %s\n", dnsAnswerLine(before))

	server.Set("web.example.test", mdns.TypeA, dnsmock.Answer{
		Addresses: []netip.Addr{netip.MustParseAddr("203.0.113.11")},
	})

	after, err := fixture06DispatchObject(ctx, p, command)
	if err != nil {
		return err
	}
	if checkErr := dnsCheckAnswer(command, after, []string{"203.0.113.11"}, ""); checkErr != nil {
		return fmt.Errorf("%w -- the daemon answered from its cache, so the zero TTL did not reach cache.put", checkErr)
	}
	fmt.Fprintf(os.Stderr, "OK: after the change -> %s\n", dnsAnswerLine(after))
	return nil
}

// dnsCheckAnswer compares one `show dns lookup` answer with what the zone says.
// An empty status means the field must be ABSENT, which is how a NOERROR answer
// arrives.
func dnsCheckAnswer(command string, data map[string]any, records []string, status string) error {
	if raw, held := data["error"]; held {
		return fmt.Errorf("%s: error=%v", command, raw)
	}

	got, ok := fixture06StringSlice(data["records"])
	if !ok && data["records"] != nil {
		return fmt.Errorf("%s: records=%v, want a list of strings", command, data["records"])
	}
	if strings.Join(got, ",") != strings.Join(records, ",") {
		return fmt.Errorf("%s: records=%v, want %v", command, got, records)
	}

	raw, held := data["status"]
	if status == "" {
		if held {
			return fmt.Errorf("%s: status=%v, want no status field (a NOERROR answer carries none)", command, raw)
		}
		return nil
	}
	if !held || raw != status {
		return fmt.Errorf("%s: status=%v, want %s", command, raw, status)
	}
	return nil
}

// dnsAnswerLine renders one `show dns lookup` answer as the text the `.ci`
// assertions read. It normalizes both empty shapes the command produces -- a
// JSON null for no records and an absent status field for NOERROR -- so a
// needle never has to match on the spelling of a missing value.
func dnsAnswerLine(data map[string]any) string {
	records := "none"
	if got, ok := fixture06StringSlice(data["records"]); ok && len(got) > 0 {
		records = strings.Join(got, ",")
	}
	status := "NOERROR"
	if raw, held := data["status"]; held {
		status = fmt.Sprint(raw)
	}
	return "records " + records + " status " + status
}

// dnsRecordsEqual reports whether an answer carries exactly the given records.
// It is the predicate form of dnsCheckAnswer, for the poll that waits on the
// stub.
func dnsRecordsEqual(data map[string]any, records ...string) bool {
	return dnsCheckAnswer("", data, records, "") == nil
}
