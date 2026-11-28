// Design: docs/architecture/testing/interop.md -- native speaker personalities.
package bgp

import "io"

// speakerRunners is populated at package initialization and read thereafter.
// Each personality owns its registration; the dispatcher knows no oracle names.
var speakerRunners = map[string]func(speakerOptions, io.Writer) error{
	speakerOracleNoDuplicateAttribute:     runOracleSpeaker,
	speakerOracleNoUnrecognizedEVPNType:   runOracleSpeaker,
	speakerOracleBFDStrictHold:            runOracleSpeaker,
	speakerOracleNoRepeatedLargeCommunity: runOracleSpeaker,
	speakerOracleRFC8097EBGPStrip:         runOracleSpeaker,
	speakerOracleRFC8097EBGPOptIn:         runOracleSpeaker,
}

// speakerRelayRunners contains only explicit relay personalities. The existing
// generic --relay-peer fallback remains authoritative for every other name,
// including names registered as ordinary speakers.
var speakerRelayRunners = map[string]func(speakerOptions, io.Writer) error{}

// speakerUpdateOracles holds the per-UPDATE judgement of each oracle that has one,
// read by applySpeakerOracle. Populated at package initialization.
var speakerUpdateOracles = map[string]func(speakerUpdate, *speakerVerdict){
	speakerOracleNoDuplicateAttribute:     applyNoDuplicateAttributeOracle,
	speakerOracleNoUnrecognizedEVPNType:   applyEVPNOracle,
	speakerOracleNoRepeatedLargeCommunity: applyNoRepeatedLargeCommunityOracle,
	speakerOracleRFC8097EBGPStrip:         applyRFC8097EBGPStripOracle,
	speakerOracleRFC8097EBGPOptIn:         applyRFC8097EBGPOptInOracle,
}
