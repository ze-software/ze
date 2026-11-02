// Design: docs/architecture/api/process-protocol.md — Stage 1 protocol version
// Related: sdk.go — Run, which sends Stage 1 and reads this answer

package sdk

import (
	"encoding/json"
	"fmt"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// acceptProtocolVersion reads the engine's Stage 1 answer and refuses any
// version this SDK does not speak. It speaks every version from 1 to
// rpc.ProtocolVersion: a later version only adds bytes an engine writes to a
// plugin that negotiated it (rpc.ProtocolVersionWarningLines), and this SDK
// reads a version 1 answer the same way it always did. An answer with no
// version, or one above rpc.ProtocolVersion, is an engine whose contract this
// SDK does not know, refused: speaking to it would send declared fields it does
// not read, or read bytes this SDK cannot parse.
func acceptProtocolVersion(answer json.RawMessage) error {
	var out rpc.DeclareRegistrationOutput
	if len(answer) > 0 {
		if err := json.Unmarshal(answer, &out); err != nil {
			return fmt.Errorf("engine answer: %w", err)
		}
	}
	if out.ProtocolVersion < 1 {
		return refuseProtocolVersion(out.ProtocolVersion)
	}
	if out.ProtocolVersion > rpc.ProtocolVersion {
		return refuseProtocolVersion(out.ProtocolVersion)
	}
	return nil
}

// refuseProtocolVersion is the refusal of an engine answer naming version.
func refuseProtocolVersion(version int) error {
	return fmt.Errorf("the engine answered plugin protocol version %d, and this SDK speaks versions 1 to %d: "+
		"build the plugin against the SDK of the same Ze release", version, rpc.ProtocolVersion)
}
