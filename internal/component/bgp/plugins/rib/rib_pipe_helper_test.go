package rib

import (
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// ribPipe answers the declared list the engine hands a route read for the
// selection words, written in the order an operator types them after the
// pipes: `ribPipe("received", "peer", "192.0.2.1", "count")` is the list of
// `show bgp rib | received | peer 192.0.2.1 | count`.
//
// One operation per keyword. A keyword the route read takes a value for
// (isShowFilter, the production table) takes the next word as its one
// argument, and every other word is an operation with no argument, so an
// unknown word reaches the parser as an unknown operation and a value keyword
// at the end reaches it with no value, which is what each refusal test asks.
// Tests only: production code receives the list from the engine.
func ribPipe(words ...string) []sdk.PipeOperation {
	var pipe []sdk.PipeOperation
	for i := 0; i < len(words); i++ {
		operation := sdk.PipeOperation{Name: words[i]}
		if isShowFilter(words[i]) && i+1 < len(words) {
			i++
			operation.Args = []string{words[i]}
		}
		pipe = append(pipe, operation)
	}
	return pipe
}
