// Design: docs/guide/route-policy.md -- import admission and retained candidates.
// Related: rib_import_policy_fixture.go -- original assertions retained by these drivers.
// Related: sysrib_execution_fixture.go -- authenticated placement and process census.

package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	importDaemonLink       = "zeimpgw"
	importConnectedLink    = "zeimpc"
	importConnectedReject  = "198.18.221.0/24"
	importConnectedPermit  = "198.18.222.0/24"
	importConnectedControl = "198.18.223.0/24"
	importTagReject        = "198.18.231.0/24"
	importTagZero          = "198.18.232.0/24"
	importTagPlain         = "198.18.233.0/24"
	importRollbackArmed    = "import-rollback.armed"
	importRollbackReached  = "import-rollback.reached"
	importRollbackReceipt  = "import-rollback.undone"
)

// importPolicyDriver leaves the zero-argument committed scenarios intact. The
// new daemon carriers additionally prove placement, kernel output and reload.
func importPolicyDriver(original, additional ObserverScenario) Driver {
	return func(ctx context.Context, args []string) error {
		if len(args) == 0 {
			return observer02("rib-import-test", original)(ctx, args)
		}
		if err := importPlacementArgs(args, 1); err != nil {
			return err
		}
		return Observe(ctx, "rib-import-test", &sdk.Registration{}, func(ctx context.Context, p *sdk.Plugin) (result error) {
			if err := sysribExecutionPlacement(ctx, p, args[0]); err != nil {
				return err
			}
			// A /31 provides a more-specific connected cover for the original
			// injected gateway without competing for its /24 static control.
			if err := importCreateLink(ctx, importDaemonLink, "198.51.100.0/31"); err != nil {
				return err
			}
			defer func() { result = errors.Join(result, importDeleteLink(ctx, importDaemonLink)) }()
			if err := waitRIBWinner(ctx, p, "198.51.100.0/31", "connected"); err != nil {
				return err
			}
			if err := original(ctx, p); err != nil {
				return err
			}
			return additional(ctx, p)
		})
	}
}

func importPlacementArgs(args []string, count int) error {
	if len(args) != count {
		return fmt.Errorf("import daemon fixture needs %d placement arguments", count)
	}
	for _, placement := range args {
		if placement == "internal" {
			continue
		}
		if placement != "forked" {
			return fmt.Errorf("unknown import placement %q: use internal or forked", placement)
		}
	}
	return nil
}

func importCreateLink(ctx context.Context, name string, addresses ...string) (result error) {
	if _, err := importCommandOutput(ctx, "ip", "link", "add", name, "type", "dummy"); err != nil {
		return err
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, importDeleteLink(ctx, name))
		}
	}()
	if _, err := importCommandOutput(ctx, "ip", "link", "set", name, "up"); err != nil {
		return err
	}
	for _, address := range addresses {
		if _, err := importCommandOutput(ctx, "ip", "addr", "add", address, "dev", name); err != nil {
			return err
		}
	}
	return nil
}

func importDeleteLink(ctx context.Context, name string) error {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	_, err := importCommandOutput(cleanup, "ip", "link", "del", name)
	return err
}

// importKernel reads every table and retains inspection errors. A failed read
// must never satisfy a negative assertion about a rejected route.
func importKernel(ctx context.Context, prefix string, want int, words ...string) error {
	var last string
	var readErr error
	if Poll(ctx, importPollAttempts, importPollDelay, func() bool {
		last, readErr = netfilterCommandOutput(ctx, "ip", "-N", "route", "show", "table", "all", "exact", prefix)
		if readErr != nil {
			return false
		}
		var entries []string
		for line := range strings.SplitSeq(last, "\n") {
			if fields := strings.Fields(line); len(fields) > 0 {
				entries = append(entries, strings.Join(fields, " ")+" ")
			}
		}
		if len(entries) != want {
			return false
		}
		for _, entry := range entries {
			for _, word := range words {
				if !strings.Contains(entry, word) {
					return false
				}
			}
		}
		return true
	}) {
		return nil
	}
	return fmt.Errorf("kernel %s: want %d routes containing %q, observed %q; read error: %v", prefix, want, words, last, readErr)
}

func importCentralKernel(ctx context.Context, p *sdk.Plugin) error {
	if err := importKernel(ctx, importControlPrefix, 1, "6 "+importControlPrefix+" ", "proto 250 "); err != nil {
		return err
	}
	if err := importKernel(ctx, importBGPPrefix, 1, "via 198.51.100.1 ", "proto 250 "); err != nil {
		return err
	}
	if err := importKernel(ctx, importJudgedPrefix, 0); err != nil {
		return err
	}
	if err := importCandidateState(ctx, p, importControlPrefix, protocolStatic, 10, nil, true, "", ""); err != nil {
		return err
	}
	if err := importCandidateState(ctx, p, importJudgedPrefix, protocolStatic, 10, nil, false, importHiddenCentral, importChainCentral); err != nil {
		return err
	}
	if err := importCandidateState(ctx, p, importJudgedPrefix, importSourceBGP, 20, nil, false, importHiddenCentral, importChainCentral); err != nil {
		return err
	}
	if err := importCandidateState(ctx, p, importBGPPrefix, importSourceBGP, 20, nil, true, "", ""); err != nil {
		return err
	}
	return importSay("central rejection keeps both candidate reasons and installs both permitted FIB controls")
}

// importCandidateState checks both operator command forms against the same
// expected stored path. The full listing cannot silently omit hidden entries.
func importCandidateState(ctx context.Context, p *sdk.Plugin, prefix, source string, distance uint8, tag *uint32, selected bool, hiddenBy, chain string) error {
	for _, command := range []string{"show rib candidates prefix " + prefix, "show rib candidates"} {
		var last string
		if !Poll(ctx, importPollAttempts, importPollDelay, func() bool {
			data, err := requireDone02(ctx, p, command)
			if err != nil {
				last = err.Error()
				return false
			}
			last = text02(data)
			var rows []candidateRow
			if decode02(data, &rows) != nil {
				return false
			}
			for _, row := range rows {
				if row.Prefix != prefix {
					continue
				}
				path, found := candidatePathOf(row, source)
				if !found {
					return false
				}
				if path.Distance != distance {
					return false
				}
				if path.Selected != selected {
					return false
				}
				if path.Hidden != (hiddenBy != "") {
					return false
				}
				if path.HiddenBy != hiddenBy {
					return false
				}
				if path.Chain != chain {
					return false
				}
				if tag == nil {
					return path.Tag == nil
				}
				if path.Tag == nil {
					return false
				}
				return *path.Tag == *tag
			}
			return false
		}) {
			return fmt.Errorf("%s: %s path metadata did not match distance=%d tag=%v selected=%t hidden-by=%q chain=%q: %s", command, source, distance, tag, selected, hiddenBy, chain, last)
		}
	}
	return nil
}

func importReadConfig() (*config.Tree, *config.Schema, error) {
	data, err := os.ReadFile(fileBGPConf)
	if err != nil {
		return nil, nil, err
	}
	schema, err := config.YANGSchema()
	if err != nil {
		return nil, nil, err
	}
	tree, err := config.NewParser(schema).Parse(string(data))
	return tree, schema, err
}

func importStaticReload(ctx context.Context, p *sdk.Plugin) error {
	tag := uint32(4242)
	if err := importCandidateState(ctx, p, importJudgedPrefix, protocolStatic, 10, &tag, false, importHiddenSource, importChainStatic); err != nil {
		return err
	}
	if err := importStaticOutcome(ctx, p, importSourceBGP, 10, true); err != nil {
		return err
	}
	tree, schema, err := importReadConfig()
	if err != nil {
		return err
	}
	chain := tree.GetContainerPath("static/filter/import")
	if chain == nil {
		return errors.New("static fixture omitted its import chain")
	}
	chain.SetSlice("filters", []string{"SLOW"})
	if err := policyFinalReload(ctx, p, config.Serialize(tree, schema), "applied"); err != nil {
		return err
	}
	if err := importStaticOutcome(ctx, p, protocolStatic, 5, false); err != nil {
		return err
	}
	set := tree.GetContainer("policy").GetList("modify")["SLOW"].GetContainer("set")
	if set == nil {
		return errors.New("static fixture omitted SLOW set")
	}
	set.Set("distance", "40")
	if err := policyFinalReload(ctx, p, config.Serialize(tree, schema), "applied"); err != nil {
		return err
	}
	if err := importStaticOutcome(ctx, p, importSourceBGP, 40, false); err != nil {
		return err
	}
	// The operation participant observes the actual changed candidate, selected
	// view and FIB in reversible commit, then refuses. Finalization happens
	// after acceptance and cannot legitimately stand for rollback.
	active, err := rfc8212ReloadActive()
	if err != nil {
		return err
	}
	if err := os.WriteFile(importRollbackArmed, []byte("refuse after distance 5 reaches FIB"), 0o600); err != nil {
		return err
	}
	set.Set("distance", "5")
	if err := policyFinalReload(ctx, p, config.Serialize(tree, schema), "failed"); err != nil {
		return err
	}
	if _, err := os.Stat(importRollbackReached); err != nil {
		return fmt.Errorf("rollback never reached changed candidate and FIB: %w", err)
	}
	if _, err := os.Stat(importRollbackReceipt); err != nil {
		return fmt.Errorf("operation inverse never ran: %w", err)
	}
	if err := importStaticOutcome(ctx, p, importSourceBGP, 40, false); err != nil {
		return err
	}
	restored, err := rfc8212ReloadActive()
	if err != nil {
		return err
	}
	if !bytes.Equal(active, restored) {
		return errors.New("refused import reload changed durable active configuration")
	}
	return importSay("static re-admission, selective distance reload and actual rollback preserve both stored producers without reinjection")
}

func importStaticOutcome(ctx context.Context, p *sdk.Plugin, winner string, distance uint8, hidden bool) error {
	if err := waitRIBWinner(ctx, p, importJudgedPrefix, winner); err != nil {
		return err
	}
	tag := uint32(4242)
	hiddenBy, chain := "", ""
	if hidden {
		hiddenBy, chain = importHiddenSource, importChainStatic
	}
	if err := importCandidateState(ctx, p, importJudgedPrefix, protocolStatic, distance, &tag, winner == protocolStatic, hiddenBy, chain); err != nil {
		return err
	}
	if err := importCandidateState(ctx, p, importJudgedPrefix, importSourceBGP, 20, nil, winner == importSourceBGP, "", ""); err != nil {
		return err
	}
	if err := importCandidateState(ctx, p, importControlPrefix, protocolStatic, 10, nil, true, "", ""); err != nil {
		return err
	}
	if err := importKernel(ctx, importControlPrefix, 1, "6 "+importControlPrefix+" ", "proto 250 "); err != nil {
		return err
	}
	if winner == protocolStatic {
		return importKernel(ctx, importJudgedPrefix, 1, "6 "+importJudgedPrefix+" ", "proto 250 ")
	}
	return importKernel(ctx, importJudgedPrefix, 1, "via 198.51.100.1 ", "proto 250 ")
}

func importRollbackParticipant(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("import rollback participant takes no arguments")
	}
	p, err := newObserver("import-rollback")
	if err != nil {
		return err
	}
	defer p.Close() //nolint:errcheck // Run retains transport failures.
	p.OnConfigure(func([]sdk.ConfigSection) error { return nil })
	p.OnConfigVerify(func([]sdk.ConfigSection) error { return nil })
	p.OnConfigApply(func([]sdk.ConfigDiffSection) error { return nil })
	p.OnConfigRollback(func(string) error { return nil })
	const operation rpc.ConfigOperationType = "import-policy-rollback-check"
	p.OnConfigOperationDecompose(func(sdk.ConfigOperationDecomposeInput) (*sdk.ConfigOperationDecomposeOutput, error) {
		return &sdk.ConfigOperationDecomposeOutput{Status: rpc.StatusOK, Operations: []rpc.ConfigOperation{{
			ID: "import-policy-observe", Root: "policy", Owner: "import-rollback", Type: operation, Verb: rpc.VerbModify,
			Target: rpc.ResourceRef{Kind: "import-policy-proof", Name: "candidate"},
		}}}, nil
	})
	p.OnConfigOperationVerify(func(sdk.ConfigOperationVerifyInput) error { return nil })
	p.OnConfigOperationApply(func(sdk.ConfigOperationApplyInput) (*sdk.ConfigOperationApplyOutput, error) {
		return &sdk.ConfigOperationApplyOutput{Status: rpc.StatusOK}, nil
	})
	p.OnConfigOperationRollback(func(input sdk.ConfigOperationRollbackInput) error {
		return os.WriteFile(importRollbackReceipt, []byte(input.TransactionID), 0o600)
	})
	p.OnConfigOperationCommit(func(input sdk.ConfigOperationCommitInput) error {
		if _, err := os.Stat(importRollbackArmed); errors.Is(err, os.ErrNotExist) {
			return nil
		} else if err != nil {
			return err
		}
		if err := importStaticOutcome(ctx, p, protocolStatic, 5, false); err != nil {
			return fmt.Errorf("rollback fault before changed FIB: %w", err)
		}
		if err := os.Rename(importRollbackArmed, importRollbackReached); err != nil {
			return err
		}
		return fmt.Errorf("deliberate import-policy reversible commit refusal after transaction %s reached candidate and FIB", input.TransactionID)
	})
	return p.Run(ctx, &sdk.Registration{WantsConfig: []string{"policy"}, ApplyBudget: 30,
		ConfigOperations: []rpc.ConfigOperationDecl{{Root: "policy", Decompose: true, Operations: []rpc.ConfigOperationType{operation}}},
	})
}

func importConnectedDriver(ctx context.Context, args []string) error {
	if err := importPlacementArgs(args, 1); err != nil {
		return err
	}
	return Observe(ctx, "rib-import-test", &sdk.Registration{}, func(ctx context.Context, p *sdk.Plugin) (result error) {
		if err := sysribExecutionPlacement(ctx, p, args[0]); err != nil {
			return err
		}
		if err := importCreateLink(ctx, importConnectedLink, "198.18.221.1/24", "198.18.222.1/24"); err != nil {
			return err
		}
		defer func() { result = errors.Join(result, importDeleteLink(ctx, importConnectedLink)) }()
		if err := waitRIBWinner(ctx, p, importConnectedPermit, "connected"); err != nil {
			return err
		}
		if err := importCandidateState(ctx, p, importConnectedPermit, "connected", 0, nil, true, "", ""); err != nil {
			return err
		}
		if err := importCandidateState(ctx, p, importConnectedReject, "connected", 0, nil, false, "source", "connected import"); err != nil {
			return err
		}
		if winner, raw := ribWinner02(ctx, p, importConnectedReject); winner != "" {
			return fmt.Errorf("rejected connected path selected as %s: %s", winner, raw)
		}
		if err := waitRIBWinner(ctx, p, importConnectedControl, protocolStatic); err != nil {
			return err
		}
		if err := importKernel(ctx, importConnectedControl, 1, "6 "+importConnectedControl+" ", "proto 250 "); err != nil {
			return err
		}
		for _, prefix := range []string{importConnectedReject, importConnectedPermit} {
			if err := importKernel(ctx, prefix, 1, "dev "+importConnectedLink+" ", "proto 2 "); err != nil {
				return err
			}
		}
		return importSay("connected source rejection preserves both OS routes and an admitted connected candidate beside a live Ze-owned FIB control")
	})
}

func importTagDriver(ctx context.Context, args []string) error {
	if err := importPlacementArgs(args, 2); err != nil {
		return err
	}
	return Observe(ctx, "rib-import-test", &sdk.Registration{}, func(ctx context.Context, p *sdk.Plugin) error {
		if err := sysribExecutionPlacement(ctx, p, args[0]); err != nil {
			return err
		}
		if err := sysribExecutionPluginPlacement(ctx, p, "static-owner", "static", args[1]); err != nil {
			return err
		}
		if err := importTagState(ctx, p, false); err != nil {
			return err
		}
		tree, schema, err := importReadConfig()
		if err != nil {
			return err
		}
		chain := tree.GetContainerPath("rib/filter/import")
		if chain == nil {
			return errors.New("tag fixture omitted central import chain")
		}
		chain.SetSlice("filters", []string{"NO4242", "NOZERO"})
		if err := policyFinalReload(ctx, p, config.Serialize(tree, schema), "applied"); err != nil {
			return err
		}
		if err := importTagState(ctx, p, true); err != nil {
			return err
		}
		chain.SetSlice("filters", []string{"NO4242"})
		if err := policyFinalReload(ctx, p, config.Serialize(tree, schema), "applied"); err != nil {
			return err
		}
		if err := importTagState(ctx, p, false); err != nil {
			return err
		}
		return importSay("independent static and RIB placements retain tag 4242, explicit zero and absence through central rejection and re-admission")
	})
}

func importTagState(ctx context.Context, p *sdk.Plugin, zeroHidden bool) error {
	tag, zero := uint32(4242), uint32(0)
	if err := importCandidateState(ctx, p, importTagReject, protocolStatic, 7, &tag, false, "central", "rib import"); err != nil {
		return err
	}
	if err := importKernel(ctx, importTagReject, 0); err != nil {
		return err
	}
	hiddenBy, chain := "", ""
	if zeroHidden {
		hiddenBy, chain = "central", "rib import"
	}
	if err := importCandidateState(ctx, p, importTagZero, protocolStatic, 7, &zero, !zeroHidden, hiddenBy, chain); err != nil {
		return err
	}
	if err := importCandidateState(ctx, p, importTagPlain, protocolStatic, 7, nil, true, "", ""); err != nil {
		return err
	}
	if err := waitRIBWinner(ctx, p, importTagPlain, protocolStatic); err != nil {
		return err
	}
	if err := importKernel(ctx, importTagPlain, 1, "6 "+importTagPlain+" ", "proto 250 "); err != nil {
		return err
	}
	if zeroHidden {
		if winner, raw := ribWinner02(ctx, p, importTagZero); winner != "" {
			return fmt.Errorf("explicit zero reject still selected as %s: %s", winner, raw)
		}
		return importKernel(ctx, importTagZero, 0)
	}
	if err := waitRIBWinner(ctx, p, importTagZero, protocolStatic); err != nil {
		return err
	}
	return importKernel(ctx, importTagZero, 1, "6 "+importTagZero+" ", "proto 250 ")
}

func importDaemonKernelEmpty(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("import kernel-empty fixture takes no arguments")
	}
	for _, prefix := range []string{importJudgedPrefix, importControlPrefix, importBGPPrefix, "198.51.100.0/31", importConnectedReject, importConnectedPermit, importConnectedControl, importTagReject, importTagZero, importTagPlain} {
		if err := importKernel(ctx, prefix, 0); err != nil {
			return err
		}
	}
	return importSay("import fixture prefixes are absent from every kernel table")
}

func importSay(message string) error {
	var proof textbuf.Buffer
	return proof.Str("OK: ").Str(message).Byte('\n').StdErr()
}
