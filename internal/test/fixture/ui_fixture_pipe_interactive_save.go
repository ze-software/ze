package fixture

import (
	"bytes"
	"context"
	"fmt"
	"os"
)

var pipeInteractiveSaveBytes = []byte(
	"{\"rtt-ms\":1.234,\"seq\":0,\"status\":\"ok\",\"target\":\"192.0.2.1\"}\n" +
		"{\"rtt-ms\":2.345,\"seq\":1,\"status\":\"ok\",\"target\":\"192.0.2.1\"}\n" +
		"{\"seq\":2,\"status\":\"timeout\",\"target\":\"192.0.2.1\"}\n",
)

func init() {
	Register("ui/pipe-interactive-save", uiDriver(pipeInteractiveSave))
}

func pipeInteractiveSave(context.Context) error {
	saved, err := os.ReadFile("interactive-save.ndjson")
	if err != nil {
		return fmt.Errorf("read interactive save: %w", err)
	}
	if !bytes.Equal(saved, pipeInteractiveSaveBytes) {
		return fmt.Errorf("saved bytes differ from the three displayed lines: %q", saved)
	}
	info, err := os.Stat("interactive-save.ndjson")
	if err != nil {
		return fmt.Errorf("stat interactive save: %w", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		return fmt.Errorf("interactive save mode=%04o, want 0600", mode)
	}
	fmt.Println("OK: interactive save bytes and mode")
	return nil
}
