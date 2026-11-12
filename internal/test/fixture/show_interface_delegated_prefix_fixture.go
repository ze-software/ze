// Design: docs/guide/pppoe.md -- `show interface delegated-prefix` and its waiting state
// Related: register_show_interface_delegated_prefix.go -- the registration of the driver below

package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// showDelegatedPrefixLAN is the dummy LAN interface the .ci numbers from
// pppoe0's delegated prefix.
const showDelegatedPrefixLAN = "zelan0"

// showInterfaceDelegatedPrefix proves the LAN half of `show interface
// delegated-prefix` through the daemon dispatcher, for a unit whose source
// holds no delegation: one row for zelan0 unit 0 naming source pppoe0 and
// subnet-id 1, in state waiting with a reason, and with no address, prefix or
// lifetime keys, because Ze has no value to report for them. It reads the raw
// JSON, so a zero cannot pass for a missing key.
func showInterfaceDelegatedPrefix(ctx context.Context, plugin *sdk.Plugin) error {
	var last string
	var rows []map[string]json.RawMessage
	ready := Poll(ctx, 40, 250*time.Millisecond, func() bool {
		rows = nil
		_, err := Dispatch(ctx, plugin, "show interface delegated-prefix", &rows)
		if err != nil {
			last = err.Error()
			return false
		}
		return len(rows) == 1
	})
	if !ready {
		return fmt.Errorf("show interface delegated-prefix never answered one row: last error %q, answer %v", last, rows)
	}
	row := rows[0]
	want := [...]struct{ key, value string }{
		{"interface", `"` + showDelegatedPrefixLAN + `"`},
		{"unit", `"0"`},
		{fieldSource, `"pppoe0"`},
		{"subnet-id", `1`},
		{fieldState, `"waiting"`},
	}
	for _, field := range want {
		got, present := row[field.key]
		if !present {
			return fmt.Errorf("show interface delegated-prefix: key %q is absent, want %s", field.key, field.value)
		}
		if !bytes.Equal(got, []byte(field.value)) {
			return fmt.Errorf("show interface delegated-prefix: %q = %s, want %s", field.key, got, field.value)
		}
	}
	reason := string(row["reason"])
	if !strings.Contains(reason, "pppoe-client pppoe0 holds no delegated prefix") {
		return fmt.Errorf("show interface delegated-prefix: reason %s does not say pppoe0 holds no delegated prefix", reason)
	}
	for _, absent := range [...]string{"address", "prefix", "delegated", "valid-lifetime", "preferred-lifetime"} {
		if got, present := row[absent]; present {
			return fmt.Errorf("show interface delegated-prefix: a waiting unit reports %q = %s, a value Ze cannot know", absent, got)
		}
	}
	var tb textbuf.Buffer
	return tb.Str("OK: show interface delegated-prefix: ").Str(showDelegatedPrefixLAN).
		Str(" unit 0 waiting on pppoe0, no address reported\n").StdErr()
}
