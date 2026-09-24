// Design: docs/architecture/wire/nlri-bgpls.md -- native routing-database export.
// Related: test/plugin/bgpls-export-lifecycle.ci -- peer-side wire assertions.
package fixture

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const bgplsNativeSource = `isis {
    net 49.0001.0000.0000.0001.00
    level l1
    hostname native-isis
}`

func init() {
	Register("plugin/bgpls-export-lifecycle", func(ctx context.Context, _ []string) error {
		// Prepare the three reload inputs before startup releases the peer.
		// The observer waits for each wire transition and completed reload.
		original, err := os.ReadFile(fileBGPConf)
		if err != nil {
			return err
		}
		text := string(original)
		if strings.Count(text, bgplsNativeSource) != 1 || strings.Count(text, "bgp-ls-export { }") != 1 {
			return fmt.Errorf("native exporter fixture lacks its unique source/export roots")
		}
		for _, input := range []struct{ path, config string }{
			{"bgpls-restore.conf", text},
			{"bgpls-without-source.conf", strings.Replace(text, bgplsNativeSource, "", 1)},
			{"bgpls-disabled.conf", strings.Replace(text, "bgp-ls-export { }", "", 1)},
		} {
			if err := os.WriteFile(input.path, []byte(input.config), 0o600); err != nil {
				return err
			}
		}
		return Observe(ctx, "plugin/bgpls-export-lifecycle", sdk.Registration{}, bgplsExportLifecycle)
	})
}

func bgplsExportLifecycle(ctx context.Context, p *sdk.Plugin) error {
	for _, step := range []struct {
		updates int
		config  string
	}{
		{3, "bgpls-without-source.conf"},
		{4, "bgpls-restore.conf"},
		{5, "bgpls-disabled.conf"},
		{6, ""},
	} {
		var row map[string]any
		var lastErr error
		if !Poll(ctx, 100, 250*time.Millisecond, func() bool {
			row, lastErr = peerRow07(ctx, p, "127.0.0.1")
			return lastErr == nil && number07(row["eor-sent"]) == 1 && number07(row["updates-sent"]) >= step.updates
		}) {
			database := command07(ctx, p, "show isis database detail")
			return fmt.Errorf("native topology lifecycle did not reach UPDATE %d: peer=%v error=%v; IS-IS database status=%s data=%v error=%v", step.updates, row, lastErr, database.status, database.data, database.err) //nolint:errorlint // Either poll error may be nil here; %w would print %!w(<nil>), and it is diagnostic context, not a cause a caller unwraps.
		}
		if got := number07(row["updates-sent"]); got != step.updates {
			return fmt.Errorf("native topology lifecycle sent %d UPDATEs, want %d", got, step.updates)
		}
		if step.config != "" {
			if err := bgplsReload(ctx, p, step.config); err != nil {
				return err
			}
		}
	}
	fmt.Fprintln(os.Stderr, "OK native BGP-LS source replay and withdrawals reached collector")
	return nil
}

func bgplsReload(ctx context.Context, p *sdk.Plugin, source string) error {
	baseline := reloadGeneration(ctx, p)
	if baseline < 0 {
		return fmt.Errorf("before %s: reload generation unavailable", source)
	}
	pid, err := readPID(fileDaemonPID)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if err := os.WriteFile(fileBGPConf, data, 0o600); err != nil {
		return err
	}
	if err := syscall.Kill(pid, syscall.SIGHUP); err != nil {
		return err
	}
	// A withdrawal can precede candidate promotion. Only the whole-reload
	// generation lets the next rewrite avoid racing that file publication.
	var status string
	var result map[string]any
	if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
		status, result, err = dispatchMap(ctx, p, "show reload-status")
		return err == nil && status == statusDone && int64(number07(result["generation"])) > baseline
	}) {
		return fmt.Errorf("%s: reload did not complete after generation %d: status=%s data=%v error=%v", source, baseline, status, result, err) //nolint:errorlint // The last poll error may be nil here; %w would print %!w(<nil>), and it is diagnostic context, not a cause a caller unwraps.
	}
	if result["last-outcome"] != "applied" {
		return fmt.Errorf("%s: reload was not applied: %v", source, result)
	}
	return nil
}
