// Design: docs/architecture/api/process-protocol.md — Stage 1 protocol version
// Related: sdk.go — Run, which sends Stage 1 and reads this answer

package sdk

import (
	"encoding/json"
	"fmt"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// acceptProtocolVersion reads the engine's Stage 1 answer and refuses any
// version other than the one this SDK speaks. An answer with no version is an
// engine from before the versioned contract, refused the same way: speaking to
// it would send declared fields it does not read.
func acceptProtocolVersion(answer json.RawMessage) error {
	var out rpc.DeclareRegistrationOutput
	if len(answer) > 0 {
		if err := json.Unmarshal(answer, &out); err != nil {
			return fmt.Errorf("engine answer: %w", err)
		}
	}
	if out.ProtocolVersion != rpc.ProtocolVersion {
		return fmt.Errorf("the engine answered plugin protocol version %d, and this SDK speaks only version %d: "+
			"build the plugin against the SDK of the same Ze release", out.ProtocolVersion, rpc.ProtocolVersion)
	}
	return nil
}
