// Design: docs/architecture/testing/interop.md -- one atomic peer-state encapsulation change.
// Related: migration_test.go -- fixed-header and attribute boundaries.
// UAPI: Linux 7.3 include/uapi/linux/xfrm.h, struct xfrm_user_migrate_state.
// Existing owner: internal/component/ike/dataplane/xfrm_migrate_linux.go -- xfrmStateMigrationRequest.
package main

import (
	"encoding/binary"
	"errors"
)

// migrationPayload holds only the fixed migration header and explicit ENCAP attribute.
// The Linux adapter MUST wrap it with vendor nl.NewNetlinkRequest; it MUST NOT reconstruct
// state algorithms, keys, lifetime or replay attributes, or replace an SA through delete/add.
func migrationPayload(selected state, encap encapsulation) ([]byte, error) {
	if !selected.key.source.Is4() {
		return nil, errors.New("migration source is not IPv4")
	}
	if !selected.key.target.Is4() {
		return nil, errors.New("migration target is not IPv4")
	}
	if selected.key.spi == 0 {
		return nil, errors.New("migration SPI is zero")
	}
	switch encap.form {
	case formUDP:
		if encap.sourcePort == 0 {
			return nil, errors.New("migration source UDP port is zero")
		}
		if encap.targetPort == 0 {
			return nil, errors.New("migration target UDP port is zero")
		}
	case formBare:
	case formUnspecified:
		return nil, errors.New("migration form is unspecified")
	default:
		return nil, errors.New("migration form is unknown")
	}
	// Linux struct xfrm_user_migrate_state is 132 bytes. The only attribute is
	// xfrm_encap_tmpl (24 bytes plus its four-byte nlattr header). No SA or replay
	// state is encoded: xfrm_do_migrate_state copies it while holding the old lock.
	body := make([]byte, 160)
	source, target := selected.key.source.As4(), selected.key.target.As4()
	copy(body[:4], target[:])
	binary.BigEndian.PutUint32(body[16:20], selected.key.spi)
	binary.NativeEndian.PutUint16(body[20:22], 2) // AF_INET, UAPI identity family.
	body[22] = 50                                 // IPPROTO_ESP.
	copy(body[24:28], target[:])
	copy(body[40:44], source[:])
	copy(body[64:120], selected.selector[:])
	binary.NativeEndian.PutUint32(body[120:124], selected.reqid)
	binary.NativeEndian.PutUint16(body[128:130], 2)
	binary.NativeEndian.PutUint16(body[132:134], 28)
	binary.NativeEndian.PutUint16(body[134:136], 4) // XFRMA_ENCAP, not an SA replacement.
	if encap.form == formUDP {
		binary.NativeEndian.PutUint16(body[136:138], 2) // UDP_ENCAP_ESPINUDP.
		binary.BigEndian.PutUint16(body[138:140], encap.sourcePort)
		binary.BigEndian.PutUint16(body[140:142], encap.targetPort)
	}
	// Bare still carries the all-zero encapsulation attribute: omission inherits UDP.
	return body, nil
}
