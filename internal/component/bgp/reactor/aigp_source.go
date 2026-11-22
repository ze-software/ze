// Design: docs/architecture/route-selection.md -- original source identity for AIGP.
// Related: source_receipt.go -- existing immutable facts, not caller authority.
package reactor

import (
	"errors"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// aigpSourceIdentity separates logical siblings and physical reconnects.
// Its zero value identifies no received source; address is display data only.
// It carries no grant, retention decision or current selecting authority.
type aigpSourceIdentity struct{ owner, connection uint64 }

// aigpOriginalSource reuses the receipt's captured facts without acquiring a
// lease. The caller MUST retain the existing final generation/policy checks:
// looking up facts MUST NOT grant live or another process's retained output.
func aigpOriginalSource(address netip.Addr, update *wireu.WireUpdate) (relaySource, error) {
	if update == nil {
		return relaySource{}, errors.New("aigp: original received wire is missing")
	}
	if update.SourceReceipt() == 0 {
		return relaySource{}, errors.New("aigp: original source receipt is missing")
	}
	if update.SourceOwner() == 0 {
		return relaySource{}, errors.New("aigp: original peer owner is missing")
	}
	source := sourceReceipts.recoverySource(update.SourceReceipt(), update.SourceOwner())
	if !source.ok {
		return relaySource{}, errors.New("aigp: original source receipt is released or belongs to another owner")
	}
	if source.info.filterInfo.ConnectionID == 0 {
		return relaySource{}, errors.New("aigp: original physical connection is missing")
	}
	if source.addr != address {
		return relaySource{}, errors.New("aigp: source address does not match its original receipt")
	}
	if source.ctxID != fwdContextIDWithASN4(update.SourceCtxID(), true) {
		return relaySource{}, errors.New("aigp: source encoding context does not match its original receipt")
	}
	if source.srcID != update.SourceID() {
		return relaySource{}, errors.New("aigp: source encoding identity does not match its original receipt")
	}
	return source, nil
}

func aigpIdentity(source *relaySource) aigpSourceIdentity {
	return aigpSourceIdentity{owner: source.info.peer.sourceOwner, connection: source.info.filterInfo.ConnectionID}
}

const codeAIGPOriginalSource = "doctor-bgp-aigp-original-source"
const aigpOriginalSourceFix = "ze request restart"

func init() {
	if err := diagnostic.Register(diagnostic.CodeMeta{Code: codeAIGPOriginalSource,
		Title:       "BGP AIGP replay lost its original source",
		Description: "Ze refused AIGP retention or replay because the original source receipt, owner or physical connection could not be resolved. The recorded cause identifies the missing original; restart Ze to rebuild session facts.",
		Examples:    []string{aigpOriginalSourceFix, "ze show diagnostic " + codeAIGPOriginalSource}}); err != nil {
		panic("BUG: AIGP original-source diagnostic registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeAIGPOriginalSource}); err != nil {
		panic("BUG: AIGP original-source event registration failed: " + err.Error())
	}
}

// reportAIGPOriginalSourceFailure records a refusal in the existing bounded
// report ring. Caller MUST refuse the operation instead of guessing source facts.
func reportAIGPOriginalSourceFailure(peer netip.Addr, cause error) {
	message := report.FitMessage("BGP AIGP retention or replay refused because its original source is unavailable: " + cause.Error())
	if !report.RaiseErrorFixOnce(reportSourceBGP, codeAIGPOriginalSource, peer.String(), message, aigpOriginalSourceFix, time.Second) {
		return
	}
	fwdLogger().Warn(message, "peer", peer, "code", codeAIGPOriginalSource, "fix", aigpOriginalSourceFix)
}
