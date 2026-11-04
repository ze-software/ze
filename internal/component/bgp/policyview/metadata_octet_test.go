package policyview

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/policy"
)

var fieldTestMetadataOctet = policy.MustRegisterField("test-metadata-octet", policy.RouteView{})

func init() {
	MustRegisterMetadata(fieldTestMetadataOctet, MetadataOctet)
}

// TestMetadataOctetNumber proves a registered octet answers a typed number,
// including a present zero, through the same cached View.Read as consumers.
func TestMetadataOctetNumber(t *testing.T) {
	reader, err := ReaderFor(fieldTestMetadataOctet)
	if err != nil {
		t.Fatal(err)
	}
	var view View
	for _, octet := range []uint8{2, 0} {
		view.Reset(wireu.NewWireUpdate(fullUpdate(), fourOctet(t)),
			Session{}.WithImportMetadata(map[string]any{"test-metadata-octet": octet}))
		for range 2 {
			value, err := view.Read(reader)
			if err != nil {
				t.Fatal(err)
			}
			number, ok := value.Number()
			if !value.Present() || !ok || number != uint64(octet) {
				t.Errorf("octet %d: Present=%v Number=(%d, %v); want present typed number", octet, value.Present(), number, ok)
			}
		}
		view.Clear()
	}
}
