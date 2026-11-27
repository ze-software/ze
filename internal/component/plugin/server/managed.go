// Design: docs/architecture/fleet-config.md — hub-side managed config handlers
// Related: config.go — ServerConfig holds hub transport config
// Related: managed_held.go — the held records HandleConfigFetch names and clears
// Related: managed_chain.go — hub-side SHA-1 chain persistence and historical base lookup

package server

import (
	"encoding/base64"
	"errors"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/fleet"
)

const managedFetchStatusCurrent = "current"

// ErrClientConfigNotFound is returned when no config exists for a client name.
var ErrClientConfigNotFound = errors.New("client config not found")

// ErrDuplicateClient is returned when a client with the same name is already connected.
var ErrDuplicateClient = errors.New("duplicate client name")

// ConfigReader reads a client's config by name from the hub's blob store.
// Returns the raw config bytes, or ErrClientConfigNotFound if the client
// has no config entry.
type ConfigReader func(name string) ([]byte, error)

// ManagedConfigService handles hub-side config-fetch and config-changed operations
// for managed clients. It reads client configs via a ConfigReader, computes
// version and SHA-1 chain hashes, and builds deterministic line diffs against
// historical base versions when available. Safe for concurrent use.
type ManagedConfigService struct {
	readConfig ConfigReader
	// readBaseVersion resolves historical raw config bytes for (name, version)
	// from the hub's versioned store so HandleConfigFetch can compute line
	// diffs. nil falls back to full config delivery.
	readBaseVersion func(name, version string) ([]byte, bool, error)
	// chainStore tracks per-client persisted SHA-1 chain heads and resync
	// failures. nil skips hub-side chain persistence while still computing
	// deterministic FoldFull/FoldDiff hashes.
	chainStore *managedChainStore
	// records holds the copy each holding client sent (heldRecords). nil
	// keeps none: no answer names a held version and nothing is cleared.
	records *heldRecords
	// sampleInterval returns the hub's configured metric sample interval in
	// seconds (0 disables sampling). nil omits sample-interval-seconds.
	sampleInterval func() int
	// onHeldCleared notifies the owning server after a "current" fetch removes
	// a stale held configuration copy so config-diverged alarms update.
	onHeldCleared func(clientName string)
}

// newManagedConfigService creates a service that reads client configs via reader.
func newManagedConfigService(reader ConfigReader) *ManagedConfigService {
	return &ManagedConfigService{readConfig: reader}
}

// HandleConfigFetch processes a config-fetch request from a managed client.
// If the client's version matches the current config (and full resync was not
// requested), returns status "current" with the hub's expected chain hash (and
// reanchor: true when only the chain head drifted or is uninitialized).
// Otherwise returns either a deterministic line diff (mode "diff") when the
// client's base version exists in hub history and the diff is smaller than the
// full config, or the full config as base64 (mode "full"). held-version names
// the copy the hub holds of a configuration the client holds, and
// discard-version carries an operator's revert decision (heldRecords.offer),
// which always delivers full bytes.
//
// A "current" answer removes the client's held record: it proves the hub and
// the client hold the same configuration, so the copy shows nothing to
// resolve. A record that cannot be removed or read is logged and the fetch is
// still answered, because an error would end the client's fetch and make it
// reconnect. An unreadable record is answered as no held-version, which is a
// guard and not an absence: the client then sends a copy, and its write
// replaces the record that did not decode.
func (s *ManagedConfigService) HandleConfigFetch(clientName string, req fleet.ConfigFetchRequest) (fleet.ConfigFetchResponse, error) {
	resp, _, err := s.handleConfigFetchWithReason(clientName, req)
	return resp, err
}

func (s *ManagedConfigService) handleConfigFetchWithReason(clientName string, req fleet.ConfigFetchRequest) (fleet.ConfigFetchResponse, string, error) {
	data, err := s.readConfig(clientName)
	if err != nil {
		return fleet.ConfigFetchResponse{}, "", err
	}

	version := fleet.VersionHash(data)
	var sampleInterval int
	if s.sampleInterval != nil {
		sampleInterval = s.sampleInterval()
	}

	if req.Version == version && req.Resync != fleet.ResyncModeFull {
		return s.handleCurrentFetch(clientName, req, version, data, sampleInterval), "", nil
	}

	held, discard, heldErr := s.records.offer(clientName, req.Version, version)
	if heldErr != nil {
		managedLogger().Warn("the hub could not read the held copy of a managed client's configuration, so it names none and offers no revert",
			"code", diagnostic.CodeDoctorStoreIntegrity, "name", clientName, "error", heldErr)
	}

	if req.Resync == fleet.ResyncModeFull {
		return fullFetchResponse(version, data, held, discard, sampleInterval), "", nil
	}
	if discard != "" {
		return fullFetchResponse(version, data, held, discard, sampleInterval), "revert requires full config", nil
	}
	if req.Version == "" {
		return fullFetchResponse(version, data, held, discard, sampleInterval), "", nil
	}

	resp, diffReason := s.buildDiffOrFullResponse(clientName, req, version, data, held, sampleInterval)
	return resp, diffReason, nil
}

func (s *ManagedConfigService) handleCurrentFetch(clientName string, req fleet.ConfigFetchRequest, version string, data []byte, sampleInterval int) fleet.ConfigFetchResponse {
	cleared, clearErr := s.records.clearAndReport(clientName)
	if clearErr != nil {
		managedLogger().Warn("the hub could not remove the held copy of a managed client that is now in sync",
			"code", diagnostic.CodeDoctorStoreIntegrity, "name", clientName, "error", clearErr)
	} else if cleared && s.onHeldCleared != nil {
		s.onHeldCleared(clientName)
	}

	anchor := fleet.FoldFull(data)
	hubChain, hasHubChain := s.chainStore.get(clientName)
	if hasHubChain && hubChain.Version == version && req.ChainHash != "" && req.ChainHash == hubChain.Chain {
		return fleet.ConfigFetchResponse{
			Status:                managedFetchStatusCurrent,
			SampleIntervalSeconds: sampleInterval,
			ChainHash:             hubChain.Chain,
		}
	}
	if !hasHubChain && req.ChainHash != "" && req.ChainHash == anchor {
		// AC-3: first steady-state fetch after bootstrap converges cleanly without ack.
		// put logs Warn under CodeDoctorManagedChainUnreadable on storage failure.
		_ = s.chainStore.put(clientName, fleet.ChainState{Version: version, Chain: anchor})
		return fleet.ConfigFetchResponse{
			Status:                managedFetchStatusCurrent,
			SampleIntervalSeconds: sampleInterval,
			ChainHash:             anchor,
		}
	}

	// Same config bytes, drifted or missing chain: persist anchor idempotently
	// and instruct the client to verify FoldFull(activeBytes) and re-anchor.
	_ = s.chainStore.put(clientName, fleet.ChainState{Version: version, Chain: anchor})
	return fleet.ConfigFetchResponse{
		Status:                managedFetchStatusCurrent,
		SampleIntervalSeconds: sampleInterval,
		Reanchor:              true,
		ChainHash:             anchor,
	}
}

func (s *ManagedConfigService) buildDiffOrFullResponse(clientName string, req fleet.ConfigFetchRequest, version string, data []byte, held string, sampleInterval int) (fleet.ConfigFetchResponse, string) {
	if s.readBaseVersion == nil {
		return fullFetchResponse(version, data, held, "", sampleInterval), "hub base version reader unavailable"
	}
	baseBytes, found, baseErr := s.readBaseVersion(clientName, req.Version)
	if baseErr != nil {
		managedLogger().Warn("the hub could not read the historical base config for a managed client; serving full config",
			"code", diagnostic.CodeDoctorStoreIntegrity, "name", clientName, "base-version", req.Version, "error", baseErr)
		return fullFetchResponse(version, data, held, "", sampleInterval), "historical base read failed: " + boundedManagedDetail(baseErr.Error())
	}
	if !found {
		return fullFetchResponse(version, data, held, "", sampleInterval), "base version " + req.Version + " not found in hub history"
	}

	delta, worth, diffErr := fleet.ComputeDiff(baseBytes, data)
	if diffErr != nil {
		return fullFetchResponse(version, data, held, "", sampleInterval), "diff computation failed: " + boundedManagedDetail(diffErr.Error())
	}
	if !worth {
		return fullFetchResponse(version, data, held, "", sampleInterval), "diff not smaller than full config or exceeded op limit"
	}

	hubChain, hasHubChain := s.chainStore.get(clientName)
	var prevChain string
	reanchor := false
	if hasHubChain && hubChain.Version == req.Version && req.ChainHash != "" && req.ChainHash == hubChain.Chain {
		prevChain = req.ChainHash
	} else {
		prevChain = fleet.FoldFull(baseBytes)
		reanchor = true
	}
	nextChain, foldErr := fleet.FoldDiff(prevChain, req.Version, version, delta)
	if foldErr != nil {
		return fullFetchResponse(version, data, held, "", sampleInterval), "chain fold failed: " + boundedManagedDetail(foldErr.Error())
	}

	return fleet.ConfigFetchResponse{
		Version:               version,
		HeldVersion:           held,
		SampleIntervalSeconds: sampleInterval,
		Mode:                  fleet.FetchModeDiff,
		BaseVersion:           req.Version,
		Diff:                  base64.StdEncoding.EncodeToString(delta),
		Reanchor:              reanchor,
		ChainHash:             nextChain,
	}, ""
}

func fullFetchResponse(version string, data []byte, held, discard string, sampleInterval int) fleet.ConfigFetchResponse {
	return fleet.ConfigFetchResponse{
		Version:               version,
		Config:                base64.StdEncoding.EncodeToString(data),
		HeldVersion:           held,
		DiscardVersion:        discard,
		SampleIntervalSeconds: sampleInterval,
		Mode:                  fleet.FetchModeFull,
		ChainHash:             fleet.FoldFull(data),
	}
}

// expectedChainFor returns the hub's expected SHA-1 chain head for (clientName,
// version) without writing to storage: the persisted chain head when it matches
// version, or FoldFull(data) otherwise.
func (s *ManagedConfigService) expectedChainFor(clientName, version string, data []byte) string {
	if hubChain, ok := s.chainStore.get(clientName); ok && hubChain.Version == version && fleet.ValidChainHash(hubChain.Chain) {
		return hubChain.Chain
	}
	return fleet.FoldFull(data)
}

// ExpectedChainHash returns the hub's expected SHA-1 chain head for clientName
// with zero store writes. The second result is false when no active config or
// chain head can be read for clientName.
func (s *ManagedConfigService) ExpectedChainHash(clientName string) (string, bool) {
	if s == nil {
		return "", false
	}
	if s.readConfig == nil {
		return "", false
	}
	data, err := s.readConfig(clientName)
	if err != nil {
		if hubChain, ok := s.chainStore.get(clientName); ok && fleet.ValidChainHash(hubChain.Chain) {
			return hubChain.Chain, true
		}
		return "", false
	}
	return s.expectedChainFor(clientName, fleet.VersionHash(data), data), true
}

// BuildConfigChanged creates a config-changed notification for a client.
// Reads the client's current config and computes its version hash and expected
// SHA-1 chain head when known.
func (s *ManagedConfigService) BuildConfigChanged(clientName string) (fleet.ConfigChanged, error) {
	data, err := s.readConfig(clientName)
	if err != nil {
		return fleet.ConfigChanged{}, err
	}

	version := fleet.VersionHash(data)
	return fleet.ConfigChanged{
		Version:   version,
		ChainHash: s.expectedChainFor(clientName, version, data),
	}, nil
}
