// Design: docs/architecture/testing/interop.md -- foreign collector epoch evidence.
// Related: check_med_pmacct_epoch.go -- decoded collector rows and epoch identity.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	medWholeSetLocRIB = "Loc-Rib"
	// Keep absolute file positions and pre-checkpoint Peer Up context. The extra
	// row detects an oversized snapshot; silently truncating could hide a reset.
	medWholeSetSelectionRows = `test -r ` + medWholeSetCheckpoint + ` && test -r ` + pmacctMsgLogPath +
		` && cat ` + medWholeSetCheckpoint + ` && head -n 257 ` + pmacctMsgLogPath
)

// waitMEDWholeSetSelected follows the saved checkpoint and its route change.
// RFC 9069 Section 6.1 says: "In this sense, the peer that conveys the Loc-RIB is
// a locally emulated peer." Its identity is separate from A, B and C's peers.
func waitMEDWholeSetSelected(ctx context.Context, lab interoplab.CheckerLab, want *medWholeSetCandidate) error {
	var description textbuf.Buffer
	var mismatch error
	last, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 60 * time.Second, Interval: 2 * time.Second,
		Description: description.Str("pmacct current Loc-RIB winner from ").Addr(want.peer).String(),
	}, func(probeCtx context.Context) (string, error) {
		return lab.Query(probeCtx, peerPMACCT, []string{"sh", "-c", medWholeSetSelectionRows}, nil)
	}, func(output string) bool {
		mismatch = requireMEDWholeSetSelected(output, want)
		return mismatch == nil
	})
	if err != nil {
		if mismatch != nil {
			err = fmt.Errorf("%w: %v", err, mismatch)
		}
	}
	return withLastOutput(err, last)
}

type medWholeSetLocEpoch struct {
	peerUp    *medWholeSetCollectorRow
	route     *medWholeSetCollectorRow
	routeLine uint64
}

func requireMEDWholeSetSelected(output string, want *medWholeSetCandidate) error {
	checkpointText, rows, found := strings.Cut(output, "\n")
	if !found {
		return errors.New("pmacct selection evidence has no checkpoint line")
	}
	checkpoint, err := strconv.ParseUint(strings.TrimSpace(checkpointText), 10, 64)
	if err != nil {
		return fmt.Errorf("pmacct selection checkpoint: %w", err)
	}
	epoch, err := readMEDWholeSetLocEpoch(rows, want.router)
	if err != nil {
		return err
	}
	if epoch.peerUp == nil {
		return errors.New("pmacct has no current Loc-RIB Peer Up")
	}
	if epoch.route == nil {
		return errors.New("pmacct has no selected route in the current Loc-RIB epoch")
	}
	if epoch.routeLine <= checkpoint {
		return errors.New("pmacct selected route does not follow the saved checkpoint")
	}
	return medWholeSetSelectedCriteria(&epoch, want)
}

func readMEDWholeSetLocEpoch(output string, router netip.Addr) (medWholeSetLocEpoch, error) {
	var epoch medWholeSetLocEpoch
	var lineNumber uint64
	prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 99, 77, 0}), 24)
	for line := range strings.SplitSeq(strings.TrimSuffix(output, "\n"), "\n") {
		lineNumber++
		if lineNumber > 256 {
			return epoch, errors.New("pmacct selection evidence exceeds the 256-line query bound")
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		var row medWholeSetCollectorRow
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return epoch, fmt.Errorf("pmacct selection line %d is not decoded JSON: %w", lineNumber, err)
		}
		if row.Router != router {
			continue
		}
		if medWholeSetEndsLocEpoch(&row, epoch.peerUp) {
			epoch = medWholeSetLocEpoch{}
			continue
		}
		if row.RIB != medWholeSetLocRIB {
			continue
		}
		if row.Peer != netip.AddrFrom4([4]byte{}) {
			continue
		}
		if row.Message == "peer_up" {
			epoch = medWholeSetLocEpoch{peerUp: &row}
			continue
		}
		if row.Message != "route_monitor" {
			continue
		}
		if row.Prefix != prefix {
			continue
		}
		if epoch.peerUp == nil {
			continue
		}
		if err := medWholeSetSameEpoch(epoch.peerUp, &row); err != nil {
			return epoch, err
		}
		epoch.route, epoch.routeLine = &row, lineNumber
	}
	return epoch, nil
}

// Stream starts replace the current sender epoch. Endings apply only to its
// connection; another peer or a previously closed connection cannot erase it.
func medWholeSetEndsLocEpoch(row, peerUp *medWholeSetCollectorRow) bool {
	if row.Event == "log_init" {
		return true
	}
	switch row.Message {
	case "init":
		return true
	case "peer_down":
		if row.RIB != medWholeSetLocRIB {
			return false
		}
		if row.Peer != netip.AddrFrom4([4]byte{}) {
			return false
		}
	case "term":
		// Termination applies only to the connection checked below.
	default:
		if row.Event != "log_close" {
			return false
		}
	}
	if peerUp == nil {
		return false
	}
	if peerUp.RouterPort == nil {
		return true
	}
	if row.RouterPort == nil {
		return true
	}
	return *row.RouterPort == *peerUp.RouterPort
}

// Loc-RIB reports omit MED. Their decoded AS_PATH, NEXT_HOP and ORIGIN identify
// the selected source, while the peer header identifies the local BGP instance.
func medWholeSetSelectedCriteria(epoch *medWholeSetLocEpoch, want *medWholeSetCandidate) error {
	if epoch.peerUp.RouterID != want.router {
		return fmt.Errorf("pmacct Loc-RIB Router ID=%s, want %s", epoch.peerUp.RouterID, want.router)
	}
	if epoch.peerUp.ASN == nil {
		return errors.New("pmacct Loc-RIB Peer Up has no ASN")
	}
	if *epoch.peerUp.ASN != 65001 {
		return fmt.Errorf("pmacct Loc-RIB Peer Up ASN=%d, want 65001", *epoch.peerUp.ASN)
	}
	route := epoch.route
	if route.ASN == nil {
		return errors.New("pmacct selected route has no local-instance ASN")
	}
	if *route.ASN != *epoch.peerUp.ASN {
		return errors.New("pmacct selected route does not belong to the current local instance")
	}
	if route.LogType != pmacctLogUpdate {
		return fmt.Errorf("pmacct latest selected row is %q, not an UPDATE", route.LogType)
	}
	var asn [10]byte
	expectedPath := strconv.AppendUint(asn[:0], uint64(want.asn), 10)
	if route.ASPath != string(expectedPath) {
		return fmt.Errorf("pmacct selected AS_PATH=%q, want %d", route.ASPath, want.asn)
	}
	if route.NextHop != want.peer {
		return fmt.Errorf("pmacct selected NEXT_HOP=%s, want %s", route.NextHop, want.peer)
	}
	if route.Origin == nil {
		return errors.New("pmacct selected route has no ORIGIN")
	}
	if *route.Origin != 0 {
		return fmt.Errorf("pmacct selected ORIGIN=%d, want IGP", *route.Origin)
	}
	return nil
}
