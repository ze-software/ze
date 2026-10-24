// Design: docs/architecture/wire/nlri.md -- ADD-PATH framing of withdrawn routes

package rs

import (
	"strings"
	"testing"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// TestExtractWireNLRIRecordsRefusesAnUnknownContext proves that an UPDATE whose
// encoding context the registry does not hold is refused with an error, not
// answered with the same nil a message with no wire UPDATE gets. Method: a wire
// UPDATE under the largest context id, which no registration reaches; the
// extraction must return an error naming the context and no records.
func TestExtractWireNLRIRecordsRefusesAnUnknownContext(t *testing.T) {
	t.Parallel()

	body := []byte{0, 0, 0, 0} // No withdrawn routes, no attributes, no NLRI.
	unknown := ^bgpctx.ContextID(0)
	if bgpctx.Registry.Get(unknown) != nil {
		t.Fatalf("context %d is registered; the test needs an unregistered id", unknown)
	}
	records, err := extractWireNLRIRecords(&bgptypes.RawMessage{RawBytes: body, WireUpdate: wireu.NewWireUpdate(body, unknown)})
	if err == nil {
		returnNLRIRecords(records)
		t.Fatal("an UPDATE under an unregistered encoding context was not refused")
	}
	if records != nil {
		t.Errorf("records returned beside the refusal: %+v", records)
	}
	if !strings.Contains(err.Error(), "encoding context") {
		t.Errorf("refusal %q does not name the encoding context", err)
	}
}

// wireNLRIRecords extracts msg's records and fails the test on a refusal.
func wireNLRIRecords(t *testing.T, msg *bgptypes.RawMessage) *nlriRecords {
	t.Helper()
	records, err := extractWireNLRIRecords(msg)
	if err != nil {
		t.Fatalf("extract NLRI records: %v", err)
	}
	return records
}
