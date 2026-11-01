// Design: docs/architecture/core-design.md — the in-process BGP chain, display-only trace
// Related: chain.go — Chain.run, which Trace shares with Run
// Related: sets.go — Sets, which AppendText renders

package policychain

import (
	"strconv"

	"github.com/ze-software/ze/internal/component/bgp/policyview"
)

// TraceStep is what one member did in a display-only run.
type TraceStep uint8

const (
	TraceUnspecified TraceStep = iota
	// TracePass: the member passed the route, and its sets folded.
	TracePass
	// TraceReject: the member rejected the route (a policy decision).
	TraceReject
	// TraceFailClosed: the member failed, and its on-error rejected the route.
	TraceFailClosed
	// TraceFailOpen: the member failed, and its on-error passed the route on
	// with none of its sets.
	TraceFailOpen
)

// MemberTrace is one member's step of a display-only run.
type MemberTrace struct {
	// Name is the member's reference: the instance name, or plugin:filter.
	Name string
	Step TraceStep
	// Err is the failure of a TraceFailClosed or TraceFailOpen step.
	Err error
	// Sets is what a passing member asked, resolved as it folded; nil for
	// every other step. It is the run's scratch: valid only during the call.
	Sets *Sets
}

// Trace judges the route view holds exactly as Run does, and hands record
// each member's step in order. It is display only (show policy test): a
// failure is neither counted in Refused nor logged, because no route was
// refused. The caller owns view and sets as for Run.
func (c *Chain) Trace(view *policyview.View, sets *RunSets, record func(MemberTrace)) Outcome {
	if record == nil {
		panic("BUG: policychain: Trace without a recorder")
	}
	return c.run(view, sets, record)
}

// traceStep hands record one member's step, and does nothing on a live run.
func traceStep(record func(MemberTrace), m *member, step TraceStep, err error, sets *Sets) {
	if record == nil {
		return
	}
	name := m.name
	if m.external != nil {
		name = m.external.plugin + ":" + m.external.filter
	}
	record(MemberTrace{Name: name, Step: step, Err: err, Sets: sets})
}

// AppendText appends the sets as one line of text for an operator, in the
// directive words the text filter answer uses where one exists:
// "local-preference 200 med 50 community add 65001:100 as-path-prepend 2".
// A numeric set is shown resolved when the chain resolved it (a passing
// member's sets), else as its action. It is display only, never parsed back.
func (s *Sets) AppendText(buf []byte) []byte {
	if s.written&setKindLocalPreference.bit() != 0 {
		buf = appendNumericText(buf, "local-preference", s.localPreference)
	}
	if s.written&setKindMED.bit() != 0 {
		buf = appendNumericText(buf, "med", s.med)
	}
	for _, community := range s.communityRemove[:s.communityRemN] {
		buf = appendCommunityText(appendWord(buf, "community remove"), community)
	}
	for _, community := range s.communityAdd[:s.communityAddN] {
		buf = appendCommunityText(appendWord(buf, "community add"), community)
	}
	switch s.intents.removePrivate {
	case RemovePrivateStrip:
		buf = appendWord(buf, "remove-private-as")
	case RemovePrivatePeerAS:
		buf = appendWord(buf, "remove-private-as replace-peer")
	case RemovePrivateUnspecified:
	}
	if s.intents.prepend > 0 {
		buf = strconv.AppendUint(appendWord(buf, "as-path-prepend "), uint64(s.intents.prepend), 10)
	}
	for section := NLRISectionAnnounce; section < nlriSectionCount; section++ {
		for _, offset := range s.intents.drops[section] {
			buf = appendWord(buf, "nlri drop")
			buf = append(buf, ' ')
			buf = append(buf, nlriSectionWords[section]...)
			buf = strconv.AppendUint(append(buf, " offset "...), uint64(offset), 10)
		}
	}
	for _, fam := range s.intents.families[:s.intents.familiesN] {
		buf = append(appendWord(buf, "family strip "), fam.String()...)
	}
	return buf
}

// nlriSectionWords names each section in AppendText.
var nlriSectionWords = [nlriSectionCount]string{
	NLRISectionAnnounce:  "announce",
	NLRISectionWithdrawn: "withdrawn",
	NLRISectionMPReach:   "mp-reach",
	NLRISectionMPUnreach: "mp-unreach",
}

// appendWord appends word, with a separating space after earlier text.
func appendWord(buf []byte, word string) []byte {
	if len(buf) > 0 {
		buf = append(buf, ' ')
	}
	return append(buf, word...)
}

func appendNumericText(buf []byte, name string, n numericSet) []byte {
	buf = appendWord(buf, name)
	if n.replace && n.delta == 0 {
		return strconv.AppendUint(append(buf, ' '), uint64(n.base), 10)
	}
	if n.replace {
		buf = strconv.AppendUint(append(buf, " set "...), uint64(n.base), 10)
	}
	if n.delta >= 0 {
		return strconv.AppendInt(append(buf, " increment "...), n.delta, 10)
	}
	return strconv.AppendInt(append(buf, " decrement "...), -n.delta, 10)
}

// appendCommunityText appends " ASN:value", RFC 1997's display form.
func appendCommunityText(buf []byte, community uint32) []byte {
	buf = strconv.AppendUint(append(buf, ' '), uint64(community>>16), 10)
	return strconv.AppendUint(append(buf, ':'), uint64(community&0xffff), 10)
}
