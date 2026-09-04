// Design: docs/architecture/api/process-protocol.md -- the filter text protocol

package fixture

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// filterSubjectNames are the five attribute names Ze advertises in the filter
// text protocol that reached no filter plugin until the renderer was repaired.
// A route carrying all five is what test/plugin/filter-subject-names-every-
// attribute.ci announces, so a name missing here is a name no policy can act on.
var filterSubjectNames = []string{
	"origin",
	"med",
	fieldLocalPreference,
	"atomic-aggregate",
	"cluster-list",
}

func init() {
	Register("plugin/filter-subject-names-every-attribute", filterSubjectObserver)
}

// filterSubjectState holds the subject the daemon handed the filter. The
// handler runs on the plugin's callback goroutine and the scenario reads it on
// its own, so the mutex is load-bearing.
type filterSubjectState struct {
	mu      sync.Mutex
	subject string
}

// filterSubjectObserver registers one import filter, records the subject the
// daemon hands it, and fails when any of the five names is absent.
//
// The assertion lives in compiled code rather than in the .ci file because the
// subject reaches the plugin over the RPC and never appears in the daemon's
// stderr. The plugin prints it, so a failure carries the text that produced it.
func filterSubjectObserver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("filter-subject-names-every-attribute takes no arguments")
	}

	state := new(filterSubjectState)
	registration := sdk.Registration{Filters: []sdk.FilterDecl{{
		Name:      "record-subject",
		Direction: sdk.FilterImport,
		OnError:   sdk.OnErrorReject,
	}}}

	setup := func(plugin *sdk.Plugin) error {
		plugin.OnFilterUpdate(func(input *sdk.FilterUpdateInput) (*sdk.FilterUpdateOutput, error) {
			state.mu.Lock()
			state.subject = input.Update
			state.mu.Unlock()
			fmt.Fprintf(os.Stderr, "filter-subject: peer=%s update=%q\n", input.Peer, input.Update)
			return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, nil
		})
		return nil
	}

	scenario := func(ctx context.Context, _ *sdk.Plugin) error {
		Poll(ctx, 100, 100*time.Millisecond, func() bool {
			state.mu.Lock()
			defer state.mu.Unlock()
			return state.subject != ""
		})

		state.mu.Lock()
		subject := state.subject
		state.mu.Unlock()

		if subject == "" {
			return fmt.Errorf("the import filter was never called for the route ze-peer announced")
		}

		var missing []string
		for _, name := range filterSubjectNames {
			if !strings.Contains(subject, name+" ") && !strings.HasSuffix(subject, name) {
				missing = append(missing, name)
			}
		}
		if len(missing) > 0 {
			return fmt.Errorf("the subject names none of %s: %q",
				strings.Join(missing, ", "), subject)
		}
		fmt.Fprintf(os.Stderr, "OK: the subject names all five attributes: %q\n", subject)
		return nil
	}

	return observeConfigured(ctx, "filter-subject-test", registration, setup, scenario)
}
