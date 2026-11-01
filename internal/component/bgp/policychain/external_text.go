// Design: plan/post-release/engineering/spec-filter-wire-0-umbrella.md -- Data Flow step 2, every BGP chain compiles to the library chain
// Related: external.go -- judgeExternal, which hands a member that declared no reads to TextCaller
// Related: ../reactor/filter_library_text.go -- libraryText, the reactor's TextCaller
//
// TRANSITIONAL, deleted in phase 5 with the rest of the text machinery (AC-8).
// Every BGP import and export chain runs as the library chain, and a plugin
// filter that still declares no reads answers the text update. Such a member
// is served here: the reactor renders the received UPDATE as text once,
// sends each text member the text as the earlier text members left it, and
// applies the final text delta beside the folded sets when the chain accepts.
// A text member reads neither an earlier member's sets nor writes a set a
// later member reads, which is what the text chain it replaces offered too.

package policychain

import "errors"

// TextAnswer is a text member's verdict once the reactor folded its delta
// into the running text.
type TextAnswer struct {
	Reject                    bool
	Teardown                  bool
	NotifyCode, NotifySubcode uint8
	// Raw is a whole-body rewrite (D-4): terminal, it ends the chain.
	Raw []byte
}

// TextCaller serves the chain's text members. It MUST be safe for use by the
// one goroutine running the chain; it is not shared between runs.
type TextCaller interface {
	// CallTextFilter sends plugin's filter the running text and folds its
	// delta. A filter that failed (no answer, a malformed or refused answer)
	// answers an error after its own on-error decided: the reactor applies
	// the declared on-error before it answers, as the text chain did, so an
	// error here is always a failed route.
	CallTextFilter(plugin, filter string) (TextAnswer, error)
}

var errExternalTextUnserved = errors.New("the filter declares no reads, so it reads the text update, " +
	"and no text caller serves this run; declare the fields it reads")

// judgeText serves m, a member whose filter declared no reads, through the
// route's TextCaller.
func judgeText(m *member, route *ExternalRoute) (externalVerdict, error) {
	if route.Text == nil {
		return externalVerdict{}, errExternalTextUnserved
	}
	answer, err := route.Text.CallTextFilter(m.external.plugin, m.external.filter)
	if err != nil {
		return externalVerdict{}, err
	}
	verdict := externalVerdict{decision: DecisionPass, teardown: answer.Teardown,
		notifyCode: answer.NotifyCode, notifySub: answer.NotifySubcode, raw: answer.Raw}
	if answer.Reject {
		verdict.decision = DecisionReject
	}
	return verdict, nil
}
