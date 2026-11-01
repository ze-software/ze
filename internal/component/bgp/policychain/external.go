// Design: plan/post-release/engineering/spec-filter-wire-0-umbrella.md -- D-1, D-5, D-9, D-12, a plugin's filter as a chain member
// Related: chain.go -- Run, which judges an external member through judgeExternal
// Related: external_intents.go -- the named intents of a reply's sets
// Related: ../policyview/fields_json.go -- AppendFields, the fields an external member is sent
//
// A chain member may be a filter a plugin runs, named `<plugin>:<filter>` in
// config. The chain sends it the fields it declared, read through the same
// view the in-process members read, so it sees every value an earlier member
// set (read-after-set, AC-13). Its answer's sets fold into the same Sets an
// in-process member writes, so both kinds produce the same UPDATE bytes
// (D-5). A set the filter did not declare, a field the attachment does not
// let a filter set, an action the field does not take and a value of the
// wrong shape each fail the route under the filter's on-error, and none of
// that route's sets apply (D-9).

package policychain

import (
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Member names one chain member: a filter instance of the policy registry,
// or, when Plugin is set, the filter Name that plugin declared at Stage 1.
type Member struct {
	Name   string
	Plugin string
}

// ExternalDeclaration is what a plugin declared for one filter at Stage 1.
type ExternalDeclaration struct {
	// Reads holds vocabulary field names, and rpc.FilterFieldRaw for the raw
	// UPDATE body. A filter that declares none reads the text update, which
	// this chain does not send.
	Reads []string
	// Sets holds the vocabulary fields the filter may set.
	Sets    []string
	OnError rpc.OnErrorPolicy
}

// ExternalCaller reaches the filters plugins run. It MUST be safe for
// concurrent use: every goroutine that runs a chain calls it.
type ExternalCaller interface {
	// FilterDeclaration answers what plugin declared for filter, ok false
	// when it has declared no such filter (it has not reached Stage 1, or
	// it has gone).
	FilterDeclaration(plugin, filter string) (ExternalDeclaration, bool)
	// CallFilter judges route by plugin's filter and answers its verdict:
	// through the plugin's typed filter slot, the view read in place, when the
	// plugin runs internal, and in a filter-update batch bounded by
	// rpc.FilterUpdateTimeout, the fields encoded, when it runs forked
	// (D-12). It MUST NOT keep route past its return.
	CallFilter(plugin string, route *rpc.FilterRoute) (*rpc.FilterUpdateOutput, error)
}

// ExternalRoute is what an external member needs beside the view: the caller,
// the direction and peer the message names, and the raw body a filter that
// declared rpc.FilterFieldRaw is sent. Raw is borrowed for the run; the
// caller MUST clear the route (SetRoute(ExternalRoute{})) before the RunSets
// returns to a pool.
type ExternalRoute struct {
	Caller    ExternalCaller
	Direction string // "import" or "export"
	Peer      string
	PeerAS    uint32
	Raw       []byte
}

// externalMember is the plugin side of a member.
type externalMember struct {
	plugin, filter string
}

// fieldsOctets is the starting capacity of the fields object RunSets keeps:
// a prefix, an AS path and a few communities fit, and a larger route grows
// the slice once, then keeps it.
const fieldsOctets = 512

var (
	errExternalUndeclared = errors.New("the plugin declares no such filter: it has not started, or it stopped")
	errExternalTextUpdate = errors.New("the filter declares no reads, so it reads the text update, " +
		"which a chain holding in-process members does not send; declare the fields it reads")
	errExternalAnsweredText = errors.New("the filter answered a text update; a chain holding in-process members " +
		"applies only sets")
	errExternalAnsweredRaw = errors.New("the filter answered a raw rewrite, which a chain holding in-process " +
		"members does not apply")
	errExternalAction = errors.New("the filter answered an action that is neither accept, reject nor modify")
	errExternalSetsOn = errors.New("the filter answered sets without action modify")
)

// externalVerdict is what an external member's answer asks beside its sets.
// A teardown is honoured on import only (Run), as the text chain does: on
// export the decision stands.
type externalVerdict struct {
	decision              Decision
	teardown              bool
	notifyCode, notifySub uint8
}

// judgeExternal sends m's filter the route view holds and folds its answer's
// sets into sets. It answers the on-error that decides a failure: the
// declared one, or reject when the filter is not declared.
func (c *Chain) judgeExternal(m *member, view *policyview.View, run *RunSets) (externalVerdict, rpc.OnErrorPolicy, error) {
	route := &run.route
	if route.Caller == nil {
		return externalVerdict{}, rpc.OnErrorReject, errors.New("no plugin server reaches the filter")
	}
	decl, ok := route.Caller.FilterDeclaration(m.external.plugin, m.external.filter)
	if !ok {
		return externalVerdict{}, rpc.OnErrorReject, errExternalUndeclared
	}
	if len(decl.Reads) == 0 {
		return externalVerdict{}, decl.OnError, errExternalTextUpdate
	}
	header := rpc.FilterUpdateInput{
		Filter:    m.external.filter,
		Direction: route.Direction,
		Peer:      route.Peer,
		PeerAS:    route.PeerAS,
	}
	run.call.begin(run, view, decl.Reads)
	defer run.call.end()
	call := rpc.ViewFilterRoute(&header, &run.call)
	out, err := route.Caller.CallFilter(m.external.plugin, &call)
	if err != nil {
		return externalVerdict{}, decl.OnError, err
	}
	verdict, err := externalAnswer(out, &decl, m.settable, &run.member, intentTarget{view: view, prefix: c.prefix})
	return verdict, decl.OnError, err
}

// externalAnswer maps one reply onto a decision and the member's sets.
func externalAnswer(out *rpc.FilterUpdateOutput, decl *ExternalDeclaration, settable setMask, sets *Sets, target intentTarget) (externalVerdict, error) {
	verdict := externalVerdict{teardown: out.Teardown, notifyCode: out.NotifyCode, notifySub: out.NotifySubcode}
	if out.Update != "" {
		return verdict, errExternalAnsweredText
	}
	switch out.Action {
	case rpc.FilterAccept, rpc.FilterReject:
		if len(out.Sets) > 0 {
			return verdict, errExternalSetsOn
		}
		verdict.decision = DecisionPass
		if out.Action == rpc.FilterReject {
			verdict.decision = DecisionReject
		}
		return verdict, nil
	case rpc.FilterModify:
		if len(out.Raw) > 0 {
			return verdict, errExternalAnsweredRaw
		}
		if err := applyExternalSets(sets, decl, settable, out.Sets, target); err != nil {
			return verdict, err
		}
		verdict.decision = DecisionPass
		return verdict, nil
	case rpc.FilterUnspecified:
	}
	return verdict, errExternalAction
}

// applyExternalSets writes a reply's sets through the Sets methods an
// in-process member calls (D-5). The first refused set fails the route, and
// the caller discards the member's Sets, so none of the route's sets apply.
// target is what the named intents resolve against.
func applyExternalSets(sets *Sets, decl *ExternalDeclaration, settable setMask, replies []rpc.FilterSet, target intentTarget) error {
	for i := range replies {
		set := &replies[i]
		if !slices.Contains(decl.Sets, set.Field) {
			return fmt.Errorf("%w: %s", errUndeclaredSet, set.Field)
		}
		kind, ok := kindNamed(set.Field)
		if !ok {
			return fmt.Errorf("sets %q, which no BGP chain can set", set.Field)
		}
		if settable&kind.bit() == 0 {
			return fmt.Errorf("sets %s, which a filter at this attachment may not set", set.Field)
		}
		if err := applyExternalSet(sets, kind, set, target); err != nil {
			return fmt.Errorf("%s %s: %w", set.Field, set.Action, err)
		}
	}
	return nil
}

func applyExternalSet(sets *Sets, kind setKind, set *rpc.FilterSet, target intentTarget) error {
	switch kind {
	case setKindLocalPreference, setKindMED:
		action, ok := numericActionOf(set.Action)
		if !ok {
			return errors.New("the field takes set, increment or decrement")
		}
		value, err := strconv.ParseUint(string(set.Value), 10, 32)
		if err != nil {
			return errors.New("the value is not a number from 0 to 4294967295")
		}
		if kind == setKindMED {
			return sets.MED(action, uint32(value))
		}
		return sets.LocalPreference(action, uint32(value))
	case setKindCommunity:
		community, err := communityValue(set.Value)
		if err != nil {
			return err
		}
		switch set.Action {
		case rpc.FilterSetActionAdd:
			return sets.AddCommunity(community)
		case rpc.FilterSetActionRemove:
			return sets.RemoveCommunity(community)
		case rpc.FilterSetActionSet, rpc.FilterSetActionIncrement, rpc.FilterSetActionDecrement,
			rpc.FilterSetActionPrepend:
		}
		return errors.New("the field takes add or remove")
	case setKindASPath:
		return applyASPathIntent(sets, set)
	case setKindPrefix:
		return applyPrefixIntent(sets, set, target)
	case setKindFamily:
		return applyFamilyIntent(sets, set)
	case setKindUnspecified, setKindCount:
	}
	return fmt.Errorf("BUG: policychain: set kind %d has no external form", kind)
}

// kindNamed answers the set kind whose vocabulary field is named name.
func kindNamed(name string) (setKind, bool) {
	for kind := setKindLocalPreference; kind < setKindCount; kind++ {
		if setKindFields[kind].String() == name {
			return kind, true
		}
	}
	return setKindUnspecified, false
}

// numericActionOf maps the open wire action onto a numeric action.
func numericActionOf(action rpc.FilterSetAction) (NumericAction, bool) {
	switch action {
	case rpc.FilterSetActionSet:
		return NumericActionSet, true
	case rpc.FilterSetActionIncrement:
		return NumericActionIncrement, true
	case rpc.FilterSetActionDecrement:
		return NumericActionDecrement, true
	case rpc.FilterSetActionAdd, rpc.FilterSetActionRemove, rpc.FilterSetActionPrepend:
	}
	return NumericActionUnspecified, false
}

// communityValue reads a community string, the shape the UPDATE event's attr
// map writes ("65000:100").
func communityValue(value []byte) (uint32, error) {
	text, ok := plainString(value)
	if !ok {
		return 0, errors.New("the value is not a community string such as \"65000:100\"")
	}
	community, err := attribute.ParseCommunity(text)
	if err != nil {
		return 0, err
	}
	return community, nil
}

// externalSettable answers the kinds the attachment lets a filter set, the
// mask an external member's writes are checked against beside its own
// declaration (D-9). policy.CheckDeclaredFilter makes the judgement Stage 1
// made of the declaration, field by field.
func externalSettable(attachment, reference string) setMask {
	var mask setMask
	for kind := setKindLocalPreference; kind < setKindCount; kind++ {
		if policy.CheckDeclaredFilter(attachment, reference, nil, []string{setKindFields[kind].String()}) == nil {
			mask |= kind.bit()
		}
	}
	return mask
}
