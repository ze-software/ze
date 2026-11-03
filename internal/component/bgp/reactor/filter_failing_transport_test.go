// Design: docs/architecture/core-design.md -- Policy Filter Chain, a failed member applies its on-error
// Related: filter_answer_transport_test.go -- answerTransport, answerPolicy

package reactor

import (
	"context"
	"encoding/json"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// failingTransport is an answerTransport whose every filter-update call
// fails with err, as a forked filter plugin that timed out or has gone does:
// the member could not run, so its on-error (reject) decides the route and
// the chain's outcome carries the failure. It counts the calls it was asked.
// Safe for concurrent use: the reactor calls it from forwarding goroutines.
type failingTransport struct {
	*answerTransport
	err error
}

// failingAnswerTransport answers every answerPlugin filter, which reads the
// raw body, with err.
func failingAnswerTransport(err error) *failingTransport {
	return &failingTransport{answerTransport: &answerTransport{reads: []string{rpc.FilterFieldRaw}}, err: err}
}

// CallFilterUpdate records the route it was sent and answers the error.
func (f *failingTransport) CallFilterUpdate(_ context.Context, _ string, route *rpc.FilterRoute) (*rpc.FilterUpdateOutput, error) {
	input, err := route.Input()
	if err != nil {
		return nil, err
	}
	input.Fields = append(json.RawMessage(nil), input.Fields...)
	input.Raw = append([]byte(nil), input.Raw...)
	f.mu.Lock()
	f.sent = append(f.sent, input)
	f.mu.Unlock()
	return nil, f.err
}
