// VALIDATES: the orchestrated .ci runner awaits foreground quick-exit ze
//            subcommands (config/show/format/...) but not daemons (`ze -`,
//            web servers, hub/start/cli/monitor), via isQuickExitZeCommand.
// PREVENTS: (1) the race where two un-awaited quick-exit ze steps share the
//            client stdout/stderr buffers and a later step clobbers an earlier
//            step's output (isis-config, format-operators); (2) the inverse
//            regression where a daemon (e.g. `ze --web ... --insecure-web`) is
//            mis-classified as quick-exit and awaited, hanging the loop forever.

package runner

import (
	"testing"
	"time"
)

// TestWithParallelHeadroom checks that per-test timeouts are widened only when
// the Run executes tests concurrently (concurrency > 1), leaving serial runs
// (-p 1 or a single selected test) and the unset/zero state untouched so a real
// slowdown still surfaces against the authored timeout.
func TestWithParallelHeadroom(t *testing.T) {
	const base = 10 * time.Second
	cases := []struct {
		name        string
		concurrency int
		want        time.Duration
	}{
		{"zero (outside a Run) unchanged", 0, base},
		{"serial run unchanged", 1, base},
		{"parallel run widened", 2, base * ParallelTimeoutHeadroom},
		{"high concurrency widened", 20, base * ParallelTimeoutHeadroom},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := &Runner{concurrency: c.concurrency}
			if got := r.withParallelHeadroom(base); got != c.want {
				t.Fatalf("concurrency=%d: got %v, want %v", c.concurrency, got, c.want)
			}
		})
	}
}

// TestFirstZeSubcommand checks that the ze verb is found past leading flags and
// that the daemon "read config from stdin" sentinel "-" is not treated as a verb.
func TestFirstZeSubcommand(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"plain verb", []string{"config", "validate", "-"}, "config"},
		{"leading flags skipped", []string{"-d", "--color", "doctor", "x.conf"}, "doctor"},
		{"bare dash is not a verb", []string{"-"}, ""},
		{"dash then nothing", []string{"--debug", "-"}, ""},
		{"isis-decode verb", []string{"isis-decode"}, "isis-decode"},
		{"empty args", nil, ""},
		{"web daemon", []string{"--web", "8080", "x.conf"}, "8080"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := firstZeSubcommand(c.args); got != c.want {
				t.Fatalf("firstZeSubcommand(%q) = %q, want %q", c.args, got, c.want)
			}
		})
	}
}

// TestIsQuickExitZeCommand verifies that quick-exit ze subcommands (awaited in
// the loop) are distinguished from daemon invocations (started, not awaited).
// The quick cases mirror real .ci command shapes across the isis/parse/ui
// suites; the daemon cases mirror the config-file invocations used by
// ldp/rsvpte/static/reload (`ze -`, `ze x.conf`) plus the explicit daemon verbs
// so a future edit cannot silently start awaiting a daemon (which would hang).
func TestIsQuickExitZeCommand(t *testing.T) {
	quick := [][]string{
		{"config", "validate", "-"},
		{"show", "bgp", "peer", "list"},
		{"explain", "doctor-isis-net-missing"},
		{"doctor", "--json", "isis-mismatch.conf"}, // .conf is an arg to a verb, not the daemon config
		{"isis-decode"},
		{"schema", "tree"},
		{"version"},
		{"completion", "bash"},
		{"bgp", "decode", "deadbeef"}, // offline bgp tool, not a daemon
		{"format", "json"},            // 14-step format-operators race source
		{"debug"},                     // ze debug help
		{"env"},
		{"run", "help"}, // run help is quick-exit, unlike a run daemon
		{"interface", "list"},
		{"-d", "config", "validate", "-"}, // leading flag, still quick
	}
	for _, a := range quick {
		if !isQuickExitZeCommand(a) {
			t.Errorf("isQuickExitZeCommand(%q) = false, want true", a)
		}
	}

	daemon := [][]string{
		{"-"},                                  // ze -  (config from stdin)
		{"x.conf"},                             // ze config-file
		{"ze-bgp.conf"},                        // bare config filename used by reload tests
		{"hub.conf"},                           // config file named hub.conf, not the hub verb
		{"--plugin", "ze.bgp-adj-rib-in", "-"}, // config from stdin with plugins
		{"--web", "8080", "x.conf"},            // web daemon with a config file
		{"--web", "8081", "--insecure-web"},    // web-server daemon, no config file (web-tool-decode)
		{"--pprof", "127.0.0.1:9000", "-"},     // pprof + config from stdin
		{"hub"},                                // explicit daemon verb
		{"start"},                              // explicit daemon verb
		{"cli"},                                // interactive, blocks on stdin
		{"monitor", "bgp"},                     // continuous streaming
	}
	for _, a := range daemon {
		if isQuickExitZeCommand(a) {
			t.Errorf("isQuickExitZeCommand(%q) = true, want false", a)
		}
	}
}

func TestSyncWriterCapsOutput(t *testing.T) {
	sw := &syncWriter{pattern: "needle"}

	// Fill to exactly the cap with a first write.
	half := make([]byte, maxOutputBytes/2)
	for i := range half {
		half[i] = 'a'
	}
	n1, err := sw.Write(half)
	if err != nil {
		t.Fatalf("first Write error: %v", err)
	}
	if n1 != len(half) {
		t.Fatalf("first Write returned %d, want %d", n1, len(half))
	}

	// Second write overflows the cap; buffer should stop at maxOutputBytes.
	overflow := make([]byte, maxOutputBytes)
	for i := range overflow {
		overflow[i] = 'b'
	}
	n2, err := sw.Write(overflow)
	if err != nil {
		t.Fatalf("second Write error: %v", err)
	}
	// Write returns len(p) after capping, so n2 == remaining capacity.
	wantN2 := maxOutputBytes - maxOutputBytes/2
	if n2 != wantN2 {
		t.Fatalf("second Write returned %d, want %d (remaining cap)", n2, wantN2)
	}

	if len(sw.String()) != maxOutputBytes {
		t.Fatalf("buffered output should be capped at %d, got %d", maxOutputBytes, len(sw.String()))
	}
}
