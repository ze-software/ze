// Design: docs/architecture/bgp/filter-path-asn.md -- the reject-asn filter plugin
// Detail: curated.go -- the table this completion reads
// Related: internal/component/cli/completer.go -- validateCompletions, the consumer
//
// Completion for the ASN leaf-lists of a reject-asn list: the five position
// leaf-lists direct, indirect, transit, origin and anywhere, plus the `asn`
// leaf-list inside an `nth` entry.
//
// Each carries `ze:validate "transit-asn"`, and the registration below
// declares that name as a SUGGESTION: curatedASNValues says which ASNs are
// worth offering, curatedASNHelp says which network each one is, and nothing
// says what is valid. RegisterSuggestion
// (internal/component/config/yang/validator_registry.go) writes a validator
// with no ValidateFn, and applyCustomValidators skips it.
//
// That is the whole point, and it is the decision the curated table exists
// under: a suggestion is NEVER a constraint. Every uint32 the leaf's YANG type
// admits stays valid, an ASN this table has never heard of is accepted with no
// warning, and an operator who ignores the dropdown loses nothing.
// TestCompletionIsNeverAConstraint holds that line.
//
// The spec calls this file complete.go. It is named register_completion.go
// because the pretool-writeedit gate refuses a Register call inside init()
// outside a file whose name starts with "register", and register.go itself
// carries the plugin registration.

package filter_path_asn

import (
	"strconv"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// curatedValidatorName is the ze:validate name the `asn` leaf-list carries in
// ze-filter-path-asn.yang. It names a completion, not a rule.
const curatedValidatorName = "transit-asn"

func init() {
	configyang.RegisterSuggestion(curatedValidatorName, curatedASNValues, curatedASNHelp)
}

// curatedASNValues returns the curated ASNs as the decimal text an operator
// types. The completer sorts and filters by prefix, so the order here is the
// table's own.
func curatedASNValues() []string {
	values := make([]string, 0, len(curatedTransitFree))
	for _, network := range curatedTransitFree {
		values = append(values, textbuf.StringUint32(network.asn))
	}
	return values
}

// curatedASNHelp returns the network behind one offered ASN, and the empty
// string for a value the table does not hold. The completer then prints its own
// generic label, so a value that arrived from another validator is never
// described as a transit-free network.
//
// The annotation itself is curatedAnnotation (curated.go), shared with the
// `show bgp reject-asn` column so the dropdown and the listing name a network
// the same way. This function is the boundary that turns the completer's typed
// text into the ASN that lookup takes.
func curatedASNHelp(value string) string {
	asn, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return ""
	}
	return curatedAnnotation(uint32(asn))
}
