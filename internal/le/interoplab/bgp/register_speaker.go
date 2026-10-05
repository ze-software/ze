// Design: docs/architecture/testing/interop.md -- native speaker personalities.
package bgp

import "io"

// speakerRunners is populated at package initialization and read thereafter.
// Each personality owns its registration; the dispatcher knows no oracle names.
var speakerRunners = map[string]func(speakerOptions, io.Writer) error{
	speakerOracleNoDuplicateAttribute:   runOracleSpeaker,
	speakerOracleNoUnrecognizedEVPNType: runOracleSpeaker,
	speakerOracleBFDStrictHold:          runOracleSpeaker,
}
