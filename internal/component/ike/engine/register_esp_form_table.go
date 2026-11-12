// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- one Child SA receives both ESP forms
// Related: doctor_esp_form_fast_path.go -- espFormOwnedTableAccept, the answer registered here

package engine

import "github.com/ze-software/ze/internal/component/ike/dataplane"

// The dataplane names the fix for a rule in a ze_ table that keeps the ESP form fast
// path off, and who writes such a table is the firewall registry's to say, which the
// dataplane does not import: the engine, which may import the firewall component,
// registers the answer.
func init() { //nolint:gochecknoinits // registers the ze_ table answer with the dataplane
	dataplane.RegisterESPFormOwnedTableAccept(espFormOwnedTableAccept)
}
