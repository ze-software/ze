// Design: docs/plugin-development/handlers.md -- route filters, the one evaluator both transports call
// Related: ../../fakefilter.go -- the SDK-only plugin this binary runs
//
// Command ze-fakefilter is the fakefilter test plugin built as its own
// binary, the way a third party builds a Go filter plugin: it links
// pkg/plugin/sdk and the SDK-only fakefilter package, and nothing that
// registers with ze (spec-filter-wire-0-umbrella AC-35). ze forks it from
// `plugin { external <name> { run "ze-fakefilter"; } }`, it connects back
// over TLS, and it answers the batched filter-update message. Asked for its
// declaration (query mode) it prints its Stage 1 line and exits.
package main

import (
	"log/slog"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/test/plugins/fakefilter"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func main() {
	os.Exit(run())
}

// run reads the trigger and the name ze gave the process, then runs the
// plugin, or answers its declaration under a query. It returns the exit code.
func run() int {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})).
		With("subsystem", fakefilter.Name)
	tr, err := fakefilter.TriggerFrom(lookup)
	if err != nil {
		log.Error("ze-fakefilter: read the trigger", "error", err)
		return 1
	}
	return sdk.RunOrDeclare(fakefilter.Registration(tr), func() int {
		// ze names the process after its config entry (ZE_PLUGIN_NAME), and
		// authenticates it by that name.
		name := os.Getenv("ZE_PLUGIN_NAME")
		if name == "" {
			log.Error("ze-fakefilter: ZE_PLUGIN_NAME is not set; run it from a ze `plugin { external <name> { run ... } }` entry")
			return 1
		}
		p, err := sdk.NewFromTLSEnv(name)
		if err != nil {
			log.Error("ze-fakefilter: connect to ze", "error", err)
			return 1
		}
		return fakefilter.Run(p, tr, log)
	})
}

// lookup reads one dotted trigger key from the environment. ze starts the
// process through `sh -c`, which passes only names that are shell
// identifiers, so the underscore spelling is the one that arrives; the dotted
// one is read first for a direct start.
func lookup(key string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return os.Getenv(strings.ReplaceAll(key, ".", "_"))
}
