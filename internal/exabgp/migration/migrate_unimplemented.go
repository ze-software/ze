// Design: docs/architecture/core-design.md -- ExaBGP migration orchestration
// Overview: migrate.go -- migrateCapability, the caller that refuses the config
// Related: docs/architecture/config/deprecated-options.md -- the operator-facing page
//
// Upstream: ExaBGP src/exabgp/bgp/message/open/capability/ms.py and
// .../capability/operational.py -- the two capabilities ze declines.
//
// ExaBGP offers two BGP extensions ze does not implement, and each one comes
// from an IETF draft that expired without becoming an RFC.
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
// The migration REFUSES a config that asks for either one. Accepting it and
// dropping the capability would hand the operator a session that negotiates
// less than the ExaBGP config asked for, with nothing said: the failure
// `ai/rules/principles.md` names first.

package migration

import (
	"errors"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// unimplementedCapabilities are the ExaBGP capability keywords ze declines, in
// the order the refusal names them. The file header says what each one is and
// why ze does not offer it.
//
// A keyword leaves this list when ze implements the extension it names, never
// to make a config load.
var unimplementedCapabilities = [...]string{"multi-session", "operational"}

// refuseUnimplementedCapabilities answers an error naming EVERY capability in
// the block that ze does not implement, or nil when the block holds none.
//
// Every one, rather than the first: api-open.conf in the ported ExaBGP suite
// asks for both, and a refusal that named only `multi-session` sent the
// operator back for a second run to learn about `operational`.
func refuseUnimplementedCapabilities(srcCap *config.Tree) error {
	// Two passes over a compile-time array of two, because the singular or
	// plural noun has to be written before the first keyword is.
	count := 0
	for _, keyword := range unimplementedCapabilities {
		if _, ok := srcCap.GetFlex(keyword); ok {
			count++
		}
	}

	if count == 0 {
		return nil
	}

	var message textbuf.Buffer
	if count == 1 {
		message.Str("unsupported capability ")
	} else {
		message.Str("unsupported capabilities ")
	}

	written := 0
	for _, keyword := range unimplementedCapabilities {
		if _, ok := srcCap.GetFlex(keyword); !ok {
			continue
		}
		if written > 0 {
			message.Str(", ")
		}
		message.Byte('"')
		message.Str(keyword)
		message.Byte('"')
		written++
	}

	message.Str(": not implemented in ze")
	return errors.New(message.String())
}
