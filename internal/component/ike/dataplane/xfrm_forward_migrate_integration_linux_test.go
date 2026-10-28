// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- live transit-policy migration.
// Related: xfrm_migrate_integration_linux_test.go -- isolated real-kernel fixture.
//
//go:build linux

package dataplane

import (
	"errors"
	"testing"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/test/userns"
)

// TestXFRMMigrationMovesForwardPolicy proves that the kernel's transit template
// follows the inbound endpoints, retains its selector and owner, and is restored
// when the final policy update fails after both states have moved. userns.Enter
// runs it as root over a user and network namespace of its own.
func TestXFRMMigrationMovesForwardPolicy(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	for _, rollback := range []bool{false, true} {
		name := "move"
		if rollback {
			name = "rollback"
		}
		t.Run(name, func(t *testing.T) {
			backend, migration := migrationKernelFixture(t, 64, false)
			forward := migration.Policies[0]
			forward.Dir = SADirFwd
			for _, policy := range migration.Policies {
				if err := backend.RemovePolicyParams(policy); err != nil {
					t.Fatal(err)
				}
			}
			migration.Policies = []SPParams{migration.Policies[0], forward, migration.Policies[1]}
			// Reinstall in the engine's receive-before-send order.
			for _, policy := range migration.Policies {
				if err := backend.InstallPolicy(policy); err != nil {
					t.Fatal(err)
				}
			}
			selector, err := xfrmPolicyFromParams(forward)
			if err != nil {
				t.Fatal(err)
			}
			refused := errors.New("outbound policy update refused after forward policy moved")
			forwardMoved := false
			if rollback {
				update := xfrmMigrationPolicyUpdate
				t.Cleanup(func() { xfrmMigrationPolicyUpdate = update })
				failed := false
				xfrmMigrationPolicyUpdate = func(policy *netlink.XfrmPolicy) error {
					if policy.Dir == netlink.XFRM_DIR_OUT {
						if !failed {
							moved, err := netlink.XfrmPolicyGet(selector)
							if err != nil {
								t.Fatal(err)
							}
							if len(moved.Tmpls) != 1 {
								t.Fatal("forward policy lost its template before outbound failure")
							}
							if !moved.Tmpls[0].Src.Equal(migration.NewRemote) {
								t.Fatal("forward policy had not moved its source before outbound failure")
							}
							if !moved.Tmpls[0].Dst.Equal(migration.NewLocal) {
								t.Fatal("forward policy had not moved its destination before outbound failure")
							}
							forwardMoved = true
							failed = true
							return refused
						}
					}
					return update(policy)
				}
			}
			err = backend.MigrateTunnel(migration)
			if rollback {
				if !forwardMoved {
					t.Fatal("rollback did not run after a real forward policy move")
				}
				if !errors.Is(err, refused) {
					t.Fatalf("migration error=%v, want outbound-policy refusal", err)
				}
				if errors.Is(err, ErrTunnelMigrationLost) {
					t.Fatalf("outbound-policy failure lost the tunnel: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			installed, err := netlink.XfrmPolicyGet(selector)
			if err != nil {
				t.Fatal(err)
			}
			source, target := migration.NewRemote, migration.NewLocal
			if rollback {
				source, target = migration.OldRemote, migration.OldLocal
			}
			if len(installed.Tmpls) != 1 {
				t.Fatalf("forward policy holds %d templates, want one", len(installed.Tmpls))
			}
			if !installed.Tmpls[0].Src.Equal(source) {
				t.Fatalf("forward tunnel source=%v, want %v", installed.Tmpls[0].Src, source)
			}
			if !installed.Tmpls[0].Dst.Equal(target) {
				t.Fatalf("forward tunnel destination=%v, want %v", installed.Tmpls[0].Dst, target)
			}
			if installed.SrcPort != selector.SrcPort || installed.DstPort != selector.DstPort {
				t.Fatal("migration changed the forward policy's negotiated ports")
			}
			owner, known := backend.policies.ownerOf(forward)
			if !known {
				t.Fatal("migration lost forward policy ownership")
			}
			if owner != forward.Owner {
				t.Fatalf("forward policy owner=%q, want %q", owner, forward.Owner)
			}
		})
	}
}
