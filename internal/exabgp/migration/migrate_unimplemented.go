// Design: docs/architecture/core-design.md -- ExaBGP migration orchestration
// Overview: migrate.go -- migrateCapability, which does the translating
// Related: docs/architecture/config/deprecated-options.md -- the operator-facing page
//
// Upstream: ExaBGP src/exabgp/bgp/message/open/capability/ -- the keywords a
// neighbor's capability block can hold.
//
// A keyword in an ExaBGP `capability { ... }` block that migrateCapability has
// no branch for used to VANISH: the function reads the keywords it knows and
// copies nothing else, so the migrated config asked for less than the ExaBGP
// config did and the operator was told nothing. That is the failure
// `ai/rules/principles.md` names first, and it was reached three ways at once:
// `multi-session` and `operational` were refused by name, `aigp` was dropped in
// silence (`plan/journal/silent-fall-through.md`), and any keyword ExaBGP grows
// next would be dropped in silence too.
//
// So the set is derived rather than listed. migrateCapability declares the
// keywords it translates, this file reads that declaration, and every other key
// in the block reaches a warning naming the peer. Two of them are worth knowing
// about by name, and each comes from an IETF draft that expired without
// becoming an RFC:
//
//	multi-session  draft-ietf-idr-bgp-multisession-07, expired 2013-03-16.
//	               Section 4 assigns capability code 68.
//	operational    draft-ietf-idr-operational-message-00, expired 2012-09-01.
//	               Section 9 requests a capability code and a message type from
//	               IANA and neither was ever allocated, so ExaBGP fills both
//	               with private values of its own (capability 0xB9, marked
//	               "ExaBGP only" in capability.py, and message type 0x06,
//	               marked "Not IANA assigned yet" in message.py).
//
// The migration CONVERTS the rest of the config and warns. It refused the whole
// config for those two until 2026-09-06, which converted nothing at all for an
// operator whose only unconvertible line was one expired draft.

package migration

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// untranslatedCapabilities answers the keys of an ExaBGP capability block that
// migrateCapability does not translate, in the order the block declares them.
// The answer is empty when every key is translated.
func untranslatedCapabilities(srcCap *config.Tree) []string {
	if srcCap == nil {
		return nil
	}

	translated := make(map[string]bool, len(capabilityEnableFields)+len(capabilityValueFields)+2)
	for _, field := range capabilityEnableFields {
		translated[field] = true
	}
	for _, field := range capabilityValueFields {
		translated[field] = true
	}
	translated["asn4"] = true
	translated["add-path"] = true

	var untranslated []string
	// One keyword reaches three different stores. A typed leaf
	// (`asn4 enable;`) lands in Values(); a ze:syntax "flex" node written with
	// a word (`multi-session enable;`, `graceful-restart 360;`) lands in
	// MultiValueNames(); one written with a block (`add-path { send true; }`)
	// lands in ContainerNames(). All three are keys of the same block, so all
	// three are read.
	seen := make(map[string]bool)
	for _, names := range [][]string{srcCap.Values(), srcCap.MultiValueNames(), srcCap.ContainerNames()} {
		for _, name := range names {
			if translated[name] || seen[name] {
				continue
			}
			seen[name] = true
			untranslated = append(untranslated, name)
		}
	}
	return untranslated
}

// untranslatedCapabilityWarning answers the warning text for a peer whose
// capability block holds keywords the migration does not translate, or "" when
// it holds none.
//
// One warning naming every keyword, rather than one per keyword: api-open.conf
// in the ported ExaBGP suite asks for three the migration cannot carry, and an
// operator reading three lines learns the same thing as one reading one.
func untranslatedCapabilityWarning(srcCap *config.Tree, peerName string) string {
	untranslated := untranslatedCapabilities(srcCap)
	if len(untranslated) == 0 {
		return ""
	}

	var message textbuf.Buffer
	message.Str("peer ").Str(peerName).Str(": the capability block asks for ")
	for index, name := range untranslated {
		if index > 0 {
			message.Str(", ")
		}
		message.Byte('"').Str(name).Byte('"')
	}
	message.Str(", which `ze exabgp migrate` does not translate, so the migrated config does not ask for ")
	if len(untranslated) == 1 {
		message.Str("it")
	} else {
		message.Str("them")
	}
	return message.String()
}
