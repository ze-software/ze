// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config

package bridgeplugin

import (
	"log/slog"
	"strings"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// bridgeSections is an exabgp root whose bridge runs one script per name in
// runs (name=command), over families, with route-refresh routeRefresh.
func bridgeSections(runs, families, routeRefresh string) []sdk.ConfigSection {
	var procs []string
	for entry := range strings.SplitSeq(runs, ",") {
		name, run, _ := strings.Cut(entry, "=")
		procs = append(procs, `"`+name+`":{"run":"`+run+`"}`)
	}
	return []sdk.ConfigSection{{Root: configRoot, Data: `{"exabgp":{"bridge":{"process":{` +
		strings.Join(procs, ",") + `},"route-refresh":"` + routeRefresh + `","family":[` + families + `]}}}`}}
}

// runningBridge returns a runner whose running config is the one sections
// carries, as Stage 2 leaves it.
func runningBridge(t *testing.T, sections []sdk.ConfigSection) *bridgeRunner {
	t.Helper()
	cfg, err := parseConfig(sections[0].Data)
	if err != nil {
		t.Fatalf("parse running config: %v", err)
	}
	return &bridgeRunner{log: slog.New(slog.DiscardHandler), config: cfg}
}

// TestBridgeVerifyRefusesRunningChange proves AC-8 (Q4 = e1): while the bridge
// runs, a reload that changes a script's run command, adds or removes a
// script, or edits the family list is refused at verify with a message naming
// each changed leaf path, saying it cannot change while ze runs, and giving
// the restart steps; an unchanged bridge and a capability-only edit (left to
// the composed capability check) are accepted by this verify.
// VALIDATES: spec-plugin-reload-applies-or-refuses row 27, AC-8.
// PREVENTS: the acked-and-dropped reload: with OnConfigure alone the SDK
// answered config-apply OK while the scripts kept running the old commands.
// MUTATION: make refuseRunningChange answer nil and every refusal row fails.
func TestBridgeVerifyRefusesRunningChange(t *testing.T) {
	running := bridgeSections("main=./a.py", `"ipv4/unicast"`, "true")
	cases := []struct {
		name      string
		candidate []sdk.ConfigSection
		want      []string
	}{
		{"unchanged", bridgeSections("main=./a.py", `"ipv4/unicast"`, "true"), nil},
		{"capability leaf only", bridgeSections("main=./a.py", `"ipv4/unicast"`, "false"), nil},
		{"run command changed", bridgeSections("main=./b.py", `"ipv4/unicast"`, "true"),
			[]string{"exabgp bridge process main cannot change while ze runs (plugin exabgp-bridge)"}},
		{"script added", bridgeSections("main=./a.py,second=./c.py", `"ipv4/unicast"`, "true"),
			[]string{"exabgp bridge process second cannot change while ze runs (plugin exabgp-bridge)"}},
		{"family edited and script removed", bridgeSections("other=./a.py", `"ipv6/unicast"`, "true"),
			[]string{
				"exabgp bridge process main cannot change",
				"exabgp bridge process other cannot change",
				"exabgp bridge family cannot change",
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := runningBridge(t, running).verifyReload(tc.candidate)
			if tc.want == nil {
				if err != nil {
					t.Fatalf("refused: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("accepted, want a refusal")
			}
			msg := err.Error()
			for _, line := range tc.want {
				if !strings.Contains(msg, line) {
					t.Errorf("refusal lacks %q:\n%s", line, msg)
				}
			}
			if !strings.HasSuffix(msg, sdk.RestartSteps()) {
				t.Errorf("refusal does not end with the restart steps:\n%s", msg)
			}
		})
	}
}

// TestBridgeVerifyWithoutExabgpSectionAccepts proves a delivery that carries
// no exabgp section leaves the bridge as it is, per the sdk.ConfigParser
// convention that an absent root means unchanged.
// VALIDATES: spec-plugin-reload-applies-or-refuses row 27.
// PREVENTS: an edit under another root refused in the bridge's name.
func TestBridgeVerifyWithoutExabgpSectionAccepts(t *testing.T) {
	r := runningBridge(t, bridgeSections("main=./a.py", `"ipv4/unicast"`, "true"))
	if err := r.verifyReload([]sdk.ConfigSection{{Root: "bgp", Data: `{"bgp":{}}`}}); err != nil {
		t.Fatalf("refused a delivery with no exabgp section: %v", err)
	}
}
