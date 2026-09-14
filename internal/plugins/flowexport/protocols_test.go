// Related: encoder_registry.go -- the registry this test reads
// Related: config.go -- Validate, which refuses a protocol no encoder registered for
//
// The test package is external so it can link the encoder subpackages: each
// one imports flowexport to register itself, so an internal test cannot.

package flowexport_test

import (
	"slices"
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/plugins/flowexport"

	// The blank imports register every encoder the daemon links, and the
	// module that declares the protocol leaf.
	_ "github.com/ze-software/ze/internal/plugins/flowexport/ipfix"
	_ "github.com/ze-software/ze/internal/plugins/flowexport/netflow9"
	_ "github.com/ze-software/ze/internal/plugins/flowexport/sflow"
	_ "github.com/ze-software/ze/internal/plugins/flowexport/yang"
)

// TestRegisteredProtocolsMatchTheModel ties the protocols the encoders
// register to the enumeration the collector list declares.
//
// The registry is the declaration: a protocol is what an encoder subpackage
// registered, and Validate reads that registry. The model is the copy an
// operator sees, and this test keeps it honest in both directions, through
// Validate so the guard an operator's config meets is the one under test.
//
// VALIDATES: the enumeration at flow-export/collector/protocol and the
// registered encoders are one set, and Validate accepts each declared protocol.
// PREVENTS: a protocol the schema offers that no encoder speaks, which was
// accepted at commit and warned about once at start, and an encoder no
// operator can select.
func TestRegisteredProtocolsMatchTheModel(t *testing.T) {
	const path = "flow-export/collector/protocol"

	declared, err := configyang.EnumValues(path)
	if err != nil {
		t.Fatalf("read the enumeration at %s: %v", path, err)
	}
	if len(declared) == 0 {
		t.Fatalf("the model declares no protocol at %s", path)
	}
	if registered := flowexport.RegisteredProtocols(); !slices.Equal(declared, registered) {
		t.Errorf("the model declares %v at %s, and the encoders registered %v", declared, path, registered)
	}

	for _, protocol := range declared {
		cfg := &flowexport.Config{Collectors: []flowexport.CollectorConfig{{
			Name: "c1", Address: "192.0.2.1", Port: 4739, Protocol: protocol,
			PollingInterval: 20, TemplateRefresh: 600,
		}}}
		if err := cfg.Validate(); err != nil {
			t.Errorf("the model declares protocol %q and Validate refuses it: %v", protocol, err)
		}
	}
}
