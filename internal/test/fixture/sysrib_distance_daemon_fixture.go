// Design: docs/architecture/rib/unified-locrib.md -- daemon-owned distance declaration.
// Related: rib_import_daemon_fixture.go -- exact candidate/FIB and reload primitives.

package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	sysribDistanceLink     = "zesrdist"
	sysribDistancePrefix   = "198.19.21.0/24"
	sysribDistanceOverride = "198.19.22.0/24"
	sysribDistanceArmed    = "sysrib-distance.armed"
	sysribDistanceReached  = "sysrib-distance.reached"
	sysribDistanceUndone   = "sysrib-distance.undone"
)

func sysribDistanceDriver(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("distance fixture needs placement and defaults|configured startup")
	}
	if err := importPlacementArgs(args[:1], 1); err != nil {
		return err
	}
	var startup uint8
	if args[1] == "defaults" {
		startup = 10
	} else if args[1] == "configured" {
		startup = 5
	} else {
		return fmt.Errorf("unknown distance startup mode %q", args[1])
	}
	return Observe(ctx, "sysrib-distance-test", &sdk.Registration{}, func(ctx context.Context, p *sdk.Plugin) error {
		if err := sysribExecutionPlacement(ctx, p, args[0]); err != nil {
			return err
		}
		if err := importCreateLink(ctx, sysribDistanceLink, "198.19.20.254/24"); err != nil {
			return err
		}
		// The paired carrier joins daemon shutdown before its dedicated
		// cleanup driver removes this device. Failed runs retain their
		// namespace owner's cleanup rather than deleting live FIB targets.
		if err := waitRIBWinner(ctx, p, "198.19.20.0/24", "connected"); err != nil {
			return err
		}
		override := uint8(3)
		entries := []rpc.RouteInstallEntry{
			{Protocol: "static", AFI: 1, SAFI: 1, Prefix: sysribDistancePrefix, RouteType: 6},
			{Protocol: "bgp", AFI: 1, SAFI: 1, Prefix: sysribDistancePrefix, NextHop: "198.19.20.1", Interface: sysribDistanceLink, OnLink: true, IsBGP: true, IsEBGP: true, RouteType: 1},
			{Protocol: "static", AFI: 1, SAFI: 1, Prefix: sysribDistanceOverride, RouteType: 6, DistanceOverride: &override},
			{Protocol: "bgp", AFI: 1, SAFI: 1, Prefix: sysribDistanceOverride, NextHop: "198.19.20.1", Interface: sysribDistanceLink, OnLink: true, IsBGP: true, IsEBGP: true, RouteType: 1},
		}
		if err := sysribInstall(ctx, p, entries...); err != nil {
			return err
		}
		if err := sysribDistanceState(ctx, p, startup, "static"); err != nil {
			return err
		}
		tree, schema, err := importReadConfig()
		if err != nil {
			return err
		}
		rib := tree.GetContainer("rib")
		if rib == nil {
			rib = config.NewTree()
			tree.SetContainer("rib", rib)
		}
		dist := rib.GetContainer("distance")
		if dist == nil {
			dist = config.NewTree()
			rib.SetContainer("distance", dist)
		}
		if startup == 10 {
			dist.Set("static", "5")
			if err := policyFinalReload(ctx, p, config.Serialize(tree, schema), "applied"); err != nil {
				return err
			}
			if err := sysribDistanceState(ctx, p, 5, "static"); err != nil {
				return err
			}
		}
		// Every later step changes only the RIB owner configuration. No route
		// producer is called again, and the explicit distance3 survivor remains.
		dist.Set("static", "250")
		if err := policyFinalReload(ctx, p, config.Serialize(tree, schema), "applied"); err != nil {
			return err
		}
		if err := sysribDistanceState(ctx, p, 250, "bgp"); err != nil {
			return err
		}
		dist.Set("static", "300")
		if err := policyFinalReload(ctx, p, config.Serialize(tree, schema), "failed"); err != nil {
			return err
		}
		if err := sysribDistanceState(ctx, p, 250, "bgp"); err != nil {
			return err
		}
		dist.Set("static", "250")
		active, err := rfc8212ReloadActive()
		if err != nil {
			return err
		}
		if err := os.WriteFile(sysribDistanceArmed, []byte("refuse after tentative distance5 and actual static FIB"), 0o600); err != nil {
			return err
		}
		dist.Set("static", "5")
		if err := policyFinalReload(ctx, p, config.Serialize(tree, schema), "failed"); err != nil {
			return err
		}
		if _, err := os.Stat(sysribDistanceReached); err != nil {
			return fmt.Errorf("distance rollback never observed tentative static FIB: %w", err)
		}
		if _, err := os.Stat(sysribDistanceUndone); err != nil {
			return fmt.Errorf("distance operation inverse never ran: %w", err)
		}
		if err := sysribDistanceState(ctx, p, 250, "bgp"); err != nil {
			return err
		}
		restored, err := rfc8212ReloadActive()
		if err != nil {
			return err
		}
		if !bytes.Equal(active, restored) {
			return errors.New("failed distance apply changed durable active configuration")
		}
		tree.RemoveContainer("rib")
		if err := policyFinalReload(ctx, p, config.Serialize(tree, schema), "applied"); err != nil {
			return err
		}
		if err := sysribDistanceState(ctx, p, 10, "static"); err != nil {
			return err
		}
		return importSay("startup distance, owner apply/reject/actual rollback and root removal reselect stored routes while the explicit distance3 survivor stays installed without reannouncement")
	})
}

func sysribDistanceState(ctx context.Context, p *sdk.Plugin, distance uint8, winner string) error {
	if err := waitRIBWinner(ctx, p, sysribDistancePrefix, winner); err != nil {
		return err
	}
	if err := importCandidateState(ctx, p, sysribDistancePrefix, "static", distance, nil, winner == "static", "", ""); err != nil {
		return err
	}
	if err := importCandidateState(ctx, p, sysribDistancePrefix, "bgp", 20, nil, winner == "bgp", "", ""); err != nil {
		return err
	}
	if err := importCandidateState(ctx, p, sysribDistanceOverride, "static", 3, nil, true, "", ""); err != nil {
		return err
	}
	if err := importCandidateState(ctx, p, sysribDistanceOverride, "bgp", 20, nil, false, "", ""); err != nil {
		return err
	}
	if err := importKernel(ctx, sysribDistanceOverride, 1, "6 "+sysribDistanceOverride+" ", "proto 250 "); err != nil {
		return err
	}
	if err := importKernel(ctx, "198.19.20.0/24", 1, "dev "+sysribDistanceLink+" ", "proto 2 "); err != nil {
		return err
	}
	if winner == "static" {
		return importKernel(ctx, sysribDistancePrefix, 1, "6 "+sysribDistancePrefix+" ", "proto 250 ")
	}
	return importKernel(ctx, sysribDistancePrefix, 1, "via 198.19.20.1 ", "dev "+sysribDistanceLink+" ", "proto 250 ")
}

func sysribDistanceRollbackDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("distance rollback participant takes no arguments")
	}
	p, err := newObserver("sysrib-distance-rollback")
	if err != nil {
		return err
	}
	defer p.Close() //nolint:errcheck // Run retains transport failures.
	p.OnConfigure(func([]sdk.ConfigSection) error { return nil })
	p.OnConfigVerify(func([]sdk.ConfigSection) error { return nil })
	p.OnConfigApply(func([]sdk.ConfigDiffSection) error { return nil })
	p.OnConfigRollback(func(string) error { return nil })
	const operation rpc.ConfigOperationType = "sysrib-distance-proof"
	p.OnConfigOperationDecompose(func(sdk.ConfigOperationDecomposeInput) (*sdk.ConfigOperationDecomposeOutput, error) {
		return &sdk.ConfigOperationDecomposeOutput{Status: rpc.StatusOK, Operations: []rpc.ConfigOperation{{ID: "sysrib-distance-observe", Root: "rib", Owner: "sysrib-distance-rollback", Type: operation, Verb: rpc.VerbModify, Target: rpc.ResourceRef{Kind: "sysrib-distance-proof", Name: "candidate"}}}}, nil
	})
	p.OnConfigOperationVerify(func(sdk.ConfigOperationVerifyInput) error { return nil })
	p.OnConfigOperationApply(func(sdk.ConfigOperationApplyInput) (*sdk.ConfigOperationApplyOutput, error) {
		return &sdk.ConfigOperationApplyOutput{Status: rpc.StatusOK}, nil
	})
	p.OnConfigOperationRollback(func(input sdk.ConfigOperationRollbackInput) error {
		return os.WriteFile(sysribDistanceUndone, []byte(input.TransactionID), 0o600)
	})
	p.OnConfigOperationCommit(func(input sdk.ConfigOperationCommitInput) error {
		if _, err := os.Stat(sysribDistanceArmed); errors.Is(err, os.ErrNotExist) {
			return nil
		} else if err != nil {
			return err
		}
		if err := sysribDistanceState(ctx, p, 5, "static"); err != nil {
			return fmt.Errorf("distance fault before actual changed FIB: %w", err)
		}
		if err := os.Rename(sysribDistanceArmed, sysribDistanceReached); err != nil {
			return err
		}
		return fmt.Errorf("deliberate distance reversible commit refusal after %s reached actual static FIB", input.TransactionID)
	})
	return p.Run(ctx, &sdk.Registration{WantsConfig: []string{"rib"}, ApplyBudget: 30, ConfigOperations: []rpc.ConfigOperationDecl{{Root: "rib", Decompose: true, Operations: []rpc.ConfigOperationType{operation}}}})
}

func sysribDistanceKernelEmpty(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("distance kernel-empty fixture takes no arguments")
	}
	for _, prefix := range []string{sysribDistancePrefix, sysribDistanceOverride, "198.19.20.0/24"} {
		if err := importKernel(ctx, prefix, 0); err != nil {
			return err
		}
	}
	return importSay("distance fixture prefixes are absent from every kernel table")
}

// This runs only after daemon/await-exit. Ze must have flushed its own
// prefixes while the actual device and its OS-owned route still exist;
// deleting the device first could erase a stale FIB entry and make that
// absence vacuous.
func sysribDistanceLinkCleanup(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("distance post-stop link cleanup takes no arguments")
	}
	for _, prefix := range []string{sysribDistancePrefix, sysribDistanceOverride} {
		if err := importKernel(ctx, prefix, 0); err != nil {
			return err
		}
	}
	if err := importKernel(ctx, "198.19.20.0/24", 1, "dev "+sysribDistanceLink+" ", "proto 2 "); err != nil {
		return err
	}
	if _, err := importCommandOutput(ctx, "ip", "link", "show", "dev", sysribDistanceLink); err != nil {
		return fmt.Errorf("joined daemon stop removed fixture device %s: %w", sysribDistanceLink, err)
	}
	if err := importDeleteLink(ctx, sysribDistanceLink); err != nil {
		return err
	}
	return importSay("joined daemon flushed Ze distance routes while the fixture device and its OS route survived before owned link cleanup")
}
