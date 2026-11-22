// Design: docs/features/interfaces.md -- creation-owned native VLAN custody
//go:build integration && linux

package ifacenetlink

import (
	"errors"
	"testing"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/test/userns"
)

// This is a native-kernel proof. A host skip is not evidence; MAIN must also
// run the named question in the Ze kernel with usable user namespaces/CAP_NET_ADMIN.
// Create one real VLAN and prove the lease gates its maps and native deletion.
func TestNativeAccessVLANLeaseProtectsActualKernelResource(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withVLANQoSNetNS(t, func() {
		addVLANQoSDummy(t, "accproof0")
		backend := &netlinkBackend{}
		t.Cleanup(func() {
			if err := backend.Close(); err != nil {
				t.Error(err)
			}
		})
		if err := backend.CreateVLAN(iface.VLANSpec{Parent: "accproof0", VLANID: 311}); err != nil {
			t.Fatal(err)
		}
		actual, err := backend.GetLink("accproof0.311")
		if err != nil {
			t.Fatal(err)
		}
		binding := iface.Binding{OsName: actual.OsName, Ifindex: actual.Index, Alias: actual.Alias}
		held, err := backend.CaptureNativeVLAN(t.Context(), binding)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := held.Close(); err != nil {
				t.Error(err)
			}
		})
		if err := backend.UpdateVLANQoSMap(binding.OsName, map[uint32]uint32{1: 1}, nil); err != nil {
			t.Fatalf("preparation blocked ordinary static maps: %v", err)
		}
		writer, err := held.Borrow(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := writer.Close(); err != nil {
				t.Error(err)
			}
		})
		if err := backend.UpdateVLANQoSMap(binding.OsName, nil, nil); !errors.Is(err, iface.ErrNativeAccessPending) {
			t.Fatalf("legacy write bypassed original participant: %v", err)
		}
		if err := writer.Replace(t.Context(), iface.NativeVLANMaps{Ingress: map[uint32]uint32{3: 6}, Egress: map[uint32]uint32{6: 3}}); err != nil {
			t.Fatal(err)
		}
		maps, err := writer.Snapshot(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if maps.Ingress[3] != 6 {
			t.Fatalf("native ingress maps: %+v", maps)
		}
		if maps.Egress[6] != 3 {
			t.Fatalf("native egress maps: %+v", maps)
		}
		if err := backend.DeleteInterface(binding.OsName); !errors.Is(err, iface.ErrNativeAccessPending) {
			t.Fatalf("leased native delete: %v", err)
		}
		if err := backend.DeleteInterface("accproof0"); !errors.Is(err, iface.ErrNativeAccessPending) {
			t.Fatalf("leased parent native delete: %v", err)
		}
		foreign := &netlinkBackend{}
		t.Cleanup(func() {
			if err := foreign.Close(); err != nil {
				t.Error(err)
			}
		})
		if err := foreign.DeleteInterface(binding.OsName); err == nil {
			t.Fatal("another backend deleted the original access")
		}
		if _, err := netlink.LinkByName(binding.OsName); err != nil {
			t.Fatalf("original access lost: %v", err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		if err := held.Close(); err != nil {
			t.Fatal(err)
		}
		if err := backend.DeleteInterface(binding.OsName); err != nil {
			t.Fatal(err)
		}
		if _, err := netlink.LinkByName(binding.OsName); err == nil {
			t.Fatal("released original native access survived delete")
		}
	})
}

// These are additional native questions for the original-access SOURCE packet.
// Append to its native_access_integration_linux_test.go after the CREATE
// transaction producer is repaired. No test below fabricates a native receipt.

// A separately opened backend MUST NOT destroy another owner's leased VLAN by
// deleting its ordinary unmarked parent. Use the real kernel cascade, then
// release the lease and require deletion through the original owner to work.
func TestNativeAccessForeignBackendCannotDeleteLeasedParent(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withVLANQoSNetNS(t, func() {
		addVLANQoSDummy(t, "accforeign0")
		original := &netlinkBackend{}
		t.Cleanup(func() {
			if err := original.Close(); err != nil {
				t.Error(err)
			}
		})
		if err := original.CreateVLAN(iface.VLANSpec{Parent: "accforeign0", VLANID: 312}); err != nil {
			t.Fatal(err)
		}
		link, err := original.GetLink("accforeign0.312")
		if err != nil {
			t.Fatal(err)
		}
		held, err := original.CaptureNativeVLAN(t.Context(), iface.Binding{OsName: link.OsName, Ifindex: link.Index, Alias: link.Alias})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := held.Close(); err != nil {
				t.Error(err)
			}
		})
		foreign := &netlinkBackend{}
		t.Cleanup(func() {
			if err := foreign.Close(); err != nil {
				t.Error(err)
			}
		})
		if err := foreign.DeleteInterface("accforeign0"); err == nil {
			t.Fatal("foreign backend deleted the leased parent")
		}
		if _, err := netlink.LinkByName("accforeign0.312"); err != nil {
			t.Fatalf("kernel destroyed original leased child: %v", err)
		}
		if err := held.Close(); err != nil {
			t.Fatal(err)
		}
		if err := original.DeleteInterface("accforeign0"); err != nil {
			t.Fatalf("original parent deletion after release: %v", err)
		}
		if _, err := netlink.LinkByName("accforeign0.312"); err == nil {
			t.Fatal("kernel parent deletion did not remove the released child")
		}
	})
}

// A confirmed parent deletion MUST retire its unleased descendant receipts.
// Recreate that parent and the same VLAN through the same backend, and require
// a new native birth and a usable lease without closing/replacing the backend.
func TestNativeAccessConfirmedParentDeletionAllowsFreshChild(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withVLANQoSNetNS(t, func() {
		addVLANQoSDummy(t, "accrec0")
		backend := &netlinkBackend{}
		t.Cleanup(func() {
			if err := backend.Close(); err != nil {
				t.Error(err)
			}
		})
		spec := iface.VLANSpec{Parent: "accrec0", VLANID: 313}
		if err := backend.CreateVLAN(spec); err != nil {
			t.Fatal(err)
		}
		first, err := backend.GetLink("accrec0.313")
		if err != nil {
			t.Fatal(err)
		}
		originalBirth, err := iface.ParseNativeVLANBirth(first.Alias)
		if err != nil {
			t.Fatal(err)
		}
		if err := backend.DeleteInterface("accrec0"); err != nil {
			t.Fatal(err)
		}
		if _, err := netlink.LinkByName("accrec0.313"); err == nil {
			t.Fatal("native child survived confirmed parent deletion")
		}
		addVLANQoSDummy(t, "accrec0")
		if err := backend.CreateVLAN(spec); err != nil {
			t.Fatalf("stale receipt blocked fresh child: %v", err)
		}
		second, err := backend.GetLink("accrec0.313")
		if err != nil {
			t.Fatal(err)
		}
		held, err := backend.CaptureNativeVLAN(t.Context(), iface.Binding{OsName: second.OsName, Ifindex: second.Index, Alias: second.Alias})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := held.Close(); err != nil {
				t.Error(err)
			}
		})
		identity, err := held.Identity()
		if err != nil {
			t.Fatal(err)
		}
		if identity.Birth == originalBirth {
			t.Fatal("recreated child reused its retired native birth")
		}
	})
}

// Linux's LinkAdd primitive MUST be represented faithfully by the unit fake.
// Verify the recorded platform limitation with direct native create, then prove
// the backend creator publishes only after its separate ownership stamp exists.
func TestNativeAccessCreatorStampsAfterKernelIgnoresCreateAlias(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withVLANQoSNetNS(t, func() {
		addVLANQoSDummy(t, "accalias0")
		parent, err := netlink.LinkByName("accalias0")
		if err != nil {
			t.Fatal(err)
		}
		probe := &netlink.Vlan{Name: "accalias0.314", ParentIndex: parent.Attrs().Index, Alias: "ze:alias-create-probe", VlanId: 314}
		if err := netlink.LinkAdd(probe); err != nil {
			t.Fatal(err)
		}
		actual, err := netlink.LinkByName("accalias0.314")
		if err != nil {
			t.Fatal(err)
		}
		if actual.Attrs().Alias != "" {
			t.Fatalf("LL-RTNL-NEWLINK-NO-ALIAS changed: native create alias %q", actual.Attrs().Alias)
		}
		if err := netlink.LinkDel(actual); err != nil {
			t.Fatal(err)
		}
		backend := &netlinkBackend{}
		t.Cleanup(func() {
			if err := backend.Close(); err != nil {
				t.Error(err)
			}
		})
		if err := backend.CreateVLAN(iface.VLANSpec{Parent: "accalias0", VLANID: 315}); err != nil {
			t.Fatalf("creator relied on ignored create alias: %v", err)
		}
		stamped, err := backend.GetLink("accalias0.315")
		if err != nil {
			t.Fatal(err)
		}
		held, err := backend.CaptureNativeVLAN(t.Context(), iface.Binding{OsName: stamped.OsName, Ifindex: stamped.Index, Alias: stamped.Alias})
		if err != nil {
			t.Fatal(err)
		}
		if err := held.Close(); err != nil {
			t.Fatal(err)
		}
	})
}
