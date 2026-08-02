package dataplane

import (
	"errors"
	"net"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func siteSelector(t *testing.T, cidr string, dir SADir, owner string) SPParams {
	t.Helper()
	_, prefix, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("parse %q: %v", cidr, err)
	}
	_, any4, err := net.ParseCIDR("0.0.0.0/0")
	if err != nil {
		t.Fatalf("parse 0.0.0.0/0: %v", err)
	}
	return SPParams{
		Src:      any4,
		Dst:      prefix,
		Dir:      dir,
		Action:   SPActionProtect,
		Owner:    owner,
		Proto:    ProtoESP,
		Mode:     ModeTunnel,
		Priority: PriorityChildSA,
	}
}

// VALIDATES: a second peer cannot claim a policy selector a first peer already holds.
// PREVENTS: the second site-to-site peer to establish silently capturing the first
// peer's traffic into its own tunnel, which the XFRM upsert allows and the kernel cannot
// refuse because a policy's identity there is its selector alone.
func TestPolicyOwnerRefusesASecondPeerOnOneSelector(t *testing.T) {
	var owners policyOwners

	first := siteSelector(t, "0.0.0.0/0", SADirOut, "site-a")
	if _, err := owners.claim(first); err != nil {
		t.Fatalf("the first peer could not claim an unheld selector: %v", err)
	}

	second := siteSelector(t, "0.0.0.0/0", SADirOut, "site-b")
	_, err := owners.claim(second)
	if err == nil {
		t.Fatal("a second peer claimed a selector the first peer holds: it would capture the first peer's traffic")
	}

	var owned *PolicyOwnedError
	if !errors.As(err, &owned) {
		t.Fatalf("error is %T, want *PolicyOwnedError so a caller can tell a takeover from a netlink failure", err)
	}
	if owned.HeldBy != "site-a" || owned.Wanted != "site-b" {
		t.Errorf("refusal names held-by %q wanted %q, want site-a / site-b", owned.HeldBy, owned.Wanted)
	}
	for _, want := range []string{"site-a", "site-b"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal text does not name %q, so an operator cannot tell which peers collided: %s", want, err)
		}
	}
	if held, _ := owners.ownerOf(first); held != "site-a" {
		t.Errorf("owner after the refused claim = %q, want site-a: the refusal must not hand the selector over", held)
	}
}

// VALIDATES: the SAME owner re-claiming its own selector succeeds.
// PREVENTS: the ownership check breaking a Child SA rekey, whose replacement inherits
// every selector field from the retired pair and must still upsert.
func TestPolicyOwnerAllowsTheSameOwnerToReclaim(t *testing.T) {
	var owners policyOwners

	p := siteSelector(t, "0.0.0.0/0", SADirOut, "site-a")
	created, err := owners.claim(p)
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if !created {
		t.Error("the first claim of an unheld selector did not report itself as new")
	}

	created, err = owners.claim(p)
	if err != nil {
		t.Fatalf("a rekey could not re-claim its own selector, so every rekey would fail: %v", err)
	}
	// The rekey's re-claim MUST report false. InstallPolicy releases the record only
	// when its own claim created it, so a re-claim reporting true would make a failed
	// rekey install forget a selector the kernel still holds, and the next foreign peer
	// could then take that live selector over.
	if created {
		t.Error("a re-claim reported itself as new; a failed rekey install would then " +
			"forget a live selector and let another peer take it over")
	}
}

// VALIDATES: a different owner cannot release a selector it does not hold, and the owner
// can.
// PREVENTS: installChildSA's rollback -- which removes the other direction's policy after
// a failed install -- taking the owning peer's live policy down, leaving that peer's
// tunnel with states and no policy.
func TestPolicyOwnerRefusesAForeignRelease(t *testing.T) {
	var owners policyOwners

	mine := siteSelector(t, "0.0.0.0/0", SADirIn, "site-a")
	if _, err := owners.claim(mine); err != nil {
		t.Fatalf("claim: %v", err)
	}

	foreign := siteSelector(t, "0.0.0.0/0", SADirIn, "site-b")
	var owned *PolicyOwnedError
	if err := owners.release(foreign); !errors.As(err, &owned) {
		t.Fatalf("a foreign release returned %v, want *PolicyOwnedError: it would blackhole the owning peer", err)
	}
	if held, ok := owners.ownerOf(mine); !ok || held != "site-a" {
		t.Errorf("owner after the refused release = %q (present=%v), want site-a", held, ok)
	}

	if err := owners.release(mine); err != nil {
		t.Fatalf("the owner could not release its own selector: %v", err)
	}
	if _, ok := owners.ownerOf(mine); ok {
		t.Error("the record outlived the release, so a later legitimate install would be refused")
	}
	if _, err := owners.claim(foreign); err != nil {
		t.Fatalf("a released selector could not be claimed by another peer: %v", err)
	}
}

// VALIDATES: a policy delete the kernel REFUSES leaves the ownership record intact, so a
// foreign peer's claim on that selector is still refused afterwards.
// PREVENTS: the guard failing open on its own error path. release both checks the owner
// and drops the record, so a delete that failed after it left ze holding no record over a
// policy the kernel still had. The next foreign peer's claim then found no owner,
// succeeded, and upserted over a live tunnel -- the exact takeover the guard exists to
// refuse.
func TestPolicyOwnerKeepsTheRecordWhenTheKernelRefusesTheDelete(t *testing.T) {
	var owners policyOwners

	mine := siteSelector(t, "0.0.0.0/0", SADirOut, "site-a")
	if _, err := owners.claim(mine); err != nil {
		t.Fatalf("claim: %v", err)
	}

	// test-relax: the separate release-then-restore pair is gone because the production
	// sequence it drove is gone. deleteThenRelease never releases before the kernel
	// answers, so there is no restore step left to assert on. The property this test
	// exists for is unchanged and asserted below, and it is now stronger: the record is
	// never absent at all, rather than absent and then put back.
	//
	// EPERM stands for every refusal that is NOT "the kernel has no such policy": the
	// policy is still installed after it, so the record must be too.
	if err := owners.deleteThenRelease(mine, func() error { return syscall.EPERM }); err == nil {
		t.Fatal("a refused kernel delete was reported as success")
	}

	if held, ok := owners.ownerOf(mine); !ok || held != "site-a" {
		t.Fatalf("owner after a refused delete = %q (present=%v), want site-a: the kernel still holds the policy", held, ok)
	}

	foreign := siteSelector(t, "0.0.0.0/0", SADirOut, "site-b")
	var owned *PolicyOwnedError
	if _, err := owners.claim(foreign); !errors.As(err, &owned) {
		t.Fatalf("a foreign peer's claim returned %v, want *PolicyOwnedError: it would upsert over site-a's live policy", err)
	}
}

// VALIDATES: an ENOENT delete leaves the record DROPPED, and still hands the kernel's
// ENOENT back to the caller.
// PREVENTS: two opposite errors. Keeping the record would leave it outliving the policy it
// named and refuse a later, legitimate install of that selector. Swallowing the ENOENT
// would make "ze asked the kernel to remove a policy it did not have" indistinguishable
// from a routine removal, which is a fact about the kernel the caller is entitled to.
func TestPolicyOwnerDropsTheRecordWhenTheKernelHasNoSuchPolicy(t *testing.T) {
	var owners policyOwners

	mine := siteSelector(t, "0.0.0.0/0", SADirOut, "site-a")
	if _, err := owners.claim(mine); err != nil {
		t.Fatalf("claim: %v", err)
	}
	// test-relax: the release-then-restore pair is gone with the production sequence it
	// drove, as in the test above. The assertions it fed are unchanged, and the error
	// expectation is tightened rather than loosened: nil is no longer accepted here.
	if err := owners.deleteThenRelease(mine, func() error { return syscall.ENOENT }); !errors.Is(err, syscall.ENOENT) {
		t.Fatalf("an ENOENT delete returned %v, want the kernel's own ENOENT back", err)
	}

	if held, ok := owners.ownerOf(mine); ok {
		t.Errorf("owner after an ENOENT delete = %q, want no record: the kernel holds no such policy", held)
	}
	foreign := siteSelector(t, "0.0.0.0/0", SADirOut, "site-b")
	if _, err := owners.claim(foreign); err != nil {
		t.Fatalf("a selector the kernel no longer holds could not be claimed: %v", err)
	}
}

// VALIDATES: an accepted delete drops the record, and the kernel is asked exactly once.
// PREVENTS: the two tests above passing because the record is never dropped at all, which
// would refuse every later install of the selector.
func TestPolicyOwnerDropsTheRecordWhenTheKernelAcceptsTheDelete(t *testing.T) {
	var owners policyOwners

	mine := siteSelector(t, "0.0.0.0/0", SADirOut, "site-a")
	if _, err := owners.claim(mine); err != nil {
		t.Fatalf("claim: %v", err)
	}
	calls := 0
	if err := owners.deleteThenRelease(mine, func() error { calls++; return nil }); err != nil {
		t.Fatalf("an accepted delete reported %v", err)
	}
	if calls != 1 {
		t.Fatalf("the kernel delete ran %d times, want 1", calls)
	}
	if held, ok := owners.ownerOf(mine); ok {
		t.Errorf("owner after an accepted delete = %q, want no record", held)
	}
}

// VALIDATES: a delete for a selector a DIFFERENT owner holds never reaches the kernel.
// PREVENTS: a peer whose own install was refused tearing the owning peer's live policy
// down on its way out, which leaves that peer's tunnel with states and no policy.
//
// It replaces the earlier takeover-race test, which asserted that the restore step
// REPORTED a selector taken over between the release and the restore. There is no such
// window now: the record is never dropped before the kernel confirms, so nothing can take
// the selector over mid-sequence and there is no report left to assert on. Reporting a
// takeover was the weaker property; refusing one is what this asserts.
func TestPolicyOwnerRefusesAForeignDeleteBeforeTheKernel(t *testing.T) {
	var owners policyOwners

	mine := siteSelector(t, "0.0.0.0/0", SADirOut, "site-a")
	if _, err := owners.claim(mine); err != nil {
		t.Fatalf("claim: %v", err)
	}

	foreign := siteSelector(t, "0.0.0.0/0", SADirOut, "site-b")
	calls := 0
	err := owners.deleteThenRelease(foreign, func() error { calls++; return nil })

	var owned *PolicyOwnedError
	if !errors.As(err, &owned) {
		t.Fatalf("a foreign delete returned %v, want *PolicyOwnedError", err)
	}
	if calls != 0 {
		t.Fatalf("the foreign delete reached the kernel %d times, want 0: it would blackhole site-a", calls)
	}
	if held, ok := owners.ownerOf(mine); !ok || held != "site-a" {
		t.Errorf("owner after the refused delete = %q (present=%v), want site-a", held, ok)
	}
}

// VALIDATES: selectors that differ in any field the kernel compares are independent.
// PREVENTS: the ownership key collapsing distinct policies together, which would refuse
// installs that the kernel would have accepted (the two directions of one Child SA are
// the everyday case).
func TestPolicyOwnerSeparatesDistinctSelectors(t *testing.T) {
	var owners policyOwners

	base := siteSelector(t, "0.0.0.0/0", SADirOut, "site-a")
	if _, err := owners.claim(base); err != nil {
		t.Fatalf("claim base: %v", err)
	}

	cases := []struct {
		name  string
		muton func(p *SPParams)
	}{
		{"direction", func(p *SPParams) { p.Dir = SADirIn }},
		{"destination prefix", func(p *SPParams) { _, p.Dst, _ = net.ParseCIDR("192.0.2.0/24") }},
		{"upper-layer protocol", func(p *SPParams) { p.UpperProto = 89 }},
		{"source port", func(p *SPParams) { p.SrcPort = PortMatch{Port: 500, Mask: 0xffff} }},
		{"destination port", func(p *SPParams) { p.DstPort = PortMatch{Port: 4500, Mask: 0xffff} }},
		{"interface index", func(p *SPParams) { p.IfIndex = 7 }},
		{"xfrm if_id", func(p *SPParams) { p.IfID = 42 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			other := siteSelector(t, "0.0.0.0/0", SADirOut, "site-b")
			tc.muton(&other)
			if _, err := owners.claim(other); err != nil {
				t.Fatalf("a selector differing in %s was refused, but the kernel treats it as a different policy: %v", tc.name, err)
			}
		})
	}
}

// VALIDATES: a bypass policy is never tracked, in either direction of the registry.
// PREVENTS: the IKE bypass -- which every peer installs identically by design -- being
// refused for the second peer, which would stop that peer's IKE from running at all.
func TestPolicyOwnerDoesNotTrackBypass(t *testing.T) {
	var owners policyOwners

	first := siteSelector(t, "0.0.0.0/0", SADirOut, "site-a")
	first.Action = SPActionBypass
	second := siteSelector(t, "0.0.0.0/0", SADirOut, "site-b")
	second.Action = SPActionBypass

	if _, err := owners.claim(first); err != nil {
		t.Fatalf("claim bypass for the first peer: %v", err)
	}
	if _, err := owners.claim(second); err != nil {
		t.Fatalf("the second peer's IKE bypass was refused, so its IKE would not run: %v", err)
	}
	if err := owners.release(second); err != nil {
		t.Fatalf("release bypass: %v", err)
	}
	if _, ok := owners.ownerOf(first); ok {
		t.Error("a bypass was recorded; it must be exempt so peers do not contend over it")
	}
}

// VALIDATES: releaseBySelector forgets every record matching source, destination and
// direction, whatever the rest of the selector holds.
// PREVENTS: the three-argument RemovePolicy leaving a record behind after the kernel
// policy is gone, which would refuse the next legitimate install of that selector.
func TestPolicyOwnerReleaseBySelectorForgetsEveryMatch(t *testing.T) {
	var owners policyOwners

	plain := siteSelector(t, "0.0.0.0/0", SADirOut, "site-a")
	ospf := siteSelector(t, "0.0.0.0/0", SADirOut, "site-a")
	ospf.UpperProto = 89
	elsewhere := siteSelector(t, "192.0.2.0/24", SADirOut, "site-a")

	for _, p := range []SPParams{plain, ospf, elsewhere} {
		if _, err := owners.claim(p); err != nil {
			t.Fatalf("claim: %v", err)
		}
	}

	owners.releaseBySelector(plain.Src, plain.Dst, plain.Dir)

	if _, ok := owners.ownerOf(plain); ok {
		t.Error("the plain record survived releaseBySelector")
	}
	if _, ok := owners.ownerOf(ospf); ok {
		t.Error("the proto-89 record on the same src/dst/dir survived releaseBySelector")
	}
	if _, ok := owners.ownerOf(elsewhere); !ok {
		t.Error("releaseBySelector forgot a record with a different destination")
	}
}

// VALIDATES: deleteThenRelease holds the registry lock across the kernel delete, so a
// selector with no record cannot be claimed by a second peer mid-delete.
// PREVENTS: the newcomer's fresh policy being removed by the in-flight delete, leaving
// it with states, no outbound policy, and traffic in the clear.
func TestPolicyOwnerHoldsTheSelectorAcrossTheKernelDelete(t *testing.T) {
	var owners policyOwners

	mine := siteSelector(t, "0.0.0.0/0", SADirOut, "site-a")
	foreign := siteSelector(t, "0.0.0.0/0", SADirOut, "site-b")

	// No record is held, which is the branch that used to leave the selector free.
	claimed := make(chan error, 1)
	started := make(chan struct{})
	var duringDelete atomic.Bool

	err := owners.deleteThenRelease(mine, func() error {
		go func() {
			close(started)
			_, err := owners.claim(foreign)
			claimed <- err
		}()
		<-started
		// The claim cannot complete while this delete runs. Without the lock it did,
		// and this delete then removed the policy site-b had just installed.
		select {
		case <-claimed:
			duringDelete.Store(true)
		case <-time.After(50 * time.Millisecond):
		}
		return nil
	})
	if err != nil {
		t.Fatalf("deleteThenRelease: %v", err)
	}
	if duringDelete.Load() {
		t.Fatal("a foreign claim completed while the kernel delete was in flight: " +
			"the delete would remove the policy that claim installed")
	}
	if err := <-claimed; err != nil {
		t.Fatalf("the foreign claim after the delete returned %v, want success: "+
			"the selector is genuinely free once the record is dropped", err)
	}
}
