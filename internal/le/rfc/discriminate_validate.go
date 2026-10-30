// Design: docs/architecture/core-design.md -- the recorded proof behind one RFC tag
// Overview: discriminate.go -- the record, its loader and its verifier
//
// discriminate_validate.go holds the shape rules a record must meet before it
// is read at all: its route, its keys, its paired breaks, its solo runs and
// its escape. The loader and the recorder both call them, so a record is
// refused for the same reason whichever side meets it.
package rfc

import (
	"path"
	"regexp"
	"slices"
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
func validateDiscrimination(record *DiscriminationRecord, rel string, position int) error {
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
	if err := refuseStrayKernelFields(record, where); err != nil {
		return err
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
func validateEscapeShape(record *DiscriminationRecord, where string) error {
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
	if record.Reason == escapeKernel {
		return validateKernelEscape(record, where)
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
func validateForeignEscape(record *DiscriminationRecord, where string) error {
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

// kernelFunctionRE is the shape of a kernel producer key: a relative path to a
// C file in the Linux source tree, then `::` and the function's name.
var kernelFunctionRE = regexp.MustCompile(`^[a-z0-9_]+(?:/[A-Za-z0-9_.-]+)*\.c::[A-Za-z_][A-Za-z0-9_]*$`)

// refuseStrayKernelFields refuses the kernel fields on any record but a
// kernel-producer escape.
//
// Both say something only that escape means: which kernel function produces
// the behavior, and which route could still prove it. On a proof they would
// read as a second producer nobody broke, and on another escape as a promise
// nothing checks.
func refuseStrayKernelFields(record *DiscriminationRecord, where string) error {
	if record.Reason == escapeKernel {
		return nil
	}
	var tb textbuf.Buffer
	if len(record.KernelFunctions) > 0 {
		return parseErr(tb.Str(where).Str(" names the kernel function ").Quoted(record.KernelFunctions[0]).
			Str(" and is not a ").Str(escapeKernel).Str(" escape. Only that escape names ").
			Str("the kernel code a claim rests on: remove the field"))
	}
	if record.LaterRoute != "" {
		return parseErr(tb.Str(where).Str(" names the later route ").Quoted(record.LaterRoute).
			Str(" and is not a ").Str(escapeKernel).Str(" escape. Only that escape names ").
			Str("a route that could still prove it: remove the field"))
	}
	if len(record.ZeStateExcluded) > 0 {
		return parseErr(tb.Str(where).Str(" carries ze-state-excluded and is not a ").
			Str(escapeKernel).Str(" escape. Only that escape answers for the Ze state its ").
			Str("unit installs: remove the field"))
	}
	return nil
}

// validateKernelEscape refuses the kernel escape that cannot be checked.
//
// The reason says Linux produces the behavior from the packet alone, so there
// is no producer in this tree to name and the record MUST NOT name one. What it
// names instead is each kernel function, which the gate matches against the
// summary's own Implementation reason, an exclusion for each Ze producer the
// unit reaches, whose reason names that producer, and the span of the tag's claim that
// states the kernel's behavior, which ties the escape to THIS claim rather than
// to every Linux integration test at once (R-9). The later route records that a
// patched kernel could still observe the red, so the row stays owed work.
func validateKernelEscape(record *DiscriminationRecord, where string) error {
	var tb textbuf.Buffer
	tb.Str(where).Str(" is ").Str(escapeKernel)
	if record.Producer != "" {
		return parseErr(tb.Str(" and names the producer ").Quoted(record.Producer).
			Str(". That reason says Linux produces the behavior and no state Ze installs ").
			Str("decides it, so there is no producer here to name. Where Ze's code can be ").
			Str("broken, take the ").Str(RouteMutant).Str(" or ").Str(RouteRevert).Str(" route"))
	}
	if len(record.KernelFunctions) == 0 {
		return parseErr(tb.Str(" and names no kernel function. Name each one the claim rests ").
			Str("on as <linux source path>::<function>, as in net/ipv4/ip_forward.c::ip_forward"))
	}
	for index, function := range record.KernelFunctions {
		if !kernelFunctionRE.MatchString(function) {
			return parseErr(tb.Str(" and names the kernel function ").Quoted(function).
				Str(". Name it as <linux source path>::<function>, as in ").
				Str("net/ipv4/ip_forward.c::ip_forward, the same spelling the RFC summary's ").
				Str("Implementation reason uses"))
		}
		if slices.Contains(record.KernelFunctions[:index], function) {
			return parseErr(tb.Str(" and names the kernel function ").Quoted(function).
				Str(" twice. Name each function once"))
		}
	}
	if err := validateZeStateExcluded(record.ZeStateExcluded, where); err != nil {
		return err
	}
	if strings.TrimSpace(record.Citation) == "" {
		return parseErr(tb.Str(" and cites no span of the tag's claim. Cite the words of ").
			Str("the claim that state the kernel's behavior: they tie the escape to THIS claim"))
	}
	if !laterRoutes[record.LaterRoute] {
		return parseErr(tb.Str(" and names the later route ").Quoted(record.LaterRoute).
			Str(", want one of: ").Join(laterRouteNames(), ", ").
			Str(". The escape publishes debt, and the later route says what would prove it"))
	}
	return nil
}

// validateZeStateExcluded refuses an exclusion whose reason cannot be judged.
//
// Whether the reason is true is a judge's question, and no gate answers it.
// What a gate can refuse is the reason that says nothing: one shorter than
// kernelExclusionWordsMin words, one that does not name the producer it
// excuses (its symbol, or the file's base name for a file-scoped key), and one
// copied verbatim onto a second producer, which is generic by construction.
func validateZeStateExcluded(exclusions []ZeStateExclusion, where string) error {
	var tb textbuf.Buffer
	tb.Str(where).Str(" excludes Ze state")
	for index := range exclusions {
		entry := exclusions[index]
		file, symbol, err := fingerprintKey(entry.Producer, where)
		if err != nil {
			return err
		}
		named := path.Base(file)
		if symbol != "" {
			named = symbol[strings.LastIndexByte(symbol, '.')+1:]
		}
		reason := strings.Join(strings.Fields(entry.Reason), " ")
		if len(strings.Fields(reason)) < kernelExclusionWordsMin {
			return parseErr(tb.Str(" of ").Quoted(entry.Producer).Str(" with the reason ").
				Quoted(entry.Reason).Str(", fewer than ").Int(int64(kernelExclusionWordsMin)).
				Str(" words. Say what state the producer installs and why the claim's outcome ").
				Str("does not rest on it"))
		}
		if !namesToken(reason, named) {
			return parseErr(tb.Str(" of ").Quoted(entry.Producer).Str(" with a reason that ").
				Str("does not name ").Str(named).Str(". A reason that names no producer reads ").
				Str("the same over every producer: say what ").Str(named).Str(" installs"))
		}
		for earlier := range index {
			if strings.Join(strings.Fields(exclusions[earlier].Reason), " ") == reason {
				return parseErr(tb.Str(" of ").Quoted(entry.Producer).Str(" with the reason ").
					Str("already given for ").Quoted(exclusions[earlier].Producer).
					Str(". Each producer installs its own state: say why each one's does not ").
					Str("decide the outcome"))
			}
		}
	}
	return nil
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
