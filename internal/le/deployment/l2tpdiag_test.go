package deployment

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/leaction"
)

func TestPPPoXDiagnosticDefaultsMatchTheProducer(t *testing.T) {
	got, err := parsePPPoXDiagnosticArguments(leaction.Arguments{})
	if err != nil {
		t.Fatalf("parse defaults: %v", err)
	}
	want := l2tpDiagnosticOptions{
		Local: [4]byte{0, 0, 0, 0}, Remote: [4]byte{127, 0, 0, 1},
		SourcePort: 1701, DestinationPort: 1701,
		TunnelID: 1, PeerTunnelID: 100, SessionID: 1, PeerSessionID: 100,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("defaults changed:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestTunnelDiagnosticDefaultsMatchTheProducer(t *testing.T) {
	got, err := parseTunnelDiagnosticArguments(leaction.Arguments{})
	if err != nil {
		t.Fatalf("parse defaults: %v", err)
	}
	want := l2tpDiagnosticOptions{
		Local: [4]byte{172, 30, 0, 1}, Remote: [4]byte{172, 30, 0, 2},
		SourcePort: 1701, DestinationPort: 1702, TunnelID: 1, PeerTunnelID: 100,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("defaults changed:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestTunnelDiagnosticAcceptsEveryEncodingBoundary(t *testing.T) {
	got, err := parseTunnelDiagnosticArguments(leaction.Arguments{
		"local": "0.0.0.0", "remote": "255.255.255.255",
		"source-port": "0", "destination-port": "65535",
		"tunnel-id": "0", "peer-tunnel-id": "4294967295",
	})
	if err != nil {
		t.Fatalf("parse boundaries: %v", err)
	}
	if got.SourcePort != 0 || got.DestinationPort != math.MaxUint16 ||
		got.TunnelID != 0 || got.PeerTunnelID != math.MaxUint32 {
		t.Fatalf("boundary values changed: %#v", got)
	}
}

func TestPPPoXDiagnosticAcceptsEveryPackedIDBoundary(t *testing.T) {
	got, err := parsePPPoXDiagnosticArguments(leaction.Arguments{
		"tunnel-id": "0", "peer-tunnel-id": "65535",
		"session-id": "0", "peer-session-id": "65535",
	})
	if err != nil {
		t.Fatalf("parse boundaries: %v", err)
	}
	if got.TunnelID != 0 || got.PeerTunnelID != math.MaxUint16 ||
		got.SessionID != 0 || got.PeerSessionID != math.MaxUint16 {
		t.Fatalf("packed boundaries changed: %#v", got)
	}
}

func TestDiagnosticArgumentsRefuseMalformedValuesBeforeThePlatformRuns(t *testing.T) {
	cases := []struct {
		name string
		args leaction.Arguments
		want string
	}{
		{name: "IPv6", args: leaction.Arguments{"local": "::1"}, want: "dotted-quad"},
		{name: "trailing address data", args: leaction.Arguments{"remote": "127.0.0.1x"}, want: "dotted-quad"},
		{name: "negative port", args: leaction.Arguments{"source-port": "-1"}, want: "unsigned 16-bit"},
		{name: "large port", args: leaction.Arguments{"destination-port": "65536"}, want: "unsigned 16-bit"},
		{name: "large PPP tunnel", args: leaction.Arguments{"tunnel-id": "65536"}, want: "unsigned 16-bit"},
		{name: "large session", args: leaction.Arguments{"session-id": "65536"}, want: "unsigned 16-bit"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := parsePPPoXDiagnosticArguments(test.args)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want text %q", err, test.want)
			}
		})
	}
}

func TestTunnelDiagnosticKeepsTheProtocolV3IDWidth(t *testing.T) {
	if _, err := parseTunnelDiagnosticArguments(leaction.Arguments{"tunnel-id": "4294967296"}); err == nil {
		t.Fatal("accepted an ID wider than its netlink attribute")
	}
}

func TestDiagnosticExitMappingRejectsTheZeroVerdict(t *testing.T) {
	boom := errors.New("setup failed")
	cases := []struct {
		verdict L2TPDiagnosticVerdict
		err     error
		want    int
	}{
		{verdict: "", want: 1},
		{verdict: L2TPDiagnosticWorking, want: 0},
		{verdict: L2TPDiagnosticFailed, want: 1},
		{verdict: L2TPDiagnosticWorking, err: boom, want: 1},
		{verdict: "unknown", want: 1},
	}
	for _, test := range cases {
		if got := diagnosticExitCode(test.verdict, test.err); got != test.want {
			t.Errorf("diagnosticExitCode(%q, %v) = %d, want %d", test.verdict, test.err, got, test.want)
		}
	}
}

func TestDiagnosticActionsAreGatelessKeywordCommands(t *testing.T) {
	list := Actions()
	for _, verb := range []string{l2tpPPPoXDiagnosticName, l2tpTunnelDiagnosticName} {
		found := false
		for _, row := range list.Actions {
			if row.Verb != verb {
				continue
			}
			found = true
			if row.Gate != "" || len(row.Forks) != 0 {
				t.Fatalf("%s is not a native gateless action: %#v", verb, row)
			}
		}
		if !found {
			t.Fatalf("action %s is not listed", verb)
		}
	}
}

func TestDiagnosticReportTextIsTheProducerPage(t *testing.T) {
	report := L2TPDiagnosticReport{Output: "line one\nline two\n"}
	if got := report.Text(); got != report.Output {
		t.Fatalf("Text() = %q, want %q", got, report.Output)
	}
}
