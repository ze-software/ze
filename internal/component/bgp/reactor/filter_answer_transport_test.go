// Design: docs/architecture/core-design.md -- Policy Filter Chain
// Related: filter_library_external_test.go -- externalTransport, the one-filter form

package reactor

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// answerAttachment carries what an answerTransport filter reads (prefix,
// as-path, local-preference, med) and lets it set local-preference and med.
const answerAttachment = "reactor-answer-test"

// answerPlugin is the plugin every answerPolicy member belongs to.
const answerPlugin = "test"

func init() {
	lp, med := routepolicy.FieldLocalPreference, routepolicy.FieldMED
	carries := []routepolicy.Field{routepolicy.FieldPrefix, routepolicy.FieldASPath, lp, med}
	routepolicy.MustRegisterAttachment("bgp", answerAttachment, carries, []routepolicy.Field{lp, med})
}

// answerTransport is the plugin server for a test whose filters decide from
// what they are sent: every filter of answerPlugin is declared, reading reads
// and setting sets, and answer gives each verdict. Safe for concurrent use:
// the reactor calls it from the goroutines that forward routes.
type answerTransport struct {
	reads, sets []string
	answer      func(rpc.FilterUpdateInput) rpc.FilterUpdateOutput

	mu   sync.Mutex
	sent []rpc.FilterUpdateInput
}

// FilterInfo answers no text attributes and whether the filter reads raw.
func (a *answerTransport) FilterInfo(_, _ string) ([]string, bool) {
	return nil, slices.Contains(a.reads, rpc.FilterFieldRaw)
}

func (a *answerTransport) FilterReads(_, _ string) []string { return a.reads }

func (a *answerTransport) FilterDeclaration(pluginName, _ string) ([]string, []string, rpc.OnErrorPolicy, bool) {
	if pluginName != answerPlugin {
		return nil, nil, rpc.OnErrorReject, false
	}
	return a.reads, a.sets, rpc.OnErrorReject, true
}

func (a *answerTransport) FilterReadvertise(pluginName, _ string) (readvertise, ok bool) {
	return false, pluginName == answerPlugin
}

func (a *answerTransport) FilterOnError(_, _ string) rpc.OnErrorPolicy { return rpc.OnErrorReject }

func (a *answerTransport) CallFilterUpdate(_ context.Context, _ string, route *rpc.FilterRoute) (*rpc.FilterUpdateOutput, error) {
	input, err := route.Input()
	if err != nil {
		return nil, err
	}
	input.Fields = append(json.RawMessage(nil), input.Fields...)
	input.Raw = append([]byte(nil), input.Raw...)
	a.mu.Lock()
	a.sent = append(a.sent, input)
	a.mu.Unlock()
	reply := a.answer(input)
	return &reply, nil
}

// calls answers how many filter-update calls the transport answered.
func (a *answerTransport) calls() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.sent)
}

// answerPolicy compiles a policy whose members are the answerPlugin filters
// names, in order, at answerAttachment under final, as bgp/config builds a
// chain that names plugin filters.
func answerPolicy(t testing.TB, final routepolicy.Final, names ...string) filterapi.Policy {
	t.Helper()
	return answerPolicyAt(t, answerAttachment, final, names...)
}

// answerSet is a modify verdict that sets field to the JSON value.
func answerSet(field, value string) rpc.FilterUpdateOutput {
	return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Sets: []rpc.FilterSet{
		{Field: field, Action: rpc.FilterSetActionSet, Value: json.RawMessage(value)}}}
}

// answerFieldsContain reports whether the fields the filter was sent hold s.
func answerFieldsContain(input rpc.FilterUpdateInput, s string) bool {
	return strings.Contains(string(input.Fields), s)
}

// localPref200Transport answers every filter by setting local-preference 200,
// the library form of a filter that answers "local-preference 200".
func localPref200Transport() *answerTransport {
	return &answerTransport{reads: []string{"local-preference"}, sets: []string{"local-preference"},
		answer: func(rpc.FilterUpdateInput) rpc.FilterUpdateOutput { return answerSet("local-preference", "200") }}
}

// rawAnswerTransport answers every filter, which reads the raw body, with a
// modify carrying raw as the full UPDATE-body replacement.
func rawAnswerTransport(raw []byte) *answerTransport {
	return &answerTransport{reads: []string{rpc.FilterFieldRaw}, answer: func(rpc.FilterUpdateInput) rpc.FilterUpdateOutput {
		return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: raw}
	}}
}
