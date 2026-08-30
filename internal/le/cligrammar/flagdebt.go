// Design: docs/architecture/cli/root-namespace-grammar.md -- the flag-register debt
//
// flagdebt.go is the ledger feeder 7 judges against, apart from the scan that
// produces the findings (flags.go).
//
// The tree carried 50 violations when the feeder landed, and 11 more the day F3
// widened to every command of the `ze` surface, so a feeder that failed on all
// of them would have blocked every commit. Each one is listed
// here with the reason it is still there, and the gate prints the whole ledger
// and its count on every run. This is DEBT, never an allowlist: a violation
// that is not listed fails the gate, and deleting an entry is what a fix
// landing looks like.

package cligrammar

import (
	"slices"
	"sort"

	"github.com/ze-software/ze/internal/component/command/grammar"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// flagRegisterDebt is this checkout's tracked flag-register debt: one entry per
// violation that shipped before the feeder existed, each with the reason it is
// still here. It is DEBT, never an allowlist. Every entry is counted and
// printed, deleting an entry is what a fix landing looks like, and a violation
// that is not listed fails the gate.
//
// The key is the rule id, the command, and the flag, spaced as the finding
// prints them. Add nothing here without a reason a reader can act on.
//
// An entry whose violation is GONE is reported as fixed rather than failing the
// gate, so a fix landing in this shared checkout never turns the gate red for
// the session that did not write it. What that report asks for is the entry's
// deletion, which is the last step of the fix rather than a later cleanup.
var flagRegisterDebt = map[string]string{
	"F3 config dump --json":     "`show config dump` is served by registry.MustRegisterLocalData, so `| json` already renders it; the flag is the second spelling",
	"F3 schema handlers --json": "`show schema handlers` is served by registry.MustRegisterLocalData, so `| json` already renders it; the flag is the second spelling",
	"F3 schema list --json":     "`show schema list` is served by registry.MustRegisterLocalData, so `| json` already renders it; the flag is the second spelling",
	"F3 yang tree --json":       "`show yang tree` is served by registry.MustRegisterLocalData, so `| json` already renders it; the flag is the second spelling",
	"F3 yang completion --json": "`show yang completion` is served by registry.MustRegisterLocalData, so `| json` already renders it; the flag is the second spelling",

	// F3 widened to every command of the `ze` surface on 2026-08-30: rendering
	// is the pipe layer's job whether or not a command's answer was registered
	// for it, so "this command reaches no pipe layer" is the defect these
	// entries record rather than the reason they are forgiven. Each names the
	// source that parses the flag, so the fix is one MustRegisterLocalData
	// registration in that package plus the flag's deletion, and the entry
	// then goes.
	"F3 config completion --json": "the flag is parsed in internal/component/config/cli/cmd_completion.go and no registry.MustRegisterLocalData call serves `config completion`, so its answer reaches no pipe layer",
	"F3 config diff --json":       "the flag is parsed in internal/component/config/cli/cmd_diff.go and no registry.MustRegisterLocalData call serves `config diff`, so its answer reaches no pipe layer",
	"F3 config fix --json":        "the flag is parsed in internal/component/config/cli/cmd_fix.go and no registry.MustRegisterLocalData call serves `config fix`, so its answer reaches no pipe layer",
	"F3 config migrate --format":  "the flag is parsed in internal/component/config/cli/cmd_migrate.go and no registry.MustRegisterLocalData call serves `config migrate`, so its answer reaches no pipe layer",
	"F3 config show --json":       "the flag is parsed in internal/component/config/cli/cmd_show.go and no registry.MustRegisterLocalData call serves `config show`, so its answer reaches no pipe layer",
	"F3 config validate --json":   "the flag is parsed in internal/component/config/cli/cmd_validate.go and no registry.MustRegisterLocalData call serves `config validate`, so its answer reaches no pipe layer",
	"F3 interface scan --json":    "the flag is parsed in internal/component/iface/cli/scan.go and no registry.MustRegisterLocalData call serves `interface scan`, so its answer reaches no pipe layer",
	"F3 interface scan --yaml":    "the flag is parsed in internal/component/iface/cli/scan.go and no registry.MustRegisterLocalData call serves `interface scan`, so its answer reaches no pipe layer",
	"F3 schema show --json":       "the flag is parsed in internal/component/config/schema/cli/main.go and no registry.MustRegisterLocalData call serves `schema show`, so its answer reaches no pipe layer",
	"F3 tacacs show --json":       "the flag is parsed in internal/component/tacacs/cli/main.go and no registry.MustRegisterLocalData call serves `tacacs show`, so its answer reaches no pipe layer",
}

// flagPathDebt is one command path's undeclared flags: why they are still
// undeclared, and exactly which flags the entry forgives.
//
// The flags are listed rather than the path being forgiven wholesale, so a path
// on this list that starts parsing a NEW flag still fails the gate. Trimming a
// flag out of an entry is what a partial fix looks like.
type flagPathDebt struct {
	Reason string
	Flags  []string
}

// flagDeclarationDebt is the F4 debt: 43 offline `ze` command paths parse 113
// flags between them, and registry.RegisterCommandFlags declares the flags of
// two paths (`exabgp plugin` and `exabgp migrate`) plus the three `l2tp` verbs.
// Every one of these is invisible to completion until its path is registered.
//
// This is DEBT, never an allowlist. Each entry names the source that parses the
// flags, so the fix is one RegisterCommandFlags call in that package's
// register.go, and the entry then goes.
//
// repeated `--json` is the same token on a different command, and naming a
// constant for it would hide which flag each entry forgives.
//
//nolint:goconst // a ledger of flag tokens and the sources that parse them: a
var flagDeclarationDebt = map[string]flagPathDebt{
	"appliance assemble": {
		Reason: "the flags are parsed in internal/appliance/cmd_assemble.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--keep"},
	},
	"appliance build": {
		Reason: "the flags are parsed in internal/appliance/cmd_build.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--all"},
	},
	"appliance config": {
		Reason: "the flags are parsed in internal/appliance/cmd_config.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--merged"},
	},
	"appliance config-push": {
		Reason: "the flags are parsed in internal/appliance/cmd_config_push.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--all", "--dry-run", "--parallel"},
	},
	"appliance export": {
		Reason: "the flags are parsed in internal/appliance/cmd_export.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--all"},
	},
	"appliance import": {
		Reason: "the flags are parsed in internal/appliance/cmd_import.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--dir", "--force"},
	},
	"appliance init": {
		Reason: "the flags are parsed in internal/appliance/cmd_init.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--batch", "--cert", "--config", "--key", "--managed"},
	},
	"appliance iso": {
		Reason: "the flags are parsed in internal/appliance/cmd_iso.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--builder", "--check", "--image", "--initrd", "--keep-staging", "--kernel", "--output", "--target"},
	},
	"appliance kernel": {
		Reason: "the flags are parsed in internal/appliance/cmd_kernel.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--arch", "--builder", "--evict-cache", "--print-cache-dir", "--profile", "--target", "--version"},
	},
	"appliance push": {
		Reason: "the flags are parsed in internal/appliance/cmd_push.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--all", "--image", "--no-reboot", "--parallel", "--testboot"},
	},
	"appliance replace-cert": {
		Reason: "the flags are parsed in internal/appliance/cmd_cert.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--cert", "--key"},
	},
	"appliance unlock": {
		Reason: "the flags are parsed in internal/appliance/cmd_unlock.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--duration", "--stop"},
	},
	"cli": {
		Reason: "the flags are parsed in internal/component/cli/client/main.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--format", "--remote", "--user", "-c", "-u"},
	},
	"config archive": {
		Reason: "the flags are parsed in internal/component/config/cli/cmd_archive.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--user"},
	},
	"config completion": {
		Reason: "the flags are parsed in internal/component/config/cli/cmd_completion.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--context", "--ghost", "--input", "--json"},
	},
	"config diff": {
		Reason: "the flags are parsed in internal/component/config/cli/cmd_diff.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--json"},
	},
	"config dump": {
		Reason: "the flags are parsed in internal/component/config/cli/cmd_dump.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--json", "--strip-private"},
	},
	"config edit": {
		Reason: "the flags are parsed in internal/component/config/cli/cmd_edit.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--insecure-web", "--user", "--web", "-f", "-u"},
	},
	"config fix": {
		Reason: "the flags are parsed in internal/component/config/cli/cmd_fix.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--json", "--plan"},
	},
	"config fmt": {
		Reason: "the flags are parsed in internal/component/config/cli/cmd_fmt.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--check", "--diff", "-w"},
	},
	"config import": {
		Reason: "the flags are parsed in internal/component/config/cli/cmd_import.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--name"},
	},
	"config migrate": {
		Reason: "the flags are parsed in internal/component/config/cli/cmd_migrate.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--dry-run", "--format", "--list", "-o"},
	},
	"config set": {
		Reason: "the flags are parsed in internal/component/config/cli/cmd_set.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--dry-run", "--reload", "--user", "-u"},
	},
	"config show": {
		Reason: "the flags are parsed in internal/component/config/cli/cmd_show.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--json"},
	},
	"config validate": {
		Reason: "the flags are parsed in internal/component/config/cli/cmd_validate.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--json", "--limit", "--pending", "-q", "-v"},
	},
	"env list": {
		Reason: "the flags are parsed in internal/plugins/env/env.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--verbose", "-v"},
	},
	"init": {
		Reason: "the flags are parsed in internal/plugins/init/main.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--force", "--managed", "--seed", "--web-cert", "--web-cert-name", "--yes"},
	},
	"interface migrate": {
		Reason: "the flags are parsed in internal/component/iface/cli/migrate.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--user", "-u"},
	},
	"interface scan": {
		Reason: "the flags are parsed in internal/component/iface/cli/scan.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--config", "--json", "--managed", "--yaml"},
	},
	"plugin cli": {
		Reason: "the flags are parsed in internal/component/bgp/cli/cmd_plugin.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--name", "--user", "-u"},
	},
	"resolve cymru": {
		Reason: "the flags are parsed in internal/component/resolve/cli/cmd_cymru.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--dns-server"},
	},
	"resolve dns": {
		Reason: "the flags are parsed in internal/component/resolve/cli/cmd_dns.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--dnssec", "--server"},
	},
	"resolve irr": {
		Reason: "the flags are parsed in internal/component/resolve/cli/cmd_irr.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--server"},
	},
	"resolve peeringdb": {
		Reason: "the flags are parsed in internal/component/resolve/cli/cmd_peeringdb.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--url"},
	},
	"schema handlers": {
		Reason: "the flags are parsed in internal/component/config/schema/cli/main.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--json"},
	},
	"schema list": {
		Reason: "the flags are parsed in internal/component/config/schema/cli/main.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--json"},
	},
	"schema show": {
		Reason: "the flags are parsed in internal/component/config/schema/cli/main.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--json"},
	},
	"signal": {
		Reason: "the flags are parsed in internal/plugins/signal/main.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--host", "--port", "--user", "-u"},
	},
	"status": {
		Reason: "the flags are parsed in internal/plugins/signal/main.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--host", "--port"},
	},
	"tacacs show": {
		Reason: "the flags are parsed in internal/component/tacacs/cli/main.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--json"},
	},
	"yang completion": {
		Reason: "the flags are parsed in internal/component/config/yang/cli/main.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--json", "--min-prefix"},
	},
	"yang doc": {
		Reason: "the flags are parsed in internal/component/config/yang/cli/main.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--list"},
	},
	"yang tree": {
		Reason: "the flags are parsed in internal/component/config/yang/cli/main.go and no RegisterCommandFlags call declares them",
		Flags:  []string{"--config", "--json"},
	},
}

// partitionDebt splits the hits into the ones that fail the gate and the ones
// the debt ledgers already track, and reports how much of each entry is still
// in the tree.
//
// An entry whose violations are all gone is reported as FIXED rather than
// failing the gate. Several sessions share this checkout, and a gate that went
// red on somebody else's landed fix is a gate nobody can keep green.
func partitionDebt(hits []FlagRegisterHit) ([]FlagRegisterHit, []FlagDebt) {
	var open []FlagRegisterHit
	present := map[string]int{}
	var tb textbuf.Buffer
	for _, hit := range hits {
		key := tb.Reset().Str(hit.Rule).Byte(' ').Str(hit.Command).Byte(' ').Str(hit.Flag).String()
		if _, tracked := flagRegisterDebt[key]; tracked {
			present[key]++
			continue
		}
		if entry, tracked := flagDeclarationDebt[hit.Command]; tracked &&
			hit.Rule == grammar.RuleFlagUndeclared && slices.Contains(entry.Flags, hit.Flag) {
			present[hit.Command]++
			continue
		}
		open = append(open, hit)
	}

	debt := make([]FlagDebt, 0, len(flagRegisterDebt)+len(flagDeclarationDebt))
	for _, key := range sortedKeys(flagRegisterDebt) {
		debt = append(debt, FlagDebt{
			Entry: key, Reason: flagRegisterDebt[key], Tracked: 1, Present: present[key],
		})
	}
	for _, path := range sortedKeys(flagDeclarationDebt) {
		entry := flagDeclarationDebt[path]
		debt = append(debt, FlagDebt{
			Entry:   tb.Reset().Str(grammar.RuleFlagUndeclared).Byte(' ').Str(path).String(),
			Reason:  entry.Reason,
			Tracked: len(entry.Flags),
			Present: present[path],
		})
	}

	sort.Slice(open, func(i, j int) bool {
		if open[i].Rule != open[j].Rule {
			return open[i].Rule < open[j].Rule
		}
		if open[i].Command != open[j].Command {
			return open[i].Command < open[j].Command
		}
		return open[i].Flag < open[j].Flag
	})
	return open, debt
}
