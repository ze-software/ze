// Design: docs/architecture/storage-backends.md -- AC18 consumer restart evidence.
package fixture

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"maps"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/zefs"
)

func init() {
	Register("storage/consumer-restart", storageConsumerRestart)
	// The tc consumer creates a kernel link, so it is its own fixture name:
	// the net-admin gate its callers declare (TestNativeIPRouteFixturesDeclareNetAdmin)
	// then stays off the four consumers that touch no kernel networking.
	Register("storage/consumer-restart-tc", storageConsumerRestartTC)
}

// Each case drives a producer, terminates its daemon, and observes the actual
// consumer in a fresh process. No case seeds its own runtime-state key.
func storageConsumerRestart(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("consumer-restart requires rir, history, ddos, or ntp")
	}
	consumers := map[string]func(context.Context, string) error{
		"rir":     storageRIRRestart,
		"history": storageHistoryRestart,
		"ddos":    storageDDoSRestart,
		"ntp":     storageNTPRestart,
	}
	run, ok := consumers[args[0]]
	if !ok {
		return fmt.Errorf("unknown consumer %q (the tc consumer is the storage/consumer-restart-tc fixture)", args[0])
	}
	return storageConsumerRestartArm(ctx, args[0], run)
}

// storageConsumerRestartTC is the tc consumer: it creates a kernel link, so its
// .ci callers declare option=needs-linux:caps=net-admin.
func storageConsumerRestartTC(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("consumer-restart-tc takes no argument")
	}
	return storageConsumerRestartArm(ctx, "tc", storageTCRestart)
}

// storageConsumerRestartArm runs one named consumer in a fixture-owned scratch
// tree and proves it used no legacy blob.
func storageConsumerRestartArm(ctx context.Context, consumer string, run func(context.Context, string) error) error {
	dir, err := os.MkdirTemp(".", "consumer-restart-")
	if err != nil {
		return err
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir) //nolint:errcheck // fixture-owned scratch tree
	err = run(ctx, dir)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(dir, "database.zefs")); !os.IsNotExist(err) {
		return fmt.Errorf("consumer used legacy database.zefs: %w", err)
	}
	fmt.Fprintf(os.Stderr, "OK: %s consumer restored operational state from selected tree without config-dir pin\n", consumer)
	return nil
}

func storageConsumerDaemon(ctx context.Context, dir, config, logName string) (*extra1Daemon, string, error) {
	return storageConsumerDaemonEnv(ctx, dir, config, logName, nil)
}

// storageConsumerDaemonEnv starts the consumer daemon with extra environment
// entries a single consumer needs on top of the shared set.
func storageConsumerDaemonEnv(ctx context.Context, dir, config, logName string, extra map[string]string) (*extra1Daemon, string, error) {
	// Empty overrides remove inherited selection. The explicit file alone chooses
	// the owner; state consumers must not depend on either environment spelling.
	ready := filepath.Join(dir, logName+".ready")
	overrides := map[string]string{
		envConfigDirDotted: "", envConfigDir: "", "ze.log.ddos.detect": "info", "ze.log.ntp": "debug",
		envReadyFile: ready,
	}
	maps.Copy(overrides, extra)
	daemon, port, err := extra1RunDaemonIn(ctx, dir, "router.conf", logName, config, overrides)
	if err != nil {
		return nil, "", err
	}
	if !Poll(ctx, 160, 100*time.Millisecond, func() bool { _, err := os.Stat(ready); return err == nil }) {
		daemon.stop()
		return nil, "", fmt.Errorf("consumer daemon did not finish startup\n%s", daemon.contents())
	}
	return daemon, port, nil
}

func storageConsumerKey(dir, key string) ([]byte, error) {
	store, err := storage.OpenReadOnly(dir)
	if err != nil {
		return nil, err
	}
	defer store.Close() //nolint:errcheck // read-only fixture handle
	return store.ReadKey(key)
}

func storageConsumerCLI(port, command string, result any) error {
	output, err := extra1Command(port, resolveRIRUser, resolveRIRPassword, command+" | json")
	if err != nil {
		return fmt.Errorf("%s: %w: %s", command, err, output)
	}
	if result == nil {
		return nil
	}
	if err := json.Unmarshal([]byte(output), result); err != nil {
		return fmt.Errorf("%s: %w: %s", command, err, output)
	}
	return nil
}

func storageRIRRestart(ctx context.Context, dir string) error {
	base, stopServer, err := resolveRIRRegistryServer(ctx)
	if err != nil {
		return err
	}
	defer stopServer()
	config := resolveRIRConfig(base)
	first, port, err := storageConsumerDaemon(ctx, dir, config, "first.log")
	if err != nil {
		return err
	}
	defer first.stop()
	if err := storageConsumerCLI(port, "update resolve rir", nil); err != nil {
		return err
	}
	check := func(port string) error {
		var answer struct {
			Registry string `json:"registry"`
			Whois    string `json:"whois"`
		}
		if err := storageConsumerCLI(port, "show resolve rir "+resolveRIRSeedASN, &answer); err != nil {
			return err
		}
		if answer.Registry != resolveRIRRefreshedRegistry || answer.Whois != resolveRIRRefreshedWhois {
			return fmt.Errorf("RIR lookup did not use refreshed delegation: %+v", answer)
		}
		return nil
	}
	if err := check(port); err != nil {
		return err
	}
	first.stop()
	before, err := storageConsumerKey(dir, zefs.KeyRIRDelegation.Pattern)
	if err != nil {
		return err
	}
	stopServer() // A fresh process cannot manufacture the answer by fetching again.
	second, port, err := storageConsumerDaemon(ctx, dir, config, "second.log")
	if err != nil {
		return err
	}
	defer second.stop()
	if err := check(port); err != nil {
		return err
	}
	after, err := storageConsumerKey(dir, zefs.KeyRIRDelegation.Pattern)
	if err != nil {
		return err
	}
	if !bytes.Equal(before, after) {
		return fmt.Errorf("RIR restart changed the delegation table")
	}
	return nil
}

// The self-updater's test override (internal/component/config/system/selfupdate.go
// envRunningVersion) and the release the history consumer runs as.
const (
	storageHistoryRunningVersionEnv = "ZE_TEST_UPDATE_RUNNING_VERSION"
	storageHistoryRunningVersion    = "26.09.01"
)

func storageHistoryRestart(ctx context.Context, dir string) error {
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// The real auto-updater records this refusal BEFORE downloading or staging.
		_, _ = w.Write([]byte(`{"version":"9999.12.31","minimum-version":"9999.12.30","sha256":"` + strings.Repeat("0", 64) + `"}`))
	})}
	go serveDelegationFiles(server, listener)
	defer server.Close() //nolint:errcheck // fixture server
	config := strings.Replace(resolveRIRConfig(""), "system {", fmt.Sprintf("system { update-check { url %q; auto-apply true; spread 0; interval 86400; }", "http://"+listener.Addr().String()), 1)
	// The suite's daemon is an unstamped development build, which the updater
	// excludes from every upgrade comparison. The test override names a release
	// the manifest's minimum-version exceeds, so the refusal is recorded.
	updaterEnv := map[string]string{storageHistoryRunningVersionEnv: storageHistoryRunningVersion}
	first, port, err := storageConsumerDaemonEnv(ctx, dir, config, "first.log", updaterEnv)
	if err != nil {
		return err
	}
	defer first.stop()
	var status struct {
		Running string `json:"running-version"`
	}
	if !Poll(ctx, 40, 100*time.Millisecond, func() bool {
		return storageConsumerCLI(port, "show system update", &status) == nil && status.Running != ""
	}) {
		return fmt.Errorf("self-updater never reported a running version; this fixture requires a ze_distro daemon")
	}
	if status.Running != storageHistoryRunningVersion {
		return fmt.Errorf("self-updater ignored %s: running %q, want %q", storageHistoryRunningVersionEnv, status.Running, storageHistoryRunningVersion)
	}
	type event struct {
		Timestamp string `json:"timestamp"`
		From      string `json:"from"`
		To        string `json:"to"`
		Result    string `json:"result"`
	}
	var history struct {
		History []event `json:"history"`
	}
	var queryErr error
	if !Poll(ctx, 80, 100*time.Millisecond, func() bool {
		queryErr = storageConsumerCLI(port, "show system update history", &history)
		return queryErr == nil && len(history.History) == 1 && history.History[0].Result == "blocked-minimum-version"
	}) {
		return fmt.Errorf("updater did not record minimum-version refusal: %+v: %w", history, queryErr)
	}
	want := history.History[0]
	if want.To != "9999.12.31" || want.Timestamp == "" {
		return fmt.Errorf("unexpected updater event: %+v", want)
	}
	first.stop()
	if _, err := storageConsumerKey(dir, zefs.KeyConfigUpdateHistory.Pattern); err != nil {
		return err
	}
	_ = server.Close() // Restart cannot create a second event from the manifest.
	second, port, err := storageConsumerDaemonEnv(ctx, dir, config, "second.log", updaterEnv)
	if err != nil {
		return err
	}
	defer second.stop()
	history.History = nil
	if err := storageConsumerCLI(port, "show system update history", &history); err != nil {
		return err
	}
	if len(history.History) != 1 || history.History[0] != want {
		return fmt.Errorf("updater failed to restore the original event: got %+v want %+v", history.History, want)
	}
	return nil
}

func storageDDoSRestart(ctx context.Context, dir string) error {
	config := resolveRIRConfig("") + `ddos { detect { enabled true; baseline-window 10; check-interval 1; absolute-floor 4294967295; bps-floor 4294967295; characterize-enable false; } }`
	first, _, err := storageConsumerDaemon(ctx, dir, config, "first.log")
	if err != nil {
		return err
	}
	defer first.stop()
	var saved struct {
		Pps struct {
			Samples []float64 `json:"samples"`
		} `json:"pps"`
		Bps struct {
			Samples []float64 `json:"samples"`
		} `json:"bps"`
	}
	// The product's periodic save is every 300 real collector ticks. Await that
	// state, not an elapsed warm-up guess or a synthetic rate injection.
	if !Poll(ctx, 760, 500*time.Millisecond, func() bool {
		raw, err := storageConsumerKey(dir, zefs.KeyDDoSDetectBaseline.Pattern)
		return err == nil && json.Unmarshal(raw, &saved) == nil && len(saved.Pps.Samples) == 10 && len(saved.Bps.Samples) == 10
	}) {
		return fmt.Errorf("detector never persisted warmed real-traffic baselines\n%s", first.contents())
	}
	first.stop()
	// Disable the feed on restart: readiness can only come from restored samples.
	config = strings.Replace(config, "enabled true; baseline-window", "enabled false; baseline-window", 1)
	second, _, err := storageConsumerDaemon(ctx, dir, config, "second.log")
	if err != nil {
		return err
	}
	defer second.stop()
	if !Poll(ctx, 40, 100*time.Millisecond, func() bool {
		log := second.contents()
		return strings.Contains(log, "baseline restored from disk") && strings.Contains(log, "pps-ready=true") && strings.Contains(log, "bps-ready=true")
	}) {
		return fmt.Errorf("disabled detector did not restore ready PPS/BPS baselines\n%s", second.contents())
	}
	return nil
}

// NTP really sets the clock; never run this against the developer host. The
// explicit opt-in guards the bare command. The suite carrier passes it and
// gates on CAP_SYS_TIME instead, so only a process that can set the clock runs.
func storageNTPRestart(ctx context.Context, dir string) error {
	if os.Getenv("ZE_STORAGE_CLOCK_TEST") != "1" {
		return fmt.Errorf("NTP restart changes the system clock; run in disposable QEMU with ZE_STORAGE_CLOCK_TEST=1")
	}
	conn, err := (&net.ListenConfig{}).ListenPacket(ctx, "udp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck // fixture server
	go storageNTPServer(conn)
	config := strings.Replace(resolveRIRConfig(""), "environment {", fmt.Sprintf("environment { ntp { enabled true; slew-threshold 0; interval 86400; server local { address %q; } }", conn.LocalAddr().String()), 1)
	first, _, err := storageConsumerDaemon(ctx, dir, config, "first.log")
	if err != nil {
		return err
	}
	defer first.stop()
	var saved time.Time
	if !Poll(ctx, 120, 100*time.Millisecond, func() bool {
		raw, err := storageConsumerKey(dir, zefs.KeyNTPLastTime.Pattern)
		return err == nil && saved.UnmarshalText(raw) == nil && !saved.IsZero()
	}) {
		return fmt.Errorf("NTP did not sync and persist local-server time\n%s", first.contents())
	}
	first.stop()
	_ = conn.Close()
	// No server on restart: only the consumer's saved-time branch can set time.
	config = strings.Replace(resolveRIRConfig(""), "environment {", "environment { ntp { enabled true; interval 86400; }", 1)
	second, _, err := storageConsumerDaemon(ctx, dir, config, "second.log")
	if err != nil {
		return err
	}
	defer second.stop()
	if !Poll(ctx, 40, 100*time.Millisecond, func() bool { return strings.Contains(second.contents(), "clock restored from saved time") }) {
		return fmt.Errorf("NTP did not restore the clock\n%s", second.contents())
	}
	if delta := time.Since(saved); delta < 0 || delta > 15*time.Second {
		return fmt.Errorf("restored system clock differs from persisted NTP time by %s", delta)
	}
	return nil
}

func storageNTPServer(conn net.PacketConn) {
	var request [512]byte
	for {
		n, remote, err := conn.ReadFrom(request[:])
		if err != nil {
			return
		}
		if n < 48 {
			continue
		}
		var response [48]byte
		response[0], response[1], response[2], response[3] = 0x24, 1, 6, 0xec
		copy(response[12:16], "GPS\x00")
		copy(response[24:32], request[40:48])
		now := time.Now()
		stamp := func(dst []byte, t time.Time) {
			binary.BigEndian.PutUint32(dst, uint32(t.Unix()+2208988800))
			binary.BigEndian.PutUint32(dst[4:], uint32((uint64(t.Nanosecond())<<32)/1_000_000_000))
		}
		stamp(response[16:24], now.Add(-time.Second))
		stamp(response[32:40], now)
		stamp(response[40:48], time.Now())
		_, _ = conn.WriteTo(response[:], remote)
	}
}
