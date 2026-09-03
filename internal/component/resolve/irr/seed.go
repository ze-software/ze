// Design: docs/architecture/resolve.md -- the shipped RIR delegation seed
//
// The seed is data rather than Go source. rir-delegation.txt ships inside the
// binary through go:embed, and the one parser in rir.go reads it. A stored
// copy a refresh writes carries the same format, so both sources are read the
// same way.
//
// Related: rir.go -- the table, the parser and the lookup
package irr

import (
	_ "embed"
	"fmt"
	"strings"
	"sync"
)

// seedDelegation is the shipped RIR delegation table, written by
// `./le iana-asn write` from the five registry delegation files.
//
//go:embed rir-delegation.txt
var seedDelegation string

// seedTable parses the embedded seed on the first call and hands that one
// result to every later caller.
//
// It answers an error rather than an empty table when the seed cannot be
// read, so no lookup can report an unreadable table as an AS number nobody
// holds (ai/rules/principles.md).
var seedTable = sync.OnceValues(func() (*rirTable, error) {
	table, err := parseRIRTable(strings.NewReader(seedDelegation))
	if err != nil {
		return nil, fmt.Errorf("embedded delegation seed: %w", err)
	}
	return table, nil
})
