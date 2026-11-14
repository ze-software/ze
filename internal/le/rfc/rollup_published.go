// Design: docs/contributing/rfc-conformance-gates.md -- Coverage figures, the published rollup
// Related: profilecoverage.go -- Rollup, rollupOf, mustProven
// Related: priority.go -- ScopeRollup
// Related: tree.go -- TreeRow.MarshalJSON, the row form this one follows
package rfc

import (
	"encoding/json"
	"errors"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// rollupJSON is the published form of a Rollup: the same flat figure keys a
// tree row publishes, and the section counts whole under `sections`.
// `mandatory-proven` is the published proven count, MustSections.Proven: Ze
// proven plus OS verified (mustProven), so it agrees with every other
// published proven count. The figures' Attainable and Excluded, and the open
// and OS splits they come from, are published only inside the sections, and
// the intent counts nowhere.
type rollupJSON struct {
	RFCs            int      `json:"rfcs"`
	NotWalked       int      `json:"not-walked"`
	MandatoryProven int      `json:"mandatory-proven"`
	MandatoryTotal  int      `json:"mandatory-total"`
	OptionalProven  int      `json:"optional-proven"`
	OptionalTotal   int      `json:"optional-total"`
	Sections        Sections `json:"sections"`
}

// scopeRollupJSON is the published form of a ScopeRollup: the rollup's keys,
// then its own. ScopeRollup carries its own JSON methods, because the ones it
// would promote from Rollup encode the rollup alone and drop
// `mandatory-complete`.
type scopeRollupJSON struct {
	rollupJSON
	MandatoryComplete bool `json:"mandatory-complete"`
}

// published answers the rollup's published form.
func (r *Rollup) published() rollupJSON {
	return rollupJSON{RFCs: r.RFCs, NotWalked: r.NotWalked,
		MandatoryProven: r.Sections.Must.Proven(), MandatoryTotal: r.Mandatory.Total,
		OptionalProven: r.Optional.Proven, OptionalTotal: r.Optional.Total, Sections: r.Sections}
}

// rollupOfPublished reads a published rollup back. The published
// `mandatory-proven` holds the OS verified rows the sections carry, so they
// are taken back out of it to restore Ze proven: the decoded rollup's
// Mandatory.Proven is then the live rollup's, and its sections' Proven the
// published count. A count smaller than its own OS verified rows was not
// written by published, and is refused rather than decoded into a negative Ze
// proven count. The intent counts are published nowhere, so the decoded
// rollup holds none (intent nil) and prints no intent clause.
func rollupOfPublished(wire *rollupJSON) (Rollup, error) {
	osVerified := wire.Sections.Must.OSVerified
	if osVerified > wire.MandatoryProven {
		var tb textbuf.Buffer
		return Rollup{}, errors.New(tb.Str("the published rollup counts ").Int(int64(wire.MandatoryProven)).
			Str(" MUST rows proven but ").Int(int64(osVerified)).
			Str(" OS verified in its sections, and the proven count includes the OS verified rows, ").
			Str("so the file was not written by Rollup.MarshalJSON. Regenerate it with `./le rfc tree` or `./le site build`").String())
	}
	return Rollup{RFCs: wire.RFCs, NotWalked: wire.NotWalked,
		Mandatory: Figure{Proven: wire.MandatoryProven - osVerified, Total: wire.MandatoryTotal},
		Optional:  Figure{Proven: wire.OptionalProven, Total: wire.OptionalTotal},
		Sections:  wire.Sections}, nil
}

// MarshalJSON publishes the rollup with its figures under the flat keys. The
// receiver is a value, so a rollup held by value in another struct publishes
// the same form.
func (r Rollup) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.published())
}

// UnmarshalJSON reads a published rollup back (rollupOfPublished).
func (r *Rollup) UnmarshalJSON(data []byte) error {
	var wire rollupJSON
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	decoded, err := rollupOfPublished(&wire)
	if err != nil {
		return err
	}
	*r = decoded
	return nil
}

// MarshalJSON publishes the rollup's form and its completeness. The receiver
// is a value: TreeTier and NextReport hold a ScopeRollup by value.
func (r ScopeRollup) MarshalJSON() ([]byte, error) {
	return json.Marshal(scopeRollupJSON{rollupJSON: r.published(), MandatoryComplete: r.MandatoryComplete})
}

// UnmarshalJSON reads a published scope rollup back: the rollup as
// rollupOfPublished restores it, and its completeness as published.
func (r *ScopeRollup) UnmarshalJSON(data []byte) error {
	var wire scopeRollupJSON
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	decoded, err := rollupOfPublished(&wire.rollupJSON)
	if err != nil {
		return err
	}
	*r = ScopeRollup{Rollup: decoded, MandatoryComplete: wire.MandatoryComplete}
	return nil
}
