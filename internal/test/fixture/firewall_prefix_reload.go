// Design: docs/functional-tests.md -- live firewall prefix-refusal proof
// Related: reload_fixture_live_stages.go -- processed-reload generation fence
// Related: daemon_store.go -- read-only access to the daemon's persistent store

package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/zefs"
)

// firewallPrefixTermKey names the configured firewall term list.
const firewallPrefixTermKey = "term"

// firewallPrefixInputChain names the chain configured by this fixture.
const firewallPrefixInputChain = "input"

func init() {
	Register("firewall/prefix-host-bits-keeps-active", firewallPrefixReload, StopsDaemon)
}

// firewallPrefixSnapshot is one observation, not a mutable daemon model.
// All fields hold caller-owned values read after the reload completion fence.
type firewallPrefixSnapshot struct {
	kernel    string
	running   map[string]any
	pointer   []byte
	persisted []byte
}

// firewallPrefixReload proves the sequence from the operator's invalid edit to
// a refused transaction, preserved kernel and config state, and a corrected
// edit that does commit. The same observer remains alive through both reloads.
func firewallPrefixReload(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("firewall/prefix-host-bits-keeps-active takes invalid and corrected config paths")
	}
	return Observe(ctx, "prefix-proof", &sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		before, err := firewallPrefixAwaitState(ctx, plugin, "original", "192.0.2.0/24")
		if err != nil {
			return fmt.Errorf("initial firewall: %w", err)
		}
		if err := liveStagesReload(ctx, plugin, args[0]); err != nil {
			return fmt.Errorf("invalid prefix reload: %w", err)
		}
		after, err := firewallPrefixReadState(ctx, plugin)
		if err != nil {
			return fmt.Errorf("after invalid prefix reload: %w", err)
		}
		if !bytes.Equal(after.pointer, before.pointer) {
			return fmt.Errorf("refused reload changed active pointer from %q to %q", before.pointer, after.pointer)
		}
		if !bytes.Equal(after.persisted, before.persisted) {
			return errors.New("refused reload changed the persisted active configuration")
		}
		if !reflect.DeepEqual(after.running, before.running) {
			return errors.New("refused reload changed the running firewall configuration")
		}
		// The test's rule matches a documentation-only source prefix, with no
		// interface in that subnet. Its packet counters therefore stay zero;
		// exact equality checks the installed rule, not just its continued presence.
		if after.kernel != before.kernel {
			return fmt.Errorf("refused reload changed the installed kernel table:\nbefore:\n%s\nafter:\n%s", before.kernel, after.kernel)
		}
		fmt.Fprintln(os.Stderr, "PREFIX_REFUSAL_KEPT_ACTIVE_AND_KERNEL") //nolint:errcheck // the .ci requires this marker

		if err := liveStagesReload(ctx, plugin, args[1]); err != nil {
			return fmt.Errorf("corrected prefix reload: %w", err)
		}
		corrected, err := firewallPrefixAwaitState(ctx, plugin, "corrected", "198.51.100.0/24")
		if err != nil {
			return fmt.Errorf("after corrected prefix reload: %w", err)
		}
		if bytes.Equal(corrected.pointer, before.pointer) {
			return errors.New("corrected reload did not advance the persisted active version")
		}
		if strings.Contains(corrected.kernel, "192.0.2.0/24") {
			return errors.New("corrected reload left the original source-prefix rule installed")
		}
		fmt.Fprintln(os.Stderr, "PREFIX_CORRECTION_COMMITTED_AND_INSTALLED") //nolint:errcheck // the .ci requires this marker
		return nil
	})
}

// firewallPrefixAwaitState bounds readiness by observed kernel, running and
// persisted state, never by a delay taken as evidence that a reload completed.
func firewallPrefixAwaitState(ctx context.Context, plugin *sdk.Plugin, term, prefix string) (firewallPrefixSnapshot, error) {
	var snapshot firewallPrefixSnapshot
	lastErr := errors.New("no state read completed before the observation context ended")
	ready := Poll(ctx, 80, 50*time.Millisecond, func() bool {
		snapshot, lastErr = firewallPrefixReadState(ctx, plugin)
		if lastErr != nil {
			return false
		}
		if !strings.Contains(snapshot.kernel, "ip saddr "+prefix) {
			lastErr = fmt.Errorf("kernel table has no source rule for %s", prefix)
			return false
		}
		running, err := firewallPrefixLeaf(snapshot.running, term)
		if err != nil {
			lastErr = fmt.Errorf("running configuration: %w", err)
			return false
		}
		if running != prefix {
			lastErr = fmt.Errorf("running source prefix is %q, want %q", running, prefix)
			return false
		}
		schema, err := config.YANGSchema()
		if err != nil {
			lastErr = err
			return false
		}
		tree, err := config.NewParser(schema).Parse(string(snapshot.persisted))
		if err != nil {
			lastErr = fmt.Errorf("persisted active config does not parse: %w", err)
			return false
		}
		firewall, ok := tree.ToMap()["firewall"].(map[string]any)
		if !ok {
			lastErr = errors.New("persisted active config has no firewall tree")
			return false
		}
		persisted, err := firewallPrefixLeaf(firewall, term)
		if err != nil {
			lastErr = fmt.Errorf("persisted configuration: %w", err)
			return false
		}
		if persisted != prefix {
			lastErr = fmt.Errorf("persisted source prefix is %q, want %q", persisted, prefix)
			return false
		}
		return true
	})
	if !ready {
		return firewallPrefixSnapshot{}, fmt.Errorf("firewall state never reached %s: %w", prefix, lastErr)
	}
	return snapshot, nil
}

// firewallPrefixReadState reads the actual kernel listing and the running tree
// over RPC, then opens a fresh read-only handle to the same disk store as Ze.
func firewallPrefixReadState(ctx context.Context, plugin *sdk.Plugin) (snapshot firewallPrefixSnapshot, retErr error) {
	kernel, err := netfilterCommandOutput(ctx, "nft", "list", "table", "inet", "ze_prefix_guard")
	if err != nil {
		return snapshot, fmt.Errorf("read installed firewall table: %w", err)
	}
	snapshot.kernel = kernel
	status, running, err := dispatchMap(ctx, plugin, "show config running")
	if err != nil {
		return snapshot, fmt.Errorf("read running config: %w", err)
	}
	if status != statusDone {
		return snapshot, fmt.Errorf("show config running returned status %q", status)
	}
	firewall, ok := running["firewall"].(map[string]any)
	if !ok {
		return snapshot, errors.New("show config running has no firewall tree")
	}
	snapshot.running = firewall
	store, err := openDaemonStoreReadOnly(os.Environ(), ".")
	if err != nil {
		return snapshot, fmt.Errorf("open persisted active config: %w", err)
	}
	defer func() { retErr = errors.Join(retErr, store.Close()) }()
	snapshot.pointer, err = store.ReadKey(zefs.KeyConfigActive.Key(fileBGPConf))
	if err != nil {
		return snapshot, fmt.Errorf("read persisted active pointer: %w", err)
	}
	if len(snapshot.pointer) == 0 {
		return snapshot, errors.New("persisted active pointer is empty")
	}
	snapshot.persisted, err = storage.ReadActiveConfig(store, fileBGPConf)
	if err != nil {
		return snapshot, fmt.Errorf("read persisted active config: %w", err)
	}
	return snapshot, nil
}

// firewallPrefixLeaf reads a leaf from parsed configuration structure. The
// fixed path bounds the walk; config spelling and comments cannot affect it.
func firewallPrefixLeaf(firewall map[string]any, term string) (string, error) {
	current := firewall
	for _, step := range []string{"table", "prefix_guard", "chain", firewallPrefixInputChain, firewallPrefixTermKey, term, "from"} {
		next, ok := current[step].(map[string]any)
		if !ok {
			return "", fmt.Errorf("firewall config has no %q map", step)
		}
		current = next
	}
	prefix, ok := current["source-address"].(string)
	if !ok {
		return "", errors.New("firewall term has no source-address value")
	}
	return prefix, nil
}
