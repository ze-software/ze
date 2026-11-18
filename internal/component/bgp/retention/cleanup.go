// Design: docs/guide/graceful-restart.md -- passive validation lifetime cleanup.
// Related: retention.go -- the original pre-DOWN family verdict and ledger.
package retention

import (
	"encoding/json"
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const CleanupEventType = "validation-cleanup"

// CleanupEvent records completion of an existing GR source cleanup command.
// It grants no received or selecting authority. The source command already
// joins final output; this event retires only passive validation metadata.
var CleanupEvent = events.Register[*cleanupWire](Namespace, CleanupEventType)

type cleanupWire struct {
	PeerOwner    uint64   `json:"peer-owner,string"`
	ConnectionID uint64   `json:"connection-id,string"`
	PeerAddress  string   `json:"peer-address"`
	Families     []string `json:"families"`
	NoLLGROnly   bool     `json:"no-llgr-only,omitempty"`
}

// Cleanup is a validated original physical source and a removal predicate.
// Its zero value is invalid and matches no route. An empty family set is the
// explicit all-family variant; NoLLGR narrows removal, never eligibility.
type Cleanup struct {
	owner      uint64
	connection uint64
	address    netip.Addr
	families   map[family.Family]bool
	noLLGR     bool
}

func (c Cleanup) Owner() uint64       { return c.owner }
func (c Cleanup) Connection() uint64  { return c.connection }
func (c Cleanup) Address() netip.Addr { return c.address }
func (c Cleanup) NoLLGROnly() bool    { return c.noLLGR }

// MatchesFamily is a removal predicate, never a current-source lookup.
func (c Cleanup) MatchesFamily(fam family.Family) bool {
	if c.owner == 0 {
		return false
	}
	if c.connection == 0 {
		return false
	}
	if len(c.families) == 0 {
		return true
	}
	return c.families[fam]
}

func EncodeCleanup(owner, connection uint64, address string, families []family.Family, noLLGR bool) (string, error) {
	if err := validateLifetime(owner, connection); err != nil {
		return "", err
	}
	if noLLGR {
		if len(families) != 1 {
			return "", errors.New("GR NO_LLGR cleanup requires one original family")
		}
	}
	parsed, err := netip.ParseAddr(address)
	if err != nil {
		return "", err
	}
	value := cleanupWire{PeerOwner: owner, ConnectionID: connection, PeerAddress: parsed.String(), NoLLGROnly: noLLGR, Families: make([]string, 0, len(families))}
	for _, fam := range families {
		if _, known := family.LookupFamily(fam.String()); !known {
			return "", errors.New("GR validation cleanup names an unknown family")
		}
		value.Families = append(value.Families, fam.String())
	}
	body, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func AcceptsCleanup(delivered string) (rpc.EventEnvelope, bool) {
	envelope, err := rpc.ParseEventEnvelope(delivered)
	if err != nil {
		return rpc.EventEnvelope{}, false
	}
	if envelope.Namespace != Namespace {
		return rpc.EventEnvelope{}, false
	}
	if envelope.Event != CleanupEventType {
		return rpc.EventEnvelope{}, false
	}
	return envelope, true
}

// DecodeCleanup validates the whole removal scope before either consumer
// mutates its current store. Unknown families refuse the complete event.
func DecodeCleanup(envelope rpc.EventEnvelope) (Cleanup, error) {
	var value cleanupWire
	if err := json.Unmarshal(envelope.Payload, &value); err != nil {
		return Cleanup{}, err
	}
	if err := validateLifetime(value.PeerOwner, value.ConnectionID); err != nil {
		return Cleanup{}, err
	}
	address, err := netip.ParseAddr(value.PeerAddress)
	if err != nil {
		return Cleanup{}, err
	}
	result := Cleanup{owner: value.PeerOwner, connection: value.ConnectionID, address: address, noLLGR: value.NoLLGROnly}
	if value.NoLLGROnly {
		if len(value.Families) != 1 {
			return Cleanup{}, errors.New("GR NO_LLGR cleanup requires one original family")
		}
	}
	for _, name := range value.Families {
		fam, known := family.LookupFamily(name)
		if !known {
			return Cleanup{}, errors.New("GR validation cleanup names an unknown family")
		}
		if result.families == nil {
			result.families = make(map[family.Family]bool, len(value.Families))
		}
		result.families[fam] = true
	}
	return result, nil
}
