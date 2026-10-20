// Design: docs/architecture/exabgp-bridge.md -- the API version a script speaks
// Related: script.go -- script.batch, which detects before it translates
// Related: fleet.go -- Fleet.Broadcast, which renders the format each version reads

package bridgerun

import (
	"github.com/ze-software/ze/internal/exabgp/bridge"
)

// apiVersion answers the API version this script is held to, or
// bridge.APIVersionAuto while it is undecided. Safe for concurrent use: the
// fan-out reads it on the event goroutine while the batch goroutine moves it.
func (s *script) apiVersion() bridge.APIVersion {
	return bridge.APIVersion(s.version.Load())
}

// format answers the format this script's events are written in now.
func (s *script) format() bridge.Format {
	return bridge.FormatFor(s.apiVersion(), s.encoder)
}

// resetAPIVersion gives a freshly forked child the fleet's setting, which
// ExaBGP's Processes._select_encoder does at each start: "With
// exabgp.api.version set, every operator helper is held to that version; left
// to auto, a helper is undecided until its first command says which API it was
// written for". A respawned child starts undecided again, because the version
// is held "for as long as it runs".
func (s *script) resetAPIVersion() {
	setting := bridge.APIVersion(s.setting.Load())
	s.version.Store(uint32(setting))
	if setting != bridge.APIVersionAuto {
		s.adopt(setting)
	}
}

// detectAPIVersion is ExaBGP's Processes.detect_api_version: "The first
// command whose form belongs to one version holds the helper to it for as long
// as it runs. Undecided commands leave it undecided." It answers the version
// the script is held to after the line.
//
// Only the batch goroutine moves the version, so the load and the store need
// no compare-and-swap.
func (s *script) detectAPIVersion(line string) bridge.APIVersion {
	current := s.apiVersion()
	if current != bridge.APIVersionAuto {
		return current
	}
	found := bridge.CommandAPIVersion(line)
	if found == bridge.APIVersionAuto {
		return current
	}
	s.version.Store(uint32(found))
	s.log.Debug("exabgp-bridge script API version detected", "script", s.name, "version", found.String())
	s.adopt(found)
	return found
}

// adopt logs what ExaBGP's _adopt_v4 and _adopt_v6 log when a helper is held
// to a version: the v4 API is deprecated, and a v6 helper whose encoder asks
// for text reads JSON regardless.
func (s *script) adopt(version bridge.APIVersion) {
	if version == bridge.APIV4 {
		s.log.Warn("exabgp-bridge script uses the deprecated ExaBGP API 4; ExaBGP 6.0 helpers write peer <selector> ...",
			"script", s.name)
		return
	}
	if version == bridge.APIV6 && s.encoder == bridge.EncoderText {
		s.log.Warn("exabgp-bridge script uses ExaBGP API 6, which is JSON only; its encoder text is ignored",
			"script", s.name)
	}
}

// answerAPIVersion performs one `api version` line against this script's
// version and the fleet's setting, and answers the reply line and whether it
// is a failure. A set reaches scripts started afterwards, a respawn included,
// as ExaBGP's does: "a change applies to helpers started afterwards".
func (s *script) answerAPIVersion(request bridge.APIVersionRequest) ([]byte, bool) {
	setting := bridge.APIVersion(s.setting.Load())
	reply, failed := request.Answer(s.apiVersion(), &setting)
	s.setting.Store(uint32(setting))
	return reply, failed
}
