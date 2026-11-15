// Design: docs/architecture/irr-owner.md -- the notices between the irr plugin and its consumers
// Related: store.go -- PrefixStore, the resolution the owner runs
//
// The irr plugin (internal/plugins/irr) and its consumers (bgp-filter-irr,
// firewall-irr) never import each other. They meet through stored keys
// (pkg/zefs KeyIRRDataset, KeyIRRSet, KeyIRRASN, KeyIRRGeneration,
// KeyIRRInterest) and through the two notices declared here, which both sides
// import because both already import this package.

package store

import "github.com/ze-software/ze/internal/core/events"

// OwnerPlugin is the registered name of the IRR owner. Each consumer names it in
// its Dependencies, so the engine starts the owner whenever a consumer runs and
// before it; it is declared here because a consumer cannot import the owner.
const OwnerPlugin = "irr"

// NoticeNamespace and the two event types name the notices of the IRR owner.
// The owner emits EventDatasetStored after each pointer write or removal; a
// consumer emits EventInterestChanged after it rewrites its interest key. Each
// side subscribes at startup stage 5, before its first read, so a notice sent
// between the subscription and the read is not lost.
const (
	NoticeNamespace      = "irr"
	EventDatasetStored   = "dataset-stored"
	EventInterestChanged = "interest-changed"
)

// DatasetNotice is the payload of irr/dataset-stored: the AS-SET whose pointer
// changed, the server and digest of the dataset it now names, and the
// generation the owner raised for the change. Removed is true after
// `clear irr`, when the pointer and dataset are gone. Generation is never zero:
// StateIncrement answers 1 for its first increment.
type DatasetNotice struct {
	ASSet      string `json:"as-set"`
	Server     string `json:"server"`
	Generation uint32 `json:"generation"`
	Digest     string `json:"digest"`
	Removed    bool   `json:"removed"`
}

// InterestNotice is the payload of irr/interest-changed: the consumer whose
// interest key (meta/irr-interest/{consumer}) was rewritten.
type InterestNotice struct {
	Consumer string `json:"consumer"`
}

// DatasetStored and InterestChanged are the registered events. Registration
// makes the (namespace, event type) pairs valid for emit-event and
// subscribe-events in the daemon, which links this package.
var (
	DatasetStored   = events.Register[*DatasetNotice](NoticeNamespace, EventDatasetStored)
	InterestChanged = events.Register[*InterestNotice](NoticeNamespace, EventInterestChanged)
)
