// Design: ai/digests/flow-ddos.md -- appliance conntrack init (gokrazy runs only ze)
//
//go:build linux && ze_appliance

package flowexport

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
)

// modprobeTimeout bounds each module load. On the appliance the modules are
// built into the kernel, so modprobe either is absent or returns at once. The
// bound is what stops a wedged modprobe from holding flow-export's startup for
// ever.
const modprobeTimeout = 10 * time.Second

// conntrackSetupOnce fences the one-time, process-wide conntrack registration.
// The tracking hook and accounting are netns-global, so a config reload that
// rebuilds the conntrack worker must not re-add a duplicate nftables rule.
var conntrackSetupOnce sync.Once

// ensureConntrackTracking makes the kernel actually track connections so
// flow-export's conntrack export (and therefore DDoS characterization) has data.
// It is compiled ONLY into the gokrazy appliance (ze_appliance): there the init
// runs solely ze, so nothing else loads nf_conntrack, registers a tracking hook,
// or enables accounting -- without which the ctnetlink dump reads an empty table
// and the recent-flow ring stays empty (what flowexport/doctor.go warns about).
// Best-effort throughout: a failure is logged and degrades to the pre-existing
// empty-ring behavior, never blocking flow-export startup.
func ensureConntrackTracking(log *slog.Logger) {
	conntrackSetupOnce.Do(func() { setupConntrackTracking(log) })
}

func setupConntrackTracking(log *slog.Logger) {
	loadConntrackModules(log)
	enableConntrackAccounting(log)
	if err := registerConntrackHook(); err != nil {
		log.Warn("flow-export: registering conntrack tracking hook failed; recent-flow ring will stay empty", "error", err)
		return
	}
	log.Info("flow-export: conntrack tracking enabled (appliance nf_conntrack init)")
}

// loadConntrackModules best-effort loads the base + netlink conntrack modules.
// On gokrazy these are built into the kernel and modprobe is absent, so a LookPath
// miss (or a modprobe error on a built-in module) is expected and only logged at
// debug -- the tracking hook below still registers against the built-in module.
func loadConntrackModules(log *slog.Logger) {
	modprobe, err := exec.LookPath("modprobe")
	if err != nil {
		return
	}
	for _, mod := range []string{"nf_conntrack", "nf_conntrack_netlink"} {
		// The context is created here rather than passed in. This runs from
		// conntrackWorker.Start, which carries none, and the bound belongs to
		// the subprocess rather than to the caller's lifetime.
		ctx, cancel := context.WithTimeout(context.Background(), modprobeTimeout)
		out, err := exec.CommandContext(ctx, modprobe, mod).CombinedOutput() //nolint:gosec // fixed module names, resolved modprobe path
		cancel()

		if err != nil {
			log.Debug("flow-export: modprobe conntrack module failed (built-in?)", "module", mod, "error", err, "output", string(out))
		}
	}
}

// enableConntrackAccounting turns on per-flow byte/packet counters. Without it the
// kernel reports zero counters, so the export worker drops every zero-delta flow
// before it reaches the ring (conntrack_worker.go). This is a sysctl (procfs)
// write, not state persistence -- the direct-fs-persistence guard allowlists it.
func enableConntrackAccounting(log *slog.Logger) {
	const acct = "/proc/sys/net/netfilter/nf_conntrack_acct"
	if err := os.WriteFile(acct, []byte("1"), 0o644); err != nil { //nolint:gosec // fixed procfs sysctl path
		log.Warn("flow-export: enabling nf_conntrack_acct failed", "error", err)
	}
}

// registerConntrackHook installs a minimal accept-only inet table whose single
// rule references `ct state`. Referencing ct in any base chain makes nftables call
// nf_ct_netns_get, which registers the conntrack tracking hooks for the whole
// netns -- the module alone tracks nothing. Installed over netlink via
// google/nftables (no `nft` binary exists on gokrazy). Idempotent by construction
// (guarded by conntrackSetupOnce); a fresh Flush replaces the table if present.
func registerConntrackHook() error {
	conn, err := nftables.New()
	if err != nil {
		return err
	}
	tbl := conn.AddTable(&nftables.Table{
		Family: nftables.TableFamilyINet,
		Name:   "ze-conntrack",
	})
	policy := nftables.ChainPolicyAccept
	ch := conn.AddChain(&nftables.Chain{
		Name:     "track",
		Table:    tbl,
		Type:     nftables.ChainTypeFilter,
		Hooknum:  nftables.ChainHookPrerouting,
		Priority: nftables.ChainPriorityConntrack,
		Policy:   &policy,
	})
	conn.AddRule(&nftables.Rule{
		Table: tbl,
		Chain: ch,
		Exprs: []expr.Any{
			// Load ct state into a register: the presence of this ct expression
			// (not any match on it) is what registers the conntrack hooks.
			&expr.Ct{Key: expr.CtKeySTATE, Register: 1},
			&expr.Counter{},
		},
	})
	return conn.Flush()
}
