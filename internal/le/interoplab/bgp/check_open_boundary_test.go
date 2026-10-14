package bgp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestOpenBoundaryHostMatchesScenario guards the scenario's size arithmetic:
// the checker's boundaryHost and the host the scenario's ze.conf configures
// are one value, 200 octets, and the domain is the 38 octets the header counts.
// Method: read the tracked ze.conf and look for both leaves verbatim.
func TestOpenBoundaryHostMatchesScenario(t *testing.T) {
	if len(boundaryHost) != 200 {
		t.Fatalf("boundaryHost is %d octets, the ze.conf arithmetic counts 200", len(boundaryHost))
	}
	path := filepath.Join("..", "..", "..", "..", "test", "interop", "scenarios", scenarioOpenExtendedParamsBoundaryFRR, "ze.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	text := string(content)
	if !strings.Contains(text, "host "+boundaryHost+";") {
		t.Fatalf("%s does not configure host %q", path, boundaryHost)
	}
	domain := "example" + strings.Repeat("d", 31)
	if !strings.Contains(text, "domain "+domain+";") {
		t.Fatalf("%s does not configure the 38-octet domain %q", path, domain)
	}
}

// TestFRRReceivedHostname reads the hostname out of FRR's neighbor JSON and
// refuses an absent or empty one, so the checker cannot pass on silence.
func TestFRRReceivedHostname(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		want   string
		wantOK bool
	}{
		{"present", `{"172.30.0.2":{"hostname":"zeboundaryhh","bgpState":"Established"}}`, "zeboundaryhh", true},
		{"absent", `{"172.30.0.2":{"bgpState":"Established"}}`, "", false},
		{"empty", `{"172.30.0.2":{"hostname":""}}`, "", false},
		{"not json", `no neighbor`, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := frrReceivedHostname(tc.input)
			if got != tc.want || ok != tc.wantOK {
				t.Fatalf("frrReceivedHostname = %q, %v; want %q, %v", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
