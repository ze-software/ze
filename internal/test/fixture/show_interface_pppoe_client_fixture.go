// Design: docs/guide/pppoe.md -- `show interface pppoe-client` and its dns-servers key
// Related: register_show_interface_pppoe_client.go -- the registration of the driver below

package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// showPPPoEClientDevice is the pppoe-client the .ci configures on kernelLink,
// where no access concentrator answers.
const showPPPoEClientDevice = "pppoe0"

// showInterfacePPPoEClient proves spec-home-cpe-13 AC-11 for a client with no
// session: `show interface pppoe-client` answers one object for pppoe0, in
// state discovery, on its source interface, whose dns-servers key is present
// and an empty list, never absent and never null. It reads the raw JSON, so a
// missing key cannot decode as an empty slice.
func showInterfacePPPoEClient(ctx context.Context, plugin *sdk.Plugin) error {
	var last string
	var clients []map[string]json.RawMessage
	ready := Poll(ctx, 40, 250*time.Millisecond, func() bool {
		clients = nil
		_, err := Dispatch(ctx, plugin, "show interface pppoe-client", &clients)
		if err != nil {
			last = err.Error()
			return false
		}
		return len(clients) == 1 && bytes.Equal(clients[0]["state"], []byte(`"discovery"`))
	})
	if !ready {
		return fmt.Errorf("show interface pppoe-client never answered one client in discovery: last error %q, answer %v", last, clients)
	}
	client := clients[0]
	want := [...]struct{ key, value string }{
		{"name", `"` + showPPPoEClientDevice + `"`},
		{"state", `"discovery"`},
		{"source-interface", `"` + kernelLink + `"`},
		{"dns-servers", `[]`},
	}
	for _, field := range want {
		got, present := client[field.key]
		if !present {
			return fmt.Errorf("show interface pppoe-client: key %q is absent, want %s", field.key, field.value)
		}
		if !bytes.Equal(got, []byte(field.value)) {
			return fmt.Errorf("show interface pppoe-client: %q = %s, want %s", field.key, got, field.value)
		}
	}
	var tb textbuf.Buffer
	tb.Str("OK: show interface pppoe-client: ").Str(showPPPoEClientDevice).
		Str(" in discovery on ").Str(kernelLink).Str(", dns-servers []\n").StdErr()
	return nil
}
