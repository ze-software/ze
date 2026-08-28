package integration

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/lepath"
)

func TestStressBirdRoundsRemainTheBaselineShape(t *testing.T) {
	want := []stressBirdRound{
		{prefixes: 100_000, prefixBase: "10.0.0.0/24", peerWait: 120 * time.Second},
		{prefixes: 250_000, prefixBase: "10.64.0.0/24", peerWait: 180 * time.Second},
		{prefixes: 500_000, prefixBase: "10.128.0.0/24", peerWait: 300 * time.Second},
		{prefixes: 1_000_000, prefixBase: "11.0.0.0/24", peerWait: 600 * time.Second},
	}
	if !reflect.DeepEqual(stressBirdRounds[:], want) {
		t.Fatalf("BIRD rounds = %#v, want %#v", stressBirdRounds, want)
	}
}

func TestStressBirdActionMetadataAndConfiguration(t *testing.T) {
	if StressBirdAction != "stress-bird" || stressBirdScenario != "04-bulk-ipv4-bird" {
		t.Fatalf("native identity = (%q, %q)", StressBirdAction, stressBirdScenario)
	}
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve checkout: %v", err)
	}
	config, err := os.ReadFile(filepath.Join(
		root, "test", "stress", "scenarios", stressBirdScenario, "bird.conf",
	))
	if err != nil {
		t.Fatalf("read BIRD configuration: %v", err)
	}
	wantConfig := `log stderr all;
router id 172.31.0.2;

protocol device {
}

protocol bgp injector {
    local 172.31.0.2 as 65001;
    neighbor 172.31.0.3 as 65100;
    passive;
    hold time 90;

    ipv4 {
        import all;
        export none;
        receive limit 2000000;
    };
}
`
	if string(config) != wantConfig {
		t.Fatalf("BIRD configuration changed:\n%s", config)
	}
}

func TestStressBirdRecordedEffectsAreNonVacuous(t *testing.T) {
	recorder := newStressBirdRecorder()
	recorder.routeCounts = []int{100_000, 250_000, 500_000, 1_000_000}
	report, code := runRecordedStressBird(t, recorder)
	if code != 0 || len(report.Rounds) != 4 {
		t.Fatalf("recorded BIRD run = code %d report %#v", code, report)
	}
	for _, effect := range []string{
		"run ip netns add ze-stress-ze-fixture",
		"run ip netns add ze-stress-bb-fixture",
		"start ip netns exec ze-stress-ze-fixture bird -f",
		"start ip netns exec ze-stress-bb-fixture /repo/bin/ze-test peer",
		"run ip netns exec ze-stress-ze-fixture birdc -s",
		"run ip netns del ze-stress-ze-fixture",
		"run ip netns del ze-stress-bb-fixture",
	} {
		if !anyStressBirdEvent(recorder.events, effect) {
			t.Fatalf("native recording omitted %q; events = %v", effect, recorder.events)
		}
	}
}
