package lg

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/report"
)

// recordingNames returns a server whose engine answers the name command as
// testServerWithNames does, and the commands it was sent.
func recordingNames(names map[uint32]string) (*LGServer, *[]string) {
	inner := testServerWithNames(names).dispatch
	var commands []string
	srv := testServerWithDispatch(func(ctx context.Context, caller plugin.CallerIdentity, cmd string) (*plugin.Response, error) {
		commands = append(commands, cmd)
		return inner(ctx, caller, cmd)
	})
	return srv, &commands
}

// TestASNamesAsksTheResolveCommand proves the looking glass names an AS by
// asking the resolve component's `resolve cymru asn-name` command through its
// dispatcher, in decimal whatever the notation the row carried, once per AS
// for one page part, and draws the answer.
//
// VALIDATES: the name drawn is the one the command answers; "(unknown)" draws
// the number alone; an empty or non-AS token asks nothing.
// PREVENTS: a private resolver path beside the shared command, an asdot
// spelling ("1.10") asked as a name Team Cymru does not hold, and one lookup
// per label when a graph labels a node twice.
func TestASNamesAsksTheResolveCommand(t *testing.T) {
	srv, commands := recordingNames(map[uint32]string{65546: "Example & Co"})
	names := srv.asNames()

	for _, text := range []string{"1.10", "65546"} {
		if got := names.name(text); got != "Example & Co" {
			t.Errorf("name(%q) = %q, want %q", text, got, "Example & Co")
		}
	}
	if got := names.name("64512"); got != "" {
		t.Errorf("name of an AS the command answers %q for = %q, want empty", ASNameUnknown, got)
	}
	for _, text := range []string{"", "not-an-as"} {
		if got := names.name(text); got != "" {
			t.Errorf("name(%q) = %q, want empty", text, got)
		}
	}
	names.done()

	want := []string{asNameCommand + "65546", asNameCommand + "64512"}
	if strings.Join(*commands, "|") != strings.Join(want, "|") {
		t.Errorf("commands = %q, want %q", *commands, want)
	}
}

// TestASNamesFailureLeavesTheNumberAndWarns proves a lookup the engine fails
// leaves the AS number without a name, and is reported once for the page
// part: one WARN line under the doctor code, and one report-bus warning
// whose fix is the command that shows the whole reason. A later page part
// that names an AS with no failure clears the warning.
//
// VALIDATES: the failure reaches the log and `show warnings`, not only the
// empty label.
// PREVENTS: a resolver failure dropped silently (the hub's old DecorateASN
// discarded it), and one log line per AS when the name servers are down.
func TestASNamesFailureLeavesTheNumberAndWarns(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	var log bytes.Buffer
	srv := testServerWithDispatch(func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
		// The resolve component's failure shape (errResponse, resolve.go).
		return &plugin.Response{Status: plugin.StatusError, Error: "team cymru name of AS65001: dns query AS65001.asn.cymru.com. TXT: no DNS server configured"}, nil
	})
	srv.logger = slog.New(slog.NewTextHandler(&log, &slog.HandlerOptions{Level: slog.LevelDebug}))

	peers := srv.extractPeers(map[string]any{"peers": []any{
		map[string]any{"peer-address": "10.0.0.1", "remote-as": "65001", "state": "established"},
		map[string]any{"peer-address": "10.0.0.2", "remote-as": "65002", "state": "established"},
	}})
	if len(peers) != 2 {
		t.Fatalf("expected 2 peers, got %d", len(peers))
	}
	for _, peer := range peers {
		if peer.RemoteASName != "" {
			t.Errorf("peer %s RemoteASName = %q, want empty after a failed lookup", peer.Address, peer.RemoteASName)
		}
		if peer.RemoteAS == "" {
			t.Errorf("peer %s lost its AS number", peer.Address)
		}
	}

	lines := strings.Split(strings.TrimSpace(log.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("log holds %d lines, want one for the table:\n%s", len(lines), log.String())
	}
	for _, part := range []string{"level=WARN", "code=" + asNameDoctorCode, "unnamed=2", "first-as=65001", "no DNS server configured", `fix="` + asNameCommand + `65001"`} {
		if !strings.Contains(lines[0], part) {
			t.Errorf("log line lacks %q: %s", part, lines[0])
		}
	}

	issue, raised := asNameWarning()
	if !raised {
		t.Fatalf("no report-bus warning raised: %+v", report.Warnings())
	}
	if issue.Fix != asNameCommand+"65001" {
		t.Errorf("warning fix = %q, want %q", issue.Fix, asNameCommand+"65001")
	}

	named := testServerWithNames(map[uint32]string{65001: "Test Org"})
	named.extractPeers(map[string]any{"peers": []any{
		map[string]any{"peer-address": "10.0.0.1", "remote-as": "65001", "state": "established"},
	}})
	if _, still := asNameWarning(); still {
		t.Errorf("the warning stayed after a table named its AS with no failure")
	}
}

// TestASNamesSpentBudgetAsksNothing proves a lookup asked after the page
// part's deadline is not dispatched, and counts as a failure.
//
// VALIDATES: asNameBudget bounds the page part, not each lookup.
// PREVENTS: a page that waits a budget for each AS number in turn.
func TestASNamesSpentBudgetAsksNothing(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	srv, commands := recordingNames(map[uint32]string{65001: "Test Org"})
	names := srv.asNames()
	names.cancel()

	if got := names.name("65001"); got != "" {
		t.Errorf("name after the deadline = %q, want empty", got)
	}
	if len(*commands) != 0 {
		t.Errorf("a lookup after the deadline was dispatched: %q", *commands)
	}
	if names.failed != 1 {
		t.Errorf("failed = %d, want 1", names.failed)
	}
	names.done()
}

// asNameWarning answers the looking glass's AS name warning on the report
// bus, when one is raised.
func asNameWarning() (report.Issue, bool) {
	for _, issue := range report.Warnings() {
		if issue.Source == asNameReportSource && issue.Code == asNameDoctorCode && issue.Subject == asNameReportSubject {
			return issue, true
		}
	}
	return report.Issue{}, false
}
