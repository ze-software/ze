// Design: docs/plugin-development/protocol.md -- The Route Fields of filter-update
// Related: register_filter_subject.go -- the registration

package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// filterSubjectField is one route field the filter declares, beside the value
// the route test/plugin/filter-subject-names-every-attribute.ci announces
// carries for it, in the UPDATE event's attr shape.
type filterSubjectField struct {
	name  string
	value string
}

// filterSubjectFields are the five attributes that once reached no filter
// plugin, each with the value the announced route carries. A field missing
// from the fields object, or carrying another value, is one no policy can act
// on.
var filterSubjectFields = []filterSubjectField{
	{name: fieldOrigin, value: `"incomplete"`},
	{name: fieldMED, value: `100`},
	{name: fieldLocalPreference, value: `150`},
	{name: "atomic-aggregate", value: `true`},
	{name: "cluster-list", value: `["1.1.1.1","2.2.2.2"]`},
}

// filterSubjectState holds the fields object the daemon handed the filter.
// The handler runs on the plugin's callback goroutine and the scenario reads
// it on its own, so the mutex is load-bearing.
type filterSubjectState struct {
	mu     sync.Mutex
	fields []byte
}

// filterSubjectObserver registers one import filter that declares the five
// fields, records the fields object the daemon hands it, and fails when any
// of the five is absent or carries a value other than the announced one.
//
// The assertion lives in compiled code rather than in the .ci file because the
// fields reach the plugin over the RPC and never appear in the daemon's
// stderr. The plugin prints them, so a failure carries the object that
// produced it.
func filterSubjectObserver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("filter-subject-names-every-attribute takes no arguments")
	}

	reads := make([]string, 0, len(filterSubjectFields))
	for _, field := range filterSubjectFields {
		reads = append(reads, field.name)
	}
	state := new(filterSubjectState)
	registration := sdk.Registration{Filters: []sdk.FilterDecl{{
		Name:      "record-subject",
		Direction: sdk.FilterImport,
		Reads:     reads,
		OnError:   sdk.OnErrorReject,
	}}}

	setup := func(plugin *sdk.Plugin) error {
		plugin.OnFilter(func(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
			raw, err := route.Fields()
			if err != nil {
				return nil, fmt.Errorf("filter-subject: read the declared fields: %w", err)
			}
			state.mu.Lock()
			state.fields = bytes.Clone(raw)
			state.mu.Unlock()
			filterSubjectSaid("filter-subject: peer="+route.Peer()+" fields=", raw)
			return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, nil
		})
		return nil
	}

	scenario := func(ctx context.Context, _ *sdk.Plugin) error {
		Poll(ctx, 100, 100*time.Millisecond, func() bool {
			state.mu.Lock()
			defer state.mu.Unlock()
			return len(state.fields) != 0
		})

		state.mu.Lock()
		raw := state.fields
		state.mu.Unlock()

		if len(raw) == 0 {
			return fmt.Errorf("the import filter was never called for the route ze-peer announced")
		}
		return filterSubjectCheck(raw)
	}

	return observeConfigured(ctx, "filter-subject-test", &registration, setup, scenario)
}

// filterSubjectCheck compares each declared field of the fields object with
// the value the announced route carries, and names every one that differs.
func filterSubjectCheck(raw []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return fmt.Errorf("decode the fields object %s: %w", raw, err)
	}

	var wrong []string
	for _, want := range filterSubjectFields {
		got, carried := fields[want.name]
		if !carried {
			wrong = append(wrong, want.name+" absent")
			continue
		}
		var compact bytes.Buffer
		if err := json.Compact(&compact, got); err != nil {
			return fmt.Errorf("compact %s value %s: %w", want.name, got, err)
		}
		if compact.String() != want.value {
			wrong = append(wrong, want.name+" is "+compact.String()+", want "+want.value)
		}
	}
	if len(wrong) > 0 {
		return fmt.Errorf("the fields object is wrong (%s): %s", strings.Join(wrong, "; "), raw)
	}
	filterSubjectSaid("OK: the fields name all five attributes: ", raw)
	// One line per field, its JSON value with the quotes taken out: the relay
	// logs a plugin's stderr as a quoted slog message, which escapes every
	// quote, so the .ci asserts these lines rather than the raw object.
	for _, want := range filterSubjectFields {
		filterSubjectSaid("OK: field "+want.name+" is ", []byte(strings.ReplaceAll(want.value, `"`, "")))
	}
	return nil
}

// filterSubjectSaid prints one line, a lead then the fields object, so a
// failure carries the object the daemon really handed the filter.
func filterSubjectSaid(lead string, fields []byte) {
	var line textbuf.Buffer
	line.Str(lead).Str(string(fields))
	line.Byte('\n')
	_ = line.StdErr() //nolint:errcheck // a failed write to stderr has nowhere left to report itself
}
