// Design: docs/architecture/core-design.md -- native spec lifecycle support
// Related: session.go -- claim, release, WIP, and state-path behavior

package specsession

import "github.com/ze-software/ze/internal/core/textbuf"

// ClaimReport is one claim attempt.
type ClaimReport struct {
	Spec         string    `json:"spec"`
	Status       string    `json:"status"`
	Transitioned bool      `json:"transitioned"`
	Refused      bool      `json:"refused"`
	InProgress   int       `json:"in-progress"`
	Cap          int       `json:"cap"`
	Stalest      []WIPSpec `json:"stalest,omitempty"`
}

// Text renders the claim result.
func (r ClaimReport) Text() string {
	var tb textbuf.Buffer
	if r.Refused {
		tb.Str("spec-session: refusing to start ").Str(r.Spec).Str("\n  ").
			Int(int64(r.InProgress)).Str(" specs are already in-progress (cap ").Int(int64(r.Cap)).Str(").\n").
			Str("  Close one before starting another. Stalest first:\n")
		for _, spec := range r.Stalest {
			tb.Str("    ").Str(spec.Updated).Byte('\t').Str(spec.Spec).Byte('\n')
		}
		return tb.Str("\n  Closing a spec: /ze-close (learned summary + git rm).\n").
			Str("  Deliberately going wider: ZE_SPEC_WIP_CAP=").Int(int64(r.InProgress + 1)).
			Str(" le spec session claim spec ").Str(r.Spec).Byte('\n').String()
	}
	tb.Str("claimed ").Str(r.Spec)
	if r.Transitioned {
		return tb.Str(" (ready -> in-progress; ").Int(int64(r.InProgress)).Byte('/').
			Int(int64(r.Cap)).Str(" in flight)\n").String()
	}
	return tb.Str(" (status: ").Str(firstNonemptyString(r.Status, "unknown")).Str(")\n").String()
}

// currentReport is this session's ownership answer.
type currentReport struct {
	Spec string `json:"spec,omitempty"`
}

// Text preserves the current action's empty-or-one-line output.
func (r currentReport) Text() string {
	if r.Spec == "" {
		return ""
	}
	return r.Spec + "\n"
}

// WIPSpec is one in-progress spec in stalest-first order.
type WIPSpec struct {
	Updated string `json:"updated"`
	Spec    string `json:"spec"`
}

// wipReport is the WIP-cap answer.
type wipReport struct {
	Cap   int       `json:"cap"`
	Specs []WIPSpec `json:"specs"`
}

// Text renders the producer-compatible WIP page.
func (r wipReport) Text() string {
	var tb textbuf.Buffer
	tb.Int(int64(len(r.Specs))).Str(" in-progress spec(s), cap ").Int(int64(r.Cap)).Str(" (stalest first):\n")
	for _, spec := range r.Specs {
		tb.Str("  ").Str(spec.Updated).Byte('\t').Str(spec.Spec).Byte('\n')
	}
	return tb.String()
}

// statePathReport is one resolved state path.
type statePathReport struct {
	Path string `json:"path,omitempty"`
}

// Text renders the path when one exists.
func (r statePathReport) Text() string {
	if r.Path == "" {
		return ""
	}
	return r.Path + "\n"
}

// releaseReport confirms which session claim was released.
type releaseReport struct {
	Released bool `json:"released"`
}

// Text preserves release's intentionally silent output.
func (r releaseReport) Text() string {
	return ""
}
