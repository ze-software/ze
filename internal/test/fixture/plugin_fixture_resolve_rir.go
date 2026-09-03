package fixture

// plugin_fixture_resolve_rir.go drives the ASN-to-registry lookup and the
// refresh over the paths an operator uses: the host binary with no daemon, and
// the daemon's CLI over SSH.
//
// Related: register_resolve_rir.go -- the two scenario names
// Related: test/plugin/resolve-rir-lookup.ci, test/plugin/resolve-rir-refresh.ci

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/resolve/irr"
	"github.com/ze-software/ze/pkg/zefs"
)

// resolveRIRUser and resolveRIRPassword are the credentials extra1InitCLI
// types at `ze init`, and extra1AdminHash is that password's stored hash.
const (
	resolveRIRUser     = "admin"
	resolveRIRPassword = "testpass"
)

// resolveRIRSeedASN is held by ARIN in the shipped seed, and the stored copy
// this fixture writes hands it to APNIC instead. The two answers differ on
// purpose: a lookup that reads the wrong source says so by naming the wrong
// registry, rather than by returning nothing.
const (
	resolveRIRSeedASN      = "15169"
	resolveRIRSeedRegistry = irr.RIRARIN
	resolveRIRSeedWhois    = irr.WhoisARIN
)

// resolveRIRUndelegatedASN sits above every range the registries delegate, so
// it is the AS number that separates "nobody holds this" from "I could not
// read the table".
const resolveRIRUndelegatedASN = "4294967294"

// resolveRIRConfig is a daemon that does nothing but answer commands: one
// administrator and an ephemeral SSH listener. No BGP peer is configured,
// because the delegation table is read from data and never from a session.
func resolveRIRConfig() string {
	return `system {
	authentication { user ` + resolveRIRUser + ` { password "` + extra1AdminHash + `"; } }
}
environment {
	ssh { enabled true; server main { ip 127.0.0.1; port 0; } }
}
`
}

// resolveRIRLookup proves an operator reaches the ASN-to-registry answer on
// both read paths, with no network and no stored copy: `ze resolve rir` on the
// host, and `show resolve rir <asn> | json` inside the daemon.
//
// The config directory is a fresh temporary one, so the store the lookup
// consults is known to hold no refreshed copy and the shipped seed is what
// answers.
func resolveRIRLookup(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("resolve-rir-lookup takes no arguments: %q", args)
	}

	workDir, err := os.MkdirTemp("", "resolve-rir-lookup-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir) //nolint:errcheck // fixture cleanup, and the directory is this run's own

	if err := resolveRIRHostAnswers(ctx, workDir, resolveRIRSeedRegistry, resolveRIRSeedWhois); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: the host answers from the embedded seed, with no daemon and no stored copy")

	code, _, stderr, err := runCaptured(ctx, extra1Environment(map[string]string{envConfigDir: workDir}), "",
		"ze", "resolve", "rir", resolveRIRUndelegatedASN)
	if err != nil {
		return err
	}
	if code == 0 {
		return fmt.Errorf("ze resolve rir %s exited 0: an AS number in no delegated range is not an answer", resolveRIRUndelegatedASN)
	}
	if !strings.Contains(stderr, "no delegated range") {
		return fmt.Errorf("ze resolve rir %s did not name the missing range: %s", resolveRIRUndelegatedASN, stderr)
	}
	fmt.Fprintln(os.Stderr, "OK: an AS number in no delegated range exits non-zero and says which answer it is")

	daemon, port, err := extra1RunDaemonIn(ctx, workDir, "resolve-rir-lookup.conf", extra1DaemonLog, resolveRIRConfig(), map[string]string{
		envConfigDir: workDir,
	})
	if err != nil {
		return err
	}
	defer daemon.stop()

	configDir, err := extra1InitCLI(ctx, port)
	if err != nil {
		return err
	}
	defer os.RemoveAll(configDir) //nolint:errcheck // fixture cleanup, and the directory is this run's own

	if err := resolveRIRDaemonAnswers(ctx, configDir, port, resolveRIRSeedRegistry, resolveRIRSeedWhois); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: the daemon answers the same registry as structured data through | json")

	output, err := extra1CLI(ctx, configDir, port, resolveRIRUser, resolveRIRPassword,
		"show resolve rir "+resolveRIRUndelegatedASN)
	if err == nil {
		return fmt.Errorf("show resolve rir %s succeeded: %s", resolveRIRUndelegatedASN, output)
	}
	if !strings.Contains(output, "no delegated range") {
		return fmt.Errorf("show resolve rir %s did not name the missing range: %s", resolveRIRUndelegatedASN, output)
	}
	fmt.Fprintln(os.Stderr, "OK: the daemon keeps the two answers apart as well")

	return nil
}

// resolveRIRRefresh proves what a refresh does when the registries are out of
// reach: it stores nothing, it names the file it could not read, and the copy
// stored before it keeps answering, on the daemon and on the host.
//
// The registries are put out of reach by pointing the daemon's proxy
// configuration at a closed port. That is the operating system's own HTTP
// proxy setting, honored by net/http, and NOT a Ze surface: a functional test
// MUST NOT reach the five public delegation files, and Ze offers no way to
// redirect them. What a refresh does when all five DO answer is proven by unit
// tests over irr.FetchDelegationTable and handleRIRRefresh, which serve their
// own registry bodies in process.
func resolveRIRRefresh(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("resolve-rir-refresh takes no arguments: %q", args)
	}

	workDir, err := os.MkdirTemp("", "resolve-rir-refresh-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir) //nolint:errcheck // fixture cleanup, and the directory is this run's own

	stored, err := resolveRIRStoreCopy(filepath.Join(workDir, "database.zefs"))
	if err != nil {
		return err
	}

	daemon, port, err := extra1RunDaemonIn(ctx, workDir, "resolve-rir-refresh.conf", extra1DaemonLog, resolveRIRConfig(), map[string]string{
		envConfigDir:  workDir,
		"HTTPS_PROXY": "http://127.0.0.1:1",
		"HTTP_PROXY":  "http://127.0.0.1:1",
		"NO_PROXY":    "",
	})
	if err != nil {
		return err
	}
	defer daemon.stop()

	configDir, err := extra1InitCLI(ctx, port)
	if err != nil {
		return err
	}
	defer os.RemoveAll(configDir) //nolint:errcheck // fixture cleanup, and the directory is this run's own

	if err := resolveRIRDaemonAnswers(ctx, configDir, port, irr.RIRAPNIC, irr.WhoisAPNIC); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: the daemon answers from the stored copy, which is newer than the shipped seed")

	if err := resolveRIRHostAnswers(ctx, workDir, irr.RIRAPNIC, irr.WhoisAPNIC); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: the host reads the same stored copy while the daemon holds the file open")

	output, err := extra1CLI(ctx, configDir, port, resolveRIRUser, resolveRIRPassword, "update resolve rir")
	if err == nil {
		return fmt.Errorf("update resolve rir reported success with the registries out of reach: %s", output)
	}
	if !strings.Contains(output, "delegated-") {
		return fmt.Errorf("the failed refresh did not name the registry file it could not read: %s", output)
	}
	fmt.Fprintln(os.Stderr, "OK: a refresh that cannot reach a registry fails and names the file")

	if err := resolveRIRDaemonAnswers(ctx, configDir, port, irr.RIRAPNIC, irr.WhoisAPNIC); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: the copy stored before the failed refresh still answers")

	daemon.stop()

	if err := resolveRIRStoredCopyUnchanged(filepath.Join(workDir, "database.zefs"), stored); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: the failed refresh left the stored table byte for byte as it was")

	return nil
}

// resolveRIRStoreCopy writes a delegation table into a fresh managed store and
// answers the bytes it wrote.
//
// The table is dated TOMORROW so it beats the shipped seed whenever that seed
// was last refreshed: the lookup prefers the stored copy only when its date is
// strictly later, and a fixed date would start losing the day the seed passed
// it. It hands the ASN the seed gives ARIN to APNIC instead, so which source
// answered is visible in the answer itself.
func resolveRIRStoreCopy(path string) ([]byte, error) {
	table, err := irr.RenderDelegationTable(irr.DelegationTable{
		Generated: time.Now().UTC().AddDate(0, 0, 1),
		Ranges: []irr.RIREntry{{
			Start: 15169,
			End:   15169,
			RIR:   irr.RIRAPNIC,
			Whois: irr.WhoisAPNIC,
		}},
	})
	if err != nil {
		return nil, err
	}

	store, err := zefs.Create(path)
	if err != nil {
		return nil, err
	}
	if err := store.WriteFile(zefs.KeyRIRDelegation.Pattern, table, 0); err != nil {
		_ = store.Close()
		return nil, err
	}
	if err := store.Close(); err != nil {
		return nil, err
	}
	return table, nil
}

// resolveRIRStoredCopyUnchanged reads the delegation key back and compares it
// with what was stored before the refresh ran.
func resolveRIRStoredCopyUnchanged(path string, want []byte) error {
	store, err := zefs.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	got, err := store.ReadFile(zefs.KeyRIRDelegation.Pattern)
	if err != nil {
		return err
	}
	if !bytes.Equal(got, want) {
		return fmt.Errorf("the failed refresh rewrote the stored table:\n%s", got)
	}
	return nil
}

// resolveRIRHostAnswers runs `ze resolve rir` on the host, with no daemon and
// no network, and requires the registry and whois host it prints.
func resolveRIRHostAnswers(ctx context.Context, configDir, registry, whois string) error {
	code, stdout, stderr, err := runCaptured(ctx, extra1Environment(map[string]string{envConfigDir: configDir}), "",
		"ze", "resolve", "rir", resolveRIRSeedASN)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("ze resolve rir %s exit=%d: %s%s", resolveRIRSeedASN, code, stdout, stderr)
	}
	if !strings.Contains(stdout, registry) || !strings.Contains(stdout, whois) {
		return fmt.Errorf("ze resolve rir %s answered %q, want %s and %s", resolveRIRSeedASN, stdout, registry, whois)
	}
	return nil
}

// resolveRIRDaemonAnswers runs `show resolve rir <asn> | json` over the
// daemon's CLI and requires the registry and whois host the JSON carries.
//
// The pipe is part of the assertion: an operator scripting against this
// command reads the structured answer, so a handler that returned prose would
// pass a substring check here and fail a parser.
func resolveRIRDaemonAnswers(ctx context.Context, configDir, port, registry, whois string) error {
	output, err := extra1CLI(ctx, configDir, port, resolveRIRUser, resolveRIRPassword,
		"show resolve rir "+resolveRIRSeedASN+" | json")
	if err != nil {
		return fmt.Errorf("show resolve rir %s | json: %w\n%s", resolveRIRSeedASN, err, output)
	}

	var answer struct {
		ASN      json.Number `json:"asn"`
		Registry string      `json:"registry"`
		Whois    string      `json:"whois"`
	}
	if err := json.Unmarshal([]byte(output), &answer); err != nil {
		return fmt.Errorf("show resolve rir %s | json did not answer JSON (%w): %s", resolveRIRSeedASN, err, output)
	}
	if answer.ASN.String() != resolveRIRSeedASN {
		return fmt.Errorf("the answer is about AS%s, want AS%s", answer.ASN.String(), resolveRIRSeedASN)
	}
	if answer.Registry != registry || answer.Whois != whois {
		return fmt.Errorf("the answer names %s at %s, want %s at %s", answer.Registry, answer.Whois, registry, whois)
	}
	return nil
}
