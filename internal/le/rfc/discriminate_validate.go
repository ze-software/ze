// Design: docs/architecture/core-design.md -- the recorded proof behind one RFC tag
// Overview: discriminate.go -- the record, its loader and its verifier
//
// discriminate_validate.go holds the shape rules a record must meet before it
// is read at all: its route, its keys, its paired breaks, its solo runs and
// its escape. The loader and the recorder both call them, so a record is
// refused for the same reason whichever side meets it.
package rfc

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// validateDiscrimination refuses a record that cannot mean what it says.
//
// position is 1-based, because it is read by a person counting rows in a file.
//
// The two keys are parsed by fingerprintKey, the ONE key parser this package
// has. A record therefore inherits its path-safety refusal and, with it, the
// refusal of the retired `<path>:<line>` form -- which is the shape a record
// must never take, because a stored line is current only until the next edit
// above it.
func validateDiscrimination(record DiscriminationRecord, rel string, position int) error {
	var tb textbuf.Buffer
	tb.Str(rel).Str(": record ").Int(int64(position))
	if record.RID == "" {
		return parseErr(tb.Str(" names no requirement: 'rid' is empty"))
	}
	where := tb.Str(" for ").Str(record.RID).String()
	if !polarities[record.Polarity] {
		return parseErr(tb.Str(" has polarity ").Quoted(record.Polarity).
			Str(", want one of: ").Join(Polarities(), ", "))
	}
	if _, _, err := fingerprintKey(record.Unit, where); err != nil {
		return err
	}
	if err := validateBehaviorSHA(record.UnitSHA, where, "unit-sha"); err != nil {
		return err
	}
	if err := validateBehaviorSHA(record.ClaimSHA, where, "claim-sha"); err != nil {
		return err
	}
	if !discriminationRoutes[record.Route] {
		return parseErr(tb.Str(" has route ").Quoted(record.Route).
			Str(", want one of: ").Join(DiscriminationRoutes(), ", "))
	}
	if !record.Proves() {
		if len(record.Paired) > 0 {
			return parseErr(tb.Reset().Str(where).Str(" takes the ").Str(RouteNoBreak).
				Str(" route and carries paired breaks. The escape states that NO break exists"))
		}
		if len(record.Solo) > 0 {
			return parseErr(tb.Reset().Str(where).Str(" takes the ").Str(RouteNoBreak).
				Str(" route and carries solo runs. The escape states that NO break exists"))
		}
		return validateEscapeShape(record, where)
	}
	if record.Reason != "" {
		return parseErr(tb.Reset().Str(where).Str(" takes the ").Str(record.Route).
			Str(" route and carries the escape reason ").Quoted(record.Reason).
			Str(". A reason says why NO break exists; this record states one"))
	}
	symbol, err := producerSymbol(record.Producer, where, record.Route)
	if err != nil {
		return err
	}
	if symbol == "" {
		return parseErr(tb.Reset().Str(where).Str(" has producer ").Quoted(record.Producer).
			Str(", which names a whole file. A break is applied to a FUNCTION, so the ").
			Str("producer key carries the symbol it was applied to"))
	}
	if err := validateBehaviorSHA(record.ProducerSHA, where, "producer-sha"); err != nil {
		return err
	}
	if strings.TrimSpace(record.Break) == "" {
		return parseErr(tb.Reset().Str(where).Str(" takes the ").Str(record.Route).
			Str(" route and states no break. The record exists so a reviewer can read what ").
			Str("was done to the producer; a proof nobody can read is a claim"))
	}
	for index := range record.Paired {
		if err := validatePairedBreak(record.Paired[index], where, record.Route); err != nil {
			return err
		}
	}
	return validateSoloRuns(record, where)
}

// validatePairedBreak holds one paired break to the rules the record's own
// producer, fingerprint and break text are held to.
func validatePairedBreak(paired PairedBreak, where, route string) error {
	symbol, err := producerSymbol(paired.Producer, where, route)
	if err != nil {
		return err
	}
	var tb textbuf.Buffer
	if symbol == "" {
		return parseErr(tb.Str(where).Str(" has the paired producer ").Quoted(paired.Producer).
			Str(", which names a whole file. A break is applied to a FUNCTION"))
	}
	if err := validateBehaviorSHA(paired.ProducerSHA, where, "paired producer-sha"); err != nil {
		return err
	}
	if strings.TrimSpace(paired.Break) == "" {
		return parseErr(tb.Str(where).Str(" pairs ").Str(paired.Producer).
			Str(" and states no break for it. A reviewer reads what was done to each producer"))
	}
	return nil
}

// producerSymbol answers the function half of a proof record's producer key.
func producerSymbol(producer, where, route string) (string, error) {
	if producer == "" {
		var tb textbuf.Buffer
		return "", parseErr(tb.Str(where).Str(" takes the ").Str(route).
			Str(" route and names no producer. A break is a change to code the claim rests ").
			Str("on, so the record names that code as <path>::<FuncName>"))
	}
	_, symbol, err := fingerprintKey(producer, where)
	return symbol, err
}

// validateEscapeShape refuses an escape that cannot be checked, or that reads
// as a proof.
//
// A no-break record states that no break exists, so a break text or a producer
// fingerprint on it is a proof half-written, and counting it as debt while it
// reads as evidence is the confusion the escape exists to avoid.
//
// Every escape names what ties it to THIS claim, and which field carries the
// tie depends on whether there is code here to point at. escapeDeclaration and
// escapeGenerated each claim something about the CODE the claim rests on, so
// they name that code as the producer and carry no citation: their tie is the
// claim naming what that file declares. escapeForeign claims there is no such
// code in this tree, so it names no producer and CITES the assertion its own
// checker makes instead. A record carrying neither tie is the blanket opt-out
// this vocabulary exists to refuse (R-9), and discriminate_escape.go is where
// each claim is checked against the tree.
func validateEscapeShape(record DiscriminationRecord, where string) error {
	var tb textbuf.Buffer
	tb.Str(where).Str(" takes the ").Str(RouteNoBreak).Str(" route and carries ")
	switch {
	case record.ProducerSHA != "":
		return parseErr(tb.Str("a producer fingerprint. The escape states that NO break exists"))
	case record.Break != "":
		return parseErr(tb.Str("a break. The escape states that NO break exists"))
	}
	if !escapeReasons[record.Reason] {
		return parseErr(tb.Reset().Str(where).Str(" takes the ").Str(RouteNoBreak).
			Str(" route and gives the reason ").Quoted(record.Reason).
			Str(", want one of: ").Join(escapeReasonNames(), ", ").
			Str(". Each reason claims a fact this gate goes and checks; free text would ").
			Str("let \"there is nothing to break here\" read as \"this tag is proven\""))
	}
	if record.Reason == escapeForeign {
		return validateForeignEscape(record, where)
	}
	if record.Citation != "" {
		return parseErr(tb.Reset().Str(where).Str(" is ").Str(record.Reason).
			Str(" and cites the assertion ").Quoted(record.Citation).
			Str(". A citation is the tie only where no producer can be named. This reason ").
			Str("names one, and its tie is the claim naming what that file declares"))
	}
	if record.Producer == "" {
		return parseErr(tb.Reset().Str(where).Str(" is ").Str(record.Reason).
			Str(" and names no producer. That reason claims something about the code the ").
			Str("claim rests on, so the record names that code and the gate goes and reads it"))
	}
	_, _, err := fingerprintKey(record.Producer, where)
	return err
}

// validateForeignEscape refuses the foreign escape that names nothing.
//
// The reason says the behavior is produced outside this repository, so there is
// no producer here to name and the record MUST NOT name one. What it must carry
// instead is the citation: the carrier kind is a property of all 37 interop tags
// at once, so on its own it discharges every one of them equally, and the
// assertion number is the only field that says which claim this escape is about.
func validateForeignEscape(record DiscriminationRecord, where string) error {
	var tb textbuf.Buffer
	if record.Producer != "" {
		return parseErr(tb.Str(where).Str(" is ").Str(escapeForeign).
			Str(" and names the producer ").Quoted(record.Producer).
			Str(". That reason says the behavior is produced OUTSIDE this repository, ").
			Str("so there is no producer here to name"))
	}
	if record.Citation != "" {
		return nil
	}
	return parseErr(tb.Str(where).Str(" is ").Str(escapeForeign).
		Str(" and cites no assertion. No code in this tree produces the behavior, so the ").
		Str("assertion this checker makes about it is the only thing that ties the escape to ").
		Str("THIS claim: cite the `fail(N, ...)` number the checker writes out"))
}

// validateBehaviorSHA refuses a fingerprint that is not one.
//
// Length alone is not the check, for the reason validateSHA already states: a
// value padded to the right width is still not a fingerprint, so the charset is
// judged too. shaRE is that shape, declared once and shared.
func validateBehaviorSHA(value, where, field string) error {
	if shaRE.MatchString(value) {
		return nil
	}
	var tb textbuf.Buffer
	return parseErr(tb.Str(where).Str(" has ").Str(field).Str(" ").Quoted(value).
		Str(", which is not a fingerprint. Expected exactly ").Int(shaHexLen).
		Str(" lowercase hex characters, as ./le rfc discriminate computes them from the ").
		Str("unit's behavior. A hand-written fingerprint is never valid: it would publish a ").
		Str("red nobody observed"))
}
