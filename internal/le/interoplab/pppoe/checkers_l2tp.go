// Design: docs/architecture/plugin/feature-gates.md -- the ze_l2tp half of the
// PPPoE interop scenario table.
// Overview: pppoe.go -- checkers(), which merges wireCheckers into the two
// scenarios that need no PPPoE codec.
//
// The four scenarios below build or decode PPPoE discovery frames with
// internal/component/l2tp/pppoe, the access concentrator's own codec. That
// package is compile-out-able, so the files holding those checkers carry
// //go:build ze_l2tp and so does this table: an always-on file naming them
// would pin the codec into every binary and defeat the compile-out
// (ai/rules/architecture.md, `./le tier check`).
//
// A build without ze_l2tp therefore drives a product with no PPPoE access
// concentrator, and offers only the two scenarios such a product can answer.
// That is the same answer internal/le/docvalid/contract_bgp.go gives for BGP,
// and it costs no scenario: every build that runs this lab carries every
// feature tag.

//go:build ze_l2tp

package pppoe

func init() {
	wireCheckers["pppoe-empty-service-name"] = checkZeAccessConcentratorEmptyServiceName
	wireCheckers["pppoe-padr-replay"] = checkZeAccessConcentratorPADRReplay
	wireCheckers["ipv6cp-zero-identifier"] = checkZeAccessConcentratorIPv6CPZeroIdentifier
	wireCheckers["ipv6cp-missing-option"] = checkZeAccessConcentratorIPv6CPMissingOption
}
