// Design: docs/architecture/cli/command-completion.md -- one surface predicate, every producer
// Related: command_meta.go -- the MCP, REST and gRPC list asks the same answer
// Related: service_web.go -- the web admin tree and the web terminal completer

package hub

import (
	"github.com/ze-software/ze/internal/component/command"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// streamingRegistration is a streaming command as the surface predicate reads
// it. Its one-shot route is the builtin RPC under the same YANG path, which
// the surface parity check requires of every streaming prefix.
func streamingRegistration(path string) (command.CommandRegistration, bool) {
	oneShot, ok := pluginserver.StreamingOneShot(path)
	if !ok {
		return command.CommandRegistration{}, false
	}
	return command.CommandRegistration{
		Path:          path,
		Kind:          command.KindBuiltin,
		Streaming:     true,
		StreamingOnly: oneShot == pluginserver.OneShotStreamingOnly,
	}, true
}

// streamingPathsOff answers the streaming commands surface s must not offer,
// as the surface predicate decides: a one-shot surface leaves out each one
// declared streaming-only.
func streamingPathsOff(s command.Surface) []string {
	var off []string
	for _, prefix := range pluginserver.StreamingPrefixes() {
		reg, ok := streamingRegistration(prefix)
		if !ok {
			continue // unregistered between the two reads
		}
		listed, err := command.OffersCommand(reg, s)
		if err != nil {
			// The registration carries its kind and a streaming handler, so
			// only a defect in the predicate lands here.
			panic("BUG: streamingPathsOff: " + err.Error())
		}
		if !listed {
			off = append(off, prefix)
		}
	}
	return off
}
